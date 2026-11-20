package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

type operatorBackupInput struct {
	AccessKeyID     string    `json:"access_key_id"`
	AccessKeySecret string    `json:"access_key_secret"`
	SecurityToken   string    `json:"security_token"`
	ExpiresAt       time.Time `json:"expires_at"`
	EncryptionKey   string    `json:"encryption_key_hex"`
	SnapshotID      string    `json:"snapshot_id"`
	ConfirmOffline  bool      `json:"confirm_offline"`
}
type cloudBackupObject struct {
	Object       StoredObject `json:"object"`
	BackupKey    string       `json:"backup_key"`
	Public       bool         `json:"public"`
	Unregistered bool         `json:"unregistered,omitempty"`
}
type cloudBackupManifest struct {
	Format     int                 `json:"format"`
	SnapshotID string              `json:"snapshot_id"`
	CreatedAt  time.Time           `json:"created_at"`
	Database   string              `json:"database"`
	SQLKey     string              `json:"sql_key"`
	SQLSHA256  string              `json:"sql_sha256"`
	Objects    []cloudBackupObject `json:"objects"`
	Schema     int                 `json:"schema"`
	Counts     map[string]int64    `json:"counts"`
}

const operatorBackupBucket = "zqzhg-backup"

// The website never calls this entry point. Operator STS and the encryption key
// enter through stdin only and are neither stored nor logged.
func (a *App) operatorFiles(ctx context.Context, action string, input io.Reader) error {
	var req operatorBackupInput
	if json.NewDecoder(io.LimitReader(input, 16384)).Decode(&req) != nil {
		return errors.New("operator_input_invalid")
	}
	key, e := hex.DecodeString(req.EncryptionKey)
	if e != nil || len(key) != 32 || req.AccessKeyID == "" || req.AccessKeySecret == "" || req.SecurityToken == "" || req.ExpiresAt.Before(time.Now().Add(15*time.Minute)) {
		return errors.New("operator_sts_and_key_required")
	}
	client := oss.NewClient(oss.LoadDefaultConfig().WithRegion("cn-beijing").WithEndpoint("https://oss-cn-beijing-internal.aliyuncs.com").WithSignatureVersion(oss.SignatureVersionV4).WithCredentialsProvider(credentials.NewStaticCredentialsProvider(req.AccessKeyID, req.AccessKeySecret, req.SecurityToken)).WithRetryMaxAttempts(3))
	if action == "backup" {
		if !req.ConfirmOffline {
			return errors.New("offline_confirmation_required")
		}
		host, port, e := net.SplitHostPort(a.Config.Addr)
		if e != nil {
			return errors.New("http_address_invalid")
		}
		if host == "" || host == "0.0.0.0" || host == "::" {
			host = "127.0.0.1"
		}
		connection, e := net.DialTimeout("tcp", net.JoinHostPort(host, port), time.Second)
		if e == nil {
			connection.Close()
			return errors.New("stop_website_before_operator_backup")
		}
		var busy int64
		if a.DB.Migrator().HasTable(&FileUploadSession{}) {
			if e = a.DB.Model(&FileUploadSession{}).Where("signature_expires_at>? OR upload_state='validating'", time.Now().UTC().Add(-30*time.Second)).Count(&busy).Error; e != nil {
				return e
			}
		}
		if busy > 0 {
			return errors.New("wait_for_upload_signatures_and_validation")
		}
		return a.createCloudBackup(ctx, client, key)
	}
	if !regexp.MustCompile(`^[a-f0-9]{32,64}$`).MatchString(req.SnapshotID) {
		return errors.New("invalid_snapshot_id")
	}
	manifest, e := readCloudBackupManifest(ctx, client, key, req.SnapshotID)
	if e != nil {
		return e
	}
	if action == "verify-backup" {
		for _, o := range manifest.Objects {
			if e := verifyObject(ctx, &ossObjectStore{Client: client, Bucket: operatorBackupBucket, InheritACL: true}, o.BackupKey, o.Object.SHA256, o.Object.Size); e != nil {
				return errors.New("backup_object_checksum_failed")
			}
		}
		if e = verifyCloudSQL(ctx, client, key, manifest, io.Discard); e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"snapshot_id": req.SnapshotID, "verified": true, "objects": len(manifest.Objects), "production_changed": false})
	}
	if action == "restore-isolated" {
		if !cloudFilesEnabled() {
			return errors.New("restore_requires_oss_configuration")
		}
		return a.restoreCloudBackup(ctx, client, key, manifest)
	}
	return errors.New("invalid_operator_action")
}
func (a *App) createCloudBackup(ctx context.Context, client *oss.Client, key []byte) error {
	id := randomToken()
	manifest := cloudBackupManifest{Format: 2, SnapshotID: id, CreatedAt: time.Now().UTC(), Database: a.currentDatabase(), SQLKey: "snapshots/" + id + "/database.sql.zqenc"}
	var objects []StoredObject
	if e := a.DB.Order("object_key").Find(&objects).Error; e != nil {
		return e
	}
	known := map[string]bool{}
	unregistered := map[string]bool{}
	for _, o := range objects {
		known[o.Key] = true
	}
	if e := filepath.WalkDir(a.Config.MediaDir, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		relative, e := filepath.Rel(a.Config.MediaDir, path)
		if e != nil {
			return e
		}
		key := filepath.ToSlash(relative)
		if d.Type()&os.ModeSymlink != 0 {
			return errors.New("backup_source_link_requires_review")
		}
		if known[key] {
			return nil
		}
		if _, e = localObjectPath(a.Config.MediaDir, key); e != nil {
			return e
		}
		sha, size, e := fileSHA(path)
		if e != nil {
			return e
		}
		objects = append(objects, StoredObject{Key: key, StoreID: 0, Size: size, SHA256: sha})
		unregistered[key] = true
		return nil
	}); e != nil {
		return e
	}
	// Committed deletion pins outlive the entire bounded operation, even if killed.
	for _, o := range objects {
		if a.DB.Migrator().HasTable(&FileBackupPin{}) {
			if e := a.DB.Create(&FileBackupPin{SnapshotID: id, ObjectKey: o.Key, ExpiresAt: time.Now().Add(24 * time.Hour)}).Error; e != nil {
				return e
			}
		}
	}
	for _, o := range objects {
		if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(o.SHA256) {
			return errors.New("backup_inventory_checksum_required")
		}
		target := "objects/sha256/" + o.SHA256
		store := &ossObjectStore{Client: client, Bucket: operatorBackupBucket, InheritACL: true}
		if _, e := store.Stat(ctx, target); e != nil {
			if !objectMissing(e) {
				return errors.New("backup_bucket_unavailable")
			}
			if o.StoreID == 0 {
				source := localObjectStore{Root: a.Config.MediaDir}
				if e = verifyObject(ctx, source, o.Key, o.SHA256, o.Size); e != nil {
					return errors.New("backup_source_checksum_failed")
				}
				r, e := source.Open(ctx, o.Key)
				if e != nil {
					return e
				}
				_, e = client.PutObject(ctx, &oss.PutObjectRequest{Bucket: oss.Ptr(operatorBackupBucket), Key: oss.Ptr(target), Body: r, ContentLength: oss.Ptr(o.Size), ContentType: oss.Ptr(mime.TypeByExtension(filepath.Ext(o.Key))), ForbidOverwrite: oss.Ptr("true")})
				r.Close()
				if e != nil {
					return errors.New("backup_copy_failed")
				}
			} else {
				profile, e := a.storageProfile(o.StoreID)
				if e != nil || profile.Kind != "oss" {
					return errors.New("backup_storage_profile_unsupported")
				}
				source := o.RemoteKey
				if source == "" {
					source = o.Key
				}
				if profile.Prefix != "" {
					source = strings.Trim(profile.Prefix, "/") + "/" + source
				}
				if _, e = client.CopyObject(ctx, &oss.CopyObjectRequest{Bucket: oss.Ptr(operatorBackupBucket), Key: oss.Ptr(target), SourceBucket: oss.Ptr(profile.Bucket), SourceKey: oss.Ptr(source), ForbidOverwrite: oss.Ptr("true")}); e != nil {
					return errors.New("backup_copy_failed")
				}
			}
		}
		if e := verifyObject(ctx, store, target, o.SHA256, o.Size); e != nil {
			return errors.New("backup_object_checksum_failed")
		}
		var n int64
		if a.DB.Migrator().HasTable(&EnterpriseFileObject{}) {
			if e := a.DB.Model(&EnterpriseFileObject{}).Where("object_key=? AND object_role='public' AND object_state='active'", o.Key).Count(&n).Error; e != nil {
				return e
			}
		}
		manifest.Objects = append(manifest.Objects, cloudBackupObject{Object: o, BackupKey: target, Public: n > 0 && !unregistered[o.Key], Unregistered: unregistered[o.Key]})
	}
	tool, e := mysqlTool("mysqldump")
	if e != nil {
		return e
	}
	manifest.Counts, e = tableCounts(a.DB)
	if e != nil {
		return e
	}
	if e = a.DB.Table("schema_migrations").Select("MAX(version)").Scan(&manifest.Schema).Error; e != nil {
		return e
	}
	args := append(mysqlArgs(env("DB_USER", "root")), "--single-transaction", "--no-tablespaces", "--hex-blob", "--column-statistics=0", "--set-gtid-purged=OFF", "--skip-comments", a.currentDatabase())
	dumpCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(dumpCtx, tool, args...)
	cmd.Env = mysqlEnvironment(env("DB_PASSWORD", "root"))
	stdout, e := cmd.StdoutPipe()
	if e != nil {
		return e
	}
	if cmd.Start() != nil {
		return errors.New("database_backup_failed")
	}
	hash := sha256.New()
	reader, writer := io.Pipe()
	done := make(chan error, 1)
	go func() {
		e := backupCrypt(io.TeeReader(stdout, hash), writer, key, true)
		if e == nil {
			e = cmd.Wait()
		} else {
			cancel()
			cmd.Wait()
		}
		writer.CloseWithError(e)
		done <- e
	}()
	_, e = oss.NewUploader(client, func(o *oss.UploaderOptions) { o.ParallelNum = 1; o.PartSize = 8 << 20 }).UploadFrom(ctx, &oss.PutObjectRequest{Bucket: oss.Ptr(operatorBackupBucket), Key: oss.Ptr(manifest.SQLKey), ContentType: oss.Ptr("application/octet-stream"), ForbidOverwrite: oss.Ptr("true")}, reader)
	if e != nil {
		cancel()
		reader.CloseWithError(e)
	}
	streamErr := <-done
	if e != nil || streamErr != nil {
		return errors.New("database_backup_stream_failed")
	}
	manifest.SQLSHA256 = hex.EncodeToString(hash.Sum(nil))
	counts, e := tableCounts(a.DB)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(counts, manifest.Counts) {
		return errors.New("backup_source_changed")
	}
	if e = verifyCloudSQL(ctx, client, key, manifest, io.Discard); e != nil {
		return e
	}
	raw, _ := json.Marshal(manifest)
	var encrypted bytes.Buffer
	if e = backupCrypt(bytes.NewReader(raw), &encrypted, key, true); e != nil {
		return e
	}
	if _, e = client.PutObject(ctx, &oss.PutObjectRequest{Bucket: oss.Ptr(operatorBackupBucket), Key: oss.Ptr("snapshots/" + id + "/manifest.json.zqenc"), Body: bytes.NewReader(encrypted.Bytes()), ContentLength: oss.Ptr(int64(encrypted.Len())), ContentType: oss.Ptr("application/octet-stream"), ForbidOverwrite: oss.Ptr("true")}); e != nil {
		return errors.New("backup_manifest_write_failed")
	}
	if _, e = readCloudBackupManifest(ctx, client, key, id); e != nil {
		return e
	}
	if a.DB.Migrator().HasTable(&FileBackupPin{}) {
		if e = a.DB.Where("snapshot_id=?", id).Delete(&FileBackupPin{}).Error; e != nil {
			return e
		}
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"snapshot_id": id, "objects": len(objects), "verified": true, "local_archive_created": false, "production_changed": false})
}
func readCloudBackupManifest(ctx context.Context, c *oss.Client, key []byte, id string) (cloudBackupManifest, error) {
	var m cloudBackupManifest
	r, e := c.GetObject(ctx, &oss.GetObjectRequest{Bucket: oss.Ptr(operatorBackupBucket), Key: oss.Ptr("snapshots/" + id + "/manifest.json.zqenc")})
	if e != nil {
		return m, errors.New("backup_manifest_unavailable")
	}
	defer r.Body.Close()
	var b bytes.Buffer
	if e = backupCrypt(io.LimitReader(r.Body, 32<<20), &b, key, false); e != nil {
		return m, e
	}
	if json.Unmarshal(b.Bytes(), &m) != nil || m.Format != 2 || m.Schema < 1 || m.Schema > 40 || len(m.Counts) == 0 || m.SnapshotID != id || m.SQLKey != "snapshots/"+id+"/database.sql.zqenc" {
		return m, errors.New("invalid_backup_manifest")
	}
	seen := map[string]bool{}
	for _, o := range m.Objects {
		if seen[o.Object.Key] || o.Unregistered && o.Public {
			return m, errors.New("invalid_backup_manifest")
		}
		seen[o.Object.Key] = true
		if !safeObjectKey(o.Object.Key) || o.Object.Size < 0 || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(o.Object.SHA256) || o.BackupKey != "objects/sha256/"+o.Object.SHA256 {
			return m, errors.New("invalid_backup_manifest")
		}
	}
	return m, nil
}
func verifyCloudSQL(ctx context.Context, c *oss.Client, key []byte, m cloudBackupManifest, out io.Writer) error {
	r, e := c.GetObject(ctx, &oss.GetObjectRequest{Bucket: oss.Ptr(operatorBackupBucket), Key: oss.Ptr(m.SQLKey)})
	if e != nil {
		return errors.New("backup_sql_unavailable")
	}
	defer r.Body.Close()
	h := sha256.New()
	if e = backupCrypt(r.Body, io.MultiWriter(out, h), key, false); e != nil {
		return e
	}
	if hex.EncodeToString(h.Sum(nil)) != m.SQLSHA256 {
		return errors.New("backup_sql_checksum_failed")
	}
	return nil
}
func (a *App) restoreCloudBackup(ctx context.Context, c *oss.Client, key []byte, m cloudBackupManifest) error {
	// SQL is bounded scratch only; object bytes use OSS server-side copy.
	path := a.workPath("restore-" + randomToken() + ".sql")
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer os.Remove(path)
	out := &boundedBackupWriter{Writer: f, Remaining: 64 << 20}
	e = verifyCloudSQL(ctx, c, key, m, out)
	closeErr := f.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	name := "zqzt_exhibition_restore_" + randomToken()[:16]
	db, e := a.createRestoreDatabase(ctx, name, path)
	if e != nil {
		return e
	}
	pool, _ := db.DB()
	defer pool.Close()
	counts, e := tableCounts(db)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(counts, m.Counts) {
		return errors.New("restore_validation_rows")
	}
	if e = migrate(db); e != nil {
		return e
	}
	restored := &App{DB: db, Config: a.Config}
	var inventory []StoredObject
	if e = db.Find(&inventory).Error; e != nil {
		return e
	}
	registered := 0
	for _, item := range m.Objects {
		if !item.Unregistered {
			registered++
		}
	}
	if len(inventory) != registered {
		return errors.New("restore_manifest_inventory_mismatch")
	}
	for _, item := range m.Objects {
		if item.Unregistered {
			continue
		}
		var original StoredObject
		if db.First(&original, "object_key=?", item.Object.Key).Error != nil || original.Size != item.Object.Size || original.SHA256 != item.Object.SHA256 {
			return errors.New("restore_manifest_inventory_mismatch")
		}
	}
	if e = restored.initializeCloudFiles(); e != nil {
		return e
	}
	private, e := restored.cloudProfile("private")
	if e != nil {
		return e
	}
	public, e := restored.cloudProfile("public")
	if e != nil {
		return e
	}
	quarantine := []map[string]any{}
	for _, item := range m.Objects {
		profile := private
		key := fmt.Sprintf("restores/%s/%s", m.SnapshotID, randomToken())
		if item.Public {
			profile = public
			key = "platform/published/restored-" + randomToken()
		}
		target := &ossObjectStore{Client: c, Bucket: profile.Bucket, InheritACL: true}
		mimeType := mime.TypeByExtension(filepath.Ext(item.Object.Key))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		if _, e = c.CopyObject(ctx, &oss.CopyObjectRequest{Bucket: oss.Ptr(profile.Bucket), Key: oss.Ptr(key), SourceBucket: oss.Ptr(operatorBackupBucket), SourceKey: oss.Ptr(item.BackupKey), ForbidOverwrite: oss.Ptr("true"), MetadataDirective: oss.Ptr("REPLACE"), ContentType: oss.Ptr(mimeType)}); e != nil {
			return errors.New("restore_object_copy_failed")
		}
		if e = verifyObject(ctx, target, key, item.Object.SHA256, item.Object.Size); e != nil {
			return errors.New("restore_object_checksum_failed")
		}
		if item.Unregistered {
			quarantine = append(quarantine, map[string]any{"original_key": item.Object.Key, "private_key": key, "size": item.Object.Size, "sha256": item.Object.SHA256})
			continue
		}
		if e = db.Model(&StoredObject{}).Where("object_key=?", item.Object.Key).Updates(map[string]any{"store_id": profile.ID, "remote_key": key}).Error; e != nil {
			return e
		}
		if e = db.Model(&EnterpriseFileObject{}).Where("object_key=?", item.Object.Key).Updates(map[string]any{"store_id": profile.ID, "bucket": profile.Bucket, "physical_key": key}).Error; e != nil {
			return e
		}
	}
	// Restored workers must never reclaim original deployment object keys.
	for _, stmt := range []string{
		"DELETE FROM file_backup_pins", "DELETE FROM media_upload_reservations", "DELETE FROM stored_object_replicas",
		"UPDATE file_upload_sessions SET upload_state='restored',object_key='',multipart_id='',lease_token='',lease_until=NULL",
		"UPDATE file_storage_jobs SET status='restored',lease_token='',lease_until=NULL WHERE job_kind<>'publish' AND status IN ('queued','processing','failed')",
		"UPDATE file_storage_jobs SET status='failed',error='restore_requires_confirmation',lease_token='',lease_until=NULL WHERE job_kind='publish' AND status IN ('queued','processing','failed')",
		"UPDATE file_storage_job_items SET status='restored',reservation_id=''",
	} {
		if e = db.Exec(stmt).Error; e != nil {
			return e
		}
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"isolated_database": name, "snapshot_id": m.SnapshotID, "objects": len(m.Objects), "unregistered_files": quarantine, "ready": true, "production_changed": false, "active_target_unchanged": true})
}

type boundedBackupWriter struct {
	io.Writer
	Remaining int64
}

func (w *boundedBackupWriter) Write(b []byte) (int, error) {
	if int64(len(b)) > w.Remaining {
		return 0, errors.New("backup_sql_scratch_limit")
	}
	n, e := w.Writer.Write(b)
	w.Remaining -= int64(n)
	return n, e
}
