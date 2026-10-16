package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strconv"
	"strings"
)

func (a *App) publicQuery(kind string) *gorm.DB {
	q := a.DB.Model(&Content{}).Joins("JOIN revisions r ON r.id = contents.published_revision_id AND r.status = 'approved'").Where("contents.kind = ? AND contents.visible = 1", kind)
	if kind == "product" {
		q = q.Joins("JOIN contents v ON v.id = contents.vendor_id AND v.kind = 'vendor' AND v.visible = 1").Joins("JOIN revisions vr ON vr.id = v.published_revision_id AND vr.status = 'approved'")
	}
	return q
}
func (a *App) publicViews(q *gorm.DB) ([]View, error) {
	var rows []Content
	if e := q.Select("contents.*").Find(&rows).Error; e != nil {
		return nil, e
	}
	items := []View{}
	for _, content := range rows {
		var rev Revision
		if e := a.DB.First(&rev, content.PublishedRevisionID).Error; e != nil {
			return nil, e
		}
		content.OwnerID = 0
		content.DraftRevisionID = 0
		content.VendorOwnerKey = nil
		content.Epoch = 0
		items = append(items, view(content, rev))
	}
	a.resolveCategoryViews(items)
	return items, nil
}
func (a *App) categoryIDs(category string) []uint64 {
	raw, _ := strconv.ParseUint(category, 10, 64)
	root := canonicalCategory(a.DB, raw)
	if root == 0 {
		return nil
	}
	ids := []uint64{root}
	var all []Category
	a.DB.Find(&all)
	seen := map[uint64]bool{root: true}
	for i := 0; i < len(ids); i++ {
		for _, cat := range all {
			if cat.ParentID == ids[i] && !seen[cat.ID] && canonicalCategory(a.DB, cat.ID) == cat.ID {
				seen[cat.ID] = true
				ids = append(ids, cat.ID)
			}
		}
	}
	var aliases []CategoryAlias
	a.DB.Find(&aliases)
	for _, alias := range aliases {
		if seen[canonicalCategory(a.DB, alias.SourceID)] && !seen[alias.SourceID] {
			seen[alias.SourceID] = true
			ids = append(ids, alias.SourceID)
		}
	}
	return ids
}
func keywordCondition(alias string) string {
	return fmt.Sprintf("(%s.name LIKE ? OR %s.name_en LIKE ? OR %s.main_products LIKE ? OR %s.keywords LIKE ?)", alias, alias, alias, alias)
}
func keyword(c *gin.Context) string {
	runes := []rune(strings.TrimSpace(c.Query("q")))
	if len(runes) > 200 {
		runes = runes[:200]
	}
	s := string(runes)
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return "%" + s + "%"
}
func (a *App) filterProducts(c *gin.Context, q *gorm.DB) *gorm.DB {
	if strings.TrimSpace(c.Query("q")) != "" {
		k := keyword(c)
		q = q.Where(keywordCondition("r"), k, k, k, k)
	}
	if ids := a.categoryIDs(c.Query("category_id")); len(ids) > 0 {
		q = q.Where("r.category_id IN ?", ids)
	}
	if vendorID := c.Query("vendor_id"); vendorID != "" {
		q = q.Where("contents.vendor_id = ?", vendorID)
	}
	if regionID := c.Query("region_id"); regionID != "" {
		q = q.Where("vr.region_id = ?", regionID)
	}
	return q
}

// Read only published vendor snapshots, with one query for the whole product page.
func (a *App) productVendorSummaries(items []View) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]uint64, 0, len(items))
	seen := map[uint64]bool{}
	for _, item := range items {
		if !seen[item.VendorID] {
			ids = append(ids, item.VendorID)
			seen[item.VendorID] = true
		}
	}
	var rows []struct {
		ID           uint64
		ShowroomSlug *string
		Payload      string
	}
	if e := a.publicQuery("vendor").Where("contents.id IN ?", ids).Select("contents.id, contents.showroom_slug, r.payload").Scan(&rows).Error; e != nil {
		return e
	}
	vendors := map[uint64]*VendorSummary{}
	for _, row := range rows {
		var data Payload
		if e := json.Unmarshal([]byte(row.Payload), &data); e != nil {
			return e
		}
		vendors[row.ID] = &VendorSummary{ID: row.ID, Name: data.Name, NameEN: data.NameEN, RegionID: data.RegionID, RegionName: data.RegionName, ShowroomSlug: showroomSlug(Content{ID: row.ID, Kind: "vendor", ShowroomSlug: row.ShowroomSlug})}
	}
	for i := range items {
		items[i].Vendor = vendors[items[i].VendorID]
	}
	return nil
}
func (a *App) searchVendors(c *gin.Context) {
	p, n := page(c)
	q := a.publicQuery("vendor")
	productMatch := "SELECT 1 FROM contents p JOIN revisions pr ON pr.id=p.published_revision_id AND pr.status='approved' WHERE p.vendor_id=contents.id AND p.kind='product' AND p.visible=1"
	if strings.TrimSpace(c.Query("q")) != "" {
		k := keyword(c)
		q = q.Where("("+keywordCondition("r")+" OR EXISTS ("+productMatch+" AND "+keywordCondition("pr")+"))", k, k, k, k, k, k, k, k)
	}
	if ids := a.categoryIDs(c.Query("category_id")); len(ids) > 0 {
		q = q.Where("(r.category_id IN ? OR EXISTS ("+productMatch+" AND pr.category_id IN ?))", ids, ids)
	}
	if region := c.Query("region_id"); region != "" {
		q = q.Where("r.region_id = ?", region)
	}
	var total int64
	if e := q.Count(&total).Error; e != nil {
		dbFail(c, e)
		return
	}
	items, e := a.publicViews(q.Order("contents.featured DESC, contents.sort_order ASC, contents.id DESC").Offset((p - 1) * n).Limit(n))
	if e != nil {
		dbFail(c, e)
		return
	}
	for i := range items {
		products, e := a.publicViews(a.filterProducts(c, a.publicQuery("product")).Where("contents.vendor_id = ?", items[i].ID).Order("contents.id DESC").Limit(3))
		if e != nil {
			dbFail(c, e)
			return
		}
		items[i].MatchingProducts = products
	}
	c.JSON(200, gin.H{"items": items, "total": total, "page": p, "page_size": n})
}
func (a *App) publicProducts(c *gin.Context) {
	p, n := page(c)
	q := a.filterProducts(c, a.publicQuery("product"))
	var total int64
	if e := q.Count(&total).Error; e != nil {
		dbFail(c, e)
		return
	}
	items, e := a.publicViews(q.Order("contents.featured DESC, contents.sort_order ASC, contents.id DESC").Offset((p - 1) * n).Limit(n))
	if e != nil {
		dbFail(c, e)
		return
	}
	if e = a.productVendorSummaries(items); e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"items": items, "total": total, "page": p, "page_size": n})
}
func (a *App) publicDetail(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		items, e := a.publicViews(a.publicQuery(kind).Where("contents.id = ?", id(c)))
		if e != nil {
			dbFail(c, e)
			return
		}
		if len(items) == 0 {
			fail(c, 404, "not_found")
			return
		}
		result := gin.H{"item": items[0]}
		if kind == "vendor" {
			p, n := page(c)
			q := a.publicQuery("product").Where("contents.vendor_id = ?", id(c))
			var total int64
			q.Count(&total)
			products, e := a.publicViews(q.Order("contents.id DESC").Offset((p - 1) * n).Limit(n))
			if e != nil {
				dbFail(c, e)
				return
			}
			result["products"] = gin.H{"items": products, "total": total, "page": p, "page_size": n}
		} else {
			vendors, e := a.publicViews(a.publicQuery("vendor").Where("contents.id = ?", items[0].VendorID))
			if e != nil || len(vendors) == 0 {
				fail(c, 404, "not_found")
				return
			}
			result["vendor"] = vendors[0]
		}
		c.JSON(200, result)
	}
}
func (a *App) home(c *gin.Context) {
	vendors, e := a.publicViews(a.publicQuery("vendor").Where("contents.featured=1").Order("contents.sort_order, contents.id DESC").Limit(homeVendorLimit))
	if e != nil {
		dbFail(c, e)
		return
	}
	products, e := a.publicViews(a.publicQuery("product").Where("contents.featured=1").Order("contents.sort_order, contents.id DESC").Limit(8))
	if e != nil {
		dbFail(c, e)
		return
	}
	var count int64
	a.publicQuery("vendor").Count(&count)
	var settings HomeDisplaySettings
	if e := a.DB.First(&settings, 1).Error; e != nil {
		dbFail(c, e)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"hero": gin.H{"media_id": settings.HeroMediaID}, "vendors": vendors, "products": products, "vendor_count": count})
}
func (a *App) taxonomy(c *gin.Context)    { a.taxonomyResult(c, false) }
func (a *App) allTaxonomy(c *gin.Context) { a.taxonomyResult(c, true) }
func (a *App) taxonomyResult(c *gin.Context, all bool) {
	categories := []Category{}
	regions := []Region{}
	halls := []Hall{}
	a.DB.Find(&halls)
	q := a.DB.Order("sort_order, id")
	if !all {
		q = q.Where("enabled=1")
	}
	if e := q.Find(&categories).Error; e != nil {
		dbFail(c, e)
		return
	}
	rq := a.DB.Model(&Region{})
	if !all {
		rq = rq.Where("enabled=1 AND id IN (SELECT r.region_id FROM contents c JOIN revisions r ON r.id=c.published_revision_id AND r.status='approved' WHERE c.kind='vendor' AND c.visible=1)")
	}
	if e := rq.Order("id").Find(&regions).Error; e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"halls": halls, "categories": a.scopedCategories(func() Account {
		if all {
			return actor(c)
		}
		return Account{}
	}(), !all), "regions": regions, "aliases": a.categoryAliases()})
}
func (a *App) saveCategory(c *gin.Context) {
	var req Category
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Name) == "" || len(req.Name) > 300 || len(req.NameEN) > 300 {
		fail(c, 400, "invalid_request")
		return
	}
	req.ID = id(c)
	if req.HallID == 0 {
		req.HallID = 1
	}
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		var hall Hall
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&hall, req.HallID).Error != nil {
			return fmt.Errorf("invalid_category")
		}
		parent := req.ParentID
		seen := map[uint64]bool{}
		for parent > 0 {
			if parent == req.ID || seen[parent] {
				return fmt.Errorf("invalid_category")
			}
			seen[parent] = true
			var p Category
			if tx.First(&p, parent).Error != nil || p.HallID != req.HallID {
				return fmt.Errorf("invalid_category")
			}
			parent = p.ParentID
		}
		if req.ID == 0 {
			if e := tx.Create(&req).Error; e != nil {
				return e
			}
			return recordOperation(tx, actor(c).ID, "category", req.ID, "category_saved", "")
		}
		var old Category
		if e := tx.First(&old, req.ID).Error; e != nil {
			return e
		}
		if e := tx.Model(&old).Select("HallID", "ParentID", "Name", "NameEN", "SortOrder", "Enabled").Updates(req).Error; e != nil {
			return e
		}
		return recordOperation(tx, actor(c).ID, "category", req.ID, "category_saved", "")
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, req)
}
func (a *App) saveRegion(c *gin.Context) {
	var req Region
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Name) == "" || len(req.Name) > 300 || len(req.NameEN) > 450 {
		fail(c, 400, "invalid_request")
		return
	}
	req.ID = id(c)
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		if req.ID == 0 {
			if e := tx.Create(&req).Error; e != nil {
				return e
			}
		} else {
			var old Region
			if e := tx.First(&old, req.ID).Error; e != nil {
				return e
			}
			if e := tx.Model(&old).Select("Name", "NameEN", "Enabled").Updates(req).Error; e != nil {
				return e
			}
		}
		return recordOperation(tx, actor(c).ID, "region", req.ID, "region_saved", "")
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, req)
}
