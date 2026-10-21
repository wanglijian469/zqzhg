package main

import (
	"github.com/gin-gonic/gin"
	"strings"
	"testing"
)

func TestProductV22ClassifiedReviewAndCompatibility(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	merchant := resourceTestSession(t, a, "merchant")
	admin := resourceTestSession(t, a, "admin")
	other := resourceTestSession(t, a, "merchant")
	vendor := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "V22测试企业", MainProducts: "机械", RegionName: "河北", Phone: "0319-1234567"}}, 201)
	approve := func(item map[string]any) {
		tc.call("POST", "/merchant/contents/"+number(item["id"])+"/submit", merchant, gin.H{"revision_id": item["revision_id"]}, 200)
		tc.call("POST", "/admin/revisions/"+number(item["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	}
	approve(vendor)
	tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "product", "data": Payload{}}, 400)
	tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "product", "data": Payload{Name: strings.Repeat("😀", 201)}}, 400)
	tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "product", "data": Payload{Name: "无效分类", CategoryID: 999999}}, 400)
	product := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "product", "data": Payload{Name: "仅名称V22产品"}}, 201)
	if product["data"].(map[string]any)["keywords"] != "" {
		t.Fatal("generated unexpected keywords")
	}
	tc.fixtureCall("PUT", "/merchant/contents/"+number(product["id"])+"/draft", other, gin.H{"revision_id": product["revision_id"], "data": Payload{Name: "越权"}}, 403)
	tc.call("POST", "/merchant/contents/"+number(product["id"])+"/submit", merchant, gin.H{"revision_id": product["revision_id"]}, 200)
	tc.fixtureCall("PUT", "/merchant/contents/"+number(product["id"])+"/draft", merchant, gin.H{"revision_id": product["revision_id"], "data": Payload{Name: "待审禁止修改"}}, 409)
	tc.call("POST", "/admin/revisions/"+number(product["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	result := tc.call("GET", "/public/products?q=仅名称V22", "", nil, 200)
	if result["total"] != float64(1) {
		t.Fatal(result)
	}
	edited := tc.fixtureCall("PUT", "/merchant/contents/"+number(product["id"])+"/draft", merchant, gin.H{"revision_id": product["revision_id"], "data": Payload{Name: "更新V22产品", Parameters: "原始规格=旧值\n型号：V22-440", NameEN: "Legacy equipment"}}, 200)
	approve(edited)
	result = tc.call("GET", "/public/products?q=V22-440", "", nil, 200)
	if result["total"] != float64(1) {
		t.Fatal(result)
	}
	tc.fixtureCall("POST", "/admin/vendors/"+number(vendor["id"])+"/products", admin, gin.H{"data": Payload{Name: "后台仅名称产品"}}, 201)
}
