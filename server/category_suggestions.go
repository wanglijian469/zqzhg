package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strconv"
	"strings"
	"time"
)

type CategorySuggestion struct {
	CategoryID uint64    `gorm:"primaryKey" json:"category_id"`
	OwnerID    uint64    `json:"owner_id"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
type CategoryAlias struct {
	SourceID  uint64    `gorm:"primaryKey" json:"source_id"`
	TargetID  uint64    `json:"target_id"`
	CreatedAt time.Time `json:"-"`
}

func (a *App) categoryAliases() []CategoryAlias {
	aliases := []CategoryAlias{}
	a.DB.Find(&aliases)
	for i := range aliases {
		aliases[i].TargetID = canonicalCategory(a.DB, aliases[i].TargetID)
	}
	return aliases
}

func categoryDescends(tx *gorm.DB, node, ancestor uint64) bool {
	seen := map[uint64]bool{}
	for node > 0 {
		if node == ancestor || seen[node] {
			return true
		}
		seen[node] = true
		var c Category
		if tx.First(&c, node).Error != nil {
			return false
		}
		node = c.ParentID
	}
	return false
}

type ScopedCategory struct {
	Category
	Status         string `json:"suggestion_status,omitempty"`
	OwnerID        uint64 `json:"owner_id,omitempty"`
	ReferenceCount int64  `json:"reference_count,omitempty"`
	VendorName     string `json:"vendor_name,omitempty"`
}

func canonicalCategory(tx *gorm.DB, id uint64) uint64 {
	seen := map[uint64]bool{}
	for id > 0 && !seen[id] {
		seen[id] = true
		var alias CategoryAlias
		tx.Where("source_id=?", id).Limit(1).Find(&alias)
		if alias.TargetID == 0 {
			return id
		}
		id = alias.TargetID
	}
	return 0
}
func categoryAllowed(tx *gorm.DB, id, owner uint64) bool {
	var cat Category
	if tx.First(&cat, id).Error != nil || !cat.Enabled {
		return false
	}
	var s CategorySuggestion
	tx.Where("category_id=?", id).Limit(1).Find(&s)
	return s.CategoryID == 0 || s.Status == "approved" || (s.Status == "pending" && s.OwnerID == owner)
}
func (a *App) scopedCategories(who Account, public bool) []ScopedCategory {
	var all []Category
	a.DB.Order("sort_order,id").Find(&all)
	var suggestions []CategorySuggestion
	a.DB.Find(&suggestions)
	meta := map[uint64]CategorySuggestion{}
	for _, s := range suggestions {
		meta[s.CategoryID] = s
	}
	result := []ScopedCategory{}
	for _, cat := range all {
		s := meta[cat.ID]
		if s.Status == "merged" {
			continue
		}
		if public && (!cat.Enabled || (s.CategoryID > 0 && s.Status != "approved")) {
			continue
		}
		if !public && who.Role != "admin" && s.CategoryID > 0 && s.Status != "approved" && s.OwnerID != who.ID {
			continue
		}
		r := ScopedCategory{Category: cat}
		if !public {
			sCopy := s
			r.Status = sCopy.Status
			if who.Role == "admin" {
				r.OwnerID = s.OwnerID
				a.DB.Model(&Content{}).Joins("JOIN revisions r ON r.id=contents.draft_revision_id OR r.id=contents.published_revision_id").Where("r.category_id=?", cat.ID).Distinct("contents.id").Count(&r.ReferenceCount)
				if s.OwnerID > 0 {
					var row struct{ Name string }
					a.DB.Table("contents c").Joins("JOIN revisions r ON r.id=c.draft_revision_id").Where("c.owner_id=? AND c.kind='vendor'", s.OwnerID).Select("r.name").Limit(1).Scan(&row)
					r.VendorName = row.Name
				}
			}
		}
		result = append(result, r)
	}
	return result
}
func (a *App) suggestCategory(c *gin.Context) {
	var req struct {
		Name     string `json:"name"`
		NameEN   string `json:"name_en"`
		ParentID uint64 `json:"parent_id"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.NameEN = strings.TrimSpace(req.NameEN)
	if req.Name == "" {
		dbFail(c, fieldFailure("invalid_request", "name", "required"))
		return
	}
	for field, value := range map[string]string{"name": req.Name, "name_en": req.NameEN} {
		if len(value) > 300 {
			dbFail(c, &FieldValidationError{Code: "invalid_request", Fields: []FieldIssue{{field, "too_long", 300, "bytes"}}})
			return
		}
	}
	who := actor(c)
	var cat Category
	var status string
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		var lock Account
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lock, who.ID).Error; e != nil {
			return e
		}
		// Root categories belong to the same default hall as administrator-created categories.
		parent := Category{HallID: 1}
		if req.ParentID == 0 {
			var hall Hall
			if e := tx.First(&hall, parent.HallID).Error; e != nil {
				return e
			}
		} else if !categoryAllowed(tx, req.ParentID, 0) || tx.First(&parent, req.ParentID).Error != nil {
			return fieldFailure("invalid_category", "parent_id", "formal_parent_required")
		}
		var matches []Category
		if e := tx.Where("hall_id=? AND parent_id=? AND name=? AND enabled=1", parent.HallID, req.ParentID, req.Name).Find(&matches).Error; e != nil {
			return e
		}
		for _, v := range matches {
			if categoryAllowed(tx, v.ID, who.ID) {
				cat = v
				var m CategorySuggestion
				tx.Where("category_id=?", v.ID).Limit(1).Find(&m)
				status = m.Status
				return nil
			}
		}
		cat = Category{HallID: parent.HallID, ParentID: parent.ID, Name: req.Name, NameEN: req.NameEN, Enabled: true}
		if e := tx.Create(&cat).Error; e != nil {
			return e
		}
		if e := tx.Create(&CategorySuggestion{CategoryID: cat.ID, OwnerID: who.ID, Status: "pending"}).Error; e != nil {
			return e
		}
		status = "pending"
		return recordOperation(tx, who.ID, "category", cat.ID, "category_suggested", "")
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(201, ScopedCategory{Category: cat, Status: status})
}
func (a *App) normalizeCategory(c *gin.Context) {
	var req struct {
		Action   string `json:"action"`
		Name     string `json:"name"`
		NameEN   string `json:"name_en"`
		ParentID uint64 `json:"parent_id"`
		TargetID uint64 `json:"target_id"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		var s CategorySuggestion
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&s, "category_id=?", id(c)).Error != nil {
			return gorm.ErrRecordNotFound
		}
		if s.Status == "merged" {
			return fmt.Errorf("stale_revision")
		}
		var cat Category
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&cat, s.CategoryID).Error; e != nil {
			return e
		}
		switch req.Action {
		case "approve":
			if s.Status != "pending" && s.Status != "disabled" {
				return fmt.Errorf("stale_revision")
			}
			req.Name = strings.TrimSpace(req.Name)
			if req.Name == "" {
				return fieldFailure("invalid_request", "name", "required")
			}
			for field, value := range map[string]string{"name": req.Name, "name_en": req.NameEN} {
				if len(value) > 300 {
					return &FieldValidationError{Code: "invalid_request", Fields: []FieldIssue{{field, "too_long", 300, "bytes"}}}
				}
			}
			if req.ParentID != 0 {
				if categoryDescends(tx, req.ParentID, cat.ID) || !categoryAllowed(tx, req.ParentID, 0) {
					return fieldFailure("invalid_category", "parent_id", "formal_parent_required")
				}
				var parent Category
				if e := tx.First(&parent, req.ParentID).Error; e != nil {
					return e
				}
				if parent.HallID != cat.HallID {
					return fmt.Errorf("invalid_category")
				}
			}
			if e := tx.Model(&cat).Updates(map[string]any{"name": req.Name, "name_en": req.NameEN, "parent_id": req.ParentID, "enabled": true}).Error; e != nil {
				return e
			}
			s.Status = "approved"
		case "merge":
			if s.Status != "pending" && s.Status != "disabled" {
				return fmt.Errorf("stale_revision")
			}
			target := canonicalCategory(tx, req.TargetID)
			if categoryDescends(tx, target, cat.ID) || !categoryAllowed(tx, target, 0) {
				return fieldFailure("invalid_category", "target_id", "formal_target_required")
			}
			var t Category
			tx.First(&t, target)
			if t.HallID != cat.HallID {
				return fmt.Errorf("invalid_category")
			}
			if e := tx.Create(&CategoryAlias{SourceID: cat.ID, TargetID: target}).Error; e != nil {
				return e
			}
			if e := tx.Model(&Category{}).Where("parent_id=?", cat.ID).Update("parent_id", target).Error; e != nil {
				return e
			}
			if e := tx.Model(&cat).Update("enabled", false).Error; e != nil {
				return e
			}
			s.Status = "merged"
		case "disable":
			if e := tx.Model(&cat).Update("enabled", false).Error; e != nil {
				return e
			}
			s.Status = "disabled"
		default:
			return fmt.Errorf("invalid_request")
		}
		if e := tx.Save(&s).Error; e != nil {
			return e
		}
		return recordOperation(tx, actor(c).ID, "category", cat.ID, "category_"+req.Action, strconv.FormatUint(req.TargetID, 10))
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func (a *App) resolveCategoryViews(items []View) {
	for i := range items {
		items[i].Data.CategoryID = canonicalCategory(a.DB, items[i].Data.CategoryID)
	}
}
