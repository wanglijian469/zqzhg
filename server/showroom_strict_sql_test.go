package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

func TestShowroomOnlyFullGroupBy(t *testing.T) {
	a := importTestApp(t) // Creates and verifies a disposable test database.
	originalDB := a.DB
	err := originalDB.Connection(func(connection *gorm.DB) error {
		connection = connection.Session(&gorm.Session{NewDB: true})
		var originalMode, globalBefore, version string
		if err := connection.Raw("SELECT @@SESSION.sql_mode").Scan(&originalMode).Error; err != nil {
			return err
		}
		if err := connection.Raw("SELECT @@GLOBAL.sql_mode").Scan(&globalBefore).Error; err != nil {
			return err
		}
		if err := connection.Raw("SELECT VERSION()").Scan(&version).Error; err != nil {
			return err
		}
		mode := originalMode
		if !strings.Contains(mode, "ONLY_FULL_GROUP_BY") {
			mode = strings.Trim(mode+",ONLY_FULL_GROUP_BY", ",")
		}
		if err := connection.Exec("SET SESSION sql_mode = ?", mode).Error; err != nil {
			return err
		}
		a.DB = connection // Pin every fixture and HTTP handler to the same session.
		defer func() {
			a.DB = originalDB
			if err := connection.Exec("SET SESSION sql_mode = ?", originalMode).Error; err != nil {
				t.Error("restore isolated connection SQL mode:", err)
			}
			var globalAfter string
			if err := connection.Raw("SELECT @@GLOBAL.sql_mode").Scan(&globalAfter).Error; err != nil || globalAfter != globalBefore {
				t.Error("global SQL mode must remain unchanged", err)
			}
		}()
		t.Logf("MySQL %s: pinned ONLY_FULL_GROUP_BY session; no global SQL mode changes", version)

		parent := Category{HallID: 1, Name: "严格分组父分类", Enabled: true, CategoryKind: "product"}
		if err := a.DB.Create(&parent).Error; err != nil {
			return err
		}
		children := []Category{
			{HallID: 1, ParentID: parent.ID, Name: "严格分组分类甲", Enabled: true, CategoryKind: "product"},
			{HallID: 1, ParentID: parent.ID, Name: "严格分组分类乙", Enabled: true, CategoryKind: "product"},
			{HallID: 1, Name: "严格分组旧分类", Enabled: true},
		}
		if err := a.DB.Create(&children).Error; err != nil {
			return err
		}
		if err := a.DB.Create(&CategoryAlias{SourceID: children[2].ID, TargetID: children[1].ID}).Error; err != nil {
			return err
		}
		vendor := v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, "严格模式隔离测试厂商")
		if err := a.DB.Model(&vendor).Update("showroom_slug", "strict-mode-supplier").Error; err != nil {
			return err
		}
		empty := v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, "无产品测试厂商")
		modern := v2Published(t, a, "product", vendor.ID, 1, children[1].ID, 1, false, 0, "现代分类公开产品")
		if err := a.DB.Create(&RevisionCategory{RevisionID: modern.PublishedRevisionID, CategoryID: children[0].ID, Role: "product", Source: "explicit"}).Error; err != nil {
			return err
		}
		if err := a.DB.Create(&RevisionCategory{RevisionID: modern.PublishedRevisionID, CategoryID: children[1].ID, Role: "secondary", Source: "explicit"}).Error; err != nil {
			return err
		}
		v2Published(t, a, "product", vendor.ID, 1, children[0].ID, 1, false, 1, "同类公开产品")
		v2Published(t, a, "product", vendor.ID, 1, children[1].ID, 1, false, 2, "旧字段回退产品")
		v2Published(t, a, "product", vendor.ID, 1, children[2].ID, 1, false, 3, "旧别名分类产品")
		for _, state := range []string{"hidden", "draft", "pending"} {
			product := v2Published(t, a, "product", vendor.ID, 1, children[1].ID, 1, false, 0, "不公开产品"+state)
			if state == "hidden" {
				if err := a.DB.Model(&product).Update("visible", false).Error; err != nil {
					return err
				}
			} else if err := a.DB.Model(&Revision{}).Where("id=?", product.PublishedRevisionID).Update("status", state).Error; err != nil {
				return err
			}
		}
		foreign := v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, "其他厂商")
		v2Published(t, a, "product", foreign.ID, 1, children[1].ID, 1, false, 0, "其他厂商产品")
		preparing := v2Published(t, a, "vendor", 0, 2, 1, 1, false, 0, "筹备展馆厂商")
		v2Published(t, a, "product", preparing.ID, 2, children[1].ID, 1, false, 0, "筹备展馆产品")
		v2Published(t, a, "product", vendor.ID, 2, children[1].ID, 1, false, 0, "跨馆产品不得进入统计")
		privateRevision := Revision{ContentID: modern.ID, Status: "draft", CategoryID: children[1].ID, Name: "私有修改", Payload: "{}"}
		if err := a.DB.Create(&privateRevision).Error; err != nil {
			return err
		}
		if err := a.DB.Create(&RevisionCategory{RevisionID: privateRevision.ID, CategoryID: children[1].ID, Role: "product", Source: "explicit"}).Error; err != nil {
			return err
		}
		if err := a.DB.Model(&modern).Update("draft_revision_id", privateRevision.ID).Error; err != nil {
			return err
		}

		t.Run("old-query-reproduces-1055", func(t *testing.T) {
			var rows []struct{ CategoryID, Total uint64 }
			expression := "COALESCE((SELECT rc.category_id FROM revision_categories rc WHERE rc.revision_id=r.id AND rc.role='product' LIMIT 1),r.category_id)"
			err := a.publicQuery("product").Where("contents.vendor_id=?", vendor.ID).
				Select(expression + " AS category_id,COUNT(*) AS total").Group(expression).Scan(&rows).Error
			var mysqlError *mysqldriver.MySQLError
			if !errors.As(err, &mysqlError) || mysqlError.Number != 1055 {
				t.Fatalf("strict regression setup must reproduce mysql_1055; got %v", err)
			}
		})

		router := a.router()
		t.Run("public-categories-and-ancestors", func(t *testing.T) {
			tc := testClient{t, router}
			result := tc.call("GET", "/public/showrooms/strict-mode-supplier", "", nil, 200)
			if result["product_count"] != float64(4) {
				t.Fatal("published product count changed", result["product_count"])
			}
			counts := map[uint64]int64{}
			for _, raw := range result["categories"].([]any) {
				row := raw.(map[string]any)
				counts[uint64(row["id"].(float64))] = int64(row["product_count"].(float64))
			}
			if len(counts) != 3 || counts[parent.ID] != 4 || counts[children[0].ID] != 2 || counts[children[1].ID] != 2 {
				t.Fatal("effective categories, legacy alias or ancestor totals changed", counts)
			}
		})
		t.Run("empty-catalog", func(t *testing.T) {
			tc := testClient{t, router}
			result := tc.call("GET", fmt.Sprintf("/public/showrooms/v-%d", empty.ID), "", nil, 200)
			if result["product_count"] != float64(0) || len(result["categories"].([]any)) != 0 {
				t.Fatal("empty catalog changed")
			}
		})
		t.Run("home-product-and-seo", func(t *testing.T) {
			tc := testClient{t, router}
			tc.call("GET", "/public/home", "", nil, 200)
			tc.call("GET", fmt.Sprintf("/public/products/%d", modern.ID), "", nil, 200)
			seoTestIndex(t, a)
			for _, path := range []string{"/v/strict-mode-supplier", fmt.Sprintf("/products/%d", modern.ID)} {
				if response := seoGet(router, path); response.Code != 200 {
					t.Fatalf("SEO %s returned %d", path, response.Code)
				}
			}
		})
		t.Run("vendor-publication-and-hall-boundaries", func(t *testing.T) {
			tc := testClient{t, router}
			tc.call("GET", fmt.Sprintf("/public/showrooms/v-%d", preparing.ID), "", nil, 404)
			if err := a.DB.Model(&Hall{}).Where("id=2").Update("status", "open").Error; err != nil {
				t.Fatal(err)
			}
			if result := tc.call("GET", "/public/showrooms/strict-mode-supplier", "", nil, 200); result["product_count"] != float64(4) {
				t.Fatal("cross-hall product must stay excluded after opening the other hall")
			}
			if err := a.DB.Model(&vendor).Update("visible", false).Error; err != nil {
				t.Fatal(err)
			}
			tc.call("GET", "/public/showrooms/strict-mode-supplier", "", nil, 404)
			if err := a.DB.Model(&vendor).Update("visible", true).Error; err != nil {
				t.Fatal(err)
			}
			if err := a.DB.Model(&Revision{}).Where("id=?", vendor.PublishedRevisionID).Update("status", "pending").Error; err != nil {
				t.Fatal(err)
			}
			tc.call("GET", "/public/showrooms/strict-mode-supplier", "", nil, 404)
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
