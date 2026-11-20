package main

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type aiLeaseKey struct{}

func aiGuardWorkerWrite(tx *gorm.DB, ctx context.Context, task, owner uint64) error {
	lease, _ := ctx.Value(aiLeaseKey{}).(string)
	if lease == "" || task == 0 {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var row AIImportTask
	if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND owner_id=? AND lease=? AND status='processing'", task, owner, lease).First(&row).Error; e != nil {
		return errors.New("ai_lease_lost")
	}
	return nil
}
func (a *App) aiStoreDraft(ctx context.Context, task AIImportTask, kind string, draft *AIImportDraft) error {
	return a.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := aiGuardWorkerWrite(tx, ctx, task.ID, task.OwnerID); e != nil {
			return e
		}
		if draft.GenerationState == "" {
			draft.GenerationState = "ready"
		}
		var prior AIImportDraft
		if tx.Table(aiTable(kind)).Where("task_id=? AND temp_id=?", task.ID, draft.TempID).First(&prior).Error == nil && prior.GenerationState == "incomplete" && prior.Version == 1 && prior.ContentID == 0 && prior.ActiveBatchID == 0 && !prior.Deleted && draft.GenerationState == "ready" {
			if e := tx.Table(aiTable(kind)).Where("id=? AND version=1 AND generation_state='incomplete'", prior.ID).Updates(map[string]any{"data": draft.Data, "review_issues": draft.ReviewIssues, "source_pages": draft.SourcePages, "generation_state": "ready", "version": 2}).Error; e != nil {
				return e
			}
		}
		if e := tx.Table(aiTable(kind)).Clauses(clause.OnConflict{DoNothing: true}).Create(draft).Error; e != nil {
			return e
		}
		if kind == "product" && draft.TargetContentID > 0 {
			if e := tx.Table(aiTable(kind)).Where("task_id=? AND temp_id=? AND content_id=0", task.ID, draft.TempID).Updates(map[string]any{"target_content_id": draft.TargetContentID, "publish_mode": draft.PublishMode}).Error; e != nil {
				return e
			}
		}
		return tx.Model(&AIImportTask{}).Where("id=?", task.ID).Update("result_version", gorm.Expr("result_version+1")).Error
	})
}

// A semantic validation failure must not poison the retry cache forever. Keep
// the original response for audit, but request a fresh answer for the bad stage.
func (a *App) aiInvalidateFailedResponse(task AIImportTask, err error) {
	switch err.Error() {
	case "ai_invalid_schema", "ai_invalid_inventory", "ai_inventory_omission", "ai_company_source_required", "ai_invalid_source_page", "ai_parameter_source_required", "ai_invalid_crop", "ai_whole_page_image", "ai_invalid_logo_region", "ai_invalid_model":
	default:
		return
	}
	var last AIImportModelCall
	if a.DB.Where("task_id=? AND response_json<>''", task.ID).Order("id DESC").First(&last).Error != nil {
		return
	}
	a.DB.Model(&last).Update("error", "ai_schema_validation_failed")
	baseStage := strings.TrimSuffix(last.Stage, "-repair")
	remove := func(stage string) bool { return stage == baseStage || stage == baseStage+"-repair" }
	if err.Error() == "ai_invalid_crop" || err.Error() == "ai_whole_page_image" {
		if i := strings.Index(last.Stage, "-review-"); i >= 0 {
			prefix := last.Stage[:i]
			a.DB.Model(&AIImportModelCall{}).Where("task_id=? AND (stage LIKE ? OR stage IN ?)", task.ID, prefix+"-detail-%", []string{prefix + "-merge", prefix + "-merge-repair"}).Update("error", "ai_schema_validation_failed")
			remove = func(stage string) bool {
				return strings.HasPrefix(stage, prefix+"-detail-") || strings.HasPrefix(stage, prefix+"-review-") || stage == prefix+"-merge" || stage == prefix+"-merge-repair"
			}
		}
	}
	// The typed checkpoint is the first cache consulted on retry. Invalidating
	// only the raw audit row would otherwise replay the same bad result forever.
	checkpoint := a.aiReadCheckpoint(task.ID)
	for stage := range checkpoint.Models {
		if remove(stage) {
			delete(checkpoint.Models, stage)
		}
	}
	a.DB.Model(&AIImportTask{}).Where("id=? AND lease=? AND status='processing'", task.ID, task.Lease).Update("checkpoint", aiJSON(checkpoint))
}
