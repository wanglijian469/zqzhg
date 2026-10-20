package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BackupPolicy struct {
	Enabled  bool   `json:"enabled"`
	Time     string `json:"time"`
	Keep     int    `json:"keep"`
	TargetID uint64 `json:"target_id"`
	Timezone string `json:"timezone"`
}
type BackupLocation struct {
	Profile     StorageProfile `json:"profile"`
	Credentials string         `json:"credentials"`
}
type backupCatalogRecord struct {
	BackupEntry
	Location *BackupLocation `json:"location,omitempty"`
}
type BackupEntry struct {
	Location  *BackupLocation `json:"-"`
	ID        string          `json:"id"`
	Key       string          `json:"key"`
	StoreID   uint64          `json:"store_id"`
	Size      int64           `json:"size"`
	SHA256    string          `json:"sha256"`
	Scheduled bool            `json:"scheduled"`
	Verified  bool            `json:"verified"`
	CreatedAt time.Time       `json:"created_at"`
}
type BackupManifest struct {
	Format    int              `json:"format"`
	Version   string           `json:"version"`
	Schema    int              `json:"schema"`
	Database  string           `json:"database"`
	CreatedAt time.Time        `json:"created_at"`
	SQLSHA256 string           `json:"sql_sha256"`
	SQLSize   int64            `json:"sql_size"`
	Objects   []StoredObject   `json:"objects"`
	Counts    map[string]int64 `json:"counts"`
}
type backupParams struct {
	Scheduled bool `json:"scheduled"`
}
type restoreParams struct {
	BackupID string `json:"backup_id"`
}
type RestoreResult struct {
	Database string         `json:"database"`
	MediaDir string         `json:"media_dir"`
	Manifest BackupManifest `json:"manifest"`
	Ready    bool           `json:"ready"`
}
type activateParams struct {
	RestoreID string `json:"restore_id"`
}

func (a *App) backupPolicy() BackupPolicy {
	policy := BackupPolicy{Time: "02:00", Keep: 7, Timezone: "Asia/Shanghai"}
	json.Unmarshal([]byte(a.setting("backup_policy", "{}")), &policy)
	policy.Timezone = "Asia/Shanghai"
	return policy
}
func (a *App) backupEntries() ([]BackupEntry, error) {
	data, err := os.ReadFile(filepath.Join(a.operations().dir, "backups", "catalog.json"))
	if os.IsNotExist(err) {
		return []BackupEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	var records []backupCatalogRecord
	err = json.Unmarshal(data, &records)
	items := make([]BackupEntry, 0, len(records))
	for _, record := range records {
		entry := record.BackupEntry
		entry.Location = record.Location
		if entry.Location != nil {
			entry.Location.Profile.Credentials = entry.Location.Credentials
		}
		items = append(items, entry)
	}
	return items, err
}
func (a *App) saveBackupEntries(items []BackupEntry) error {
	records := make([]backupCatalogRecord, 0, len(items))
	for _, entry := range items {
		records = append(records, backupCatalogRecord{entry, entry.Location})
	}
	return atomicJSON(filepath.Join(a.operations().dir, "backups", "catalog.json"), records)
}
func (a *App) backupProfilePinned(id uint64) bool {
	if a.backupPolicy().TargetID == id {
		return true
	}
	items, err := a.backupEntries()
	if err != nil {
		return true
	}
	for _, entry := range items {
		if entry.StoreID == id {
			return true
		}
	}
	return false
}
func (a *App) backupStore(id uint64) (ObjectStore, error) {
	if id == 0 {
		return localObjectStore{a.localBackupDir()}, nil
	}
	return a.store(id)
}
func (a *App) backupEntryStore(entry BackupEntry) (ObjectStore, error) {
	if entry.Location != nil {
		profile := entry.Location.Profile
		profile.RootKey = ""
		profile.Credentials = entry.Location.Credentials
		var current StorageProfile
		if a.DB != nil && a.DB.First(&current, entry.StoreID).Error == nil && sameStorageLocation(profile, current) {
			profile.Credentials = current.Credentials
		}
		return a.profileStore(profile)
	}
	if entry.StoreID == 0 {
		for _, root := range []string{a.localBackupDir(), filepath.Join(a.operations().dir, "backups")} {
			if _, e := (localObjectStore{root}).Stat(context.Background(), entry.Key); e == nil {
				return localObjectStore{root}, nil
			}
		}
	}
	return a.backupStore(entry.StoreID)
}
func (a *App) backupEntry(id string) (BackupEntry, error) {
	items, err := a.backupEntries()
	if err != nil {
		return BackupEntry{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return BackupEntry{}, errors.New("backup_not_found")
}
func mysqlTool(name string) (string, error) {
	if value := os.Getenv("MYSQL_BIN"); value != "" {
		path := filepath.Join(value, name)
		if runtime.GOOS == "windows" {
			path += ".exe"
		}
		if _, e := os.Stat(path); e == nil {
			return path, nil
		}
	}
	if path, e := exec.LookPath(name); e == nil {
		return path, nil
	}
	if runtime.GOOS == "windows" {
		path := filepath.Join("D:/InforGuardMC/InforGuardMC/mysql/bin", name+".exe")
		if _, e := os.Stat(path); e == nil {
			return path, nil
		}
	}
	return "", errors.New("mysql_tools_required")
}
func mysqlArgs(user string) []string {
	return []string{"--no-defaults", "--host=" + env("DB_HOST", "127.0.0.1"), "--port=" + env("DB_PORT", "13306"), "--user=" + user, "--default-character-set=utf8mb4"}
}
func mysqlEnvironment(password string) []string {
	items := []string{}
	for _, value := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(value), "MYSQL_PWD=") {
			items = append(items, value)
		}
	}
	return append(items, "MYSQL_PWD="+password)
}
func (a *App) dumpDatabase(ctx context.Context, path string) error {
	tool, err := mysqlTool("mysqldump")
	if err != nil {
		return err
	}
	args := mysqlArgs(env("DB_USER", "root"))
	args = append(args, "--single-transaction", "--no-tablespaces", "--hex-blob", "--column-statistics=0", "--set-gtid-purged=OFF", "--skip-comments", "--result-file="+path, a.currentDatabase())
	cmd := exec.CommandContext(ctx, tool, args...)
	cmd.Env = mysqlEnvironment(env("DB_PASSWORD", "root"))
	if cmd.Run() != nil {
		return errors.New("database_backup_failed")
	}
	return nil
}
func tableCounts(db *gorm.DB) (map[string]int64, error) {
	tables, err := db.Migrator().GetTables()
	if err != nil {
		return nil, err
	}
	counts := map[string]int64{}
	for _, table := range tables {
		if !regexp.MustCompile(`^[a-zA-Z0-9_]+$`).MatchString(table) {
			return nil, errors.New("invalid_database_table")
		}
		var count int64
		if err = db.Table(table).Count(&count).Error; err != nil {
			return nil, err
		}
		counts[table] = count
	}
	return counts, nil
}
func (a *App) runBackup(ctx context.Context, job *OperationJob) (any, error) {
	r := a.operations()
	r.gate.RLock()
	defer r.gate.RUnlock()
	key, err := a.masterKey()
	if err != nil {
		return nil, err
	}
	if _, err = mysqlTool("mysqldump"); err != nil {
		return nil, err
	}
	var req backupParams
	json.Unmarshal(job.Params, &req)
	policy := a.backupPolicy()
	var location *BackupLocation
	var store ObjectStore
	if policy.TargetID == 0 {
		store, err = a.backupStore(0)
		location = &BackupLocation{Profile: StorageProfile{Kind: "local", LocalRoot: a.localBackupDir()}}
	} else {
		var profile StorageProfile
		profile, err = a.storageProfile(policy.TargetID)
		if err == nil {
			location = &BackupLocation{profile, profile.Credentials}
			if profile.Kind == "oss" {
				if e := a.testStore(ctx, profile); e != nil {
					return nil, e
				}
			}
			store, err = a.profileStore(profile)
		}
	}
	if err != nil {
		return nil, err
	}
	sqlPath := a.workPath(job.ID + ".sql")
	defer os.Remove(sqlPath)
	plainPath := a.workPath(job.ID + ".zip")
	defer os.Remove(plainPath)
	encryptedPath := a.workPath(job.ID + ".zqbak")
	defer os.Remove(encryptedPath)
	a.updateJob(job.ID, "snapshot", 0, 0)
	var objects []StoredObject
	var counts map[string]int64
	var schema int
	r.pauseWrites.Store(true)
	r.writes.Lock()
	snapshotErr := func() error {
		var e error
		objects, e = a.inventory()
		if e != nil {
			return e
		}
		counts, e = tableCounts(a.DB)
		if e != nil {
			return e
		}
		if e = a.DB.Raw("SELECT COALESCE(MAX(version),0) FROM schema_migrations").Scan(&schema).Error; e != nil {
			return e
		}
		return a.dumpDatabase(ctx, sqlPath)
	}()
	r.writes.Unlock()
	r.pauseWrites.Store(false)
	if snapshotErr != nil {
		return nil, snapshotErr
	}
	hash, sqlSize, err := fileSHA(sqlPath)
	if err != nil {
		return nil, err
	}
	manifest := BackupManifest{1, "2.0.0", schema, a.currentDatabase(), time.Now().UTC(), hash, sqlSize, objects, counts}
	archive, err := os.OpenFile(plainPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, err
	}
	writer := zip.NewWriter(archive)
	archiveErr := func() error {
		entry, e := writer.Create("manifest.json")
		if e != nil {
			return e
		}
		if e = json.NewEncoder(entry).Encode(manifest); e != nil {
			return e
		}
		entry, e = writer.Create("database.sql")
		if e != nil {
			return e
		}
		sql, e := os.Open(sqlPath)
		if e != nil {
			return e
		}
		_, e = io.Copy(entry, sql)
		sql.Close()
		if e != nil {
			return e
		}
		for i, object := range objects {
			if e = a.updateJob(job.ID, "copying_media", int64(i), int64(len(objects))); e != nil {
				return e
			}
			_, source, e := a.resolveObject(ctx, object.Key)
			if e != nil {
				return e
			}
			body, e := source.Open(ctx, object.Key)
			if e != nil {
				return objectError(e)
			}
			entry, e = writer.Create("objects/" + object.Key)
			if e != nil {
				body.Close()
				return e
			}
			checksum := sha256.New()
			n, e := io.Copy(io.MultiWriter(entry, checksum), &contextReader{ctx, body})
			body.Close()
			if e != nil {
				return e
			}
			if n != object.Size || hex.EncodeToString(checksum.Sum(nil)) != object.SHA256 {
				return errors.New("checksum_mismatch")
			}
		}
		return nil
	}()
	closeErr := writer.Close()
	fileErr := archive.Close()
	if archiveErr != nil {
		return nil, archiveErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if fileErr != nil {
		return nil, fileErr
	}
	if err = a.updateJob(job.ID, "encrypting", int64(len(objects)), int64(len(objects))); err != nil {
		return nil, err
	}
	if err = cryptBackupFile(plainPath, encryptedPath, key, true); err != nil {
		return nil, err
	}
	archiveHash, size, err := fileSHA(encryptedPath)
	if err != nil {
		return nil, err
	}
	objectKey := "archives/" + job.ID + ".zqbak"
	file, err := os.Open(encryptedPath)
	if err != nil {
		return nil, err
	}
	err = store.Put(ctx, objectKey, file, size)
	file.Close()
	if err != nil {
		return nil, err
	}
	if err = verifyObject(ctx, store, objectKey, archiveHash, size); err != nil {
		return nil, err
	}
	entry := BackupEntry{Location: location, ID: job.ID, Key: objectKey, StoreID: policy.TargetID, Size: size, SHA256: archiveHash, Scheduled: req.Scheduled, Verified: true, CreatedAt: time.Now().UTC()}
	entries, err := a.backupEntries()
	if err != nil {
		return nil, err
	}
	entries = append(entries, entry)
	if err = a.saveBackupEntries(entries); err != nil {
		return nil, err
	}
	if req.Scheduled {
		a.applyBackupRetention(policy.Keep)
	}
	return entry, nil
}
func (a *App) scheduledBackup() {
	policy := a.backupPolicy()
	if !policy.Enabled {
		return
	}
	now := time.Now().In(shanghai)
	if now.Format("15:04") < policy.Time {
		return
	}
	day := now.Format("2006-01-02")
	if a.setting("backup_last_scheduled", "") == day {
		return
	}
	if _, _, err := a.newJob("backup", backupParams{true}, 0); err == nil {
		a.setSetting("backup_last_scheduled", day)
	}
}
func (a *App) applyBackupRetention(keep int) {
	if keep < 1 {
		return
	}
	entries, err := a.backupEntries()
	if err != nil {
		return
	}
	scheduled := []BackupEntry{}
	for _, item := range entries {
		if item.Scheduled {
			scheduled = append(scheduled, item)
		}
	}
	sort.Slice(scheduled, func(i, j int) bool { return scheduled[i].CreatedAt.After(scheduled[j].CreatedAt) })
	remove := map[string]bool{}
	for i := keep; i < len(scheduled); i++ {
		remove[scheduled[i].ID] = true
	}
	remaining := []BackupEntry{}
	for _, entry := range entries {
		if !remove[entry.ID] {
			remaining = append(remaining, entry)
			continue
		}
		store, err := a.backupEntryStore(entry)
		if err != nil || store.Delete(context.Background(), entry.Key) != nil {
			remaining = append(remaining, entry)
		}
	}
	a.saveBackupEntries(remaining)
}
func (a *App) backupRoutes(admin *gin.RouterGroup) {
	admin.GET("/backups", func(c *gin.Context) {
		entries, err := a.backupEntries()
		if err != nil {
			fail(c, 500, "backup_catalog_unavailable")
			return
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].CreatedAt.After(entries[j].CreatedAt) })
		var lastDownload OperationLog
		a.DB.Where("action=?", "backup_downloaded").Order("id DESC").Limit(1).Find(&lastDownload)
		var downloadedAt *time.Time
		if lastDownload.ID > 0 {
			downloadedAt = &lastDownload.CreatedAt
		}
		c.JSON(200, gin.H{"items": entries, "policy": a.backupPolicy(), "last_download_at": downloadedAt})
	})
	admin.PUT("/backups/policy", func(c *gin.Context) {
		var policy BackupPolicy
		if c.ShouldBindJSON(&policy) != nil || !regexp.MustCompile(`^(?:[01][0-9]|2[0-3]):[0-5][0-9]$`).MatchString(policy.Time) || policy.Keep < 1 || policy.Keep > 90 {
			fail(c, 400, "invalid_request")
			return
		}
		policy.Timezone = "Asia/Shanghai"
		if _, err := a.backupStore(policy.TargetID); err != nil {
			fail(c, 400, "invalid_storage_profile")
			return
		}
		if policy.Enabled {
			if _, err := a.masterKey(); err != nil {
				fail(c, 400, jobFailure(err))
				return
			}
			if _, err := mysqlTool("mysqldump"); err != nil {
				fail(c, 400, "mysql_tools_required")
				return
			}
		}
		data, _ := json.Marshal(policy)
		if err := a.setSetting("backup_policy", string(data)); err != nil {
			dbFail(c, err)
			return
		}
		a.audit(actor(c).ID, "backup", 0, "backup_policy_saved")
		c.JSON(200, policy)
	})
	admin.POST("/backups", func(c *gin.Context) { a.respondNewJob(c, "backup", backupParams{}) })
	admin.GET("/backups/:id/download", a.downloadBackup)
	admin.POST("/backups/:id/verify", func(c *gin.Context) {
		if _, err := a.backupEntry(c.Param("id")); err != nil {
			fail(c, 404, "not_found")
			return
		}
		a.respondNewJob(c, "verify", restoreParams{c.Param("id")})
	})
	admin.POST("/backups/:id/restore", func(c *gin.Context) {
		if _, err := a.backupEntry(c.Param("id")); err != nil {
			fail(c, 404, "not_found")
			return
		}
		a.respondNewJob(c, "restore", restoreParams{c.Param("id")})
	})
	admin.POST("/restores/:id/activate", func(c *gin.Context) { a.activateRestore(c, false) })
	admin.POST("/restores/rollback", func(c *gin.Context) { a.activateRestore(c, true) })
	admin.POST("/backups/imports", a.beginBackupImport)
	admin.PUT("/backups/imports/:id/chunks/:index", a.uploadBackupChunk)
	admin.POST("/backups/imports/:id/complete", a.finishBackupImport)
}
func (a *App) downloadBackup(c *gin.Context) {
	_ = http.NewResponseController(c.Writer).SetWriteDeadline(time.Now().Add(24 * time.Hour))
	entry, err := a.backupEntry(c.Param("id"))
	if err != nil {
		fail(c, 404, "not_found")
		return
	}
	store, err := a.backupEntryStore(entry)
	if err != nil {
		fail(c, 503, "storage_unavailable")
		return
	}
	body, err := store.Open(c.Request.Context(), entry.Key)
	if err != nil {
		fail(c, 503, "storage_unavailable")
		return
	}
	defer body.Close()
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", strconv.FormatInt(entry.Size, 10))
	c.Header("Content-Disposition", "attachment; filename=backup-"+entry.ID+".zqbak")
	if who, ok := c.Get("account"); ok {
		a.audit(who.(Account).ID, "backup", 0, "backup_downloaded")
	}
	c.Status(200)
	io.Copy(c.Writer, body)
}
func (a *App) fetchBackup(ctx context.Context, id, destination string) error {
	entry, err := a.backupEntry(id)
	if err != nil {
		return err
	}
	store, err := a.backupEntryStore(entry)
	if err != nil {
		return err
	}
	body, err := store.Open(ctx, entry.Key)
	if err != nil {
		return objectError(err)
	}
	defer body.Close()
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(file, hash), &contextReader{ctx, io.LimitReader(body, entry.Size+1)})
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil && (n != entry.Size || hex.EncodeToString(hash.Sum(nil)) != entry.SHA256) {
		err = errors.New("checksum_mismatch")
	}
	if err != nil {
		os.Remove(destination)
	}
	return err
}
func (a *App) unpackBackup(ctx context.Context, job *OperationJob, destination string) (BackupManifest, error) {
	var req restoreParams
	var manifest BackupManifest
	if json.Unmarshal(job.Params, &req) != nil {
		return manifest, errors.New("invalid_job")
	}
	key, err := a.masterKey()
	if err != nil {
		return manifest, err
	}
	encrypted := a.workPath(job.ID + "-in.zqbak")
	plain := a.workPath(job.ID + "-in.zip")
	defer os.Remove(encrypted)
	defer os.Remove(plain)
	if err = a.fetchBackup(ctx, req.BackupID, encrypted); err != nil {
		return manifest, err
	}
	if err = cryptBackupFile(encrypted, plain, key, false); err != nil {
		return manifest, err
	}
	archive, err := zip.OpenReader(plain)
	if err != nil {
		return manifest, errors.New("invalid_backup")
	}
	defer archive.Close()
	var manifestFile *zip.File
	files := map[string]*zip.File{}
	var total uint64
	maxBytes := uint64(settingInt("MAX_BACKUP_GIB", 20)) << 30
	for _, file := range archive.File {
		if _, exists := files[file.Name]; exists {
			return manifest, errors.New("invalid_backup_duplicate")
		}
		if file.Mode()&os.ModeSymlink != 0 || file.FileInfo().IsDir() {
			return manifest, errors.New("invalid_backup_entry")
		}
		if file.UncompressedSize64 > maxBytes || total > maxBytes-file.UncompressedSize64 {
			return manifest, errors.New("invalid_backup_size")
		}
		total += file.UncompressedSize64
		files[file.Name] = file
		if file.Name == "manifest.json" {
			manifestFile = file
		}
	}
	if manifestFile == nil || manifestFile.UncompressedSize64 > 8<<20 {
		return manifest, errors.New("invalid_backup_manifest")
	}
	body, err := manifestFile.Open()
	if err != nil {
		return manifest, err
	}
	err = json.NewDecoder(io.LimitReader(body, 8<<20)).Decode(&manifest)
	body.Close()
	if err != nil || manifest.Format != 1 || manifest.Schema > 16 || manifest.Schema < 10 || len(manifest.Objects) > 100000 {
		return manifest, errors.New("backup_version_unsupported")
	}
	if _, exists := files["database.sql"]; !exists {
		return manifest, errors.New("invalid_backup_sql")
	}
	if uint64(len(files)) != uint64(len(manifest.Objects))+2 {
		return manifest, errors.New("invalid_backup_entries")
	}
	if err = os.MkdirAll(destination, 0700); err != nil {
		return manifest, err
	}
	free, err := diskFreeBytes(destination)
	if err != nil || free < total+uint64(manifest.SQLSize) {
		return manifest, errors.New("disk_space_insufficient")
	}
	expected := map[string]struct {
		Hash string
		Size int64
	}{"database.sql": {manifest.SQLSHA256, manifest.SQLSize}}
	for _, object := range manifest.Objects {
		if !safeObjectKey(object.Key) || object.Size < 0 || len(object.SHA256) != 64 {
			return manifest, errors.New("invalid_backup_object")
		}
		name := "objects/" + object.Key
		if _, exists := expected[name]; exists {
			return manifest, errors.New("invalid_backup_duplicate")
		}
		expected[name] = struct {
			Hash string
			Size int64
		}{object.SHA256, object.Size}
	}
	index := int64(0)
	for name, want := range expected {
		if err = a.updateJob(job.ID, "validating", index, int64(len(expected))); err != nil {
			return manifest, err
		}
		file := files[name]
		if file == nil || file.UncompressedSize64 != uint64(want.Size) {
			return manifest, errors.New("invalid_backup_size")
		}
		path, err := localObjectPath(destination, name)
		if err != nil {
			return manifest, err
		}
		if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return manifest, err
		}
		source, err := file.Open()
		if err != nil {
			return manifest, err
		}
		target, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			source.Close()
			return manifest, err
		}
		hash := sha256.New()
		n, e := io.Copy(io.MultiWriter(target, hash), &contextReader{ctx, io.LimitReader(source, want.Size+1)})
		source.Close()
		closeErr := target.Close()
		if e == nil {
			e = closeErr
		}
		if e != nil {
			return manifest, e
		}
		if n != want.Size || hex.EncodeToString(hash.Sum(nil)) != want.Hash {
			return manifest, errors.New("checksum_mismatch")
		}
		index++
	}
	return manifest, nil
}
func (a *App) runVerify(ctx context.Context, job *OperationJob) (any, error) {
	r := a.operations()
	r.gate.RLock()
	defer r.gate.RUnlock()
	dir := filepath.Join(r.dir, "verification", job.ID)
	defer removePrivateTree(r.dir, dir)
	manifest, err := a.unpackBackup(ctx, job, dir)
	if err != nil {
		return nil, err
	}
	var req restoreParams
	json.Unmarshal(job.Params, &req)
	entries, err := a.backupEntries()
	if err != nil {
		return nil, err
	}
	for i := range entries {
		if entries[i].ID == req.BackupID {
			entries[i].Verified = true
		}
	}
	if err = a.saveBackupEntries(entries); err != nil {
		return nil, err
	}
	return gin.H{"verified": true, "objects": len(manifest.Objects), "schema": manifest.Schema}, nil
}
func removePrivateTree(root, target string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("unsafe_cleanup_path")
	}
	return os.RemoveAll(target)
}

// Restricted identifiers are generated by the application; restore SQL never
// executes as the maintenance account.
func (a *App) runRestore(ctx context.Context, job *OperationJob) (any, error) {
	r := a.operations()
	r.gate.RLock()
	defer r.gate.RUnlock()
	if _, err := mysqlTool("mysql"); err != nil {
		return nil, err
	}
	directory := filepath.Join(r.dir, "restores", job.ID)
	manifest, err := a.unpackBackup(ctx, job, directory)
	if err != nil {
		return nil, err
	}
	if err = a.updateJob(job.ID, "restoring_database", 0, 1); err != nil {
		return nil, err
	}
	name := "zqzt_exhibition_restore_" + job.ID[:16]
	db, err := a.createRestoreDatabase(ctx, name, filepath.Join(directory, "database.sql"))
	if err != nil {
		return nil, err
	}
	pool, _ := db.DB()
	defer pool.Close()
	counts, err := tableCounts(db)
	if err != nil {
		return nil, err
	}
	for table, want := range manifest.Counts {
		if counts[table] != want {
			return nil, errors.New("restore_validation_rows")
		}
	}
	if err = validateMediaInventory(db, manifest.Objects); err != nil {
		return nil, err
	}
	var admins int64
	if db.Model(&Account{}).Where("role='admin' AND password_hash<>''").Count(&admins).Error != nil || admins == 0 {
		return nil, errors.New("restore_validation_admin_required")
	}
	if err = migrate(db); err != nil {
		return nil, err
	}
	if err = db.Where("1=1").Delete(&StoredObjectReplica{}).Error; err != nil {
		return nil, err
	}
	if err = db.Where("1=1").Delete(&MediaObjectOwner{}).Error; err != nil {
		return nil, err
	}
	if err = db.Model(&MerchantStorageQuota{}).Where("1=1").Update("initialized", false).Error; err != nil {
		return nil, err
	}
	if err = db.Where("1=1").Delete(&MediaUploadReservation{}).Error; err != nil {
		return nil, err
	}
	if err = db.Model(&StoredObject{}).Where("1=1").Update("store_id", 0).Error; err != nil {
		return nil, err
	}
	if err = db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&SystemSetting{SettingKey: "active_storage", Value: "0", UpdatedAt: time.Now().UTC()}).Error; err != nil {
		return nil, err
	}
	if err = atomicJSON(filepath.Join(directory, "manifest.json"), manifest); err != nil {
		return nil, err
	}
	os.Remove(filepath.Join(directory, "database.sql"))
	return RestoreResult{Database: name, MediaDir: filepath.Join(directory, "objects"), Manifest: manifest, Ready: true}, nil
}
func (a *App) activateRestore(c *gin.Context, rollback bool) {
	var req struct {
		Password         string `json:"password"`
		RestoredUsername string `json:"restored_username"`
		RestoredPassword string `json:"restored_password"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	who := actor(c)
	if !permit("restore-reauth:"+strconv.FormatUint(who.ID, 10), 10, time.Minute) {
		fail(c, 429, "rate_limited")
		return
	}
	if who.Role != "admin" || bcrypt.CompareHashAndPassword([]byte(who.PasswordHash), []byte(req.Password)) != nil {
		fail(c, 403, "invalid_credentials")
		return
	}
	var target ActiveTarget
	if rollback {
		host, err := readHostTargetFrom(a.operations().dir)
		if err != nil || host.Previous == nil {
			fail(c, 409, "rollback_unavailable")
			return
		}
		target = *host.Previous
	} else {
		r := a.operations()
		r.mu.Lock()
		job := r.jobs[c.Param("id")]
		if job == nil || job.Kind != "restore" || job.State != "succeeded" {
			r.mu.Unlock()
			fail(c, 409, "restore_not_ready")
			return
		}
		var result RestoreResult
		err := json.Unmarshal(job.Result, &result)
		r.mu.Unlock()
		if err != nil || !result.Ready {
			fail(c, 409, "restore_not_ready")
			return
		}
		target = ActiveTarget{result.Database, result.MediaDir}
	}
	db, err := openExistingDB(target.Database, env("DB_USER", "root"), env("DB_PASSWORD", "root"))
	if err != nil {
		fail(c, 400, "restore_validation_database")
		return
	}
	pool, _ := db.DB()
	defer pool.Close()
	username := req.RestoredUsername
	if username == "" {
		username = who.Username
	}
	password := req.RestoredPassword
	if password == "" {
		password = req.Password
	}
	var restoredAdmin Account
	if db.Where("username=? AND role='admin'", username).First(&restoredAdmin).Error != nil || bcrypt.CompareHashAndPassword([]byte(restoredAdmin.PasswordHash), []byte(password)) != nil {
		fail(c, 400, "restored_admin_credentials_required")
		return
	}
	kind := "activate"
	if rollback {
		kind = "rollback"
	}
	a.respondNewJob(c, kind, activateParams{c.Param("id")})
}
func readHostTargetFrom(dir string) (HostTarget, error) {
	var target HostTarget
	data, err := os.ReadFile(filepath.Join(dir, "active.json"))
	if err != nil {
		return target, err
	}
	err = json.Unmarshal(data, &target)
	return target, err
}
func (a *App) runActivation(ctx context.Context, job *OperationJob, rollback bool) (any, error) {
	r := a.operations()
	var target ActiveTarget
	if rollback {
		host, err := readHostTargetFrom(r.dir)
		if err != nil || host.Previous == nil {
			return nil, errors.New("restore_not_ready")
		}
		target = *host.Previous
	} else {
		var req activateParams
		if json.Unmarshal(job.Params, &req) != nil {
			return nil, errors.New("invalid_job")
		}
		r.mu.Lock()
		prepared := r.jobs[req.RestoreID]
		if prepared == nil || prepared.State != "succeeded" {
			r.mu.Unlock()
			return nil, errors.New("restore_not_ready")
		}
		var result RestoreResult
		err := json.Unmarshal(prepared.Result, &result)
		r.mu.Unlock()
		if err != nil || !result.Ready {
			return nil, errors.New("restore_not_ready")
		}
		target = ActiveTarget{result.Database, result.MediaDir}
	}
	next, err := openExistingDB(target.Database, env("DB_USER", "root"), env("DB_PASSWORD", "root"))
	if err != nil {
		return nil, err
	}
	nextPool, _ := next.DB()
	committed := false
	defer func() {
		if !committed {
			nextPool.Close()
		}
	}()
	if _, err = os.Stat(target.MediaDir); err != nil {
		return nil, errors.New("restore_validation_media")
	}
	if err = a.updateJob(job.ID, "maintenance", 0, 1); err != nil {
		return nil, err
	}
	r.maintenance.Store(true)
	defer r.maintenance.Store(false)
	r.gate.Lock()
	defer r.gate.Unlock()
	previous := ActiveTarget{a.currentDatabase(), a.Config.MediaDir}
	host := HostTarget{Current: target, Previous: &previous, Pending: true, Outcome: "pending", JobID: job.ID}
	if err = atomicJSON(filepath.Join(r.dir, "active.json"), host); err != nil {
		return nil, err
	}
	old := a.DB
	oldMedia := a.Config.MediaDir
	a.DB = next
	a.Config.MediaDir = target.MediaDir
	restoreOld := func() {
		a.DB = old
		a.Config.MediaDir = oldMedia
		host.Current = previous
		host.Previous = &target
		host.Pending = false
		host.Outcome = "rolled_back"
		atomicJSON(filepath.Join(r.dir, "active.json"), host)
	}
	if err = nextPool.PingContext(ctx); err != nil {
		restoreOld()
		return nil, errors.New("restore_validation_health")
	}
	var adminCount int64
	if next.Model(&Account{}).Where("role='admin'").Count(&adminCount).Error != nil || adminCount == 0 {
		restoreOld()
		return nil, errors.New("restore_validation_admin_required")
	}
	if err = next.Transaction(func(tx *gorm.DB) error {
		if e := tx.Where("1=1").Delete(&Session{}).Error; e != nil {
			return e
		}
		return tx.Where("1=1").Delete(&SMSCode{}).Error
	}); err != nil {
		restoreOld()
		return nil, err
	}
	host.Pending = false
	host.Outcome = "committed"
	if err = atomicJSON(filepath.Join(r.dir, "active.json"), host); err != nil {
		restoreOld()
		return nil, err
	}
	committed = true
	oldPool, _ := old.DB()
	oldPool.Close()
	return gin.H{"active_database": target.Database, "relogin_required": true, "rollback_available": true}, nil
}
