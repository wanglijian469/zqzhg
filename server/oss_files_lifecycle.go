package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// This decision is committed with platform approval, before any public object is written.
func (a *App) queueFilePublication(tx *gorm.DB, kind string, id, revision, epoch, owner, vendor uint64, ids []uint64) (bool, error) {
	if !cloudFilesEnabled() {
		return false, nil
	}
	needed := false
	for _, mid := range ids {
		var m Media
		if tx.First(&m, mid).Error != nil {
			return false, errors.New("media_not_ready")
		}
		for _, key := range []string{m.Path, m.ThumbPath} {
			if key == "" {
				continue
			}
			var o StoredObject
			if tx.First(&o, "object_key=?", key).Error != nil {
				continue
			}
			var p StorageProfile
			if o.StoreID > 0 && tx.First(&p, o.StoreID).Error == nil && p.AuthMode == "ecs_role" {
				needed = true
			}
		}
	}
	if !needed {
		return false, nil
	}
	if !a.cloudReadyDB(tx, true) {
		return false, errors.New("oss_public_unverified")
	}
	var prior FileStorageJob
	if tx.Where("job_kind='publish' AND resource_kind=? AND resource_id=? AND revision_id=? AND status NOT IN ('cancelled')", kind, id, revision).First(&prior).Error == nil {
		return true, nil
	}
	b, _ := json.Marshal(ids)
	now := time.Now().UTC()
	j := FileStorageJob{ID: randomToken(), JobKind: "publish", OwnerID: owner, EnterpriseID: vendor, ResourceKind: kind, ResourceID: id, RevisionID: revision, ExpectedEpoch: epoch, Status: "queued", Checkpoint: string(b), CreatedAt: now, UpdatedAt: now}
	return true, tx.Create(&j).Error
}
func (a *App) publicationCurrent(j FileStorageJob) bool {
	if j.ResourceKind == "content" {
		var c Content
		var r Revision
		if a.DB.First(&c, j.ResourceID).Error != nil || a.DB.First(&r, j.RevisionID).Error != nil {
			return false
		}
		return c.DraftRevisionID == j.RevisionID && c.Epoch == j.ExpectedEpoch && r.Status == "approved"
	}
	if j.ResourceKind == "resource" {
		var c Resource
		var r ResourceRevision
		if a.DB.First(&c, j.ResourceID).Error != nil || a.DB.First(&r, j.RevisionID).Error != nil {
			return false
		}
		return c.DraftRevisionID == j.RevisionID && c.Epoch == j.ExpectedEpoch && r.Status == "approved"
	}
	return false
}
func (a *App) runNextStorageFileJob(ctx context.Context) bool {
	return a.withCloudWorkerLock(ctx, a.runNextStorageFileJobLocked)
}
func (a *App) runNextStorageFileJobLocked(ctx context.Context) bool {
	var j FileStorageJob
	now := time.Now().UTC()
	token := randomToken()
	until := now.Add(2 * time.Minute)
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("status IN ('queued','processing') AND (lease_until IS NULL OR lease_until<?)", now).Order("created_at").First(&j).Error; e != nil {
			return e
		}
		return tx.Model(&j).Updates(map[string]any{"status": "processing", "lease_token": token, "lease_until": until, "updated_at": now}).Error
	})
	if e != nil {
		a.reclaimPublicFile(ctx)
		a.reclaimCancelledFileCopy(ctx)
		return false
	}
	j.LeaseToken = token
	ctx, stop := a.cloudLeaseContext(ctx, "job", j.ID, token)
	defer stop()
	if j.JobKind == "publish" {
		e = a.executeFilePublication(ctx, j)
	} else {
		e = a.executeFileMigration(ctx, j)
	}
	state := "completed"
	code := ""
	if e != nil {
		state = "failed"
		code = cloudProcessingError(e)
		var item FileStorageJobItem
		if a.DB.Where("job_id=? AND status NOT IN ('completed','restored')", j.ID).Order("id").First(&item).Error == nil {
			a.DB.Model(&item).Updates(map[string]any{"status": "failed", "error": code, "updated_at": time.Now().UTC()})
		}
		if errors.Is(e, errFilePublicationStale) {
			state = "cancelled"
			code = "stale_revision"
		}
	}
	a.DB.Model(&FileStorageJob{}).Where("id=? AND lease_token=?", j.ID, token).Updates(map[string]any{"status": state, "error": code, "lease_token": "", "lease_until": nil, "updated_at": time.Now().UTC()})
	return true
}

var errFilePublicationStale = errors.New("file_publication_stale")

func (a *App) executeFilePublication(ctx context.Context, j FileStorageJob) error {
	if !a.publicationCurrent(j) {
		return errFilePublicationStale
	}
	var ids []uint64
	if json.Unmarshal([]byte(j.Checkpoint), &ids) != nil {
		return errors.New("invalid_request")
	}
	for _, mid := range ids {
		if !a.publicationCurrent(j) {
			return errFilePublicationStale
		}
		var m Media
		if e := a.DB.First(&m, mid).Error; e != nil {
			return e
		}
		if m.OwnerID != j.OwnerID {
			return errors.New("not_found")
		}
		if alias, e := a.publicWatermarkAlias(m.ID); e != nil {
			return e
		} else if alias.ID > 0 {
			m = alias
		}
		if blocked, e := a.watermarkSourceIsPublicDisplay(m.ID); e != nil {
			return e
		} else if blocked {
			return errors.New("watermark_not_ready")
		}
		var meta MediaMetadata
		if a.DB.First(&meta, "media_id=?", m.ID).Error == nil && meta.State != "ready" {
			return errors.New("media_not_ready")
		}
		for _, key := range []string{m.Path, m.ThumbPath} {
			if key == "" {
				continue
			}
			var source StoredObject
			if a.DB.First(&source, "object_key=?", key).Error != nil {
				continue
			}
			var profile StorageProfile
			if a.DB.First(&profile, source.StoreID).Error != nil || profile.AuthMode != "ecs_role" {
				continue
			}
			if e := a.publishFileObject(ctx, j, m, source); e != nil {
				return e
			}
		}
	}
	return a.DB.Transaction(func(tx *gorm.DB) error {
		var owned FileStorageJob
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&owned, "id=? AND lease_token=?", j.ID, j.LeaseToken).Error; e != nil {
			return e
		}
		if j.ResourceKind == "content" {
			var c Content
			if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&c, j.ResourceID).Error; e != nil {
				return e
			}
			if c.Epoch != j.ExpectedEpoch || c.DraftRevisionID != j.RevisionID {
				return errFilePublicationStale
			}
			var r Revision
			if tx.First(&r, j.RevisionID).Error != nil || r.Status != "approved" {
				return errFilePublicationStale
			}
			return tx.Model(&c).Updates(map[string]any{"published_revision_id": j.RevisionID, "visible": true, "epoch": c.Epoch + 1}).Error
		}
		var r Resource
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&r, j.ResourceID).Error; e != nil {
			return e
		}
		if r.Epoch != j.ExpectedEpoch || r.DraftRevisionID != j.RevisionID {
			return errFilePublicationStale
		}
		var rev ResourceRevision
		if tx.First(&rev, j.RevisionID).Error != nil || rev.Status != "approved" {
			return errFilePublicationStale
		}
		return tx.Model(&r).Updates(map[string]any{"published_revision_id": j.RevisionID, "visible": true, "epoch": r.Epoch + 1}).Error
	})
}
func (a *App) reserveFileCopy(item *FileStorageJobItem, owner uint64) error {
	return a.DB.Transaction(func(tx *gorm.DB) error {
		var acc Account
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&acc, owner).Error; e != nil {
			return e
		}
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(item, item.ID).Error; e != nil {
			return e
		}
		if item.ReservationID != "" {
			var n int64
			tx.Model(&MediaUploadReservation{}).Where("id=?", item.ReservationID).Count(&n)
			if n > 0 {
				return nil
			}
		}
		used, reserved, limit, e := a.ownerUsage(tx, owner)
		if e != nil {
			return e
		}
		if phoneAccount(acc) && used+reserved+item.FileSize > limit {
			return errors.New("storage_quota_exceeded")
		}
		item.ReservationID = randomToken()
		if e = tx.Create(&MediaUploadReservation{ID: item.ReservationID, OwnerID: owner, Bytes: item.FileSize, ExpiresAt: time.Now().Add(2 * time.Hour), HoldUntilReclaimed: true}).Error; e != nil {
			return e
		}
		return tx.Model(item).Update("reservation_id", item.ReservationID).Error
	})
}
func (a *App) publishFileObject(ctx context.Context, j FileStorageJob, m Media, source StoredObject) error {
	var known EnterpriseFileObject
	if e := a.DB.Where("source_key=? AND object_role='public' AND object_state='active'", source.Key).First(&known).Error; e == nil {
		store, e := a.objectStore(StoredObject{Key: known.ObjectKey, RemoteKey: known.PhysicalKey, StoreID: known.StoreID})
		if e != nil {
			return e
		}
		if e = verifyObject(ctx, store, known.ObjectKey, source.SHA256, source.Size); e != nil {
			return errors.New("checksum_mismatch")
		}
		return nil
	} else if !errors.Is(e, gorm.ErrRecordNotFound) {
		return e
	}
	p, e := a.cloudProfile("public")
	if e != nil {
		return e
	}
	var item FileStorageJobItem
	if a.DB.Where("job_id=? AND source_key=? AND target_store_id=? AND status<>'restored'", j.ID, source.Key, p.ID).First(&item).Error != nil {
		key := fmt.Sprintf("companies/%d/published/%s%s", j.EnterpriseID, randomToken(), filepath.Ext(source.Key))
		if j.EnterpriseID == 0 {
			key = "platform/published/" + randomToken() + filepath.Ext(source.Key)
		}
		item = FileStorageJobItem{JobID: j.ID, SourceKey: source.Key, TargetKey: key, SourceStoreID: source.StoreID, TargetStoreID: p.ID, MediaID: m.ID, FileSize: source.Size, SHA256: source.SHA256, Status: "queued", UpdatedAt: time.Now().UTC()}
		if e = a.DB.Create(&item).Error; e != nil {
			return e
		}
	}
	// A completed checkpoint still needs a verified active mapping. If metadata
	// was rolled back, reconstruct it from the immutable target below.
	if e = a.reserveFileCopy(&item, j.OwnerID); e != nil {
		return e
	}
	_, src, e := a.resolveObject(ctx, source.Key)
	if e != nil {
		return e
	}
	dst, e := a.store(p.ID)
	if e != nil {
		return e
	}
	if _, e = dst.Stat(ctx, item.TargetKey); e != nil {
		var service *oss.ServiceError
		if !errors.Is(e, os.ErrNotExist) && (!errors.As(e, &service) || service.StatusCode != 404) {
			return cloudError(e)
		}
		if from, ok := src.(*ossObjectStore); ok {
			to := dst.(*ossObjectStore)
			_, e = to.Client.CopyObject(ctx, &oss.CopyObjectRequest{Bucket: oss.Ptr(to.Bucket), Key: oss.Ptr(to.key(item.TargetKey)), SourceBucket: oss.Ptr(from.Bucket), SourceKey: oss.Ptr(from.key(source.Key)), ForbidOverwrite: oss.Ptr("true")})
		} else {
			var body interface {
				Read([]byte) (int, error)
				Close() error
			}
			body, e = src.Open(ctx, source.Key)
			if e == nil {
				e = dst.Put(ctx, item.TargetKey, body, source.Size)
				body.Close()
			}
		}
		if e != nil {
			return cloudError(e)
		}
	}
	if e = verifyObject(ctx, dst, item.TargetKey, source.SHA256, source.Size); e != nil {
		return errors.New("checksum_mismatch")
	}
	return a.DB.Transaction(func(tx *gorm.DB) error {
		var acc Account
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&acc, j.OwnerID).Error; e != nil {
			return e
		}
		now := time.Now().UTC()
		o := StoredObject{Key: item.TargetKey, StoreID: p.ID, Size: source.Size, SHA256: source.SHA256, UpdatedAt: now}
		if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&o).Error; e != nil {
			return e
		}
		if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&MediaObjectOwner{ObjectKey: item.TargetKey, OwnerID: j.OwnerID, Size: source.Size}).Error; e != nil {
			return e
		}
		row := EnterpriseFileObject{ObjectKey: item.TargetKey, PhysicalKey: item.TargetKey, MediaID: m.ID, OwnerID: j.OwnerID, EnterpriseID: j.EnterpriseID, StoreID: p.ID, Bucket: p.Bucket, SourceKey: source.Key, ObjectRole: "public", BusinessType: "published", FileSize: source.Size, SHA256: source.SHA256, ObjectState: "active", CreatedAt: now, UpdatedAt: now}
		if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; e != nil {
			return e
		}
		if e := tx.Where("id=?", item.ReservationID).Delete(&MediaUploadReservation{}).Error; e != nil {
			return e
		}
		if e := tx.Model(&EnterpriseFile{}).Where("media_id=?", m.ID).Updates(map[string]any{"visibility": "public", "file_status": "published", "updated_at": now}).Error; e != nil {
			return e
		}
		return tx.Model(&item).Updates(map[string]any{"status": "completed", "updated_at": now}).Error
	})
}
func (a *App) cloudPublicAddress(source string) (string, bool) {
	if !a.cloudReady(true) {
		return "", false
	}
	var o EnterpriseFileObject
	if a.DB.Where("source_key=? AND object_role='public' AND object_state='active'", source).First(&o).Error != nil {
		return "", false
	}
	p, e := a.storageProfile(o.StoreID)
	if e != nil {
		return "", false
	}
	physical := o.PhysicalKey
	if physical == "" {
		physical = o.ObjectKey
	}
	u, e := a.publicObjectURL(p, physical)
	return u, e == nil
}
func (a *App) mediaCurrentlyPublic(mid uint64) bool {
	yes, e := a.mediaPublicState(mid)
	return e == nil && yes
}
func (a *App) mediaPublicState(mid uint64) (bool, error) {
	var n int64
	if e := a.DB.Raw("SELECT COUNT(*) FROM media_references mr JOIN contents c ON c.published_revision_id=mr.revision_id JOIN revisions r ON r.id=mr.revision_id AND r.status='approved' JOIN halls h ON h.id=c.hall_id AND h.status='open' LEFT JOIN contents v ON v.id=c.vendor_id LEFT JOIN revisions vr ON vr.id=v.published_revision_id WHERE mr.media_id=? AND c.visible=1 AND (c.kind='vendor' OR (v.visible=1 AND vr.status='approved'))", mid).Scan(&n).Error; e != nil {
		return false, e
	}
	if n > 0 {
		return true, nil
	}
	if yes, e := a.resourceMediaIsPublic(mid); e != nil {
		return false, e
	} else if yes {
		return true, nil
	}
	if e := a.DB.Table("home_display_settings hs").Joins("JOIN home_display_media hm ON hm.media_id=hs.hero_media_id").Where("hm.media_id=? AND EXISTS(SELECT 1 FROM halls h WHERE h.id=hs.id AND h.status IN ('open','preparing'))", mid).Count(&n).Error; e != nil {
		return false, e
	}
	if n > 0 {
		return true, nil
	}
	var cats []Category
	if e := a.DB.Where("image_media_id=? AND deleted_at IS NULL", mid).Find(&cats).Error; e != nil {
		return false, e
	}
	for _, cat := range cats {
		if selectableCategory(a.DB, cat.ID, cat.CategoryKind, cat.HallID, 0, false) {
			var h Hall
			if e := a.DB.First(&h, cat.HallID).Error; e != nil {
				return false, e
			}
			if h.Status == "open" {
				return true, nil
			}
		}
	}
	return false, nil
}
func (a *App) objectHasPublicUse(source string) (bool, error) {
	var rows []Media
	if e := a.DB.Where("path=? OR thumb_path=?", source, source).Find(&rows).Error; e != nil {
		return false, e
	}
	for _, m := range rows {
		yes, e := a.mediaPublicState(m.ID)
		if e != nil {
			return false, e
		}
		if yes {
			return true, nil
		}
	}
	return false, nil
}
func (a *App) reclaimPublicFile(ctx context.Context) bool {
	var rows []EnterpriseFileObject
	runtime := a.cloudFiles()
	runtime.mu.Lock()
	after := runtime.gcAfter
	runtime.mu.Unlock()
	if a.DB.Where("object_role='public' AND object_state='active' AND object_key>?", after).Order("object_key").Limit(100).Find(&rows).Error != nil {
		return false
	}
	runtime.mu.Lock()
	if len(rows) == 0 {
		runtime.gcAfter = ""
	} else {
		runtime.gcAfter = rows[len(rows)-1].ObjectKey
	}
	runtime.mu.Unlock()
	for _, o := range rows {
		inUse, err := a.objectHasPublicUse(o.SourceKey)
		if err != nil || inUse || a.objectPinned(a.DB, o.ObjectKey) {
			continue
		}
		var pending int64
		if e := a.DB.Table("file_storage_job_items i").Joins("JOIN file_storage_jobs j ON j.id=i.job_id").Where("i.target_key=? AND j.status IN ('queued','processing','failed')", o.ObjectKey).Count(&pending).Error; e != nil {
			continue
		}
		if pending > 0 {
			continue
		}
		store, e := a.objectStore(StoredObject{Key: o.ObjectKey, StoreID: o.StoreID, RemoteKey: o.PhysicalKey})
		if e != nil {
			continue
		}
		if e = store.Delete(ctx, o.ObjectKey); e != nil {
			continue
		}
		if _, e = store.Stat(ctx, o.ObjectKey); e == nil {
			continue
		} else {
			var se *oss.ServiceError
			if !errors.As(e, &se) || se.StatusCode != 404 {
				continue
			}
		}
		return a.DB.Transaction(func(tx *gorm.DB) error {
			if e := tx.Where("object_key=?", o.ObjectKey).Delete(&StoredObject{}).Error; e != nil {
				return e
			}
			if e := tx.Where("object_key=?", o.ObjectKey).Delete(&MediaObjectOwner{}).Error; e != nil {
				return e
			}
			if e := tx.Model(&o).Updates(map[string]any{"object_state": "deleted", "updated_at": time.Now().UTC()}).Error; e != nil {
				return e
			}
			var remaining int64
			if e := tx.Model(&EnterpriseFileObject{}).Where("media_id=? AND object_role='public' AND object_state='active'", o.MediaID).Count(&remaining).Error; e != nil {
				return e
			}
			if remaining == 0 {
				return tx.Model(&EnterpriseFile{}).Where("media_id=?", o.MediaID).Updates(map[string]any{"visibility": "private", "file_status": "ready", "version": gorm.Expr("version+1"), "updated_at": time.Now().UTC()}).Error
			}
			return nil
		}) == nil
	}
	return false
}
