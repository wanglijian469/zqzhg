package main

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

type AISubmissionBatch struct {
	ID             uint64    `json:"id"`
	TaskID         uint64    `json:"task_id"`
	OwnerID        uint64    `json:"-"`
	Kind           string    `json:"kind"`
	IdempotencyKey string    `json:"-"`
	Fingerprint    string    `json:"-"`
	RequestJSON    string    `json:"-"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (AISubmissionBatch) TableName() string { return "ai_import_submission_batches" }

type AISubmissionItem struct {
	ID          uint64    `json:"id"`
	BatchID     uint64    `json:"batch_id"`
	TaskID      uint64    `json:"task_id"`
	OwnerID     uint64    `json:"-"`
	Kind        string    `json:"kind"`
	DraftID     uint64    `json:"draft_id"`
	Version     uint64    `json:"version"`
	RequestJSON string    `json:"-"`
	Status      string    `json:"status"`
	Error       string    `json:"error"`
	FieldsJSON  string    `json:"-"`
	ContentID   uint64    `json:"content_id"`
	RevisionID  uint64    `json:"revision_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (AISubmissionItem) TableName() string { return "ai_import_submission_items" }
func aiSubmissionDone(status string) bool  { return status == "submitted" || status == "linked" }
func (item AISubmissionItem) MarshalJSON() ([]byte, error) {
	type plain AISubmissionItem
	raw := json.RawMessage(item.FieldsJSON)
	if !json.Valid(raw) {
		raw = json.RawMessage("[]")
	}
	return json.Marshal(struct {
		plain
		Fields json.RawMessage `json:"fields"`
	}{plain(item), raw})
}
func aiBatchBlockers(d AIImportDraft, kind string, existing bool) []string {
	reasons := []string{}
	if d.ContentID > 0 {
		return reasons
	}
	if d.GenerationState == "incomplete" {
		reasons = append(reasons, "ai_product_incomplete")
	}
	if d.ActiveBatchID > 0 {
		reasons = append(reasons, "ai_draft_submitting")
	}
	var p Payload
	json.Unmarshal([]byte(d.Data), &p)
	if kind == "company" && existing {
		var selected []string
		json.Unmarshal([]byte(d.SelectedFields), &selected)
		if len(selected) == 0 {
			return reasons
		}
	}
	if strings.TrimSpace(p.Name) == "" {
		reasons = append(reasons, "name_required")
	}
	if kind == "product" {
		if len(p.MediaIDs) == 0 {
			reasons = append(reasons, "main_image_required")
		}
		if p.CategoryID == 0 && p.ProductCategoryID == 0 && p.VendorProductCategoryID == 0 {
			reasons = append(reasons, "product_category_required")
		}
		if d.PublishMode == "undecided" {
			reasons = append(reasons, "ai_update_choice_required")
		}
	}
	return reasons
}

type aiBatchEdit struct {
	ID               uint64   `json:"id"`
	Version          uint64   `json:"version"`
	Data             *Payload `json:"data,omitempty"`
	Resolution       string   `json:"resolution"`
	SelectedFields   []string `json:"selected_fields,omitempty"`
	PublishMode      string   `json:"publish_mode,omitempty"`
	TargetContentID  uint64   `json:"target_content_id,omitempty"`
	TargetRevisionID uint64   `json:"target_revision_id,omitempty"`
}
type aiBatchRequest struct {
	Key        string         `json:"idempotency_key"`
	Items      []aiBatchEdit  `json:"items"`
	Agreements []AgreementRef `json:"agreements"`
}

func (a *App) aiSubmitCompanyBatch(c *gin.Context) { a.aiSubmitBatch(c, "company") }
func (a *App) aiSubmitProductBatch(c *gin.Context) { a.aiSubmitBatch(c, "product") }
func (a *App) aiRetrySubmissionBatch(c *gin.Context) {
	var incoming struct {
		Agreements []AgreementRef `json:"agreements"`
	}
	if c.ShouldBindJSON(&incoming) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	batchID, e := strconv.ParseUint(c.Param("batch_id"), 10, 64)
	if e != nil {
		fail(c, 404, "not_found")
		return
	}
	var batch AISubmissionBatch
	if e = a.DB.Where("id=? AND task_id=? AND owner_id=?", batchID, id(c), actor(c).ID).First(&batch).Error; e != nil {
		dbFail(c, e)
		return
	}
	if batch.Kind != "company" && batch.Kind != "product" {
		fail(c, 409, "ai_legacy_batch_use_current_drafts")
		return
	}
	var req aiBatchRequest
	if json.Unmarshal([]byte(batch.RequestJSON), &req) != nil {
		fail(c, 409, "ai_publish_conflict")
		return
	}
	req.Agreements = incoming.Agreements
	c.Request.Body = io.NopCloser(strings.NewReader(aiJSON(req)))
	a.aiSubmitBatch(c, batch.Kind)
}
func (a *App) aiSubmitBatch(c *gin.Context, kind string) {
	var req aiBatchRequest
	if c.ShouldBindJSON(&req) != nil || !validContentRequestID(req.Key) || len(req.Items) == 0 || len(req.Items) > 1000 || (kind == "company" && len(req.Items) != 1) {
		fail(c, 400, "invalid_request")
		return
	}
	sort.Slice(req.Items, func(i, j int) bool { return req.Items[i].ID < req.Items[j].ID })
	for i, v := range req.Items {
		if v.ID == 0 || v.Version == 0 || len(v.Resolution) > 8000 || (i > 0 && req.Items[i-1].ID == v.ID) {
			fail(c, 400, "invalid_request")
			return
		}
	}
	who := actor(c)
	fingerprint := digestText(kind + aiJSON(req.Items))
	var task AIImportTask
	var batch AISubmissionBatch
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND owner_id=?", id(c), who.ID).First(&task).Error; e != nil {
			return e
		}
		e := tx.Where("task_id=? AND owner_id=? AND idempotency_key=?", task.ID, who.ID, req.Key).First(&batch).Error
		if e == nil {
			if batch.Fingerprint != fingerprint || batch.Kind != kind {
				return serviceFailure{"ai_publish_conflict", 409}
			}
			return acceptServiceDocuments(tx, who.ID, req.Agreements, "web_use", []string{"terms", "privacy", "showroom-agreement"})
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		if task.Status != "review_pending" && task.Status != "completed" && task.Status != "failed" && task.Status != "processing" && task.Status != "queued" {
			return serviceFailure{"ai_task_locked", 409}
		}
		if kind == "product" {
			var vendor Content
			if e := tx.Where("kind='vendor' AND owner_id=?", who.ID).First(&vendor).Error; e != nil {
				if errors.Is(e, gorm.ErrRecordNotFound) {
					return serviceFailure{"ai_company_required", 409}
				}
				return e
			}
			task.VendorID = vendor.ID
		}
		rows := []AIImportDraft{}
		for _, v := range req.Items {
			var d AIImportDraft
			if e := tx.Table(aiTable(kind)).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND task_id=? AND owner_id=? AND deleted=0", v.ID, task.ID, who.ID).First(&d).Error; e != nil {
				return e
			}
			if d.ContentID == 0 && d.Version != v.Version {
				return serviceFailure{"stale_revision", 409}
			}
			if d.ActiveBatchID != 0 {
				return serviceFailure{"ai_draft_submitting", 409}
			}
			rows = append(rows, d)
		}
		if e := acceptServiceDocuments(tx, who.ID, req.Agreements, "web_use", []string{"terms", "privacy", "showroom-agreement"}); e != nil {
			return e
		}
		now := time.Now().UTC()
		batch = AISubmissionBatch{TaskID: task.ID, OwnerID: who.ID, Kind: kind, IdempotencyKey: req.Key, Fingerprint: fingerprint, RequestJSON: aiJSON(req), Status: "pending", CreatedAt: now, UpdatedAt: now}
		if e := tx.Create(&batch).Error; e != nil {
			return e
		}
		for i, d := range rows {
			item := AISubmissionItem{BatchID: batch.ID, TaskID: task.ID, OwnerID: who.ID, Kind: kind, DraftID: d.ID, Version: req.Items[i].Version, RequestJSON: aiJSON(req.Items[i]), Status: "pending", FieldsJSON: "[]", CreatedAt: now, UpdatedAt: now}
			if d.ContentID > 0 {
				item.Status = "submitted"
				item.ContentID = d.ContentID
				item.RevisionID = d.RevisionID
			}
			if e := tx.Create(&item).Error; e != nil {
				return e
			}
			if d.ContentID == 0 {
				if e := tx.Table(aiTable(kind)).Where("id=?", d.ID).Update("active_batch_id", batch.ID).Error; e != nil {
					return e
				}
			}
		}
		changes := map[string]any{"submission_mode": "batch", "vendor_id": task.VendorID}
		if task.Status != "processing" && task.Status != "queued" {
			changes["status"] = "review_pending"
			changes["stage"] = "review_pending"
		}
		return tx.Model(&task).Updates(changes).Error
	})
	if err != nil {
		dbFail(c, err)
		return
	}
	var items []AISubmissionItem
	if e := a.DB.Where("batch_id=? AND owner_id=?", batch.ID, who.ID).Order("id").Find(&items).Error; e != nil {
		dbFail(c, e)
		return
	}
	for _, item := range items {
		if !aiSubmissionDone(item.Status) {
			a.aiExecuteBatchItem(c, who, task, batch, item)
		}
	}
	if e := a.aiFinishBatch(task, batch); e != nil {
		dbFail(c, e)
		return
	}
	if e := a.DB.First(&batch, batch.ID).Error; e != nil {
		dbFail(c, e)
		return
	}
	if e := a.DB.Where("batch_id=? AND owner_id=?", batch.ID, who.ID).Order("id").Find(&items).Error; e != nil {
		dbFail(c, e)
		return
	}
	submitted, failed := 0, 0
	results := []gin.H{}
	for _, item := range items {
		if aiSubmissionDone(item.Status) {
			submitted++
		} else {
			failed++
		}
		var fields any
		json.Unmarshal([]byte(item.FieldsJSON), &fields)
		results = append(results, gin.H{"id": item.DraftID, "version": item.Version, "status": item.Status, "content_id": item.ContentID, "revision_id": item.RevisionID, "error": item.Error, "fields": fields})
	}
	c.JSON(200, gin.H{"batch": batch, "submitted": submitted, "failed": failed, "results": results, "message": "platform_review_required"})
}
func (a *App) aiExecuteBatchItem(c *gin.Context, who Account, task AIImportTask, batch AISubmissionBatch, item AISubmissionItem) {
	err := a.contentTransaction(func(tx *gorm.DB) error {
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&task, "id=? AND owner_id=?", task.ID, who.ID).Error; e != nil {
			return e
		}
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, "id=? AND owner_id=?", item.ID, who.ID).Error; e != nil {
			return e
		}
		if aiSubmissionDone(item.Status) {
			return nil
		}
		var d AIImportDraft
		if e := tx.Table(aiTable(item.Kind)).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND task_id=? AND owner_id=? AND deleted=0", item.DraftID, task.ID, who.ID).First(&d).Error; e != nil {
			return e
		}
		if d.ContentID > 0 {
			return tx.Model(&item).Updates(map[string]any{"status": "submitted", "content_id": d.ContentID, "revision_id": d.RevisionID, "error": "", "fields_json": "[]", "updated_at": time.Now().UTC()}).Error
		}
		if d.ActiveBatchID != 0 && d.ActiveBatchID != batch.ID {
			return serviceFailure{"ai_draft_submitting", 409}
		}
		if d.GenerationState == "incomplete" {
			return serviceFailure{"ai_product_incomplete", 409}
		}
		if d.Version != item.Version {
			return serviceFailure{"stale_revision", 409}
		}
		var edit aiBatchEdit
		if json.Unmarshal([]byte(item.RequestJSON), &edit) != nil {
			return serviceInvalid()
		}
		if e := a.aiApplyBatchEdit(tx, who, task, &d, item.Kind, edit); e != nil {
			return e
		}
		var p Payload
		json.Unmarshal([]byte(d.Data), &p)
		var issues []string
		json.Unmarshal([]byte(d.ReviewIssues), &issues)
		requires := len(issues) > 0
		if p.ProductDetails != nil {
			for _, m := range p.ProductDetails.Models {
				for _, v := range m.Parameters {
					requires = requires || v.ReviewRequired
				}
			}
		}
		keptExisting := false
		if item.Kind == "company" {
			var count int64
			tx.Model(&Content{}).Where("owner_id=? AND kind='vendor'", who.ID).Count(&count)
			var fields []string
			json.Unmarshal([]byte(d.SelectedFields), &fields)
			if count > 0 && len(fields) == 0 {
				requires = false
				keptExisting = true
			}
		}
		if requires && strings.TrimSpace(edit.Resolution) == "" {
			return serviceFailure{"ai_resolution_required", 400}
		}
		if e := a.aiValidateReady(c.Request.Context(), tx, who, task, d, item.Kind); e != nil {
			return e
		}
		review := AIImportReviewItem{TaskID: task.ID, OwnerID: who.ID, DraftID: d.ID, DraftKind: item.Kind, DraftVersion: d.Version, Issue: d.ReviewIssues, Resolution: edit.Resolution, ActorID: who.ID, CreatedAt: time.Now().UTC()}
		if e := tx.Create(&review).Error; e != nil {
			return e
		}
		if e := tx.Table(aiTable(item.Kind)).Where("id=?", d.ID).Updates(map[string]any{"reviewed_version": d.Version, "data": aiJSON(aiReviewedPayload(p)), "active_batch_id": batch.ID, "updated_at": time.Now().UTC()}).Error; e != nil {
			return e
		}
		var e error
		if item.Kind == "company" {
			e = a.aiPublishCompanyTx(tx, task, who, d.Version, batch.ID)
		} else {
			e = a.aiPublishProductTx(tx, task, who, aiPublishSelection{ID: d.ID, Version: d.Version}, batch.ID)
		}
		if e != nil {
			return e
		}
		if e = tx.Table(aiTable(item.Kind)).First(&d, d.ID).Error; e != nil {
			return e
		}
		if e = tx.Table(aiTable(item.Kind)).Where("id=? AND active_batch_id=?", d.ID, batch.ID).Update("active_batch_id", 0).Error; e != nil {
			return e
		}
		state := "submitted"
		if keptExisting {
			state = "linked"
		}
		return tx.Model(&item).Updates(map[string]any{"version": d.Version, "status": state, "content_id": d.ContentID, "revision_id": d.RevisionID, "error": "", "fields_json": "[]", "updated_at": time.Now().UTC()}).Error
	})
	if err != nil {
		code := aiSafeError(err)
		var fields any = []FieldIssue{}
		var validation *FieldValidationError
		if errors.As(err, &validation) {
			fields = validation.Fields
		}
		a.DB.Transaction(func(tx *gorm.DB) error {
			// Do not overwrite a concurrent successful retry's result.
			if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, item.ID).Error; e != nil {
				return e
			}
			if aiSubmissionDone(item.Status) {
				return nil
			}
			if e := tx.Model(&item).Updates(map[string]any{"status": "failed", "error": code, "fields_json": aiJSON(fields), "updated_at": time.Now().UTC()}).Error; e != nil {
				return e
			}
			return tx.Table(aiTable(item.Kind)).Where("id=? AND content_id=0 AND (active_batch_id=? OR active_batch_id=0)", item.DraftID, batch.ID).Updates(map[string]any{"active_batch_id": 0, "error": code}).Error
		})
	}
}
func (a *App) aiApplyBatchEdit(tx *gorm.DB, who Account, task AIImportTask, d *AIImportDraft, kind string, edit aiBatchEdit) error {
	baseBefore := d.BaseRevisionID
	if kind == "company" && len(edit.SelectedFields) == 0 {
		var existing Content
		if e := tx.Where("kind='vendor' AND owner_id=?", who.ID).First(&existing).Error; e == nil {
			// No AI fields are being used. Do not validate, copy or overwrite unused suggestions.
			if d.SelectedFields != "[]" {
				d.Version++
			}
			d.SelectedFields = "[]"
			return tx.Table(aiTable(kind)).Where("id=?", d.ID).Updates(map[string]any{"selected_fields": "[]", "version": d.Version, "reviewed_version": 0, "error": "", "updated_at": time.Now().UTC()}).Error
		} else if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
	}
	p := Payload{}
	if json.Unmarshal([]byte(d.Data), &p) != nil {
		return serviceInvalid()
	}
	if edit.Data != nil {
		p = *edit.Data
	}
	business := "product"
	if kind == "company" {
		business = "vendor"
	}
	if e := a.validateDraftPayload(tx, who, business, &p); e != nil {
		return e
	}
	if e := a.aiValidateTaskMedia(tx, task.ID, p); e != nil {
		return e
	}
	data := aiJSON(p)
	updates := map[string]any{}
	changed := data != d.Data
	if kind == "company" {
		selected := aiJSON(edit.SelectedFields)
		if edit.SelectedFields == nil {
			selected = "[]"
		}
		changed = changed || selected != d.SelectedFields
		d.SelectedFields = selected
		updates["selected_fields"] = selected
		if edit.TargetRevisionID > 0 {
			var vendor Content
			if e := tx.Where("kind='vendor' AND owner_id=?", who.ID).First(&vendor).Error; e != nil {
				return e
			}
			if vendor.DraftRevisionID != edit.TargetRevisionID {
				return serviceFailure{"stale_revision", 409}
			}
			d.BaseRevisionID = edit.TargetRevisionID
			updates["base_revision_id"] = d.BaseRevisionID
		}
	} else {
		mode := edit.PublishMode
		if mode == "" {
			mode = d.PublishMode
		}
		if mode == "" {
			mode = "create"
		}
		if mode != "create" && mode != "update" && mode != "undecided" {
			return serviceInvalid()
		}
		target := d.TargetContentID
		if edit.TargetContentID > 0 {
			target = edit.TargetContentID
		}
		if target > 0 {
			var ct Content
			if e := tx.Where("id=? AND owner_id=? AND kind='product'", target, who.ID).First(&ct).Error; e != nil {
				return e
			}
			if edit.TargetRevisionID > 0 {
				if ct.DraftRevisionID != edit.TargetRevisionID {
					return serviceFailure{"stale_revision", 409}
				}
				d.BaseRevisionID = edit.TargetRevisionID
			} else if target != d.TargetContentID {
				return serviceFailure{"stale_revision", 409}
			}
		}
		if mode == "update" && target == 0 {
			return serviceFailure{"ai_update_choice_required", 409}
		}
		changed = changed || mode != d.PublishMode || target != d.TargetContentID
		d.PublishMode = mode
		d.TargetContentID = target
		updates["publish_mode"] = mode
		updates["target_content_id"] = target
		updates["base_revision_id"] = d.BaseRevisionID
	}
	changed = changed || baseBefore != d.BaseRevisionID
	if changed {
		d.Version++
	}
	d.Data = data
	updates["data"] = data
	updates["version"] = d.Version
	updates["reviewed_version"] = 0
	updates["error"] = ""
	updates["updated_at"] = time.Now().UTC()
	return tx.Table(aiTable(kind)).Where("id=?", d.ID).Updates(updates).Error
}
func (a *App) aiFinishBatch(task AIImportTask, batch AISubmissionBatch) error {
	return a.DB.Transaction(func(tx *gorm.DB) error {
		var t AIImportTask
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&t, task.ID).Error; e != nil {
			return e
		}
		var pending, failed int64
		tx.Model(&AISubmissionItem{}).Where("batch_id=? AND status='pending'", batch.ID).Count(&pending)
		tx.Model(&AISubmissionItem{}).Where("batch_id=? AND status='failed'", batch.ID).Count(&failed)
		state := "submitted"
		if pending > 0 {
			state = "pending"
		} else if failed > 0 {
			state = "partial_failed"
		}
		if e := tx.Model(&batch).Updates(map[string]any{"status": state, "updated_at": time.Now().UTC()}).Error; e != nil {
			return e
		}
		var remaining int64
		tx.Table(aiTable("product")).Where("task_id=? AND deleted=0 AND content_id=0", task.ID).Count(&remaining)
		status, stage := "review_pending", "review_pending"
		if remaining == 0 && t.VendorID > 0 {
			status, stage = "completed", "submitted"
		}
		if t.Status == "processing" || t.Status == "queued" {
			return nil
		}
		return tx.Model(&t).Updates(map[string]any{"status": status, "stage": stage, "updated_at": time.Now().UTC()}).Error
	})
}

// Only metadata is backfilled: historical drafts, approvals and business IDs stay untouched.
func migrateAIImportBatches(db *gorm.DB) error {
	var tasks []AIImportTask
	if e := db.Where("publish_key<>''").Find(&tasks).Error; e != nil {
		return e
	}
	for _, t := range tasks {
		e := db.Transaction(func(tx *gorm.DB) error {
			var n int64
			tx.Model(&AISubmissionBatch{}).Where("task_id=? AND owner_id=? AND idempotency_key=?", t.ID, t.OwnerID, t.PublishKey).Count(&n)
			if n > 0 {
				return nil
			}
			batch := AISubmissionBatch{TaskID: t.ID, OwnerID: t.OwnerID, Kind: "legacy", IdempotencyKey: t.PublishKey, Fingerprint: digestText(t.PublishManifest), RequestJSON: t.PublishManifest, Status: "legacy", CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
			if e := tx.Create(&batch).Error; e != nil {
				return e
			}
			var manifest struct {
				Products []aiPublishSelection `json:"products"`
			}
			json.Unmarshal([]byte(t.PublishManifest), &manifest)
			for _, kind := range []string{"company", "product"} {
				var rows []AIImportDraft
				q := tx.Table(aiTable(kind)).Where("task_id=? AND owner_id=?", t.ID, t.OwnerID)
				if kind == "product" {
					ids := []uint64{}
					for _, v := range manifest.Products {
						ids = append(ids, v.ID)
					}
					if len(ids) == 0 {
						continue
					}
					q = q.Where("id IN ?", ids)
				}
				if e := q.Find(&rows).Error; e != nil {
					return e
				}
				for _, d := range rows {
					state := "legacy_pending"
					if d.ContentID > 0 {
						state = "submitted"
					} else if d.Error != "" {
						state = "failed"
					}
					item := AISubmissionItem{BatchID: batch.ID, TaskID: t.ID, OwnerID: t.OwnerID, Kind: kind, DraftID: d.ID, Version: d.Version, RequestJSON: "{}", Status: state, Error: d.Error, FieldsJSON: "[]", ContentID: d.ContentID, RevisionID: d.RevisionID, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt}
					if e := tx.Create(&item).Error; e != nil {
						return e
					}
				}
			}
			return tx.Model(&t).Update("submission_mode", "batch").Error
		})
		if e != nil {
			return e
		}
	}
	return nil
}
