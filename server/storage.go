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
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type StorageProfile struct {
	RootKey     string    `json:"root_key"`
	ID          uint64    `json:"id"`
	Name        string    `json:"name"`
	Kind        string    `json:"kind"`
	LocalRoot   string    `json:"local_root"`
	Region      string    `json:"region"`
	Endpoint    string    `json:"endpoint"`
	Bucket      string    `json:"bucket"`
	Prefix      string    `json:"prefix"`
	Credentials string    `json:"-"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type StoredObject struct {
	Key       string    `gorm:"primaryKey;column:object_key" json:"key"`
	StoreID   uint64    `json:"store_id"`
	Size      int64     `json:"size"`
	SHA256    string    `json:"sha256"`
	UpdatedAt time.Time `json:"updated_at"`
}
type StorageCredentials struct {
	AccessKeyID     string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
}
type ObjectInfo struct {
	Size        int64
	Modified    time.Time
	ContentType string
}
type ObjectStore interface {
	Put(context.Context, string, io.Reader, int64) error
	Open(context.Context, string) (io.ReadCloser, error)
	Stat(context.Context, string) (ObjectInfo, error)
	Delete(context.Context, string) error
}
type localObjectStore struct{ Root string }

func safeObjectKey(key string) bool {
	return key != "" && len(key) <= 255 && !strings.ContainsAny(key, ":\\\x00\r\n") && !strings.HasPrefix(key, "/") && func() bool {
		for _, p := range strings.Split(key, "/") {
			if p == ".." || p == "." || p == "" {
				return false
			}
		}
		return true
	}()
}
func localObjectPath(root, key string) (string, error) {
	if !safeObjectKey(key) {
		return "", errors.New("invalid_object_key")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, filepath.FromSlash(key))
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("invalid_object_key")
	}
	// Never follow a link in a storage prefix into a different directory.
	current := root
	for _, part := range strings.Split(filepath.FromSlash(key), string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, e := os.Lstat(current)
		if e == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("invalid_object_key")
		}
		if e != nil && !os.IsNotExist(e) {
			return "", e
		}
	}
	return path, nil
}
func (s localObjectStore) Put(ctx context.Context, key string, body io.Reader, size int64) error {
	path, err := localObjectPath(s.Root, key)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if _, err = os.Lstat(path); err == nil {
		return os.ErrExist
	} else if !os.IsNotExist(err) {
		return err
	}
	// Publish only complete objects. A killed process leaves an unreferenced temp
	// file; retry never encounters a partially published immutable object.
	temp := filepath.Join(filepath.Dir(path), ".put-"+randomToken()+".part")
	file, err := os.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer os.Remove(temp)
	n, err := io.Copy(file, &contextReader{ctx, body})
	if err == nil && n != size {
		err = errors.New("checksum_mismatch")
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	// Link publishes without replacing an existing name on both NTFS and ext4.
	return os.Link(temp, path)
}
func (s localObjectStore) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	path, err := localObjectPath(s.Root, key)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}
func (s localObjectStore) Stat(ctx context.Context, key string) (ObjectInfo, error) {
	path, err := localObjectPath(s.Root, key)
	if err != nil {
		return ObjectInfo{}, err
	}
	stat, err := os.Stat(path)
	if err != nil || stat.IsDir() {
		if err == nil {
			err = os.ErrNotExist
		}
		return ObjectInfo{}, err
	}
	return ObjectInfo{stat.Size(), stat.ModTime(), mime.TypeByExtension(filepath.Ext(key))}, nil
}
func (s localObjectStore) Delete(ctx context.Context, key string) error {
	path, err := localObjectPath(s.Root, key)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

type ossObjectStore struct {
	Client *oss.Client
	Bucket string
	Prefix string
}

func (s *ossObjectStore) key(key string) string {
	if s.Prefix == "" {
		return key
	}
	return strings.Trim(s.Prefix, "/") + "/" + key
}
func (s *ossObjectStore) Put(ctx context.Context, key string, body io.Reader, size int64) error {
	if !safeObjectKey(key) {
		return errors.New("invalid_object_key")
	}
	_, err := s.Client.PutObject(ctx, &oss.PutObjectRequest{Bucket: oss.Ptr(s.Bucket), Key: oss.Ptr(s.key(key)), Body: body, ContentLength: oss.Ptr(size), Acl: oss.ObjectACLPrivate, ForbidOverwrite: oss.Ptr("true"), ContentType: oss.Ptr(mime.TypeByExtension(filepath.Ext(key)))})
	return err
}
func (s *ossObjectStore) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	result, err := s.Client.GetObject(ctx, &oss.GetObjectRequest{Bucket: oss.Ptr(s.Bucket), Key: oss.Ptr(s.key(key))})
	if err != nil {
		return nil, err
	}
	return result.Body, nil
}
func (s *ossObjectStore) Stat(ctx context.Context, key string) (ObjectInfo, error) {
	result, err := s.Client.HeadObject(ctx, &oss.HeadObjectRequest{Bucket: oss.Ptr(s.Bucket), Key: oss.Ptr(s.key(key))})
	if err != nil {
		return ObjectInfo{}, err
	}
	modified := time.Time{}
	if result.LastModified != nil {
		modified = *result.LastModified
	}
	contentType := ""
	if result.ContentType != nil {
		contentType = *result.ContentType
	}
	return ObjectInfo{result.ContentLength, modified, contentType}, nil
}
func (s *ossObjectStore) Delete(ctx context.Context, key string) error {
	_, err := s.Client.DeleteObject(ctx, &oss.DeleteObjectRequest{Bucket: oss.Ptr(s.Bucket), Key: oss.Ptr(s.key(key))})
	return err
}
func (a *App) storageProfile(id uint64) (StorageProfile, error) {
	if id == 0 {
		return StorageProfile{Name: "ECS / Local", Kind: "local", RootKey: "primary", LocalRoot: a.Config.MediaDir}, nil
	}
	var profile StorageProfile
	err := a.DB.First(&profile, id).Error
	return profile, err
}
func (a *App) store(id uint64) (ObjectStore, error) {
	profile, err := a.storageProfile(id)
	if err != nil {
		return nil, err
	}
	return a.profileStore(profile)
}
func (a *App) profileStore(profile StorageProfile) (ObjectStore, error) {
	if profile.Kind == "local" {
		resolved, e := a.resolveLocalProfile(profile)
		if e != nil {
			return nil, e
		}
		return localObjectStore{resolved.LocalRoot}, nil
	}
	if profile.Kind != "oss" {
		return nil, errors.New("invalid_storage_profile")
	}
	raw, err := a.openSecret(profile.Credentials)
	if err != nil {
		return nil, err
	}
	var secret StorageCredentials
	if json.Unmarshal(raw, &secret) != nil {
		return nil, errors.New("invalid_encrypted_config")
	}
	cfg := oss.LoadDefaultConfig().WithRegion(profile.Region).WithCredentialsProvider(credentials.NewStaticCredentialsProvider(secret.AccessKeyID, secret.AccessKeySecret))
	if profile.Endpoint != "" {
		cfg = cfg.WithEndpoint(profile.Endpoint)
	}
	return &ossObjectStore{oss.NewClient(cfg), profile.Bucket, profile.Prefix}, nil
}
func (a *App) defaultStoreID() uint64 {
	id, _ := strconv.ParseUint(a.setting("active_storage", "0"), 10, 64)
	return id
}
func (a *App) objectRecord(key string) (StoredObject, error) {
	if !safeObjectKey(key) {
		return StoredObject{}, errors.New("invalid_object_key")
	}
	var object StoredObject
	err := a.DB.First(&object, "object_key=?", key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		hash, size, e := fileSHA(filepath.Join(a.Config.MediaDir, filepath.FromSlash(key)))
		if e != nil {
			return object, e
		}
		object = StoredObject{Key: key, StoreID: 0, Size: size, SHA256: hash, UpdatedAt: time.Now().UTC()}
		if e = a.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&object).Error; e != nil {
			return object, e
		}
		a.DB.First(&object, "object_key=?", key)
		return object, nil
	}
	return object, err
}
func (a *App) storeWorkFile(ctx context.Context, key, path string) error {
	if !safeObjectKey(key) {
		return errors.New("invalid_object_key")
	}
	hash, size, err := fileSHA(path)
	if err != nil {
		return err
	}
	storeID := a.defaultStoreID()
	profile, pe := a.storageProfile(storeID)
	if pe != nil {
		return pe
	}
	if pe = a.checkLocalWritable(profile); pe != nil {
		return pe
	}
	store, err := a.store(storeID)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, e := store.Stat(ctx, key); e == nil {
		if err = verifyObject(ctx, store, key, hash, size); err != nil {
			return err
		}
	} else if err = store.Put(ctx, key, file, size); err != nil {
		return err
	}
	object := StoredObject{Key: key, StoreID: storeID, Size: size, SHA256: hash, UpdatedAt: time.Now().UTC()}
	return a.DB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "object_key"}}, DoUpdates: clause.AssignmentColumns([]string{"store_id", "size", "sha256", "updated_at"})}).Create(&object).Error
}
func verifyObject(ctx context.Context, store ObjectStore, key, expected string, size int64) error {
	body, err := store.Open(ctx, key)
	if err != nil {
		return objectError(err)
	}
	defer body.Close()
	hash := sha256.New()
	n, err := io.Copy(hash, &contextReader{ctx, body})
	if err != nil {
		return err
	}
	if n != size || hex.EncodeToString(hash.Sum(nil)) != expected {
		return errors.New("checksum_mismatch")
	}
	return nil
}
func (a *App) materializeObject(ctx context.Context, key string) (string, error) {
	object, store, err := a.resolveObject(ctx, key)
	if err != nil {
		return "", err
	}
	body, err := store.Open(ctx, key)
	if err != nil {
		return "", err
	}
	defer body.Close()
	path := a.workPath(randomToken() + filepath.Ext(key))
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(file, hash), &contextReader{ctx, body})
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil && (n != object.Size || hex.EncodeToString(hash.Sum(nil)) != object.SHA256) {
		err = errors.New("checksum_mismatch")
	}
	if err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}
func (a *App) serveObject(c *gin.Context, key string) {
	object, store, err := a.resolveObject(c.Request.Context(), key)
	if err != nil {
		fail(c, 503, "storage_unavailable")
		return
	}
	if local, ok := store.(localObjectStore); ok {
		path, err := localObjectPath(local.Root, key)
		if err != nil {
			fail(c, 404, "not_found")
			return
		}
		file, err := os.Open(path)
		if err != nil {
			fail(c, 404, "not_found")
			return
		}
		defer file.Close()
		stat, err := file.Stat()
		if err != nil {
			fail(c, 404, "not_found")
			return
		}
		http.ServeContent(c.Writer, c.Request, key, stat.ModTime(), file)
		return
	}
	serveOSSObject(c, store.(*ossObjectStore), key, object.Size)
}
func serveOSSObject(c *gin.Context, cloud *ossObjectStore, key string, size int64) {
	ctx := c.Request.Context()
	if c.Request.Method == "HEAD" {
		info, err := cloud.Stat(ctx, key)
		if err != nil {
			fail(c, 404, "not_found")
			return
		}
		c.Header("Content-Length", strconv.FormatInt(info.Size, 10))
		c.Header("Content-Type", info.ContentType)
		c.Header("Accept-Ranges", "bytes")
		c.Status(200)
		return
	}
	request := &oss.GetObjectRequest{Bucket: oss.Ptr(cloud.Bucket), Key: oss.Ptr(cloud.key(key)), RangeBehavior: oss.Ptr("standard")}
	if value := c.GetHeader("Range"); value != "" {
		request.Range = oss.Ptr(value)
	}
	result, err := cloud.Client.GetObject(ctx, request)
	if err != nil {
		var service *oss.ServiceError
		if errors.As(err, &service) && service.StatusCode == 416 {
			c.Header("Content-Range", fmt.Sprintf("bytes */%d", size))
			c.Status(416)
			return
		}
		fail(c, 404, "not_found")
		return
	}
	defer result.Body.Close()
	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Length", strconv.FormatInt(result.ContentLength, 10))
	if result.ContentType != nil {
		c.Header("Content-Type", *result.ContentType)
	}
	status := 200
	if result.ContentRange != nil {
		c.Header("Content-Range", *result.ContentRange)
		status = 206
	}
	c.Status(status)
	_, _ = io.Copy(c.Writer, result.Body)
}
func (a *App) inventory() ([]StoredObject, error) {
	err := filepath.WalkDir(a.Config.MediaDir, func(path string, entry os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("invalid_object_key")
		}
		if entry.IsDir() {
			return nil
		}
		key, e := filepath.Rel(a.Config.MediaDir, path)
		if e != nil {
			return e
		}
		key = filepath.ToSlash(key)
		if strings.HasPrefix(entry.Name(), ".put-") {
			return nil
		}
		if !safeObjectKey(key) {
			return errors.New("invalid_object_key")
		}
		var n int64
		if e = a.DB.Model(&StoredObject{}).Where("object_key=?", key).Count(&n).Error; e != nil {
			return e
		}
		if n > 0 {
			return nil
		}
		sha, size, e := fileSHA(path)
		if e != nil {
			return e
		}
		return a.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&StoredObject{Key: key, Size: size, SHA256: sha, UpdatedAt: time.Now().UTC()}).Error
	})
	if err != nil {
		return nil, err
	}
	var objects []StoredObject
	err = a.DB.Order("object_key").Find(&objects).Error
	if err == nil {
		err = validateMediaInventory(a.DB, objects)
	}
	return objects, err
}
func (a *App) validateProfile(profile StorageProfile) error {
	if strings.TrimSpace(profile.Name) == "" || len(profile.Name) > 100 {
		return errors.New("invalid_storage_profile")
	}
	if profile.Kind == "local" {
		root, err := filepath.Abs(profile.LocalRoot)
		if err != nil || !filepath.IsAbs(profile.LocalRoot) || filepath.Dir(root) == root {
			return errors.New("invalid_storage_path")
		}
		web, _ := filepath.Abs(a.Config.WebDir)
		rel, e := filepath.Rel(web, root)
		if e == nil && (rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")) {
			return errors.New("invalid_storage_path")
		}
		return nil
	}
	if profile.Kind != "oss" || profile.Region == "" || profile.Bucket == "" || len(profile.Bucket) > 100 {
		return errors.New("invalid_storage_profile")
	}
	if profile.Endpoint != "" {
		u, err := url.Parse(profile.Endpoint)
		if err != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || !strings.HasSuffix(u.Hostname(), ".aliyuncs.com") {
			return errors.New("invalid_oss_endpoint")
		}
	}
	if profile.Prefix != "" && !safeObjectKey(strings.Trim(profile.Prefix, "/")) {
		return errors.New("invalid_object_key")
	}
	return nil
}
func (a *App) testStore(ctx context.Context, profile StorageProfile) error {
	if e := a.checkLocalWritable(profile); e != nil {
		return e
	}
	store, err := a.profileStore(profile)
	if err != nil {
		return err
	}
	if cloud, ok := store.(*ossObjectStore); ok {
		acl, e := cloud.Client.GetBucketAcl(ctx, &oss.GetBucketAclRequest{Bucket: oss.Ptr(cloud.Bucket)})
		if e != nil || acl.ACL == nil || *acl.ACL != "private" {
			return errors.New("storage_private_bucket_required")
		}
	}
	key := ".probe-" + randomToken()
	data := []byte(randomToken())
	if err = store.Put(ctx, key, bytes.NewReader(data), int64(len(data))); err != nil {
		return errors.New("storage_unavailable")
	}
	hash := sha256.Sum256(data)
	err = verifyObject(ctx, store, key, hex.EncodeToString(hash[:]), int64(len(data)))
	deleteErr := store.Delete(ctx, key)
	if err == nil {
		err = deleteErr
	}
	return err
}
func (a *App) storageRoutes(admin *gin.RouterGroup) {
	admin.GET("/storage/profiles", func(c *gin.Context) {
		var profiles []StorageProfile
		if e := a.DB.Order("id").Find(&profiles).Error; e != nil {
			dbFail(c, e)
			return
		}
		legacy, _ := a.storageProfile(0)
		profiles = append([]StorageProfile{legacy}, profiles...)
		items := []gin.H{}
		for _, profile := range profiles {
			items = append(items, gin.H{"profile": profile, "credentials_configured": profile.Credentials != "", "active": profile.ID == a.defaultStoreID()})
		}
		c.JSON(200, gin.H{"items": items, "roots": a.localRoots()})
	})
	admin.POST("/storage/profiles", a.saveStorageProfile)
	admin.PUT("/storage/profiles/:id", a.saveStorageProfile)
	admin.POST("/storage/profiles/:id/test", func(c *gin.Context) {
		profile, err := a.storageProfile(id(c))
		if err != nil {
			fail(c, 404, "not_found")
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()
		if err = a.testStore(ctx, profile); err != nil {
			fail(c, 400, jobFailure(err))
			return
		}
		c.JSON(200, gin.H{"ok": true})
	})
	admin.POST("/storage/profiles/:id/activate", func(c *gin.Context) {
		profile, err := a.storageProfile(id(c))
		if err != nil {
			fail(c, 404, "not_found")
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()
		if err = a.testStore(ctx, profile); err != nil {
			fail(c, 400, jobFailure(err))
			return
		}
		if err = a.setSetting("active_storage", strconv.FormatUint(profile.ID, 10)); err != nil {
			dbFail(c, err)
			return
		}
		a.audit(actor(c).ID, "storage", profile.ID, "storage_activated")
		c.JSON(200, gin.H{"ok": true})
	})
	admin.DELETE("/storage/profiles/:id", func(c *gin.Context) {
		r := a.operations()
		if !r.work.TryLock() {
			fail(c, 409, "job_busy")
			return
		}
		defer r.work.Unlock()
		profileID := id(c)
		var count int64
		a.DB.Model(&StoredObject{}).Where("store_id=?", profileID).Count(&count)
		var replicaCount int64
		a.DB.Model(&StoredObjectReplica{}).Where("store_id=?", profileID).Count(&replicaCount)
		count += replicaCount
		if profileID == 0 || count > 0 || a.defaultStoreID() == profileID || a.backupProfilePinned(profileID) {
			fail(c, 409, "storage_in_use")
			return
		}
		if err := a.DB.Delete(&StorageProfile{}, profileID).Error; err != nil {
			dbFail(c, err)
			return
		}
		a.audit(actor(c).ID, "storage", profileID, "storage_deleted")
		c.JSON(200, gin.H{"ok": true})
	})
	admin.POST("/storage/migrations", func(c *gin.Context) {
		var req migrationParams
		if c.ShouldBindJSON(&req) != nil || req.SourceID == req.TargetID {
			fail(c, 400, "invalid_request")
			return
		}
		if _, err := a.store(req.TargetID); err != nil {
			fail(c, 400, "invalid_storage_profile")
			return
		}
		a.respondNewJob(c, "migration", req)
	})
	admin.POST("/storage/cleanup", a.cleanupRetiredObjects)
}
func (a *App) saveStorageProfile(c *gin.Context) {
	var req struct {
		StorageProfile
		AccessKeyID     string `json:"access_key_id"`
		AccessKeySecret string `json:"access_key_secret"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	profile := req.StorageProfile
	profile.ID = 0
	if profile.Kind == "local" {
		root, e := a.registeredRoot(profile.RootKey, profile.LocalRoot)
		if e != nil {
			fail(c, 400, e.Error())
			return
		}
		profile.RootKey = root.Key
		profile.LocalRoot = root.Path
	}
	if c.Request.Method == "PUT" {
		var old StorageProfile
		if a.DB.First(&old, id(c)).Error != nil {
			fail(c, 404, "not_found")
			return
		}
		var count int64
		a.DB.Model(&StoredObject{}).Where("store_id=?", old.ID).Count(&count)
		if (count > 0 || func() bool {
			var n int64
			a.DB.Model(&StoredObjectReplica{}).Where("store_id=?", old.ID).Count(&n)
			return n > 0
		}() || a.backupProfilePinned(old.ID)) && (profile.Kind != old.Kind || profile.LocalRoot != old.LocalRoot || profile.Region != old.Region || profile.Endpoint != old.Endpoint || profile.Bucket != old.Bucket || profile.Prefix != old.Prefix) {
			fail(c, 409, "storage_in_use")
			return
		}
		profile.ID = old.ID
		profile.CreatedAt = old.CreatedAt
		profile.Credentials = old.Credentials
	}

	if err := a.validateProfile(profile); err != nil {
		fail(c, 400, err.Error())
		return
	}
	if profile.Kind == "oss" {
		if req.AccessKeyID != "" || req.AccessKeySecret != "" {
			if req.AccessKeyID == "" || req.AccessKeySecret == "" {
				fail(c, 400, "invalid_credentials")
				return
			}
			raw, _ := json.Marshal(StorageCredentials{req.AccessKeyID, req.AccessKeySecret})
			sealed, err := a.sealSecret(raw)
			if err != nil {
				fail(c, 400, jobFailure(err))
				return
			}
			profile.Credentials = sealed
		}
		if profile.Credentials == "" {
			fail(c, 400, "invalid_credentials")
			return
		}
	}
	legacy, _ := a.storageProfile(0)
	profiles := []StorageProfile{legacy}
	var saved []StorageProfile
	a.DB.Find(&saved)
	profiles = append(profiles, saved...)
	for _, existing := range profiles {
		if (profile.ID == 0 || existing.ID != profile.ID) && sameStorageLocation(profile, existing) {
			fail(c, 409, "duplicate_storage_location")
			return
		}
	}
	if err := a.DB.Save(&profile).Error; err != nil {
		dbFail(c, err)
		return
	}
	a.audit(actor(c).ID, "storage", profile.ID, "storage_saved")
	c.JSON(200, gin.H{"profile": profile, "credentials_configured": profile.Credentials != ""})
}

type migrationParams struct {
	SourceID uint64 `json:"source_id"`
	TargetID uint64 `json:"target_id"`
}
type retiredObject struct {
	Object    StoredObject `json:"object"`
	LocalRoot string       `json:"local_root"`
	Database  string       `json:"database"`
}

func (a *App) currentDatabase() string {
	var name string
	a.DB.Raw("SELECT DATABASE()").Scan(&name)
	return name
}
func (a *App) runStorageMigration(ctx context.Context, job *OperationJob) (any, error) {
	r := a.operations()
	r.gate.RLock()
	defer r.gate.RUnlock()
	var req migrationParams
	if json.Unmarshal(job.Params, &req) != nil || req.SourceID == req.TargetID {
		return nil, errors.New("invalid_job")
	}
	if _, err := a.inventory(); err != nil {
		return nil, err
	}
	var objects []StoredObject
	if err := a.DB.Where("store_id=?", req.SourceID).Order("object_key").Find(&objects).Error; err != nil {
		return nil, err
	}
	sourceProfile, err := a.storageProfile(req.SourceID)
	if err != nil {
		return nil, err
	}
	targetProfile, err := a.storageProfile(req.TargetID)
	if err != nil {
		return nil, err
	}
	if e := a.checkLocalWritable(targetProfile); e != nil {
		return nil, e
	}
	if targetProfile.Kind == "oss" {
		if e := a.testStore(ctx, targetProfile); e != nil {
			return nil, e
		}
	}
	if sameStorageLocation(sourceProfile, targetProfile) {
		return nil, errors.New("duplicate_storage_location")
	}
	source, err := a.store(req.SourceID)
	if err != nil {
		return nil, err
	}
	target, err := a.store(req.TargetID)
	if err != nil {
		return nil, err
	}
	retired := []retiredObject{}
	retiredPath := filepath.Join(r.dir, "retired.json")
	if data, e := os.ReadFile(retiredPath); e == nil {
		if json.Unmarshal(data, &retired) != nil {
			return nil, errors.New("invalid_retired_catalog")
		}
	}
	for i, object := range objects {
		if err = a.updateJob(job.ID, "copying", int64(i), int64(len(objects))); err != nil {
			return nil, err
		}
		body, e := source.Open(ctx, object.Key)
		if e != nil {
			return nil, objectError(e)
		}
		if _, e = target.Stat(ctx, object.Key); e == nil {
			e = verifyObject(ctx, target, object.Key, object.SHA256, object.Size)
		} else {
			e = target.Put(ctx, object.Key, body, object.Size)
		}
		body.Close()
		if e != nil {
			return nil, e
		}
		if e = verifyObject(ctx, target, object.Key, object.SHA256, object.Size); e != nil {
			return nil, e
		}
		localRoot := ""
		if local, ok := source.(localObjectStore); ok {
			localRoot = local.Root
		}
		database := a.currentDatabase()
		listed := false
		for _, old := range retired {
			if old.Database == database && old.Object.Key == object.Key && old.Object.StoreID == object.StoreID {
				listed = true
				break
			}
		}
		if !listed {
			retired = append(retired, retiredObject{object, localRoot, database})
		}
		if e = atomicJSON(retiredPath, retired); e != nil {
			return nil, e
		}
		if e = a.registerReplica(a.DB, object, localRoot); e != nil {
			return nil, e
		}
		if e = a.DB.Model(&StoredObject{}).Where("object_key=? AND store_id=?", object.Key, req.SourceID).Updates(map[string]any{"store_id": req.TargetID, "updated_at": time.Now().UTC()}).Error; e != nil {
			return nil, e
		}
	}
	a.updateJob(job.ID, "complete", int64(len(objects)), int64(len(objects)))
	return gin.H{"migrated": len(objects)}, nil
}
func (a *App) cleanupRetiredObjects(c *gin.Context) {
	var req struct {
		Confirm bool `json:"confirm"`
	}
	if c.ShouldBindJSON(&req) != nil || !req.Confirm {
		fail(c, 400, "confirmation_required")
		return
	}
	r := a.operations()
	if !r.work.TryLock() {
		fail(c, 409, "job_busy")
		return
	}
	defer r.work.Unlock()
	path := filepath.Join(r.dir, "retired.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		c.JSON(200, gin.H{"deleted": 0})
		return
	}
	var entries []retiredObject
	if err != nil || json.Unmarshal(data, &entries) != nil {
		fail(c, 500, "internal_error")
		return
	}
	remaining := []retiredObject{}
	deleted := 0
	for _, entry := range entries {
		var count int64
		a.DB.Model(&StoredObject{}).Where("object_key=? AND store_id=?", entry.Object.Key, entry.Object.StoreID).Count(&count)
		if count > 0 || entry.Database != a.currentDatabase() {
			remaining = append(remaining, entry)
			continue
		}
		var current StoredObject
		if a.DB.First(&current, "object_key=?", entry.Object.Key).Error == nil {
			currentStore, ce := a.store(current.StoreID)
			if ce != nil || verifyObject(c.Request.Context(), currentStore, current.Key, current.SHA256, current.Size) != nil {
				remaining = append(remaining, entry)
				continue
			}
			currentProfile, e := a.storageProfile(current.StoreID)
			oldProfile, e2 := a.storageProfile(entry.Object.StoreID)
			if entry.LocalRoot != "" {
				oldProfile = StorageProfile{Kind: "local", LocalRoot: entry.LocalRoot}
				e2 = nil
			}
			if e == nil && e2 == nil && sameStorageLocation(currentProfile, oldProfile) {
				remaining = append(remaining, entry)
				continue
			}
		}
		var store ObjectStore
		if entry.LocalRoot != "" {
			store = localObjectStore{entry.LocalRoot}
		} else {
			store, err = a.store(entry.Object.StoreID)
			if err != nil {
				remaining = append(remaining, entry)
				continue
			}
		}
		if err = verifyObject(c.Request.Context(), store, entry.Object.Key, entry.Object.SHA256, entry.Object.Size); err != nil {
			remaining = append(remaining, entry)
			continue
		}
		if err = store.Delete(c.Request.Context(), entry.Object.Key); err != nil {
			remaining = append(remaining, entry)
			continue
		}
		a.DB.Where("object_key=? AND store_id=?", entry.Object.Key, entry.Object.StoreID).Delete(&StoredObjectReplica{})
		deleted++
	}
	if atomicJSON(path, remaining) != nil {
		fail(c, 500, "internal_error")
		return
	}
	a.audit(actor(c).ID, "storage", 0, "storage_copies_cleaned")
	c.JSON(200, gin.H{"deleted": deleted, "remaining": len(remaining)})
}
func sameStorageLocation(first, second StorageProfile) bool {
	if first.Kind != second.Kind {
		return false
	}
	if first.Kind == "local" {
		normalize := func(root string) string {
			path, _ := filepath.Abs(root)
			if resolved, e := filepath.EvalSymlinks(path); e == nil {
				path = resolved
			}
			return filepath.Clean(path)
		}
		x, y := normalize(first.LocalRoot), normalize(second.LocalRoot)
		if os.PathSeparator == '\\' {
			return strings.EqualFold(x, y)
		}
		return x == y
	}
	return first.Region == second.Region && first.Bucket == second.Bucket && strings.Trim(first.Prefix, "/") == strings.Trim(second.Prefix, "/")
}

// Confirm the SQL snapshot can resolve every media object, including queued
// sources. Outputs not yet produced by transcoding are deliberately excluded.
func validateMediaInventory(db *gorm.DB, objects []StoredObject) error {
	keys := map[string]bool{}
	for _, object := range objects {
		keys[object.Key] = true
	}
	var media []Media
	var metadata []MediaMetadata
	if err := db.Find(&media).Error; err != nil {
		return err
	}
	if err := db.Find(&metadata).Error; err != nil {
		return err
	}
	meta := map[uint64]MediaMetadata{}
	for _, item := range metadata {
		meta[item.MediaID] = item
	}
	for _, item := range media {
		detail := meta[item.ID]
		required := []string{item.Path, item.ThumbPath}
		if detail.Kind == "video" && detail.State != "ready" {
			required = []string{detail.SourcePath}
			if detail.SourcePath == "" {
				return errors.New("object_missing_processing_source")
			}
		}
		for _, key := range required {
			if key != "" && !keys[key] {
				return errors.New("object_missing_media_reference")
			}
		}
	}
	for _, table := range []string{"media_references", "resource_media_references", "home_display_media"} {
		var missing int64
		if err := db.Table(table + " refs").Joins("LEFT JOIN media ON media.id=refs.media_id").Where("media.id IS NULL").Count(&missing).Error; err != nil {
			return err
		}
		if missing > 0 {
			return errors.New("object_missing_media_reference")
		}
	}
	return nil
}
