package main

import (
	"encoding/json"

	"testing"
	"time"
)

func TestCIAMEFreshCatalogAndDisabledProcurement(t *testing.T) {
	t.Setenv("PROCUREMENT_ENABLED", "false")
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	var names []string
	for _, kind := range []string{"vendor", "product"} {
		categories := tc.call("GET", "/public/taxonomy?category_kind="+kind, "", nil, 200)["categories"].([]any)
		if len(categories) != 188 {
			t.Fatalf("%s catalog: %d", kind, len(categories))
		}
		roots := 0
		projection := []string{}
		for _, raw := range categories {
			c := raw.(map[string]any)
			if c["parent_id"] == float64(0) {
				roots++
			}
			projection = append(projection, c["name"].(string))
		}
		if roots != 29 {
			t.Fatal("primary count", roots)
		}
		if kind == "vendor" {
			names = projection
		} else {
			for i, name := range projection {
				if names[i] != name {
					t.Fatal("catalog projections diverged", i)
				}
			}
		}
	}
	for _, path := range []string{"/public/procurements", "/public/procurements/1", "/public/procurement-vendors", "/procurement/profile", "/procurement/notifications", "/admin/procurement/projects"} {
		if tc.call("GET", path, "", nil, 503)["error"] == nil { /* status is the contract; error shape is shared */
		}
	}
	config := tc.call("GET", "/public/config", "", nil, 200)
	if config["capabilities"].(map[string]any)["procurement"] != false {
		t.Fatal("procurement advertised")
	}
	if config["site"].(map[string]any)["icp_number"] != "冀ICP备2026047360号-2" {
		t.Fatal("missing ICP registration")
	}
}

func TestCIAMEMigrationRetainsSnapshotsAndPendingAssignments(t *testing.T) {
	a := importTestAppAtSchema(t, 21)
	exact := Category{HallID: 1, CategoryKind: "vendor", Name: "谷物收获机械", Enabled: true}
	broad := Category{HallID: 1, CategoryKind: "vendor", Name: "综合农机生产与服务", Enabled: true}
	for _, c := range []*Category{&exact, &broad} {
		if e := a.DB.Create(c).Error; e != nil {
			t.Fatal(e)
		}
	}
	snapshots := map[uint64]string{}
	for _, cat := range []Category{exact, broad} {
		c := Content{HallID: 1, Kind: "vendor", OwnerID: cat.ID, Visible: true}
		if e := a.DB.Create(&c).Error; e != nil {
			t.Fatal(e)
		}
		p := Payload{Name: cat.Name, PrimaryCategoryID: cat.ID, MainProducts: "保留产品说明", MediaIDs: []uint64{987}, Phone: "13900000000"}
		blob, _ := json.Marshal(p)
		r := Revision{ContentID: c.ID, Name: p.Name, Payload: string(blob), Status: "approved", CreatedAt: time.Now()}
		if e := a.DB.Create(&r).Error; e != nil {
			t.Fatal(e)
		}
		if e := a.DB.Model(&c).Updates(map[string]any{"draft_revision_id": r.ID, "published_revision_id": r.ID}).Error; e != nil {
			t.Fatal(e)
		}
		if e := indexTaxonomyRevision(a.DB, c, r, p, false); e != nil {
			t.Fatal(e)
		}
		snapshots[r.ID] = r.Payload
	}
	if e := migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	if e := migrate(a.DB); e != nil {
		t.Fatal("repeat migration", e)
	}
	if e := migrateCIAMETaxonomy(a.DB); e != nil {
		t.Fatal("repeat seeding", e)
	}
	for id, original := range snapshots {
		var r Revision
		a.DB.First(&r, id)
		if r.Payload != original {
			t.Fatal("immutable snapshot changed")
		}
		var mapping TaxonomyMapping
		if e := a.DB.First(&mapping, "revision_id=?", id).Error; e != nil {
			t.Fatal(e)
		}
		if r.Name == exact.Name && (mapping.Status != "automatic" || mapping.TargetID == 0) {
			t.Fatal("exact mapping", mapping)
		}
		if r.Name == broad.Name && (mapping.Status != "pending" || mapping.TargetID != 0) {
			t.Fatal("ambiguous mapping", mapping)
		}
	}
	var count int64
	a.DB.Model(&Category{}).Where("enabled=1 AND hall_id=1 AND category_kind IN ('vendor','product')").Count(&count)
	if count != 376 {
		t.Fatal("migration not idempotent", count)
	}
	var old Category
	a.DB.First(&old, exact.ID)
	if old.Enabled {
		t.Fatal("old category still selectable")
	}
	var target Category
	a.DB.First(&target, canonicalCategory(a.DB, exact.ID))
	if target.Name != exact.Name || target.ParentID == 0 {
		t.Fatal("exact target", target)
	}
	var pending int64
	a.DB.Model(&TaxonomyMapping{}).Where("status='pending'").Count(&pending)
	if pending == 0 {
		t.Fatal("missing pending records")
	}
}
