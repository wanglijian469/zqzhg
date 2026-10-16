package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"html"
	"net/url"
	"os"
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
	Credentials        []CompanyCredential `json:"credentials"`
	GalleryAnnotations []GalleryAnnotation `json:"gallery_annotations"`
	ShortName          string              `json:"short_name"`
	ShortNameEN        string              `json:"short_name_en"`
	Summary            string              `json:"summary"`
	SummaryEN          string              `json:"summary_en"`
	LogoMediaID        uint64              `json:"logo_media_id"`
	CoverMediaID       uint64              `json:"cover_media_id"`
	Established        string              `json:"established"`
	Staff              string              `json:"staff"`
	FactoryArea        string              `json:"factory_area"`
	Equipment          string              `json:"equipment"`
	EquipmentEN        string              `json:"equipment_en"`
	Capacity           string              `json:"capacity"`
	CapacityEN         string              `json:"capacity_en"`
	Website            string              `json:"website"`
	ContactName        string              `json:"contact_name"`
	ContactNameEN      string              `json:"contact_name_en"`
	Contacts           []CompanyContact    `json:"contacts"`
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
		q = q.Where("contents.showroom_slug=?", slug)
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
	if e = a.publicQuery("product").Where("contents.vendor_id=?", item.ID).Distinct("r.category_id").Pluck("r.category_id", &ids).Error; e != nil {
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
	if e = a.publicQuery("product").Where("contents.vendor_id=?", item.ID).Select("r.category_id AS category_id,COUNT(*) AS total").Group("r.category_id").Scan(&counts).Error; e != nil {
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
	if a.Config.PublicBaseURL != "" {
		canonical = strings.TrimRight(a.Config.PublicBaseURL, "/") + canonical
	}
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
	c.Header("Cache-Control", "no-store")
	var businessCategory *struct {
		ID     uint64 `json:"id"`
		Name   string `json:"name"`
		NameEN string `json:"name_en"`
	}
	if item.Data.CategoryID > 0 {
		var category Category
		if a.DB.First(&category, canonicalCategory(a.DB, item.Data.CategoryID)).Error == nil {
			businessCategory = &struct {
				ID     uint64 `json:"id"`
				Name   string `json:"name"`
				NameEN string `json:"name_en"`
			}{category.ID, category.Name, category.NameEN}
		}
	}
	c.JSON(200, gin.H{"business_category": businessCategory, "representative_products": representatives, "item": item, "categories": cats, "product_count": total, "canonical_url": canonical, "resource_counts": resourceCounts})
}
func (a *App) setShowroomSlug(c *gin.Context) {
	var req struct {
		Slug string `json:"slug"`
	}
	if c.ShouldBindJSON(&req) != nil || !regexp.MustCompile(`^[a-z][a-z0-9-]{1,63}$`).MatchString(req.Slug) || regexp.MustCompile(`^v-[0-9]+$`).MatchString(req.Slug) {
		fail(c, 400, "invalid_showroom_slug")
		return
	}
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		var content Content
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("kind='vendor'").First(&content, id(c)).Error; e != nil {
			return e
		}
		if content.ShowroomSlug != nil {
			if *content.ShowroomSlug == req.Slug {
				return nil
			}
			return fmt.Errorf("showroom_slug_locked")
		}
		var n int64
		if e := tx.Model(&Content{}).Where("showroom_slug=?", req.Slug).Count(&n).Error; e != nil {
			return e
		}
		if n > 0 {
			return fmt.Errorf("showroom_slug_exists")
		}
		if e := tx.Model(&content).Update("showroom_slug", req.Slug).Error; e != nil {
			if strings.Contains(e.Error(), "Duplicate entry") {
				return fmt.Errorf("showroom_slug_exists")
			}
			return e
		}
		return recordOperation(tx, actor(c).ID, "vendor", content.ID, "showroom_slug_set", req.Slug)
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"showroom_slug": req.Slug})
}

// The SPA remains interactive; metadata is rendered by Go for link crawlers.
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
	source, e := os.ReadFile(index)
	if e != nil {
		return false
	}
	items, e := a.resolveShowroom(slug)
	if e != nil {
		c.Header("Cache-Control", "no-store")
		c.Data(503, "text/html; charset=utf-8", []byte(strings.Replace(string(source), "<head>", `<head><meta name="robots" content="noindex">`, 1)))
		return true
	}
	c.Header("Cache-Control", "no-store")
	if len(items) == 0 {
		c.Header("X-Robots-Tag", "noindex")
		c.Data(404, "text/html; charset=utf-8", []byte(strings.Replace(string(source), "<head>", `<head><meta name="robots" content="noindex">`, 1)))
		return true
	}
	item := items[0]
	data := item.Data
	desc := data.MainProducts
	cover := uint64(0)
	if data.CompanyProfile != nil {
		if data.CompanyProfile.Summary != "" {
			desc = data.CompanyProfile.Summary
		}
		cover = data.CompanyProfile.CoverMediaID
	}
	if cover == 0 && len(data.MediaIDs) > 0 {
		cover = data.MediaIDs[0]
	}
	base := strings.TrimRight(a.Config.PublicBaseURL, "/")
	if base == "" {
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		base = scheme + "://" + c.Request.Host
	}
	canonical := base + "/v/" + item.ShowroomSlug
	title := data.Name + " · " + data.MainProducts + " | 中企展馆"
	meta := `<title>` + html.EscapeString(title) + `</title><meta name="description" content="` + html.EscapeString(desc) + `"><link rel="canonical" href="` + html.EscapeString(canonical) + `">`
	for k, v := range map[string]string{"og:type": "website", "og:title": title, "og:description": desc, "og:url": canonical} {
		meta += `<meta property="` + k + `" content="` + html.EscapeString(v) + `">`
	}
	if cover > 0 {
		meta += `<meta property="og:image" content="` + html.EscapeString(fmt.Sprintf("%s/api/v1/media/%d", base, cover)) + `">`
	}
	org, _ := json.Marshal(map[string]string{"@context": "https://schema.org", "@type": "Organization", "name": data.Name, "url": canonical, "telephone": data.Phone, "address": data.Address})
	meta += `<script type="application/ld+json">` + string(org) + `</script>`
	page := regexp.MustCompile(`(?s)<title>.*?</title>`).ReplaceAllStringFunc(string(source), func(string) string { return "" })
	page = strings.Replace(page, "<head>", "<head>"+meta, 1)
	c.Data(200, "text/html; charset=utf-8", []byte(page))
	return true
}
