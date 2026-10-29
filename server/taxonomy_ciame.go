package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
)

// One source catalog owns both existing API projections; IDs remain kind-specific.
//
//go:embed catalog/ciame-2026.json
var ciameCatalogJSON []byte

type ciameGroup struct {
	Name     string   `json:"name"`
	Children []string `json:"children"`
}

func ciameGroups() ([]ciameGroup, error) {
	var catalog struct {
		Groups []ciameGroup `json:"groups"`
	}
	if err := json.Unmarshal(ciameCatalogJSON, &catalog); err != nil {
		return nil, err
	}
	n := 0
	for _, g := range catalog.Groups {
		n += len(g.Children)
	}
	if len(catalog.Groups) != 29 || n != 159 {
		return nil, fmt.Errorf("invalid CIAME catalog: %d/%d", len(catalog.Groups), n)
	}
	return catalog.Groups, nil
}

// Only synonyms with an unambiguous meaning are automatic. Broad labels stay pending.
var ciameSynonyms = map[string]string{
	"耕整地机械": "耕整地机械及零部件", "播种施肥机械": "种植施肥机械及零部件",
	"收获机械": "收获机械及零部件", "田间管理与植保": "田间管理机械",
	"畜牧与饲料机械": "畜牧机械及零部件", "畜牧养殖与饲料机械": "畜牧机械及零部件",
	"农产品加工机械":  "农产品初加工机械及零部件",
	"拖拉机与动力整机": "动力机械及零部件", "发动机及配件": "动力机械及零部件",
	"发动机总成": "农用内燃机", "柴油发动机": "农用内燃机", "汽油发动机": "农用内燃机",
	"液压油缸": "液压油箱、油管、接头、油缸零部件", "液压阀": "液压阀、液压分配器零部件",
	"液压管路与接头": "液压油箱、油管、接头、油缸零部件",
	"齿轮与齿轮轴":  "通用齿轮、链条、皮带类传动零部件", "链条链轮": "通用齿轮、链条、皮带类传动零部件",
	"皮带带轮": "通用齿轮、链条、皮带类传动零部件", "变速箱与减速器": "变速、传动、分动箱零部件",
	"烘干设备": "干燥机械", "轮胎轮辋": "车轮零部件", "转向机构": "转向、控制器零部件", "制动机构": "制动器零部件",
	"耕作刀具与犁铧": "刀片类零部件", "打捆机构": "打捆、打结类零部件",
}

func migrateCIAMETaxonomy(db *gorm.DB) error {
	groups, err := ciameGroups()
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if e := lockTaxonomy(tx, 1); e != nil {
			return e
		}
		var done int64
		if e := tx.Model(&SystemSetting{}).Where("setting_key=?", "taxonomy_ciame_2026_v22").Count(&done).Error; e != nil {
			return e
		}
		if done > 0 {
			return nil
		}
		var old []Category
		if e := tx.Where("hall_id=1 AND category_kind IN ('vendor','product')").Find(&old).Error; e != nil {
			return e
		}
		byID := map[uint64]Category{}
		canonical := map[uint64]uint64{}
		for _, c := range old {
			byID[c.ID] = c
			canonical[c.ID] = canonicalCategory(tx, c.ID)
		}
		targets := map[string]map[string]uint64{}
		for _, kind := range []string{"vendor", "product"} {
			targets[kind] = map[string]uint64{}
			for i, g := range groups {
				code := fmt.Sprintf("ciame.2026.%s.%02d", kind, i+1)
				root := Category{HallID: 1, CategoryKind: kind, Code: &code, Name: g.Name, Enabled: true, SortOrder: (i + 1) * 100}
				if e := tx.Where("code=?", code).FirstOrCreate(&root).Error; e != nil {
					return e
				}
				targets[kind][g.Name] = root.ID
				for j, name := range g.Children {
					childCode := fmt.Sprintf("%s.%02d", code, j+1)
					child := Category{HallID: 1, CategoryKind: kind, Code: &childCode, Name: name, ParentID: root.ID, Enabled: true, SortOrder: (i+1)*100 + j + 1}
					if e := tx.Where("code=?", childCode).FirstOrCreate(&child).Error; e != nil {
						return e
					}
					if _, exists := targets[kind][name]; exists {
						targets[kind][name] = 0
					} else {
						targets[kind][name] = child.ID
					}
				}
			}
		}
		for _, c := range old {
			from := byID[canonical[c.ID]]
			name := from.Name
			target := targets[c.CategoryKind][name]
			if target == 0 {
				target = targets[c.CategoryKind][ciameSynonyms[name]]
			}
			if target > 0 {
				alias := CategoryAlias{SourceID: c.ID, TargetID: target}
				if e := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "source_id"}}, DoUpdates: clause.AssignmentColumns([]string{"target_id"})}).Create(&alias).Error; e != nil {
					return e
				}
				var to Category
				if e := tx.First(&to, target).Error; e != nil {
					return e
				}
				if e := tx.Model(&to).Update("search_aliases", joinedCategoryAliases(to.SearchAliases, c.Name, c.NameEN, c.SearchAliases)).Error; e != nil {
					return e
				}
			}
			if e := tx.Model(&Category{}).Where("id=?", c.ID).Update("enabled", false).Error; e != nil {
				return e
			}
		}
		// Mapping metadata changes; immutable revision JSON and references keep provenance.
		var after uint64
		for {
			var revisions []Revision
			if e := tx.Table("revisions r").Joins("JOIN contents c ON c.id=r.content_id AND c.hall_id=1").Where("r.id>?", after).Select("r.*").Order("r.id").Limit(250).Find(&revisions).Error; e != nil {
				return e
			}
			if len(revisions) == 0 {
				break
			}
			for _, rev := range revisions {
				var content Content
				if e := tx.First(&content, rev.ContentID).Error; e != nil {
					return e
				}
				var refs []RevisionCategory
				if e := tx.Where("revision_id=?", rev.ID).Find(&refs).Error; e != nil {
					return e
				}
				row := TaxonomyMapping{RevisionID: rev.ID, ContentID: content.ID, CategoryKind: content.Kind, LegacyCategoryID: rev.CategoryID, Status: "automatic", UpdatedAt: time.Now().UTC()}
				for _, ref := range refs {
					id := canonicalCategory(tx, ref.CategoryID)
					var cat Category
					if e := tx.First(&cat, id).Error; e != nil {
						return e
					}
					if cat.Code == nil || !strings.HasPrefix(*cat.Code, "ciame.2026.") {
						row.Status = "pending"
						row.LegacyCategoryID = ref.CategoryID
					}
					if ref.Role != "secondary" {
						row.TargetID = id
					}
				}
				if len(refs) == 0 {
					row.Status = "pending"
				}
				if row.Status == "pending" {
					row.TargetID = 0
				}
				if e := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "revision_id"}}, DoUpdates: clause.AssignmentColumns([]string{"legacy_category_id", "target_id", "status", "updated_at"})}).Create(&row).Error; e != nil {
					return e
				}
			}
			after = revisions[len(revisions)-1].ID
		}
		return tx.Create(&SystemSetting{SettingKey: "taxonomy_ciame_2026_v22", Value: `{"complete":true,"primary":29,"secondary":159}`, UpdatedAt: time.Now().UTC()}).Error
	})
}
