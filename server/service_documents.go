package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const freeStorageBytes int64 = 200000000
const serviceMode = "free_display"

//go:embed service-documents.json
var serviceDocumentFiles embed.FS

type ServiceDocument struct {
	ID          uint64     `json:"id"`
	DocKey      string     `json:"key"`
	Version     uint64     `json:"version"`
	Title       string     `json:"title"`
	TitleEN     string     `json:"title_en"`
	Body        string     `json:"body"`
	BodyEN      string     `json:"body_en"`
	Digest      string     `json:"digest"`
	Status      string     `json:"status"`
	EffectiveAt time.Time  `json:"effective_at"`
	UpdatedBy   uint64     `json:"-"`
	CreatedAt   time.Time  `json:"created_at"`
	PublishedAt *time.Time `json:"published_at"`
}
type ServiceAcceptance struct {
	ID         uint64    `json:"id"`
	AccountID  uint64    `json:"account_id"`
	DocumentID uint64    `json:"document_id"`
	Digest     string    `json:"digest"`
	Source     string    `json:"source"`
	AcceptedAt time.Time `json:"accepted_at"`
}
type AgreementRef struct {
	Key     string `json:"key"`
	Version uint64 `json:"version"`
	Digest  string `json:"digest"`
}
type serviceFailure struct {
	code   string
	status int
}

func (e serviceFailure) Error() string { return e.code }
func serviceInvalid() error            { return serviceFailure{"invalid_request", 400} }
func digestText(s string) string       { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:]) }
func documentDigest(d ServiceDocument) string {
	b, _ := json.Marshal([]any{d.DocKey, d.Version, d.Title, d.TitleEN, d.Body, d.BodyEN, d.EffectiveAt.UTC()})
	return digestText(string(b))
}

var serviceDocumentKeys = []string{"service-info", "showroom-agreement", "terms", "privacy"}

func validDocumentKey(key string) bool {
	for _, k := range serviceDocumentKeys {
		if key == k {
			return true
		}
	}
	return false
}
func serviceLock(tx *gorm.DB, write bool) error {
	strength := "SHARE"
	if write {
		strength = "UPDATE"
	}
	var row SystemSetting
	return tx.Clauses(clause.Locking{Strength: strength}).First(&row, "setting_key=?", "free_display_lock").Error
}

func migrateFreeDisplay(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC().Truncate(time.Millisecond)
		if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&SystemSetting{SettingKey: "free_display_lock", Value: "1", UpdatedAt: now}).Error; e != nil {
			return e
		}
		raw, e := serviceDocumentFiles.ReadFile("service-documents.json")
		if e != nil {
			return e
		}
		var docs []ServiceDocument
		if e = json.Unmarshal(raw, &docs); e != nil {
			return e
		}
		for _, d := range docs {
			d.Version = 1
			d.Status = "published"
			d.EffectiveAt = now
			d.CreatedAt = now
			d.PublishedAt = &now
			d.Digest = documentDigest(d)
			if e = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&d).Error; e != nil {
				return e
			}
		}
		var setting SystemSetting
		e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&setting, "setting_key=?", "system_config_v3").Error
		if e == nil {
			var v map[string]any
			if e = json.Unmarshal([]byte(setting.Value), &v); e != nil {
				return e
			}
			uploads, ok := v["uploads"].(map[string]any)
			if !ok {
				uploads = map[string]any{}
				v["uploads"] = uploads
			}
			uploads["default_quota_bytes"] = freeStorageBytes
			if n, ok := v["version"].(float64); ok {
				v["version"] = n + 1
			}
			blob, _ := json.Marshal(v)
			if e = tx.Model(&setting).Updates(map[string]any{"value": string(blob), "updated_at": now}).Error; e != nil {
				return e
			}
		} else if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		return tx.Exec("INSERT IGNORE INTO home_rotation_pool(vendor_id,hall_id,state,reason,updated_by,updated_at) SELECT id,hall_id,'approved','历史人工精选迁移',0,UTC_TIMESTAMP(3) FROM contents WHERE kind='vendor' AND featured=1").Error
	})
}
func activeServiceDocuments(tx *gorm.DB) ([]ServiceDocument, error) {
	docs := []ServiceDocument{}
	for _, key := range serviceDocumentKeys {
		var d ServiceDocument
		e := tx.Where("doc_key=? AND status='published' AND effective_at<=?", key, time.Now().UTC()).Order("version DESC").First(&d).Error
		if e != nil {
			return nil, e
		}
		docs = append(docs, d)
	}
	return docs, nil
}

func (a *App) serviceDocumentVersions() []gin.H {
	docs, e := activeServiceDocuments(a.DB)
	if e != nil {
		return nil
	}
	result := []gin.H{}
	for _, d := range docs {
		result = append(result, gin.H{"key": d.DocKey, "version": d.Version, "digest": d.Digest, "effective_at": d.EffectiveAt})
	}
	return result
}
func acceptServiceDocuments(tx *gorm.DB, owner uint64, refs []AgreementRef, source string, keys []string) error {
	if e := serviceLock(tx, false); e != nil {
		return e
	}
	docs, e := activeServiceDocuments(tx)
	if e != nil {
		return e
	}
	if source != "android" && source != "ios" {
		source = "web"
	}
	for _, key := range keys {
		var wanted ServiceDocument
		for _, d := range docs {
			if d.DocKey == key {
				wanted = d
			}
		}
		found := false
		for _, ref := range refs {
			if ref.Key == key && ref.Version == wanted.Version && ref.Digest == wanted.Digest {
				found = true
			}
		}
		if !found {
			return serviceFailure{"agreement_required", 409}
		}
		row := ServiceAcceptance{AccountID: owner, DocumentID: wanted.ID, Digest: wanted.Digest, Source: source, AcceptedAt: time.Now().UTC()}
		if e = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; e != nil {
			return e
		}
	}
	return nil
}
func publicationCompliance(tx *gorm.DB, owner, vendorID uint64, vendorPayload *Payload) error {
	if e := serviceLock(tx, false); e != nil {
		return e
	}
	docs, e := activeServiceDocuments(tx)
	if e != nil {
		return e
	}
	for _, d := range docs {
		if d.DocKey == "service-info" {
			continue
		}
		var n int64
		if e = tx.Model(&ServiceAcceptance{}).Where("account_id=? AND document_id=? AND digest=?", owner, d.ID, d.Digest).Count(&n).Error; e != nil {
			return e
		}
		if n == 0 {
			return serviceFailure{"agreement_required", 409}
		}
	}
	return nil
}
func (a *App) serviceDocumentRoutes(pub, merchant, admin *gin.RouterGroup) {
	pub.GET("/service-documents", func(c *gin.Context) {
		docs, e := activeServiceDocuments(a.DB)
		if e != nil {
			dbFail(c, e)
			return
		}
		c.JSON(200, gin.H{"items": docs, "service_mode": serviceMode})
	})
	pub.GET("/service-documents/:key/history", func(c *gin.Context) {
		if !validDocumentKey(c.Param("key")) {
			fail(c, 404, "not_found")
			return
		}
		docs := []ServiceDocument{}
		if e := a.DB.Where("doc_key=? AND status='published'", c.Param("key")).Order("version DESC").Find(&docs).Error; e != nil {
			dbFail(c, e)
			return
		}
		c.JSON(200, gin.H{"items": docs})
	})
	merchant.POST("/service-acceptances", func(c *gin.Context) {
		var req struct {
			Agreements []AgreementRef `json:"agreements"`
			Source     string         `json:"source"`
		}
		if c.ShouldBindJSON(&req) != nil {
			fail(c, 400, "invalid_request")
			return
		}
		e := a.DB.Transaction(func(tx *gorm.DB) error {
			return acceptServiceDocuments(tx, actor(c).ID, req.Agreements, req.Source, []string{"terms", "privacy", "showroom-agreement"})
		})
		if e != nil {
			dbFail(c, e)
			return
		}
		c.JSON(200, gin.H{"ok": true})
	})
	admin.GET("/service-documents", func(c *gin.Context) {
		docs := []ServiceDocument{}
		if e := a.DB.Order("doc_key,version DESC").Find(&docs).Error; e != nil {
			dbFail(c, e)
			return
		}
		c.JSON(200, gin.H{"items": docs})
	})
	admin.POST("/service-documents", a.saveServiceDocument)
	admin.PUT("/service-documents/:id", a.saveServiceDocument)
	admin.POST("/service-documents/:id/publish", func(c *gin.Context) {
		e := a.DB.Transaction(func(tx *gorm.DB) error {
			if e := serviceLock(tx, true); e != nil {
				return e
			}
			var d ServiceDocument
			if e := tx.First(&d, id(c)).Error; e != nil {
				return e
			}
			if d.Status != "draft" {
				return serviceFailure{"stale_revision", 409}
			}
			var newer int64
			if e := tx.Model(&ServiceDocument{}).Where("doc_key=? AND version>? AND status='published'", d.DocKey, d.Version).Count(&newer).Error; e != nil {
				return e
			}
			if newer > 0 {
				return serviceFailure{"stale_revision", 409}
			}
			now := time.Now().UTC()
			if e := tx.Model(&d).Updates(map[string]any{"status": "published", "published_at": now, "updated_by": actor(c).ID}).Error; e != nil {
				return e
			}
			return recordOperation(tx, actor(c).ID, "service_document", d.ID, "document_published", d.DocKey)
		})
		if e != nil {
			dbFail(c, e)
			return
		}
		c.JSON(200, gin.H{"ok": true})
	})
}
func (a *App) saveServiceDocument(c *gin.Context) {
	var req struct {
		Key            string    `json:"key"`
		Title          string    `json:"title"`
		TitleEN        string    `json:"title_en"`
		Body           string    `json:"body"`
		BodyEN         string    `json:"body_en"`
		EffectiveAt    time.Time `json:"effective_at"`
		ExpectedDigest string    `json:"expected_digest"`
	}
	if c.ShouldBindJSON(&req) != nil || !validDocumentKey(req.Key) || strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.TitleEN) == "" || strings.TrimSpace(req.Body) == "" || strings.TrimSpace(req.BodyEN) == "" || utf8.RuneCountInString(req.Body) > 50000 || utf8.RuneCountInString(req.BodyEN) > 50000 || utf8.RuneCountInString(req.Title) > 200 || utf8.RuneCountInString(req.TitleEN) > 200 || req.EffectiveAt.IsZero() {
		fail(c, 400, "invalid_request")
		return
	}
	var saved ServiceDocument
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		if e := serviceLock(tx, true); e != nil {
			return e
		}
		var d ServiceDocument
		if id(c) > 0 {
			if e := tx.First(&d, id(c)).Error; e != nil {
				return e
			}
			if d.Status != "draft" || d.Digest != req.ExpectedDigest || d.DocKey != req.Key {
				return serviceFailure{"stale_revision", 409}
			}
		} else {
			var latest ServiceDocument
			err := tx.Where("doc_key=?", req.Key).Order("version DESC").First(&latest).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			d.Version = latest.Version + 1
			d.DocKey = req.Key
			d.Status = "draft"
			d.CreatedAt = time.Now().UTC()
		}
		d.Title = req.Title
		d.TitleEN = req.TitleEN
		d.Body = req.Body
		d.BodyEN = req.BodyEN
		d.EffectiveAt = req.EffectiveAt.UTC().Truncate(time.Millisecond)
		d.UpdatedBy = actor(c).ID
		d.Digest = documentDigest(d)
		if e := tx.Save(&d).Error; e != nil {
			return e
		}
		saved = d
		return recordOperation(tx, actor(c).ID, "service_document", d.ID, "document_draft_saved", d.DocKey)
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, saved)
}

func (a *App) servicePageHTML(c *gin.Context, index string) bool {
	path := c.Request.URL.Path
	base := a.publicBase(c)
	canonical := base + path
	if path == "/service-requests" {
		return a.sendPublicHTML(c, index, 200, "投诉与服务申请 | "+a.brandName(), "提交投诉、申诉或企业权益申请，凭私密凭证查询反馈。", canonical, "noindex,follow")
	}
	key := strings.TrimPrefix(path, "/services/")
	if path == "/services" {
		key = "service-info"
	}
	if !validDocumentKey(key) {
		return a.sendPublicHTML(c, index, 404, "页面不存在 | "+a.brandName(), "请选择有效服务说明。", canonical, "noindex,follow")
	}
	q := a.DB.Where("doc_key=? AND status='published'", key)
	robots := "index,follow"
	if version := c.Query("version"); version != "" {
		n, e := strconv.ParseUint(version, 10, 64)
		if e != nil || n == 0 {
			return a.sendPublicHTML(c, index, 404, "版本不存在 | "+a.brandName(), "请选择当前版本。", canonical, "noindex,follow")
		}
		q = q.Where("version=?", n)
		canonical += "?version=" + strconv.FormatUint(n, 10)
		robots = "noindex,follow"
	} else {
		q = q.Where("effective_at<=?", time.Now().UTC())
	}
	var d ServiceDocument
	e := q.Order("version DESC").First(&d).Error
	if e != nil {
		status := 503
		if errors.Is(e, gorm.ErrRecordNotFound) {
			status = 404
		}
		return a.sendPublicHTML(c, index, status, "服务说明暂不可用 | "+a.brandName(), "请稍后重试。", canonical, "noindex,follow")
	}
	text := "<main class=\"container section service-document\" data-seo-content><h1>" + html.EscapeString(d.Title) + "</h1><p>版本 " + strconv.FormatUint(d.Version, 10) + "</p>"
	for _, p := range strings.Split(d.Body, "\n") {
		text += "<p>" + html.EscapeString(p) + "</p>"
	}
	text += "</main>"
	desc := "基础入驻永久免费，标准企业展厅、专属网址及二维码免费，默认免费存储200 MB。"
	return a.sendPublicHTML(c, index, 200, d.Title+" | "+a.brandName(), desc, canonical, robots, seoContent{Body: text})
}
