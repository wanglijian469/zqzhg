package main

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestTaxonomyBrowserHarness(t *testing.T) {
	if os.Getenv("TAXONOMY_BROWSER_HARNESS") != "1" {
		t.Skip("explicit disposable taxonomy browser harness")
	}
	a := importTestApp(t)
	root, _ := filepath.Abs("..")
	a.Config.WebDir = env("BROWSER_WEB_DIR", filepath.Join(root, ".local", "compact-dist"))
	admin := resourceTestSession(t, a, "admin")
	account := Account{Username: "taxonomy-browser-" + randomToken()[:12], Role: "merchant"}
	if e := a.DB.Create(&account).Error; e != nil {
		t.Fatal(e)
	}
	session, e := a.newSession(account)
	if e != nil {
		t.Fatal(e)
	}
	merchant := session["token"].(string)
	cat := func(code string) uint64 {
		var c Category
		if e := a.DB.Where("code=?", code).First(&c).Error; e != nil {
			t.Fatal(e)
		}
		return c.ID
	}
	tc := testClient{t, a.router()}
	vendor := tc.call("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "分类升级验收企业（隔离测试）", NameEN: "Taxonomy test company", MainProducts: "测试液压油缸", RegionName: "河北省 邢台市 宁晋县", Phone: "0319-1234567", PrimaryCategoryID: cat("agri.vendor.09"), SecondaryCategoryIDs: []uint64{cat("agri.vendor.08")}, BusinessNature: "manufacturer"}}, 201)
	tc.call("POST", "/merchant/contents/"+number(vendor["id"])+"/submit", merchant, gin.H{"revision_id": vendor["revision_id"]}, 200)
	tc.call("POST", "/admin/revisions/"+number(vendor["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	product := tc.call("POST", "/merchant/contents", merchant, gin.H{"kind": "product", "data": Payload{Name: "验收液压油缸（隔离测试）", NameEN: "Test hydraulic cylinder", ProductCategoryID: cat("agri.product.compact.07.01"), Brand: "测试品牌", ApplicableModels: []string{"A100"}, Parameters: "原参数=保留"}}, 201)
	tc.call("POST", "/merchant/contents/"+number(product["id"])+"/submit", merchant, gin.H{"revision_id": product["revision_id"]}, 200)
	tc.call("POST", "/admin/revisions/"+number(product["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	belt := tc.call("POST", "/admin/industrial-belts", admin, gin.H{"name": "验收产业带（隔离测试）", "name_en": "Test industrial belt", "enabled": true}, 200)
	tc.call("PUT", "/admin/industrial-belts/"+number(belt["id"])+"/vendors", admin, gin.H{"vendor_ids": []any{vendor["id"]}}, 200)
	// A pre-upgrade revision is indexed separately without changing its payload.
	historical := Content{HallID: 1, Kind: "product", OwnerID: account.ID, VendorID: uint64(vendor["id"].(float64))}
	a.DB.Create(&historical)
	raw, _ := json.Marshal(gin.H{"name": "待映射历史展品（隔离测试）", "category_id": 2, "parameters": "旧参数不可覆盖"})
	revision := Revision{ContentID: historical.ID, Name: "待映射历史展品（隔离测试）", CategoryID: 2, Payload: string(raw), Status: "draft"}
	a.DB.Create(&revision)
	a.DB.Model(&historical).Update("draft_revision_id", revision.ID)
	if e := indexTaxonomyRevision(a.DB, historical, revision, Payload{CategoryID: 2}, true); e != nil {
		t.Fatal(e)
	}
	pic := image.NewRGBA(image.Rect(0, 0, 120, 120))
	draw.Draw(pic, pic.Bounds(), &image.Uniform{C: color.RGBA{30, 110, 180, 255}}, image.Point{}, draw.Src)
	sample := filepath.Join(root, ".local", "taxonomy-isolated.png")
	f, e := os.Create(sample)
	if e != nil {
		t.Fatal(e)
	}
	_ = png.Encode(f, pic)
	_ = f.Close()
	defer os.Remove(sample)
	srv := httptest.NewServer(a.router())
	defer srv.Close()
	a.Config.PublicBaseURL = srv.URL
	fixture := filepath.Join(root, ".local", "taxonomy-browser-fixture.json")
	defer os.Remove(fixture)
	blob, _ := json.Marshal(gin.H{"base": srv.URL, "admin_token": admin, "merchant_token": merchant, "account_id": account.ID, "vendor_id": vendor["id"], "product_id": product["id"], "mapping_revision_id": revision.ID, "cylinder_id": cat("agri.product.compact.07.01"), "gear_id": cat("agri.product.compact.05.01"), "vendor_secondary_id": cat("agri.vendor.08"), "belt_id": belt["id"], "sample_path": sample})
	if e := os.WriteFile(fixture, blob, 0600); e != nil {
		t.Fatal(e)
	}
	command := exec.CommandContext(t.Context(), "node", filepath.Join(root, "scripts", "verify-taxonomy.cjs"))
	command.Dir = root
	output, e := command.CombinedOutput()
	t.Log(string(output))
	if e != nil {
		t.Fatal(e)
	}
	var preserved Revision
	a.DB.First(&preserved, revision.ID)
	if preserved.Payload != string(raw) {
		t.Fatal("browser mapping rewrote the historical snapshot")
	}
}
