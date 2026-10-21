package main

import (
	"bytes"
	"fmt"
	"github.com/gin-gonic/gin"
	"image"
	"image/png"
	"strings"
	"testing"
)

func TestOnboardingFieldIssues(t *testing.T) {
	p := Payload{Name: "测试", RegionName: "河北", MainProducts: "机械", Phone: "0319-1234567", CompanyProfile: &CompanyProfile{Website: "www.example.com", Contacts: []CompanyContact{{Area: "河北"}}}}
	issues := payloadFieldIssues("vendor", p)
	found := map[string]bool{}
	for _, issue := range issues {
		found[issue.Field] = true
	}
	if !found["company_profile.website"] || !found["company_profile.contacts.0.phone"] {
		t.Fatal(issues)
	}
	p.CompanyProfile = nil
	p.Name = strings.Repeat("😀", 200)
	p.Address = strings.Repeat("中", 166)
	if len(payloadFieldIssues("vendor", p)) != 0 {
		t.Fatal("valid unicode boundary")
	}
	p.Name += "😀"
	p.Address += "中"
	if len(payloadFieldIssues("vendor", p)) != 2 {
		t.Fatal("length boundary")
	}
}

func TestCategoryMigrationRestartsPartialDDL(t *testing.T) {
	a := importTestApp(t)
	if e := a.DB.Exec("DELETE FROM schema_migrations WHERE version=6").Error; e != nil {
		t.Fatal(e)
	}
	if e := a.DB.Exec("DROP TABLE category_aliases").Error; e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		if e := migrate(a.DB); e != nil {
			t.Fatal(e)
		}
	}
	if !a.DB.Migrator().HasTable(&CategoryAlias{}) || !a.DB.Migrator().HasTable(&CategorySuggestion{}) {
		t.Fatal("partial migration not recovered")
	}
}
func TestCompanyCategorySuggestionsAndAliases(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	owner := resourceTestSession(t, a, "merchant")
	other := resourceTestSession(t, a, "merchant")
	admin := resourceTestSession(t, a, "admin")
	cat := tc.call("POST", "/merchant/categories", owner, gin.H{"name": "新型翻转犁", "parent_id": 1}, 201)
	cid := number(cat["id"])
	again := tc.call("POST", "/merchant/categories", owner, gin.H{"name": "新型翻转犁", "parent_id": 1}, 201)
	if again["id"] != cat["id"] {
		t.Fatal("duplicate suggestion")
	}
	contains := func(token, path string) bool {
		response := tc.call("GET", path, token, nil, 200)
		for _, item := range response["categories"].([]any) {
			if number(item.(map[string]any)["id"]) == cid {
				return true
			}
		}
		return false
	}
	if !contains(owner, "/merchant/taxonomy") || contains(other, "/merchant/taxonomy") || contains("", "/public/taxonomy") {
		t.Fatal("category scope")
	}
	v := tc.fixtureCall("POST", "/merchant/contents", owner, gin.H{"kind": "vendor", "data": Payload{Name: "分类企业", RegionName: "河北", MainProducts: "机械", Phone: "0319-1234567", CategoryID: uint64(cat["id"].(float64))}}, 201)
	tc.fixtureCall("POST", "/merchant/contents", other, gin.H{"kind": "vendor", "data": Payload{Name: "其他企业", RegionName: "河北", MainProducts: "机械", Phone: "13900001111", CategoryID: uint64(cat["id"].(float64))}}, 400)
	approve := func(item map[string]any) {
		tc.call("POST", "/merchant/contents/"+number(item["id"])+"/submit", owner, gin.H{"revision_id": item["revision_id"]}, 200)
		tc.call("POST", "/admin/revisions/"+number(item["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	}
	approve(v)
	if tc.call("GET", "/public/vendors?category_id="+cid, "", nil, 200)["total"] != float64(1) {
		t.Fatal("vendor classification ignored")
	}
	product := tc.fixtureCall("POST", "/merchant/contents", owner, gin.H{"kind": "product", "data": Payload{Name: "分类展品", CategoryID: uint64(cat["id"].(float64))}}, 201)
	approve(product)
	tc.call("POST", "/admin/category-suggestions/"+cid+"/normalize", admin, gin.H{"action": "merge", "target_id": 2}, 200)
	for _, query := range []string{cid, "2"} {
		if tc.call("GET", "/public/products?category_id="+query, "", nil, 200)["total"] != float64(1) {
			t.Fatal("merged filter", query)
		}
	}
	show := tc.call("GET", "/public/showrooms/v-"+number(v["id"]), "", nil, 200)
	found := false
	for _, row := range show["categories"].([]any) {
		c := row.(map[string]any)
		if c["id"] == product["data"].(map[string]any)["product_category_id"] && c["product_count"] == float64(1) {
			found = true
		}
	}
	if !found {
		t.Fatal("merged counts", show)
	}
	var raw Revision
	a.DB.First(&raw, product["revision_id"])
	if !strings.Contains(raw.Payload, fmt.Sprintf(`"category_id":%s`, cid)) {
		t.Fatal("history rewritten")
	}
	official := tc.call("POST", "/merchant/categories", owner, gin.H{"name": "新分类待规范", "parent_id": 1}, 201)
	tc.call("POST", "/admin/category-suggestions/"+number(official["id"])+"/normalize", admin, gin.H{"action": "approve", "name": "规范分类", "parent_id": 1}, 200)
	if contains("", "/public/taxonomy") {
		t.Fatal("merged category exposed")
	}
	list := tc.call("GET", "/public/taxonomy", "", nil, 200)
	ok := false
	for _, r := range list["categories"].([]any) {
		if r.(map[string]any)["id"] == official["id"] {
			ok = true
		}
	}
	if !ok {
		t.Fatal("approved not global")
	}
}
func TestAdminBrandImagesOutsideGalleryQuota(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	admin := resourceTestSession(t, a, "admin")
	item := tc.fixtureCall("POST", "/admin/vendors", admin, gin.H{"login_name": "品牌图片账号", "data": Payload{Name: "品牌图片企业", RegionName: "河北", MainProducts: "机械", Phone: "0319-1234567"}}, 201)["item"].(map[string]any)
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 32, 32)))
	ids := []uint64{}
	upload := func() uint64 {
		r := resourceUpload(t, a, admin, "/admin/contents/"+number(item["id"])+"/media", "brand.png", b.Bytes(), 201)
		return uint64(r["id"].(float64))
	}
	for i := 0; i < 12; i++ {
		ids = append(ids, upload())
	}
	logo, cover := upload(), upload()
	p := Payload{Name: "品牌图片企业", RegionName: "河北", MainProducts: "机械", Phone: "0319-1234567", MediaIDs: ids, CompanyProfile: &CompanyProfile{LogoMediaID: logo, CoverMediaID: cover}}
	draft := tc.fixtureCall("PUT", "/admin/contents/"+number(item["id"])+"/draft", admin, gin.H{"revision_id": item["revision_id"], "data": p}, 200)
	tc.call("GET", fmt.Sprintf("/media/%d", logo), "", nil, 404)
	tc.call("POST", "/admin/contents/"+number(item["id"])+"/submit", admin, gin.H{"revision_id": draft["revision_id"]}, 200)
	tc.call("POST", "/admin/revisions/"+number(draft["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	p.CompanyProfile.LogoMediaID = upload()
	next := tc.fixtureCall("PUT", "/admin/contents/"+number(item["id"])+"/draft", admin, gin.H{"revision_id": draft["revision_id"], "data": p}, 200)
	tc.call("POST", "/admin/contents/"+number(item["id"])+"/submit", admin, gin.H{"revision_id": next["revision_id"]}, 200)
	pub := tc.call("GET", "/public/showrooms/v-"+number(item["id"]), "", nil, 200)
	profile := pub["item"].(map[string]any)["data"].(map[string]any)["company_profile"].(map[string]any)
	if profile["logo_media_id"] != float64(logo) || profile["cover_media_id"] != float64(cover) {
		t.Fatal("draft brand image leaked")
	}
}
