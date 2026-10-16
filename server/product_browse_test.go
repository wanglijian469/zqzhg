package main

import (
	"github.com/gin-gonic/gin"
	"testing"
)

func TestPublicProductBrowse(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	admin := tc.call("POST", "/auth/admin", "", gin.H{"username": "admin", "password": "Import-Test-Admin-2026!"}, 200)["token"].(string)
	category := tc.call("POST", "/admin/categories", admin, gin.H{"name": "产品搜索测试子分类", "parent_id": 1, "enabled": true}, 200)
	approve := func(item map[string]any, token string) {
		tc.call("POST", "/merchant/contents/"+number(item["id"])+"/submit", token, gin.H{"revision_id": item["revision_id"]}, 200)
		tc.call("POST", "/admin/revisions/"+number(item["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	}
	create := func(token, kind string, data Payload) map[string]any {
		return tc.call("POST", "/merchant/contents", token, gin.H{"kind": kind, "data": data}, 201)
	}
	merchant := tc.login("13900008101")
	vendorData := Payload{Name: "产品浏览测试厂家（仅测试）", NameEN: "Browse Test Factory", MainProducts: "测试翻转犁", RegionName: "产品浏览测试地区甲", Phone: "13900008101"}
	vendor := create(merchant, "vendor", vendorData)
	product := create(merchant, "product", Payload{Name: "型号TEST-440翻转犁", NameEN: "TEST-440 reversible plough", Keywords: "翻转犁 plough", CategoryID: uint64(category["id"].(float64))})
	approve(product, merchant)
	if tc.call("GET", "/public/products", "", nil, 200)["total"].(float64) != 0 {
		t.Fatal("product exposed before its booth is public")
	}
	approve(vendor, merchant)
	second := create(merchant, "product", Payload{Name: "型号TEST-550翻转犁", Keywords: "翻转犁 plough", CategoryID: uint64(category["id"].(float64))})
	approve(second, merchant)
	create(merchant, "product", Payload{Name: "仅草稿的TEST-999", CategoryID: 1})
	pending := create(merchant, "product", Payload{Name: "仅待审的TEST-888", CategoryID: 1})
	tc.call("POST", "/merchant/contents/"+number(pending["id"])+"/submit", merchant, gin.H{"revision_id": pending["revision_id"]}, 200)
	other := tc.login("13900008102")
	otherVendor := create(other, "vendor", Payload{Name: "另一地区测试厂商", MainProducts: "测试产品", RegionName: "产品浏览测试地区乙", Phone: "13900008102"})
	approve(otherVendor, other)
	otherProduct := create(other, "product", Payload{Name: "型号OTHER-660", Keywords: "plough", CategoryID: 1})
	approve(otherProduct, other)
	all := tc.call("GET", "/public/products", "", nil, 200)
	if all["total"].(float64) != 3 {
		t.Fatal("draft or pending products appeared in the public list")
	}
	first := tc.call("GET", "/public/products?q=TEST-440", "", nil, 200)
	if first["total"].(float64) != 1 {
		t.Fatal("model search failed")
	}
	summary := first["items"].([]any)[0].(map[string]any)["vendor"].(map[string]any)
	if summary["name"] != vendorData.Name || summary["name_en"] != vendorData.NameEN || summary["region_name"] != vendorData.RegionName || number(summary["id"]) != number(vendor["id"]) {
		t.Fatal("incorrect public vendor summary")
	}
	region := number(summary["region_id"])
	for _, query := range []string{"?q=plough&category_id=1&region_id=" + region, "?q=翻转犁&category_id=" + number(category["id"]) + "&region_id=" + region, "?region_id=" + region} {
		if tc.call("GET", "/public/products"+query, "", nil, 200)["total"].(float64) != 2 {
			t.Fatalf("combined/descendant/region filter failed: %s", query)
		}
	}
	if tc.call("GET", "/public/products?q=not-matching-model", "", nil, 200)["total"].(float64) != 0 {
		t.Fatal("empty search should return no products")
	}
	page1 := tc.call("GET", "/public/products?region_id="+region+"&page_size=1", "", nil, 200)
	page2 := tc.call("GET", "/public/products?region_id="+region+"&page_size=1&page=2", "", nil, 200)
	if page1["total"].(float64) != 2 || len(page1["items"].([]any)) != 1 || page1["items"].([]any)[0].(map[string]any)["id"] == page2["items"].([]any)[0].(map[string]any)["id"] {
		t.Fatal("pagination lost or repeated products")
	}
	// A pending vendor edit must not change the public name or region filter.
	vendorData.Name = "仅待审的新厂家名称"
	vendorData.RegionName = "产品浏览测试地区乙"
	vendorData.RegionID = 0
	newDraft := tc.call("PUT", "/merchant/contents/"+number(vendor["id"])+"/draft", merchant, gin.H{"revision_id": vendor["revision_id"], "data": vendorData}, 200)
	tc.call("POST", "/merchant/contents/"+number(vendor["id"])+"/submit", merchant, gin.H{"revision_id": newDraft["revision_id"]}, 200)
	public := tc.call("GET", "/public/products?region_id="+region, "", nil, 200)
	if public["total"].(float64) != 2 || public["items"].([]any)[0].(map[string]any)["vendor"].(map[string]any)["name"] == vendorData.Name {
		t.Fatal("vendor pending draft leaked into public search")
	}
	tc.call("POST", "/merchant/contents/"+number(second["id"])+"/offline", merchant, gin.H{}, 200)
	if tc.call("GET", "/public/products?region_id="+region, "", nil, 200)["total"].(float64) != 1 {
		t.Fatal("offline product stayed in results")
	}
	tc.call("POST", "/admin/contents/"+number(vendor["id"])+"/offline", admin, gin.H{}, 200)
	if tc.call("GET", "/public/products?region_id="+region, "", nil, 200)["total"].(float64) != 0 {
		t.Fatal("offline vendor products stayed public")
	}
	tc.call("GET", "/public/products/"+number(product["id"]), "", nil, 404)
}
