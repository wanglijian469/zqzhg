package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPublicProtectionBudgets(t *testing.T) {
	if guardKind("/api/v1/favorites") != "list" {
		t.Fatal("favorite data bypass")
	}
	cfg := guardConfig{List: 2, Detail: 3, Total: 4, Media: 20, Vendors: 2, Products: 3, MaxIPs: 3, Enabled: true}
	g := newPublicGuard(cfg)
	now := time.Now()
	g.now = func() time.Time { return now }
	if g.check("ip", "list", nil) != 0 || g.check("ip", "detail", nil) != 0 || g.check("ip", "list", nil) != 0 {
		t.Fatal("normal browsing denied")
	}
	if g.check("ip", "list", nil) == 0 {
		t.Fatal("list bypass")
	}
	for i := 0; i < 4; i++ {
		g.check("ip", "list", nil)
	}
	if g.check("ip", "detail", nil) < 590 {
		t.Fatal("temporary ban missing")
	}
	now = now.Add(11 * time.Minute)
	if g.check("ip", "list", nil) != 0 {
		t.Fatal("ban did not expire")
	}
	for i := 0; i < 8; i++ {
		if g.check("media", "media", nil) != 0 {
			t.Fatal("media shares business quota")
		}
	}
	if g.check("refs", "", map[string]string{"vendor:1": "vendor", "vendor:2": "vendor"}) != 0 || g.check("refs", "", map[string]string{"vendor:1": "vendor"}) != 0 {
		t.Fatal("duplicate refs count twice")
	}
	if g.check("refs", "", map[string]string{"vendor:3": "vendor"}) == 0 {
		t.Fatal("distinct quota bypass")
	}
	now = now.Add(2 * time.Hour)
	if g.check("new", "list", nil) != 0 || len(g.clients) != 1 {
		t.Fatal("expired IPs not cleaned")
	}
}
func TestPublicProtectionRoutesAndForwardedIP(t *testing.T) {
	a := &App{guard: newPublicGuard(guardConfig{List: 2, Detail: 4, Total: 5, Media: 20, Vendors: 10, Products: 20, MaxIPs: 10, Enabled: true})}
	r := gin.New()
	r.SetTrustedProxies(nil)
	r.Use(a.publicProtection())
	r.GET("/browse", func(c *gin.Context) { c.String(200, "browse") })
	r.GET("/api/v1/public/products", func(c *gin.Context) { c.JSON(200, gin.H{"items": []gin.H{{"id": 1, "kind": "product"}}}) })
	r.GET("/api/v1/media/:id", func(c *gin.Context) { c.Header("Accept-Ranges", "bytes"); c.Status(206) })
	call := func(path, ua, forward string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("User-Agent", ua)
		req.Header.Set("X-Forwarded-For", forward)
		r.ServeHTTP(w, req)
		return w
	}
	if call("/browse", "Mozilla", "").Code != 200 || call("/api/v1/public/products", "Mozilla", "").Code != 200 {
		t.Fatal("normal denied")
	}
	w := call("/api/v1/public/products", "Googlebot", "1.2.3.4")
	if w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatal("HTML/API or proxy spoof bypass", w.Code)
	}
	if call("/api/v1/media/1", "Mozilla", "").Code != 206 {
		t.Fatal("media Range broken")
	}
	if call("/browse", "GPTBot", "").Code != 403 {
		t.Fatal("declared AI bot accepted")
	}
	if call("/api/v1/public/products?q="+strings.Repeat("a", 201), "Mozilla", "").Code != 400 {
		t.Fatal("oversized query accepted")
	}
}
func TestShowroomAddressOwnershipConcurrencyAndHistory(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	admin := resourceTestSession(t, a, "admin")
	merchants := []string{tc.login("13900009501"), tc.login("13900009502")}
	vendors := []map[string]any{}
	for i, m := range merchants {
		v := tc.fixtureCall("POST", "/merchant/contents", m, gin.H{"kind": "vendor", "data": Payload{Name: fmt.Sprintf("河北真实测试机械企业%d", i), MainProducts: "测试产品", Phone: "13900009501", RegionName: "河北"}}, 201)
		vendors = append(vendors, v)
		tc.call("POST", "/merchant/contents/"+number(v["id"])+"/submit", m, gin.H{"revision_id": v["revision_id"]}, 200)
		tc.call("POST", "/admin/revisions/"+number(v["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	}
	tc.call("PUT", "/admin/vendors/"+number(vendors[1]["id"])+"/showroom-slug", merchants[0], gin.H{"slug": "bad-access"}, 403)
	for _, s := range []string{"admin", "v-999", "bad--slug", "bad-", "a", "汉字"} {
		tc.call("PUT", "/merchant/showroom-link", merchants[0], gin.H{"slug": s}, 400)
	}
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for _, m := range merchants {
		wg.Add(1)
		go func(token string) {
			defer wg.Done()
			data, _ := json.Marshal(gin.H{"slug": "Shared-Brand"})
			req := httptest.NewRequest("PUT", "/api/v1/merchant/showroom-link", bytes.NewReader(data))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			tc.router.ServeHTTP(w, req)
			statuses <- w.Code
		}(m)
	}
	wg.Wait()
	close(statuses)
	counts := map[int]int{}
	for s := range statuses {
		counts[s]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatal(counts)
	}
	var claimed Content
	a.DB.Where("showroom_slug=?", "shared-brand").First(&claimed)
	owner := merchants[0]
	if claimed.OwnerID != uint64(vendors[0]["owner_id"].(float64)) {
		owner = merchants[1]
	}
	tc.call("PUT", "/merchant/showroom-link", owner, gin.H{"slug": "another-brand"}, 409)
	tc.call("PUT", "/admin/vendors/"+fmt.Sprint(claimed.ID)+"/showroom-slug", admin, gin.H{"slug": "corrected-brand", "reason": "企业更正品牌拼写"}, 200)
	pub := tc.call("GET", "/public/showrooms/shared-brand", "", nil, 200)
	if pub["item"].(map[string]any)["showroom_slug"] != "corrected-brand" {
		t.Fatal("old alias lost")
	}
	// Migration restart must preserve registry entries, including historical aliases.
	a.DB.Exec("DELETE FROM schema_migrations WHERE version=14")
	if e := migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	var n int64
	a.DB.Model(&ShowroomAddress{}).Where("vendor_id=?", claimed.ID).Count(&n)
	if n != 2 {
		t.Fatal("migration retry lost history", n)
	}
	a.guard.cfg.Vendors = 1
	a.guard.clients = map[string]*guardClient{}
	tc.call("GET", "/public/showrooms/shared-brand", "", nil, 200)
	tc.call("GET", "/public/showrooms/v-"+fmt.Sprint(claimed.ID), "", nil, 200)
	otherID := number(vendors[0]["id"])
	if otherID == fmt.Sprint(claimed.ID) {
		otherID = number(vendors[1]["id"])
	}
	tc.call("GET", "/public/vendors/"+otherID, "", nil, 429)
}
