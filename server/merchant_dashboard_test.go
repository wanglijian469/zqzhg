package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/url"
	"testing"
)

func TestMerchantSpecificationSearchIsScopedAndCompatible(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	merchant, other := tc.login("13900009511"), tc.login("13900009512")
	tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "规格检索测试厂家", RegionName: "测试地区", MainProducts: "机械", Phone: "13900009511"}}, 201)
	for i := 0; i < 23; i++ {
		tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "product", "data": Payload{Name: fmt.Sprintf("检索设备 %02d", i), CategoryID: 1, Parameters: "型号：1LF-440\n规格：液压设备\n特殊：A%_\\", Description: "PRIVATE_ONLY_DESCRIPTION"}}, 201)
	}
	for _, test := range []struct {
		query             string
		total, page, size int
	}{
		{"1LF-440", 23, 1, 20}, {"液压", 23, 2, 3}, {"A%_\\", 23, 1, 20},
		{"PRIVATE_ONLY_DESCRIPTION", 0, 1, 0}, {"not-present", 0, 1, 0},
	} {
		path := fmt.Sprintf("/merchant/contents?kind=product&status=draft&q=%s&page=%d", url.QueryEscape(test.query), test.page)
		r := tc.call("GET", path, merchant, nil, 200)
		if r["total"] != float64(test.total) || len(r["items"].([]any)) != test.size {
			t.Fatal(path, r)
		}
		if r := tc.call("GET", path, other, nil, 200); r["total"] != float64(0) {
			t.Fatal("cross-account specifications leaked", r)
		}
	}
	var admin Account
	a.DB.Where("role='admin'").First(&admin)
	session, err := a.newSession(admin)
	if err != nil {
		t.Fatal(err)
	}
	if r := tc.call("GET", "/admin/contents?kind=product&q=1LF-440", session["token"].(string), nil, 200); r["total"] != float64(0) {
		t.Fatal("administrator search changed", r)
	}
	if r := tc.call("GET", "/merchant/contents?kind=product&q=1LF-440&public_state=public", merchant, nil, 200); r["total"] != float64(0) {
		t.Fatal("draft exposed as public", r)
	}
	first := tc.call("GET", "/merchant/contents?kind=product&page_size=1", merchant, nil, 200)["items"].([]any)[0].(map[string]any)
	tc.call("GET", "/merchant/contents/"+number(first["id"]), other, nil, 403)
	tc.call("POST", "/merchant/contents/"+number(first["id"])+"/offline", other, gin.H{}, 403)
}

func TestMerchantDashboardFiltersAndPublication(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	merchant := tc.login("13900009501")
	other := tc.login("13900009502")
	var adminAccount Account
	if e := a.DB.Where("username=? AND role=?", "admin", "admin").First(&adminAccount).Error; e != nil {
		t.Fatal(e)
	}
	adminSession, e := a.newSession(adminAccount)
	if e != nil {
		t.Fatal(e)
	}
	admin := adminSession["token"].(string)
	vendor := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "工作台测试厂商", RegionName: "测试地区", MainProducts: "工作台农机", Phone: "13900009501"}}, 201)
	approve := func(item map[string]any) {
		tc.call("POST", "/merchant/contents/"+number(item["id"])+"/submit", merchant, gin.H{"revision_id": item["revision_id"]}, 200)
		tc.call("POST", "/admin/revisions/"+number(item["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	}
	approve(vendor)
	var products []map[string]any
	for i := 0; i < 23; i++ {
		products = append(products, tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "product", "data": Payload{Name: fmt.Sprintf("MATCH 工作台产品 %02d", i), CategoryID: 1}}, 201))
	}
	approve(products[0])
	revised := tc.fixtureCall("PUT", "/merchant/contents/"+number(products[0]["id"])+"/draft", merchant, gin.H{"revision_id": products[0]["revision_id"], "data": Payload{Name: "MATCH 新待审文字", CategoryID: 1}}, 200)
	for _, item := range []map[string]any{revised, products[1], products[2]} {
		tc.call("POST", "/merchant/contents/"+number(item["id"])+"/submit", merchant, gin.H{"revision_id": item["revision_id"]}, 200)
	}
	tc.call("POST", "/admin/revisions/"+number(products[2]["revision_id"])+"/review", admin, gin.H{"decision": "reject", "note": "请补充参数"}, 200)
	check := func(token string, want gin.H) {
		t.Helper()
		counts := tc.call("GET", "/merchant/dashboard", token, nil, 200)["products"].(map[string]any)
		for key, value := range want {
			if counts[key] != float64(value.(int)) {
				t.Fatalf("%s: %v != %v", key, counts[key], value)
			}
		}
	}
	check(merchant, gin.H{"total": 23, "public": 1, "draft": 20, "pending": 2, "rejected": 1})
	check(other, gin.H{"total": 0, "public": 0, "draft": 0, "pending": 0, "rejected": 0})
	for _, test := range []struct {
		path  string
		total int
		size  int
	}{{"?kind=product&q=MATCH&page=2", 23, 3}, {"?kind=product&q=MATCH&status=pending&public_state=public", 1, 1}, {"?kind=product&q=MATCH&status=pending&public_state=unpublished", 1, 1}, {"?kind=product&q=不存在", 0, 0}} {
		r := tc.call("GET", "/merchant/contents"+test.path, merchant, nil, 200)
		if r["total"] != float64(test.total) || len(r["items"].([]any)) != test.size {
			t.Fatal(test.path, r)
		}
	}
	if r := tc.call("GET", "/merchant/contents?kind=product&q=MATCH", other, nil, 200); r["total"] != float64(0) {
		t.Fatal("cross-account search leaked", r)
	}
	tc.call("POST", "/merchant/contents/"+number(vendor["id"])+"/offline", merchant, gin.H{}, 200)
	check(merchant, gin.H{"total": 23, "public": 0})
	if r := tc.call("GET", "/merchant/contents?kind=product&public_state=public", merchant, nil, 200); r["total"] != float64(0) {
		t.Fatal("offline parent remained public", r)
	}
	tc.call("GET", "/merchant/dashboard", "", nil, 401)
}

func TestAppDownloadConfigDoesNotEncodeLoopback(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	t.Setenv("APP_DOWNLOAD_BASE_URL", "http://172.27.35.2:8080")
	r := tc.call("GET", "/public/app", "", nil, 200)
	if r["qr_url"] != "http://172.27.35.2:8080/downloads/dalunongji.apk" || r["ios_available"] != false {
		t.Fatal(r)
	}
	t.Setenv("APP_DOWNLOAD_BASE_URL", "http://localhost:8080")
	if r := tc.call("GET", "/public/app", "", nil, 200); r["qr_url"] != "" {
		t.Fatal("loopback QR must not be offered", r)
	}
}
