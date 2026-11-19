package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Backfill is metadata only: it never opens, moves, publishes or deletes a file.
func backfillEnterpriseFiles(db *gorm.DB) error {
	a := &App{DB: db}
	var cursor uint64
	for {
		var rows []Media
		if e := db.Where("id>?", cursor).Order("id").Limit(200).Find(&rows).Error; e != nil {
			return e
		}
		if len(rows) == 0 {
			return nil
		}
		for _, m := range rows {
			var meta MediaMetadata
			db.First(&meta, "media_id=?", m.ID)
			kind := meta.Kind
			if kind == "" {
				kind = "image"
			}
			if e := db.Transaction(func(tx *gorm.DB) error { return a.registerEnterpriseFile(tx, m, kind, "", "", 0) }); e != nil {
				return e
			}
			cursor = m.ID
		}
	}
}

// Only a deliberate operator request creates a migration. Old files are retained
// as verified replicas, and logical media paths never change.
func (a *App) queueEnterpriseMigration(c *gin.Context) {
	var req struct {
		EnterpriseID uint64 `json:"enterprise_id"`
		Confirm      bool   `json:"confirm"`
		DryRun       bool   `json:"dry_run"`
	}
	if c.ShouldBindJSON(&req) != nil || req.EnterpriseID == 0 {
		fail(c, 400, "invalid_request")
		return
	}
	var vendor Content
	if a.DB.Where("id=? AND kind='vendor'", req.EnterpriseID).First(&vendor).Error != nil {
		fail(c, 404, "not_found")
		return
	}
	objects, queryError := localObjectsForOwner(a.DB, vendor.OwnerID)
	if queryError != nil {
		cloudFail(c, queryError)
		return
	}
	var bytes int64
	for _, o := range objects {
		bytes += o.Size
	}
	if req.DryRun {
		c.JSON(200, gin.H{"objects": len(objects), "bytes": bytes, "enterprise_id": vendor.ID, "source_retained": true})
		return
	}
	if !req.Confirm {
		fail(c, 400, "confirmation_required")
		return
	}
	if !a.cloudReady(true) {
		cloudFail(c, errors.New("oss_public_unverified"))
		return
	}
	p, e := a.cloudProfile("private")
	if e != nil {
		cloudFail(c, e)
		return
	}
	var j FileStorageJob
	e = a.DB.Transaction(func(tx *gorm.DB) error {
		var acc Account
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&acc, vendor.OwnerID).Error; e != nil {
			return e
		}
		var n int64
		if e := tx.Model(&FileStorageJob{}).Where("enterprise_id=? AND job_kind='migrate' AND status IN ('queued','processing','failed')", vendor.ID).Count(&n).Error; e != nil {
			return e
		}
		if n > 0 {
			return errors.New("job_busy")
		}
		now := time.Now().UTC()
		j = FileStorageJob{ID: randomToken(), JobKind: "migrate", EnterpriseID: vendor.ID, OwnerID: vendor.OwnerID, Status: "queued", Checkpoint: "{}", CreatedAt: now, UpdatedAt: now}
		if e := tx.Create(&j).Error; e != nil {
			return e
		}
		for _, o := range objects {
			var m Media
			tx.Where("owner_id=? AND (path=? OR thumb_path=?)", vendor.OwnerID, o.Key, o.Key).Order("id").First(&m)
			if m.ID == 0 {
				var meta MediaMetadata
				tx.Where("source_path=?", o.Key).First(&meta)
				m.ID = meta.MediaID
			}
			item := FileStorageJobItem{JobID: j.ID, SourceKey: o.Key, TargetKey: fmt.Sprintf("companies/%d/drafts/%s%s", vendor.ID, randomToken(), filepath.Ext(o.Key)), SourceStoreID: 0, TargetStoreID: p.ID, MediaID: m.ID, FileSize: o.Size, SHA256: o.SHA256, Status: "queued", UpdatedAt: now}
			if e := tx.Create(&item).Error; e != nil {
				return e
			}
		}
		return recordOperation(tx, actor(c).ID, "storage", vendor.ID, "oss_migration_requested", j.ID)
	})
	if e != nil {
		cloudFail(c, e)
		return
	}
	c.JSON(202, j)
}

// Old installations may have different collations on the two object-key
// columns. Resolve owner keys first, then perform bounded indexed lookups using
// parameters; this avoids cross-column collation comparison and any schema edit.
func localObjectsForOwner(db *gorm.DB, owner uint64) ([]StoredObject, error) {
	var keys []string
	if err := db.Model(&MediaObjectOwner{}).Where("owner_id=?", owner).Pluck("object_key", &keys).Error; err != nil {
		return nil, err
	}
	objects := []StoredObject{}
	for start := 0; start < len(keys); start += 200 {
		end := start + 200
		if end > len(keys) {
			end = len(keys)
		}
		var batch []StoredObject
		if err := db.Where("store_id=0 AND object_key IN ?", keys[start:end]).Find(&batch).Error; err != nil {
			return nil, err
		}
		objects = append(objects, batch...)
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].Key < objects[j].Key })
	return objects, nil
}

func (a *App) executeFileMigration(ctx context.Context, j FileStorageJob) error {
	if j.JobKind != "migrate" {
		return errors.New("invalid_request")
	}
	var items []FileStorageJobItem
	if e := a.DB.Where("job_id=? AND status<>'completed'", j.ID).Order("id").Find(&items).Error; e != nil {
		return e
	}
	for _, item := range items {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// The source is read and verified before the target is trusted, including on retry.
		var source StoredObject
		if e := a.DB.First(&source, "object_key=?", item.SourceKey).Error; e != nil {
			return e
		}
		if source.StoreID != 0 && source.RemoteKey != item.TargetKey {
			return errors.New("file_upload_conflict")
		}
		local := localObjectStore{Root: a.Config.MediaDir}
		if e := verifyObject(ctx, local, item.SourceKey, item.SHA256, item.FileSize); e != nil {
			return errors.New("checksum_mismatch")
		}
		dst, e := a.store(item.TargetStoreID)
		if e != nil {
			return e
		}
		if _, e = dst.Stat(ctx, item.TargetKey); e != nil {
			if !objectMissing(e) {
				return cloudError(e)
			}
			body, e := local.Open(ctx, item.SourceKey)
			if e != nil {
				return e
			}
			e = dst.Put(ctx, item.TargetKey, body, item.FileSize)
			body.Close()
			if e != nil {
				return cloudError(e)
			}
		}
		if e = verifyObject(ctx, dst, item.TargetKey, item.SHA256, item.FileSize); e != nil {
			return errors.New("checksum_mismatch")
		}
		e = a.DB.Transaction(func(tx *gorm.DB) error {
			var acc Account
			if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&acc, j.OwnerID).Error; e != nil {
				return e
			}
			var current StoredObject
			if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, "object_key=?", item.SourceKey).Error; e != nil {
				return e
			}
			if current.StoreID != 0 && current.RemoteKey != item.TargetKey {
				return errors.New("file_upload_conflict")
			}
			if a.objectPinned(tx, item.SourceKey) {
				return errors.New("file_backup_pinned")
			}
			if e := a.registerReplica(tx, StoredObject{Key: item.SourceKey, StoreID: 0, Size: item.FileSize, SHA256: item.SHA256}, a.Config.MediaDir); e != nil {
				return e
			}
			if e := tx.Model(&current).Updates(map[string]any{"store_id": item.TargetStoreID, "remote_key": item.TargetKey, "updated_at": time.Now().UTC()}).Error; e != nil {
				return e
			}
			if e := tx.Model(&EnterpriseFileObject{}).Where("object_key=?", item.SourceKey).Updates(map[string]any{"store_id": item.TargetStoreID, "physical_key": item.TargetKey, "bucket": cloudConfig().PrivateBucket}).Error; e != nil {
				return e
			}
			return tx.Model(&item).Updates(map[string]any{"status": "completed", "updated_at": time.Now().UTC()}).Error
		})
		if e != nil {
			return e
		}
	}
	// Already public media receive a separately accounted public copy. No approval
	// or business version is created by the migration itself.
	var done []FileStorageJobItem
	if e := a.DB.Where("job_id=?", j.ID).Find(&done).Error; e != nil {
		return e
	}
	for _, item := range done {
		if item.MediaID == 0 || !a.mediaCurrentlyPublic(item.MediaID) {
			continue
		}
		var m Media
		var o StoredObject
		if a.DB.First(&m, item.MediaID).Error != nil || a.DB.First(&o, "object_key=?", item.SourceKey).Error != nil {
			return errors.New("not_found")
		}
		if blocked, e := a.watermarkSourceIsPublicDisplay(m.ID); e != nil {
			return e
		} else if blocked {
			continue
		}
		if e := a.publishFileObject(ctx, j, m, o); e != nil {
			return e
		}
	}
	return nil
}

func objectMissing(e error) bool {
	if os.IsNotExist(e) {
		return true
	}
	return cloudError(e).Error() == "oss_object_missing"
}

func (a *App) cloudJobRoutes(admin, merchant *gin.RouterGroup) {
	admin.POST("/storage/oss/migrations", a.queueEnterpriseMigration)
	for _, group := range []*gin.RouterGroup{admin.Group("/storage"), merchant} {
		r := group.Group("/file-jobs")
		r.GET("", func(c *gin.Context) {
			p, n := page(c)
			q := a.DB.Model(&FileStorageJob{})
			if actor(c).Role == "merchant" {
				q = q.Where("owner_id=?", actor(c).ID)
			}
			var total int64
			var jobs []FileStorageJob
			if e := q.Count(&total).Error; e != nil {
				cloudFail(c, e)
				return
			}
			if e := q.Order("created_at DESC").Offset((p - 1) * n).Limit(n).Find(&jobs).Error; e != nil {
				cloudFail(c, e)
				return
			}
			c.JSON(200, gin.H{"items": jobs, "total": total})
		})
		r.GET("/:id", func(c *gin.Context) {
			j, e := a.ownedFileJob(c)
			if e != nil {
				cloudFail(c, e)
				return
			}
			var items []FileStorageJobItem
			if e = a.DB.Where("job_id=?", j.ID).Find(&items).Error; e != nil {
				cloudFail(c, e)
				return
			}
			c.JSON(200, gin.H{"job": j, "items": items})
		})
		r.POST("/:id/retry", func(c *gin.Context) {
			j, e := a.ownedFileJob(c)
			if e != nil {
				cloudFail(c, e)
				return
			}
			if j.Status != "failed" {
				cloudFail(c, errors.New("file_upload_conflict"))
				return
			}
			if j.JobKind == "migrate" && actor(c).Role == "merchant" {
				fail(c, 403, "forbidden")
				return
			}
			result := a.DB.Model(&j).Where("status='failed'").Updates(map[string]any{"status": "queued", "error": "", "lease_until": nil, "lease_token": "", "updated_at": time.Now().UTC()})
			if result.Error != nil {
				cloudFail(c, result.Error)
				return
			}
			c.JSON(202, gin.H{"id": j.ID})
		})
	}
}
func (a *App) ownedFileJob(c *gin.Context) (FileStorageJob, error) {
	var j FileStorageJob
	e := a.DB.First(&j, "id=?", c.Param("id")).Error
	if e != nil || (actor(c).Role == "merchant" && actor(c).ID != j.OwnerID) {
		return j, errors.New("not_found")
	}
	return j, nil
}

// JSON lines can be redirected to an operator machine without a local archive.
func (a *App) scanLocalFiles(ctx context.Context) error {
	return filepath.WalkDir(a.Config.MediaDir, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			return nil
		}
		key, e := filepath.Rel(a.Config.MediaDir, path)
		if e != nil {
			return e
		}
		key = filepath.ToSlash(key)
		info, e := d.Info()
		if e != nil {
			return e
		}
		var o StoredObject
		registered := a.DB.First(&o, "object_key=?", key).Error == nil
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"key": key, "bytes": info.Size(), "registered": registered, "symlink": d.Type()&os.ModeSymlink != 0, "primary_store_id": o.StoreID})
	})
}
