package main

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Daily summaries are durable scheduled notification rows. They become visible
// at available_at without a separate queue or a restart-sensitive timer.
// The worker only collects uploads never attached to a submitted draft/message.
func (a *App) startProcurementWorker(ctx context.Context) {
	if !procurementEnabled() {
		return
	}
	go func() {
		timer := time.NewTicker(time.Minute)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				a.cleanupProcurementUploads(ctx)
			}
		}
	}()
}
func (a *App) cleanupProcurementUploads(ctx context.Context) {
	if !procurementEnabled() {
		return
	}
	runtime := a.operations()
	if runtime.maintenance.Load() || runtime.pauseWrites.Load() {
		return
	}
	runtime.gate.RLock()
	defer runtime.gate.RUnlock()
	runtime.writes.RLock()
	defer runtime.writes.RUnlock()
	var files []ProcurementAttachment
	if a.DB.Where("scope='' AND created_at<?", time.Now().UTC().Add(-24*time.Hour)).Limit(50).Find(&files).Error != nil {
		return
	}
	for _, file := range files {
		if ctx.Err() != nil {
			return
		}
		remove := false
		err := a.DB.Transaction(func(tx *gorm.DB) error {
			if _, e := procLockProject(tx, file.ProjectID); e != nil {
				return e
			}
			var current ProcurementAttachment
			if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, file.ID).Error; e != nil {
				return e
			}
			if current.Scope != "" {
				return nil
			}
			if e := tx.Delete(&current).Error; e != nil {
				return e
			}
			var references int64
			if e := tx.Model(&ProcurementAttachment{}).Where("object_key=?", file.ObjectKey).Count(&references).Error; e != nil {
				return e
			}
			remove = references == 0
			return nil
		})
		if err == nil && remove {
			a.cleanupNewObjects(file.ObjectKey)
		}
	}
}
