package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Resource struct {
	ID                  uint64    `json:"id"`
	OwnerID             uint64    `json:"owner_id"`
	VendorID            uint64    `json:"vendor_id"`
	Kind                string    `json:"kind"`
	DraftRevisionID     uint64    `json:"draft_revision_id"`
	PublishedRevisionID uint64    `json:"published_revision_id"`
	Epoch               uint64    `json:"-"`
	Visible             bool      `json:"visible"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}
type ResourcePayload struct {
	Title         string   `json:"title"`
	TitleEN       string   `json:"title_en"`
	Description   string   `json:"description"`
	DescriptionEN string   `json:"description_en"`
	Purpose       string   `json:"purpose"`
	Format        string   `json:"format"`
	ProductID     uint64   `json:"product_id"`
	FileMediaID   uint64   `json:"file_media_id"`
	CoverMediaID  uint64   `json:"cover_media_id"`
	PageMediaIDs  []uint64 `json:"page_media_ids"`
}
type ResourceRevision struct {
	ID         uint64
	ResourceID uint64
	BaseEpoch  uint64
	ProductID  uint64
	Title      string
	TitleEN    string
	Purpose    string
	Payload    string
	Status     string
	ReviewNote string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
type ResourceMediaReference struct {
	RevisionID uint64
	MediaID    uint64
}
type ResourceReviewLog struct {
	ID         uint64    `json:"id"`
	ResourceID uint64    `json:"resource_id"`
	RevisionID uint64    `json:"revision_id"`
	ActorID    uint64    `json:"actor_id"`
	Action     string    `json:"action"`
	Note       string    `json:"note"`
	CreatedAt  time.Time `json:"created_at"`
}
type MediaMetadata struct {
	MediaID    uint64    `gorm:"primaryKey" json:"media_id"`
	Kind       string    `json:"kind"`
	State      string    `json:"state"`
	Size       int64     `json:"size"`
	SourcePath string    `json:"-"`
	Duration   float64   `json:"duration"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	PageCount  int       `json:"page_count"`
	Error      string    `json:"error"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (MediaMetadata) TableName() string { return "media_metadata" }

type ResourceView struct {
	Resource
	Data          ResourcePayload `json:"data"`
	RevisionID    uint64          `json:"revision_id"`
	Status        string          `json:"status"`
	ReviewNote    string          `json:"review_note"`
	PublicVisible bool            `json:"public_visible"`
	File          *MediaMetadata  `json:"file,omitempty"`
}

const resourcePublicSQL = "resources.visible=1 AND pr.status='approved' AND EXISTS (SELECT 1 FROM contents v JOIN revisions vr ON vr.id=v.published_revision_id JOIN halls h ON h.id=v.hall_id AND h.status='open' WHERE v.id=resources.vendor_id AND v.kind='vendor' AND v.visible=1 AND vr.status='approved') AND (pr.product_id=0 OR EXISTS (SELECT 1 FROM contents p JOIN revisions pvr ON pvr.id=p.published_revision_id WHERE p.id=pr.product_id AND p.kind='product' AND p.vendor_id=resources.vendor_id AND p.visible=1 AND pvr.status='approved'))"

func settingInt(key string, def int) int {
	n, e := strconv.Atoi(env(key, strconv.Itoa(def)))
	if e != nil || n < 1 {
		return def
	}
	return n
}
func (a *App) resourceQuery() *gorm.DB {
	return a.DB.Model(&Resource{}).Joins("JOIN resource_revisions pr ON pr.id=resources.published_revision_id").Where(resourcePublicSQL)
}
func resourceMedia(p ResourcePayload) []uint64 {
	ids := []uint64{}
	seen := map[uint64]bool{}
	for _, id := range append(append([]uint64{p.FileMediaID, p.CoverMediaID}, p.PageMediaIDs...), 0) {
		if id > 0 && !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	return ids
}
func (a *App) resourceView(r Resource, rev ResourceRevision) ResourceView {
	var p ResourcePayload
	json.Unmarshal([]byte(rev.Payload), &p)
	v := ResourceView{Resource: r, Data: p, RevisionID: rev.ID, Status: rev.Status, ReviewNote: rev.ReviewNote}
	var n int64
	if r.PublishedRevisionID > 0 && a.resourceQuery().Where("resources.id=?", r.ID).Count(&n).Error == nil {
		v.PublicVisible = n > 0
	}
	if p.FileMediaID > 0 {
		var m MediaMetadata
		if a.DB.First(&m, "media_id=?", p.FileMediaID).Error == nil {
			v.File = &m
		}
	}
	return v
}
func loadResource(tx *gorm.DB, rid uint64, who Account, lock bool) (Resource, error) {
	var r Resource
	q := tx
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	e := q.First(&r, rid).Error
	if e == nil && !platformAccount(who) && who.ID != r.OwnerID {
		e = fmt.Errorf("forbidden")
	}
	return r, e
}
func resourceLog(tx *gorm.DB, r Resource, rev uint64, who Account, action, note string) error {
	if e := tx.Create(&ResourceReviewLog{ResourceID: r.ID, RevisionID: rev, ActorID: who.ID, Action: action, Note: note}).Error; e != nil {
		return e
	}
	return recordOperation(tx, who.ID, r.Kind, r.ID, action, note)
}
func (a *App) validateResource(tx *gorm.DB, r Resource, p *ResourcePayload, ready bool) error {
	p.Title = strings.TrimSpace(p.Title)
	if p.Title == "" || utf8.RuneCountInString(p.Title) > 200 || utf8.RuneCountInString(p.TitleEN) > 200 || len(p.Description) > 10000 || len(p.DescriptionEN) > 10000 {
		return fmt.Errorf("invalid_request")
	}
	var vendor Content
	if e := tx.First(&vendor, "id=? AND kind='vendor' AND owner_id=?", r.VendorID, r.OwnerID).Error; e != nil {
		return fmt.Errorf("invalid_request")
	}
	if p.ProductID > 0 {
		var product Content
		if tx.First(&product, "id=? AND kind='product' AND vendor_id=? AND owner_id=?", p.ProductID, r.VendorID, r.OwnerID).Error != nil {
			return fmt.Errorf("invalid_request")
		}
	}
	expected := ""
	if r.Kind == "video" {
		expected = "video"
		p.Format = "video"
		if p.FileMediaID == 0 || len(p.PageMediaIDs) > 0 || (p.Purpose != "company" && p.Purpose != "production" && p.Purpose != "product") || (p.Purpose == "product" && p.ProductID == 0) {
			return fmt.Errorf("invalid_request")
		}
		if p.ProductID > 0 {
			var n int64
			tx.Model(&Resource{}).Joins("JOIN resource_revisions d ON d.id=resources.draft_revision_id").Joins("LEFT JOIN resource_revisions p ON p.id=resources.published_revision_id").Where("resources.vendor_id=? AND resources.kind='video' AND resources.id<>? AND (d.product_id=? OR p.product_id=?)", r.VendorID, r.ID, p.ProductID, p.ProductID).Count(&n)
			if n >= int64(settingInt("MAX_PRODUCT_VIDEOS", 3)) {
				return fmt.Errorf("resource_limit_reached")
			}
		}
	} else if r.Kind == "catalog" {
		if p.Format == "pdf" {
			expected = "pdf"
			if p.FileMediaID == 0 || len(p.PageMediaIDs) > 0 {
				return fmt.Errorf("invalid_request")
			}
		} else if p.Format == "images" {
			if p.FileMediaID != 0 || len(p.PageMediaIDs) < 1 || len(p.PageMediaIDs) > settingInt("MAX_CATALOG_PAGES", 30) {
				return fmt.Errorf("invalid_request")
			}
			seen := map[uint64]bool{}
			for _, id := range p.PageMediaIDs {
				if id == 0 || seen[id] {
					return fmt.Errorf("invalid_media")
				}
				seen[id] = true
			}
			if p.CoverMediaID == 0 {
				p.CoverMediaID = p.PageMediaIDs[0]
			}
		} else {
			return fmt.Errorf("invalid_request")
		}
	} else {
		return fmt.Errorf("invalid_request")
	}
	if p.FileMediaID > 0 && p.CoverMediaID == p.FileMediaID {
		return fmt.Errorf("invalid_media")
	}
	for _, id := range resourceMedia(*p) {
		var m Media
		if tx.First(&m, id).Error != nil || m.OwnerID != r.OwnerID {
			return fmt.Errorf("invalid_media")
		}
		var meta MediaMetadata
		e := tx.First(&meta, "media_id=?", id).Error
		if id == p.FileMediaID {
			if e != nil || meta.Kind != expected {
				return fmt.Errorf("invalid_media")
			}
			if ready && meta.State != "ready" {
				return fmt.Errorf("media_not_ready")
			}
		} else if e == nil && meta.Kind != "image" {
			return fmt.Errorf("invalid_media")
		}
	}
	return nil
}
func saveResourceRevision(tx *gorm.DB, r *Resource, p ResourcePayload) error {
	data, e := json.Marshal(p)
	if e != nil {
		return e
	}
	rev := ResourceRevision{ResourceID: r.ID, BaseEpoch: r.Epoch, ProductID: p.ProductID, Title: p.Title, TitleEN: p.TitleEN, Purpose: p.Purpose, Payload: string(data), Status: "draft"}
	if e = tx.Create(&rev).Error; e != nil {
		return e
	}
	for _, id := range resourceMedia(p) {
		if e = tx.Create(&ResourceMediaReference{RevisionID: rev.ID, MediaID: id}).Error; e != nil {
			return e
		}
	}
	r.DraftRevisionID = rev.ID
	return tx.Model(r).Updates(map[string]any{"draft_revision_id": rev.ID, "updated_at": time.Now().UTC()}).Error
}
func (a *App) createResource(c *gin.Context) {
	var req struct {
		Kind string          `json:"kind"`
		Data ResourcePayload `json:"data"`
	}
	if c.ShouldBindJSON(&req) != nil || (req.Kind != "video" && req.Kind != "catalog") {
		fail(c, 400, "invalid_request")
		return
	}
	who := actor(c)
	var r Resource
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		var v Content
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id=? AND kind='vendor'", who.ID).First(&v).Error != nil {
			return fmt.Errorf("vendor_required")
		}
		var n int64
		if e := tx.Model(&Resource{}).Where("vendor_id=? AND kind=?", v.ID, req.Kind).Count(&n).Error; e != nil {
			return e
		}
		max := settingInt("MAX_VENDOR_VIDEOS", 20)
		if req.Kind == "catalog" {
			max = settingInt("MAX_VENDOR_CATALOGS", 10)
		}
		if n >= int64(max) {
			return fmt.Errorf("resource_limit_reached")
		}
		r = Resource{OwnerID: who.ID, VendorID: v.ID, Kind: req.Kind}
		if e := a.validateResource(tx, r, &req.Data, false); e != nil {
			return e
		}
		if e := tx.Create(&r).Error; e != nil {
			return e
		}
		return saveResourceRevision(tx, &r, req.Data)
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	var rev ResourceRevision
	a.DB.First(&rev, r.DraftRevisionID)
	c.JSON(201, a.resourceView(r, rev))
}
func (a *App) resourceDetail(c *gin.Context) {
	r, e := loadResource(a.DB, id(c), actor(c), false)
	if e != nil {
		dbFail(c, e)
		return
	}
	var rev ResourceRevision
	if a.DB.First(&rev, r.DraftRevisionID).Error != nil {
		fail(c, 500, "internal_error")
		return
	}
	c.JSON(200, a.resourceView(r, rev))
}
func (a *App) saveResourceDraft(c *gin.Context) {
	var req struct {
		RevisionID uint64          `json:"revision_id"`
		Data       ResourcePayload `json:"data"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	var r Resource
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		var e error
		r, e = loadResource(tx, id(c), actor(c), true)
		if e != nil {
			return e
		}
		var vendor Content
		if e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&vendor, r.VendorID).Error; e != nil {
			return e
		}
		if r.DraftRevisionID != req.RevisionID {
			return fmt.Errorf("stale_revision")
		}
		var rev ResourceRevision
		if e = tx.First(&rev, r.DraftRevisionID).Error; e != nil {
			return e
		}
		if rev.Status == "pending" {
			return fmt.Errorf("pending_review")
		}
		if e = a.validateResource(tx, r, &req.Data, false); e != nil {
			return e
		}
		if e = saveResourceRevision(tx, &r, req.Data); e != nil {
			return e
		}
		return resourceLog(tx, r, r.DraftRevisionID, actor(c), "draft_saved", "")
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	var rev ResourceRevision
	a.DB.First(&rev, r.DraftRevisionID)
	c.JSON(200, a.resourceView(r, rev))
}
func (a *App) submitResource(c *gin.Context) {
	var req struct {
		RevisionID uint64 `json:"revision_id"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		r, e := loadResource(tx, id(c), actor(c), true)
		if e != nil {
			return e
		}
		if r.DraftRevisionID != req.RevisionID {
			return fmt.Errorf("stale_revision")
		}
		var rev ResourceRevision
		if e = tx.First(&rev, req.RevisionID).Error; e != nil {
			return e
		}
		if rev.Status != "draft" {
			return fmt.Errorf("pending_review")
		}
		if rev.BaseEpoch != r.Epoch {
			return fmt.Errorf("stale_revision")
		}
		var p ResourcePayload
		if json.Unmarshal([]byte(rev.Payload), &p) != nil {
			return fmt.Errorf("invalid_request")
		}
		if e = a.validateResource(tx, r, &p, true); e != nil {
			return e
		}
		if e = tx.Model(&rev).Update("status", "pending").Error; e != nil {
			return e
		}
		return resourceLog(tx, r, rev.ID, actor(c), "submit", "")
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func (a *App) reviewResource(c *gin.Context) {
	var req struct {
		Decision string `json:"decision"`
		Note     string `json:"note"`
	}
	if c.ShouldBindJSON(&req) != nil || (req.Decision != "approve" && req.Decision != "reject") || utf8.RuneCountInString(req.Note) > 1000 {
		fail(c, 400, "invalid_request")
		return
	}
	if req.Decision == "reject" && strings.TrimSpace(req.Note) == "" {
		fail(c, 400, "rejection_reason_required")
		return
	}
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		var rev ResourceRevision
		if e := tx.First(&rev, id(c)).Error; e != nil {
			return e
		}
		r, e := loadResource(tx, rev.ResourceID, actor(c), true)
		if e != nil {
			return e
		}
		if e = tx.First(&rev, id(c)).Error; e != nil {
			return e
		}
		if rev.Status != "pending" {
			return fmt.Errorf("already_reviewed")
		}
		if r.Epoch != rev.BaseEpoch || r.DraftRevisionID != rev.ID {
			return fmt.Errorf("stale_revision")
		}
		status := "rejected"
		if req.Decision == "approve" {
			var p ResourcePayload
			json.Unmarshal([]byte(rev.Payload), &p)
			if e = a.validateResource(tx, r, &p, true); e != nil {
				return e
			}
			status = "approved"
			if e = tx.Model(&r).Updates(map[string]any{"published_revision_id": rev.ID, "visible": true, "epoch": r.Epoch + 1}).Error; e != nil {
				return e
			}
		}
		if e = tx.Model(&rev).Updates(map[string]any{"status": status, "review_note": req.Note}).Error; e != nil {
			return e
		}
		return resourceLog(tx, r, rev.ID, actor(c), req.Decision, req.Note)
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func (a *App) offlineResource(c *gin.Context) {
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		r, e := loadResource(tx, id(c), actor(c), true)
		if e != nil {
			return e
		}
		if e = tx.Model(&r).Updates(map[string]any{"visible": false, "epoch": r.Epoch + 1}).Error; e != nil {
			return e
		}
		if e = tx.Model(&ResourceRevision{}).Where("resource_id=? AND status='pending'", r.ID).Updates(map[string]any{"status": "rejected", "review_note": "offline"}).Error; e != nil {
			return e
		}
		return resourceLog(tx, r, r.PublishedRevisionID, actor(c), "offline", "")
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func (a *App) resourceHistory(c *gin.Context) {
	_, e := loadResource(a.DB, id(c), actor(c), false)
	if e != nil {
		dbFail(c, e)
		return
	}
	logs := []ResourceReviewLog{}
	if a.DB.Where("resource_id=?", id(c)).Order("id DESC").Limit(100).Find(&logs).Error != nil {
		fail(c, 500, "internal_error")
		return
	}
	c.JSON(200, logs)
}
func (a *App) listResources(admin, public bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, n := page(c)
		q := a.DB.Model(&Resource{}).Joins("JOIN resource_revisions dr ON dr.id=resources.draft_revision_id")
		alias := "dr"
		if public {
			q = a.resourceQuery()
			alias = "pr"
		}
		if !admin && !public {
			q = q.Where("resources.owner_id=?", actor(c).ID)
		}
		if public && c.Param("slug") != "" {
			vs, e := a.resolveShowroom(c.Param("slug"))
			if e != nil || len(vs) == 0 {
				fail(c, 404, "not_found")
				return
			}
			q = q.Where("resources.vendor_id=?", vs[0].ID)
		}
		for _, key := range []string{"kind", "vendor_id"} {
			if v := c.Query(key); v != "" {
				q = q.Where("resources."+key+"=?", v)
			}
		}
		for _, key := range []string{"purpose", "product_id"} {
			if v := c.Query(key); v != "" {
				q = q.Where(alias+"."+key+"=?", v)
			}
		}
		if !public {
			if v := c.Query("status"); v != "" {
				q = q.Where("dr.status=?", v)
			}
		}
		if v := strings.TrimSpace(c.Query("q")); v != "" {
			v = string([]rune(v)[:min(len([]rune(v)), 200)])
			q = q.Where("("+alias+".title LIKE ? OR "+alias+".title_en LIKE ?)", "%"+v+"%", "%"+v+"%")
		}
		var total int64
		if q.Count(&total).Error != nil {
			fail(c, 500, "internal_error")
			return
		}
		rs := []Resource{}
		if q.Select("resources.*").Order("resources.updated_at DESC, resources.id DESC").Offset((p-1)*n).Limit(n).Find(&rs).Error != nil {
			fail(c, 500, "internal_error")
			return
		}
		items := []ResourceView{}
		for _, r := range rs {
			rid := r.DraftRevisionID
			if public {
				rid = r.PublishedRevisionID
			}
			var rev ResourceRevision
			if a.DB.First(&rev, rid).Error != nil {
				fail(c, 500, "internal_error")
				return
			}
			items = append(items, a.resourceView(r, rev))
		}
		c.JSON(200, gin.H{"items": items, "total": total, "page": p, "page_size": n})
	}
}
func (a *App) publicResource(c *gin.Context) {
	var r Resource
	if a.resourceQuery().Where("resources.id=?", id(c)).Select("resources.*").First(&r).Error != nil {
		fail(c, 404, "not_found")
		return
	}
	var rev ResourceRevision
	if a.DB.First(&rev, r.PublishedRevisionID).Error != nil {
		fail(c, 500, "internal_error")
		return
	}
	c.JSON(200, a.resourceView(r, rev))
}
func (a *App) resourceRoutes(pub, merchant, admin *gin.RouterGroup) {
	pub.GET("/showrooms/:slug/resources", a.listResources(false, true))
	pub.GET("/resources", a.listResources(false, true))
	pub.GET("/resources/:id", a.publicResource)
	for _, g := range []*gin.RouterGroup{merchant, admin} {
		g.GET("/resources", a.listResources(g == admin, false))
		g.GET("/resources/:id", a.resourceDetail)
		g.PUT("/resources/:id/draft", a.saveResourceDraft)
		g.POST("/resources/:id/submit", a.submitResource)
		g.POST("/resources/:id/offline", a.offlineResource)
		g.GET("/resources/:id/history", a.resourceHistory)
	}
	merchant.POST("/resources", a.createResource)
	merchant.POST("/resource-media", a.uploadResourceMedia)
	merchant.GET("/resource-media/:id", a.resourceMediaStatus)
	merchant.POST("/resource-media/:id/retry", a.retryResourceMedia)
	admin.GET("/resource-media/:id", a.resourceMediaStatus)
	admin.POST("/resource-media/:id/retry", a.retryResourceMedia)
	for _, path := range []string{"media", "resource-media"} {
		isImage := path == "media"
		admin.POST("/resources/:id/"+path, func(c *gin.Context) {
			r, e := loadResource(a.DB, id(c), actor(c), false)
			if e != nil {
				dbFail(c, e)
				return
			}
			c.Set("upload_owner_id", r.OwnerID)
			c.Set("upload_resource_id", r.ID)
			if isImage {
				a.upload(c)
			} else {
				a.uploadResourceMedia(c)
			}
		})
	}
	admin.POST("/resource-revisions/:id/review", a.reviewResource)
}
