package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"testing"
)

func codeValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func TestAdministrativeSnapshotResolution(t *testing.T) {
	for _, test := range []struct {
		name                     string
		payload                  Payload
		legacy                   Region
		province, city, district string
	}{
		{"county", Payload{RegionCode: "130528"}, Region{}, "13", "1305", "130528"},
		{"city", Payload{RegionCode: "1305"}, Region{}, "13", "1305", ""},
		{"municipality", Payload{RegionCode: "110108"}, Region{}, "11", "1101", "110108"},
		{"direct county city", Payload{RegionCode: "429004"}, Region{}, "42", "429004", ""},
		{"exact historical name", Payload{RegionName: "河北省邢台市宁晋县"}, Region{}, "13", "1305", "130528"},
		{"ambiguous municipality name", Payload{RegionName: "北京市"}, Region{}, "", "", ""},
		{"no fuzzy inference", Payload{RegionName: "宁晋附近"}, Region{}, "", "", ""},
		{"snapshot not mutable region", Payload{RegionName: "旧地区"}, Region{Name: "河北省 邢台市 宁晋县", AdministrativeCode: ptrCode("130528")}, "", "", ""},
		{"compatible legacy mapping", Payload{RegionName: "北京市"}, Region{AdministrativeCode: ptrCode("1101")}, "11", "1101", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := administrativeSnapshot(test.payload, test.legacy)
			if codeValue(s.ProvinceCode) != test.province || codeValue(s.CityCode) != test.city || codeValue(s.DistrictCode) != test.district {
				t.Fatalf("unexpected snapshot: %#v", s)
			}
		})
	}
}
func ptrCode(value string) *string { return &value }

// Synthetic manufacturers are created exclusively in importTestApp's disposable DB.
func industryFixture(t *testing.T, a *App, p Payload, hall uint64) Content {
	t.Helper()
	if p.Name == "" {
		p.Name = "产业导航测试企业（仅测试）"
	}
	if p.RegionCode != "" {
		loadAdministrativeRegions()
		p.RegionName = administrativeByCode[p.RegionCode].FullName
	}
	c := Content{Kind: "vendor", HallID: hall, Visible: true}
	if e := a.DB.Create(&c).Error; e != nil {
		t.Fatal(e)
	}
	if e := makeDraft(a.DB, &c, p); e != nil {
		t.Fatal(e)
	}
	if e := a.DB.Model(&Revision{}).Where("id=?", c.DraftRevisionID).Update("status", "approved").Error; e != nil {
		t.Fatal(e)
	}
	c.PublishedRevisionID = c.DraftRevisionID
	if e := a.DB.Save(&c).Error; e != nil {
		t.Fatal(e)
	}
	return c
}
func mapCount(t *testing.T, result map[string]any, code string) float64 {
	t.Helper()
	var sum float64
	count := float64(-1)
	for _, raw := range result["items"].([]any) {
		item := raw.(map[string]any)
		sum += item["vendor_count"].(float64)
		if item["code"] == code {
			count = item["vendor_count"].(float64)
		}
	}
	if len(result["items"].([]any)) > 0 && sum+result["unassigned_count"].(float64) != result["total"].(float64) {
		t.Fatal("count conservation failed", result)
	}
	return count
}

func TestIndustryMapPublicStatisticsFiltersAndHierarchy(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	county := industryFixture(t, a, Payload{RegionCode: "130528", CategoryID: 1, Name: "隔离测试农机企业"}, 1)
	industryFixture(t, a, Payload{RegionCode: "1305", CategoryID: 2}, 1)
	industryFixture(t, a, Payload{RegionCode: "110108"}, 1)
	industryFixture(t, a, Payload{RegionCode: "429004"}, 1)
	industryFixture(t, a, Payload{RegionName: "未核实的历史地区（测试）"}, 1)
	offline := industryFixture(t, a, Payload{RegionCode: "130528"}, 1)
	a.DB.Model(&offline).Update("visible", false)
	for _, status := range []string{"draft", "pending", "rejected"} {
		v := industryFixture(t, a, Payload{RegionCode: "130528"}, 1)
		a.DB.Model(&Revision{}).Where("id=?", v.PublishedRevisionID).Update("status", status)
	}
	// Existing product-aware keyword/category semantics must match the browse page.
	v2Published(t, a, "product", county.ID, 1, 2, 0, false, 0, "产业测试专用关键词")
	root := tc.call("GET", "/public/industry-map/regions", "", nil, 200)
	if root["total"] != float64(5) || root["unassigned_count"] != float64(1) || mapCount(t, root, "13") != 2 {
		t.Fatal(root)
	}
	province := tc.call("GET", "/public/industry-map/regions?region_code=13", "", nil, 200)
	if mapCount(t, province, "1305") != 2 {
		t.Fatal(province)
	}
	city := tc.call("GET", "/public/industry-map/regions?region_code=1305", "", nil, 200)
	if mapCount(t, city, "130528") != 1 || city["unassigned_count"] != float64(1) {
		t.Fatal(city)
	}
	leaf := tc.call("GET", "/public/industry-map/regions?region_code=429004", "", nil, 200)
	if leaf["total"] != float64(1) || len(leaf["items"].([]any)) != 0 || len(leaf["trail"].([]any)) != 2 {
		t.Fatal(leaf)
	}
	beijing := tc.call("GET", "/public/industry-map/regions?region_code=1101", "", nil, 200)
	if mapCount(t, beijing, "110108") != 1 {
		t.Fatal(beijing)
	}
	for _, query := range []string{"?region_code=1305&unassigned=1", "?unassigned=1", "?q=产业测试专用关键词", "?category_id=1", "?region_code=130528"} {
		result := tc.call("GET", "/public/industry-map/vendors"+query, "", nil, 200)
		if result["total"] != float64(1) {
			t.Fatal(query, result)
		}
	}
	for _, query := range []string{"?category_id=2", "?q=产业测试专用关键词", "?category_id=999999", "?region_code=130528"} {
		stats := tc.call("GET", "/public/industry-map/regions"+query, "", nil, 200)
		list := tc.call("GET", "/public/industry-map/vendors"+query, "", nil, 200)
		if stats["total"] != list["total"] {
			t.Fatal("filters diverged", query, stats, list)
		}
	}
	for _, query := range []string{"?region_code=999999", "?category_id=abc", "?page=0", "?page_size=101"} {
		tc.call("GET", "/public/industry-map/vendors"+query, "", nil, 400)
	}
	tc.call("GET", "/public/industry-map/vendors?unassigned=bad", "", nil, 400)
	for i := 0; i < 21; i++ {
		industryFixture(t, a, Payload{RegionCode: "130528", Name: fmt.Sprintf("分页测试企业%02d（仅测试）", i)}, 1)
	}
	first := tc.call("GET", "/public/industry-map/vendors?region_code=130528&page_size=99", "", nil, 200)
	second := tc.call("GET", "/public/industry-map/vendors?region_code=130528&page=2", "", nil, 200)
	if first["page_size"] != float64(20) || len(first["items"].([]any)) != 20 || len(second["items"].([]any)) != 2 {
		t.Fatal(first, second)
	}
	ids := map[any]bool{}
	for _, page := range []map[string]any{first, second} {
		for _, raw := range page["items"].([]any) {
			id := raw.(map[string]any)["id"]
			if ids[id] {
				t.Fatal("unstable pagination")
			}
			ids[id] = true
		}
	}
	var hall Hall
	a.DB.First(&hall, 2)
	a.DB.Model(&hall).Update("status", "open")
	industryFixture(t, a, Payload{RegionCode: "130528"}, 2)
	isolated := tc.call("GET", "/public/industry-map/regions?hall=cable", "", nil, 200)
	if isolated["total"] != float64(1) {
		t.Fatal(isolated)
	}
	a.DB.Model(&hall).Update("status", "preparing")
	if tc.call("GET", "/public/industry-map/vendors?hall=cable", "", nil, 200)["total"] != float64(0) {
		t.Fatal("preparing hall leaked")
	}
	// Record actual optimizer decisions on a disposable database.
	var explain []map[string]any
	if e := a.DB.Raw("EXPLAIN SELECT r.city_code,COUNT(DISTINCT c.id) FROM revisions r JOIN contents c ON c.published_revision_id=r.id WHERE r.province_code='13' AND r.status='approved' AND c.kind='vendor' AND c.visible=1 AND c.hall_id=1 GROUP BY r.city_code").Scan(&explain).Error; e != nil {
		t.Fatal(e)
	}
	data, _ := json.Marshal(explain)
	t.Log("Industry map EXPLAIN:", string(data))
}

func TestIndustryMapPublishedSnapshotAndAuthorization(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	merchant := resourceTestSession(t, a, "merchant")
	other := resourceTestSession(t, a, "merchant")
	admin := resourceTestSession(t, a, "admin")
	p := Payload{Name: "修订版隔离测试厂商", MainProducts: "测试机械", RegionCode: "130528", Phone: "0319-1234567"}
	v := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": p}, 201)
	vendorID := number(v["id"])
	publish := func(v map[string]any) {
		tc.call("POST", "/merchant/contents/"+vendorID+"/submit", merchant, gin.H{"revision_id": v["revision_id"]}, 200)
		tc.call("POST", "/admin/revisions/"+number(v["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	}
	publish(v)
	p.RegionCode = "110108"
	v = tc.fixtureCall("PUT", "/merchant/contents/"+vendorID+"/draft", merchant, gin.H{"revision_id": v["revision_id"], "data": p}, 200)
	tc.fixtureCall("PUT", "/merchant/contents/"+vendorID+"/draft", other, gin.H{"revision_id": v["revision_id"], "data": p}, 403)
	if mapCount(t, tc.call("GET", "/public/industry-map/regions", "", nil, 200), "13") != 1 {
		t.Fatal("draft changed public location")
	}
	tc.call("POST", "/merchant/contents/"+vendorID+"/submit", merchant, gin.H{"revision_id": v["revision_id"]}, 200)
	tc.fixtureCall("PUT", "/merchant/contents/"+vendorID+"/draft", merchant, gin.H{"revision_id": v["revision_id"], "data": p}, 409)
	tc.call("POST", "/admin/revisions/"+number(v["revision_id"])+"/review", admin, gin.H{"decision": "reject", "note": "仅测试驳回"}, 200)
	if mapCount(t, tc.call("GET", "/public/industry-map/regions", "", nil, 200), "13") != 1 {
		t.Fatal("rejection changed public location")
	}
	// Legacy client sends region_id/name but omits region_code.
	p.RegionCode = ""
	p.RegionID = uint64(v["data"].(map[string]any)["region_id"].(float64))
	p.RegionName = "北京市 海淀区"
	v = tc.fixtureCall("PUT", "/merchant/contents/"+vendorID+"/draft", merchant, gin.H{"revision_id": v["revision_id"], "data": p}, 200)
	publish(v)
	if mapCount(t, tc.call("GET", "/public/industry-map/regions", "", nil, 200), "11") != 1 {
		t.Fatal("legacy client lost location")
	}
	tc.call("POST", "/admin/contents/"+vendorID+"/offline", admin, gin.H{}, 200)
	if tc.call("GET", "/public/industry-map/regions", "", nil, 200)["total"] != float64(0) {
		t.Fatal("offline vendor leaked")
	}
}

func TestIndustryMapMigrationBackfillAndRecovery(t *testing.T) {
	a := importTestApp(t)
	v := industryFixture(t, a, Payload{RegionCode: "130528"}, 1)
	if e := a.DB.Model(&Revision{}).Where("id=?", v.PublishedRevisionID).Updates(map[string]any{"province_code": nil, "city_code": nil, "district_code": nil}).Error; e != nil {
		t.Fatal(e)
	}
	if e := a.DB.Exec("DELETE FROM schema_migrations WHERE version=16").Error; e != nil {
		t.Fatal(e)
	}
	var original Revision
	a.DB.First(&original, v.PublishedRevisionID)
	a.DB.Model(&Revision{}).Where("id=?", original.ID).UpdateColumn("payload", "broken-json")
	if e := migrate(a.DB); e == nil {
		t.Fatal("failed backfill marked complete")
	}
	var versions int64
	a.DB.Table("schema_migrations").Where("version=16").Count(&versions)
	if versions != 0 {
		t.Fatal("failed migration recorded")
	}
	a.DB.Model(&Revision{}).Where("id=?", original.ID).UpdateColumn("payload", original.Payload)
	if e := migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	if e := migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	var restored Revision
	a.DB.First(&restored, original.ID)
	if restored.Payload != original.Payload || !restored.UpdatedAt.Equal(original.UpdatedAt) || codeValue(restored.DistrictCode) != "130528" {
		t.Fatal(restored)
	}
	if !a.DB.Migrator().HasIndex(&Revision{}, "idx_revision_administrative_public") {
		t.Fatal("administrative index missing")
	}
}

func TestIndustryMapGuardClassification(t *testing.T) {
	for _, path := range []string{"/regions", "/api/v1/public/industry-map/regions", "/api/v1/public/industry-map/vendors"} {
		if guardKind(path) != "list" {
			t.Fatal(path)
		}
	}
}
