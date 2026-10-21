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

func (a *App) publicQuery(kind string, includePreparing ...bool) *gorm.DB {
	q := a.DB.Model(&Content{})
	if len(includePreparing) == 0 || !includePreparing[0] {
		q = q.Where("EXISTS (SELECT 1 FROM halls h WHERE h.id=contents.hall_id AND h.status='open')")
	}
	q = q.Joins("JOIN revisions r ON r.id = contents.published_revision_id AND r.status = 'approved'").Where("contents.kind = ? AND contents.visible = 1", kind)
	if kind == "product" {
		q = q.Joins("JOIN contents v ON v.id = contents.vendor_id AND v.kind = 'vendor' AND v.visible = 1 AND v.hall_id = contents.hall_id").Joins("JOIN revisions vr ON vr.id = v.published_revision_id AND vr.status = 'approved'")
	}
	return q
}
func (a *App) publicViews(q *gorm.DB) ([]View, error) {
	var rows []Content
	if e := q.Select("contents.*").Find(&rows).Error; e != nil {
		return nil, e
	}
	items := []View{}
	ids := []uint64{}
	for _, row := range rows {
		ids = append(ids, row.PublishedRevisionID)
	}
	revisions := []Revision{}
	if len(ids) > 0 {
		if e := a.DB.Where("id IN ?", ids).Find(&revisions).Error; e != nil {
			return nil, e
		}
	}
	byID := map[uint64]Revision{}
	for _, rev := range revisions {
		byID[rev.ID] = rev
	}
	for _, content := range rows {
		rev, ok := byID[content.PublishedRevisionID]
		if !ok {
			return nil, gorm.ErrRecordNotFound
		}
		content.OwnerID = 0
		content.DraftRevisionID = 0
		content.VendorOwnerKey = nil
		content.Epoch = 0
		items = append(items, view(content, rev))
	}
	if e := a.resolveCategoryViews(items); e != nil {
		return nil, e
	}
	return items, nil
}
func (a *App) categoryIDs(category string) []uint64 {
	raw, _ := strconv.ParseUint(category, 10, 64)
	if raw == 0 {
		return nil
	}
	return categoryGraphIDs(a.DB, []uint64{raw})
}

func specKeywordCondition(alias string) string {
	return fmt.Sprintf("(%s.name LIKE ? OR %s.name_en LIKE ? OR %s.main_products LIKE ? OR %s.keywords LIKE ? OR JSON_UNQUOTE(JSON_EXTRACT(%s.payload, '$.parameters')) LIKE ?)", alias, alias, alias, alias, alias)
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
func positiveFilterID(value string) uint64 {
	n, e := strconv.ParseUint(value, 10, 64)
	if e != nil || n == 0 {
		return ^uint64(0)
	}
	return n
}
func (a *App) filterProducts(c *gin.Context, q *gorm.DB) *gorm.DB {
	if strings.TrimSpace(c.Query("q")) != "" {
		k := keyword(c)
		q = q.Where("("+specKeywordCondition("r")+" OR JSON_UNQUOTE(JSON_EXTRACT(r.payload,'$.brand')) LIKE ? OR JSON_UNQUOTE(JSON_EXTRACT(r.payload,'$.model')) LIKE ? OR JSON_UNQUOTE(JSON_EXTRACT(r.payload,'$.applicable_brands')) LIKE ? OR JSON_UNQUOTE(JSON_EXTRACT(r.payload,'$.applicable_models')) LIKE ? OR EXISTS (SELECT 1 FROM revision_categories rc WHERE rc.revision_id=r.id AND rc.category_id IN ?))", k, k, k, k, k, k, k, k, k, categoryKeywordIDs(a.DB, k))
	}
	if ids, modern := a.newCategoryFilter(c.Query("category_id"), "product"); modern {
		if len(ids) == 0 {
			return q.Where("1=0")
		}
		q = taxonomyFilter(q, "r", ids)
	} else if ids := a.categoryIDs(c.Query("category_id")); c.Query("category_id") != "" {
		if len(ids) == 0 {
			return q.Where("1=0")
		}
		q = q.Where("r.category_id IN ?", ids)
	}
	if vendorID := c.Query("vendor_id"); vendorID != "" {
		q = q.Where("contents.vendor_id = ?", positiveFilterID(vendorID))
	}
	if regionID := c.Query("region_id"); regionID != "" {
		q = q.Where("vr.region_id = ?", positiveFilterID(regionID))
	}
	return filterBelt(c, q, "contents.vendor_id")
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
func publicPage(c *gin.Context, size int) (int, int) {
	p, n := page(c)
	requested, e := strconv.Atoi(c.Query("page_size"))
	if e != nil || requested < 1 || requested > size {
		n = size
	}
	return p, n
}
func (a *App) filterVendors(c *gin.Context, q *gorm.DB) *gorm.DB {
	productMatch := "SELECT 1 FROM contents p JOIN revisions pr ON pr.id=p.published_revision_id AND pr.status='approved' WHERE p.vendor_id=contents.id AND p.hall_id=contents.hall_id AND p.kind='product' AND p.visible=1"
	if strings.TrimSpace(c.Query("q")) != "" {
		k := keyword(c)
		q = q.Where("("+specKeywordCondition("r")+" OR EXISTS ("+productMatch+" AND "+specKeywordCondition("pr")+"))", k, k, k, k, k, k, k, k, k, k)
	}
	if ids, modern := a.newCategoryFilter(c.Query("category_id"), "vendor"); modern {
		if len(ids) == 0 {
			return q.Where("1=0")
		}
		q = taxonomyFilter(q, "r", ids)
	} else if ids := a.categoryIDs(c.Query("category_id")); c.Query("category_id") != "" {
		if len(ids) == 0 {
			return q.Where("1=0")
		}
		q = q.Where("(r.category_id IN ? OR EXISTS ("+productMatch+" AND pr.category_id IN ?))", ids, ids)
	}
	if nature := c.Query("business_nature"); nature != "" {
		q = q.Where("JSON_UNQUOTE(JSON_EXTRACT(r.payload,'$.business_nature'))=?", nature)
	}
	return filterBelt(c, q, "contents.id")
}
func (a *App) searchVendors(c *gin.Context) {
	p, n := publicPage(c, 20)
	q, ok := a.hallQuery(c, a.publicQuery("vendor"))
	if !ok {
		return
	}
	q = a.filterVendors(c, q)
	if region := c.Query("region_id"); region != "" {
		q = q.Where("r.region_id = ?", positiveFilterID(region))
	}
	var total int64
	if e := q.Count(&total).Error; e != nil {
		dbFail(c, e)
		return
	}
	items, e := a.publicViews(q.Order(publicOrder(c, "vendor")).Offset((p - 1) * n).Limit(n))
	if e != nil {
		dbFail(c, e)
		return
	}
	if e = a.batchRepresentatives(items, a.filterProducts(c, a.publicQuery("product")), "contents.id DESC"); e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"items": items, "total": total, "page": p, "page_size": n})
}
func (a *App) publicProducts(c *gin.Context) {
	p, n := publicPage(c, 24)
	base, ok := a.hallQuery(c, a.publicQuery("product"))
	if !ok {
		return
	}
	q := a.filterProducts(c, base)
	var total int64
	if e := q.Count(&total).Error; e != nil {
		dbFail(c, e)
		return
	}
	items, e := a.publicViews(q.Order(publicOrder(c, "product")).Offset((p - 1) * n).Limit(n))
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
			p, n := publicPage(c, 24)
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
func publicOrder(c *gin.Context, kind string) string {
	switch c.Query("sort") {
	case "latest":
		if kind == "vendor" {
			return "contents.created_at DESC, contents.id DESC"
		}
		return "r.reviewed_at DESC, contents.id DESC"
	case "name":
		if kind == "vendor" {
			return "r.name ASC, contents.id ASC"
		}
	}
	return "contents.featured DESC, contents.sort_order ASC, contents.id DESC"
}
func (a *App) home(c *gin.Context) {
	hall, ok := a.requestHall(c)
	if !ok {
		return
	}
	if hall.Status != "open" {
		c.JSON(200, gin.H{"hall": hall, "hero": gin.H{"media_id": 0}, "vendors": []View{}, "products": []View{}, "vendor_count": 0})
		return
	}
	vendors, e := a.homeVendors(hall.ID)
	if e != nil {
		dbFail(c, e)
		return
	}
	if e = a.batchRepresentatives(vendors, a.publicQuery("product").Where("contents.hall_id=?", hall.ID), "contents.featured DESC, contents.sort_order, contents.id DESC"); e != nil {
		dbFail(c, e)
		return
	}
	products, e := a.homeProducts(hall.ID)
	if e != nil {
		dbFail(c, e)
		return
	}
	if e = a.productVendorSummaries(products); e != nil {
		dbFail(c, e)
		return
	}
	var count int64
	if e = a.publicQuery("vendor").Where("contents.hall_id=?", hall.ID).Count(&count).Error; e != nil {
		dbFail(c, e)
		return
	}
	var settings HomeDisplaySettings
	if e = a.DB.First(&settings, hall.ID).Error; e != nil {
		dbFail(c, e)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"hall": hall, "hero": gin.H{"media_id": settings.HeroMediaID}, "vendors": vendors, "products": products, "vendor_count": count})
}
func (a *App) taxonomy(c *gin.Context)    { a.taxonomyResult(c, false) }
func (a *App) allTaxonomy(c *gin.Context) { a.taxonomyResult(c, true) }
func (a *App) taxonomyResult(c *gin.Context, all bool) {
	var scopedHall uint64
	if !all || c.Query("hall") != "" {
		hall, ok := a.requestHall(c)
		if !ok {
			return
		}
		scopedHall = hall.ID
	}
	if all && !platformAccount(actor(c)) {
		var booth Content
		a.DB.Where("kind='vendor' AND owner_id=?", actor(c).ID).First(&booth)
		if booth.HallID > 0 {
			scopedHall = booth.HallID
		} else if scopedHall == 0 {
			scopedHall = 1
		}
	}
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
		rq = rq.Where("enabled=1 AND id IN (SELECT r.region_id FROM contents c JOIN revisions r ON r.id=c.published_revision_id AND r.status='approved' JOIN halls h ON h.id=c.hall_id AND h.status='open' WHERE c.kind='vendor' AND c.visible=1 AND c.hall_id=?)", scopedHall)
	}
	if e := rq.Order("id").Find(&regions).Error; e != nil {
		dbFail(c, e)
		return
	}
	scoped := a.scopedCategories(func() Account {
		if all {
			return actor(c)
		}
		return Account{}
	}(), !all && c.Query("category_kind") != "all")
	filtered := []ScopedCategory{}
	for _, cat := range scoped {
		if scopedHall == 0 || cat.HallID == scopedHall {
			kind := c.Query("category_kind")
			if kind == "" {
				kind = "legacy"
			}
			if kind != "all" && cat.CategoryKind != kind {
				continue
			}
			filtered = append(filtered, cat)
		}
	}
	belts := []IndustrialBelt{}
	bq := a.DB.Order("sort_order,id")
	if scopedHall > 0 {
		bq = bq.Where("hall_id=?", scopedHall)
	}
	if !all {
		bq = bq.Where("enabled=1")
	}
	if e := bq.Find(&belts).Error; e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"halls": halls, "categories": filtered, "regions": regions, "aliases": a.categoryAliases(), "industrial_belts": belts})
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
		var previousImage uint64
		if req.ID > 0 {
			var existing Category
			if e := tx.First(&existing, req.ID).Error; e != nil {
				return e
			}
			if req.CategoryKind == "" {
				req.CategoryKind = existing.CategoryKind
			}
			if req.CategoryKind != existing.CategoryKind || existing.DeletedAt != nil {
				return fmt.Errorf("invalid_category")
			}
			if req.Enabled && canonicalCategory(tx, existing.ID) != existing.ID {
				return fmt.Errorf("invalid_category")
			}
			req.Code = existing.Code
			previousImage = existing.ImageMediaID
		} else if req.CategoryKind == "" {
			req.CategoryKind = "legacy"
		}
		if e := validateCategoryTree(tx, req); e != nil {
			return e
		}
		if req.ImageMediaID > 0 && req.ImageMediaID != previousImage {
			var m Media
			if tx.First(&m, req.ImageMediaID).Error != nil || m.OwnerID != actor(c).ID {
				return fmt.Errorf("invalid_media")
			}
		}
		if len(req.SearchAliases) > 3000 {
			return fmt.Errorf("invalid_request")
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
		if old.HallID != req.HallID {
			return fmt.Errorf("invalid_category")
		}
		if e := tx.Model(&old).Select("HallID", "ParentID", "Name", "NameEN", "SortOrder", "Enabled", "ImageMediaID", "SearchAliases").Updates(req).Error; e != nil {
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

func keywordCondition(alias string) string {
	return fmt.Sprintf("(%s.name LIKE ? OR %s.name_en LIKE ? OR %s.main_products LIKE ? OR %s.keywords LIKE ?)", alias, alias, alias, alias)
}

// Preserve each manufacturer's selected order while avoiding long supplier runs.
func spreadHomeProducts(items []View) []View {
	remaining := append([]View{}, items...)
	result := []View{}
	vendors := map[uint64]int{}
	categories := map[uint64]int{}
	for len(remaining) > 0 {
		best := 0
		score := func(v View) int { return vendors[v.VendorID]*100 + categories[effectiveViewCategory(v)]*10 }
		seenVendors := map[uint64]bool{remaining[0].VendorID: true}
		for i := 1; i < len(remaining); i++ {
			if seenVendors[remaining[i].VendorID] {
				continue
			}
			seenVendors[remaining[i].VendorID] = true
			if score(remaining[i]) < score(remaining[best]) {
				best = i
			}
		}
		next := remaining[best]
		result = append(result, next)
		vendors[next.VendorID]++
		categories[effectiveViewCategory(next)]++
		remaining = append(remaining[:best], remaining[best+1:]...)
	}
	return result
}
