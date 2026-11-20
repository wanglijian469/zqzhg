package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
	"zqzt/exhibition/internal/bailian"
)

type AIImportWorkItem struct {
	ID              uint64     `json:"id"`
	TaskID          uint64     `json:"-"`
	Stage           string     `json:"stage"`
	Status          string     `json:"status"`
	PromptVersionID uint64     `json:"prompt_version_id"`
	Fingerprint     string     `json:"-"`
	Attempts        int        `json:"attempts"`
	Error           string     `json:"error,omitempty"`
	ErrorField      string     `json:"error_field,omitempty"`
	NextRetryAt     *time.Time `json:"next_retry_at,omitempty"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (AIImportWorkItem) TableName() string { return "ai_import_work_items" }

type aiBudgetKey struct{}
type aiRequestBudget struct {
	WorkID     uint64
	Stage      string
	Remaining  int
	RetryAfter int
}

var errAIRetryScheduled = errors.New("ai_retry_scheduled")
var errAIPaused = errors.New("ai_pause_requested")

func (a *App) aiCheckContinue(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if a.operations().maintenance.Load() || a.operations().pauseWrites.Load() {
		return errAIPaused
	}
	return nil
}

func aiLocalFailure(e error) bool {
	if e == nil {
		return false
	}
	code := e.Error()
	if code == "ai_invalid_model_settings" || code == "ai_invalid_prompt_rules" || code == "ai_model_key_unreadable" {
		return false
	}
	return code == "ai_retry_scheduled" || strings.HasPrefix(code, "ai_invalid_") || strings.HasPrefix(code, "ai_schema_") || strings.HasPrefix(code, "ai_parameter_") || code == "ai_company_source_required" || code == "ai_no_products_found" || code == "ai_whole_page_image" || code == "ai_inventory_omission" || strings.HasPrefix(code, "bailian_invalid_") || code == "bailian_transport_unknown" || code == "bailian_temporary_failure"
}
func (a *App) aiBumpResult(task uint64) error {
	return a.DB.Model(&AIImportTask{}).Where("id=?", task).Update("result_version", gorm.Expr("result_version+1")).Error
}

func (a *App) aiModel(ctx context.Context, task AIImportTask, stage, model, prompt string, parts []bailian.Part, out any) error {
	if e := a.aiCheckContinue(ctx); e != nil {
		return e
	}
	if b, _ := ctx.Value(aiBudgetKey{}).(*aiRequestBudget); b != nil && b.WorkID != 0 {
		return a.aiModelAttempt(ctx, task, stage, model, prompt, parts, out)
	}
	version := task.PromptVersionID
	if version == 0 {
		version = 1
	}
	item := AIImportWorkItem{TaskID: task.ID, Stage: stage, Status: "queued", PromptVersionID: version, UpdatedAt: time.Now().UTC()}
	if e := a.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&item).Error; e != nil {
		return e
	}
	if e := a.DB.First(&item, "task_id=? AND stage=?", task.ID, stage).Error; e != nil {
		return e
	}
	if item.Status == "failed" {
		return errors.New(item.Error)
	}
	if item.Status == "waiting" && item.NextRetryAt != nil && item.NextRetryAt.After(time.Now()) {
		return errAIRetryScheduled
	}
	task.PromptVersionID = item.PromptVersionID
	budget, _ := ctx.Value(aiBudgetKey{}).(*aiRequestBudget)
	if budget == nil {
		budget = &aiRequestBudget{}
	}
	budget.WorkID = item.ID
	budget.Stage = stage
	budget.Remaining = max(0, 3-item.Attempts)
	ctx = context.WithValue(ctx, aiBudgetKey{}, budget)
	if e := a.DB.Model(&item).Updates(map[string]any{"status": "processing", "next_retry_at": nil}).Error; e != nil {
		return e
	}
	e := a.aiModelAttempt(ctx, task, stage, model, prompt, parts, out)
	status, code := "succeeded", ""
	var next *time.Time
	if e != nil {
		code = aiSafeError(e)
		status = "failed"
		if aiLocalFailure(e) && budget.Remaining > 0 {
			status = "waiting"
			seconds := 2
			if budget.Remaining == 1 {
				seconds = 8
			}
			seconds = max(seconds, budget.RetryAfter)
			when := time.Now().UTC().Add(time.Duration(seconds) * time.Second)
			next = &when
		}
	}
	if u := a.DB.Model(&item).Updates(map[string]any{"status": status, "error": code, "next_retry_at": next, "updated_at": time.Now().UTC()}).Error; u != nil {
		return u
	}
	if u := a.aiBumpResult(task.ID); u != nil {
		return u
	}
	if status == "waiting" {
		return errAIRetryScheduled
	}
	return e
}
func (a *App) aiAttemptAllowed(ctx context.Context) error {
	if b, _ := ctx.Value(aiBudgetKey{}).(*aiRequestBudget); b != nil {
		if b.Remaining <= 0 {
			return errors.New("ai_invalid_schema")
		}
		b.Remaining--
		return a.DB.Model(&AIImportWorkItem{}).Where("id=?", b.WorkID).Update("attempts", gorm.Expr("attempts+1")).Error
	}
	return nil
}
func (a *App) aiUnitBusinessRule(ctx context.Context, task AIImportTask, stage string) (string, error) {
	version := task.PromptVersionID
	if b, _ := ctx.Value(aiBudgetKey{}).(*aiRequestBudget); b != nil && b.WorkID > 0 {
		var item AIImportWorkItem
		if e := a.DB.First(&item, b.WorkID).Error; e != nil {
			return "", e
		}
		version = item.PromptVersionID
	}
	return a.aiPromptText(version, stage)
}
func (a *App) aiUnits(task uint64) ([]AIImportWorkItem, error) {
	items := []AIImportWorkItem{}
	e := a.DB.Where("task_id=?", task).Order("id").Find(&items).Error
	return items, e
}
func (a *App) aiTaskSnapshot(task AIImportTask, details bool) gin.H {
	out := gin.H{}
	_ = json.Unmarshal([]byte(aiJSON(task)), &out)
	items, e := a.aiUnits(task.ID)
	if e != nil {
		out["summary_unavailable"] = true
		return out
	}
	summary := map[string]int{"total": len(items), "succeeded": 0, "failed": 0, "waiting": 0, "processing": 0, "queued": 0, "manual": 0}
	for _, v := range items {
		summary[v.Status]++
	}
	out["unit_summary"] = summary
	active := []AIImportWorkItem{}
	indexed := 0
	for _, v := range items {
		if v.Status == "processing" {
			active = append(active, v)
		}
		if strings.HasPrefix(v.Stage, "index-") && v.Status == "succeeded" {
			indexed++
		}
	}
	if len(items) == 0 {
		for key := range a.aiReadCheckpoint(task.ID).Models {
			if strings.HasPrefix(key, "index-") && !strings.Contains(strings.TrimPrefix(key, "index-"), "-") {
				indexed++
			}
		}
		if task.Progress == 100 && (task.Status == "review_pending" || task.Status == "completed") {
			out["recognition_outcome"] = "complete"
		}
	}
	out["active_units"] = active
	out["coverage"] = gin.H{"indexed_pages": indexed, "total_pages": task.Total}
	if details {
		out["units"] = items
	}
	if active, e := aiActivePrompt(a.DB); e == nil {
		out["latest_prompt_version"] = active.ID
	}
	return out
}
func (a *App) aiFinalizeOutcome(task AIImportTask) (string, *time.Time, error) {
	items, e := a.aiUnits(task.ID)
	if e != nil {
		return "none", nil, e
	}
	outcome := "complete"
	var incomplete int64
	if e := a.DB.Table(aiTable("product")).Where("task_id=? AND generation_state='incomplete' AND deleted=0", task.ID).Count(&incomplete).Error; e != nil {
		return "none", nil, e
	}
	if incomplete > 0 {
		outcome = "partial"
	}
	var next *time.Time
	for _, v := range items {
		if v.Status == "failed" || v.Status == "waiting" || v.Status == "queued" || v.Status == "processing" {
			outcome = "partial"
		}
		if v.Status == "waiting" && v.NextRetryAt != nil && (next == nil || v.NextRetryAt.Before(*next)) {
			at := *v.NextRetryAt
			next = &at
		}
	}
	return outcome, next, nil
}
func (a *App) aiMarkUnitIssue(task AIImportTask, stage string, e error) error {
	a.DB.Model(&AIImportModelCall{}).Where("task_id=? AND (stage=? OR stage=?)", task.ID, stage, stage+"-repair").Update("error", "ai_invalid_evidence")
	c := a.aiReadCheckpoint(task.ID)
	delete(c.Models, stage)
	delete(c.Models, stage+"-repair")
	if err := a.DB.Model(&AIImportTask{}).Where("id=? AND lease=?", task.ID, task.Lease).Update("checkpoint", aiJSON(c)).Error; err != nil {
		return err
	}
	var row AIImportWorkItem
	if err := a.DB.First(&row, "task_id=? AND stage=?", task.ID, stage).Error; err != nil {
		return err
	}
	status := "failed"
	var next *time.Time
	if row.Attempts < 3 {
		status = "waiting"
		at := time.Now().UTC().Add(8 * time.Second)
		next = &at
	}
	return a.DB.Model(&row).Updates(map[string]any{"status": status, "error": aiSafeError(e), "next_retry_at": next}).Error
}
func (a *App) aiStorePartialProduct(ctx context.Context, task AIImportTask, product AIInventoryProduct, x *AIProductExtraction, problem error) error {
	p := Payload{Name: product.ProductName, ProductDetails: &ProductDetails{SourcePages: product.SourcePages, Models: []AIProductModel{}, Images: []AIProductImage{}, Features: []string{}, Applications: []string{}}}
	if x != nil {
		p = aiProductPayload(*x)
		p.Name = product.ProductName
		p.ProductDetails.SourcePages = product.SourcePages
	}
	issues := []string{"识别尚未完整完成，请重试失败项或对照原件人工补齐。原因：" + aiSafeError(problem)}
	d := AIImportDraft{TaskID: task.ID, OwnerID: task.OwnerID, TempID: product.TempID, Version: 1, GenerationState: "incomplete", Data: aiJSON(p), SourcePages: aiJSON(product.SourcePages), FieldSources: "{}", ReviewIssues: aiJSON(issues), SelectedFields: "[]"}
	return a.aiStoreDraft(ctx, task, "product", &d)
}
func aiProductStage(stage, temp string) bool { return strings.HasPrefix(stage, temp+"-") }
func (a *App) aiManualComplete(tx *gorm.DB, d AIImportDraft, resolution string) error {
	if strings.TrimSpace(resolution) == "" {
		return serviceFailure{"ai_resolution_required", 400}
	}
	if e := tx.Model(&AIImportWorkItem{}).Where("task_id=? AND stage LIKE ? AND status IN ('failed','waiting','queued')", d.TaskID, d.TempID+"-%").Updates(map[string]any{"status": "manual", "next_retry_at": nil}).Error; e != nil {
		return e
	}
	if e := tx.Create(&AIImportReviewItem{TaskID: d.TaskID, OwnerID: d.OwnerID, DraftID: d.ID, DraftKind: "product", DraftVersion: d.Version + 1, Issue: "厂商人工补齐未完成识别", Resolution: resolution, ActorID: d.OwnerID, CreatedAt: time.Now().UTC()}).Error; e != nil {
		return e
	}
	return tx.Table(aiTable("product")).Where("id=?", d.ID).Update("generation_state", "ready").Error
}
func aiUnitLabel(stage string) string {
	if strings.HasPrefix(stage, "index-") {
		return "来源页目录 " + strings.TrimPrefix(stage, "index-")
	}
	if stage == "company" {
		return "企业资料"
	}
	if stage == "inventory-merge" {
		return "产品目录整理"
	}
	return fmt.Sprintf("产品处理 %s", stage)
}
func (a *App) aiStableCatalog(task AIImportTask, incoming []AIInventoryProduct) ([]AIInventoryProduct, error) {
	c := a.aiReadCheckpoint(task.ID)
	if len(c.Catalog) == 0 {
		var drafts []AIImportDraft
		if e := a.DB.Table(aiTable("product")).Where("task_id=?", task.ID).Order("id").Find(&drafts).Error; e != nil {
			return nil, e
		}
		for _, d := range drafts {
			var p Payload
			_ = json.Unmarshal([]byte(d.Data), &p)
			var pages []int
			_ = json.Unmarshal([]byte(d.SourcePages), &pages)
			c.Catalog = append(c.Catalog, AIInventoryProduct{TempID: d.TempID, ProductName: p.Name, SourcePages: pages})
		}
	}
	for _, p := range incoming {
		found := false
		for i, prior := range c.Catalog {
			if prior.ProductName == p.ProductName && ((len(prior.BlockIDs) > 0 && prior.SeriesName == p.SeriesName && aiJSON(prior.BlockIDs) == aiJSON(p.BlockIDs)) || (len(prior.BlockIDs) == 0 && aiJSON(prior.SourcePages) == aiJSON(p.SourcePages))) {
				found = true
				if len(prior.BlockIDs) == 0 {
					p.TempID = prior.TempID
					c.Catalog[i] = p
				}
				break
			}
		}
		if !found {
			for _, prior := range c.Catalog {
				overlap := false
				for _, oldBlock := range prior.BlockIDs {
					for _, newBlock := range p.BlockIDs {
						overlap = overlap || oldBlock == newBlock
					}
				}
				if overlap {
					p.ReviewIssues = append(p.ReviewIssues, "与已保存候选 "+prior.TempID+" 存在来源区块重叠，请核对跨页合并或结构变体；系统未自动删除。")
				}
			}
			p.TempID = fmt.Sprintf("product-%d", len(c.Catalog)+1)
			c.Catalog = append(c.Catalog, p)
		}
	}
	if e := a.DB.Model(&AIImportTask{}).Where("id=? AND lease=? AND status='processing'", task.ID, task.Lease).Update("checkpoint", aiJSON(c)).Error; e != nil {
		return nil, e
	}
	return c.Catalog, nil
}

// A later recovered directory page must not rerun or overwrite a company phase
// whose valid result is already saved. The original model-call ledger is kept.
func (a *App) aiReuseSavedCompany(ctx context.Context, task AIImportTask) (bool, error) {
	reused := false
	err := a.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := aiGuardWorkerWrite(tx, ctx, task.ID, task.OwnerID); err != nil {
			return err
		}
		var draft AIImportDraft
		err := tx.Table(aiTable("company")).Where("task_id=? AND owner_id=? AND generation_state='ready' AND deleted=0", task.ID, task.OwnerID).First(&draft).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		reused = true
		result := tx.Model(&AIImportWorkItem{}).Where("task_id=? AND stage LIKE 'company%' AND status<>'succeeded'", task.ID).Updates(map[string]any{"status": "succeeded", "error": "", "error_field": "saved_draft_reused", "next_retry_at": nil, "updated_at": time.Now().UTC()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			return tx.Model(&AIImportTask{}).Where("id=?", task.ID).Update("result_version", gorm.Expr("result_version+1")).Error
		}
		return nil
	})
	return reused, err
}
