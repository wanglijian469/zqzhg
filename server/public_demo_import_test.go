package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestPublicDemoImportIsIncrementalAndKeepsURLs(t *testing.T) {
	a := importTestApp(t)
	// Cloud has watermarking enabled. Public export images must keep their
	// existing rendering and must not create variants on another DB connection.
	cfg := defaultSystemConfig()
	cfg.Watermark.Enabled = true
	rawConfig, _ := json.Marshal(cfg)
	a.DB.Where("setting_key=?", "system_config_v3").Delete(&SystemSetting{})
	if err := a.DB.Create(&SystemSetting{SettingKey: "system_config_v3", Value: string(rawConfig)}).Error; err != nil {
		t.Fatal(err)
	}
	var vendorCategory, productCategory Category
	if err := a.DB.Where("category_kind='vendor' AND parent_id=0 AND enabled=1").First(&vendorCategory).Error; err != nil {
		t.Fatal(err)
	}
	if err := a.DB.Where("category_kind='product' AND parent_id>0 AND enabled=1").First(&productCategory).Error; err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	var img bytes.Buffer
	_ = png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	_ = os.WriteFile(filepath.Join(root, "public.png"), img.Bytes(), 0600)
	vendorCategory.ID, productCategory.ID = 987, 988
	bundle := publicDemoBundle{Format: 1, Usage: mapDemoUsage, Source: "http://127.0.0.1:8080", Categories: []Category{vendorCategory, productCategory}, Media: map[string]publicDemoMedia{"100": {"public.png", previewDigest(img.Bytes())}}, Vendors: []View{{Content: Content{ID: 101, Kind: "vendor", Visible: true, PublishedRevisionID: 1}, ShowroomSlug: "demo-test-booth", Data: Payload{Name: "开发演示企业（隔离样本）", MainProducts: "演示机械", Phone: "0319-1234567", RegionCode: "130528", PrimaryCategoryID: 987, CategoryID: 987, MediaIDs: []uint64{100}, CompanyProfile: &CompanyProfile{LogoMediaID: 100}}}}, Products: []View{{Content: Content{ID: 102, Kind: "product", VendorID: 101, Visible: true, PublishedRevisionID: 2}, Data: Payload{Name: "开发演示产品", Model: "DEMO-1", Description: "隔离测试样本", CategoryID: 988, ProductCategoryID: 988, MediaIDs: []uint64{100}}}}}
	path := filepath.Join(root, "catalog.json")
	write := func() {
		raw, _ := json.Marshal(bundle)
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write()
	result, err := a.importPublicDemo(path)
	if err != nil {
		t.Fatal(err)
	}
	if result.Vendors != 1 || result.Products != 1 || result.Images != 1 {
		t.Fatalf("unexpected imported counts: %+v", result)
	}
	var vendor Content
	if err = a.DB.Where("showroom_slug='demo-test-booth'").First(&vendor).Error; err != nil {
		t.Fatal(err)
	}
	var owner Account
	a.DB.First(&owner, vendor.OwnerID)
	if owner.Phone != nil || owner.PasswordHash != "" || owner.Role != "merchant" {
		t.Fatal("public contact was converted into a login identity")
	}
	original := vendor.PublishedRevisionID
	bundle.Vendors[0].Data.Description = "changed source must not overwrite cloud"
	bundle.Vendors[0].ShowroomSlug = "changed-demo-url"
	write()
	result, err = a.importPublicDemo(path)
	if err != nil {
		t.Fatal(err)
	}
	if result.Vendors != 0 || result.Products != 0 || result.Images != 0 || result.PreservedVendors != 1 || result.PreservedProducts != 1 {
		t.Fatalf("import was not idempotent: %+v", result)
	}
	a.DB.First(&vendor, vendor.ID)
	if vendor.PublishedRevisionID != original || showroomSlug(vendor) != "demo-test-booth" {
		t.Fatal("existing snapshot or URL overwritten")
	}
	var rev Revision
	a.DB.First(&rev, original)
	if !view(vendor, rev).Demonstration {
		t.Fatal("imported demonstration was not identified")
	}
	bundle.Vendors[0].Visible = false
	write()
	if _, err = a.importPublicDemo(path); err == nil {
		t.Fatal("non-public source accepted")
	}
}
