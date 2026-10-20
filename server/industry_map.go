package main

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"strconv"
)

var administrativeColumns = []string{"r.province_code", "r.city_code", "r.district_code"}

func industryRegion(c *gin.Context) (*AdministrativeRegion, bool) {
	loadAdministrativeRegions()
	code := c.Query("region_code")
	if code == "" {
		return nil, true
	}
	node, ok := administrativeByCode[code]
	if !ok {
		fail(c, 400, "invalid_region")
		return nil, false
	}
	return &node, true
}

func industryRegionQuery(q *gorm.DB, node *AdministrativeRegion) *gorm.DB {
	for node != nil {
		q = q.Where(administrativeColumns[node.Level-1]+" = ?", node.Code)
		if node.ParentCode == "" {
			break
		}
		parent := administrativeByCode[node.ParentCode]
		node = &parent
	}
	return q
}

func industryNextColumn(node *AdministrativeRegion) string {
	if node == nil {
		return administrativeColumns[0]
	}
	if !node.HasChildren {
		return ""
	}
	return administrativeColumns[node.Level]
}

func (a *App) industryQuery(c *gin.Context, node *AdministrativeRegion) (*gorm.DB, bool) {
	// Validate even when the deployment disables rate limiting.
	if !validPublicQuery(c) {
		fail(c, 400, "invalid_request")
		return nil, false
	}
	if value, exists := c.GetQuery("category_id"); exists {
		if n, e := strconv.ParseUint(value, 10, 64); e != nil || n == 0 {
			fail(c, 400, "invalid_category")
			return nil, false
		}
	}
	q, ok := a.hallQuery(c, a.publicQuery("vendor").WithContext(c.Request.Context()))
	if !ok {
		return nil, false
	}
	return industryRegionQuery(a.filterVendors(c, q), node), true
}

func (a *App) industryMapRegions(c *gin.Context) {
	node, ok := industryRegion(c)
	if !ok {
		return
	}
	q, ok := a.industryQuery(c, node)
	if !ok {
		return
	}
	column := industryNextColumn(node)
	var total, unassigned int64
	counts := map[string]int64{}
	if column == "" {
		if e := q.Distinct("contents.id").Count(&total).Error; e != nil {
			dbFail(c, e)
			return
		}
	} else {
		var groups []struct {
			Code  string
			Count int64
		}
		// One bounded aggregate per level, not one query per region or vendor.
		if e := q.Select("COALESCE(" + column + ", '') AS code, COUNT(DISTINCT contents.id) AS count").Group(column).Scan(&groups).Error; e != nil {
			dbFail(c, e)
			return
		}
		for _, group := range groups {
			counts[group.Code] = group.Count
			total += group.Count
		}
		unassigned = counts[""]
	}
	parent := ""
	trail := []AdministrativeRegion{}
	if node != nil {
		parent = node.Code
		for current := node; current != nil; {
			trail = append([]AdministrativeRegion{*current}, trail...)
			if current.ParentCode == "" {
				break
			}
			p := administrativeByCode[current.ParentCode]
			current = &p
		}
	}
	type regionCount struct {
		AdministrativeRegion
		VendorCount int64 `json:"vendor_count"`
	}
	items := []regionCount{}
	for _, child := range administrativeChildren[parent] {
		items = append(items, regionCount{child, counts[child.Code]})
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"region": node, "trail": trail, "items": items, "total": total, "unassigned_count": unassigned, "version": "2023-06-30", "historical": true, "coverage": "mainland-31"})
}

func (a *App) industryMapVendors(c *gin.Context) {
	node, ok := industryRegion(c)
	if !ok {
		return
	}
	q, ok := a.industryQuery(c, node)
	if !ok {
		return
	}
	if value, exists := c.GetQuery("unassigned"); exists {
		if value != "1" && value != "0" {
			fail(c, 400, "invalid_request")
			return
		}
		if value == "1" {
			if column := industryNextColumn(node); column != "" {
				q = q.Where(column + " IS NULL")
			} else {
				q = q.Where("1=0")
			}
		}
	}
	p, n := publicPage(c, 20)
	var total int64
	if e := q.Distinct("contents.id").Count(&total).Error; e != nil {
		dbFail(c, e)
		return
	}
	items, e := a.publicViews(q.Order("contents.featured DESC, contents.sort_order ASC, contents.id DESC").Offset((p - 1) * n).Limit(n))
	if e != nil {
		dbFail(c, e)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"items": items, "total": total, "page": p, "page_size": n})
}
