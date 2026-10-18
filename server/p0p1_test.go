package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"image"
	"image/png"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// All fixtures use importTestApp's disposable database and media directory.
func v2Published(t *testing.T, a *App, kind string, vendor, hall, cat, region uint64, featured bool, order int, name string) Content {
	t.Helper()
	c := Content{Kind: kind, VendorID: vendor, HallID: hall, Visible: true, Featured: featured, SortOrder: order, CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	if e := a.DB.Create(&c).Error; e != nil {
		t.Fatal(e)
	}
	p, _ := json.Marshal(Payload{Name: name, CategoryID: cat, RegionID: region, MainProducts: "真实设备", Parameters: "型号：V2-UNION", Phone: "0319-1234567"})
	now := time.Now().UTC()
	r := Revision{ContentID: c.ID, Status: "approved", Name: name, CategoryID: cat, RegionID: region, Payload: string(p), ReviewedAt: &now}
	if e := a.DB.Create(&r).Error; e != nil {
		t.Fatal(e)
	}
	c.PublishedRevisionID = r.ID
	if e := a.DB.Save(&c).Error; e != nil {
		t.Fatal(e)
	}
	return c
}
func TestV2HallSettingsPermissionsConcurrencyAndMedia(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	admin := resourceTestSession(t, a, "admin")
	merchant := resourceTestSession(t, a, "merchant")
	tc.call("GET", "/admin/halls", "", nil, 401)
	tc.call("GET", "/admin/halls", merchant, nil, 403)
	for _, grant := range []string{"home.manage", "vendor.review"} {
		op := tc.call("POST", "/admin/operators", admin, gin.H{"username": "hall-" + randomToken()[:10], "enabled": true, "permissions": []string{grant}}, 201)
		login := tc.call("POST", "/auth/admin", "", gin.H{"username": op["username"], "password": op["initial_password"]}, 200)
		want := 403
		if grant == "home.manage" {
			want = 200
		}
		tc.call("GET", "/admin/halls", login["token"].(string), nil, want)
	}
	var photo bytes.Buffer
	png.Encode(&photo, image.NewRGBA(image.Rect(0, 0, 12, 8)))
	hero := resourceUpload(t, a, admin, "/admin/home-display/media", "cable.png", photo.Bytes(), 201)
	private := resourceUpload(t, a, merchant, "/merchant/media", "private.png", photo.Bytes(), 201)
	req := gin.H{"name": "电线电缆厅新名称", "name_en": "Wire & Cable Test Hall", "description": "仅筹建资料，无虚构企业", "description_en": "Under preparation.", "status": "preparing", "slug": "cable", "hero_media_id": hero["id"], "version": 1}
	tc.call("PUT", "/admin/halls/2", merchant, req, 403)
	invalid := gin.H{}
	for k, v := range req {
		invalid[k] = v
	}
	invalid["slug"] = "renamed"
	tc.call("PUT", "/admin/halls/2", admin, invalid, 400)
	invalid["slug"] = "cable"
	invalid["hero_media_id"] = private["id"]
	tc.call("PUT", "/admin/halls/2", admin, invalid, 400)
	saved := tc.call("PUT", "/admin/halls/2", admin, req, 200)
	if saved["version"] != float64(2) || saved["slug"] != "cable" {
		t.Fatal(saved)
	}
	homeMediaRead(t, a, "", "/media/"+number(hero["id"]), 200)
	tc.call("GET", "/media/"+number(private["id"]), "", nil, 404)
	halls := tc.call("GET", "/public/halls", "", nil, 200)["items"].([]any)
	if halls[1].(map[string]any)["hero_media_id"] != hero["id"] {
		t.Fatal(halls)
	}
	tc.call("PUT", "/admin/home-display?hall=cable", admin, gin.H{"hero_media_id": 0, "version": 1}, 409)
	req["version"] = 2
	req["status"] = "open"
	start := make(chan struct{})
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			codes <- homePutRequest(tc.router.(*gin.Engine), "/admin/halls/2", admin, req).Code
		}()
	}
	close(start)
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for code := range codes {
		counts[code]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatal(counts)
	}
	v := v2Published(t, a, "vendor", 0, 2, 0, 0, true, 0, "电缆厂家")
	p := v2Published(t, a, "product", v.ID, 2, 0, 0, true, 0, "电缆设备")
	tc.call("GET", fmt.Sprintf("/public/products/%d", p.ID), "", nil, 200)
	if tc.call("GET", "/public/products", "", nil, 200)["total"] != float64(0) {
		t.Fatal("cross-hall data")
	}
	req["version"] = 3
	req["status"] = "preparing"
	tc.call("PUT", "/admin/halls/2", admin, req, 200)
	tc.call("GET", fmt.Sprintf("/public/products/%d", p.ID), "", nil, 404)
	if tc.call("GET", "/public/products?hall=cable", "", nil, 200)["total"] != float64(0) {
		t.Fatal("preparing content leaked")
	}
	homeMediaRead(t, a, "", "/media/"+number(hero["id"]), 200)
	var logs int64
	a.DB.Model(&OperationLog{}).Where("action='hall_settings_saved'").Count(&logs)
	if logs != 3 {
		t.Fatal("audit", logs)
	}
	a.DB.Exec("DELETE FROM schema_migrations WHERE version=12")
	if e := migrate(a.DB); e != nil {
		t.Fatal("index retry", e)
	}
	if e := migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	var count int64
	a.DB.Model(&Content{}).Count(&count)
	if count != 2 {
		t.Fatal("migration lost data")
	}
}
func TestV2RecommendationsStrictCapsDiversityAndVisibility(t *testing.T) {
	a := importTestApp(t)
	v := v2Published(t, a, "vendor", 0, 1, 1, 1, true, -10, "推荐厂商")
	for i := 0; i < 10; i++ {
		v2Published(t, a, "product", v.ID, 1, uint64(i%2+1), 1, true, 0, fmt.Sprintf("设备%d", i))
	}
	products, e := a.homeProducts(1)
	if e != nil || len(products) != 3 {
		t.Fatal("strict cap with insufficient suppliers", len(products), e)
	}
	for i := 0; i < 4; i++ {
		other := v2Published(t, a, "vendor", 0, 1, uint64(i%2+1), uint64(i%2+1), i < 2, 0, fmt.Sprintf("厂商%d", i))
		for j := 0; j < 6; j++ {
			v2Published(t, a, "product", other.ID, 1, uint64(j%2+1), 1, i < 2, 0, fmt.Sprintf("产品%d-%d", i, j))
		}
	}
	products, e = a.homeProducts(1)
	if e != nil || len(products) != 12 {
		t.Fatal(len(products), e)
	}
	stable, e := a.homeProducts(1)
	if e != nil {
		t.Fatal(e)
	}
	byVendor := map[uint64]int{}
	cats := map[uint64]bool{}
	for i, p := range products {
		byVendor[p.VendorID]++
		cats[p.Data.CategoryID] = true
		if byVendor[p.VendorID] > 3 {
			t.Fatal("supplier cap")
		}
		if stable[i].ID != p.ID {
			t.Fatal("unstable selection")
		}
		if i > 0 && products[i-1].VendorID == p.VendorID {
			t.Fatal("adjacent same supplier")
		}
	}
	if len(cats) < 2 || len(byVendor) < 4 {
		t.Fatal("diversity", cats, byVendor)
	}
	vendors, e := a.homeVendors(1)
	if e != nil || vendors[0].ID != v.ID {
		t.Fatal("manual priority", e)
	}
	a.DB.Model(&Content{}).Where("id=?", v.ID).Update("visible", false)
	products, e = a.homeProducts(1)
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range products {
		if p.VendorID == v.ID {
			t.Fatal("hidden supplier leaked")
		}
	}
	victim := products[0]
	a.DB.Model(&Content{}).Where("id=?", victim.ID).Update("visible", false)
	a.DB.Model(&Content{}).Where("kind='product'").Updates(map[string]any{"featured": false, "sort_order": 0})
	promoted := products[len(products)-1]
	a.DB.Model(&Content{}).Where("id=?", promoted.ID).Update("featured", true)
	products, e = a.homeProducts(1)
	if e != nil || products[0].ID != promoted.ID {
		t.Fatal("recommendation update", e)
	}
	for _, p := range products {
		if p.ID == victim.ID {
			t.Fatal("offline leaked")
		}
	}
	a.DB.Model(&Content{}).Where("id=?", promoted.ID).Update("featured", false)
	newer := v2Published(t, a, "product", promoted.VendorID, 1, 1, 1, true, -5, "新精选产品")
	products, _ = a.homeProducts(1)
	if products[0].ID != newer.ID {
		t.Fatal("revoked featured priority retained")
	}
}
func TestV2CombinedFiltersAndBoundedRepresentatives(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	region := Region{Name: "筛选地区", Enabled: true}
	a.DB.Create(&region)
	child := Category{HallID: 1, ParentID: 1, Name: "筛选子分类", Enabled: true}
	a.DB.Create(&child)
	for i := 0; i < 27; i++ {
		v := v2Published(t, a, "vendor", 0, 1, 1, region.ID, true, 0, fmt.Sprintf("联合厂商%02d", i))
		for j := 0; j < 5; j++ {
			v2Published(t, a, "product", v.ID, 1, child.ID, region.ID, true, 0, fmt.Sprintf("联合产品%02d-%d", i, j))
		}
	}
	other := v2Published(t, a, "vendor", 0, 1, 1, 9999, true, 0, "异地厂家")
	v2Published(t, a, "product", other.ID, 1, child.ID, 9999, true, 0, "联合异地产品")
	filter := fmt.Sprintf("?q=V2-UNION&category_id=1&region_id=%d", region.ID)
	for _, spec := range []struct {
		kind        string
		total, size int
	}{{"vendors", 27, 20}, {"products", 135, 24}} {
		one := tc.call("GET", "/public/"+spec.kind+filter, "", nil, 200)
		two := tc.call("GET", "/public/"+spec.kind+filter+"&page=2", "", nil, 200)
		if one["total"] != float64(spec.total) || len(one["items"].([]any)) != spec.size {
			t.Fatal("AND total/page", spec.kind, one["total"])
		}
		seen := map[float64]bool{}
		for _, list := range []map[string]any{one, two} {
			for _, raw := range list["items"].([]any) {
				p := raw.(map[string]any)
				id := p["id"].(float64)
				if seen[id] {
					t.Fatal("overlapping pages")
				}
				seen[id] = true
				if spec.kind == "vendors" && len(p["matching_products"].([]any)) != 3 {
					t.Fatal("representatives bound")
				}
			}
		}
		for _, bad := range []string{"&region_id=invalid", "&category_id=999999", "&hall=cable"} {
			if tc.call("GET", "/public/"+spec.kind+"?q=V2-UNION"+bad, "", nil, 200)["total"] != float64(0) {
				t.Fatal("invalid filter", bad)
			}
		}
	}
	for _, spec := range []struct {
		kind string
		size int
	}{{"vendors", 20}, {"products", 24}} {
		r := tc.call("GET", "/public/"+spec.kind+"?page_size=100", "", nil, 200)
		if r["page_size"] != float64(spec.size) || len(r["items"].([]any)) != spec.size {
			t.Fatal("public page size cap", spec.kind)
		}
	}
	// Query count must not grow per result row. GORM callbacks include the UNION raw query.
	queries := 0
	name := "v2_query_count"
	a.DB.Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) {
		if !tx.DryRun {
			queries++
		}
	})
	a.DB.Callback().Row().Before("gorm:row").Register(name, func(*gorm.DB) { queries++ })
	defer a.DB.Callback().Query().Remove(name)
	defer a.DB.Callback().Row().Remove(name)
	tc.call("GET", "/public/vendors?page_size=1", "", nil, 200)
	oneCount := queries
	queries = 0
	tc.call("GET", "/public/vendors?page_size=20", "", nil, 200)
	twentyCount := queries
	if twentyCount > oneCount+2 {
		t.Fatal("N+1 queries", oneCount, twentyCount)
	}
	t.Logf("representative query count: 1 vendor=%d; 20 vendors=%d", oneCount, twentyCount)
}
func TestV2InitialHTMLSEOAndPrivacy(t *testing.T) {
	a := importTestApp(t)
	a.Config.PublicBaseURL = "https://zqzhg.com"
	source := "<!doctype html><html><head><title>旧标题</title><meta name='description' content='old'><meta name='robots' content='index'><link rel='canonical' href='old'></head><body><div id='root'></div></body></html>"
	if e := os.WriteFile(filepath.Join(a.Config.WebDir, "index.html"), []byte(source), 0600); e != nil {
		t.Fatal(e)
	}
	v := v2Published(t, a, "vendor", 0, 1, 1, 1, true, 0, "真实厂家 & 工厂")
	p := v2Published(t, a, "product", v.ID, 1, 1, 1, true, 0, "独立设备 <一号>")
	router := a.router()
	for _, spec := range []struct {
		path, title, canonical, robots string
		status                         int
	}{
		{"/", "中企展馆 · 农业机械厅", "https://zqzhg.com/", "index,follow", 200},
		{"/browse?hall=agriculture&page=1", "找厂家", "https://zqzhg.com/browse", "index,follow", 200},
		{"/browse/products?page=2", "找产品", "https://zqzhg.com/browse/products?page=2", "index,follow", 200},
		{"/browse?q=MODEL&category_id=1&page=2", "找厂家", "https://zqzhg.com/browse?category_id=1&amp;page=2&amp;q=MODEL", "noindex,follow", 200},
		{"/?hall=cable", "电线电缆厅", "https://zqzhg.com/?hall=cable", "noindex,follow", 200},
		{fmt.Sprintf("/products/%d?hall=cable", p.ID), "独立设备 &lt;一号&gt;", fmt.Sprintf("https://zqzhg.com/products/%d", p.ID), "index,follow", 200},
		{"/products/999999", "产品暂未公开", "https://zqzhg.com/products/999999", "noindex,follow", 404},
		{"/?hall=unknown", "展馆暂不可用", "https://zqzhg.com/?hall=unknown", "noindex,follow", 404},
	} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", spec.path, nil))
		body := w.Body.String()
		if strings.HasPrefix(spec.path, "/products/") && spec.status == 200 && !strings.Contains(body, "V2-UNION") {
			t.Fatal("model absent from initial SEO", body)
		}
		if w.Code != spec.status || !strings.Contains(body, spec.title) || !strings.Contains(body, spec.canonical) || !strings.Contains(body, spec.robots) || strings.Count(body, "<title>") != 1 || strings.Contains(body, "旧标题") {
			t.Fatalf("SEO %s: %d %s", spec.path, w.Code, body)
		}
	}
	a.DB.Model(&Content{}).Where("id=?", v.ID).Update("visible", false)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/products/%d", p.ID), nil))
	if w.Code != 404 || strings.Contains(w.Body.String(), "独立设备") {
		t.Fatal("private HTML leaked")
	}
}

func TestV2VendorCandidatesPreserveRareCategoriesAndRegions(t *testing.T) {
	a := importTestApp(t)
	rare := v2Published(t, a, "vendor", 0, 1, 2, 777, false, 0, "较早的少数分类厂家")
	for i := 0; i < 45; i++ {
		v2Published(t, a, "vendor", 0, 1, 1, uint64(i+1), false, 0, fmt.Sprintf("主流分类地区厂家%d", i))
	}
	items, e := a.homeVendors(1)
	if e != nil || len(items) != 8 {
		t.Fatal(len(items), e)
	}
	found := false
	cats, regions := map[uint64]bool{}, map[uint64]bool{}
	for _, v := range items {
		found = found || v.ID == rare.ID
		cats[v.Data.CategoryID] = true
		regions[v.Data.RegionID] = true
	}
	if !found || len(cats) != 2 || len(regions) != 8 {
		t.Fatal("rare category or region diversity lost", found, cats, regions)
	}
	again, e := a.homeVendors(1)
	if e != nil {
		t.Fatal(e)
	}
	for i, v := range items {
		if again[i].ID != v.ID {
			t.Fatal("unstable manufacturer order")
		}
	}
}
