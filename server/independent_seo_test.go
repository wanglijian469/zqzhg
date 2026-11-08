package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func seoTestIndex(t *testing.T, a *App) {
	t.Helper()
	source := `<!doctype html><html><head><title>stale-title</title><meta name="description" content="stale"><meta name="keywords" content="stale-keywords"><meta name="robots" content="stale"><link rel="canonical" href="stale"><script type="application/ld+json">{"name":"stale-schema"}</script></head><body><div id="root"> </div></body></html>`
	if err := os.WriteFile(filepath.Join(a.Config.WebDir, "index.html"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
}

func seoSetPayload(t *testing.T, a *App, item Content, payload Payload) {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.DB.Model(&Revision{}).Where("id=?", item.PublishedRevisionID).Updates(map[string]any{"payload": string(raw), "name": payload.Name}).Error; err != nil {
		t.Fatal(err)
	}
}

func seoGet(router *gin.Engine, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	return w
}

func TestIndependentSEOHTMLAndPublication(t *testing.T) {
	a := importTestApp(t)
	a.Config.PublicBaseURL = "https://seo.example"
	seoTestIndex(t, a)
	v := v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, "SEO厂家 & 制造厂")
	slug := "seo-company"
	if err := a.DB.Model(&v).Update("showroom_slug", slug).Error; err != nil {
		t.Fatal(err)
	}
	seoSetPayload(t, a, v, Payload{Name: "SEO厂家 & 制造厂", MainProducts: "精密机械", Description: "企业生产介绍", RegionName: "河北省邢台市", Address: "测试工业园", Phone: "0319-1234567", Wechat: "seo-company", CompanyProfile: &CompanyProfile{Summary: "企业公开简介 <script>alert(1)</script>", CoverMediaID: 81}})
	p := v2Published(t, a, "product", v.ID, 1, 1, 1, false, 0, "SEO产品 <一号>")
	seoSetPayload(t, a, p, Payload{Name: "SEO产品 <一号>", Model: "MODEL-EXPLICIT", Parameters: "功率：80千瓦", Description: "产品独立正文 </script><script>alert(2)</script>", Brand: "真实品牌", MediaIDs: []uint64{82}})
	pending := Revision{ContentID: v.ID, Status: "pending", Name: "待审秘密", Payload: `{"name":"待审秘密"}`}
	a.DB.Create(&pending)
	a.DB.Model(&v).Update("draft_revision_id", pending.ID)
	router := a.router()
	for _, spec := range []struct{ path, kind, canonical string }{
		{"/v/seo-company?tab=about", "Organization", "https://seo.example/v/seo-company"},
		{fmt.Sprintf("/products/%d?hall=cable&utm_source=test", p.ID), "Product", fmt.Sprintf("https://seo.example/products/%d", p.ID)},
	} {
		w := seoGet(router, spec.path)
		raw := w.Body.String()
		if w.Code != 200 || !strings.Contains(raw, `<div id="root"><main`) || strings.Contains(raw, "待审秘密") || strings.Contains(raw, "stale-") || strings.Contains(raw, "<script>alert(") {
			t.Fatalf("unsafe or missing initial HTML: %d %s", w.Code, raw)
		}
		for _, pattern := range []string{`<title>`, `<meta name='description'`, `<meta name='keywords'`, `<meta name='robots'`, `<link rel='canonical'`, `application/ld\+json`} {
			if len(regexp.MustCompile(pattern).FindAllString(raw, -1)) != 1 {
				t.Fatalf("duplicate/missing metadata %s", pattern)
			}
		}
		if !strings.Contains(raw, "href='"+spec.canonical+"'") || !strings.Contains(raw, "property='og:image'") {
			t.Fatal("canonical/image missing", raw)
		}
		match := regexp.MustCompile(`<script type="application/ld\+json">(.*?)</script>`).FindStringSubmatch(raw)
		var structured map[string]json.RawMessage
		if len(match) != 2 || json.Unmarshal([]byte(match[1]), &structured) != nil {
			t.Fatal("invalid structured data", match)
		}
		entity := structured
		if spec.kind == "Product" {
			var graph []map[string]json.RawMessage
			if json.Unmarshal(structured["@graph"], &graph) != nil || len(graph) != 2 || string(graph[1]["@type"]) != `"BreadcrumbList"` ||
				!strings.Contains(raw, `aria-label="面包屑导航"`) {
				t.Fatal("product breadcrumb missing", match)
			}
			entity = graph[0]
		} else if strings.Contains(raw, `aria-label="面包屑导航"`) || strings.Contains(raw, `"BreadcrumbList"`) {
			t.Fatal("showroom should not contain breadcrumbs")
		}
		var kind, canonical string
		if json.Unmarshal(entity["@type"], &kind) != nil || json.Unmarshal(entity["url"], &canonical) != nil ||
			kind != spec.kind || canonical != spec.canonical {
			t.Fatal("invalid structured data", match)
		}
		if _, exists := entity["offers"]; exists {
			t.Fatal("invented offers")
		}
		body := strings.SplitN(raw, "</head>", 2)[1]
		if spec.kind == "Organization" {
			for _, text := range []string{"企业公开简介", "精密机械", "河北省邢台市", "测试工业园", `href="tel:0319-1234567"`, fmt.Sprintf(`href="/products/%d"`, p.ID)} {
				if !strings.Contains(body, text) {
					t.Fatal("company body missing", text, body)
				}
			}
		} else {
			for _, text := range []string{"产品独立正文", "MODEL-EXPLICIT", "功率：80千瓦", `href="/v/seo-company"`, "SEO产品 &lt;一号&gt;"} {
				if !strings.Contains(body, text) {
					t.Fatal("product body missing", text, body)
				}
			}
		}
	}
	for _, query := range []string{"?tab=products&page=2", "?q=设备", "?category_id=1", "?sort=latest"} {
		w := seoGet(router, "/v/seo-company"+query)
		if w.Code != 200 || w.Header().Get("X-Robots-Tag") != "noindex,follow" || !strings.Contains(w.Body.String(), "href='https://seo.example/v/seo-company'") {
			t.Fatal("variant policy", query, w.Code, w.Body.String())
		}
	}
	a.DB.Model(&v).Update("visible", false)
	for _, path := range []string{"/v/seo-company", fmt.Sprintf("/products/%d", p.ID), "/products/not-a-number", "/v/missing"} {
		w := seoGet(router, path)
		if w.Code != 404 || w.Header().Get("X-Robots-Tag") != "noindex,follow" || strings.Contains(w.Body.String(), "data-seo-content") || strings.Contains(w.Body.String(), "产品独立正文") {
			t.Fatal("unpublished body leaked", path, w.Code)
		}
	}
}

func TestIndependentSEOHistoryRedirects(t *testing.T) {
	a := importTestApp(t)
	seoTestIndex(t, a)
	v := v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, "历史地址测试")
	a.DB.Model(&v).Update("showroom_slug", "current-brand")
	a.DB.Create(&ShowroomAddress{Slug: "old-brand", VendorID: v.ID})
	router := a.router()
	for _, path := range []string{fmt.Sprintf("/vendors/%d", v.ID), fmt.Sprintf("/v/v-%d", v.ID), "/v/old-brand"} {
		w := seoGet(router, path+"?tab=products&page=2&q=MODEL")
		if w.Code != 301 || w.Header().Get("Location") != "/v/current-brand?tab=products&page=2&q=MODEL" {
			t.Fatal("old URL not permanently redirected", path, w.Code, w.Header())
		}
	}
	if w := seoGet(router, "/v/current-brand"); w.Code != 200 {
		t.Fatal("redirect loop", w.Code)
	}
}

func TestIndependentSEOSitemapVisibilityAndBoundaries(t *testing.T) {
	a := importTestApp(t)
	seoTestIndex(t, a)
	a.Config.PublicBaseURL = "https://seo.example"
	v := v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, "地图公开厂商")
	a.DB.Model(&v).Update("showroom_slug", "map-brand")
	p := v2Published(t, a, "product", v.ID, 1, 1, 1, false, 0, "地图公开产品")
	// Test both sides of the 1,000-entry boundary without thousands of API calls.
	items := make([]Content, sitemapPageSize)
	for i := range items {
		items[i] = Content{Kind: "product", VendorID: v.ID, HallID: 1, Visible: true, PublishedRevisionID: p.PublishedRevisionID}
	}
	if err := a.DB.CreateInBatches(&items, 100).Error; err != nil {
		t.Fatal(err)
	}
	private := v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, "下架厂商")
	a.DB.Model(&private).Update("visible", false)
	v2Published(t, a, "product", private.ID, 1, 1, 1, false, 0, "下架厂商的产品")
	v2Published(t, a, "vendor", 0, 2, 1, 1, false, 0, "筹建厂商")
	for _, state := range []string{"draft", "pending"} {
		item := v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, state)
		a.DB.Model(&Revision{}).Where("id=?", item.PublishedRevisionID).Update("status", state)
	}
	router := a.router()
	w := seoGet(router, "/sitemap.xml")
	var root sitemapURLs
	if w.Code != 200 || xml.Unmarshal(w.Body.Bytes(), &root) != nil || root.XMLNS != sitemapNamespace ||
		!strings.Contains(w.Body.String(), "https://seo.example/v/map-brand</loc><lastmod>") {
		t.Fatal("invalid root sitemap", w.Code, w.Body.String())
	}
	for page, expected := range map[int]int{1: 1000, 2: 1} {
		w = seoGet(router, fmt.Sprintf("/sitemaps/products-%d.xml", page))
		var urls sitemapURLs
		if w.Code != 200 || xml.Unmarshal(w.Body.Bytes(), &urls) != nil || len(urls.Entries) != expected || urls.XMLNS != sitemapNamespace {
			t.Fatal("invalid product shard", page, w.Code, w.Body.String())
		}
	}
	w = seoGet(router, "/sitemap-vendors-1.xml")
	var vendors sitemapURLs
	if w.Code != 200 || xml.Unmarshal(w.Body.Bytes(), &vendors) != nil || len(vendors.Entries) != 1 || vendors.Entries[0].Loc != "https://seo.example/v/map-brand" {
		t.Fatal("private vendor exposed", w.Body.String())
	}
	for _, path := range []string{"/sitemaps/products-3.xml", "/sitemaps/vendors-0.xml", "/sitemaps/unknown-1.xml", "/sitemaps/vendors-01.xml"} {
		if w = seoGet(router, path); w.Code != 404 {
			t.Fatal("invalid shard accepted", path, w.Code)
		}
	}
	robots := seoGet(router, "/robots.txt").Body.String()
	for _, text := range []string{"Sitemap: https://seo.example/sitemap.xml", "Allow: /api/v1/media/", "Allow: /api/v1/public/", "Disallow: /api/", "User-agent: gptbot"} {
		if !strings.Contains(robots, text) {
			t.Fatal("robots policy missing", text)
		}
	}
	a.DB.Model(&v).Update("visible", false)
	if w = seoGet(router, "/sitemaps/products-1.xml"); w.Code != 404 {
		t.Fatal("vendor offline did not remove products", w.Code)
	}
	// Canonical origin precedence uses the existing saved setting.
	cfg, _ := a.systemConfig()
	cfg.Site.WebURL = "https://configured.example"
	raw, _ := json.Marshal(cfg)
	if err := a.DB.Create(&SystemSetting{SettingKey: "system_config_v3", Value: string(raw)}).Error; err != nil {
		t.Fatal(err)
	}
	if robots = seoGet(router, "/robots.txt").Body.String(); !strings.Contains(robots, "https://configured.example/sitemap.xml") {
		t.Fatal("saved origin ignored", robots)
	}
}

func TestIndependentSEODirectoryDiscoveryAndLastMod(t *testing.T) {
	a := importTestApp(t)
	seoTestIndex(t, a)
	a.Config.PublicBaseURL = "https://zqzhg.com"
	var category Category
	if err := a.DB.Where("hall_id=1 AND category_kind='vendor' AND enabled=1 AND parent_id=0").
		First(&category).Error; err != nil {
		t.Fatal(err)
	}
	const company = "河北丹顿液压科技有限公司"
	vendor := Content{ID: 21, Kind: "vendor", HallID: 1, Visible: true, Featured: true}
	if err := a.DB.Create(&vendor).Error; err != nil {
		t.Fatal(err)
	}
	approved := Revision{ContentID: vendor.ID, Status: "approved", CategoryID: category.ID, Payload: "{}"}
	if err := a.DB.Create(&approved).Error; err != nil {
		t.Fatal(err)
	}
	vendor.PublishedRevisionID = approved.ID
	if err := a.DB.Save(&vendor).Error; err != nil {
		t.Fatal(err)
	}
	if err := a.DB.Create(&HomeRotationPool{VendorID: vendor.ID, HallID: 1, State: "approved"}).Error; err != nil {
		t.Fatal(err)
	}
	seoSetPayload(t, a, vendor, Payload{Name: company, MainProducts: "液压油缸", Description: "主营液压技术研发、技术咨询与推广及液压油缸、液压阀等配件生产销售"})
	if err := a.DB.Model(&approved).
		Updates(map[string]any{"province_code": "13", "city_code": "1305", "district_code": "130528"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := a.DB.Exec("INSERT IGNORE INTO revision_categories(revision_id, category_id, role) VALUES(?, ?, 'primary')", vendor.PublishedRevisionID, category.ID).Error; err != nil {
		t.Fatal(err)
	}
	router := a.router()
	cat := fmt.Sprintf("/browse?category_id=%d", category.ID)
	public := []string{"/", "/browse", cat, "/regions", "/regions?region_code=13",
		"/regions?region_code=1305", "/regions?region_code=130528"}
	for _, path := range public {
		w := seoGet(router, path)
		if w.Code != 200 || w.Header().Get("X-Robots-Tag") != "" ||
			!strings.Contains(w.Body.String(), `href="/v/v-21"`) {
			t.Fatalf("directory discovery %s: %d %s", path, w.Code, w.Body.String())
		}
	}
	for _, path := range []string{cat + "&q=无", cat + "&page=2", "/regions?region_code=000000",
		"/browse?category_id=999999999", "/account", "/admin"} {
		w := seoGet(router, path)
		if !strings.Contains(w.Header().Get("X-Robots-Tag"), "noindex") {
			t.Fatalf("indexable private or filtered page: %s", path)
		}
	}
	w := seoGet(router, "/v/v-21")
	for _, expected := range []string{company + " - 企业展厅 - 中企展馆", "主营液压技术研发", "液压油缸",
		"href='https://zqzhg.com/v/v-21'", "Organization"} {
		if w.Code != 200 || !strings.Contains(w.Body.String(), expected) {
			t.Fatalf("showroom metadata missing %q (%d)", expected, w.Code)
		}
	}
	sitemap := func() map[string]string {
		t.Helper()
		w := seoGet(router, "/sitemap.xml")
		var root sitemapURLs
		if err := xml.Unmarshal(w.Body.Bytes(), &root); w.Code != 200 || err != nil {
			t.Fatalf("invalid sitemap: %d %v %s", w.Code, err, w.Body.String())
		}
		result := map[string]string{}
		for _, entry := range root.Entries {
			result[strings.TrimPrefix(entry.Loc, "https://zqzhg.com")] = entry.LastMod
		}
		return result
	}
	initial := sitemap()
	for _, path := range public {
		if _, found := initial[path]; !found {
			t.Fatal("missing public directory in sitemap", path)
		}
	}
	if _, err := time.Parse(time.RFC3339, initial["/v/v-21"]); err != nil {
		t.Fatal("invalid showroom lastmod", initial["/v/v-21"], err)
	}
	draft := Revision{ContentID: vendor.ID, Status: "pending", Name: "待审名称", Payload: "{}"}
	if err := a.DB.Create(&draft).Error; err != nil {
		t.Fatal(err)
	}
	if err := a.DB.Model(&vendor).Update("draft_revision_id", draft.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after := sitemap()["/v/v-21"]; after != initial["/v/v-21"] {
		t.Fatal("draft changed the public date", after)
	}
	if err := a.DB.Model(&vendor).Update("visible", false).Error; err != nil {
		t.Fatal(err)
	}
	if _, found := sitemap()["/v/v-21"]; found || seoGet(router, "/v/v-21").Code != 404 {
		t.Fatal("offline showroom remained public")
	}
	nextDate := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
	if err := a.DB.Model(&Revision{}).Where("id=?", vendor.PublishedRevisionID).
		Update("reviewed_at", nextDate).Error; err != nil {
		t.Fatal(err)
	}
	if err := a.DB.Model(&vendor).Update("visible", true).Error; err != nil {
		t.Fatal(err)
	}
	if after := sitemap()["/v/v-21"]; after != nextDate.Format(time.RFC3339) {
		t.Fatal("republished date missing", after)
	}
}

func TestIndependentSEOUnavailable(t *testing.T) {
	a := importTestApp(t)
	seoTestIndex(t, a)
	v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, "故障前公开厂家")
	router := a.router()
	err := a.DB.Callback().Query().Before("gorm:query").Register("seo_database_unavailable", func(tx *gorm.DB) { tx.AddError(errors.New("simulated database failure")) })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.DB.Callback().Query().Remove("seo_database_unavailable") })
	for _, path := range []string{"/v/v-1", "/products/1", "/sitemap.xml", "/sitemaps/vendors-1.xml"} {
		w := seoGet(router, path)
		if w.Code != 503 || !strings.Contains(w.Header().Get("X-Robots-Tag"), "noindex") || strings.Contains(w.Body.String(), "故障前公开厂家") {
			t.Fatal("database failure returned indexable content", path, w.Code, w.Body.String())
		}
	}
}

func TestIndependentSEOCanonicalAndSafeTemplate(t *testing.T) {
	query, _ := url.ParseQuery("tab=products&page=2&q=test&lang=en")
	if canonicalPublic("https://seo.example", "/v/brand", query) != "https://seo.example/v/brand" || !showroomSearchVariant(query) {
		t.Fatal("showroom variant not consolidated")
	}
	result, err := renderSEO(seoPage{Company: true, Data: Payload{Name: `<img src=x onerror=alert(1)>`, CompanyProfile: &CompanyProfile{Website: "javascript:alert(1)"}}}, nil)
	if err != nil || strings.Contains(result.Body, `<img src=x`) || strings.Contains(result.Body, `href="javascript:`) {
		t.Fatal("unsafe public HTML", result.Body, err)
	}
	for _, spec := range []struct {
		payload Payload
		want    string
	}{
		{Payload{Model: "explicit", Parameters: "型号：legacy"}, "explicit"},
		{Payload{Parameters: "功率：80\n型号：legacy"}, "legacy"},
		{Payload{}, ""},
	} {
		if seoProductModel(spec.payload) != spec.want {
			t.Fatal("wrong product model")
		}
	}
}
