package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Originals have already passed content validation. Reuse their media IDs and
// bytes instead of charging a second private copy for the AI task.
func (a *App) aiCreateTaskFromCloudFiles(c *gin.Context) {
	var req struct {
		FileIDs    []uint64       `json:"file_ids"`
		RequestID  string         `json:"request_id"`
		Mode       string         `json:"mode"`
		Agreements []AgreementRef `json:"agreements"`
	}
	if c.ShouldBindJSON(&req) != nil || !validContentRequestID(req.RequestID) || len(req.FileIDs) < 1 || len(req.FileIDs) > settingInt("AI_IMPORT_MAX_PAGES", 100) {
		fail(c, 400, "ai_invalid_upload")
		return
	}
	who := actor(c)
	var prior AIImportTask
	if a.DB.Where("owner_id=? AND request_id=?", who.ID, req.RequestID).First(&prior).Error == nil {
		c.JSON(200, gin.H{"task": prior, "duplicate": true})
		return
	}
	cfg, _, e := a.aiBailianConfig()
	if e != nil {
		fail(c, 503, aiSafeError(e))
		return
	}
	hash := sha256.New()
	var total int64
	pages := 0
	name := ""
	assets := []AIImportAsset{}
	seen := map[uint64]bool{}
	for i, mid := range req.FileIDs {
		if seen[mid] {
			fail(c, 400, "ai_invalid_upload")
			return
		}
		seen[mid] = true
		var m Media
		var f EnterpriseFile
		var meta MediaMetadata
		if a.DB.Where("id=? AND owner_id=?", mid, who.ID).First(&m).Error != nil || a.DB.Where("media_id=? AND owner_id=? AND business_type='ai_original'", mid, who.ID).First(&f).Error != nil || a.DB.Where("media_id=? AND state='ready'", mid).First(&meta).Error != nil {
			fail(c, 404, "not_found")
			return
		}
		o, store, e := a.resolveObject(c.Request.Context(), m.Path)
		if e != nil {
			cloudFail(c, cloudError(e))
			return
		}
		total += o.Size
		if total > int64(settingInt("AI_IMPORT_MAX_FILE_BYTES", 30000000)) {
			fail(c, 400, "file_too_large")
			return
		}
		count := 1
		if meta.Kind == "pdf" {
			if len(req.FileIDs) != 1 {
				fail(c, 400, "ai_invalid_upload")
				return
			}
			if !aiToolsReady() {
				fail(c, 503, "ai_pdf_tools_required")
				return
			}
			count = meta.PageCount
		}
		pages += count
		if count < 1 || pages > settingInt("AI_IMPORT_MAX_PAGES", 100) {
			fail(c, 400, "ai_page_limit")
			return
		}
		r, e := store.Open(c.Request.Context(), m.Path)
		if e != nil {
			cloudFail(c, cloudError(e))
			return
		}
		fmt.Fprintf(hash, "%d:", o.Size)
		n, e := io.Copy(hash, io.LimitReader(r, o.Size+1))
		r.Close()
		if e != nil || n != o.Size {
			cloudFail(c, errors.New("file_size_mismatch"))
			return
		}
		if name == "" {
			name = f.OriginalFilename
		}
		assets = append(assets, AIImportAsset{OwnerID: who.ID, MediaID: mid, Page: i + 1, ImageType: "original", CropRegion: "null", SHA256: o.SHA256, Width: meta.Width, Height: meta.Height})
	}
	fingerprint := fmt.Sprintf("%x", hash.Sum(nil))
	if req.Mode != "update" && a.DB.Where("owner_id=? AND file_hash=?", who.ID, fingerprint).Order("id DESC").First(&prior).Error == nil {
		c.JSON(200, gin.H{"task": prior, "duplicate": true})
		return
	}
	task := AIImportTask{ModelMode: "admin", ExtractionModel: cfg.Model, ReviewModel: cfg.ReviewModel, OwnerID: who.ID, RequestID: req.RequestID, FileHash: fingerprint, FileName: filepath.Base(name), Status: "queued", Stage: "queued", Total: pages, Checkpoint: "{}", PublishManifest: "[]"}
	e = a.DB.Transaction(func(tx *gorm.DB) error {
		var acc Account
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&acc, who.ID).Error; e != nil {
			return e
		}
		var prior AIImportTask
		if tx.Where("owner_id=? AND request_id=?", who.ID, req.RequestID).First(&prior).Error == nil {
			task = prior
			return nil
		}
		var active int64
		if e := tx.Model(&AIImportTask{}).Where("owner_id=? AND status IN ('queued','processing')", who.ID).Count(&active).Error; e != nil {
			return e
		}
		if active >= int64(settingInt("AI_IMPORT_OWNER_CONCURRENCY", 1)) {
			return serviceFailure{"ai_task_busy", 409}
		}
		var v Content
		if tx.Where("owner_id=? AND kind='vendor'", who.ID).First(&v).Error == nil {
			task.VendorID = v.ID
		}
		if e := acceptServiceDocuments(tx, who.ID, req.Agreements, "web_use", []string{"terms", "privacy", "showroom-agreement"}); e != nil {
			return e
		}
		if e := tx.Create(&task).Error; e != nil {
			return e
		}
		ids := []uint64{}
		for _, asset := range assets {
			// Recheck ownership while holding the account lock to serialize deletion.
			var m Media
			if e := tx.Where("id=? AND owner_id=?", asset.MediaID, who.ID).First(&m).Error; e != nil {
				return e
			}
			asset.TaskID = task.ID
			if e := tx.Create(&asset).Error; e != nil {
				return e
			}
			ids = append(ids, asset.ID)
			if e := tx.Model(&EnterpriseFile{}).Where("media_id=?", m.ID).Updates(map[string]any{"business_id": task.ID, "enterprise_id": task.VendorID, "updated_at": time.Now().UTC()}).Error; e != nil {
				return e
			}
		}
		encoded, _ := json.Marshal(ids)
		task.SourceIDs = string(encoded)
		return tx.Model(&task).Update("source_ids", task.SourceIDs).Error
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(202, gin.H{"task": task, "duplicate": false})
}
