package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type VendorProductCategory struct {
	ID                 uint64    `json:"id"`
	VendorID           uint64    `json:"vendor_id"`
	HallID             uint64    `json:"hall_id"`
	IndustryCategoryID uint64    `json:"industry_category_id"`
	Name               string    `json:"name"`
	NameEN             string    `json:"name_en"`
	NameKey            string    `json:"-"`
	SortOrder          int       `json:"sort_order"`
	Enabled            bool      `json:"enabled"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	IndustryName       string    `json:"industry_name" gorm:"-"`
	IndustryNameEN     string    `json:"industry_name_en" gorm:"-"`
	ProductCount       int64     `json:"product_count" gorm:"-"`
	NeedsIndustry      bool      `json:"needs_industry" gorm:"-"`
}
type RevisionVendorProductCategory struct {
	RevisionID uint64 `json:"revision_id" gorm:"primaryKey"`
	CategoryID uint64 `json:"category_id"`
	VendorID   uint64 `json:"vendor_id"`
}
type VendorProductCategorySource struct {
	VendorID         uint64 `gorm:"primaryKey"`
	SourceCategoryID uint64 `gorm:"primaryKey"`
	CategoryID       uint64
}

func vendorCategoryNameKey(name string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(name))))
	return hex.EncodeToString(sum[:])
}
func vendorCategoryAvailable(db *gorm.DB) bool {
	return db.Migrator().HasTable(&VendorProductCategory{})
}
func vendorCategoryIndustry(db *gorm.DB, id, hall uint64) (Category, error) {
	var root Category
	if id == 0 || db.First(&root, canonicalCategory(db, id)).Error != nil || root.CategoryKind != "vendor" || root.ParentID != 0 || root.HallID != hall || !root.Enabled || root.DeletedAt != nil || !categoryAllowed(db, root.ID, 0) {
		return root, fieldFailure("invalid_category", "industry_category_id", "category_unavailable")
	}
	return root, nil
}
func vendorCategoryRoot(db *gorm.DB, id uint64) uint64 {
	seen := map[uint64]bool{}
	for id > 0 && !seen[id] {
		seen[id] = true
		var c Category
		if db.First(&c, canonicalCategory(db, id)).Error != nil {
			return 0
		}
		if c.ParentID > 0 {
			id = c.ParentID
			continue
		}
		if c.CategoryKind == "vendor" {
			return c.ID
		}
		if c.Code != nil && strings.HasPrefix(*c.Code, "ciame.2026.product.") {
			var target Category
			code := strings.Replace(*c.Code, "ciame.2026.product.", "ciame.2026.vendor.", 1)
			if db.Where("code=? AND hall_id=? AND category_kind='vendor'", code, c.HallID).First(&target).Error == nil {
				return target.ID
			}
		}
		return 0
	}
	return 0
}
func vendorCategoryPlatformRoot(db *gorm.DB, industry uint64) uint64 {
	var c Category
	if industry == 0 || db.First(&c, industry).Error != nil || c.Code == nil || !strings.HasPrefix(*c.Code, "ciame.2026.vendor.") {
		return 0
	}
	var product Category
	if db.Where("code=? AND hall_id=? AND category_kind='product'", strings.Replace(*c.Code, "ciame.2026.vendor.", "ciame.2026.product.", 1), c.HallID).First(&product).Error != nil {
		return 0
	}
	return product.ID
}

// Called in a transaction. The vendor row serializes creation, including concurrent legacy saves.
func vendorCategoryFromLegacy(tx *gorm.DB, vendorID, sourceID uint64) (uint64, error) {
	if sourceID == 0 {
		return 0, nil
	}
	var vendor Content
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND kind='vendor'", vendorID).First(&vendor).Error; err != nil {
		return 0, err
	}
	var mapping VendorProductCategorySource
	if tx.Where("vendor_id=? AND source_category_id=?", vendorID, sourceID).Limit(1).Find(&mapping).Error != nil {
		return 0, fmt.Errorf("invalid_category")
	}
	if mapping.CategoryID > 0 {
		return mapping.CategoryID, nil
	}
	var old Category
	if tx.First(&old, canonicalCategory(tx, sourceID)).Error != nil {
		return 0, nil
	}
	root := vendorCategoryRoot(tx, old.ID)
	key := vendorCategoryNameKey(old.Name)
	var cat VendorProductCategory
	if err := tx.Where("vendor_id=? AND industry_category_id=? AND name_key=?", vendorID, root, key).Limit(1).Find(&cat).Error; err != nil {
		return 0, err
	}
	if cat.ID == 0 {
		cat = VendorProductCategory{VendorID: vendorID, HallID: vendor.HallID, IndustryCategoryID: root, Name: old.Name, NameEN: old.NameEN, NameKey: key, Enabled: true}
		if err := tx.Create(&cat).Error; err != nil {
			return 0, err
		}
	}
	mapping = VendorProductCategorySource{VendorID: vendorID, SourceCategoryID: sourceID, CategoryID: cat.ID}
	if err := tx.Create(&mapping).Error; err != nil {
		return 0, err
	}
	return cat.ID, nil
}
func migrateVendorProductCategories(db *gorm.DB) error {
	var after uint64
	for {
		var revs []Revision
		if err := db.Joins("JOIN contents vc_content ON vc_content.id=revisions.content_id AND vc_content.kind='product'").Where("revisions.id>?", after).Select("revisions.*").Order("revisions.id").Limit(250).Find(&revs).Error; err != nil {
			return err
		}
		if len(revs) == 0 {
			return nil
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			for _, rev := range revs {
				var existing RevisionVendorProductCategory
				if err := tx.Where("revision_id=?", rev.ID).Limit(1).Find(&existing).Error; err != nil {
					return err
				}
				if existing.CategoryID > 0 {
					continue
				}
				var content Content
				if err := tx.First(&content, rev.ContentID).Error; err != nil {
					return err
				}
				var p Payload
				if err := json.Unmarshal([]byte(rev.Payload), &p); err != nil {
					return err
				}
				source := p.ProductCategoryID
				if source == 0 {
					var rc RevisionCategory
					tx.Where("revision_id=? AND role='product'", rev.ID).Limit(1).Find(&rc)
					source = rc.CategoryID
				}
				if source == 0 {
					source = p.CategoryID
				}
				categoryID := p.VendorProductCategoryID
				if categoryID == 0 {
					var err error
					categoryID, err = vendorCategoryFromLegacy(tx, content.VendorID, source)
					if err != nil {
						return err
					}
				}
				if categoryID > 0 {
					var cat VendorProductCategory
					if err := tx.Where("id=? AND vendor_id=?", categoryID, content.VendorID).First(&cat).Error; err != nil {
						return err
					}
					if err := tx.Create(&RevisionVendorProductCategory{rev.ID, categoryID, content.VendorID}).Error; err != nil {
						return err
					}
				}
			}
			return nil
		}); err != nil {
			return err
		}
		after = revs[len(revs)-1].ID
	}
}
func prepareVendorProductCategory(tx *gorm.DB, content Content, p *Payload) error {
	if content.Kind != "product" {
		if p.VendorProductCategoryID > 0 {
			return fieldFailure("invalid_category", "vendor_product_category_id", "category_unavailable")
		}
		return nil
	}
	if !vendorCategoryAvailable(tx) {
		return nil
	}
	var vendorLock Content
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND kind='vendor'", content.VendorID).First(&vendorLock).Error; err != nil {
		return err
	}
	legacyAssignment := false
	if p.VendorProductCategoryID == 0 {
		// An explicit clear is a draft operation, never a request to recreate a legacy assignment.
		if p.taxonomyFields["vendor_product_category_id"] {
			return nil
		}
		source := p.ProductCategoryID
		if source == 0 {
			source = p.CategoryID
		}
		legacyAssignment = true
		var err error
		p.VendorProductCategoryID, err = vendorCategoryFromLegacy(tx, content.VendorID, source)
		if err != nil {
			return err
		}
	}
	if p.VendorProductCategoryID == 0 {
		return nil
	}
	var cat VendorProductCategory
	if tx.Where("id=? AND vendor_id=? AND hall_id=?", p.VendorProductCategoryID, content.VendorID, content.HallID).First(&cat).Error != nil {
		return fieldFailure("invalid_category", "vendor_product_category_id", "category_unavailable")
	}
	if !cat.Enabled || (cat.IndustryCategoryID == 0 && !legacyAssignment) {
		var retained int64
		tx.Model(&RevisionVendorProductCategory{}).Where("revision_id=? AND vendor_id=? AND category_id=?", content.DraftRevisionID, content.VendorID, cat.ID).Count(&retained)
		if retained == 0 {
			return fieldFailure("invalid_category", "vendor_product_category_id", "category_unavailable")
		}
	}
	return nil
}
func indexVendorProductCategory(tx *gorm.DB, content Content, rev Revision, p Payload) error {
	if content.Kind != "product" || p.VendorProductCategoryID == 0 {
		return nil
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&RevisionVendorProductCategory{rev.ID, p.VendorProductCategoryID, content.VendorID}).Error
}
func (a *App) hydrateVendorProductCategories(items []View) error {
	ids := []uint64{}
	for _, v := range items {
		if v.Kind == "product" {
			ids = append(ids, v.RevisionID)
		}
	}
	if len(ids) == 0 || !vendorCategoryAvailable(a.DB) {
		return nil
	}
	var bindings []RevisionVendorProductCategory
	if err := a.DB.Where("revision_id IN ?", ids).Find(&bindings).Error; err != nil {
		return err
	}
	catIDs := []uint64{}
	byRev := map[uint64]RevisionVendorProductCategory{}
	for _, b := range bindings {
		byRev[b.RevisionID] = b
		catIDs = append(catIDs, b.CategoryID)
	}
	if len(catIDs) == 0 {
		return nil
	}
	var cats []VendorProductCategory
	if err := a.DB.Where("id IN ?", catIDs).Find(&cats).Error; err != nil {
		return err
	}
	if err := a.decorateVendorCategories(cats); err != nil {
		return err
	}
	byID := map[uint64]VendorProductCategory{}
	for _, cat := range cats {
		byID[cat.ID] = cat
	}
	for i := range items {
		v := &items[i]
		b := byRev[v.RevisionID]
		cat, ok := byID[b.CategoryID]
		if v.Kind == "product" && ok && cat.VendorID == v.VendorID && b.VendorID == v.VendorID {
			v.Data.VendorProductCategoryID = cat.ID
			copy := cat
			v.VendorProductCategory = &copy
		}
	}
	return nil
}
func (a *App) decorateVendorCategories(cats []VendorProductCategory) error {
	ids := []uint64{}
	for _, c := range cats {
		if c.IndustryCategoryID > 0 {
			ids = append(ids, c.IndustryCategoryID)
		}
	}
	var roots []Category
	if len(ids) > 0 {
		if err := a.DB.Where("id IN ?", ids).Find(&roots).Error; err != nil {
			return err
		}
	}
	byID := map[uint64]Category{}
	for _, r := range roots {
		byID[r.ID] = r
	}
	for i := range cats {
		r, ok := byID[cats[i].IndustryCategoryID]
		cats[i].IndustryName = r.Name
		cats[i].IndustryNameEN = r.NameEN
		cats[i].NeedsIndustry = !ok
	}
	return nil
}
func (a *App) publicVendorCategories(vendorID uint64) ([]VendorProductCategory, error) {
	cats := []VendorProductCategory{}
	if !vendorCategoryAvailable(a.DB) {
		return cats, nil
	}
	type count struct {
		CategoryID uint64
		Total      int64
	}
	var counts []count
	if err := a.publicQuery("product").Joins("JOIN revision_vendor_product_categories vpc_binding ON vpc_binding.revision_id=r.id AND vpc_binding.vendor_id=contents.vendor_id").Where("contents.vendor_id=?", vendorID).Select("vpc_binding.category_id,COUNT(*) AS total").Group("vpc_binding.category_id").Scan(&counts).Error; err != nil {
		return nil, err
	}
	totals := map[uint64]int64{}
	ids := []uint64{}
	for _, c := range counts {
		totals[c.CategoryID] = c.Total
		ids = append(ids, c.CategoryID)
	}
	if len(ids) > 0 {
		if err := a.DB.Where("vendor_id=? AND id IN ?", vendorID, ids).Order("sort_order,id").Find(&cats).Error; err != nil {
			return nil, err
		}
	}
	for i := range cats {
		cats[i].ProductCount = totals[cats[i].ID]
	}
	return cats, a.decorateVendorCategories(cats)
}
func (a *App) categoryVendor(c *gin.Context, tx *gorm.DB, lock bool) (Content, error) {
	who := actor(c)
	var vendor Content
	q := tx.Where("kind='vendor'")
	if platformAccount(who) {
		vid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		if vid == 0 {
			return vendor, fmt.Errorf("vendor_required")
		}
		q = q.Where("id=?", vid)
	} else {
		q = q.Where("owner_id=?", who.ID)
	}
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := q.First(&vendor).Error
	return vendor, err
}
func (a *App) listVendorProductCategories(c *gin.Context) {
	vendor, err := a.categoryVendor(c, a.DB, false)
	if err != nil {
		dbFail(c, err)
		return
	}
	cats := []VendorProductCategory{}
	if err = a.DB.Where("vendor_id=?", vendor.ID).Order("sort_order,id").Find(&cats).Error; err != nil {
		dbFail(c, err)
		return
	}
	if err = a.decorateVendorCategories(cats); err != nil {
		dbFail(c, err)
		return
	}
	roots := []Category{}
	for _, r := range a.scopedCategories(Account{}, true) {
		if r.HallID == vendor.HallID && r.CategoryKind == "vendor" && r.ParentID == 0 {
			roots = append(roots, r.Category)
		}
	}
	var rev Revision
	a.DB.First(&rev, vendor.DraftRevisionID)
	var p Payload
	json.Unmarshal([]byte(rev.Payload), &p)
	preferred := []uint64{}
	seen := map[uint64]bool{}
	for _, id := range append([]uint64{p.PrimaryCategoryID, p.CategoryID}, p.SecondaryCategoryIDs...) {
		root := vendorCategoryRoot(a.DB, id)
		if root > 0 && !seen[root] {
			preferred = append(preferred, root)
			seen[root] = true
		}
	}
	c.JSON(200, gin.H{"items": cats, "industry_categories": roots, "preferred_industry_ids": preferred, "vendor_id": vendor.ID})
}
func (a *App) saveVendorProductCategory(c *gin.Context) {
	var req struct {
		Name               string `json:"name"`
		NameEN             string `json:"name_en"`
		IndustryCategoryID uint64 `json:"industry_category_id"`
		SortOrder          int    `json:"sort_order"`
		Enabled            *bool  `json:"enabled"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.NameEN = strings.TrimSpace(req.NameEN)
	if req.Name == "" || len(req.Name) > 300 || len(req.NameEN) > 300 {
		dbFail(c, fieldFailure("invalid_request", "name", "invalid"))
		return
	}
	var saved VendorProductCategory
	err := a.contentTransaction(func(tx *gorm.DB) error {
		vendor, e := a.categoryVendor(c, tx, true)
		if e != nil {
			return e
		}
		root, e := vendorCategoryIndustry(tx, req.IndustryCategoryID, vendor.HallID)
		if e != nil {
			return e
		}
		key := vendorCategoryNameKey(req.Name)
		cid := vendorCategoryParamID(c)
		if c.Request.Method != "POST" && cid == 0 {
			return fmt.Errorf("invalid_request")
		}
		if cid > 0 {
			if e = tx.Where("id=? AND vendor_id=?", cid, vendor.ID).First(&saved).Error; e != nil {
				return e
			}
		}
		var n int64
		if e = tx.Model(&VendorProductCategory{}).Where("vendor_id=? AND industry_category_id=? AND name_key=? AND id<>?", vendor.ID, root.ID, key, cid).Count(&n).Error; e != nil {
			return e
		}
		if n > 0 {
			return fmt.Errorf("vendor_product_category_exists")
		}
		if cid == 0 {
			saved = VendorProductCategory{VendorID: vendor.ID, HallID: vendor.HallID, Enabled: true}
		}
		saved.Name = req.Name
		saved.NameEN = req.NameEN
		saved.NameKey = key
		saved.IndustryCategoryID = root.ID
		saved.SortOrder = req.SortOrder
		if req.Enabled != nil {
			saved.Enabled = *req.Enabled
		}
		if cid == 0 {
			e = tx.Create(&saved).Error
		} else {
			e = tx.Save(&saved).Error
		}
		if e != nil {
			return e
		}
		return recordOperation(tx, actor(c).ID, "vendor_product_category", saved.ID, "vendor_product_category_saved", fmt.Sprintf("vendor=%d", vendor.ID))
	})
	if err != nil {
		dbFail(c, err)
		return
	}
	decorated := []VendorProductCategory{saved}
	if err := a.decorateVendorCategories(decorated); err != nil {
		dbFail(c, err)
		return
	}
	saved = decorated[0]
	status := 200
	if c.Request.Method == "POST" {
		status = 201
	}
	c.JSON(status, saved)
}
func (a *App) deleteVendorProductCategory(c *gin.Context) {
	err := a.contentTransaction(func(tx *gorm.DB) error {
		vendor, e := a.categoryVendor(c, tx, true)
		if e != nil {
			return e
		}
		var cat VendorProductCategory
		if e = tx.Where("id=? AND vendor_id=?", vendorCategoryParamID(c), vendor.ID).First(&cat).Error; e != nil {
			return e
		}
		var n int64
		if e = tx.Model(&RevisionVendorProductCategory{}).Where("category_id=?", cat.ID).Count(&n).Error; e != nil {
			return e
		}
		if n > 0 {
			return fmt.Errorf("vendor_product_category_in_use")
		}
		if e = tx.Where("category_id=?", cat.ID).Delete(&VendorProductCategorySource{}).Error; e != nil {
			return e
		}
		if e = tx.Delete(&cat).Error; e != nil {
			return e
		}
		return recordOperation(tx, actor(c).ID, "vendor_product_category", cat.ID, "vendor_product_category_deleted", fmt.Sprintf("vendor=%d", vendor.ID))
	})
	if err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func (a *App) vendorProductCategoryRoutes(merchant, admin *gin.RouterGroup) {
	for _, group := range []*gin.RouterGroup{merchant, admin} {
		path := "/product-categories"
		if group == admin {
			path = "/vendors/:id/product-categories"
		}
		group.GET(path, a.listVendorProductCategories)
		group.POST(path, a.saveVendorProductCategory)
		group.PUT(path+"/:categoryID", a.saveVendorProductCategory)
		group.DELETE(path+"/:categoryID", a.deleteVendorProductCategory)
	}
}

func vendorCategoryKeywordSQL(alias string) string {
	return "EXISTS (SELECT 1 FROM revision_vendor_product_categories vpk JOIN vendor_product_categories vpc ON vpc.id=vpk.category_id AND vpc.vendor_id=vpk.vendor_id WHERE vpk.revision_id=" + alias + ".id AND vpk.vendor_id=contents.vendor_id AND (vpc.name LIKE ? OR vpc.name_en LIKE ?))"
}
func productCategoryFilter(q *gorm.DB, db *gorm.DB, alias string, ids []uint64) *gorm.DB {
	if !vendorCategoryAvailable(db) {
		return taxonomyFilter(q, alias, ids)
	}
	// Only stable CIAME codes map a manufacturer industry to a platform product root.
	return q.Where("(EXISTS (SELECT 1 FROM revision_categories rc WHERE rc.revision_id="+alias+".id AND rc.category_id IN ?) OR EXISTS (SELECT 1 FROM revision_vendor_product_categories vpb JOIN vendor_product_categories vpc ON vpc.id=vpb.category_id AND vpc.vendor_id=vpb.vendor_id JOIN categories vi ON vi.id=vpc.industry_category_id JOIN categories pi ON pi.code=REPLACE(vi.code,'ciame.2026.vendor.','ciame.2026.product.') AND pi.category_kind='product' AND pi.hall_id=vi.hall_id WHERE vpb.revision_id="+alias+".id AND vpb.vendor_id=contents.vendor_id AND vi.category_kind='vendor' AND vi.code LIKE 'ciame.2026.vendor.%' AND pi.id IN ?))", ids, ids)
}

func vendorCategoryParamID(c *gin.Context) uint64 {
	value, _ := strconv.ParseUint(c.Param("categoryID"), 10, 64)
	return value
}
