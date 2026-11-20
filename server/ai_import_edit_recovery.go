package main

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func aiEditableDraft(tx *gorm.DB, kind string, draftID, owner uint64) (AIImportDraft, error) {
	var row AIImportDraft
	if e := tx.Table(aiTable(kind)).Select("id,task_id,owner_id").Where("id=? AND owner_id=? AND deleted=0", draftID, owner).First(&row).Error; e != nil {
		return row, e
	}
	if e := aiEditable(tx, row.TaskID, owner); e != nil {
		return row, e
	}
	// A current locking read after the task lock prevents stale repeatable-read
	// snapshots from reporting a concurrent save/split as successful.
	e := tx.Table(aiTable(kind)).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND owner_id=? AND deleted=0", draftID, owner).First(&row).Error
	if e == nil {
		var active int64
		prefix := row.TempID + "-%"
		if kind == "company" {
			prefix = "company%"
		}
		if er := tx.Model(&AIImportWorkItem{}).Where("task_id=? AND stage LIKE ? AND status='processing'", row.TaskID, prefix).Count(&active).Error; er != nil {
			return row, er
		}
		if active > 0 {
			return row, serviceFailure{"ai_item_processing", 409}
		}
	}
	if e == nil && row.ActiveBatchID != 0 {
		return row, serviceFailure{"ai_draft_submitting", 409}
	}
	return row, e
}
func (a *App) aiReopenFailed(c *gin.Context) {
	a.aiReopenReview(c, true)
}
func (a *App) aiContinueReview(c *gin.Context) {
	a.aiReopenReview(c, false)
}
func (a *App) aiReopenReview(c *gin.Context, failedOnly bool) {
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		var task AIImportTask
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND owner_id=?", id(c), actor(c).ID).First(&task).Error; e != nil {
			return e
		}
		if task.PublishKey == "" || (failedOnly && task.Status != "review_pending") || (!failedOnly && task.Status != "completed") {
			return serviceFailure{"ai_task_locked", 409}
		}
		var failed int64
		query := tx.Table(aiTable("product")).Where("task_id=? AND owner_id=? AND content_id=0 AND deleted=0", task.ID, task.OwnerID)
		if failedOnly {
			query = query.Where("error<>''")
		}
		if e := query.Count(&failed).Error; e != nil {
			return e
		}
		if failed == 0 {
			return serviceFailure{"ai_task_locked", 409}
		}
		if e := tx.Table(aiTable("product")).Where("task_id=? AND content_id=0 AND error<>''", task.ID).Update("reviewed_version", 0).Error; e != nil {
			return e
		}
		if e := tx.Model(&task).Updates(map[string]any{"publish_key": "", "publish_manifest": "[]", "status": "review_pending", "stage": "review_pending"}).Error; e != nil {
			return e
		}
		detail := "remaining_unsubmitted_items"
		if failedOnly {
			detail = "failed_items_only"
		}
		return recordOperation(tx, actor(c).ID, "ai_import", task.ID, "ai_publish_reopened", detail)
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func aiReviewedPayload(p Payload) Payload {
	if p.ProductDetails != nil {
		for i := range p.ProductDetails.Models {
			for j := range p.ProductDetails.Models[i].Parameters {
				p.ProductDetails.Models[i].Parameters[j].ReviewRequired = false
			}
		}
	}
	return p
}
