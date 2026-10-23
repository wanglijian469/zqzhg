package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type LocalRoot struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	RequireMount bool   `json:"require_mount"`
}

func (a *App) localRoots() []LocalRoot {
	root, _ := filepath.Abs(a.Config.MediaDir)
	roots := []LocalRoot{{Key: "primary", Name: "ECS / Local", Path: root, RequireMount: env("MEDIA_REQUIRE_MOUNT", "false") == "true"}}
	var extra []LocalRoot
	if json.Unmarshal([]byte(env("LOCAL_STORAGE_ROOTS_JSON", "[]")), &extra) == nil {
		for _, r := range extra {
			if r.Key != "" && r.Key != "primary" && filepath.IsAbs(r.Path) {
				r.Path = filepath.Clean(r.Path)
				roots = append(roots, r)
			}
		}
	}
	return roots
}
func (a *App) registeredRoot(key, path string) (LocalRoot, error) {
	for _, r := range a.localRoots() {
		if key != "" && r.Key == key || key == "" && path != "" && func() bool { abs, _ := filepath.Abs(path); return filepath.Clean(abs) == r.Path }() {
			return r, nil
		}
	}
	return LocalRoot{}, errors.New("storage_root_not_registered")
}
func (a *App) resolveLocalProfile(p StorageProfile) (StorageProfile, error) {
	if p.Kind != "local" {
		return p, nil
	}
	if p.RootKey != "" {
		r, e := a.registeredRoot(p.RootKey, "")
		if e != nil {
			return p, e
		}
		if r.RequireMount && !storageMountPresent(r.Path) {
			return p, errors.New("storage_mount_missing")
		}
		p.LocalRoot = r.Path
	}
	return p, nil
}
func (a *App) checkLocalWritable(p StorageProfile) error {
	if p.Kind != "local" {
		return nil
	}
	r, e := a.registeredRoot(p.RootKey, p.LocalRoot)
	if e != nil {
		if p.ID > 0 && p.ID == a.defaultStoreID() && p.RootKey == "" {
			return nil
		}
		return e
	}
	if r.RequireMount && !storageMountPresent(r.Path) {
		return errors.New("storage_mount_missing")
	}
	return nil
}

type StoredObjectReplica struct {
	ObjectKey  string `gorm:"primaryKey"`
	StoreID    uint64 `gorm:"primaryKey"`
	LocalRoot  string
	Size       int64
	SHA256     string
	VerifiedAt time.Time
}
type verifiedReplica struct {
	Store    ObjectStore
	Checked  time.Time
	Size     int64
	Modified time.Time
	SHA      string
}

func (a *App) resolveObject(ctx context.Context, key string) (StoredObject, ObjectStore, error) {
	object, e := a.objectRecord(key)
	if e != nil {
		return object, nil, e
	}
	store, e := a.store(object.StoreID)
	if e == nil {
		if info, se := store.Stat(ctx, key); se == nil && info.Size == object.Size && a.verifyCachedObject(ctx, store, object, info, fmt.Sprintf("primary:%d:%s", object.StoreID, key)) == nil {
			return object, store, nil
		}
	}
	a.importLegacyReplicas(ctx, key, object)
	var copies []StoredObjectReplica
	if er := a.DB.Where("object_key=?", key).Order("verified_at DESC").Find(&copies).Error; er != nil {
		return object, nil, er
	}
	for _, copy := range copies {
		if copy.Size != object.Size || copy.SHA256 != object.SHA256 {
			continue
		}
		var old ObjectStore
		var err error
		if copy.LocalRoot != "" {
			old = localObjectStore{copy.LocalRoot}
		} else {
			old, err = a.store(copy.StoreID)
		}
		if err != nil {
			continue
		}
		info, err := old.Stat(ctx, key)
		if err != nil || info.Size != object.Size {
			continue
		}
		cacheKey := fmt.Sprintf("%s:%d:%s", key, copy.StoreID, copy.LocalRoot)
		a.replicaMu.Lock()
		cached, ok := a.replicaCache[cacheKey]
		a.replicaMu.Unlock()
		valid := ok && time.Since(cached.Checked) < 10*time.Minute && cached.Size == info.Size && cached.Modified.Equal(info.Modified) && cached.SHA == object.SHA256
		if !valid {
			if verifyObject(ctx, old, key, object.SHA256, object.Size) != nil {
				continue
			}
			a.replicaMu.Lock()
			if a.replicaCache == nil || len(a.replicaCache) > 10000 {
				a.replicaCache = map[string]verifiedReplica{}
			}
			a.replicaCache[cacheKey] = verifiedReplica{old, time.Now(), info.Size, info.Modified, object.SHA256}
			a.replicaMu.Unlock()
		}
		a.replicaMu.Lock()
		a.storageFallbackReads++
		a.replicaMu.Unlock()
		return object, old, nil
	}
	return object, nil, errors.New("storage_unavailable")
}
func (a *App) registerReplica(tx *gorm.DB, object StoredObject, localRoot string) error {
	return tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&StoredObjectReplica{ObjectKey: object.Key, StoreID: object.StoreID, LocalRoot: localRoot, Size: object.Size, SHA256: object.SHA256, VerifiedAt: time.Now().UTC()}).Error
}

type MerchantStorageQuota struct {
	AccountID   uint64 `gorm:"primaryKey"`
	QuotaBytes  *int64
	Initialized bool
	UpdatedAt   time.Time
}

func (MerchantStorageQuota) TableName() string { return "merchant_storage_quotas" }

type MediaObjectOwner struct {
	ObjectKey string `gorm:"primaryKey"`
	OwnerID   uint64
	Size      int64
}
type MediaUploadReservation struct {
	ID        string `gorm:"primaryKey"`
	OwnerID   uint64
	Bytes     int64
	ExpiresAt time.Time
}

func (a *App) syncOwnerInventory(tx *gorm.DB, owner uint64) error {
	var quota MerchantStorageQuota
	e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&quota, "account_id=?", owner).Error
	if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
		return e
	}
	if quota.Initialized {
		return nil
	}
	var media []Media
	if e := tx.Where("owner_id=?", owner).Find(&media).Error; e != nil {
		return e
	}
	seen := map[string]bool{}
	var metas []MediaMetadata
	if e := tx.Joins("JOIN media ON media.id=media_metadata.media_id").Where("media.owner_id=?", owner).Find(&metas).Error; e != nil {
		return e
	}
	for _, m := range media {
		seen[m.Path] = true
		seen[m.ThumbPath] = true
	}
	if tx.Migrator().HasTable(&ProcurementAttachment{}) {
		var attachments []ProcurementAttachment
		if e := tx.Where("owner_id=?", owner).Find(&attachments).Error; e != nil {
			return e
		}
		for _, attachment := range attachments {
			seen[attachment.ObjectKey] = true
		}
	}
	for _, m := range metas {
		seen[m.SourcePath] = true
		if m.Kind == "video" && m.SourcePath == "" {
			for _, media := range media {
				if media.ID == m.MediaID {
					stem := strings.TrimSuffix(strings.TrimSuffix(media.Path, filepath.Ext(media.Path)), "-ready")
					for _, ext := range []string{".mp4", ".mov"} {
						seen[stem+ext] = true
					}
				}
			}
		}
	}
	for key := range seen {
		if key == "" {
			continue
		}
		var obj StoredObject
		e := tx.First(&obj, "object_key=?", key).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			p, pe := localObjectPath(a.Config.MediaDir, key)
			if pe != nil {
				continue
			}
			sha, size, pe := fileSHA(p)
			if pe != nil {
				continue
			}
			obj = StoredObject{Key: key, StoreID: 0, Size: size, SHA256: sha, UpdatedAt: time.Now().UTC()}
			if e = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&obj).Error; e != nil {
				return e
			}
		} else if e != nil {
			return e
		}
		if e = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&MediaObjectOwner{ObjectKey: key, OwnerID: owner, Size: obj.Size}).Error; e != nil {
			return e
		}
	}
	quota.AccountID = owner
	quota.Initialized = true
	quota.UpdatedAt = time.Now().UTC()
	return tx.Save(&quota).Error
}
func (a *App) ownerUsage(tx *gorm.DB, owner uint64) (int64, int64, int64, error) {
	var used, reserved int64
	if e := tx.Model(&MediaObjectOwner{}).Where("owner_id=?", owner).Select("COALESCE(SUM(size),0)").Scan(&used).Error; e != nil {
		return 0, 0, 0, e
	}
	if e := tx.Model(&MediaUploadReservation{}).Where("owner_id=? AND expires_at>?", owner, time.Now().UTC()).Select("COALESCE(SUM(bytes),0)").Scan(&reserved).Error; e != nil {
		return 0, 0, 0, e
	}
	cfg, e := systemConfigDB(tx)
	if e != nil {
		return 0, 0, 0, e
	}
	limit := cfg.Uploads.DefaultQuotaBytes
	var q MerchantStorageQuota
	if tx.First(&q, "account_id=?", owner).Error == nil && q.QuotaBytes != nil {
		limit = *q.QuotaBytes
	}
	return used, reserved, limit, nil
}

type UploadLease struct {
	app   *App
	ID    string
	Owner uint64
}

func (a *App) reserveUpload(owner uint64, bytes int64) (*UploadLease, error) {
	a.uploadMu.Lock()
	defer a.uploadMu.Unlock()
	if bytes < 0 {
		return nil, errors.New("invalid_request")
	}
	if e := a.checkUploadDisks(uint64(bytes)); e != nil {
		return nil, e
	}
	lease := &UploadLease{app: a, ID: randomToken(), Owner: owner}
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		var acc Account
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&acc, owner).Error; e != nil {
			return e
		}
		if e := a.syncOwnerInventory(tx, owner); e != nil {
			return e
		}
		used, reserved, limit, e := a.ownerUsage(tx, owner)
		if e != nil {
			return e
		}
		if phoneAccount(acc) && used+reserved+bytes > limit {
			return errors.New("storage_quota_exceeded")
		}
		return tx.Create(&MediaUploadReservation{ID: lease.ID, OwnerID: owner, Bytes: bytes, ExpiresAt: time.Now().UTC().Add(2 * time.Hour)}).Error
	})
	return lease, e
}
func (l *UploadLease) Release() {
	if l != nil {
		l.app.DB.Where("id=?", l.ID).Delete(&MediaUploadReservation{})
	}
}
func (l *UploadLease) Commit(keys ...string) error {
	if l == nil {
		return errors.New("invalid_request")
	}
	a := l.app
	return a.DB.Transaction(func(tx *gorm.DB) error {
		var account Account
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&account, l.Owner).Error; e != nil {
			return e
		}
		var extra int64
		objects := []MediaObjectOwner{}
		seen := map[string]bool{}
		for _, key := range keys {
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			var count int64
			tx.Model(&MediaObjectOwner{}).Where("object_key=?", key).Count(&count)
			if count > 0 {
				continue
			}
			var object StoredObject
			if e := tx.First(&object, "object_key=?", key).Error; e != nil {
				return e
			}
			extra += object.Size
			objects = append(objects, MediaObjectOwner{ObjectKey: key, OwnerID: l.Owner, Size: object.Size})
		}
		if e := tx.Where("id=?", l.ID).Delete(&MediaUploadReservation{}).Error; e != nil {
			return e
		}
		used, reserved, limit, e := a.ownerUsage(tx, l.Owner)
		if e != nil {
			return e
		}
		if phoneAccount(account) && used+reserved+extra > limit {
			return errors.New("storage_quota_exceeded")
		}
		for _, obj := range objects {
			if e := tx.Create(&obj).Error; e != nil {
				return e
			}
		}
		return nil
	})
}
func uploadOwner(c *gin.Context) uint64 {
	if owner, ok := c.Get("upload_owner_id"); ok {
		return owner.(uint64)
	}
	return actor(c).ID
}
func (a *App) checkUploadDisks(required uint64) error {
	cfg, configErr := a.systemConfig()
	if configErr != nil {
		return configErr
	}
	var pending int64
	if e := a.DB.Model(&MediaUploadReservation{}).Where("expires_at>?", time.Now().UTC()).Select("COALESCE(SUM(bytes),0)").Scan(&pending).Error; e != nil {
		return e
	}
	required += uint64(pending)
	paths := []string{a.mediaTempDir()}
	p, e := a.storageProfile(a.defaultStoreID())
	if e != nil {
		return e
	}
	if p.Kind == "local" {
		p, e = a.resolveLocalProfile(p)
		if e != nil {
			return e
		}
		if e = a.checkLocalWritable(p); e != nil {
			return e
		}
		paths = append(paths, p.LocalRoot)
	}
	for _, path := range paths {
		if e := os.MkdirAll(path, 0700); e != nil {
			return errors.New("storage_unavailable")
		}
		free, e := diskFreeBytes(path)
		if e != nil {
			return errors.New("storage_unavailable")
		}
		if free < cfg.Disk.MinFreeBytes || free-cfg.Disk.MinFreeBytes < required {
			return errors.New("disk_space_insufficient")
		}
	}
	return nil
}
func (a *App) usageJSON(owner uint64) (gin.H, error) {
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		var acc Account
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&acc, owner).Error; e != nil {
			return e
		}
		return a.syncOwnerInventory(tx, owner)
	})
	if e != nil {
		return nil, e
	}
	used, reserved, limit, e := a.ownerUsage(a.DB, owner)
	return gin.H{"account_id": owner, "used_bytes": used, "reserved_bytes": reserved, "quota_bytes": limit}, e
}
func (a *App) storageStatus(c *gin.Context) {
	var objects int64
	var bytes int64
	a.DB.Model(&StoredObject{}).Count(&objects)
	a.DB.Model(&StoredObject{}).Select("COALESCE(SUM(size),0)").Scan(&bytes)
	paths := a.storagePaths()
	disks := []gin.H{}
	for name, v := range paths {
		path := v.(string)
		available, total, e := diskSpace(path)
		row := gin.H{"name": name, "path": path, "available_bytes": nil, "total_bytes": nil, "status": "unavailable"}
		if e == nil {
			cfg, _ := a.systemConfig()
			percent := 0
			if total > 0 {
				percent = int((total - available) * 100 / total)
			}
			state := "healthy"
			if percent >= cfg.Disk.WarnPercent {
				state = "warning"
			}
			if percent >= cfg.Disk.CriticalPercent || available < cfg.Disk.MinFreeBytes {
				state = "critical"
			}
			row["available_bytes"] = available
			row["total_bytes"] = total
			row["used_percent"] = percent
			row["status"] = state
		}
		disks = append(disks, row)
	}
	a.replicaMu.Lock()
	fallback := a.storageFallbackReads
	a.replicaMu.Unlock()
	var profiles []StorageProfile
	a.DB.Order("id").Find(&profiles)
	primary, _ := a.storageProfile(0)
	profiles = append([]StorageProfile{primary}, profiles...)
	stores := []gin.H{}
	for _, profile := range profiles {
		var amount int64
		a.DB.Model(&StoredObject{}).Where("store_id=?", profile.ID).Select("COALESCE(SUM(size),0)").Scan(&amount)
		row := gin.H{"id": profile.ID, "name": profile.Name, "kind": profile.Kind, "registered_bytes": amount, "available_bytes": nil, "total_bytes": nil}
		if profile.Kind == "local" {
			if resolved, e := a.resolveLocalProfile(profile); e == nil {
				if free, total, e := diskSpace(resolved.LocalRoot); e == nil {
					row["available_bytes"] = free
					row["total_bytes"] = total
				}
			}
		}
		stores = append(stores, row)
	}
	c.JSON(200, gin.H{"stores": stores, "paths": paths, "disks": disks, "registered_objects": objects, "registered_bytes": bytes, "fallback_reads": fallback, "active_store_id": a.defaultStoreID(), "roots": a.localRoots(), "backup_policy": a.backupPolicy()})
}
func (a *App) storageUsageRoutes(admin, merchant *gin.RouterGroup) {
	admin.GET("/storage/status", a.storageStatus)
	merchant.GET("/storage/usage", func(c *gin.Context) {
		value, e := a.usageJSON(actor(c).ID)
		if e != nil {
			dbFail(c, e)
			return
		}
		c.JSON(200, value)
	})
	admin.GET("/storage/quotas", func(c *gin.Context) {
		p, n := page(c)
		var accounts []Account
		var total int64
		q := a.DB.Model(&Account{}).Where("role='merchant'")
		q.Count(&total)
		q.Order("id").Offset((p - 1) * n).Limit(n).Find(&accounts)
		rows := []gin.H{}
		for _, acc := range accounts {
			v, e := a.usageJSON(acc.ID)
			if e != nil {
				dbFail(c, e)
				return
			}
			v["username"] = acc.Username
			rows = append(rows, v)
		}
		c.JSON(200, gin.H{"items": rows, "total": total, "page": p, "page_size": n})
	})
	admin.PUT("/storage/quotas/:id", func(c *gin.Context) {
		var req struct {
			QuotaBytes *int64 `json:"quota_bytes"`
		}
		if c.ShouldBindJSON(&req) != nil || req.QuotaBytes != nil && (*req.QuotaBytes < 0 || *req.QuotaBytes > 1<<50) {
			fail(c, 400, "invalid_request")
			return
		}
		e := a.DB.Transaction(func(tx *gorm.DB) error {
			var acc Account
			if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("role='merchant'").First(&acc, id(c)).Error; e != nil {
				return e
			}
			if e := a.syncOwnerInventory(tx, acc.ID); e != nil {
				return e
			}
			if e := tx.Model(&MerchantStorageQuota{}).Where("account_id=?", acc.ID).Updates(map[string]any{"quota_bytes": req.QuotaBytes, "updated_at": time.Now().UTC()}).Error; e != nil {
				return e
			}
			return recordOperation(tx, actor(c).ID, "account", acc.ID, "storage_quota_saved", func() string {
				if req.QuotaBytes == nil {
					return "default"
				}
				return fmt.Sprint(*req.QuotaBytes)
			}())
		})
		if e != nil {
			dbFail(c, e)
			return
		}
		v, e := a.usageJSON(id(c))
		if e != nil {
			dbFail(c, e)
			return
		}
		c.JSON(200, v)
	})
	admin.DELETE("/media/:id", a.deleteUnusedMedia)
}
func (a *App) deleteUnusedMedia(c *gin.Context) {
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
	var media Media
	if a.DB.First(&media, id(c)).Error != nil {
		fail(c, 404, "not_found")
		return
	}
	var dependents int64
	if e := a.DB.Model(&MediaWatermark{}).Where("source_media_id=?", media.ID).Count(&dependents).Error; e != nil {
		dbFail(c, e)
		return
	}
	if dependents > 0 {
		fail(c, 409, "media_in_use")
		return
	}
	var refs int64
	for _, table := range []string{"media_references", "resource_media_references", "home_display_media"} {
		var n int64
		if a.DB.Table(table).Where("media_id=?", media.ID).Count(&n).Error != nil {
			fail(c, 500, "internal_error")
			return
		}
		refs += n
	}
	var categoryRefs int64
	if e := a.DB.Model(&Category{}).Where("image_media_id=?", media.ID).Count(&categoryRefs).Error; e != nil {
		dbFail(c, e)
		return
	}
	refs += categoryRefs
	if refs > 0 {
		fail(c, 409, "media_in_use")
		return
	}
	var meta MediaMetadata
	a.DB.First(&meta, "media_id=?", media.ID)
	if meta.State == "queued" || meta.State == "processing" {
		fail(c, 409, "media_in_use")
		return
	}
	keys := []string{media.Path, media.ThumbPath, meta.SourcePath}
	for _, key := range keys {
		if key == "" {
			continue
		}
		var other int64
		if e := a.DB.Model(&Media{}).Where("id<>? AND (path=? OR thumb_path=?)", media.ID, key, key).Count(&other).Error; e != nil {
			dbFail(c, e)
			return
		}
		var sourceRefs int64
		if e := a.DB.Model(&MediaMetadata{}).Where("media_id<>? AND source_path=?", media.ID, key).Count(&sourceRefs).Error; e != nil {
			dbFail(c, e)
			return
		}
		other += sourceRefs
		if other > 0 {
			fail(c, 409, "media_in_use")
			return
		}
	}
	for _, key := range keys {
		if key == "" {
			continue
		}
		var object StoredObject
		if a.DB.First(&object, "object_key=?", key).Error != nil {
			var e error
			object, e = a.objectRecord(key)
			if e != nil {
				continue
			}
		}
		store, e := a.store(object.StoreID)
		if e != nil {
			fail(c, 503, "storage_unavailable")
			return
		}
		if e = store.Delete(c.Request.Context(), key); e != nil && !os.IsNotExist(e) {
			fail(c, 503, "storage_unavailable")
			return
		}
		a.DB.Where("object_key=?", key).Delete(&StoredObject{})
		a.DB.Where("object_key=?", key).Delete(&MediaObjectOwner{})
		a.DB.Where("object_key=?", key).Delete(&StoredObjectReplica{})
	}
	a.DB.Where("media_id=?", media.ID).Delete(&MediaWatermark{})
	a.DB.Where("media_id=?", media.ID).Delete(&MediaMetadata{})
	a.DB.Delete(&media)
	a.audit(actor(c).ID, "media", media.ID, "unused_media_deleted")
	c.JSON(200, gin.H{"ok": true})
}
func (a *App) cleanupNewObjects(keys ...string) {
	for _, key := range keys {
		if key == "" {
			continue
		}
		var obj StoredObject
		if a.DB.First(&obj, "object_key=?", key).Error != nil {
			continue
		}
		if s, e := a.store(obj.StoreID); e == nil {
			if e = s.Delete(context.Background(), key); e == nil {
				a.DB.Where("object_key=?", key).Delete(&StoredObject{})
				a.DB.Where("object_key=?", key).Delete(&MediaObjectOwner{})
			}
		}
	}
}

func validateRuntimePaths(a *App) error {
	paths := []string{a.Config.MediaDir, a.mediaTempDir(), a.localBackupDir(), a.operations().dir}
	web, _ := filepath.Abs(a.Config.WebDir)
	exe, _ := os.Executable()
	release := filepath.Dir(filepath.Dir(exe))
	for _, p := range paths {
		abs, e := filepath.Abs(p)
		if e != nil {
			return e
		}
		if a.Config.Env == "production" {
			if !filepath.IsAbs(p) {
				return errors.New("production storage paths must be absolute")
			}
			for _, root := range []string{web, release} {
				rel, e := filepath.Rel(root, abs)
				if e == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
					return errors.New("storage must be outside deployment and web directories")
				}
			}
		}
		if e = os.MkdirAll(abs, 0700); e != nil {
			return e
		}
	}
	return nil
}
func (a *App) verifyCachedObject(ctx context.Context, store ObjectStore, object StoredObject, info ObjectInfo, key string) error {
	a.replicaMu.Lock()
	cached, ok := a.replicaCache[key]
	a.replicaMu.Unlock()
	if ok && time.Since(cached.Checked) < 10*time.Minute && cached.Size == info.Size && cached.Modified.Equal(info.Modified) && cached.SHA == object.SHA256 {
		return nil
	}
	if e := verifyObject(ctx, store, object.Key, object.SHA256, object.Size); e != nil {
		return e
	}
	a.replicaMu.Lock()
	defer a.replicaMu.Unlock()
	if a.replicaCache == nil || len(a.replicaCache) > 10000 {
		a.replicaCache = map[string]verifiedReplica{}
	}
	a.replicaCache[key] = verifiedReplica{store, time.Now(), info.Size, info.Modified, object.SHA256}
	return nil
}
func (a *App) importLegacyReplicas(ctx context.Context, key string, expected StoredObject) {
	raw, e := os.ReadFile(filepath.Join(a.operations().dir, "retired.json"))
	if e != nil {
		return
	}
	var old []retiredObject
	if json.Unmarshal(raw, &old) != nil {
		return
	}
	db := a.currentDatabase()
	for _, entry := range old {
		if entry.Database != db || entry.Object.Key != key || entry.Object.SHA256 != expected.SHA256 || entry.Object.Size != expected.Size {
			continue
		}
		var n int64
		a.DB.Model(&StoredObjectReplica{}).Where("object_key=? AND store_id=?", key, entry.Object.StoreID).Count(&n)
		if n > 0 {
			continue
		}
		var store ObjectStore
		if entry.LocalRoot != "" {
			store = localObjectStore{entry.LocalRoot}
		} else {
			store, e = a.store(entry.Object.StoreID)
			if e != nil {
				continue
			}
		}
		if verifyObject(ctx, store, key, expected.SHA256, expected.Size) == nil {
			a.registerReplica(a.DB, entry.Object, entry.LocalRoot)
		}
	}
}
