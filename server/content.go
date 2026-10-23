package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func id(c *gin.Context) uint64 { v, _ := strconv.ParseUint(c.Param("id"), 10, 64); return v }
func view(content Content, rev Revision) View {
	var data Payload
	json.Unmarshal([]byte(rev.Payload), &data)
	return View{Content: content, ShowroomSlug: showroomSlug(content), PublicVisible: content.Visible && content.PublishedRevisionID != 0, Data: data, Status: rev.Status, RevisionID: rev.ID, ReviewNote: rev.ReviewNote}
}
func (a *App) privateView(content Content, rev Revision) View {
	result := view(content, rev)
	result.Data.CategoryID = canonicalCategory(a.DB, result.Data.CategoryID)
	items := []View{result}
	if a.hydrateTaxonomyViews(items) == nil {
		result = items[0]
	}
	if result.PublicVisible && content.Kind == "product" {
		var count int64
		result.PublicVisible = a.publicQuery("vendor").Where("contents.id = ?", content.VendorID).Count(&count).Error == nil && count == 1
	}
	return result
}
func dbFail(c *gin.Context, e error) {
	if errors.Is(e, gorm.ErrRecordNotFound) {
		fail(c, 404, "not_found")
		return
	}
	var validation *FieldValidationError
	if errors.As(e, &validation) {
		status := 400
		if validation.Code == "phone_exists" || validation.Code == "phone_already_bound" || validation.Code == "login_name_exists" || validation.Code == "vendor_required" {
			status = 409
		}
		c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"code": validation.Code, "fields": validation.Fields}})
		return
	}
	code := e.Error()
	switch code {
	case "forbidden":
		fail(c, 403, code)
	case "watermark_not_ready", "watermark_vendor_required", "storage_quota_exceeded", "disk_space_insufficient", "media_in_use", "pending_review", "stale_revision", "already_reviewed", "vendor_required", "product_limit_reached", "login_name_exists", "showroom_slug_exists", "showroom_slug_locked", "resource_limit_reached", "media_not_ready", "featured_limit_reached", "representative_limit_reached":
		fail(c, 409, code)
	case "storage_unavailable":
		fail(c, 503, e.Error())
	case "invalid_watermark", "invalid_system_settings", "mixed_content_address", "address_probe_required", "storage_root_not_registered", "storage_mount_missing", "invalid_request", "invalid_media", "invalid_category", "invalid_region", "invalid_showroom_slug", "rejection_reason_required":
		fail(c, 400, code)
	default:
		fail(c, 500, "internal_error")
	}
}
func loadOwned(tx *gorm.DB, contentID uint64, who Account, lock bool) (Content, error) {
	var content Content
	q := tx
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	e := q.First(&content, contentID).Error
	if e == nil && !platformAccount(who) && content.OwnerID != who.ID {
		e = fmt.Errorf("forbidden")
	}
	return content, e
}
func (a *App) validatePayload(tx *gorm.DB, who Account, kind string, p *Payload, retained ...Payload) error {
	if kind == "vendor" && p.RegionCode != "" {
		if e := resolveAdministrativeRegion(tx, p); e != nil {
			return e
		}
	}
	p.Name = strings.TrimSpace(p.Name)
	p.RegionName = strings.TrimSpace(p.RegionName)
	if issues := payloadFieldIssues(kind, *p); len(issues) > 0 {
		code := "invalid_request"
		return &FieldValidationError{Code: code, Fields: issues}
	}
	if kind == "vendor" && p.RegionCode == "" {
		if e := resolveTextRegion(tx, p); e != nil {
			return e
		}
	}
	if p.CategoryID > 0 {
		p.CategoryID = canonicalCategory(tx, p.CategoryID)
		if !categoryAllowed(tx, p.CategoryID, who.ID) && (len(retained) == 0 || canonicalCategory(tx, retained[0].CategoryID) != p.CategoryID) {
			return fieldFailure("invalid_category", "category_id", "category_unavailable")
		}
	}
	if p.RegionID > 0 {
		var region Region
		if tx.First(&region, "id = ? AND enabled = 1", p.RegionID).Error != nil {
			return fmt.Errorf("invalid_region")
		}
		p.RegionName = region.Name
	}
	if kind != "vendor" && p.CompanyProfile != nil {
		return fmt.Errorf("invalid_request")
	}
	if e := validateCompanyProfile(p.CompanyProfile); e != nil {
		return e
	}
	seen := map[uint64]bool{}
	for _, mid := range p.MediaIDs {
		if mid == 0 || seen[mid] {
			return fmt.Errorf("invalid_media")
		}
		seen[mid] = true
	}
	for _, mid := range payloadMedia(*p) {
		var m Media
		if tx.First(&m, mid).Error != nil || m.OwnerID != who.ID {
			return fmt.Errorf("invalid_media")
		}
		var meta MediaMetadata
		if tx.First(&meta, "media_id=?", mid).Error == nil && meta.Kind != "image" {
			return fmt.Errorf("invalid_media")
		}
	}
	return nil
}
func makeDraft(tx *gorm.DB, content *Content, p Payload) error {
	p.IndustrialBeltIDs = nil
	if e := validateContentHall(tx, content, p); e != nil {
		return e
	}
	if content.HallID == 1 {
		if e := lockTaxonomy(tx, content.HallID); e != nil {
			return e
		}
	}
	if e := validateTaxonomy(tx, *content, &p); e != nil {
		return e
	}
	if content.DraftRevisionID == 0 && content.HallID == 1 {
		content.TaxonomyRequired = true
		if e := tx.Model(content).Update("taxonomy_required", true).Error; e != nil {
			return e
		}
	}
	if e := tx.Model(content).Update("hall_id", content.HallID).Error; e != nil {
		return e
	}
	blob, e := json.Marshal(p)
	if e != nil {
		return e
	}
	r := Revision{ContentID: content.ID, BaseEpoch: content.Epoch, Status: "draft", Name: p.Name, NameEN: p.NameEN, Keywords: p.Keywords, MainProducts: p.MainProducts, RegionID: p.RegionID, CategoryID: p.CategoryID, Payload: string(blob)}
	if content.Kind == "vendor" {
		var region Region
		if p.RegionID != 0 {
			if e = tx.First(&region, p.RegionID).Error; e != nil {
				return e
			}
		}
		r.AdministrativeSnapshot = administrativeSnapshot(p, region)
	}
	if e = tx.Create(&r).Error; e != nil {
		return e
	}
	if e = indexTaxonomyRevision(tx, *content, r, p, false); e != nil {
		return e
	}
	for _, mid := range payloadMedia(p) {
		if e = tx.Create(&MediaReference{RevisionID: r.ID, MediaID: mid}).Error; e != nil {
			return e
		}
	}
	content.DraftRevisionID = r.ID
	return tx.Model(content).Update("draft_revision_id", r.ID).Error
}

const maxVendorProducts = 1000

// Lock the booth before counting: concurrent creates share the same quota.
// Drafts and offline products count too; editing an existing product consumes no slot.
func productSlot(tx *gorm.DB, ownerID uint64) (uint64, error) {
	var vendor Content
	if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("kind = 'vendor' AND owner_id = ?", ownerID).First(&vendor).Error; e != nil {
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return 0, fmt.Errorf("vendor_required")
		}
		return 0, e
	}
	var count int64
	if e := tx.Model(&Content{}).Clauses(clause.Locking{Strength: "UPDATE"}).Where("kind = 'product' AND vendor_id = ?", vendor.ID).Count(&count).Error; e != nil {
		return 0, e
	}
	if count >= maxVendorProducts {
		return 0, fmt.Errorf("product_limit_reached")
	}
	return vendor.ID, nil
}
func (a *App) createContent(c *gin.Context) {
	var req struct {
		HallID uint64  `json:"hall_id"`
		Kind   string  `json:"kind"`
		Data   Payload `json:"data"`
	}
	if c.ShouldBindJSON(&req) != nil || (req.Kind != "vendor" && req.Kind != "product") {
		fail(c, 400, "invalid_request")
		return
	}
	who := actor(c)
	content := Content{Kind: req.Kind, OwnerID: who.ID, HallID: req.HallID}
	e := a.watermarkTransaction(func(tx *gorm.DB) error {
		if e := a.validatePayload(tx, who, req.Kind, &req.Data); e != nil {
			return e
		}
		if req.Kind == "vendor" {
			hall := Hall{}
			hid := content.HallID
			if hid == 0 {
				hid = 1
			}
			if tx.First(&hall, hid).Error != nil || hall.Status != "open" {
				return fmt.Errorf("invalid_request")
			}
			content.VendorOwnerKey = &who.ID
			var n int64
			tx.Model(&Content{}).Where("vendor_owner_key = ?", who.ID).Count(&n)
			if n > 0 {
				return fmt.Errorf("vendor_required")
			}
		} else {
			var e error
			content.VendorID, e = productSlot(tx, who.ID)
			if e != nil {
				return e
			}
		}
		if e := tx.Create(&content).Error; e != nil {
			return e
		}
		return a.makeDraft(tx, &content, req.Data)
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	var rev Revision
	a.DB.First(&rev, content.DraftRevisionID)
	c.JSON(201, a.privateView(content, rev))
}
func (a *App) saveDraft(c *gin.Context) {
	var req struct {
		RevisionID uint64  `json:"revision_id"`
		Data       Payload `json:"data"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	who := actor(c)
	var content Content
	e := a.watermarkTransaction(func(tx *gorm.DB) error {
		var e error
		content, e = loadOwned(tx, id(c), who, true)
		if e != nil {
			return e
		}
		if content.DraftRevisionID != req.RevisionID {
			return fmt.Errorf("stale_revision")
		}
		var old Revision
		if e = tx.First(&old, content.DraftRevisionID).Error; e != nil {
			return e
		}
		if old.Status == "pending" {
			return fmt.Errorf("pending_review")
		}
		var previousTaxonomy Payload
		if e = json.Unmarshal([]byte(old.Payload), &previousTaxonomy); e != nil {
			return e
		}
		oldView := []View{view(content, old)}
		// Use the transaction to avoid a pool read outside the locked content.
		if e = (&App{DB: tx}).hydrateTaxonomyViews(oldView); e != nil {
			return e
		}
		previousTaxonomy.PrimaryCategoryID = oldView[0].Data.PrimaryCategoryID
		previousTaxonomy.SecondaryCategoryIDs = oldView[0].Data.SecondaryCategoryIDs
		previousTaxonomy.ProductCategoryID = oldView[0].Data.ProductCategoryID
		mergeTaxonomyFields(&req.Data, previousTaxonomy)
		if !content.TaxonomyRequired && ((req.Data.PrimaryCategoryID > 0 && req.Data.PrimaryCategoryID != previousTaxonomy.PrimaryCategoryID) || (req.Data.ProductCategoryID > 0 && req.Data.ProductCategoryID != previousTaxonomy.ProductCategoryID)) {
			content.TaxonomyRequired = true
			if e = tx.Model(&content).Update("taxonomy_required", true).Error; e != nil {
				return e
			}
		}
		if canonicalCategory(tx, req.Data.CategoryID) != canonicalCategory(tx, previousTaxonomy.CategoryID) {
			if !req.Data.taxonomyFields["primary_category_id"] {
				req.Data.PrimaryCategoryID = 0
			}
			if !req.Data.taxonomyFields["product_category_id"] {
				req.Data.ProductCategoryID = 0
			}
		}
		if content.Kind == "vendor" {
			var previous Payload
			if e = json.Unmarshal([]byte(old.Payload), &previous); e != nil {
				return e
			}
			mergePortalProfile(&req.Data, previous)
		}
		if e = a.validatePayload(tx, Account{ID: content.OwnerID}, content.Kind, &req.Data, previousTaxonomy); e != nil {
			return e
		}
		if e = a.makeDraft(tx, &content, req.Data); e != nil {
			return e
		}
		if platformAccount(who) {
			return recordOperation(tx, who.ID, content.Kind, content.ID, "draft_saved", "")
		}
		return nil
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	var rev Revision
	a.DB.First(&rev, content.DraftRevisionID)
	c.JSON(200, a.privateView(content, rev))
}
func (a *App) submit(c *gin.Context) {
	var req struct {
		RevisionID uint64 `json:"revision_id"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	who := actor(c)
	e := a.watermarkTransaction(func(tx *gorm.DB) error {
		content, e := loadOwned(tx, id(c), who, true)
		if e != nil {
			return e
		}
		if content.DraftRevisionID != req.RevisionID {
			return fmt.Errorf("stale_revision")
		}
		var rev Revision
		if e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&rev, req.RevisionID).Error; e != nil {
			return e
		}
		if rev.Status != "draft" {
			return fmt.Errorf("pending_review")
		}
		if content.Epoch != rev.BaseEpoch {
			return fmt.Errorf("stale_revision")
		}
		if e = submitTaxonomy(tx, content, rev); e != nil {
			return e
		}
		if e = tx.Model(&rev).Update("status", "pending").Error; e != nil {
			return e
		}
		return recordReview(tx, ReviewLog{ContentID: content.ID, RevisionID: rev.ID, ActorID: who.ID, Action: "submit"})
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func (a *App) review(c *gin.Context) {
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
	who := actor(c)
	e := a.watermarkTransaction(func(tx *gorm.DB) error {
		var initial Revision
		if e := tx.First(&initial, id(c)).Error; e != nil {
			return e
		}
		content, e := loadOwned(tx, initial.ContentID, who, true)
		if e != nil {
			return e
		}
		var rev Revision
		if e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&rev, id(c)).Error; e != nil {
			return e
		}
		if rev.Status != "pending" {
			return fmt.Errorf("already_reviewed")
		}
		if content.Epoch != rev.BaseEpoch || content.DraftRevisionID != rev.ID {
			return fmt.Errorf("stale_revision")
		}
		status := "rejected"
		if req.Decision == "approve" {
			status = "approved"
			var p Payload
			if json.Unmarshal([]byte(rev.Payload), &p) != nil {
				return fmt.Errorf("invalid_request")
			}
			if e = validateContentHall(tx, &content, p); e != nil {
				return e
			}
			if content.Kind == "vendor" && p.RegionID == 0 {
				region := Region{Name: p.RegionName, Enabled: true}
				if e = tx.Where("name = ?", p.RegionName).FirstOrCreate(&region).Error; e != nil {
					return e
				}
				p.RegionID = region.ID
				blob, _ := json.Marshal(p)
				if e = tx.Model(&rev).Updates(map[string]any{"region_id": region.ID, "payload": string(blob)}).Error; e != nil {
					return e
				}
			}
			if e = tx.Model(&content).Updates(map[string]any{"published_revision_id": rev.ID, "visible": true, "epoch": content.Epoch + 1}).Error; e != nil {
				return e
			}
		}
		if e = tx.Model(&rev).Updates(map[string]any{"status": status, "review_note": req.Note, "reviewer_id": who.ID, "reviewed_at": time.Now().UTC()}).Error; e != nil {
			return e
		}
		return recordReview(tx, ReviewLog{ContentID: content.ID, RevisionID: rev.ID, ActorID: who.ID, Action: req.Decision, Note: req.Note})
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func (a *App) offline(admin bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		who := actor(c)
		e := a.watermarkTransaction(func(tx *gorm.DB) error {
			content, e := loadOwned(tx, id(c), who, true)
			if e != nil {
				return e
			}
			if e = tx.Model(&content).Updates(map[string]any{"visible": false, "featured": false, "epoch": content.Epoch + 1}).Error; e != nil {
				return e
			}
			if e = tx.Model(&Revision{}).Where("content_id = ? AND status = 'pending'", content.ID).Updates(map[string]any{"status": "rejected", "review_note": "offline", "reviewer_id": who.ID, "reviewed_at": time.Now().UTC()}).Error; e != nil {
				return e
			}
			action := "offline"
			if admin {
				action = "admin_offline"
			}
			return recordReview(tx, ReviewLog{ContentID: content.ID, RevisionID: content.PublishedRevisionID, ActorID: who.ID, Action: action})
		})
		if e != nil {
			dbFail(c, e)
			return
		}
		c.JSON(200, gin.H{"ok": true})
	}
}
func (a *App) privateDetail(c *gin.Context) {
	content, e := loadOwned(a.DB, id(c), actor(c), false)
	if e != nil {
		dbFail(c, e)
		return
	}
	var r Revision
	if e = a.DB.First(&r, content.DraftRevisionID).Error; e != nil {
		dbFail(c, e)
		return
	}
	items := []View{a.privateView(content, r)}
	if platformAccount(actor(c)) {
		if e = a.adminEnrich(items); e != nil {
			dbFail(c, e)
			return
		}
	}
	c.JSON(200, items[0])
}
func (a *App) myContents(c *gin.Context)    { a.listContents(c, false) }
func (a *App) adminContents(c *gin.Context) { a.listContents(c, true) }
func page(c *gin.Context) (int, int) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	n, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if p < 1 {
		p = 1
	}
	if n < 1 || n > 100 {
		n = 20
	}
	return p, n
}
func (a *App) listContents(c *gin.Context, admin bool) {
	p, n := page(c)
	q := a.DB.Model(&Content{}).Joins("JOIN revisions dr ON dr.id = contents.draft_revision_id")
	if c.Query("hall") != "" {
		hall, ok := a.requestHall(c)
		if !ok {
			return
		}
		q = q.Where("contents.hall_id=?", hall.ID)
	}
	if admin && actor(c).Role == "operator" {
		kinds := []string{}
		for _, kind := range []string{"vendor", "product"} {
			if a.hasPermission(actor(c), kind+".read") {
				kinds = append(kinds, kind)
			}
		}
		q = q.Where("contents.kind IN ?", kinds)
	}
	if !admin {
		q = q.Where("contents.owner_id = ?", actor(c).ID)
	}
	if kind := c.Query("kind"); kind != "" {
		q = q.Where("contents.kind = ?", kind)
	}
	if status := c.Query("status"); status != "" {
		q = q.Where("dr.status = ?", status)
	}
	if strings.TrimSpace(c.Query("q")) != "" {
		k := keyword(c)
		if !admin && c.Query("kind") == "product" {
			q = q.Where(specKeywordCondition("dr"), k, k, k, k, k)
		} else {
			q = q.Where(keywordCondition("dr"), k, k, k, k)
		}
	}
	if v := c.Query("public_state"); v == "public" {
		q = q.Where(contentPublicSQL)
	} else if v == "unpublished" {
		q = q.Where("NOT (" + contentPublicSQL + ")")
	}
	if admin {
		if v := c.Query("vendor_id"); v != "" {
			q = q.Where("contents.vendor_id=?", v)
		}
		if ids := a.categoryIDs(c.Query("category_id")); len(ids) > 0 {
			if _, modern := a.newCategoryFilter(c.Query("category_id"), c.Query("kind")); modern {
				q = taxonomyFilter(q, "dr", ids)
			} else {
				q = q.Where("dr.category_id IN ?", ids)
			}
		}
		if value := c.Query("industrial_belt_id"); value != "" {
			q = filterBelt(c, q, "CASE WHEN contents.kind='vendor' THEN contents.id ELSE contents.vendor_id END")
		}
		if v := c.Query("region_id"); v != "" {
			q = q.Where("(contents.kind='vendor' AND dr.region_id=?) OR (contents.kind='product' AND EXISTS (SELECT 1 FROM contents bv JOIN revisions br ON br.id=bv.draft_revision_id WHERE bv.id=contents.vendor_id AND br.region_id=?))", v, v)
		}
		if v := c.Query("featured"); v == "1" || v == "0" {
			q = q.Where("contents.featured=?", v == "1")
		}
	}
	var total int64
	if q.Count(&total).Error != nil {
		fail(c, 500, "internal_error")
		return
	}
	var contents []Content
	if q.Select("contents.*").Order("contents.updated_at DESC, contents.id DESC").Offset((p-1)*n).Limit(n).Find(&contents).Error != nil {
		fail(c, 500, "internal_error")
		return
	}
	items := []View{}
	for _, content := range contents {
		var rev Revision
		if a.DB.First(&rev, content.DraftRevisionID).Error != nil {
			fail(c, 500, "internal_error")
			return
		}
		items = append(items, a.privateView(content, rev))
	}
	if admin {
		if e := a.adminEnrich(items); e != nil {
			dbFail(c, e)
			return
		}
	}
	c.JSON(200, gin.H{"items": items, "total": total, "page": p, "page_size": n})
}
func (a *App) history(c *gin.Context) {
	content, e := loadOwned(a.DB, id(c), actor(c), false)
	if e != nil {
		dbFail(c, e)
		return
	}
	var logs []ReviewLog
	if a.DB.Where("content_id = ?", content.ID).Order("id DESC").Limit(100).Find(&logs).Error != nil {
		fail(c, 500, "internal_error")
		return
	}
	if logs == nil {
		logs = []ReviewLog{}
	}
	c.JSON(200, logs)
}
func (a *App) featured(c *gin.Context) {
	var req struct {
		Featured  bool `json:"featured"`
		SortOrder int  `json:"sort_order"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	e := a.watermarkTransaction(func(tx *gorm.DB) error {
		var target Content
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&target, id(c)).Error; e != nil {
			return e
		}
		if _, e := lockHomeDisplay(tx, target.HallID); e != nil {
			return e
		}
		content, e := loadOwned(tx, id(c), actor(c), true)
		if e != nil {
			return e
		}
		if req.Featured {
			var count int64
			ta := &App{DB: tx}
			if e = ta.publicQuery(content.Kind, true).Where("contents.id=?", content.ID).Count(&count).Error; e != nil {
				return e
			}
			if count != 1 {
				return fmt.Errorf("invalid_request")
			}
			if content.Kind == "vendor" && !content.Featured {
				if e := ta.publicQuery("vendor", true).Where("contents.featured=1 AND contents.hall_id=?", content.HallID).Count(&count).Error; e != nil {
					return e
				}
				if count >= homeVendorLimit {
					return fmt.Errorf("featured_limit_reached")
				}
			}
		}
		if e = tx.Model(&content).Updates(map[string]any{"featured": req.Featured, "sort_order": req.SortOrder}).Error; e != nil {
			return e
		}
		return recordOperation(tx, actor(c).ID, content.Kind, content.ID, "featured_changed", fmt.Sprintf("featured=%t sort=%d", req.Featured, req.SortOrder))
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
