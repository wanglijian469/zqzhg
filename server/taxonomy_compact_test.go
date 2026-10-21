package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"strings"
	"testing"
)

func TestCompactTaxonomyMigrationAndCompatibility(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	active := tc.call("GET", "/public/taxonomy?category_kind=product", "", nil, 200)["categories"].([]any)
	roots := 0
	for _, raw := range active {
		if raw.(map[string]any)["parent_id"] == float64(0) {
			roots++
		}
	}
	if len(active) != 45 || roots != 10 {
		t.Fatal("fresh compact directory", len(active), roots)
	}
	if len(compactMappingCodes()) != 134 {
		t.Fatal("mapping must cover every previous seed")
	}
	// Recreate a Schema17 data set inside this disposable database.
	if e := a.DB.Transaction(func(tx *gorm.DB) error {
		for _, sql := range []string{"DELETE FROM category_aliases", "DELETE FROM categories WHERE code LIKE 'agri.product.compact.%'", "UPDATE categories SET enabled=1 WHERE code LIKE 'agri.product.%'", "DELETE FROM system_settings WHERE setting_key='taxonomy_compact_v18'", "DELETE FROM schema_migrations WHERE version=18"} {
			if e := tx.Exec(sql).Error; e != nil {
				return e
			}
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	old := []Category{}
	a.DB.Where("category_kind='product'").Find(&old)
	codeID := map[string]uint64{}
	for _, c := range old {
		if c.Code != nil {
			codeID[*c.Code] = c.ID
		}
	}
	vendor := Content{HallID: 1, Kind: "vendor", OwnerID: 1, Visible: true}
	a.DB.Create(&vendor)
	vb, _ := json.Marshal(Payload{Name: "精简验收厂家", CategoryID: 1})
	vr := Revision{ContentID: vendor.ID, Name: "精简验收厂家", Payload: string(vb), Status: "approved"}
	a.DB.Create(&vr)
	a.DB.Model(&vendor).Updates(map[string]any{"published_revision_id": vr.ID, "draft_revision_id": vr.ID})
	snapshots := map[uint64]string{}
	add := func(cat uint64) {
		t.Helper()
		c := Content{HallID: 1, Kind: "product", OwnerID: 1, VendorID: vendor.ID, Visible: true}
		if e := a.DB.Create(&c).Error; e != nil {
			t.Fatal(e)
		}
		p := Payload{Name: "隔离分类记录", ProductCategoryID: cat, CategoryID: 2, Parameters: "原参数与快照不变", MediaIDs: []uint64{987}, Brand: "保留品牌"}
		blob, _ := json.Marshal(p)
		r := Revision{ContentID: c.ID, Name: p.Name, CategoryID: 2, Payload: string(blob), Status: "approved"}
		if e := a.DB.Create(&r).Error; e != nil {
			t.Fatal(e)
		}
		a.DB.Model(&c).Updates(map[string]any{"published_revision_id": r.ID, "draft_revision_id": r.ID})
		if e := indexTaxonomyRevision(a.DB, c, r, p, false); e != nil {
			t.Fatal(e)
		}
		snapshots[r.ID] = r.Payload
	}
	for _, c := range old {
		if c.ParentID > 0 {
			add(c.ID)
		}
	}
	// Existing administrator merges have priority over the original seed mapping.
	if e := mergeTaxonomyCategory(a.DB, codeID["agri.product.13.01"], codeID["agri.product.11.01"], 1); e != nil {
		t.Fatal(e)
	}
	custom := Category{HallID: 1, CategoryKind: "product", Name: "自定义机构", ParentID: codeID["agri.product.12"], Enabled: true}
	a.DB.Create(&custom)
	customChild := Category{HallID: 1, CategoryKind: "product", Name: "自定义三级", ParentID: custom.ID, Enabled: true}
	a.DB.Create(&customChild)
	add(customChild.ID)
	overflow := Category{HallID: 1, CategoryKind: "product", Name: "待处理自定义", ParentID: codeID["agri.product.01"], Enabled: true}
	a.DB.Create(&overflow)
	overflowChild := Category{HallID: 1, CategoryKind: "product", Name: "超层级原分类", ParentID: overflow.ID, Enabled: true}
	a.DB.Create(&overflowChild)
	add(overflowChild.ID)
	if e := migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	for _, c := range old {
		var actual Category
		a.DB.First(&actual, c.ID)
		if actual.Name != c.Name || actual.NameEN != c.NameEN || actual.ParentID != c.ParentID || actual.ImageMediaID != c.ImageMediaID || actual.Enabled {
			t.Fatal("source classification overwritten", c.ID)
		}
		expectedCode := compactMappingCodes()[*c.Code]
		if c.ID == codeID["agri.product.13.01"] {
			expectedCode = "agri.product.compact.05.01"
		}
		var target Category
		a.DB.Where("code=?", expectedCode).First(&target)
		if canonicalCategory(a.DB, c.ID) != target.ID {
			t.Fatal("mapping mismatch", *c.Code, expectedCode)
		}
	}
	for id, payload := range snapshots {
		var r Revision
		a.DB.First(&r, id)
		if r.Payload != payload {
			t.Fatal("raw snapshot changed", id)
		}
	}
	var engine Category
	a.DB.Where("code='agri.product.compact.06'").First(&engine)
	a.DB.First(&custom, custom.ID)
	if custom.ParentID != engine.ID {
		t.Fatal("custom subtree not moved")
	}
	a.DB.First(&overflow, overflow.ID)
	if overflow.ParentID != codeID["agri.product.01"] {
		t.Fatal("unsafe subtree was moved")
	}
	var pending int64
	a.DB.Model(&TaxonomyMapping{}).Where("status='pending' AND legacy_category_id=?", overflowChild.ID).Count(&pending)
	if pending != 1 {
		t.Fatal("missing manual entry")
	}
	for keyword, want := range map[string]float64{"发动机机构件": 2, "缸体缸盖": 2, "Hydraulic cylinders": 3} {
		// All names are deliberately identical, so these hits must come from classification aliases.
		result := tc.call("GET", "/public/products?q="+strings.ReplaceAll(keyword, " ", "%20"), "", nil, 200)
		if result["total"] != want {
			t.Fatal("category search", keyword, result["total"], want)
		}
	}
	result := tc.call("GET", fmt.Sprintf("/public/products?category_id=%d", engine.ID), "", nil, 200)
	if result["total"] != float64(9) {
		t.Fatal("parent includes mapped and custom descendants", result["total"])
	}
	recommendation := tc.call("POST", "/public/category-recommendations", "", gin.H{"name": "旋耕刀"}, 200)
	if len(recommendation["items"].([]any)) == 0 {
		t.Fatal("old alias recommendation lost")
	}
	var countBefore int64
	a.DB.Model(&Category{}).Count(&countBefore)
	if e := a.DB.Exec("DELETE FROM schema_migrations WHERE version=18").Error; e != nil {
		t.Fatal(e)
	}
	if e := migrate(a.DB); e != nil {
		t.Fatal("interrupted marker retry", e)
	}
	var countAfter int64
	a.DB.Model(&Category{}).Count(&countAfter)
	if countBefore != countAfter {
		t.Fatal("duplicate retry seed")
	}
	if e := compactAgricultureTaxonomy(a.DB); e != nil {
		t.Fatal(e)
	}
}
