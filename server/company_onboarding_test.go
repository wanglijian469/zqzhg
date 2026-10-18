package main

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"sync"
	"testing"
)

func TestAdministrativeDirectoryHierarchy(t *testing.T) {
	loadAdministrativeRegions()
	if len(administrativeChildren[""]) != 31 {
		t.Fatal("province catalogue incomplete")
	}
	n := administrativeByCode["130528"]
	if n.FullName != "河北省 邢台市 宁晋县" || n.ParentCode != "1305" {
		t.Fatal(n)
	}
	if administrativeByCode["1101"].Name != "北京市" || administrativeByCode["110108"].FullName != "北京市 海淀区" {
		t.Fatal("municipality adaptation")
	}
	if administrativeByCode["429004"].ParentCode != "42" || administrativeByCode["429004"].Level != 2 || administrativeByCode["429004"].HasChildren {
		t.Fatal("direct county city adaptation")
	}
}
func TestCompanyOnboardingRegionAndReview(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	admin := resourceTestSession(t, a, "admin")
	registration := tc.call("POST", "/auth/merchant/register", "", gin.H{"login_name": "company-onboarding-test", "password": "Company-Onboarding-2026!"}, 201)
	merchant := registration["token"].(string)
	if tc.call("GET", "/merchant/contents?kind=vendor", merchant, nil, 200)["total"] != float64(0) {
		t.Fatal("registration created an empty vendor")
	}
	legacy := Region{ID: 18, Name: "河北省 邢台市 宁晋县", Enabled: true}
	a.DB.Create(&legacy)
	root := tc.call("GET", "/public/administrative-regions", "", nil, 200)
	if len(root["items"].([]any)) != 31 {
		t.Fatal(root)
	}
	tc.call("GET", "/public/administrative-regions?parent_code=invalid", "", nil, 400)
	tc.call("GET", "/public/administrative-regions?name=河北省邢台市宁晋县", "", nil, 200)
	payload := Payload{Name: "工作台入驻测试企业", RegionCode: "130528", MainProducts: "测试机械", Phone: "0319-1234567"}
	bad := payload
	bad.RegionCode = "999999"
	tc.call("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": bad}, 400)
	bad.RegionCode = "13"
	tc.call("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": bad}, 400)
	v := tc.call("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": payload}, 201)
	d := v["data"].(map[string]any)
	if d["region_id"] != float64(18) || d["region_name"] != legacy.Name {
		t.Fatal(d)
	}
	payload.Description = "真实主营介绍"
	v = tc.call("PUT", "/merchant/contents/"+number(v["id"])+"/draft", merchant, gin.H{"revision_id": v["revision_id"], "data": payload}, 200)
	tc.call("POST", "/merchant/contents/"+number(v["id"])+"/submit", merchant, gin.H{"revision_id": v["revision_id"]}, 200)
	tc.call("POST", "/admin/revisions/"+number(v["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	if tc.call("GET", "/public/vendors?region_id=18", "", nil, 200)["total"] != float64(1) {
		t.Fatal("region filtering lost registered vendor")
	}
	tc.call("POST", "/merchant/contents", merchant, gin.H{"kind": "product", "data": Payload{Name: "既有V22名称最小提交"}}, 201)
	other := resourceTestSession(t, a, "merchant")
	payload.Name = "市级资料测试"
	payload.RegionCode = "1305"
	city := tc.call("POST", "/merchant/contents", other, gin.H{"kind": "vendor", "data": payload}, 201)
	if city["data"].(map[string]any)["region_name"] != "河北省 邢台市" {
		t.Fatal(city)
	}
	old := resourceTestSession(t, a, "merchant")
	payload.RegionCode = ""
	payload.RegionID = 18
	payload.RegionName = legacy.Name
	tc.call("POST", "/merchant/contents", old, gin.H{"kind": "vendor", "data": payload}, 201)
}
func TestCompanyRegionConcurrentResolutionAndMigrationRetry(t *testing.T) {
	a := importTestApp(t)
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	ids := make(chan uint64, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p := Payload{RegionCode: "110108"}
			e := a.DB.Transaction(func(tx *gorm.DB) error { return resolveAdministrativeRegion(tx, &p) })
			errs <- e
			ids <- p.RegionID
		}()
	}
	wg.Wait()
	close(errs)
	close(ids)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var first uint64
	for id := range ids {
		if first == 0 {
			first = id
		}
		if id == 0 || id != first {
			t.Fatal("duplicate or missing region", id, first)
		}
	}
	if e := a.DB.Exec("DELETE FROM schema_migrations WHERE version=13").Error; e != nil {
		t.Fatal(e)
	}
	if e := a.DB.Exec("DROP INDEX idx_regions_administrative_code ON regions").Error; e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		if e := migrate(a.DB); e != nil {
			t.Fatal(e)
		}
	}
	var region Region
	if e := a.DB.First(&region, first).Error; e != nil || region.AdministrativeCode == nil || *region.AdministrativeCode != "110108" {
		t.Fatal("migration lost region", e)
	}
}
