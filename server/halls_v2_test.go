package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestHallV2MigrationIsolationAndLegacyDefaults(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	halls := tc.call("GET", "/public/halls", "", nil, 200)["items"].([]any)
	if len(halls) != 2 {
		t.Fatal(halls)
	}
	var cable Hall
	if e := a.DB.Where("slug='cable'").First(&cable).Error; e != nil {
		t.Fatal(e)
	}
	if cable.Status != "preparing" {
		t.Fatal(cable)
	}
	if e := a.DB.Exec("DELETE FROM schema_migrations WHERE version=11").Error; e != nil {
		t.Fatal(e)
	}
	if e := migrate(a.DB); e != nil {
		t.Fatal("interrupted migration retry", e)
	}
	if e := migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	merchant := resourceTestSession(t, a, "merchant")
	data := Payload{Name: "V2真实接口测试厂商", MainProducts: "机械", RegionName: "测试地区", Phone: "13900009111"}
	vendor := tc.call("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": data}, 201)
	if vendor["hall_id"] != float64(1) {
		t.Fatal("legacy default hall", vendor)
	}
	product := tc.call("POST", "/merchant/contents", merchant, gin.H{"kind": "product", "data": Payload{Name: "参数型号测试", Keywords: "机械", CategoryID: 1, Parameters: "型号：V2-MODEL-991"}}, 201)
	if product["hall_id"] != float64(1) {
		t.Fatal("product hall inheritance", product)
	}
	admin := resourceTestSession(t, a, "admin")
	for _, item := range []map[string]any{vendor, product} {
		tc.call("POST", "/merchant/contents/"+number(item["id"])+"/submit", merchant, gin.H{"revision_id": item["revision_id"]}, 200)
		tc.call("POST", "/admin/revisions/"+number(item["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	}
	for _, endpoint := range []string{"vendors", "products"} {
		r := tc.call("GET", "/public/"+endpoint+"?q=V2-MODEL-991", "", nil, 200)
		if r["total"] != float64(1) {
			t.Fatal("parameter model search", endpoint, r)
		}
		r = tc.call("GET", "/public/"+endpoint+"?hall=cable", "", nil, 200)
		if r["total"] != float64(0) {
			t.Fatal("preparing hall leaked data", r)
		}
		tc.call("GET", "/public/"+endpoint+"?hall=unknown", "", nil, 404)
	}
	for _, endpoint := range []string{"home", "taxonomy"} {
		r := tc.call("GET", "/public/"+endpoint+"?hall=cable", "", nil, 200)
		key := "vendors"
		if endpoint == "taxonomy" {
			key = "categories"
		}
		if len(r[key].([]any)) != 0 {
			t.Fatal("preparing hall leaked", endpoint, r)
		}
	}
	cableCategory := Category{HallID: cable.ID, Name: "电缆", Enabled: true}
	a.DB.Create(&cableCategory)
	tc.call("POST", "/merchant/contents", merchant, gin.H{"kind": "product", "data": Payload{Name: "跨厅拒绝", CategoryID: cableCategory.ID}}, 400)
	// An open second hall still must not expose agriculture in any query.
	a.DB.Model(&cable).Update("status", "open")
	for _, endpoint := range []string{"vendors", "products"} {
		r := tc.call("GET", "/public/"+endpoint+"?hall=cable", "", nil, 200)
		if r["total"] != float64(0) {
			t.Fatal("open hall isolation", r)
		}
	}
	config := tc.call("GET", "/admin/home-display?hall=cable", admin, nil, 200)
	tc.call("PUT", "/admin/home-display?hall=cable", admin, gin.H{"hero_media_id": 0, "version": config["version"]}, 200)
	ag := tc.call("GET", "/admin/home-display", admin, nil, 200)
	if ag["version"] != float64(1) {
		t.Fatal("hall settings version leaked", ag)
	}
	other := resourceTestSession(t, a, "merchant")
	second := tc.call("POST", "/merchant/contents", other, gin.H{"kind": "vendor", "hall_id": cable.ID, "data": data}, 201)
	inherited := tc.call("POST", "/merchant/contents", other, gin.H{"kind": "product", "hall_id": 1, "data": Payload{Name: "电缆产品", CategoryID: cableCategory.ID}}, 201)
	if inherited["hall_id"] != float64(cable.ID) {
		t.Fatal("client overrode parent hall", inherited)
	}
	for _, item := range []map[string]any{second, inherited} {
		tc.call("POST", "/merchant/contents/"+number(item["id"])+"/submit", other, gin.H{"revision_id": item["revision_id"]}, 200)
		tc.call("POST", "/admin/revisions/"+number(item["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	}
	if tc.call("GET", "/public/products?hall=cable", "", nil, 200)["total"] != float64(1) {
		t.Fatal("second hall product missing")
	}
	if tc.call("GET", "/public/products", "", nil, 200)["total"] != float64(1) {
		t.Fatal("default hall leaked second hall")
	}
	tc.call("GET", "/public/products/"+number(inherited["id"]), "", nil, 200)
	a.DB.Model(&cable).Update("status", "preparing")
	tc.call("GET", "/public/products/"+number(inherited["id"]), "", nil, 404)
	tc.call("PUT", "/admin/contents/"+number(second["id"])+"/featured", admin, gin.H{"featured": true}, 200)
	prepared := tc.call("GET", "/admin/home-display?hall=cable", admin, nil, 200)
	if prepared["selected_vendor_count"] != float64(1) {
		t.Fatal("preparing hall cannot stage curated selection", prepared)
	}
	if tc.call("GET", "/public/vendors?hall=cable", "", nil, 200)["total"] != float64(0) {
		t.Fatal("staging leaked into public hall")
	}
}

func TestHallV2ScalePaginationAndRecommendations(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	const vendorCount = 2000
	const perVendor = 12
	region := Region{Name: "规模测试地区", Enabled: true}
	if e := a.DB.Create(&region).Error; e != nil {
		t.Fatal(e)
	}
	vendors := make([]Content, vendorCount)
	for i := range vendors {
		vendors[i] = Content{Kind: "vendor", HallID: 1, Visible: true, Featured: i < 10, CreatedAt: time.Now().Add(time.Duration(i) * time.Second)}
	}
	if e := a.DB.CreateInBatches(vendors, 200).Error; e != nil {
		t.Fatal(e)
	}
	products := make([]Content, 0, vendorCount*perVendor)
	for _, v := range vendors {
		for j := 0; j < perVendor; j++ {
			products = append(products, Content{Kind: "product", HallID: 1, VendorID: v.ID, Visible: true, Featured: true})
		}
	}
	if e := a.DB.CreateInBatches(products, 200).Error; e != nil {
		t.Fatal(e)
	}
	revisions := make([]Revision, 0, len(vendors)+len(products))
	now := time.Now().UTC()
	for i, c := range append(vendors, products...) {
		category := uint64(i%2 + 1)
		payload, _ := json.Marshal(Payload{Name: fmt.Sprintf("规模内容%06d", i), MainProducts: "测试设备", RegionID: region.ID, RegionName: region.Name, CategoryID: category, Parameters: "型号：SCALE-V2"})
		revisions = append(revisions, Revision{ContentID: c.ID, Status: "approved", Name: fmt.Sprintf("规模内容%06d", i), CategoryID: category, RegionID: region.ID, Payload: string(payload), ReviewedAt: &now})
	}
	if e := a.DB.CreateInBatches(revisions, 200).Error; e != nil {
		t.Fatal(e)
	}
	if e := a.DB.Exec("UPDATE contents c JOIN revisions r ON r.content_id=c.id SET c.published_revision_id=r.id").Error; e != nil {
		t.Fatal(e)
	}
	for _, spec := range []struct {
		endpoint string
		total    int
		size     int
	}{{"vendors", vendorCount, 20}, {"products", vendorCount * perVendor, 24}} {
		path := fmt.Sprintf("/public/%s?page_size=%d", spec.endpoint, spec.size)
		one := tc.call("GET", path, "", nil, 200)
		two := tc.call("GET", path+"&page=2", "", nil, 200)
		if one["total"] != float64(spec.total) || len(one["items"].([]any)) != spec.size {
			t.Fatal("page count", spec.endpoint, one["total"])
		}
		if number(one["items"].([]any)[0].(map[string]any)["id"]) == number(two["items"].([]any)[0].(map[string]any)["id"]) {
			t.Fatal("repeated pages")
		}
		tc.call("GET", path+"&sort=latest&region_id="+fmt.Sprint(region.ID), "", nil, 200)
	}
	home := tc.call("GET", "/public/home", "", nil, 200)
	if len(home["vendors"].([]any)) != 8 || len(home["products"].([]any)) != 12 {
		t.Fatal("home limits")
	}
	manufacturers := map[float64]bool{}
	categories := map[float64]bool{}
	for _, v := range home["products"].([]any) {
		p := v.(map[string]any)
		manufacturers[p["vendor_id"].(float64)] = true
		categories[p["data"].(map[string]any)["category_id"].(float64)] = true
		if p["vendor"] == nil {
			t.Fatal("home vendor summary missing")
		}
	}
	if len(manufacturers) < 6 || len(categories) != 2 {
		t.Fatal("insufficient diversity", len(manufacturers), len(categories))
	}
	// Half the manufacturers move to an isolated second region. Combined counts
	// must reflect manufacturer location and descendant product categories.
	secondRegion := Region{Name: "规模联合筛选地区", Enabled: true}
	a.DB.Create(&secondRegion)
	for i, v := range vendors {
		if i%2 == 0 {
			if e := a.DB.Model(&Revision{}).Where("content_id=?", v.ID).Updates(map[string]any{"region_id": secondRegion.ID, "payload": gorm.Expr("JSON_SET(payload,'$.region_id',?,'$.region_name',?)", secondRegion.ID, secondRegion.Name)}).Error; e != nil {
				t.Fatal(e)
			}
		}
	}
	for _, spec := range []struct {
		kind        string
		count, size int
	}{{"vendors", 1000, 20}, {"products", 6000, 24}} {
		path := fmt.Sprintf("/public/%s?q=SCALE-V2&category_id=1&region_id=%d", spec.kind, secondRegion.ID)
		seen := map[float64]bool{}
		for page := 1; page <= 3; page++ {
			result := tc.call("GET", path+fmt.Sprintf("&page=%d", page), "", nil, 200)
			if result["total"] != float64(spec.count) || len(result["items"].([]any)) != spec.size {
				t.Fatal("scale AND count", spec.kind, result["total"])
			}
			for _, raw := range result["items"].([]any) {
				id := raw.(map[string]any)["id"].(float64)
				if seen[id] {
					t.Fatal("scale repeated pages")
				}
				seen[id] = true
			}
		}
	}
	var plans []map[string]interface{}
	if e := a.DB.Raw("EXPLAIN SELECT id FROM contents WHERE hall_id=1 AND kind='vendor' AND visible=1 ORDER BY featured DESC,sort_order,id DESC LIMIT 20").Scan(&plans).Error; e != nil {
		t.Fatal(e)
	}
	if e := a.DB.Raw("EXPLAIN SELECT id FROM contents WHERE hall_id=1 AND vendor_id=? AND kind='product' AND visible=1 ORDER BY featured DESC,sort_order,id DESC LIMIT 3", vendors[0].ID).Scan(&plans).Error; e != nil {
		t.Fatal(e)
	}
	t.Logf("2000 manufacturers / 24000 products; page sizes 20/24; homepage 8/12; query plan: %+v", plans)
}

func TestHomeProductPresentationSpreadsSuppliers(t *testing.T) {
	items := []View{}
	for i := uint64(1); i <= 8; i++ {
		items = append(items, View{Content: Content{ID: i, VendorID: 1}, Data: Payload{CategoryID: 1}})
	}
	items = append(items, View{Content: Content{ID: 9, VendorID: 2}, Data: Payload{CategoryID: 2}}, View{Content: Content{ID: 10, VendorID: 3}, Data: Payload{CategoryID: 3}})
	spread := spreadHomeProducts(items)
	if spread[0].ID != 1 || spread[1].VendorID != 2 || spread[2].VendorID != 3 || spread[3].ID != 2 {
		t.Fatal("suppliers were not spread in stable order", spread)
	}
}
