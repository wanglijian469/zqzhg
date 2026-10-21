package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"image"
	"image/png"
	"strings"
	"testing"
)

func TestTaxonomyPayloadPresence(t *testing.T) {
	old := Payload{PrimaryCategoryID: 20, SecondaryCategoryIDs: []uint64{21}, Brand: "保留品牌", ApplicableModels: []string{"A100"}}
	var next Payload
	if e := json.Unmarshal([]byte(`{"name":"旧客户端编辑"}`), &next); e != nil {
		t.Fatal(e)
	}
	mergeTaxonomyFields(&next, old)
	if next.PrimaryCategoryID != 20 || next.Brand != old.Brand || len(next.ApplicableModels) != 1 {
		t.Fatal("omitted fields lost", next)
	}
	if e := json.Unmarshal([]byte(`{"primary_category_id":0,"secondary_category_ids":[],"brand":"","applicable_models":[]}`), &next); e != nil {
		t.Fatal(e)
	}
	mergeTaxonomyFields(&next, old)
	if next.PrimaryCategoryID != 0 || len(next.SecondaryCategoryIDs) != 0 || next.Brand != "" || len(next.ApplicableModels) != 0 {
		t.Fatal("explicit clear ignored", next)
	}
}

func TestAgricultureTaxonomyLifecycle(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	admin := resourceTestSession(t, a, "admin")
	owner := resourceTestSession(t, a, "merchant")
	other := resourceTestSession(t, a, "merchant")
	cat := func(code string) uint64 {
		var c Category
		if e := a.DB.Where("code=?", code).First(&c).Error; e != nil {
			t.Fatal(e)
		}
		return c.ID
	}
	vendorMain, secondary, third := cat("agri.vendor.01"), cat("agri.vendor.08"), cat("agri.vendor.09")
	productRoot, cylinder, gear := cat("agri.product.compact.07"), cat("agri.product.compact.07.01"), cat("agri.product.compact.05.01")
	vtax := tc.call("GET", "/public/taxonomy?category_kind=vendor", "", nil, 200)
	if len(vtax["categories"].([]any)) != 14 {
		t.Fatal("vendor seed count")
	}
	ptax := tc.call("GET", "/public/taxonomy?category_kind=product", "", nil, 200)
	roots := 0
	for _, r := range ptax["categories"].([]any) {
		if r.(map[string]any)["parent_id"] == float64(0) {
			roots++
		}
	}
	if roots != 10 {
		t.Fatal("product root count", roots)
	}
	if len(tc.call("GET", "/public/taxonomy", "", nil, 200)["categories"].([]any)) != 2 {
		t.Fatal("legacy taxonomy changed")
	}
	vp := gin.H{"name": "分类隔离测试厂家", "region_name": "河北", "main_products": "液压油缸", "phone": "0319-1234567"}
	v := tc.call("POST", "/merchant/contents", owner, gin.H{"kind": "vendor", "data": vp}, 201)
	tc.call("POST", "/merchant/contents/"+number(v["id"])+"/submit", owner, gin.H{"revision_id": v["revision_id"]}, 400)
	vp["primary_category_id"] = vendorMain
	vp["secondary_category_ids"] = []uint64{secondary, third}
	vp["business_nature"] = "manufacturer"
	v = tc.call("PUT", "/merchant/contents/"+number(v["id"])+"/draft", owner, gin.H{"revision_id": v["revision_id"], "data": vp}, 200)
	publish := func(item map[string]any) {
		tc.call("POST", "/merchant/contents/"+number(item["id"])+"/submit", owner, gin.H{"revision_id": item["revision_id"]}, 200)
		tc.call("POST", "/admin/revisions/"+number(item["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	}
	publish(v)
	if tc.call("GET", fmt.Sprintf("/public/vendors?category_id=%d", secondary), "", nil, 200)["total"] != float64(1) {
		t.Fatal("secondary category filter")
	}
	vp["secondary_category_ids"] = []uint64{vendorMain}
	tc.call("PUT", "/merchant/contents/"+number(v["id"])+"/draft", owner, gin.H{"revision_id": v["revision_id"], "data": vp}, 400)
	vp["secondary_category_ids"] = []uint64{secondary, third, cat("agri.vendor.07")}
	tc.call("PUT", "/merchant/contents/"+number(v["id"])+"/draft", owner, gin.H{"revision_id": v["revision_id"], "data": vp}, 400)
	pp := gin.H{"name": "测试液压油缸", "brand": "测试品牌", "model": "X100", "applicable_brands": []string{"品牌A"}, "applicable_models": []string{"A100"}}
	p := tc.call("POST", "/merchant/contents", owner, gin.H{"kind": "product", "data": pp}, 201)
	tc.call("POST", "/merchant/contents/"+number(p["id"])+"/submit", owner, gin.H{"revision_id": p["revision_id"]}, 400)
	pp["product_category_id"] = vendorMain
	tc.call("PUT", "/merchant/contents/"+number(p["id"])+"/draft", owner, gin.H{"revision_id": p["revision_id"], "data": pp}, 400)
	pp["product_category_id"] = productRoot
	tc.call("PUT", "/merchant/contents/"+number(p["id"])+"/draft", owner, gin.H{"revision_id": p["revision_id"], "data": pp}, 400)
	pp["product_category_id"] = cylinder
	p = tc.call("PUT", "/merchant/contents/"+number(p["id"])+"/draft", owner, gin.H{"revision_id": p["revision_id"], "data": pp}, 200)
	publish(p)
	for _, path := range []string{fmt.Sprintf("/public/products?category_id=%d", productRoot), "/public/products?q=A100", "/public/products?q=测试品牌", "/public/products?q=液压油缸"} {
		if tc.call("GET", path, "", nil, 200)["total"] != float64(1) {
			t.Fatal("product query", path)
		}
	}
	// Read old-client update fields from an actual pre-upgrade JSON body.
	oldp := gin.H{"name": "旧App继续编辑", "category_id": 0, "parameters": "历史参数原文"}
	edited := tc.call("PUT", "/merchant/contents/"+number(p["id"])+"/draft", owner, gin.H{"revision_id": p["revision_id"], "data": oldp}, 200)
	d := edited["data"].(map[string]any)
	if d["product_category_id"] != float64(cylinder) || d["brand"] != "测试品牌" || len(d["applicable_models"].([]any)) != 1 {
		t.Fatal("old client lost fields", d)
	}
	// A draft classification change must not affect current public filters.
	oldp["product_category_id"] = gear
	draft := tc.call("PUT", "/merchant/contents/"+number(p["id"])+"/draft", owner, gin.H{"revision_id": edited["revision_id"], "data": oldp}, 200)
	if tc.call("GET", fmt.Sprintf("/public/products?category_id=%d", gear), "", nil, 200)["total"] != float64(0) {
		t.Fatal("draft leaked")
	}
	if tc.call("GET", fmt.Sprintf("/public/products?category_id=%d", cylinder), "", nil, 200)["total"] != float64(1) {
		t.Fatal("published classification lost")
	}
	tc.call("PUT", "/merchant/contents/"+number(p["id"])+"/draft", other, gin.H{"revision_id": draft["revision_id"], "data": oldp}, 403)
	rec := tc.call("POST", "/public/category-recommendations", "", gin.H{"name": "旋耕刀"}, 200)
	if len(rec["items"].([]any)) == 0 {
		t.Fatal("alias recommendation absent")
	}
	if len(tc.call("POST", "/public/category-recommendations", "", gin.H{"name": "无匹配名称XYZ"}, 200)["items"].([]any)) != 0 {
		t.Fatal("fabricated recommendation")
	}
	belt := tc.call("POST", "/admin/industrial-belts", admin, gin.H{"name": "隔离测试产业带", "enabled": true}, 200)
	tc.call("PUT", "/admin/industrial-belts/"+number(belt["id"])+"/vendors", admin, gin.H{"vendor_ids": []any{v["id"]}}, 200)
	for _, path := range []string{"/public/vendors?industrial_belt_id=", "/public/products?industrial_belt_id="} {
		if tc.call("GET", path+number(belt["id"]), "", nil, 200)["total"] != float64(1) {
			t.Fatal("belt query", path)
		}
	}
	tc.call("DELETE", fmt.Sprintf("/admin/categories/%d", cylinder), admin, nil, 400)
	tc.call("POST", fmt.Sprintf("/admin/categories/%d/merge", cylinder), admin, gin.H{"target_id": vendorMain}, 400)
	// Third-level operations validate depth and preserve product references.
	leafParent := cat("agri.product.compact.04.04")
	child := tc.call("POST", "/admin/categories", admin, gin.H{"category_kind": "product", "name": "三级弹簧", "parent_id": leafParent, "enabled": true}, 200)
	tc.call("POST", "/admin/categories", admin, gin.H{"category_kind": "product", "name": "四级禁止", "parent_id": child["id"], "enabled": true}, 400)
	tc.call("DELETE", "/admin/categories/"+number(child["id"]), admin, nil, 200)
	tc.call("GET", "/admin/taxonomy-mappings", owner, nil, 403)
	// A stopped assignment remains usable by its already-published content,
	// while fresh selections, recommendations and public category images stop.
	keep := tc.call("POST", "/merchant/contents", owner, gin.H{"kind": "product", "data": gin.H{"name": "已有齿轮", "product_category_id": gear}}, 201)
	publish(keep)
	var imageBytes bytes.Buffer
	_ = png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 12, 12)))
	uploaded := resourceUpload(t, a, admin, "/admin/category-media", "classification.png", imageBytes.Bytes(), 201)
	var stopped Category
	a.DB.First(&stopped, gear)
	stopped.ImageMediaID = uint64(uploaded["id"].(float64))
	tc.call("PUT", fmt.Sprintf("/admin/categories/%d", gear), admin, stopped, 200)
	tc.call("GET", "/media/"+number(uploaded["id"]), "", nil, 200)
	stopped.Enabled = false
	tc.call("PUT", fmt.Sprintf("/admin/categories/%d", gear), admin, stopped, 200)
	tc.call("GET", "/media/"+number(uploaded["id"]), "", nil, 404)
	keep = tc.call("PUT", "/merchant/contents/"+number(keep["id"])+"/draft", owner, gin.H{"revision_id": keep["revision_id"], "data": gin.H{"name": "已有停用分类可继续编辑"}}, 200)
	publish(keep)
	tc.call("POST", "/merchant/contents", owner, gin.H{"kind": "product", "data": gin.H{"name": "新选停用分类应拒绝", "product_category_id": gear}}, 400)
}

func TestTaxonomyHistoricalBackfillAndMerge(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	admin := resourceTestSession(t, a, "admin")
	var cylinder, gear Category
	a.DB.Where("code=?", "agri.product.compact.07.01").First(&cylinder)
	a.DB.Where("code=?", "agri.product.compact.05.01").First(&gear)
	old := Category{HallID: 1, Name: "液压油缸", CategoryKind: "legacy", Enabled: true}
	if e := a.DB.Create(&old).Error; e != nil {
		t.Fatal(e)
	}
	snapshots := map[uint64]string{}
	for _, legacy := range []uint64{old.ID, 2, 0} {
		content := Content{HallID: 1, Kind: "product", OwnerID: 1}
		a.DB.Create(&content)
		blob, _ := json.Marshal(Payload{Name: "历史液压件", CategoryID: legacy, Parameters: "旧参数保留", MediaIDs: []uint64{}})
		rev := Revision{ContentID: content.ID, CategoryID: legacy, Payload: string(blob), Name: "历史液压件", Status: "approved"}
		if e := a.DB.Create(&rev).Error; e != nil {
			t.Fatal(e)
		}
		a.DB.Model(&content).Updates(map[string]any{"published_revision_id": rev.ID, "draft_revision_id": rev.ID, "visible": true})
		snapshots[rev.ID] = rev.Payload
	}
	for i := 0; i < 2; i++ {
		if e := initializeAgricultureTaxonomy(a.DB); e != nil {
			t.Fatal(e)
		}
	}
	var automatic, pending int64
	a.DB.Model(&TaxonomyMapping{}).Where("status='automatic'").Count(&automatic)
	a.DB.Model(&TaxonomyMapping{}).Where("status='pending'").Count(&pending)
	if automatic != 1 || pending != 2 {
		t.Fatal("ambiguous mapping", automatic, pending)
	}
	var mappings []TaxonomyMapping
	a.DB.Where("status='pending'").Find(&mappings)
	ids := []uint64{}
	for _, r := range mappings {
		ids = append(ids, r.RevisionID)
	}
	tc.call("POST", "/admin/taxonomy-mappings/resolve", admin, gin.H{"revision_ids": ids, "target_id": cylinder.ID, "preview": true}, 200)
	var unchanged int64
	a.DB.Model(&TaxonomyMapping{}).Where("status='pending'").Count(&unchanged)
	if unchanged != 2 {
		t.Fatal("preview mutated data")
	}
	tc.call("POST", "/admin/taxonomy-mappings/resolve", admin, gin.H{"revision_ids": ids, "target_id": cylinder.ID}, 200)
	for id, blob := range snapshots {
		var rev Revision
		a.DB.First(&rev, id)
		if rev.Payload != blob {
			t.Fatal("historical snapshot overwritten")
		}
	}
	// Retry after DDL completed but the migration marker was not committed.
	if e := a.DB.Exec("DELETE FROM schema_migrations WHERE version=17").Error; e != nil {
		t.Fatal(e)
	}
	if e := migrate(a.DB); e != nil {
		t.Fatal("interrupted migration retry", e)
	}
	for id, blob := range snapshots {
		var rev Revision
		a.DB.First(&rev, id)
		if rev.Payload != blob {
			t.Fatal("migration retry changed historical payload")
		}
	}
	tc.call("POST", fmt.Sprintf("/admin/categories/%d/merge", cylinder.ID), admin, gin.H{"target_id": gear.ID}, 200)
	// A retained source cannot be merged a second time or used to create a cycle.
	tc.call("POST", fmt.Sprintf("/admin/categories/%d/merge", cylinder.ID), admin, gin.H{"target_id": gear.ID}, 400)
	tc.call("POST", fmt.Sprintf("/admin/categories/%d/merge", gear.ID), admin, gin.H{"target_id": cylinder.ID}, 400)
	cylinder.Enabled = true
	tc.call("PUT", fmt.Sprintf("/admin/categories/%d", cylinder.ID), admin, cylinder, 400)
	for id := range snapshots {
		items := []View{{RevisionID: id, Content: Content{Kind: "product"}}}
		if e := a.hydrateTaxonomyViews(items); e != nil {
			t.Fatal(e)
		}
		if items[0].Data.ProductCategoryID != gear.ID {
			t.Fatal("merged mapping not resolved")
		}
	}
	var originals int64
	a.DB.Model(&Category{}).Where("id IN ?", []uint64{1, 2, old.ID}).Count(&originals)
	if originals != 3 {
		t.Fatal("legacy removed")
	}
	if !strings.Contains(categoryPath([]Category{{ID: 1, Name: "父"}, {ID: 2, ParentID: 1, Name: "子"}}, 2, false), "父 / 子") {
		t.Fatal("path")
	}
}
