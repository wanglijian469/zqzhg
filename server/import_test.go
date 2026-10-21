package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"image"
	"image/png"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func importTestApp(t *testing.T) *App {
	t.Helper()
	if os.Getenv("RUN_MYSQL_TESTS") != "1" {
		t.Skip("requires isolated MySQL database")
	}
	name := fmt.Sprintf("zqzt_exhibition_test_%d", time.Now().UnixNano())
	db, e := openDB(name)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Exec("DROP DATABASE `" + name + "`"); pool, _ := db.DB(); pool.Close() })
	if e = migrate(db); e != nil {
		t.Fatal(e)
	}
	t.Setenv("ADMIN_PASSWORD", "Import-Test-Admin-2026!")
	if e = seed(db); e != nil {
		t.Fatal(e)
	}
	cfg := defaultGuardConfig()
	cfg.List = 100000
	cfg.Detail = 100000
	cfg.Total = 200000
	cfg.Media = 200000
	cfg.Vendors = 100000
	cfg.Products = 100000
	return &App{guard: newPublicGuard(cfg), DB: db, Config: Config{Env: "development", SMSProvider: "development", MediaDir: t.TempDir(), WebDir: t.TempDir()}, SMS: DevelopmentSMS{}}
}

func TestThousandProductsAndConcurrentLastSlot(t *testing.T) {
	a := importTestApp(t)
	router := a.router()
	tc := testClient{t, router}
	merchant := tc.login("13900001001")
	v := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "配额测试厂商（仅测试）", RegionName: "测试地区", MainProducts: "测试机械", Phone: "13900001001"}}, 201)
	var vendor Content
	a.DB.First(&vendor, uint64(v["id"].(float64)))
	// Explicit test fixtures count drafts/offline products as occupied slots.
	fixtures := make([]Content, 999)
	for i := range fixtures {
		fixtures[i] = Content{Kind: "product", OwnerID: vendor.OwnerID, VendorID: vendor.ID}
	}
	if e := a.DB.CreateInBatches(fixtures, 100).Error; e != nil {
		t.Fatal(e)
	}
	type response struct {
		status int
		body   map[string]any
	}
	results := make(chan response, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		go func(i int) {
			<-start
			b, _ := json.Marshal(gin.H{"kind": "product", "data": Payload{Name: fmt.Sprintf("并发边界测试%d", i), CategoryID: 1}})
			req := httptest.NewRequest("POST", "/api/v1/merchant/contents", bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+merchant)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			var body map[string]any
			json.Unmarshal(w.Body.Bytes(), &body)
			results <- response{w.Code, body}
		}(i)
	}
	close(start)
	accepted, rejected := 0, 0
	var product map[string]any
	for i := 0; i < 2; i++ {
		r := <-results
		switch r.status {
		case 201:
			accepted++
			product = r.body
		case 409:
			rejected++
			if r.body["error"].(map[string]any)["code"] != "product_limit_reached" {
				t.Fatalf("unexpected conflict: %v", r.body)
			}
		default:
			t.Fatalf("unexpected %d: %v", r.status, r.body)
		}
	}
	if accepted != 1 || rejected != 1 {
		t.Fatalf("last slot results: accepted=%d rejected=%d", accepted, rejected)
	}
	var count int64
	a.DB.Model(&Content{}).Where("kind='product' AND vendor_id=?", vendor.ID).Count(&count)
	if count != 1000 {
		t.Fatalf("quota exceeded: %d", count)
	}
	tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "product", "data": Payload{Name: "第1001款（测试）", CategoryID: 1}}, 409)
	tc.fixtureCall("PUT", "/merchant/contents/"+number(product["id"])+"/draft", merchant, gin.H{"revision_id": product["revision_id"], "data": Payload{Name: "满额仍可修改（测试）", CategoryID: 1}}, 200)
	other := tc.login("13900001002")
	tc.fixtureCall("POST", "/merchant/contents", other, gin.H{"kind": "vendor", "data": Payload{Name: "另一测试厂商", RegionName: "测试地区", MainProducts: "测试机械", Phone: "13900001002"}}, 201)
	tc.fixtureCall("POST", "/merchant/contents", other, gin.H{"kind": "product", "data": Payload{Name: "独立配额测试", CategoryID: 1}}, 201)
}

func TestVendorImportAtomicAndIdempotent(t *testing.T) {
	a := importTestApp(t)
	source := t.TempDir()
	f, e := os.Create(filepath.Join(source, "photo.png"))
	if e != nil {
		t.Fatal(e)
	}
	png.Encode(f, image.NewRGBA(image.Rect(0, 0, 20, 20)))
	f.Close()
	manifest := importManifest{SourceKey: "V9999", SourceHash: strings.Repeat("a", 64), SourceRoot: source, Vendor: importItem{Data: Payload{Name: "导入测试厂家（仅测试）", MainProducts: "翻转犁", Phone: "13900002001", RegionName: "测试地区"}, MediaPaths: []string{"photo.png"}}, Products: []importItem{{Data: Payload{Name: "导入测试产品"}, Category: "翻转犁", MediaPaths: []string{"missing.png"}}}}
	path := filepath.Join(source, "manifest.json")
	write := func() {
		b, _ := json.Marshal(manifest)
		if e := os.WriteFile(path, b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	write()
	if _, e = a.importVendor(path); e == nil {
		t.Fatal("missing image must fail the entire import")
	}
	var count int64
	a.DB.Model(&Content{}).Count(&count)
	if count != 0 {
		t.Fatal("partial contents survived rollback")
	}
	a.DB.Model(&Account{}).Where("username='import:V9999'").Count(&count)
	if count != 0 {
		t.Fatal("import account survived rollback")
	}
	files, _ := os.ReadDir(a.Config.MediaDir)
	if len(files) != 0 {
		t.Fatal("partial images survived rollback")
	}
	manifest.Products[0].MediaPaths = []string{"photo.png"}
	write()
	first, e := a.importVendor(path)
	if e != nil {
		t.Fatal(e)
	}
	if first.Products != 1 || first.Images != 2 {
		t.Fatalf("wrong import counts: %+v", first)
	}
	second, e := a.importVendor(path)
	if e != nil || !second.AlreadyImported || second.VendorID != first.VendorID {
		t.Fatalf("idempotence failed: %+v %v", second, e)
	}
	a.DB.Model(&Content{}).Count(&count)
	if count != 2 {
		t.Fatal("repeated import duplicated content")
	}
	tc := testClient{t, a.router()}
	tc.call("GET", fmt.Sprintf("/public/vendors/%d", first.VendorID), "", nil, 200)
	var media []Media
	a.DB.Find(&media)
	for _, m := range media {
		tc.call("GET", fmt.Sprintf("/media/%d", m.ID), "", nil, 200)
	}
	manifest.SourceHash = strings.Repeat("b", 64)
	write()
	if _, e = a.importVendor(path); e == nil {
		t.Fatal("changed source must preserve existing content")
	}
	manifest.SourceKey = "V9998"
	manifest.Products = make([]importItem, 1001)
	write()
	if _, e = a.importVendor(path); e == nil {
		t.Fatal("import cannot bypass quota")
	}
}
