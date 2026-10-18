package main

import (
	"fmt"
	"gorm.io/gorm"
	"strconv"
	"strings"
)

// Each UNION branch uses an indexed manufacturer scope and its own LIMIT. This
// avoids a query per manufacturer, and works without MySQL 8 window functions.
func unionCandidateIDs(db *gorm.DB, queries []*gorm.DB, grouped bool) ([]uint64, error) {
	parts := []string{}
	args := []any{}
	for i, q := range queries {
		st := q.Session(&gorm.Session{DryRun: true}).Find(&[]Content{}).Statement
		column := "id"
		if grouped {
			column = "ids"
		}
		parts = append(parts, fmt.Sprintf("SELECT %s FROM (%s) AS candidate_%d", column, st.SQL.String(), i))
		args = append(args, st.Vars...)
	}
	ids := []uint64{}
	seen := map[uint64]bool{}
	if len(parts) == 0 {
		return ids, nil
	}
	if grouped {
		var rows []struct{ IDs string }
		if e := db.Raw(strings.Join(parts, " UNION ALL "), args...).Scan(&rows).Error; e != nil {
			return nil, e
		}
		for _, r := range rows {
			for _, text := range strings.Split(r.IDs, ",") {
				id, _ := strconv.ParseUint(text, 10, 64)
				if id > 0 && !seen[id] {
					seen[id] = true
					ids = append(ids, id)
				}
			}
		}
	} else {
		var rows []struct{ ID uint64 }
		if e := db.Raw(strings.Join(parts, " UNION ALL "), args...).Scan(&rows).Error; e != nil {
			return nil, e
		}
		for _, r := range rows {
			if !seen[r.ID] {
				seen[r.ID] = true
				ids = append(ids, r.ID)
			}
		}
	}
	return ids, nil
}
func (a *App) batchRepresentatives(vendors []View, base *gorm.DB, order string) error {
	queries := []*gorm.DB{}
	for _, v := range vendors {
		queries = append(queries, base.Session(&gorm.Session{}).Where("contents.vendor_id=?", v.ID).Select("contents.id").Order(order).Limit(3))
	}
	ids, e := unionCandidateIDs(a.DB, queries, false)
	if e != nil {
		return e
	}
	items := []View{}
	if len(ids) > 0 {
		items, e = a.publicViews(a.publicQuery("product").Where("contents.id IN ?", ids).Order(order))
		if e != nil {
			return e
		}
	}
	byVendor := map[uint64][]View{}
	for _, p := range items {
		byVendor[p.VendorID] = append(byVendor[p.VendorID], p)
	}
	for i := range vendors {
		vendors[i].MatchingProducts = byVendor[vendors[i].ID]
		if vendors[i].MatchingProducts == nil {
			vendors[i].MatchingProducts = []View{}
		}
	}
	return nil
}
func (a *App) homeVendors(hallID uint64) ([]View, error) {
	base := func() *gorm.DB { return a.publicQuery("vendor").Where("contents.hall_id=?", hallID) }
	// Reserve bounded candidates independently for categories and regions, so
	// many regions in a recent dominant category cannot crowd out a rare category.
	ids := []uint64{0}
	seen := map[uint64]bool{0: true}
	for _, scope := range []struct {
		field string
		limit int
	}{{"r.category_id", 24}, {"r.region_id", 16}} {
		var groups []struct{ ID uint64 }
		if e := base().Select("MAX(contents.id) AS id").Group("contents.featured,contents.sort_order," + scope.field).Order("contents.featured DESC,contents.sort_order,MAX(contents.id) DESC").Limit(scope.limit).Scan(&groups).Error; e != nil {
			return nil, e
		}
		for _, g := range groups {
			if !seen[g.ID] {
				ids = append(ids, g.ID)
				seen[g.ID] = true
			}
		}
	}

	var extra []struct{ ID uint64 }
	if e := base().Where("contents.id NOT IN ?", ids).Select("contents.id").Order("contents.featured DESC,contents.sort_order,contents.id DESC").Limit(49 - len(ids)).Scan(&extra).Error; e != nil {
		return nil, e
	}
	for _, v := range extra {
		ids = append(ids, v.ID)
	}
	candidates, e := a.publicViews(base().Where("contents.id IN ?", ids).Order("contents.featured DESC,contents.sort_order,contents.id DESC"))
	if e != nil {
		return nil, e
	}
	selected := []View{}
	cats, regions := map[uint64]int{}, map[uint64]int{}
	for len(selected) < 8 && len(candidates) > 0 {
		best := 0
		for i := 1; i < len(candidates); i++ {
			x, y := candidates[i], candidates[best]
			if x.Featured != y.Featured {
				if x.Featured {
					best = i
				}
				continue
			}
			if x.SortOrder != y.SortOrder {
				if x.SortOrder < y.SortOrder {
					best = i
				}
				continue
			}
			if cats[x.Data.CategoryID]*10+regions[x.Data.RegionID] < cats[y.Data.CategoryID]*10+regions[y.Data.RegionID] {
				best = i
			}
		}
		p := candidates[best]
		selected = append(selected, p)
		cats[p.Data.CategoryID]++
		regions[p.Data.RegionID]++
		candidates = append(candidates[:best], candidates[best+1:]...)
	}
	return selected, nil
}
func (a *App) homeProducts(hallID uint64) ([]View, error) {
	base := func() *gorm.DB { return a.publicQuery("product").Where("contents.hall_id=?", hallID) }
	var suppliers []struct{ VendorID uint64 }
	if e := base().Select("contents.vendor_id").Group("contents.vendor_id").Order("MAX(contents.featured) DESC,MIN(contents.sort_order),MAX(r.reviewed_at) DESC,MAX(contents.id) DESC").Limit(24).Scan(&suppliers).Error; e != nil {
		return nil, e
	}
	queries := []*gorm.DB{}
	for _, v := range suppliers {
		queries = append(queries, base().Where("contents.vendor_id=?", v.VendorID).Select("SUBSTRING_INDEX(GROUP_CONCAT(contents.id ORDER BY contents.featured DESC,contents.sort_order,r.reviewed_at DESC,contents.id DESC SEPARATOR ','),',',3) AS ids").Group("r.category_id").Order("MAX(contents.featured) DESC,MIN(contents.sort_order),MAX(r.reviewed_at) DESC,MAX(contents.id) DESC").Limit(6))
	}
	ids, e := unionCandidateIDs(a.DB, queries, true)
	if e != nil {
		return nil, e
	}
	if len(ids) == 0 {
		return []View{}, nil
	}
	candidates, e := a.publicViews(base().Where("contents.id IN ?", ids).Order("contents.featured DESC,contents.sort_order,r.reviewed_at DESC,contents.id DESC"))
	if e != nil {
		return nil, e
	}
	selected := []View{}
	vendorUse, categoryUse := map[uint64]int{}, map[uint64]int{}
	seen := map[uint64]bool{}
	for round := 1; round <= 3 && len(selected) < 12; round++ {
		for len(selected) < 12 {
			best := -1
			last := uint64(0)
			if len(selected) > 0 {
				last = selected[len(selected)-1].VendorID
			}
			better := func(x, y View) bool {
				if (x.VendorID == last) != (y.VendorID == last) {
					return x.VendorID != last
				}
				if x.Featured != y.Featured {
					return x.Featured
				}
				if x.SortOrder != y.SortOrder {
					return x.SortOrder < y.SortOrder
				}
				return categoryUse[x.Data.CategoryID] < categoryUse[y.Data.CategoryID]
			}
			for i, p := range candidates {
				if seen[p.ID] || vendorUse[p.VendorID] >= round {
					continue
				}
				if best < 0 || better(p, candidates[best]) {
					best = i
				}
			}
			if best < 0 {
				break
			}
			p := candidates[best]
			selected = append(selected, p)
			seen[p.ID] = true
			vendorUse[p.VendorID]++
			categoryUse[p.Data.CategoryID]++
		}
	}
	return selected, nil
}
