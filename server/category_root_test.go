package main

import (
	"github.com/gin-gonic/gin"
	"testing"
)

func TestMerchantRootCategoriesScopePublishAndNormalization(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	owner := resourceTestSession(t, a, "merchant")
	other := resourceTestSession(t, a, "merchant")
	admin := resourceTestSession(t, a, "admin")
	tc.call("POST", "/merchant/categories", "", gin.H{"name": "齿轮", "parent_id": 0}, 401)
	root := tc.call("POST", "/merchant/categories", owner, gin.H{"name": "齿轮", "name_en": "Gears", "parent_id": 0}, 201)
	rid := number(root["id"])
	if root["parent_id"] != float64(0) || root["hall_id"] != float64(1) || root["suggestion_status"] != "pending" {
		t.Fatal("wrong root", root)
	}
	again := tc.call("POST", "/merchant/categories", owner, gin.H{"name": "齿轮", "parent_id": 0}, 201)
	if again["id"] != root["id"] {
		t.Fatal("duplicate root")
	}
	foreign := tc.call("POST", "/merchant/categories", other, gin.H{"name": "齿轮", "parent_id": 0}, 201)
	if foreign["id"] == root["id"] {
		t.Fatal("foreign suggestion reused")
	}
	contains := func(path, token string, id any) bool {
		v := tc.call("GET", path, token, nil, 200)
		for _, row := range v["categories"].([]any) {
			if row.(map[string]any)["id"] == id {
				return true
			}
		}
		return false
	}
	if !contains("/merchant/taxonomy", owner, root["id"]) || contains("/merchant/taxonomy", other, root["id"]) || contains("/public/taxonomy", "", root["id"]) {
		t.Fatal("root scope")
	}
	tc.call("POST", "/merchant/categories", owner, gin.H{"name": "错误上级", "parent_id": 999999}, 400)
	tc.call("POST", "/merchant/categories", owner, gin.H{"name": "待规范不能为上级", "parent_id": root["id"]}, 400)
	v := tc.fixtureCall("POST", "/merchant/contents", owner, gin.H{"kind": "vendor", "data": Payload{Name: "一级分类验收企业", RegionName: "河北", MainProducts: "齿轮", Phone: "0319-1234567", CategoryID: uint64(root["id"].(float64))}}, 201)
	product := tc.fixtureCall("POST", "/merchant/contents", owner, gin.H{"kind": "product", "data": Payload{Name: "齿轮展品", CategoryID: uint64(root["id"].(float64))}}, 201)
	for _, item := range []map[string]any{v, product} {
		tc.call("POST", "/merchant/contents/"+number(item["id"])+"/submit", owner, gin.H{"revision_id": item["revision_id"]}, 200)
		tc.call("POST", "/admin/revisions/"+number(item["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	}
	if tc.call("GET", "/public/products?category_id="+rid, "", nil, 200)["total"] != float64(1) {
		t.Fatal("pending root publication filter")
	}
	tc.call("POST", "/admin/category-suggestions/"+rid+"/normalize", admin, gin.H{"action": "approve", "name": "齿轮传动", "parent_id": 0}, 200)
	if !contains("/public/taxonomy", "", root["id"]) {
		t.Fatal("approved root absent")
	}
	child := tc.call("POST", "/merchant/categories", owner, gin.H{"name": "斜齿轮", "parent_id": root["id"]}, 201)
	tc.call("POST", "/admin/category-suggestions/"+number(child["id"])+"/normalize", admin, gin.H{"action": "approve", "name": "斜齿轮", "parent_id": 0}, 200)
	var c Category
	a.DB.First(&c, uint64(child["id"].(float64)))
	if c.ParentID != 0 || c.HallID != 1 {
		t.Fatal("promote child to root")
	}
	tc.call("POST", "/admin/category-suggestions/"+number(foreign["id"])+"/normalize", admin, gin.H{"action": "merge", "target_id": root["id"]}, 200)
	tc.call("POST", "/admin/category-suggestions/"+rid+"/normalize", admin, gin.H{"action": "disable"}, 200)
	if contains("/public/taxonomy", "", root["id"]) {
		t.Fatal("disabled root exposed")
	}
	var count int64
	a.DB.Model(&Content{}).Count(&count)
	if count != 2 {
		t.Fatal("normalization removed existing contents")
	}
}
