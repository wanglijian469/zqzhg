package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

func vendorCategoryTestCompany(t *testing.T, a *App, tc testClient, token, name string) map[string]any {
	t.Helper()
	return tc.fixtureCall("POST", "/merchant/contents", token, gin.H{"kind": "vendor", "data": Payload{Name: name, MainProducts: "液压机械", RegionName: "河北省 邢台市 宁晋县", Phone: "0319-1234567"}}, 201)
}
func vendorCategoryTestRoot(t *testing.T, a *App) Category {
	t.Helper()
	var root Category
	if err := a.DB.Where("category_kind='vendor' AND parent_id=0 AND enabled=1 AND code LIKE 'ciame.2026.vendor.%'").First(&root).Error; err != nil {
		t.Fatal(err)
	}
	return root
}
func TestVendorProductCategoryOwnershipVersionsAndSearch(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	owner := resourceTestSession(t, a, "merchant")
	other := resourceTestSession(t, a, "merchant")
	admin := resourceTestSession(t, a, "admin")
	v := vendorCategoryTestCompany(t, a, tc, owner, "独立产品分类企业甲")
	v2 := vendorCategoryTestCompany(t, a, tc, other, "独立产品分类企业乙")
	tc.call("POST", "/merchant/service-acceptances", owner, gin.H{"agreements": serviceTestRefs(t, a, "terms", "privacy", "showroom-agreement")}, 200)
	root := vendorCategoryTestRoot(t, a)
	create := func(token, name string) map[string]any {
		return tc.call("POST", "/merchant/product-categories", token, gin.H{"name": name, "industry_category_id": root.ID}, 201)
	}
	cat := create(owner, "企业专用回转马达")
	foreign := create(other, "企业专用回转马达")
	if cat["id"] == foreign["id"] {
		t.Fatal("categories shared between companies")
	}
	list := tc.call("GET", "/merchant/product-categories", owner, nil, 200)
	if len(list["items"].([]any)) != 1 {
		t.Fatal("foreign category leaked", list)
	}
	tc.call("POST", "/merchant/product-categories", owner, gin.H{"name": "企业专用回转马达", "industry_category_id": root.ID}, 409)
	tc.call("PUT", "/merchant/product-categories/"+number(foreign["id"]), owner, gin.H{"name": "越权", "industry_category_id": root.ID}, 404)
	tc.call("DELETE", "/merchant/product-categories/"+number(foreign["id"]), owner, nil, 404)
	tc.call("POST", "/merchant/contents", owner, gin.H{"kind": "product", "data": gin.H{"name": "越权绑定", "vendor_product_category_id": foreign["id"]}}, 400)
	tc.call("POST", "/merchant/contents", owner, gin.H{"kind": "product", "data": gin.H{"name": "错误兼容分类", "vendor_product_category_id": cat["id"], "product_category_id": root.ID}}, 400)
	imageID := watermarkUpload(t, a, owner, "/merchant/media")
	tc.call("POST", "/merchant/contents", owner, gin.H{"kind": "product", "data": gin.H{"name": "测试回转马达", "vendor_product_category_id": cat["id"], "media_ids": []uint64{imageID}}}, 201)
	products := tc.call("GET", "/merchant/contents?kind=product", owner, nil, 200)["items"].([]any)
	product := products[0].(map[string]any)
	for _, item := range []map[string]any{v, product} {
		tc.call("POST", "/merchant/contents/"+number(item["id"])+"/submit", owner, gin.H{"revision_id": item["revision_id"]}, 200)
		tc.call("POST", "/admin/revisions/"+number(item["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	}
	query := "/public/products?vendor_id=" + number(v["id"]) + "&vendor_product_category_id=" + number(cat["id"])
	if tc.call("GET", query, "", nil, 200)["total"] != float64(1) {
		t.Fatal("own category filter failed")
	}
	if tc.call("GET", "/public/products?vendor_id="+number(v2["id"])+"&vendor_product_category_id="+number(cat["id"]), "", nil, 200)["total"] != float64(0) {
		t.Fatal("foreign scope matched")
	}
	if tc.call("GET", "/public/products?vendor_product_category_id="+number(cat["id"]), "", nil, 200)["total"] != float64(0) {
		t.Fatal("category query must require company")
	}
	if tc.call("GET", "/public/products?q=企业专用回转马达", "", nil, 200)["total"] != float64(1) {
		t.Fatal("category name not searchable")
	}
	platformRoot := vendorCategoryPlatformRoot(a.DB, root.ID)
	if tc.call("GET", fmt.Sprintf("/public/products?category_id=%d", platformRoot), "", nil, 200)["total"] != float64(1) {
		t.Fatal("stable industry root projection missing")
	}
	cat2 := create(owner, "备用分类")
	saved := tc.call("PUT", "/merchant/contents/"+number(product["id"])+"/draft", owner, gin.H{"revision_id": product["revision_id"], "data": gin.H{"name": "测试回转马达", "vendor_product_category_id": cat2["id"]}}, 200)
	if tc.call("GET", query, "", nil, 200)["total"] != float64(1) {
		t.Fatal("draft changed published category")
	}
	// A pre-upgrade client omits the new field while changing a description.
	kept := tc.call("PUT", "/merchant/contents/"+number(product["id"])+"/draft", owner, gin.H{"revision_id": saved["revision_id"], "data": gin.H{"name": "测试回转马达", "description": "旧客户端其他编辑"}}, 200)
	if kept["data"].(map[string]any)["vendor_product_category_id"] != cat2["id"] {
		t.Fatal("old client erased company category", kept)
	}
	tc.call("DELETE", "/merchant/product-categories/"+number(cat2["id"]), owner, nil, 409)
	tc.call("PUT", "/merchant/product-categories/"+number(cat["id"]), owner, gin.H{"name": "企业专用回转马达改名", "industry_category_id": root.ID, "enabled": false}, 200)
	tc.call("POST", "/merchant/contents", owner, gin.H{"kind": "product", "data": gin.H{"name": "停用新选", "vendor_product_category_id": cat["id"]}}, 400)
	if tc.call("GET", "/public/products?q=企业专用回转马达改名", "", nil, 200)["total"] != float64(1) {
		t.Fatal("rename or deactivation hid published products")
	}
	tc.call("GET", "/admin/vendors/"+number(v["id"])+"/product-categories", admin, nil, 200)
	tc.call("GET", "/admin/vendors/"+number(v["id"])+"/product-categories", owner, nil, 403)
}
func TestVendorProductCategoryConcurrentNames(t *testing.T) {
	a := importTestApp(t)
	router := a.router()
	tc := testClient{t, router}
	owner := resourceTestSession(t, a, "merchant")
	vendorCategoryTestCompany(t, a, tc, owner, "分类并发验收企业")
	root := vendorCategoryTestRoot(t, a)
	raw, _ := json.Marshal(gin.H{"name": "同一企业并发分类", "industry_category_id": root.ID})
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("POST", "/api/v1/merchant/product-categories", bytes.NewReader(raw))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+owner)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			codes <- w.Code
		}()
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for code := range codes {
		counts[code]++
	}
	if counts[201] != 1 || counts[409] != 1 {
		t.Fatal(counts)
	}
}
func TestVendorProductCategoryMigrationPreservesRevisionPayloads(t *testing.T) {
	a := importTestAppAtSchema(t, 30)
	var leaf Category
	if err := a.DB.Where("category_kind='product' AND parent_id>0 AND enabled=1").First(&leaf).Error; err != nil {
		t.Fatal(err)
	}
	v1 := v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, "迁移企业甲")
	v2 := v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, "迁移企业乙")
	for _, v := range []Content{v1, v2} {
		p := v2Published(t, a, "product", v.ID, 1, leaf.ID, 1, false, 0, "原产品")
		for _, state := range []string{"draft", "pending"} {
			rev := Revision{ContentID: p.ID, Status: state, Name: "原草稿", CategoryID: leaf.ID, Payload: fmt.Sprintf(`{"name":"原草稿","product_category_id":%d}`, leaf.ID)}
			if err := a.DB.Create(&rev).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	unknown := Category{Name: "历史企业专用系列", CategoryKind: "product", HallID: 1, Enabled: true}
	if err := a.DB.Create(&unknown).Error; err != nil {
		t.Fatal(err)
	}
	v2Published(t, a, "product", v1.ID, 1, unknown.ID, 1, false, 0, "待整理历史产品")
	var before []Revision
	a.DB.Order("id").Find(&before)
	if err := migrate(a.DB); err != nil {
		t.Fatal(err)
	}
	if err := migrateVendorProductCategories(a.DB); err != nil {
		t.Fatal(err)
	}
	var cats []VendorProductCategory
	a.DB.Find(&cats)
	if len(cats) != 3 || cats[0].VendorID == cats[1].VendorID {
		t.Fatal("migration shared or duplicated company categories", cats)
	}
	var n int64
	a.DB.Model(&RevisionVendorProductCategory{}).Count(&n)
	if n != 7 {
		t.Fatal("missing published/draft/pending bindings", n)
	}
	var after []Revision
	a.DB.Order("id").Find(&after)
	if len(before) != len(after) {
		t.Fatal("revision count changed")
	}
	for i, r := range before {
		if r.Payload != after[i].Payload {
			t.Fatal("migration rewrote audit snapshot")
		}
	}
	if err := a.decorateVendorCategories(cats); err != nil {
		t.Fatal(err)
	}
	for _, cat := range cats {
		if cat.Name == unknown.Name {
			if cat.VendorID != v1.ID || cat.IndustryCategoryID != 0 || !cat.NeedsIndustry {
				t.Fatal("unmapped historical category must retain its name and assignment", cat)
			}
			continue
		}
		if cat.Name != leaf.Name || cat.NameEN != leaf.NameEN || cat.IndustryCategoryID == 0 {
			t.Fatal("stable industry mapping missing", cat)
		}
	}
}

func TestVendorProductCategoryEncryptedBackupRestore(t *testing.T) {
	a := importTestApp(t)
	if _, err := mysqlTool("mysqldump"); err != nil {
		t.Fatal("real backup tools required", err)
	}
	t.Setenv("OPERATIONS_DIR", a.operations().dir)
	vendor := v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, "分类备份企业")
	product := v2Published(t, a, "product", vendor.ID, 1, 1, 1, false, 0, "分类备份产品")
	root := vendorCategoryTestRoot(t, a)
	cat := VendorProductCategory{VendorID: vendor.ID, HallID: 1, IndustryCategoryID: root.ID, Name: "独立备份分类", NameKey: vendorCategoryNameKey("独立备份分类"), Enabled: true}
	if err := a.DB.Create(&cat).Error; err != nil {
		t.Fatal(err)
	}
	binding := RevisionVendorProductCategory{product.PublishedRevisionID, cat.ID, vendor.ID}
	if err := a.DB.Create(&binding).Error; err != nil {
		t.Fatal(err)
	}
	if err := a.DB.Create(&VendorProductCategorySource{vendor.ID, 1, cat.ID}).Error; err != nil {
		t.Fatal(err)
	}
	backup, _, err := a.newJob("backup", backupParams{}, 1)
	if err != nil {
		t.Fatal(err)
	}
	done := waitOperation(t, a, backup.ID)
	if done.State != "succeeded" {
		t.Fatal("backup failed", done.Error)
	}
	var entry BackupEntry
	if err = json.Unmarshal(done.Result, &entry); err != nil {
		t.Fatal(err)
	}
	restore, _, err := a.newJob("restore", restoreParams{entry.ID}, 1)
	if err != nil {
		t.Fatal(err)
	}
	done = waitOperation(t, a, restore.ID)
	if done.State != "succeeded" {
		t.Fatal("restore failed", done.Error)
	}
	var result RestoreResult
	if err = json.Unmarshal(done.Result, &result); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result.Database, "zqzt_exhibition_restore_") || result.Manifest.Schema != 31 || !result.Ready {
		t.Fatal("wrong restored schema/target", result)
	}
	restored, err := openExistingDB(result.Database, env("DB_USER", "root"), env("DB_PASSWORD", "root"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		restored.Exec("DROP DATABASE `" + result.Database + "`")
		pool, _ := restored.DB()
		pool.Close()
	}()
	var actual VendorProductCategory
	if err = restored.First(&actual, cat.ID).Error; err != nil || actual.VendorID != vendor.ID || actual.Name != cat.Name {
		t.Fatal("restored company category mismatch", err)
	}
	var actualBinding RevisionVendorProductCategory
	if err = restored.First(&actualBinding, "revision_id=?", binding.RevisionID).Error; err != nil || actualBinding.CategoryID != cat.ID {
		t.Fatal("restored revision assignment mismatch", err)
	}
	var actualSource VendorProductCategorySource
	if err = restored.First(&actualSource, "vendor_id=? AND source_category_id=1", vendor.ID).Error; err != nil || actualSource.CategoryID != cat.ID {
		t.Fatal("restored legacy mapping mismatch", err)
	}
	t.Log("Encrypted schema31 backup restored all three enterprise classification tables in isolated database")
}

func TestVendorProductCategoryBrowser(t *testing.T) {
	if os.Getenv("RUN_VENDOR_CATEGORY_BROWSER") != "1" {
		t.Skip("set RUN_VENDOR_CATEGORY_BROWSER=1 after building web")
	}
	a := importTestApp(t)
	rootDir, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	a.Config.WebDir = filepath.Join(rootDir, "web", "dist")
	tc := testClient{t, a.router()}
	owner := resourceTestSession(t, a, "merchant")
	admin := resourceTestSession(t, a, "admin")
	var industry Category
	if err = a.DB.Where("code=?", "ciame.2026.vendor.27").First(&industry).Error; err != nil {
		t.Fatal(err)
	}
	vendor := tc.fixtureCall("POST", "/merchant/contents", owner, gin.H{"kind": "vendor", "data": Payload{Name: "企业分类验收（隔离数据）", NameEN: "Company category acceptance fixture", MainProducts: "液压回转马达、液压油缸", PrimaryCategoryID: industry.ID, RegionName: "河北省 邢台市 宁晋县", Phone: "0319-1234567"}}, 201)
	tc.call("POST", "/merchant/service-acceptances", owner, gin.H{"agreements": serviceTestRefs(t, a, "terms", "privacy", "showroom-agreement")}, 200)
	category := tc.call("POST", "/merchant/product-categories", owner, gin.H{"name": "液压回转马达", "industry_category_id": industry.ID}, 201)
	tc.call("POST", "/merchant/product-categories", owner, gin.H{"name": strings.Repeat("液压回转马达与非标定制总成", 6), "name_en": strings.Repeat("HydraulicRotaryMotorCustomAssembly", 7), "industry_category_id": industry.ID}, 201)
	catID := uint64(category["id"].(float64))
	vendorID := uint64(vendor["id"].(float64))
	imageID := watermarkUpload(t, a, owner, "/merchant/media")
	product := tc.call("POST", "/merchant/contents", owner, gin.H{"kind": "product", "data": gin.H{"name": "2.5K系列液压回转马达", "name_en": "2.5K hydraulic rotary motor", "vendor_product_category_id": catID, "model": "2.5K", "media_ids": []uint64{imageID}, "parameters": "型号：2.5K\n按订单生产"}}, 201)
	for _, item := range []map[string]any{vendor, product} {
		tc.call("POST", "/merchant/contents/"+number(item["id"])+"/submit", owner, gin.H{"revision_id": item["revision_id"]}, 200)
		tc.call("POST", "/admin/revisions/"+number(item["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	}
	var vendorRow Content
	a.DB.First(&vendorRow, vendorID)
	for i := 0; i < 25; i++ {
		p := v2Published(t, a, "product", vendorID, 1, 0, 1, false, i, fmt.Sprintf("分类检索隔离验收型号 %02d", i))
		a.DB.Model(&p).Update("owner_id", vendorRow.OwnerID)
		if err = a.DB.Create(&RevisionVendorProductCategory{p.PublishedRevisionID, catID, vendorID}).Error; err != nil {
			t.Fatal(err)
		}
	}
	srv := httptest.NewServer(a.router())
	defer srv.Close()
	a.Config.PublicBaseURL = srv.URL
	fixture, _ := json.Marshal(gin.H{"base": srv.URL, "owner": owner, "admin": admin, "account": vendorRow.OwnerID, "vendor": vendorID, "category": catID, "industry": industry.ID, "product": product["id"], "company": "/v/" + showroomSlug(vendorRow)})
	cmd := exec.CommandContext(t.Context(), "node", filepath.Join(rootDir, "scripts/verify-vendor-product-categories.cjs"))
	cmd.Dir = rootDir
	cmd.Stdin = bytes.NewReader(fixture)
	output, err := cmd.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
}

func TestVendorProductCategoryLegacySwitchAndPlatformValidation(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	owner := resourceTestSession(t, a, "merchant")
	vendorCategoryTestCompany(t, a, tc, owner, "旧版分类兼容企业")
	var leaves []Category
	if err := a.DB.Where("category_kind='product' AND parent_id>0 AND enabled=1 AND code LIKE 'ciame.2026.product.%'").Order("id").Limit(2).Find(&leaves).Error; err != nil || len(leaves) != 2 {
		t.Fatal("missing category fixtures", err)
	}
	root := vendorCategoryTestRoot(t, a)
	tc.call("POST", "/merchant/contents", owner, gin.H{"kind": "product", "data": gin.H{"name": "错误类型旧分类", "product_category_id": root.ID}}, 400)
	p := tc.call("POST", "/merchant/contents", owner, gin.H{"kind": "product", "data": gin.H{"name": "旧版分类产品", "product_category_id": leaves[0].ID}}, 201)
	first := p["data"].(map[string]any)["vendor_product_category_id"]
	if first == nil {
		t.Fatal("legacy creation was not assigned a company category")
	}
	next := tc.call("PUT", "/merchant/contents/"+number(p["id"])+"/draft", owner, gin.H{"revision_id": p["revision_id"], "data": gin.H{"name": "旧版分类产品", "product_category_id": leaves[1].ID}}, 200)
	if next["data"].(map[string]any)["vendor_product_category_id"] == first {
		t.Fatal("explicit legacy category change retained previous company assignment")
	}
	descriptor := next["vendor_product_category"].(map[string]any)
	if descriptor["name"] != leaves[1].Name {
		t.Fatal("wrong legacy category name", descriptor)
	}
	tc.call("PUT", "/merchant/product-categories/not-an-id", owner, gin.H{"name": "无效修改ID", "industry_category_id": root.ID}, 400)
	var n int64
	a.DB.Model(&VendorProductCategory{}).Count(&n)
	if n != 2 {
		t.Fatal("invalid requests created company categories", n)
	}
}
func TestVendorProductCategoryAdminPermissions(t *testing.T) {
	a := &App{}
	for _, test := range []struct{ method, want string }{{"GET", "product.read"}, {"POST", "product.write"}, {"PUT", "product.write"}, {"DELETE", "product.write"}} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(test.method, "/api/v1/admin/vendors/21/product-categories", nil)
		if actual := a.permissionForRequest(c); actual != test.want {
			t.Errorf("%s: got %s want %s", test.method, actual, test.want)
		}
	}
}
