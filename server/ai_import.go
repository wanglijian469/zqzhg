package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"mime/multipart"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func aiEnabled() bool { return env("AI_IMPORT_ENABLED", "false") == "true" }
func aiGuard(c *gin.Context) {
	if !aiEnabled() {
		fail(c, 503, "ai_disabled")
		return
	}
	c.Next()
}
func (a *App) aiImportRoutes(merchant *gin.RouterGroup) {
	r := merchant.Group("/ai-import", aiGuard)
	r.GET("/config", func(c *gin.Context) {
		cfg, _, e := a.aiBailianConfig()
		c.JSON(200, gin.H{"configured": e == nil && cfg.Validate() == nil, "default_model": cfg.Model, "review_model": cfg.ReviewModel, "model_selection": "admin", "allow_manual_max": false, "pdf_tools_ready": aiToolsReady(), "max_file_bytes": settingInt("AI_IMPORT_MAX_FILE_BYTES", 30000000), "max_pages": settingInt("AI_IMPORT_MAX_PAGES", 100), "image_mib": a.uploadConfig().ImageMiB})
	})
	r.POST("/tasks", a.aiCreateTask)
	r.GET("/tasks", a.aiListTasks)
	r.GET("/tasks/:id", a.aiGetTask)
	r.GET("/tasks/:id/result", a.aiResult)
	r.POST("/tasks/:id/retry", a.aiRetry)
	r.PUT("/company/:id", a.aiSaveDraft("company"))
	r.PUT("/products/:id", a.aiSaveDraft("product"))
	r.DELETE("/products/:id", a.aiDeleteDraft)
	r.POST("/tasks/:id/review", a.aiReview)
	r.POST("/tasks/:id/publish", a.aiPublish)
	r.POST("/tasks/:id/company/submit", a.aiSubmitCompanyBatch)
	r.POST("/tasks/:id/products/submit", a.aiSubmitProductBatch)
	r.POST("/tasks/:id/batches/:batch_id/retry", a.aiRetrySubmissionBatch)
	r.POST("/tasks/:id/reopen-failed", a.aiReopenFailed)
	r.POST("/tasks/:id/continue-review", a.aiContinueReview)
	r.POST("/tasks/:id/products/merge", a.aiMerge)
	r.POST("/products/:id/split", a.aiSplit)
	r.GET("/assets/:id", a.aiReadAsset)
	r.POST("/assets/:id/crops", a.aiCropAsset)
}
func (a *App) aiOwnedTask(c *gin.Context) (AIImportTask, error) {
	var t AIImportTask
	e := a.DB.Where("id=? AND owner_id=?", id(c), actor(c).ID).First(&t).Error
	return t, e
}
func (a *App) aiListTasks(c *gin.Context) {
	p, n := page(c)
	rows := []AIImportTask{}
	var total int64
	q := a.DB.Model(&AIImportTask{}).Where("owner_id=?", actor(c).ID)
	if e := q.Count(&total).Error; e != nil {
		dbFail(c, e)
		return
	}
	if e := q.Order("id DESC").Offset((p - 1) * n).Limit(n).Find(&rows).Error; e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"items": rows, "total": total, "page": p, "page_size": n})
}
func (a *App) aiGetTask(c *gin.Context) {
	t, e := a.aiOwnedTask(c)
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, t)
}
func (a *App) aiCreateTask(c *gin.Context) {
	if cloudFilesEnabled() && env("OSS_UPLOADS_ENABLED", "true") != "true" {
		fail(c, 503, "oss_writes_disabled")
		return
	}
	cfg, _, configError := a.aiBailianConfig()
	if configError != nil {
		fail(c, 503, aiSafeError(configError))
		return
	}
	if e := cfg.Validate(); e != nil {
		fail(c, 503, aiSafeError(e))
		return
	}
	if strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		a.aiCreateTaskFromCloudFiles(c)
		return
	}
	limit := int64(settingInt("AI_IMPORT_MAX_FILE_BYTES", 30000000))
	c.Request.Body = httpMaxBody(c, limit+(2<<20))
	form, e := c.MultipartForm()
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	if e != nil {
		fail(c, 400, "ai_invalid_upload")
		return
	}
	// Client model fields are deprecated; only administrator configuration decides.
	modelMode := "admin"
	var maxConfirmed *time.Time
	var agreements []AgreementRef
	if raw := c.PostForm("agreements"); raw != "" && json.Unmarshal([]byte(raw), &agreements) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	files := form.File["files"]
	if len(files) == 0 {
		files = form.File["file"]
	}
	if len(files) < 1 || len(files) > settingInt("AI_IMPORT_MAX_PAGES", 100) {
		fail(c, 400, "ai_invalid_upload")
		return
	}
	requestID := c.PostForm("request_id")
	if !validContentRequestID(requestID) {
		fail(c, 400, "invalid_client_request_id")
		return
	}
	who := actor(c)
	var prior AIImportTask
	if a.DB.Where("owner_id=? AND request_id=?", who.ID, requestID).First(&prior).Error == nil {
		c.JSON(200, gin.H{"task": prior, "duplicate": true})
		return
	}
	type upload struct {
		file  *multipart.FileHeader
		data  []byte
		ext   string
		pages int
	}
	inputs := []upload{}
	var totalBytes int64
	pages := 0
	hash := sha256.New()
	for _, f := range files {
		totalBytes += f.Size
		if f.Size <= 0 || totalBytes > limit {
			fail(c, 400, "file_too_large")
			return
		}
		src, err := f.Open()
		if err != nil {
			fail(c, 400, "ai_invalid_upload")
			return
		}
		b, err := io.ReadAll(io.LimitReader(src, limit+1))
		src.Close()
		if err != nil || int64(len(b)) != f.Size {
			fail(c, 400, "ai_invalid_upload")
			return
		}
		ext := strings.ToLower(filepath.Ext(f.Filename))
		count := 1
		if ext == ".pdf" {
			if len(files) != 1 || !bytes.HasPrefix(b, []byte("%PDF-")) {
				fail(c, 400, "invalid_pdf")
				return
			}
			if !aiToolsReady() {
				fail(c, 503, "ai_pdf_tools_required")
				return
			}
			checkCtx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
			count, err = aiPDFCount(checkCtx, b)
			cancel()
			if err != nil {
				dbFail(c, serviceFailure{"invalid_pdf", 400})
				return
			}
		} else if ext == ".jpg" || ext == ".jpeg" || ext == ".png" {
			if f.Size > int64(a.uploadConfig().ImageMiB)<<20 {
				fail(c, 400, "file_too_large")
				return
			}
			if _, _, err = aiImageDimensions(b); err != nil {
				fail(c, 400, "invalid_image")
				return
			}
			_, format, decodeErr := image.Decode(bytes.NewReader(b))
			if decodeErr != nil || (ext == ".png" && format != "png") || (ext != ".png" && format != "jpeg") {
				fail(c, 400, "invalid_image")
				return
			}
		} else {
			fail(c, 400, "unsupported_file")
			return
		}
		fmt.Fprintf(hash, "%d:", len(b))
		hash.Write(b)
		pages += count
		inputs = append(inputs, upload{f, b, ext, count})
	}
	if pages > settingInt("AI_IMPORT_MAX_PAGES", 100) {
		fail(c, 400, "ai_page_limit")
		return
	}
	fingerprint := fmt.Sprintf("%x", hash.Sum(nil))
	if c.PostForm("mode") != "update" && a.DB.Where("owner_id=? AND file_hash=?", who.ID, fingerprint).Order("id DESC").First(&prior).Error == nil {
		c.JSON(200, gin.H{"task": prior, "duplicate": true})
		return
	}
	// Persist private originals first; compensate only these new objects on a race/failure.
	assets := []AIImportAsset{}
	persisted := false
	defer func() {
		if !persisted {
			for _, v := range assets {
				var m Media
				a.DB.First(&m, v.MediaID)
				a.DB.Delete(&v)
				a.DB.Where("media_id=?", v.MediaID).Delete(&MediaMetadata{})
				a.DB.Delete(&m)
				a.cleanupNewObjects(m.Path)
			}
		}
	}()
	for i, in := range inputs {
		asset, err := a.aiSaveAsset(c.Request.Context(), who.ID, 0, 0, i+1, "original", in.ext, in.data, nil)
		if err != nil {
			dbFail(c, err)
			return
		}
		assets = append(assets, asset)
	}
	task := AIImportTask{ModelMode: modelMode, MaxConfirmedAt: maxConfirmed, ExtractionModel: cfg.Model, ReviewModel: cfg.ReviewModel, OwnerID: who.ID, RequestID: requestID, FileHash: fingerprint, FileName: inputs[0].file.Filename, Status: "queued", Stage: "queued", Total: pages, Checkpoint: "{}", PublishManifest: "[]"}
	ids := []uint64{}
	for _, v := range assets {
		ids = append(ids, v.ID)
	}
	task.SourceIDs = aiJSON(ids)
	e = a.DB.Transaction(func(tx *gorm.DB) error {
		var account Account
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&account, who.ID).Error; e != nil {
			return e
		}
		var active int64
		if e := tx.Model(&AIImportTask{}).Where("owner_id=? AND status IN ('queued','processing')", who.ID).Count(&active).Error; e != nil {
			return e
		}
		if active >= int64(settingInt("AI_IMPORT_OWNER_CONCURRENCY", 1)) {
			return serviceFailure{"ai_task_busy", 409}
		}
		var vendor Content
		if tx.Where("owner_id=? AND kind='vendor'", who.ID).First(&vendor).Error == nil {
			task.VendorID = vendor.ID
		}
		if e := acceptServiceDocuments(tx, who.ID, agreements, "web_use", []string{"terms", "privacy", "showroom-agreement"}); e != nil {
			return e
		}
		if e := tx.Create(&task).Error; e != nil {
			return e
		}
		return tx.Model(&AIImportAsset{}).Where("id IN ? AND owner_id=? AND task_id=0", ids, who.ID).Update("task_id", task.ID).Error
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	persisted = true
	c.JSON(202, gin.H{"task": task, "duplicate": false})
}
func aiDraftView(d AIImportDraft) gin.H {
	var p Payload
	json.Unmarshal([]byte(d.Data), &p)
	normalizeProductDetails(p.ProductDetails)
	var pages []int
	json.Unmarshal([]byte(d.SourcePages), &pages)
	var issues []string
	json.Unmarshal([]byte(d.ReviewIssues), &issues)
	var sources map[string][]int
	json.Unmarshal([]byte(d.FieldSources), &sources)
	var fields []string
	json.Unmarshal([]byte(d.SelectedFields), &fields)
	return gin.H{"id": d.ID, "task_id": d.TaskID, "temp_id": d.TempID, "active_batch_id": d.ActiveBatchID, "version": d.Version, "reviewed_version": d.ReviewedVersion, "data": p, "source_pages": pages, "field_sources": sources, "review_issues": issues, "selected_fields": fields, "base_revision_id": d.BaseRevisionID, "content_id": d.ContentID, "revision_id": d.RevisionID, "error": d.Error, "target_content_id": d.TargetContentID, "publish_mode": d.PublishMode}
}
func (a *App) aiResult(c *gin.Context) {
	t, e := a.aiOwnedTask(c)
	if e != nil {
		dbFail(c, e)
		return
	}
	companies := []AIImportDraft{}
	products := []AIImportDraft{}
	assets := []AIImportAsset{}
	for _, v := range []struct {
		table string
		out   any
	}{{aiTable("company"), &companies}, {aiTable("product"), &products}} {
		if e = a.DB.Table(v.table).Where("task_id=? AND owner_id=? AND deleted=0", t.ID, t.OwnerID).Order("id").Find(v.out).Error; e != nil {
			dbFail(c, e)
			return
		}
	}
	if e = a.DB.Where("task_id=? AND owner_id=?", t.ID, t.OwnerID).Order("id").Find(&assets).Error; e != nil {
		dbFail(c, e)
		return
	}
	views := []gin.H{}
	for _, d := range products {
		v := aiDraftView(d)
		v["blocking_reasons"] = aiBatchBlockers(d, "product", true)
		if d.TargetContentID > 0 {
			var target Content
			var rev Revision
			if a.DB.Where("id=? AND owner_id=? AND kind='product'", d.TargetContentID, t.OwnerID).First(&target).Error == nil && a.DB.First(&rev, target.DraftRevisionID).Error == nil {
				v["target_product"] = a.privateView(target, rev)
			}
		}
		if d.ContentID > 0 {
			var ct Content
			var rev Revision
			if a.DB.First(&ct, d.ContentID).Error == nil && a.DB.First(&rev, ct.DraftRevisionID).Error == nil {
				v["business_status"] = rev.Status
				v["public_visible"] = a.privateView(ct, rev).PublicVisible
			}
		}
		views = append(views, v)
	}
	var company any
	if len(companies) > 0 {
		v := aiDraftView(companies[0])
		v["blocking_reasons"] = aiBatchBlockers(companies[0], "company", t.VendorID > 0)
		company = v
	}
	var existing any
	if t.VendorID > 0 {
		var ct Content
		var rev Revision
		if a.DB.First(&ct, "id=? AND owner_id=?", t.VendorID, t.OwnerID).Error == nil && a.DB.First(&rev, ct.DraftRevisionID).Error == nil {
			existing = a.privateView(ct, rev)
		}
	}
	calls := []AIImportModelCall{}
	if e = a.DB.Where("task_id=? AND error<>?", t.ID, "bailian_attempts_recorded").Find(&calls).Error; e != nil {
		dbFail(c, e)
		return
	}
	usage := []gin.H{}
	for _, call := range calls {
		usage = append(usage, gin.H{"stage": call.Stage, "model": call.Model, "request_id": call.RequestID, "error": call.Error, "usage": json.RawMessage(call.UsageJSON), "estimated_cost": call.EstimatedCost, "price_basis": call.PriceBasis})
	}
	batches := []AISubmissionBatch{}
	batchItems := []AISubmissionItem{}
	if e = a.DB.Where("task_id=? AND owner_id=?", t.ID, t.OwnerID).Order("id DESC").Find(&batches).Error; e != nil {
		dbFail(c, e)
		return
	}
	if e = a.DB.Where("task_id=? AND owner_id=?", t.ID, t.OwnerID).Order("id").Find(&batchItems).Error; e != nil {
		dbFail(c, e)
		return
	}
	submitted, failed, visible := 0, 0, 0
	for _, v := range views {
		if v["content_id"].(uint64) > 0 {
			submitted++
		}
		if v["error"] != "" {
			failed++
		}
		if v["public_visible"] == true {
			visible++
		}
	}
	c.JSON(200, gin.H{"task": t, "company": company, "existing_company": existing, "publish_request": json.RawMessage(taskManifest(t)), "products": views, "assets": assets, "calls": usage, "batches": batches, "batch_items": batchItems, "submission_stats": gin.H{"total": len(views), "submitted": submitted, "unsubmitted": len(views) - submitted, "failed": failed, "public": visible}})
}
func (a *App) aiRetry(c *gin.Context) {
	t, e := a.aiOwnedTask(c)
	if e != nil {
		dbFail(c, e)
		return
	}
	e = a.DB.Transaction(func(tx *gorm.DB) error {
		var account Account
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&account, t.OwnerID).Error; e != nil {
			return e
		}
		var n int64
		tx.Model(&AIImportTask{}).Where("owner_id=? AND status IN ('queued','processing')", t.OwnerID).Count(&n)
		if n > 0 {
			return serviceFailure{"ai_task_busy", 409}
		}
		r := tx.Model(&AIImportTask{}).Where("id=? AND owner_id=? AND status='failed' AND publish_key=''", t.ID, t.OwnerID).Updates(map[string]any{"status": "queued", "error": "", "lease": "", "locked_until": nil})
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return serviceFailure{"ai_task_locked", 409}
		}
		return nil
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func aiEditable(tx *gorm.DB, taskID, owner uint64) error {
	var t AIImportTask
	if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND owner_id=?", taskID, owner).First(&t).Error; e != nil {
		return e
	}
	if (t.Status != "review_pending" && !(t.Status == "completed" && t.SubmissionMode == "batch")) || (t.PublishKey != "" && t.SubmissionMode != "batch") {
		return serviceFailure{"ai_task_locked", 409}
	}
	return nil
}
func (a *App) aiSaveDraft(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			PublishMode      string   `json:"publish_mode"`
			TargetContentID  *uint64  `json:"target_content_id"`
			TargetRevisionID *uint64  `json:"target_revision_id"`
			Version          uint64   `json:"version"`
			Data             Payload  `json:"data"`
			SelectedFields   []string `json:"selected_fields"`
		}
		if c.ShouldBindJSON(&req) != nil {
			fail(c, 400, "invalid_request")
			return
		}
		var row AIImportDraft
		e := a.DB.Transaction(func(tx *gorm.DB) error {
			var e error
			row, e = aiEditableDraft(tx, kind, id(c), actor(c).ID)
			if e != nil {
				return e
			}
			if row.Version != req.Version || row.ContentID > 0 {
				return serviceFailure{"stale_revision", 409}
			}
			businessKind := "product"
			if kind == "company" {
				businessKind = "vendor"
			}
			if e := a.validateDraftPayload(tx, actor(c), businessKind, &req.Data); e != nil {
				return e
			}
			if e := a.aiValidateTaskMedia(tx, row.TaskID, req.Data); e != nil {
				return e
			}
			updates := map[string]any{"data": aiJSON(req.Data), "selected_fields": aiJSON(req.SelectedFields), "version": row.Version + 1, "reviewed_version": 0, "error": "", "updated_at": time.Now().UTC()}
			if kind == "product" {
				mode := req.PublishMode
				if mode == "" {
					mode = row.PublishMode
				}
				if mode == "" {
					mode = "create"
				}
				if mode != "create" && mode != "update" && mode != "undecided" {
					return serviceFailure{"invalid_request", 400}
				}
				target := row.TargetContentID
				baseRevision := row.BaseRevisionID
				if req.TargetContentID != nil {
					target = *req.TargetContentID
				}
				if target > 0 {
					var targetContent Content
					if e := tx.Where("id=? AND owner_id=? AND kind='product'", target, actor(c).ID).First(&targetContent).Error; e != nil {
						return e
					}
					if req.TargetRevisionID != nil {
						if *req.TargetRevisionID != targetContent.DraftRevisionID {
							return serviceFailure{"stale_revision", 409}
						}
						baseRevision = *req.TargetRevisionID
					} else if target != row.TargetContentID {
						return serviceFailure{"stale_revision", 409}
					}
				}
				if mode == "update" && target == 0 {
					return serviceFailure{"ai_update_choice_required", 409}
				}
				updates["target_content_id"] = target
				updates["publish_mode"] = mode
				updates["base_revision_id"] = baseRevision
			}
			return tx.Table(aiTable(kind)).Where("id=? AND version=?", row.ID, req.Version).Updates(updates).Error
		})
		if e != nil {
			dbFail(c, e)
			return
		}
		c.JSON(200, gin.H{"ok": true, "version": row.Version + 1})
	}
}
func (a *App) aiValidateTaskMedia(tx *gorm.DB, task uint64, p Payload) error {
	var t AIImportTask
	if e := tx.First(&t, task).Error; e != nil {
		return e
	}
	if p.ProductDetails != nil {
		if !aiPagesValid(p.ProductDetails.SourcePages, t.Total) {
			return serviceFailure{"ai_invalid_source_page", 400}
		}
		for _, m := range p.ProductDetails.Models {
			for _, v := range m.Parameters {
				if v.SourcePage < 1 || v.SourcePage > t.Total {
					return serviceFailure{"ai_invalid_source_page", 400}
				}
			}
		}
		for _, im := range p.ProductDetails.Images {
			if im.SourcePage < 1 || im.SourcePage > t.Total {
				return serviceFailure{"ai_invalid_source_page", 400}
			}
		}
	}
	for _, mid := range payloadMedia(p) {
		var n int64
		if e := tx.Model(&AIImportAsset{}).Where("media_id=? AND task_id<>?", mid, task).Count(&n).Error; e != nil {
			return e
		}
		if n > 0 {
			return serviceFailure{"ai_asset_wrong_task", 400}
		}
	}
	return nil
}
func (a *App) aiReview(c *gin.Context) {
	var req struct {
		Items []struct {
			Kind       string `json:"kind"`
			ID         uint64 `json:"id"`
			Version    uint64 `json:"version"`
			Resolution string `json:"resolution"`
		} `json:"items"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.Items) == 0 {
		fail(c, 400, "invalid_request")
		return
	}
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		if e := aiEditable(tx, id(c), actor(c).ID); e != nil {
			return e
		}
		for _, v := range req.Items {
			if v.Kind != "company" && v.Kind != "product" {
				return serviceFailure{"invalid_request", 400}
			}
			var d AIImportDraft
			if e := tx.Table(aiTable(v.Kind)).Where("id=? AND task_id=? AND owner_id=? AND deleted=0", v.ID, id(c), actor(c).ID).First(&d).Error; e != nil {
				return e
			}
			if d.Version != v.Version {
				return serviceFailure{"stale_revision", 409}
			}
			var issues []string
			json.Unmarshal([]byte(d.ReviewIssues), &issues)
			var p Payload
			if json.Unmarshal([]byte(d.Data), &p) != nil {
				return serviceFailure{"invalid_request", 400}
			}
			required := len(issues) > 0
			if p.ProductDetails != nil {
				for _, m := range p.ProductDetails.Models {
					for _, par := range m.Parameters {
						required = required || par.ReviewRequired
					}
				}
			}
			if required && strings.TrimSpace(v.Resolution) == "" {
				return serviceFailure{"ai_resolution_required", 400}
			}
			if len(v.Resolution) > 8000 {
				return serviceFailure{"invalid_request", 400}
			}
			record := AIImportReviewItem{TaskID: d.TaskID, OwnerID: d.OwnerID, DraftID: d.ID, DraftKind: v.Kind, DraftVersion: d.Version, Issue: d.ReviewIssues, Resolution: v.Resolution, ActorID: actor(c).ID, CreatedAt: time.Now().UTC()}
			if e := tx.Create(&record).Error; e != nil {
				return e
			}
			if e := tx.Table(aiTable(v.Kind)).Where("id=?", d.ID).Updates(map[string]any{"reviewed_version": d.Version, "data": aiJSON(aiReviewedPayload(p)), "updated_at": time.Now().UTC()}).Error; e != nil {
				return e
			}
		}
		return nil
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func (a *App) aiDeleteDraft(c *gin.Context) {
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		var d AIImportDraft
		var e error
		d, e = aiEditableDraft(tx, "product", id(c), actor(c).ID)
		if e != nil {
			return e
		}
		if d.ContentID > 0 {
			return serviceFailure{"ai_task_locked", 409}
		}
		return tx.Table(aiTable("product")).Where("id=?", d.ID).Update("deleted", true).Error
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

type aiPublishSelection struct {
	ID      uint64 `json:"id"`
	Version uint64 `json:"version"`
}

func (a *App) aiPublish(c *gin.Context) {
	var req struct {
		Key            string               `json:"idempotency_key"`
		CompanyVersion uint64               `json:"company_version"`
		Products       []aiPublishSelection `json:"products"`
	}
	if c.ShouldBindJSON(&req) != nil || !validContentRequestID(req.Key) || len(req.Products) == 0 || len(req.Products) > 1000 {
		fail(c, 400, "invalid_request")
		return
	}
	sort.Slice(req.Products, func(i, j int) bool { return req.Products[i].ID < req.Products[j].ID })
	for i, v := range req.Products {
		if v.ID == 0 || (i > 0 && req.Products[i-1].ID == v.ID) {
			fail(c, 400, "invalid_request")
			return
		}
	}
	manifest := aiJSON(req)
	who := actor(c)
	var task AIImportTask
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND owner_id=?", id(c), who.ID).First(&task).Error; e != nil {
			return e
		}
		if task.PublishKey != "" {
			if task.PublishKey != req.Key || task.PublishManifest != manifest {
				return serviceFailure{"ai_publish_conflict", 409}
			}
			return nil
		}
		if task.Status != "review_pending" {
			return serviceFailure{"ai_task_locked", 409}
		}
		var company AIImportDraft
		if e := tx.Table(aiTable("company")).Where("task_id=? AND owner_id=?", task.ID, who.ID).First(&company).Error; e != nil {
			return e
		}
		if company.Version != req.CompanyVersion || company.ReviewedVersion != company.Version {
			return serviceFailure{"ai_review_required", 409}
		}
		if e := a.aiValidateReady(c.Request.Context(), tx, who, task, company, "company"); e != nil {
			return e
		}
		for _, v := range req.Products {
			var d AIImportDraft
			if e := tx.Table(aiTable("product")).Where("id=? AND task_id=? AND owner_id=? AND deleted=0", v.ID, task.ID, who.ID).First(&d).Error; e != nil {
				return e
			}
			if d.Version != v.Version || d.ReviewedVersion != d.Version {
				return serviceFailure{"ai_review_required", 409}
			}
			if e := a.aiValidateReady(c.Request.Context(), tx, who, task, d, "product"); e != nil {
				return e
			}
		}
		return tx.Model(&task).Updates(map[string]any{"publish_key": req.Key, "publish_manifest": manifest}).Error
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	// Each item and its final ID are committed together; retry never duplicates content.
	companyErr := a.aiPublishCompany(task, who, req.CompanyVersion)
	success, failed := 0, 0
	results := []gin.H{}
	for _, v := range req.Products {
		err := companyErr
		if err == nil {
			err = a.aiPublishProduct(task, who, v)
		}
		code := ""
		if err != nil {
			failed++
			code = aiSafeError(err)
			a.DB.Table(aiTable("product")).Where("id=? AND task_id=?", v.ID, task.ID).Update("error", code)
		} else {
			success++
		}
		var d AIImportDraft
		a.DB.Table(aiTable("product")).First(&d, v.ID)
		results = append(results, gin.H{"id": v.ID, "content_id": d.ContentID, "revision_id": d.RevisionID, "error": code})
	}
	if failed == 0 {
		a.DB.Model(&AIImportTask{}).Where("id=?", task.ID).Updates(map[string]any{"status": "completed", "stage": "submitted", "error": ""})
	}
	c.JSON(200, gin.H{"submitted": success, "failed": failed, "results": results, "message": "platform_review_required"})
}
func aiSafeError(e error) string {
	var sf serviceFailure
	if errors.As(e, &sf) {
		return sf.code
	}
	var vf *FieldValidationError
	if errors.As(e, &vf) {
		return vf.Code
	}
	switch e.Error() {
	case "pending_review", "stale_revision", "storage_quota_exceeded", "disk_space_insufficient", "product_limit_reached", "invalid_category", "invalid_media", "invalid_request", "vendor_required":
		return e.Error()
	}
	if strings.HasPrefix(e.Error(), "bailian_") || strings.HasPrefix(e.Error(), "ai_") {
		return e.Error()
	}
	return "ai_processing_failed"
}
func (a *App) aiPublishCompany(task AIImportTask, who Account, version uint64) error {
	return a.contentTransaction(func(tx *gorm.DB) error { return a.aiPublishCompanyTx(tx, task, who, version, 0) })
}
func (a *App) aiPublishCompanyTx(tx *gorm.DB, task AIImportTask, who Account, version uint64, batchID uint64) error {
	var d AIImportDraft
	if e := tx.Table(aiTable("company")).Clauses(clause.Locking{Strength: "UPDATE"}).Where("task_id=? AND owner_id=?", task.ID, who.ID).First(&d).Error; e != nil {
		return e
	}
	if d.ActiveBatchID != 0 && d.ActiveBatchID != batchID {
		return serviceFailure{"ai_draft_submitting", 409}
	}
	if d.ContentID > 0 {
		return nil
	}
	if d.Version != version || d.ReviewedVersion != version {
		return serviceFailure{"ai_review_required", 409}
	}
	var p Payload
	if e := json.Unmarshal([]byte(d.Data), &p); e != nil {
		return e
	}
	var content Content
	e := tx.Where("kind='vendor' AND owner_id=?", who.ID).First(&content).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		content = Content{Kind: "vendor", OwnerID: who.ID, HallID: 1}
		if e = a.createOwnedDraft(tx, &content, who, &p); e != nil {
			return e
		}
	} else if e != nil {
		return e
	} else {
		var fields []string
		json.Unmarshal([]byte(d.SelectedFields), &fields)
		if len(fields) == 0 {
			if e := tx.Model(&AIImportTask{}).Where("id=?", task.ID).Update("vendor_id", content.ID).Error; e != nil {
				return e
			}
			return tx.Table(aiTable("company")).Where("id=?", d.ID).Updates(map[string]any{"content_id": content.ID, "revision_id": content.DraftRevisionID, "error": ""}).Error
		}
		if content.DraftRevisionID != d.BaseRevisionID {
			return serviceFailure{"stale_revision", 409}
		}
		var rev Revision
		if e = tx.First(&rev, content.DraftRevisionID).Error; e != nil {
			return e
		}
		var old Payload
		if e = json.Unmarshal([]byte(rev.Payload), &old); e != nil {
			return e
		}
		if rev.Status == "pending" {
			return serviceFailure{"pending_review", 409}
		}
		merged, e := aiMergeCompany(old, p, fields)
		if e != nil {
			return e
		}
		p = merged
		if e = a.saveOwnedDraft(tx, &content, who, content.ID, d.BaseRevisionID, &p); e != nil {
			return e
		}
	}
	if e = a.submitOwnedDraft(tx, who, content.ID, content.DraftRevisionID); e != nil {
		return e
	}
	if e = tx.Model(&AIImportTask{}).Where("id=? AND owner_id=?", task.ID, who.ID).Update("vendor_id", content.ID).Error; e != nil {
		return e
	}
	return tx.Table(aiTable("company")).Where("id=?", d.ID).Updates(map[string]any{"content_id": content.ID, "revision_id": content.DraftRevisionID, "error": ""}).Error
}
func aiMergeCompany(old, next Payload, fields []string) (Payload, error) {
	allowed := map[string]bool{"name": true, "name_en": true, "description": true, "description_en": true, "main_products": true, "phone": true, "wechat": true, "address": true, "region_code": true, "region_name": true, "region_id": true, "primary_category_id": true, "secondary_category_ids": true, "business_nature": true, "company_profile": true, "media_ids": true, "keywords": true}
	var before, after map[string]json.RawMessage
	json.Unmarshal([]byte(aiJSON(old)), &before)
	json.Unmarshal([]byte(aiJSON(next)), &after)
	for _, field := range fields {
		if strings.HasPrefix(field, "company_profile.") {
			key := strings.TrimPrefix(field, "company_profile.")
			valid := map[string]bool{"logo_media_id": true, "cover_media_id": true, "short_name": true, "short_name_en": true, "summary": true, "summary_en": true, "established": true, "staff": true, "factory_area": true, "equipment": true, "capacity": true, "website": true, "contact_name": true, "contacts": true}
			if !valid[key] {
				return old, serviceFailure{"invalid_request", 400}
			}
			var a, b map[string]json.RawMessage
			json.Unmarshal(before["company_profile"], &a)
			json.Unmarshal(after["company_profile"], &b)
			if a == nil {
				a = map[string]json.RawMessage{}
			}
			if v, ok := b[key]; ok {
				a[key] = v
			} else {
				a[key] = json.RawMessage("null")
			}
			before["company_profile"] = json.RawMessage(aiJSON(a))
			continue
		}
		if !allowed[field] {
			return old, serviceFailure{"invalid_request", 400}
		}
		before[field] = after[field]
	}
	var result Payload
	e := json.Unmarshal([]byte(aiJSON(before)), &result)
	return result, e
}
func (a *App) aiPublishProduct(task AIImportTask, who Account, v aiPublishSelection) error {
	return a.contentTransaction(func(tx *gorm.DB) error { return a.aiPublishProductTx(tx, task, who, v, 0) })
}
func (a *App) aiPublishProductTx(tx *gorm.DB, task AIImportTask, who Account, v aiPublishSelection, batchID uint64) error {
	var d AIImportDraft
	if e := tx.Table(aiTable("product")).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND task_id=? AND owner_id=? AND deleted=0", v.ID, task.ID, who.ID).First(&d).Error; e != nil {
		return e
	}
	if d.ActiveBatchID != 0 && d.ActiveBatchID != batchID {
		return serviceFailure{"ai_draft_submitting", 409}
	}
	if d.ContentID > 0 {
		return nil
	}
	if d.Version != v.Version || d.ReviewedVersion != d.Version {
		return serviceFailure{"ai_review_required", 409}
	}
	var p Payload
	if e := json.Unmarshal([]byte(d.Data), &p); e != nil {
		return e
	}
	if e := a.aiValidateTaskMedia(tx, task.ID, p); e != nil {
		return e
	}
	content := Content{Kind: "product", OwnerID: who.ID}
	if d.PublishMode == "undecided" {
		return serviceFailure{"ai_update_choice_required", 409}
	}
	if d.PublishMode == "update" {
		if e := tx.Where("id=? AND owner_id=? AND kind='product'", d.TargetContentID, who.ID).First(&content).Error; e != nil {
			return e
		}
		if e := a.saveOwnedDraft(tx, &content, who, content.ID, d.BaseRevisionID, &p); e != nil {
			return e
		}
	} else {
		if e := a.createOwnedDraft(tx, &content, who, &p); e != nil {
			return e
		}
	}

	if e := a.submitOwnedDraft(tx, who, content.ID, content.DraftRevisionID); e != nil {
		return e
	}
	return tx.Table(aiTable("product")).Where("id=?", d.ID).Updates(map[string]any{"content_id": content.ID, "revision_id": content.DraftRevisionID, "error": ""}).Error
}

// Merge/split operate only on import drafts; they never delete business products.
func (a *App) aiMerge(c *gin.Context) {
	var req struct {
		Products []aiPublishSelection `json:"products"`
		Name     string               `json:"name"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.Products) < 2 || strings.TrimSpace(req.Name) == "" {
		fail(c, 400, "invalid_request")
		return
	}
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		if e := aiEditable(tx, id(c), actor(c).ID); e != nil {
			return e
		}
		rows := []AIImportDraft{}
		seen := map[uint64]bool{}
		for _, v := range req.Products {
			if seen[v.ID] {
				return serviceFailure{"invalid_request", 400}
			}
			seen[v.ID] = true
			var d AIImportDraft
			if e := tx.Table(aiTable("product")).Where("id=? AND task_id=? AND owner_id=? AND deleted=0 AND content_id=0", v.ID, id(c), actor(c).ID).First(&d).Error; e != nil {
				return e
			}
			if d.Version != v.Version {
				return serviceFailure{"stale_revision", 409}
			}
			rows = append(rows, d)
		}
		var p Payload
		json.Unmarshal([]byte(rows[0].Data), &p)
		p.Name = req.Name
		if p.ProductDetails == nil {
			p.ProductDetails = &ProductDetails{}
		}
		pages := map[int]bool{}
		for i, d := range rows {
			var q Payload
			json.Unmarshal([]byte(d.Data), &q)
			var source []int
			json.Unmarshal([]byte(d.SourcePages), &source)
			for _, n := range source {
				pages[n] = true
			}
			if i > 0 {
				p.Description += "\n" + q.Description
				p.Parameters += "\n" + q.Parameters
				p.MediaIDs = append(p.MediaIDs, q.MediaIDs...)
				if q.ProductDetails != nil {
					p.ProductDetails.Models = append(p.ProductDetails.Models, q.ProductDetails.Models...)
					p.ProductDetails.Images = append(p.ProductDetails.Images, q.ProductDetails.Images...)
					p.ProductDetails.Features = append(p.ProductDetails.Features, q.ProductDetails.Features...)
					p.ProductDetails.Applications = append(p.ProductDetails.Applications, q.ProductDetails.Applications...)
				}
			}
		}
		p.MediaIDs = aiUniqueMedia(p.MediaIDs)
		if len(p.MediaIDs) > 12 {
			return serviceFailure{"ai_photo_limit", 400}
		}
		list := []int{}
		for n := range pages {
			list = append(list, n)
		}
		sort.Ints(list)
		p.ProductDetails.SourcePages = list
		d := AIImportDraft{TaskID: id(c), OwnerID: actor(c).ID, TempID: "merge-" + randomToken(), Version: 1, Data: aiJSON(p), SourcePages: aiJSON(list), FieldSources: "{}", ReviewIssues: aiJSON([]string{"合并后的产品、型号和图片关联需重新核对"}), SelectedFields: "[]"}
		if e := tx.Table(aiTable("product")).Create(&d).Error; e != nil {
			return e
		}
		for _, r := range rows {
			if e := tx.Table(aiTable("product")).Where("id=?", r.ID).Update("deleted", true).Error; e != nil {
				return e
			}
		}
		return nil
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func aiUniqueMedia(ids []uint64) []uint64 {
	seen := map[uint64]bool{}
	out := []uint64{}
	for _, v := range ids {
		if !seen[v] {
			out = append(out, v)
			seen[v] = true
		}
	}
	return out
}
func (a *App) aiSplit(c *gin.Context) {
	var req struct {
		Version  uint64    `json:"version"`
		Products []Payload `json:"products"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.Products) < 2 || len(req.Products) > 100 {
		fail(c, 400, "invalid_request")
		return
	}
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		var row AIImportDraft
		var e error
		row, e = aiEditableDraft(tx, "product", id(c), actor(c).ID)
		if e != nil {
			return e
		}
		if row.ContentID > 0 {
			return serviceFailure{"ai_task_locked", 409}
		}
		if row.Version != req.Version {
			return serviceFailure{"stale_revision", 409}
		}
		var original Payload
		json.Unmarshal([]byte(row.Data), &original)
		if original.ProductDetails != nil {
			kept := map[string]bool{}
			for _, p := range req.Products {
				if p.ProductDetails != nil {
					for _, m := range p.ProductDetails.Models {
						kept[m.ModelName+"\x00"+m.Variant] = true
					}
				}
			}
			for _, m := range original.ProductDetails.Models {
				if !kept[m.ModelName+"\x00"+m.Variant] {
					return serviceFailure{"ai_split_model_missing", 400}
				}
			}
		}
		for _, p := range req.Products {
			if e := a.validateDraftPayload(tx, actor(c), "product", &p); e != nil {
				return e
			}
			if e := a.aiValidateTaskMedia(tx, row.TaskID, p); e != nil {
				return e
			}
			d := AIImportDraft{TaskID: row.TaskID, OwnerID: row.OwnerID, TempID: "split-" + randomToken(), Version: 1, Data: aiJSON(p), SourcePages: row.SourcePages, FieldSources: row.FieldSources, ReviewIssues: aiJSON([]string{"拆分后的型号、参数和图片归属需重新核对"}), SelectedFields: "[]"}
			if e := tx.Table(aiTable("product")).Create(&d).Error; e != nil {
				return e
			}
		}
		return tx.Table(aiTable("product")).Where("id=?", row.ID).Update("deleted", true).Error
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func taskManifest(t AIImportTask) string {
	if t.PublishKey == "" {
		return "null"
	}
	return t.PublishManifest
}
