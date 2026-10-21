package main

import (
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sort"
	"strings"
	"time"
)

// Schema17 remains immutable. Compact codes are independent of database IDs.
const compactProductCatalog = `整机与作业机械|Power and field machinery|拖拉机与动力整机:Tractors and power units,耕整地机械:Tillage machinery,播种施肥机械:Seeding and fertilizing machinery,田间管理与植保机械:Field management and crop protection machinery,收获机械:Harvesting machinery
收获后处理与农产品加工|Post-harvest and agricultural processing|收获后处理与搬运设备:Post-harvest handling equipment,粮油加工设备:Grain and oilseed processing equipment,果蔬加工与包装设备:Fruit vegetable processing and packaging
畜牧与饲料机械|Livestock and feed machinery|饲料加工设备:Feed processing equipment,畜牧养殖设备:Livestock farming equipment
农机具及通用配件|Implements and general parts|配套机具与挂接装置:Supporting implements and hitches,刀片刀具与犁铧:Blades tools and ploughshares,其他农机易损件:Other agricultural wear parts,轴承密封与标准件:Bearings seals and standard parts
传动系统及零部件|Transmission systems and parts|齿轮与变速传动:Gears gearboxes and reducers,传动轴与联接件:Drive shafts and couplings,链条皮带与动力输出:Chains belts and power take-off
发动机及配件|Engines and parts|发动机总成:Complete engines,发动机机构件:Engine mechanical parts,供油进排气系统:Fuel intake and exhaust systems,冷却润滑与滤清器:Cooling lubrication and filters
液压与行走底盘|Hydraulics and running gear|液压油缸:Hydraulic cylinders,液压泵阀与管路:Hydraulic pumps valves and hoses,行走底盘件:Running gear components,转向制动件:Steering and brake components
收获与耕播专用件|Harvesting tillage and seeding components|收获作业机构:Harvesting mechanisms,耕整地机构件:Tillage mechanisms,播种施肥机构件:Seeding and fertilizing mechanisms
电气与智能设备|Electrical and smart equipment|电气线束与仪表:Electrical wiring and instruments,传感器与控制器:Sensors and controllers,导航监测与农业物联网:Navigation monitoring and agricultural IoT
灌溉与设施农业|Irrigation and protected agriculture|水泵与灌排设备:Pumps irrigation and drainage equipment,水肥一体化设备:Fertigation equipment,温室与设施配件:Greenhouses and facility parts,设施环境控制设备:Facility environment control equipment`

// Entries map each Schema17 leaf, in the original directory order, to group.leaf.
var compactLeafDestinations = [][]string{
	{"01.01", "01.01", "01.01", "01.01"},
	{"01.02", "01.02", "01.02", "01.02", "01.02", "01.02"},
	{"01.03", "01.03", "01.03", "01.03", "01.03", "01.03"},
	{"01.04", "01.04", "01.04", "01.04"},
	{"01.05", "01.05", "01.05", "01.05", "01.05", "01.05"},
	{"02.01", "02.01", "02.01", "02.01", "02.01"},
	{"03.01", "03.01", "03.02", "03.02", "03.02"},
	{"02.02", "02.02", "02.03", "02.03"},
	{"04.01", "04.01", "04.01", "04.01", "04.01"},
	{"04.02", "04.02", "04.02", "04.03", "04.03"},
	{"05.01", "05.01", "05.02", "05.02", "05.03", "05.03", "05.03"},
	{"06.01", "06.01", "06.02", "06.02", "06.03", "06.03", "06.04", "06.04"},
	{"07.01", "07.02", "07.02", "07.02", "07.02", "07.02"},
	{"07.03", "07.03", "07.03", "07.03", "07.04", "07.04"},
	{"08.01", "08.01", "08.01", "08.01", "08.01", "08.01"},
	{"08.02", "08.02", "08.02", "08.03", "08.03", "08.03"},
	{"04.04", "04.04", "04.04", "04.04", "04.04", "04.04", "04.04"},
	{"09.01", "09.01", "09.02", "09.03", "09.03", "09.03"},
	{"10.01", "10.01", "10.01", "10.01", "10.02", "10.01"},
	{"10.03", "10.04", "10.04", "10.04", "10.03", "10.04"},
}

func compactMappingCodes() map[string]string {
	result := map[string]string{}
	for i, leaves := range compactLeafDestinations {
		root := fmt.Sprintf("agri.product.%02d", i+1)
		target := leaves[0]
		same := true
		for j, dest := range leaves {
			result[fmt.Sprintf("%s.%02d", root, j+1)] = "agri.product.compact." + dest
			if dest != target {
				same = false
			}
		}
		if !same {
			target = strings.Split(target, ".")[0]
		}
		result[root] = "agri.product.compact." + target
	}
	return result
}

func joinedCategoryAliases(values ...string) string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		for _, word := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == '，' || r == '\n' }) {
			word = strings.TrimSpace(word)
			if word != "" && !seen[word] {
				seen[word] = true
				out = append(out, word)
			}
		}
	}
	return strings.Join(out, ",")
}

func compactAgricultureTaxonomy(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if e := lockTaxonomy(tx, 1); e != nil {
			return e
		}
		var done int64
		if e := tx.Model(&SystemSetting{}).Where("setting_key='taxonomy_compact_v18'").Count(&done).Error; e != nil {
			return e
		}
		if done > 0 {
			return nil
		}
		targets := map[string]Category{}
		for i, line := range strings.Split(compactProductCatalog, "\n") {
			p := strings.Split(line, "|")
			code := fmt.Sprintf("agri.product.compact.%02d", i+1)
			root := Category{HallID: 1, CategoryKind: "product", Code: &code, Name: p[0], NameEN: p[1], Enabled: true, SortOrder: (i + 1) * 10}
			if e := tx.Where("code=?", code).FirstOrCreate(&root).Error; e != nil {
				return e
			}
			targets[code] = root
			for j, child := range strings.Split(p[2], ",") {
				names := strings.SplitN(child, ":", 2)
				subcode := fmt.Sprintf("%s.%02d", code, j+1)
				sub := Category{HallID: 1, CategoryKind: "product", Code: &subcode, ParentID: root.ID, Name: names[0], NameEN: names[1], Enabled: true, SortOrder: (j + 1) * 10}
				if e := tx.Where("code=?", subcode).FirstOrCreate(&sub).Error; e != nil {
					return e
				}
				targets[subcode] = sub
			}
		}
		var old []Category
		if e := tx.Where("hall_id=1 AND category_kind='product'").Order("id").Find(&old).Error; e != nil {
			return e
		}
		mapping := compactMappingCodes()
		byID := map[uint64]Category{}
		destinations := map[uint64]uint64{}
		for _, cat := range old {
			byID[cat.ID] = cat
			if cat.Code != nil {
				if target, ok := mapping[*cat.Code]; ok {
					destinations[cat.ID] = targets[target].ID
				}
			}
		}
		// Resolve every existing merge before adding new aliases, preserving admin intent.
		effective := map[uint64]uint64{}
		for source, target := range destinations {
			canonical := canonicalCategory(tx, source)
			if canonical != source {
				if mapped, ok := destinations[canonical]; ok {
					target = mapped
				} else {
					target = canonical
				}
			}
			if target == 0 {
				return fmt.Errorf("invalid_category")
			}
			effective[source] = target
		}
		for _, cat := range old {
			target, ok := effective[cat.ID]
			if !ok {
				continue
			}
			var to Category
			if e := tx.First(&to, target).Error; e != nil {
				return e
			}
			aliases := joinedCategoryAliases(to.SearchAliases, cat.Name, cat.NameEN, cat.SearchAliases)
			if e := tx.Model(&to).Update("search_aliases", aliases).Error; e != nil {
				return e
			}
			if e := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "source_id"}}, DoUpdates: clause.AssignmentColumns([]string{"target_id"})}).Create(&CategoryAlias{SourceID: cat.ID, TargetID: target}).Error; e != nil {
				return e
			}
			if e := tx.Model(&cat).Update("enabled", false).Error; e != nil {
				return e
			}
			if e := recordOperation(tx, 0, "category", cat.ID, "category_compacted", fmt.Sprintf("schema=18; target=%d", target)); e != nil {
				return e
			}
		}
		// Keep custom subtrees when their known parent can move without exceeding level 3.
		unresolved := []uint64{}
		for _, cat := range old {
			if _, seed := effective[cat.ID]; seed {
				continue
			}
			target, move := effective[cat.ParentID]
			if !move {
				continue
			}
			candidate := cat
			candidate.ParentID = target
			if e := validateCategoryTree(tx, candidate); e != nil {
				unresolved = append(unresolved, cat.ID)
				continue
			}
			if e := tx.Model(&cat).Update("parent_id", target).Error; e != nil {
				return e
			}
			if e := recordOperation(tx, 0, "category", cat.ID, "category_reparented", fmt.Sprintf("schema=18; parent=%d", target)); e != nil {
				return e
			}
		}
		// Flatten pre-existing alias chains so SQL search/counts and old clients agree.
		var aliases []CategoryAlias
		if e := tx.Find(&aliases).Error; e != nil {
			return e
		}
		for _, alias := range aliases {
			final := canonicalCategory(tx, alias.SourceID)
			if final == 0 {
				return fmt.Errorf("invalid_category")
			}
			if e := tx.Model(&alias).Update("target_id", final).Error; e != nil {
				return e
			}
		}
		for _, parent := range unresolved {
			branch := []uint64{parent}
			for i := 0; i < len(branch); i++ {
				for _, cat := range old {
					if cat.ParentID == branch[i] {
						branch = append(branch, cat.ID)
					}
				}
			}
			var refs []RevisionCategory
			if e := tx.Where("category_id IN ?", branch).Find(&refs).Error; e != nil {
				return e
			}
			for _, ref := range refs {
				var rev Revision
				var content Content
				if e := tx.First(&rev, ref.RevisionID).Error; e != nil {
					return e
				}
				if e := tx.First(&content, rev.ContentID).Error; e != nil {
					return e
				}
				row := TaxonomyMapping{RevisionID: rev.ID, ContentID: content.ID, CategoryKind: "product", LegacyCategoryID: ref.CategoryID, Status: "pending", UpdatedAt: time.Now().UTC()}
				if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; e != nil {
					return e
				}
			}
		}
		return tx.Create(&SystemSetting{SettingKey: "taxonomy_compact_v18", Value: `{"complete":true}`, UpdatedAt: time.Now().UTC()}).Error
	})
}

func canonicalAliasID(links map[uint64]uint64, id uint64) uint64 {
	seen := map[uint64]bool{}
	for links[id] > 0 {
		if seen[id] {
			return 0
		}
		seen[id] = true
		id = links[id]
	}
	return id
}

// Expand aliases and descendants in memory; no query per historical category.
func categoryGraphIDs(db *gorm.DB, roots []uint64) []uint64 {
	var cats []Category
	var aliases []CategoryAlias
	db.Find(&cats)
	db.Find(&aliases)
	next := map[uint64]uint64{}
	for _, a := range aliases {
		next[a.SourceID] = a.TargetID
	}
	canonical := func(id uint64) uint64 {
		seen := map[uint64]bool{}
		for next[id] > 0 {
			if seen[id] {
				return 0
			}
			seen[id] = true
			id = next[id]
		}
		return id
	}
	selected := map[uint64]bool{}
	queue := []uint64{}
	for _, root := range roots {
		if id := canonical(root); id > 0 && !selected[id] {
			selected[id] = true
			queue = append(queue, id)
		}
	}
	for i := 0; i < len(queue); i++ {
		for _, cat := range cats {
			if cat.ParentID == queue[i] && canonical(cat.ID) == cat.ID && !selected[cat.ID] {
				selected[cat.ID] = true
				queue = append(queue, cat.ID)
			}
		}
	}
	for _, cat := range cats {
		if selected[canonical(cat.ID)] {
			selected[cat.ID] = true
		}
	}
	result := []uint64{}
	for id := range selected {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func categoryKeywordIDs(db *gorm.DB, keyword string) []uint64 {
	var ids []uint64
	db.Model(&Category{}).Where("category_kind='product' AND deleted_at IS NULL AND (name LIKE ? OR name_en LIKE ? OR search_aliases LIKE ?)", keyword, keyword, keyword).Pluck("id", &ids)
	return categoryGraphIDs(db, ids)
}
