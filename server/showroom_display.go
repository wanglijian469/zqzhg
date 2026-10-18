package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

const representativeLimit = 3

type ShowroomDisplaySettings struct {
	VendorID   uint64 `gorm:"primaryKey"`
	Mode       string
	ProductIDs string
	Version    uint64
	UpdatedBy  uint64
	UpdatedAt  time.Time
}

func (ShowroomDisplaySettings) TableName() string { return "showroom_display_settings" }
func (a *App) displaySettings(vendorID uint64) (ShowroomDisplaySettings, []uint64, error) {
	s := ShowroomDisplaySettings{VendorID: vendorID, Mode: "auto", ProductIDs: "[]"}
	e := a.DB.First(&s, "vendor_id=?", vendorID).Error
	if e != nil && e != gorm.ErrRecordNotFound {
		return s, nil, e
	}
	ids := []uint64{}
	if e = json.Unmarshal([]byte(s.ProductIDs), &ids); e != nil {
		return s, nil, e
	}
	return s, ids, nil
}
func (a *App) representativeProducts(vendorID uint64) ([]View, error) {
	s, ids, e := a.displaySettings(vendorID)
	if e != nil {
		return nil, e
	}
	q := a.publicQuery("product").Where("contents.vendor_id=?", vendorID)
	if s.Mode == "auto" {
		return a.publicViews(q.Order("contents.featured DESC, contents.sort_order ASC, contents.id DESC").Limit(representativeLimit))
	}
	if len(ids) == 0 {
		return []View{}, nil
	}
	items, e := a.publicViews(q.Where("contents.id IN ?", ids))
	if e != nil {
		return nil, e
	}
	byID := map[uint64]View{}
	for _, v := range items {
		byID[v.ID] = v
	}
	result := []View{}
	for _, id := range ids {
		if v, ok := byID[id]; ok && len(result) < representativeLimit {
			result = append(result, v)
		}
	}
	return result, nil
}
func (a *App) ownedDisplayVendor(c *gin.Context) (Content, error) {
	var v Content
	e := a.DB.Where("kind='vendor' AND owner_id=?", actor(c).ID).First(&v).Error
	if e == gorm.ErrRecordNotFound {
		return v, fmt.Errorf("vendor_required")
	}
	return v, e
}
func (a *App) merchantShowroomDisplay(c *gin.Context) {
	v, e := a.ownedDisplayVendor(c)
	if e != nil {
		dbFail(c, e)
		return
	}
	s, ids, e := a.displaySettings(v.ID)
	if e != nil {
		dbFail(c, e)
		return
	}
	products, e := a.representativeProducts(v.ID)
	if e != nil {
		dbFail(c, e)
		return
	}
	byID := map[uint64]View{}
	for _, p := range products {
		byID[p.ID] = p
	}
	selected := []gin.H{}
	for _, id := range ids {
		row := gin.H{"product_id": id, "public_visible": false}
		if p, ok := byID[id]; ok {
			row["public_visible"] = true
			row["item"] = p
		}
		selected = append(selected, row)
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"vendor_id": v.ID, "mode": s.Mode, "product_ids": ids, "version": s.Version, "limit": representativeLimit, "selected": selected, "preview_products": products})
}
func (a *App) saveMerchantShowroomDisplay(c *gin.Context) {
	var req struct {
		Mode       string   `json:"mode"`
		ProductIDs []uint64 `json:"product_ids"`
		Version    *uint64  `json:"version"`
	}
	if c.ShouldBindJSON(&req) != nil || req.Version == nil || (req.Mode != "auto" && req.Mode != "manual") {
		fail(c, 400, "invalid_request")
		return
	}
	if len(req.ProductIDs) > representativeLimit {
		fail(c, 409, "representative_limit_reached")
		return
	}
	seen := map[uint64]bool{}
	for _, id := range req.ProductIDs {
		if id == 0 || seen[id] {
			dbFail(c, fieldFailure("invalid_request", "product_ids", "unique_product_ids_required"))
			return
		}
		seen[id] = true
	}
	if req.Mode == "auto" && len(req.ProductIDs) > 0 {
		fail(c, 400, "invalid_request")
		return
	}
	v, e := a.ownedDisplayVendor(c)
	if e != nil {
		dbFail(c, e)
		return
	}
	e = a.DB.Transaction(func(tx *gorm.DB) error {
		var locked Content
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, "id=? AND owner_id=? AND kind='vendor'", v.ID, actor(c).ID).Error; e != nil {
			return e
		}
		fresh := ShowroomDisplaySettings{VendorID: v.ID, Mode: "auto", ProductIDs: "[]", UpdatedAt: time.Now().UTC()}
		if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&fresh).Error; e != nil {
			return e
		}
		var s ShowroomDisplaySettings
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&s, "vendor_id=?", v.ID).Error; e != nil {
			return e
		}
		if s.Version != *req.Version {
			return fmt.Errorf("stale_revision")
		}
		if len(req.ProductIDs) > 0 {
			var count int64
			ta := &App{DB: tx}
			if e := ta.publicQuery("product").Where("contents.vendor_id=? AND contents.owner_id=? AND contents.id IN ?", v.ID, actor(c).ID, req.ProductIDs).Count(&count).Error; e != nil {
				return e
			}
			if count != int64(len(req.ProductIDs)) {
				return fieldFailure("invalid_request", "product_ids", "public_owned_products_required")
			}
		}
		ids := req.ProductIDs
		if ids == nil {
			ids = []uint64{}
		}
		b, _ := json.Marshal(ids)
		s.Mode = req.Mode
		s.ProductIDs = string(b)
		s.Version++
		s.UpdatedBy = actor(c).ID
		s.UpdatedAt = time.Now().UTC()
		if e := tx.Save(&s).Error; e != nil {
			return e
		}
		return recordOperation(tx, actor(c).ID, "vendor", v.ID, "showroom_representatives_saved", fmt.Sprintf("mode=%s product_ids=%s", s.Mode, s.ProductIDs))
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	a.merchantShowroomDisplay(c)
}

// showroomHomeProducts preserves the three-slot configuration and fills only the homepage.
func (a *App) showroomHomeProducts(vendorID uint64, representatives []View) ([]View, error) {
	settings, ids, err := a.displaySettings(vendorID)
	if err != nil {
		return nil, err
	}
	result := []View{}
	if settings.Mode == "manual" && len(ids) == 0 {
		return result, nil
	}
	seen := map[uint64]bool{}
	for _, item := range representatives {
		if !seen[item.ID] && len(result) < 8 {
			result = append(result, item)
			seen[item.ID] = true
		}
	}
	q := a.publicQuery("product").Where("contents.vendor_id=?", vendorID)
	if len(result) > 0 {
		selected := []uint64{}
		for id := range seen {
			selected = append(selected, id)
		}
		q = q.Where("contents.id NOT IN ?", selected)
	}
	extras, err := a.publicViews(q.Order("contents.featured DESC, contents.sort_order ASC, contents.id DESC").Limit(8 - len(result)))
	if err != nil {
		return nil, err
	}
	return append(result, extras...), nil
}
