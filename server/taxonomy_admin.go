package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
)

func lockTaxonomy(tx *gorm.DB, hall uint64) error {
	var h Hall
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&h, hall).Error
}
func categoryReferenceCount(tx *gorm.DB, id uint64) int64 {
	var n, legacy int64
	ids := []uint64{id}
	var aliases []CategoryAlias
	tx.Find(&aliases)
	links := map[uint64]uint64{}
	for _, alias := range aliases {
		links[alias.SourceID] = alias.TargetID
	}
	for _, alias := range aliases {
		if canonicalAliasID(links, alias.SourceID) == id {
			ids = append(ids, alias.SourceID)
		}
	}
	tx.Model(&RevisionCategory{}).Where("category_id IN ?", ids).Count(&n)
	tx.Model(&Revision{}).Where("category_id IN ?", ids).Count(&legacy)
	return n + legacy
}
func (a *App) deleteCategory(c *gin.Context) {
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		var cat Category
		if e := tx.First(&cat, id(c)).Error; e != nil {
			return e
		}
		if e := lockTaxonomy(tx, cat.HallID); e != nil {
			return e
		}
		var children, aliases int64
		tx.Model(&Category{}).Where("parent_id=? AND deleted_at IS NULL", cat.ID).Count(&children)
		tx.Model(&CategoryAlias{}).Where("target_id=? OR source_id=?", cat.ID, cat.ID).Count(&aliases)
		if cat.CategoryKind == "legacy" || children > 0 || aliases > 0 || categoryReferenceCount(tx, cat.ID) > 0 {
			return fieldFailure("invalid_category", "category_id", "category_in_use")
		}
		if e := tx.Model(&cat).Updates(map[string]any{"deleted_at": time.Now().UTC(), "enabled": false}).Error; e != nil {
			return e
		}
		return recordOperation(tx, actor(c).ID, "category", cat.ID, "category_deleted", cat.Name)
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func mergeTaxonomyCategory(tx *gorm.DB, source, target, actorID uint64) error {
	var from, to Category
	if e := tx.First(&from, source).Error; e != nil {
		return e
	}
	if e := lockTaxonomy(tx, from.HallID); e != nil {
		return e
	}
	if canonicalCategory(tx, source) != source {
		return fmt.Errorf("invalid_category")
	}
	target = canonicalCategory(tx, target)
	if tx.First(&to, target).Error != nil || source == target || from.HallID != to.HallID || from.CategoryKind != to.CategoryKind || from.DeletedAt != nil || !selectableCategory(tx, target, to.CategoryKind, to.HallID, 0, false) || categoryDescends(tx, target, source) {
		return fmt.Errorf("invalid_category")
	}
	var children []Category
	if e := tx.Where("parent_id=? AND deleted_at IS NULL", source).Find(&children).Error; e != nil {
		return e
	}
	if len(children) > 0 {
		for _, child := range children {
			child.ParentID = target
			if e := validateCategoryTree(tx, child); e != nil {
				return e
			}
		}
		if e := tx.Model(&Category{}).Where("parent_id=?", source).Update("parent_id", target).Error; e != nil {
			return e
		}
	}
	if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&CategoryAlias{SourceID: source, TargetID: target}).Error; e != nil {
		return e
	}
	if e := tx.Model(&from).Update("enabled", false).Error; e != nil {
		return e
	}
	// Relations keep their original ID. Canonical alias lookup provides the merged view.
	return recordOperation(tx, actorID, "category", source, "category_merged", fmt.Sprintf("target=%d; references=%d", target, categoryReferenceCount(tx, source)))
}
func (a *App) mergeCategory(c *gin.Context) {
	var req struct {
		TargetID uint64 `json:"target_id"`
	}
	if c.ShouldBindJSON(&req) != nil || req.TargetID == 0 {
		fail(c, 400, "invalid_request")
		return
	}
	if e := a.DB.Transaction(func(tx *gorm.DB) error { return mergeTaxonomyCategory(tx, id(c), req.TargetID, actor(c).ID) }); e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func (a *App) taxonomyMappings(c *gin.Context) {
	p, n := page(c)
	q := a.DB.Table("taxonomy_mappings tm").Joins("JOIN revisions mr ON mr.id=tm.revision_id").Joins("JOIN contents mc ON mc.id=tm.content_id").Joins("LEFT JOIN categories lc ON lc.id=tm.legacy_category_id")
	if status := c.Query("status"); status != "" {
		q = q.Where("tm.status=?", status)
	}
	if kind := c.Query("category_kind"); kind != "" {
		q = q.Where("tm.category_kind=?", kind)
	}
	if c.Query("q") != "" {
		k := keyword(c)
		q = q.Where("mr.name LIKE ? OR lc.name LIKE ?", k, k)
	}
	var total int64
	if e := q.Count(&total).Error; e != nil {
		dbFail(c, e)
		return
	}
	type row struct {
		TaxonomyMapping
		Name           string `json:"name"`
		LegacyName     string `json:"legacy_name"`
		RevisionStatus string `json:"revision_status"`
		Current        bool   `json:"current"`
	}
	items := []row{}
	if e := q.Select("tm.*,mr.name,COALESCE(lc.name,'') AS legacy_name,mr.status AS revision_status,(mr.id=mc.draft_revision_id OR mr.id=mc.published_revision_id) AS current").Order("tm.revision_id DESC").Offset((p - 1) * n).Limit(n).Scan(&items).Error; e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"items": items, "total": total, "page": p, "page_size": n})
}
func (a *App) resolveTaxonomyMappings(c *gin.Context) {
	var req struct {
		RevisionIDs []uint64 `json:"revision_ids"`
		TargetID    uint64   `json:"target_id"`
		Preview     bool     `json:"preview"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.RevisionIDs) == 0 || len(req.RevisionIDs) > 100 {
		fail(c, 400, "invalid_request")
		return
	}
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		if e := lockTaxonomy(tx, 1); e != nil {
			return e
		}
		rows := []TaxonomyMapping{}
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("revision_id IN ?", req.RevisionIDs).Find(&rows).Error; e != nil {
			return e
		}
		if len(rows) != len(req.RevisionIDs) {
			return fmt.Errorf("stale_revision")
		}
		for _, r := range rows {
			var content Content
			if e := tx.First(&content, r.ContentID).Error; e != nil {
				return e
			}
			if !selectableCategory(tx, req.TargetID, r.CategoryKind, content.HallID, 0, r.CategoryKind == "product") {
				return fmt.Errorf("invalid_category")
			}
			if req.Preview {
				continue
			}
			if e := tx.Where("revision_id=?", r.RevisionID).Delete(&RevisionCategory{}).Error; e != nil {
				return e
			}
			role := "primary"
			if r.CategoryKind == "product" {
				role = "product"
			}
			if e := tx.Create(&RevisionCategory{r.RevisionID, req.TargetID, role, "manual"}).Error; e != nil {
				return e
			}
			if e := tx.Model(&r).Updates(map[string]any{"target_id": req.TargetID, "status": "manual", "updated_at": time.Now().UTC()}).Error; e != nil {
				return e
			}
			if e := recordOperation(tx, actor(c).ID, r.CategoryKind, r.ContentID, "taxonomy_mapping_resolved", fmt.Sprintf("revision=%d; target=%d", r.RevisionID, req.TargetID)); e != nil {
				return e
			}
		}
		return nil
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true, "preview": req.Preview, "count": len(req.RevisionIDs), "target_id": req.TargetID})
}
func (a *App) industrialBelts(c *gin.Context) {
	items := []IndustrialBelt{}
	q := a.DB.Order("sort_order,id")
	if c.Query("hall_id") != "" {
		q = q.Where("hall_id=?", positiveFilterID(c.Query("hall_id")))
	}
	if e := q.Find(&items).Error; e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"items": items})
}
func (a *App) saveIndustrialBelt(c *gin.Context) {
	var req IndustrialBelt
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Name) == "" || len(req.Name) > 300 || len(req.NameEN) > 300 {
		fail(c, 400, "invalid_request")
		return
	}
	req.ID = id(c)
	if req.HallID == 0 {
		req.HallID = 1
	}
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		if e := lockTaxonomy(tx, req.HallID); e != nil {
			return e
		}
		if req.ID == 0 {
			if e := tx.Create(&req).Error; e != nil {
				return e
			}
		} else {
			var old IndustrialBelt
			if e := tx.First(&old, req.ID).Error; e != nil {
				return e
			}
			if old.HallID != req.HallID {
				return fmt.Errorf("invalid_request")
			}
			if e := tx.Model(&old).Select("Name", "NameEN", "SortOrder", "Enabled").Updates(req).Error; e != nil {
				return e
			}
		}
		return recordOperation(tx, actor(c).ID, "industrial_belt", req.ID, "industrial_belt_saved", req.Name)
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, req)
}
func (a *App) beltVendors(c *gin.Context) {
	var belt IndustrialBelt
	if e := a.DB.First(&belt, id(c)).Error; e != nil {
		dbFail(c, e)
		return
	}
	var ids []uint64
	if e := a.DB.Model(&VendorIndustrialBelt{}).Where("belt_id=?", belt.ID).Pluck("vendor_id", &ids).Error; e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"vendor_ids": ids})
}
func (a *App) beltVendorOptions(c *gin.Context) {
	var belt IndustrialBelt
	if e := a.DB.First(&belt, id(c)).Error; e != nil {
		dbFail(c, e)
		return
	}
	p, n := publicPage(c, 20)
	q := a.DB.Table("contents bv").Joins("JOIN revisions br ON br.id=bv.draft_revision_id").Where("bv.kind='vendor' AND bv.hall_id=?", belt.HallID)
	if c.Query("q") != "" {
		q = q.Where("br.name LIKE ?", keyword(c))
	}
	var total int64
	if e := q.Count(&total).Error; e != nil {
		dbFail(c, e)
		return
	}
	items := []struct {
		ID   uint64 `json:"id"`
		Name string `json:"name"`
	}{}
	if e := q.Select("bv.id,br.name").Order("bv.id").Offset((p - 1) * n).Limit(n).Scan(&items).Error; e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"items": items, "total": total, "page": p, "page_size": n})
}
func (a *App) saveBeltVendors(c *gin.Context) {
	var req struct {
		VendorIDs []uint64 `json:"vendor_ids"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.VendorIDs) > 10000 {
		fail(c, 400, "invalid_request")
		return
	}
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		var belt IndustrialBelt
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&belt, id(c)).Error; e != nil {
			return e
		}
		unique := map[uint64]bool{}
		rows := []VendorIndustrialBelt{}
		for _, id := range req.VendorIDs {
			if unique[id] {
				continue
			}
			unique[id] = true
			var n int64
			if e := tx.Model(&Content{}).Where("id=? AND kind='vendor' AND hall_id=?", id, belt.HallID).Count(&n).Error; e != nil {
				return e
			}
			if n != 1 {
				return fmt.Errorf("invalid_request")
			}
			rows = append(rows, VendorIndustrialBelt{id, belt.ID})
		}
		if e := tx.Where("belt_id=?", belt.ID).Delete(&VendorIndustrialBelt{}).Error; e != nil {
			return e
		}
		if len(rows) > 0 {
			if e := tx.CreateInBatches(rows, 100).Error; e != nil {
				return e
			}
		}
		return recordOperation(tx, actor(c).ID, "industrial_belt", belt.ID, "industrial_belt_vendors_saved", fmt.Sprintf("vendors=%d", len(rows)))
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
