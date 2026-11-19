package main

import (
	"context"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

// A stale approved revision may leave a copied but unregistered target after a
// crash. Only targets from cancelled publication jobs are eligible; primary
// business objects and historical local replicas are never handled here.
func (a *App) reclaimCancelledFileCopy(ctx context.Context) bool {
	var item FileStorageJobItem
	if a.DB.Table("file_storage_job_items i").Select("i.*").Joins("JOIN file_storage_jobs j ON j.id=i.job_id").Where("j.job_kind='publish' AND j.status='cancelled' AND i.reservation_id<>'' AND i.status<>'reclaimed'").Order("i.id").First(&item).Error != nil {
		return false
	}
	var registered int64
	if a.DB.Model(&StoredObject{}).Where("object_key=?", item.TargetKey).Count(&registered).Error != nil || registered > 0 || a.objectPinned(a.DB, item.TargetKey) {
		return false
	}
	if a.DB.Model(&EnterpriseFileObject{}).Where("object_key=? AND object_state='active'", item.TargetKey).Count(&registered).Error != nil || registered > 0 {
		return false
	}
	target, e := a.store(item.TargetStoreID)
	if e != nil {
		return false
	}
	if e = target.Delete(ctx, item.TargetKey); e != nil {
		return false
	}
	if _, e = target.Stat(ctx, item.TargetKey); e == nil || !objectMissing(e) {
		return false
	}
	return a.DB.Transaction(func(tx *gorm.DB) error {
		var current FileStorageJobItem
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, item.ID).Error; e != nil {
			return e
		}
		if e := tx.Where("id=? AND hold_until_reclaimed=1", current.ReservationID).Delete(&MediaUploadReservation{}).Error; e != nil {
			return e
		}
		return tx.Model(&current).Updates(map[string]any{"status": "reclaimed", "reservation_id": "", "updated_at": time.Now().UTC()}).Error
	}) == nil
}
