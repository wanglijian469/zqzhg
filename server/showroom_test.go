package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShowroomProfileAndPublication(t *testing.T) {
	a := importTestApp(t)
	a.Config.PublicBaseURL = "https://exhibition.example"
	tc := testClient{t, a.router()}
	admin := tc.call("POST", "/auth/admin", "", gin.H{"username": "admin", "password": "Import-Test-Admin-2026!"}, 200)["token"].(string)
	merchant := tc.login("13900009101")
	other := tc.login("13900009102")
	d := Payload{Name: "展台测试厂家（仅测试）", MainProducts: "型号 TEST", Phone: "13900009101", RegionName: "测试地区", CompanyProfile: &CompanyProfile{ShortName: "展台测试", Summary: "旧公开简介", Website: "https://example.com", Contacts: []CompanyContact{{Area: "测试区域", Name: "业务员", Phone: "13900009101"}}}}
	vendor := tc.call("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": d}, 201)
	vid := number(vendor["id"])
	if vendor["showroom_slug"] != "v-"+vid {
		t.Fatal("missing default URL")
	}
	tc.call("GET", "/public/showrooms/v-"+vid, "", nil, 404)
	tc.call("PUT", "/admin/vendors/"+vid+"/showroom-slug", merchant, gin.H{"slug": "sample"}, 403)
	tc.call("PUT", "/admin/vendors/"+vid+"/showroom-slug", admin, gin.H{"slug": "v-999"}, 400)
	tc.call("PUT", "/admin/vendors/"+vid+"/showroom-slug", admin, gin.H{"slug": "sample"}, 200)
	tc.call("PUT", "/admin/vendors/"+vid+"/showroom-slug", admin, gin.H{"slug": "changed"}, 409)
	v2 := tc.call("POST", "/merchant/contents", other, gin.H{"kind": "vendor", "data": Payload{Name: "另一个测试厂", MainProducts: "TEST", Phone: "13900009102", RegionName: "测试地区"}}, 201)
	tc.call("PUT", "/admin/vendors/"+number(v2["id"])+"/showroom-slug", admin, gin.H{"slug": "sample"}, 409)
	approve := func(v map[string]any) {
		tc.call("POST", "/merchant/contents/"+number(v["id"])+"/submit", merchant, gin.H{"revision_id": v["revision_id"]}, 200)
		tc.call("POST", "/admin/revisions/"+number(v["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	}
	approve(vendor)
	pub := tc.call("GET", "/public/showrooms/sample", "", nil, 200)
	if pub["canonical_url"] != "https://exhibition.example/v/sample" {
		t.Fatal("wrong canonical URL")
	}
	tc.call("GET", "/public/showrooms/v-"+vid, "", nil, 200)
	tc.call("GET", "/public/vendors/"+vid, "", nil, 200)
	// A legacy app omits the new field; preserve the company profile.
	legacy := d
	legacy.CompanyProfile = nil
	legacy.Name = "旧客户端编辑"
	draft := tc.call("PUT", "/merchant/contents/"+vid+"/draft", merchant, gin.H{"revision_id": vendor["revision_id"], "data": legacy}, 200)
	profile := draft["data"].(map[string]any)["company_profile"].(map[string]any)
	if profile["summary"] != "旧公开简介" {
		t.Fatal("legacy save erased profile")
	}
	d.CompanyProfile.Summary = "仅待审的新简介"
	next := tc.call("PUT", "/merchant/contents/"+vid+"/draft", merchant, gin.H{"revision_id": draft["revision_id"], "data": d}, 200)
	tc.call("POST", "/merchant/contents/"+vid+"/submit", merchant, gin.H{"revision_id": next["revision_id"]}, 200)
	tc.call("PUT", "/merchant/contents/"+vid+"/draft", merchant, gin.H{"revision_id": next["revision_id"], "data": d}, 409)
	pub = tc.call("GET", "/public/showrooms/sample", "", nil, 200)
	if pub["item"].(map[string]any)["data"].(map[string]any)["company_profile"].(map[string]any)["summary"] != "旧公开简介" {
		t.Fatal("pending profile leaked")
	}
	os.WriteFile(filepath.Join(a.Config.WebDir, "index.html"), []byte(`<html><head><title>Default</title></head><body><div id="root"></div></body></html>`), 0600)
	w := httptest.NewRecorder()
	tc.router.ServeHTTP(w, httptest.NewRequest("GET", "/v/sample", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `https://exhibition.example/v/sample`) || strings.Contains(w.Body.String(), "仅待审的新简介") {
		t.Fatal("metadata publication mismatch")
	}
	tc.call("POST", "/admin/contents/"+vid+"/offline", admin, gin.H{}, 200)
	tc.call("GET", "/public/showrooms/sample", "", nil, 404)
	w = httptest.NewRecorder()
	tc.router.ServeHTTP(w, httptest.NewRequest("GET", "/v/sample", nil))
	if w.Code != 404 || !strings.Contains(w.Body.String(), "noindex") || strings.Contains(w.Body.String(), "旧公开简介") {
		t.Fatal("offline metadata leaked")
	}
}

func TestShowroomMediaAndThousandProductCatalog(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	merchant := tc.login("13900009103")
	var owner Account
	a.DB.Where("username=?", "13900009103").First(&owner)
	logo := Media{OwnerID: owner.ID, Path: "test-logo.png", ThumbPath: "test-logo-thumb.png"}
	a.DB.Create(&logo)
	foreign := Media{OwnerID: owner.ID + 999, Path: "foreign.png", ThumbPath: "foreign-thumb.png"}
	a.DB.Create(&foreign)
	d := Payload{Name: "千款目录测试厂", RegionName: "测试地区", Phone: "13900009103", MainProducts: "TEST", CompanyProfile: &CompanyProfile{CoverMediaID: foreign.ID}}
	tc.call("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": d}, 400)
	d.CompanyProfile.CoverMediaID = logo.ID
	d.CompanyProfile.LogoMediaID = logo.ID
	vendor := tc.call("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": d}, 201)
	var refs int64
	a.DB.Model(&MediaReference{}).Where("revision_id=? AND media_id=?", vendor["revision_id"], logo.ID).Count(&refs)
	if refs != 1 {
		t.Fatal("cover/logo must share one version reference")
	}
	admin := tc.call("POST", "/auth/admin", "", gin.H{"username": "admin", "password": "Import-Test-Admin-2026!"}, 200)["token"].(string)
	tc.call("POST", "/merchant/contents/"+number(vendor["id"])+"/submit", merchant, gin.H{"revision_id": vendor["revision_id"]}, 200)
	tc.call("POST", "/admin/revisions/"+number(vendor["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	child := Category{HallID: 1, ParentID: 1, Name: "测试子分类", Enabled: true}
	a.DB.Create(&child)
	products := make([]Content, 1000)
	for i := range products {
		products[i] = Content{Kind: "product", OwnerID: owner.ID, VendorID: uint64(vendor["id"].(float64)), Visible: true}
	}
	if e := a.DB.CreateInBatches(products, 100).Error; e != nil {
		t.Fatal(e)
	}
	revisions := make([]Revision, 1000)
	for i, p := range products {
		payload := Payload{Name: fmt.Sprintf("TEST-%04d", i), CategoryID: child.ID}
		blob, _ := json.Marshal(payload)
		revisions[i] = Revision{ContentID: p.ID, Name: payload.Name, CategoryID: child.ID, Status: "approved", Payload: string(blob)}
	}
	if e := a.DB.CreateInBatches(revisions, 100).Error; e != nil {
		t.Fatal(e)
	}
	for i, p := range products {
		if e := a.DB.Model(&p).Update("published_revision_id", revisions[i].ID).Error; e != nil {
			t.Fatal(e)
		}
	}
	pub := tc.call("GET", "/public/showrooms/"+vendor["showroom_slug"].(string), "", nil, 200)
	if pub["product_count"].(float64) != 1000 || len(pub["categories"].([]any)) != 2 {
		t.Fatal("catalog count or category ancestors wrong")
	}
	for _, p := range []int{1, 2, 84} {
		result := tc.call("GET", fmt.Sprintf("/public/products?vendor_id=%s&category_id=1&page_size=12&page=%d", number(vendor["id"]), p), "", nil, 200)
		expected := 12
		if p == 84 {
			expected = 4
		}
		if result["total"].(float64) != 1000 || len(result["items"].([]any)) != expected {
			t.Fatal("1000 product pagination failed")
		}
	}
	tc.call("POST", "/admin/contents/"+number(vendor["id"])+"/offline", admin, gin.H{}, 200)
	if tc.call("GET", "/public/products?vendor_id="+number(vendor["id"]), "", nil, 200)["total"].(float64) != 0 {
		t.Fatal("offline catalog exposed")
	}
}
