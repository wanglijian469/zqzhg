package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type CompanyContact struct {
	Area   string `json:"area"`
	AreaEN string `json:"area_en"`
	Name   string `json:"name"`
	NameEN string `json:"name_en"`
	Phone  string `json:"phone"`
	Wechat string `json:"wechat"`
}
type CompanyProfile struct {
	SupplyProductCategoryIDs []uint64            `json:"supply_product_category_ids"`
	SupplyRegionCodes        []string            `json:"supply_region_codes"`
	Credentials              []CompanyCredential `json:"credentials"`
	GalleryAnnotations       []GalleryAnnotation `json:"gallery_annotations"`
	ShortName                string              `json:"short_name"`
	ShortNameEN              string              `json:"short_name_en"`
	Summary                  string              `json:"summary"`
	SummaryEN                string              `json:"summary_en"`
	LogoMediaID              uint64              `json:"logo_media_id"`
	CoverMediaID             uint64              `json:"cover_media_id"`
	Established              string              `json:"established"`
	Staff                    string              `json:"staff"`
	FactoryArea              string              `json:"factory_area"`
	Equipment                string              `json:"equipment"`
	EquipmentEN              string              `json:"equipment_en"`
	Capacity                 string              `json:"capacity"`
	CapacityEN               string              `json:"capacity_en"`
	Website                  string              `json:"website"`
	ContactName              string              `json:"contact_name"`
	ContactNameEN            string              `json:"contact_name_en"`
	Contacts                 []CompanyContact    `json:"contacts"`
}

type GalleryAnnotation struct {
	MediaID       uint64 `json:"media_id"`
	Title         string `json:"title"`
	TitleEN       string `json:"title_en"`
	Description   string `json:"description"`
	DescriptionEN string `json:"description_en"`
}
type CompanyCredential struct {
	GalleryAnnotation
	Kind string `json:"kind"`
}

// Omitted collections preserve old-client data; explicit [] clears it.
func mergePortalProfile(next *Payload, previous Payload) {
	if next.CompanyProfile == nil {
		next.CompanyProfile = previous.CompanyProfile
	}
	if next.CompanyProfile == nil {
		return
	}
	if previous.CompanyProfile != nil {
		if next.CompanyProfile.SupplyProductCategoryIDs == nil {
			next.CompanyProfile.SupplyProductCategoryIDs = previous.CompanyProfile.SupplyProductCategoryIDs
		}
		if next.CompanyProfile.SupplyRegionCodes == nil {
			next.CompanyProfile.SupplyRegionCodes = previous.CompanyProfile.SupplyRegionCodes
		}
		if next.CompanyProfile.Credentials == nil {
			next.CompanyProfile.Credentials = previous.CompanyProfile.Credentials
		}
		if next.CompanyProfile.GalleryAnnotations == nil {
			next.CompanyProfile.GalleryAnnotations = previous.CompanyProfile.GalleryAnnotations
		}
	}
	ids := map[uint64]bool{}
	for _, mid := range next.MediaIDs {
		ids[mid] = true
	}
	annotations := []GalleryAnnotation{}
	for _, row := range next.CompanyProfile.GalleryAnnotations {
		if ids[row.MediaID] {
			annotations = append(annotations, row)
		}
	}
	next.CompanyProfile.GalleryAnnotations = annotations
}

func showroomSlug(c Content) string {
	if c.Kind != "vendor" {
		return ""
	}
	if c.ShowroomSlug != nil {
		return *c.ShowroomSlug
	}
	return fmt.Sprintf("v-%d", c.ID)
}
func payloadMedia(p Payload) []uint64 {
	ids := append([]uint64{}, p.MediaIDs...)
	if p.CompanyProfile != nil {
		ids = append(ids, p.CompanyProfile.LogoMediaID, p.CompanyProfile.CoverMediaID)
		for _, row := range p.CompanyProfile.Credentials {
			ids = append(ids, row.MediaID)
		}
	}
	seen := map[uint64]bool{}
	result := []uint64{}
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}
func validateCompanyProfile(p *CompanyProfile) error {
	if p == nil {
		return nil
	}
	if len(p.SupplyProductCategoryIDs) > 35 || len(p.SupplyRegionCodes) > 50 {
		return fmt.Errorf("invalid_request")
	}
	for _, code := range p.SupplyRegionCodes {
		if !procValidRegion(code) {
			return fmt.Errorf("invalid_request")
		}
	}
	for _, s := range []string{p.ShortName, p.ShortNameEN, p.Established, p.Staff, p.FactoryArea, p.ContactName, p.ContactNameEN} {
		if utf8.RuneCountInString(s) > 200 {
			return fmt.Errorf("invalid_request")
		}
	}
	for _, s := range []string{p.Summary, p.SummaryEN, p.Equipment, p.EquipmentEN, p.Capacity, p.CapacityEN} {
		if utf8.RuneCountInString(s) > 1000 {
			return fmt.Errorf("invalid_request")
		}
	}
	if len(p.Website) > 1000 || len(p.Contacts) > 20 {
		return fmt.Errorf("invalid_request")
	}
	if p.Website != "" {
		u, e := url.Parse(p.Website)
		if e != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
			return fmt.Errorf("invalid_request")
		}
	}
	for _, c := range p.Contacts {
		if utf8.RuneCountInString(c.Area) > 500 || utf8.RuneCountInString(c.AreaEN) > 500 || utf8.RuneCountInString(c.Name) > 200 || utf8.RuneCountInString(c.NameEN) > 200 || len(c.Phone) > 32 || len(c.Wechat) > 100 || (strings.TrimSpace(c.Phone) == "" && strings.TrimSpace(c.Wechat) == "") {
			return fmt.Errorf("invalid_request")
		}
	}
	return nil
}
func validatePublicBase(base string) error {
	if base == "" {
		return nil
	}
	u, e := url.Parse(base)
	if e != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return fmt.Errorf("PUBLIC_BASE_URL must be an http(s) origin without credentials, path, query or fragment")
	}
	return nil
}
func (a *App) resolveShowroom(slug string) ([]View, error) {
	q := a.publicQuery("vendor")
	if regexp.MustCompile(`^v-[1-9][0-9]*$`).MatchString(slug) {
		id, _ := strconv.ParseUint(slug[2:], 10, 64)
		q = q.Where("contents.id=?", id)
	} else {
		q = q.Where("(contents.showroom_slug=? OR contents.id IN (SELECT vendor_id FROM showroom_addresses WHERE slug=?))", slug, slug)
	}
	return a.publicViews(q)
}
func (a *App) publicShowroom(c *gin.Context) {
	items, e := a.resolveShowroom(c.Param("slug"))
	if e != nil {
		dbFail(c, e)
		return
	}
	if len(items) == 0 {
		fail(c, 404, "not_found")
		return
	}
	item := items[0]
	var total int64
	var ids []uint64
	q := a.publicQuery("product").Where("contents.vendor_id=?", item.ID)
	if e = q.Count(&total).Error; e != nil {
		dbFail(c, e)
		return
	}
	if e = a.publicQuery("product").Where("contents.vendor_id=?", item.ID).Distinct("COALESCE((SELECT rc.category_id FROM revision_categories rc WHERE rc.revision_id=r.id AND rc.role='product' LIMIT 1),r.category_id)").Pluck("COALESCE((SELECT rc.category_id FROM revision_categories rc WHERE rc.revision_id=r.id AND rc.role='product' LIMIT 1),r.category_id)", &ids).Error; e != nil {
		dbFail(c, e)
		return
	}
	// Include ancestors, so a parent category can filter all of its children.
	var all []Category
	if e = a.DB.Order("sort_order,id").Find(&all).Error; e != nil {
		dbFail(c, e)
		return
	}
	wanted := map[uint64]bool{}
	for _, id := range ids {
		id = canonicalCategory(a.DB, id)
		wanted[id] = true
	}
	for changed := true; changed; {
		changed = false
		for _, cat := range all {
			if wanted[cat.ID] && cat.ParentID > 0 && !wanted[cat.ParentID] {
				wanted[cat.ParentID] = true
				changed = true
			}
		}
	}
	type ShowroomCategory struct {
		Category
		ProductCount int64 `json:"product_count"`
	}
	var counts []struct {
		CategoryID uint64
		Total      int64
	}
	if e = a.publicQuery("product").Where("contents.vendor_id=?", item.ID).Select("COALESCE((SELECT rc.category_id FROM revision_categories rc WHERE rc.revision_id=r.id AND rc.role='product' LIMIT 1),r.category_id) AS category_id,COUNT(*) AS total").Group("COALESCE((SELECT rc.category_id FROM revision_categories rc WHERE rc.revision_id=r.id AND rc.role='product' LIMIT 1),r.category_id)").Scan(&counts).Error; e != nil {
		dbFail(c, e)
		return
	}
	byID := map[uint64]Category{}
	for _, cat := range all {
		byID[cat.ID] = cat
	}
	categoryCounts := map[uint64]int64{}
	for _, count := range counts {
		visited := map[uint64]bool{}
		for id := canonicalCategory(a.DB, count.CategoryID); id > 0 && !visited[id]; id = byID[id].ParentID {
			visited[id] = true
			categoryCounts[id] += count.Total
		}
	}
	cats := []ShowroomCategory{}
	for _, cat := range all {
		if wanted[cat.ID] && canonicalCategory(a.DB, cat.ID) == cat.ID {
			cats = append(cats, ShowroomCategory{Category: cat, ProductCount: categoryCounts[cat.ID]})
		}
	}
	canonical := "/v/" + item.ShowroomSlug
	canonical = a.publicBase(c) + canonical
	resourceCounts := map[string]int64{}
	for _, kind := range []string{"video", "catalog"} {
		var n int64
		if e = a.resourceQuery().Where("resources.vendor_id=? AND resources.kind=?", item.ID, kind).Count(&n).Error; e != nil {
			dbFail(c, e)
			return
		}
		resourceCounts[kind] = n
	}
	representatives, e := a.representativeProducts(item.ID)
	if e != nil {
		dbFail(c, e)
		return
	}
	homeProducts, e := a.showroomHomeProducts(item.ID, representatives)
	if e != nil {
		dbFail(c, e)
		return
	}
	c.Header("Cache-Control", "no-store")
	var businessCategory *struct {
		ID     uint64 `json:"id"`
		Name   string `json:"name"`
		NameEN string `json:"name_en"`
	}
	businessID := item.Data.PrimaryCategoryID
	if businessID == 0 {
		businessID = item.Data.CategoryID
	}
	if businessID > 0 {
		var category Category
		if a.DB.First(&category, canonicalCategory(a.DB, businessID)).Error == nil {
			businessCategory = &struct {
				ID     uint64 `json:"id"`
				Name   string `json:"name"`
				NameEN string `json:"name_en"`
			}{category.ID, category.Name, category.NameEN}
		}
	}
	c.JSON(200, gin.H{"business_category": businessCategory, "representative_products": representatives, "home_products": homeProducts, "item": item, "categories": cats, "product_count": total, "canonical_url": canonical, "resource_counts": resourceCounts})
}
func (a *App) setShowroomSlug(c *gin.Context) { a.saveShowroomAddress(c, true) }

// The SPA remains interactive; Go renders public content for every visitor.
func (a *App) showroomHTML(c *gin.Context, index string) bool {
	path := c.Request.URL.Path
	slug := ""
	if strings.HasPrefix(path, "/v/") {
		slug = strings.TrimPrefix(path, "/v/")
	} else if strings.HasPrefix(path, "/vendors/") {
		slug = "v-" + strings.TrimPrefix(path, "/vendors/")
	} else {
		return false
	}
	canonical := canonicalPublic(a.publicBase(c), path, nil)
	unavailable := func(status int) bool {
		return a.sendPublicHTML(c, index, status, "展厅暂不可用 | "+a.brandName(), "展厅不存在、暂未公开或已下架。", canonical, "noindex,follow")
	}
	items, err := a.resolveShowroom(slug)
	if err != nil {
		return unavailable(503)
	}
	if len(items) == 0 {
		return unavailable(404)
	}
	item := items[0]
	if a.redirectOldShowroom(c, item) {
		return true
	}
	canonical = canonicalPublic(a.publicBase(c), "/v/"+item.ShowroomSlug, nil)
	representatives, err := a.representativeProducts(item.ID)
	if err != nil {
		return unavailable(503)
	}
	products, err := a.showroomHomeProducts(item.ID, representatives)
	if err != nil {
		return unavailable(503)
	}
	detail, err := a.vendorSEO(c, item, products, canonical)
	if err != nil {
		return unavailable(500)
	}
	data := item.Data
	desc := data.MainProducts
	if data.Description != "" {
		desc = data.Description
	}
	if data.CompanyProfile != nil && data.CompanyProfile.Summary != "" {
		desc = data.CompanyProfile.Summary
	}
	runes := []rune(strings.TrimSpace(desc))
	if len(runes) > 160 {
		desc = string(runes[:160])
	}
	title := data.Name
	if strings.TrimSpace(data.MainProducts) != "" {
		title += " · " + data.MainProducts
	}
	title += " | " + a.brandName()
	robots := "index,follow"
	if showroomSearchVariant(c.Request.URL.Query()) {
		robots = "noindex,follow"
	}
	return a.sendPublicHTML(c, index, 200, title, desc, canonical, robots, detail)
}
