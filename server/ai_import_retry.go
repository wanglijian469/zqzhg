package main

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
)

func (a *App) aiRetryUnits(c *gin.Context) {
	var req struct {
		UnitIDs       []uint64 `json:"unit_ids"`
		PromptPolicy  string   `json:"prompt_policy"`
		PromptVersion uint64   `json:"prompt_version_id"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.UnitIDs) > 5000 || (req.PromptPolicy != "" && req.PromptPolicy != "original" && req.PromptPolicy != "latest") {
		fail(c, 400, "invalid_request")
		return
	}
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		var t AIImportTask
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND owner_id=?", id(c), actor(c).ID).First(&t).Error; e != nil {
			return e
		}
		var active int64
		if e := tx.Model(&AIImportTask{}).Where("owner_id=? AND status IN ('queued','processing')", t.OwnerID).Count(&active).Error; e != nil {
			return e
		}
		if active > 0 {
			return serviceFailure{"ai_task_busy", 409}
		}
		var account Account
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&account, t.OwnerID).Error; e != nil {
			return e
		}
		if t.PublishKey != "" && t.SubmissionMode != "batch" {
			return serviceFailure{"ai_task_locked", 409}
		}
		query := tx.Model(&AIImportWorkItem{}).Where("task_id=? AND status IN ('failed','waiting','queued','processing')", t.ID)
		if len(req.UnitIDs) > 0 {
			var owned int64
			if e := tx.Model(&AIImportWorkItem{}).Where("task_id=? AND id IN ?", t.ID, req.UnitIDs).Count(&owned).Error; e != nil {
				return e
			}
			if owned != int64(len(req.UnitIDs)) {
				return serviceFailure{"not_found", 404}
			}
			query = query.Where("id IN ?", req.UnitIDs)
		}
		var rows []AIImportWorkItem
		if e := query.Find(&rows).Error; e != nil {
			return e
		}
		version := uint64(0)
		if req.PromptPolicy == "latest" {
			v, e := aiActivePrompt(tx)
			if e != nil {
				return e
			}
			if v.ID != req.PromptVersion {
				return serviceFailure{"ai_prompt_version_changed", 409}
			}
			version = v.ID
		}
		ids := []uint64{}
		checkpoint := a.aiReadCheckpoint(t.ID)
		var drafts []AIImportDraft
		if e := tx.Table(aiTable("product")).Where("task_id=?", t.ID).Find(&drafts).Error; e != nil {
			return e
		}
		var companies []AIImportDraft
		if e := tx.Table(aiTable("company")).Where("task_id=?", t.ID).Find(&companies).Error; e != nil {
			return e
		}
		for _, row := range rows {
			// Human-edited and submitted results cannot be regenerated underneath the user.
			protected := false
			if strings.HasPrefix(row.Stage, "company") {
				for _, d := range companies {
					if d.Version > 1 || d.ContentID > 0 {
						protected = true
					}
				}
			}
			for _, d := range drafts {
				if aiProductStage(row.Stage, d.TempID) && (d.Version > 1 || d.ContentID > 0 || d.Deleted) {
					protected = true
					break
				}
			}
			if protected {
				continue
			}
			ids = append(ids, row.ID)
			for key := range checkpoint.Models {
				if key == row.Stage || strings.HasPrefix(key, row.Stage+"-") {
					delete(checkpoint.Models, key)
				}
			}
		}
		if len(ids) == 0 && t.Status != "failed" {
			return serviceFailure{"ai_no_failed_units", 409}
		}
		updates := map[string]any{"status": "queued", "attempts": 0, "error": "", "error_field": "", "next_retry_at": nil}
		if version > 0 {
			updates["prompt_version_id"] = version
		}
		if len(ids) > 0 {
			if e := tx.Model(&AIImportWorkItem{}).Where("id IN ?", ids).Updates(updates).Error; e != nil {
				return e
			}
		}
		if e := recordOperation(tx, actor(c).ID, "ai_import", t.ID, "ai_retry_units", "policy="+req.PromptPolicy+";units="+aiJSON(ids)); e != nil {
			return e
		}
		return tx.Model(&t).Updates(map[string]any{"status": "queued", "error": "", "lease": "", "locked_until": nil, "next_retry_at": nil, "checkpoint": aiJSON(checkpoint), "result_version": gorm.Expr("result_version+1"), "updated_at": time.Now().UTC()}).Error
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
