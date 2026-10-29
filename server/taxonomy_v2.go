package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type RevisionCategory struct {
	RevisionID uint64 `json:"revision_id" gorm:"primaryKey"`
	CategoryID uint64 `json:"category_id" gorm:"primaryKey"`
	Role       string `json:"role"`
	Source     string `json:"source"`
}
type TaxonomyMapping struct {
	RevisionID       uint64    `json:"revision_id" gorm:"primaryKey"`
	ContentID        uint64    `json:"content_id"`
	CategoryKind     string    `json:"category_kind"`
	LegacyCategoryID uint64    `json:"legacy_category_id"`
	TargetID         uint64    `json:"target_id"`
	Status           string    `json:"status"`
	UpdatedAt        time.Time `json:"updated_at"`
}
type IndustrialBelt struct {
	ID        uint64 `json:"id"`
	HallID    uint64 `json:"hall_id"`
	Name      string `json:"name"`
	NameEN    string `json:"name_en"`
	SortOrder int    `json:"sort_order"`
	Enabled   bool   `json:"enabled"`
}
type VendorIndustrialBelt struct {
	VendorID uint64 `json:"vendor_id" gorm:"primaryKey"`
	BeltID   uint64 `json:"belt_id" gorm:"primaryKey"`
}

func effectiveViewCategory(v View) uint64 {
	if v.Kind == "vendor" && v.Data.PrimaryCategoryID > 0 {
		return v.Data.PrimaryCategoryID
	}
	if v.Kind == "product" && v.Data.ProductCategoryID > 0 {
		return v.Data.ProductCategoryID
	}
	return v.Data.CategoryID
}

func (p *Payload) UnmarshalJSON(data []byte) error {
	type plain Payload
	var value plain
	if e := json.Unmarshal(data, &value); e != nil {
		return e
	}
	*p = Payload(value)
	var raw map[string]json.RawMessage
	if e := json.Unmarshal(data, &raw); e != nil {
		return e
	}
	p.taxonomyFields = map[string]bool{}
	for k := range raw {
		p.taxonomyFields[k] = true
	}
	return nil
}
func mergeTaxonomyFields(next *Payload, old Payload) {
	has := next.taxonomyFields
	if has == nil {
		return
	} // Internal Go callers provide an entire payload.
	if !has["primary_category_id"] {
		next.PrimaryCategoryID = old.PrimaryCategoryID
	}
	if !has["secondary_category_ids"] {
		next.SecondaryCategoryIDs = old.SecondaryCategoryIDs
	}
	if !has["business_nature"] {
		next.BusinessNature = old.BusinessNature
	}
	if !has["product_category_id"] {
		next.ProductCategoryID = old.ProductCategoryID
	}
	if !has["brand"] {
		next.Brand = old.Brand
	}
	if !has["model"] {
		next.Model = old.Model
	}
	if !has["applicable_brands"] {
		next.ApplicableBrands = old.ApplicableBrands
	}
	if !has["applicable_models"] {
		next.ApplicableModels = old.ApplicableModels
	}
	next.IndustrialBeltIDs = nil // Administrator-managed, never a merchant payload write.
}

func selectableCategory(tx *gorm.DB, id uint64, kind string, hall uint64, owner uint64, leaf bool) bool {
	id = canonicalCategory(tx, id)
	var c Category
	if id == 0 || tx.First(&c, id).Error != nil || c.CategoryKind != kind || c.HallID != hall || !categoryAllowed(tx, id, owner) || c.DeletedAt != nil {
		return false
	}
	seen := map[uint64]bool{c.ID: true}
	for c.ParentID > 0 {
		parentID := c.ParentID
		c = Category{}
		if seen[parentID] || tx.First(&c, parentID).Error != nil || c.CategoryKind != kind || c.HallID != hall || !c.Enabled || c.DeletedAt != nil {
			return false
		}
		seen[c.ID] = true
	}
	if leaf {
		var n int64
		tx.Table("categories lc").Joins("LEFT JOIN category_suggestions ls ON ls.category_id=lc.id").Where("lc.parent_id=? AND lc.deleted_at IS NULL AND (ls.category_id IS NULL OR ls.status='approved' OR (ls.status='pending' AND ls.owner_id=?))", id, owner).Count(&n)
		if n > 0 {
			return false
		}
	}
	return true
}
func validateTaxonomy(tx *gorm.DB, content Content, p *Payload) error {
	if p.CompanyProfile != nil && procurementEnabled() {
		for _, category := range p.CompanyProfile.SupplyProductCategoryIDs {
			if !selectableCategory(tx, canonicalCategory(tx, category), "product", content.HallID, 0, false) {
				return fieldFailure("invalid_category", "company_profile.supply_product_category_ids", "category_unavailable")
			}
		}
	}
	if content.HallID != 1 {
		return nil
	}
	if len(p.SecondaryCategoryIDs) > 2 {
		return &FieldValidationError{"invalid_request", []FieldIssue{{"secondary_category_ids", "too_many", 2, "items"}}}
	}
	if content.Kind == "product" && (p.PrimaryCategoryID != 0 || len(p.SecondaryCategoryIDs) > 0 || p.BusinessNature != "") {
		return fieldFailure("invalid_request", "primary_category_id", "category_unavailable")
	}
	if content.Kind == "vendor" && p.ProductCategoryID != 0 {
		return fieldFailure("invalid_request", "product_category_id", "category_unavailable")
	}
	validNature := map[string]bool{"": true, "manufacturer": true, "dealer": true, "manufacturing_trading": true, "processing": true, "other": true}
	if !validNature[p.BusinessNature] {
		return fieldFailure("invalid_request", "business_nature", "invalid")
	}
	for field, value := range map[string]string{"brand": p.Brand, "model": p.Model} {
		if utf8.RuneCountInString(value) > 200 {
			return &FieldValidationError{"invalid_request", []FieldIssue{{field, "too_long", 200, "characters"}}}
		}
	}
	for field, values := range map[string][]string{"applicable_brands": p.ApplicableBrands, "applicable_models": p.ApplicableModels} {
		if len(values) > 30 {
			return &FieldValidationError{"invalid_request", []FieldIssue{{field, "too_many", 30, "items"}}}
		}
		for _, v := range values {
			if strings.TrimSpace(v) == "" || utf8.RuneCountInString(v) > 200 {
				return fieldFailure("invalid_request", field, "invalid")
			}
		}
	}
	// Unchanged historical assignments may be retained even after a category is stopped.
	retained := map[uint64]bool{}
	if content.DraftRevisionID > 0 {
		var rows []RevisionCategory
		if e := tx.Where("revision_id=?", content.DraftRevisionID).Find(&rows).Error; e != nil {
			return e
		}
		for _, r := range rows {
			retained[canonicalCategory(tx, r.CategoryID)] = true
		}
	}
	seen := map[uint64]bool{}
	check := func(id *uint64, kind, field string, leaf bool) error {
		if *id == 0 {
			return nil
		}
		*id = canonicalCategory(tx, *id)
		if seen[*id] || (!retained[*id] && !selectableCategory(tx, *id, kind, content.HallID, content.OwnerID, leaf)) {
			return fieldFailure("invalid_category", field, "category_unavailable")
		}
		var c Category
		if tx.First(&c, *id).Error != nil || c.CategoryKind != kind || c.HallID != content.HallID {
			return fieldFailure("invalid_category", field, "category_unavailable")
		}
		seen[*id] = true
		return nil
	}
	if e := check(&p.PrimaryCategoryID, "vendor", "primary_category_id", false); e != nil {
		return e
	}
	for i := range p.SecondaryCategoryIDs {
		if p.SecondaryCategoryIDs[i] == 0 {
			return fieldFailure("invalid_category", "secondary_category_ids", "category_unavailable")
		}
		if e := check(&p.SecondaryCategoryIDs[i], "vendor", "secondary_category_ids", false); e != nil {
			return e
		}
	}
	return check(&p.ProductCategoryID, "product", "product_category_id", true)
}

func exactLegacyTarget(tx *gorm.DB, kind string, hall, legacy uint64) uint64 {
	if legacy == 0 {
		return 0
	}
	var old Category
	if tx.First(&old, canonicalCategory(tx, legacy)).Error != nil {
		return 0
	}
	if old.CategoryKind == kind {
		if selectableCategory(tx, old.ID, kind, hall, 0, kind == "product") {
			return old.ID
		}
		return 0
	}
	if old.CategoryKind != "legacy" {
		return 0
	}
	name := old.Name
	if kind == "product" {
		name = map[string]string{"翻转犁": "铧式犁", "驱动耙": "耙地机械", "齿轮": "齿轮与齿轮轴"}[old.Name]
		if name == "" {
			name = old.Name
		}
	}
	// Broad legacy labels cannot identify a manufacturer's main business.
	if old.Name == "农业机械" || old.Name == "农机配件" {
		return 0
	}
	var rows []Category
	if tx.Where("hall_id=? AND category_kind=? AND name=? AND deleted_at IS NULL", hall, kind, name).Find(&rows).Error != nil {
		return 0
	}
	targets := map[uint64]bool{}
	for _, row := range rows {
		target := canonicalCategory(tx, row.ID)
		if selectableCategory(tx, target, kind, hall, 0, kind == "product") {
			targets[target] = true
		}
	}
	if len(targets) == 1 {
		for target := range targets {
			return target
		}
	}
	return 0
}
func indexTaxonomyRevision(tx *gorm.DB, content Content, rev Revision, p Payload, legacy bool) error {
	rows := []RevisionCategory{}
	source := "explicit"
	primary := p.PrimaryCategoryID
	role := "primary"
	if content.Kind == "product" {
		primary = p.ProductCategoryID
		role = "product"
	}
	field := "primary_category_id"
	if content.Kind == "product" {
		field = "product_category_id"
	}
	if primary == 0 && (legacy || !p.taxonomyFields[field]) {
		primary = exactLegacyTarget(tx, content.Kind, content.HallID, p.CategoryID)
		source = "automatic"
	}
	if primary > 0 {
		rows = append(rows, RevisionCategory{rev.ID, primary, role, source})
	}
	for _, id := range p.SecondaryCategoryIDs {
		if id > 0 {
			rows = append(rows, RevisionCategory{rev.ID, id, "secondary", "explicit"})
		}
	}
	if len(rows) > 0 {
		if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; e != nil {
			return e
		}
	}
	if legacy || (p.CategoryID > 0 && (source == "automatic" || primary == 0)) || (!content.TaxonomyRequired && primary == 0) {
		status := "pending"
		if primary > 0 {
			status = "automatic"
		}
		row := TaxonomyMapping{RevisionID: rev.ID, ContentID: content.ID, CategoryKind: content.Kind, LegacyCategoryID: p.CategoryID, TargetID: primary, Status: status, UpdatedAt: time.Now().UTC()}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
	}
	return nil
}
func initializeAgricultureTaxonomy(db *gorm.DB) error {
	if e := db.Transaction(seedAgricultureTaxonomy); e != nil {
		return e
	}
	// Keyset pagination keeps large historical databases bounded and retryable.
	var after uint64
	for {
		var rows []Revision
		if e := db.Joins("JOIN contents tc ON tc.id=revisions.content_id AND tc.hall_id=1").Where("revisions.id>?", after).Select("revisions.*").Order("revisions.id").Limit(250).Find(&rows).Error; e != nil {
			return e
		}
		if len(rows) == 0 {
			break
		}
		if e := db.Transaction(func(tx *gorm.DB) error {
			for _, rev := range rows {
				var n int64
				if e := tx.Model(&TaxonomyMapping{}).Where("revision_id=?", rev.ID).Count(&n).Error; e != nil {
					return e
				}
				if n > 0 {
					continue
				}
				var content Content
				if e := tx.First(&content, rev.ContentID).Error; e != nil {
					return e
				}
				var p Payload
				if e := json.Unmarshal([]byte(rev.Payload), &p); e != nil {
					return e
				}
				if e := indexTaxonomyRevision(tx, content, rev, p, true); e != nil {
					return e
				}
			}
			return nil
		}); e != nil {
			return e
		}
		after = rows[len(rows)-1].ID
	}
	return nil
}
func (a *App) hydrateTaxonomyViews(items []View) error {
	if len(items) == 0 {
		return nil
	}
	revIDs := []uint64{}
	vendorIDs := []uint64{}
	for _, item := range items {
		revIDs = append(revIDs, item.RevisionID)
		if item.Kind == "vendor" {
			vendorIDs = append(vendorIDs, item.ID)
		} else {
			vendorIDs = append(vendorIDs, item.VendorID)
		}
	}
	var rows []RevisionCategory
	if e := a.DB.Where("revision_id IN ?", revIDs).Order("role,category_id").Find(&rows).Error; e != nil {
		return e
	}
	byRev := map[uint64][]RevisionCategory{}
	for _, r := range rows {
		byRev[r.RevisionID] = append(byRev[r.RevisionID], r)
	}
	var memberships []VendorIndustrialBelt
	if e := a.DB.Table("vendor_industrial_belts vib").Joins("JOIN industrial_belts ib ON ib.id=vib.belt_id AND ib.enabled=1").Where("vib.vendor_id IN ?", vendorIDs).Select("vib.*").Find(&memberships).Error; e != nil {
		return e
	}
	belts := map[uint64][]uint64{}
	for _, r := range memberships {
		belts[r.VendorID] = append(belts[r.VendorID], r.BeltID)
	}
	for i := range items {
		item := &items[i]
		seen := map[uint64]bool{}
		item.Data.SecondaryCategoryIDs = []uint64{}
		for _, r := range byRev[item.RevisionID] {
			id := canonicalCategory(a.DB, r.CategoryID)
			if seen[id] {
				continue
			}
			seen[id] = true
			switch r.Role {
			case "primary":
				item.Data.PrimaryCategoryID = id
			case "product":
				item.Data.ProductCategoryID = id
			case "secondary":
				item.Data.SecondaryCategoryIDs = append(item.Data.SecondaryCategoryIDs, id)
			}
		}
		vendor := item.ID
		if item.Kind == "product" {
			vendor = item.VendorID
		}
		item.Data.IndustrialBeltIDs = belts[vendor]
	}
	return nil
}
func submitTaxonomy(tx *gorm.DB, content Content, rev Revision) error {
	if !content.TaxonomyRequired || content.HallID != 1 {
		return nil
	}
	var n int64
	role := "primary"
	field := "primary_category_id"
	if content.Kind == "product" {
		role = "product"
		field = "product_category_id"
	}
	if e := tx.Model(&RevisionCategory{}).Where("revision_id=? AND role=?", rev.ID, role).Count(&n).Error; e != nil {
		return e
	}
	if n == 0 {
		return fieldFailure("invalid_category", field, "required")
	}
	var rows []RevisionCategory
	if e := tx.Where("revision_id=?", rev.ID).Find(&rows).Error; e != nil {
		return e
	}
	// Leaf validity was checked when the assignment was made. A later tree
	// expansion may retain existing assignments, while new selections use leaves.
	for _, r := range rows {
		if !selectableCategory(tx, r.CategoryID, content.Kind, content.HallID, content.OwnerID, false) {
			var published []RevisionCategory
			if content.PublishedRevisionID > 0 {
				if e := tx.Where("revision_id=? AND role=?", content.PublishedRevisionID, r.Role).Find(&published).Error; e != nil {
					return e
				}
			}
			retained := false
			for _, old := range published {
				if canonicalCategory(tx, old.CategoryID) == canonicalCategory(tx, r.CategoryID) {
					retained = true
					break
				}
			}
			if retained {
				continue
			}
			return fieldFailure("invalid_category", field, "category_unavailable")
		}
	}
	return nil
}

func categoryPath(all []Category, id uint64, english bool) string {
	parts := []string{}
	seen := map[uint64]bool{}
	for id > 0 && !seen[id] {
		seen[id] = true
		found := false
		for _, c := range all {
			if c.ID == id {
				name := c.Name
				if english && c.NameEN != "" {
					name = c.NameEN
				}
				parts = append([]string{name}, parts...)
				id = c.ParentID
				found = true
				break
			}
		}
		if !found {
			break
		}
	}
	return strings.Join(parts, " / ")
}
func categoryBranchEnabled(all []Category, id uint64) bool {
	seen := map[uint64]bool{}
	for id > 0 {
		if seen[id] {
			return false
		}
		seen[id] = true
		found := false
		for _, c := range all {
			if c.ID == id {
				if !c.Enabled || c.DeletedAt != nil {
					return false
				}
				id = c.ParentID
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
func (a *App) recommendCategories(c *gin.Context) {
	var req struct {
		HallID   uint64 `json:"hall_id"`
		Name     string `json:"name"`
		Keywords string `json:"keywords"`
	}
	if c.ShouldBindJSON(&req) != nil || utf8.RuneCountInString(req.Name) > 200 || utf8.RuneCountInString(req.Keywords) > 500 {
		fail(c, 400, "invalid_request")
		return
	}
	if req.HallID == 0 {
		req.HallID = 1
	}
	var cats []Category
	if e := a.DB.Where("hall_id=? AND category_kind='product' AND deleted_at IS NULL", req.HallID).Order("sort_order,id").Find(&cats).Error; e != nil {
		dbFail(c, e)
		return
	}
	type recommendation struct {
		Category
		Path   string `json:"path"`
		PathEN string `json:"path_en"`
		Score  int    `json:"-"`
	}
	result := []recommendation{}
	name := strings.ToLower(strings.TrimSpace(req.Name))
	keywords := strings.ToLower(req.Keywords)
	for _, cat := range cats {
		if !selectableCategory(a.DB, cat.ID, "product", req.HallID, 0, true) {
			continue
		}
		score := 0
		for _, word := range append([]string{cat.Name, cat.NameEN}, strings.FieldsFunc(cat.SearchAliases, func(r rune) bool { return r == ',' || r == '，' || r == '\n' })...) {
			word = strings.ToLower(strings.TrimSpace(word))
			if word == "" {
				continue
			}
			s := 0
			if name == word {
				s = 10000 + utf8.RuneCountInString(word)
			} else if strings.Contains(name, word) {
				s = 1000 + utf8.RuneCountInString(word)
			} else if strings.Contains(keywords, word) {
				s = 100 + utf8.RuneCountInString(word)
			}
			if s > score {
				score = s
			}
		}
		if score > 0 {
			result = append(result, recommendation{cat, categoryPath(cats, cat.ID, false), categoryPath(cats, cat.ID, true), score})
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Score > result[j].Score })
	if len(result) > 3 {
		result = result[:3]
	}
	c.JSON(200, gin.H{"items": result})
}

func taxonomyFilter(q *gorm.DB, revisionAlias string, ids []uint64) *gorm.DB {
	return q.Where("EXISTS (SELECT 1 FROM revision_categories rc WHERE rc.revision_id="+revisionAlias+".id AND rc.category_id IN ?)", ids)
}
func (a *App) newCategoryFilter(value, kind string) ([]uint64, bool) {
	if value == "" {
		return nil, false
	}
	var cat Category
	if a.DB.First(&cat, positiveFilterID(value)).Error != nil || cat.CategoryKind == "legacy" {
		return nil, false
	}
	if cat.CategoryKind != kind {
		return []uint64{}, true
	}
	return a.categoryIDs(value), true
}
func filterBelt(c *gin.Context, q *gorm.DB, vendorExpr string) *gorm.DB {
	if value := c.Query("industrial_belt_id"); value != "" {
		q = q.Where("EXISTS (SELECT 1 FROM vendor_industrial_belts vib JOIN industrial_belts ib ON ib.id=vib.belt_id AND ib.enabled=1 WHERE vib.vendor_id="+vendorExpr+" AND vib.belt_id=?)", positiveFilterID(value))
	}
	return q
}

func validateCategoryTree(tx *gorm.DB, req Category) error {
	if req.CategoryKind != "legacy" && req.CategoryKind != "vendor" && req.CategoryKind != "product" {
		return fmt.Errorf("invalid_category")
	}
	if req.CategoryKind == "legacy" {
		return nil
	}
	// Simulate the move, including all descendants, before writing any row.
	var all []Category
	if e := tx.Where("hall_id=? AND category_kind=? AND deleted_at IS NULL", req.HallID, req.CategoryKind).Find(&all).Error; e != nil {
		return e
	}
	byID := map[uint64]Category{}
	for _, v := range all {
		byID[v.ID] = v
	}
	key := req.ID
	if key == 0 {
		key = ^uint64(0)
	}
	req.ID = key
	byID[key] = req
	for _, v := range byID {
		depth := 1
		seen := map[uint64]bool{v.ID: true}
		for v.ParentID > 0 {
			p, ok := byID[v.ParentID]
			if !ok || seen[p.ID] {
				return fmt.Errorf("invalid_category")
			}
			seen[p.ID] = true
			depth++
			v = p
		}
		if depth > 3 {
			return fmt.Errorf("invalid_category")
		}
	}
	return nil
}
