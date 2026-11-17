package main

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"gorm.io/gorm"
)

func aiProductIdentity(p Payload) string {
	models := []string{}
	if p.ProductDetails != nil {
		for _, m := range p.ProductDetails.Models {
			models = append(models, m.ModelName+"\x00"+m.Variant)
		}
	} else if p.Model != "" {
		models = append(models, p.Model+"\x00")
	}
	sort.Strings(models)
	return strings.TrimSpace(p.Name) + "\x01" + strings.Join(models, "\x02")
}

// Exact identity only. Ambiguous names/models never select another product.
func (a *App) aiSuggestProductUpdate(task AIImportTask, p *Payload, d *AIImportDraft) error {
	var rows []Content
	if e := a.DB.Where("owner_id=? AND kind='product'", task.OwnerID).Find(&rows).Error; e != nil {
		return e
	}
	matches := []View{}
	for _, row := range rows {
		var rev Revision
		if e := a.DB.First(&rev, row.DraftRevisionID).Error; e != nil {
			return e
		}
		v := a.privateView(row, rev)
		if aiProductIdentity(v.Data) == aiProductIdentity(*p) {
			matches = append(matches, v)
		}
	}
	if len(matches) != 1 {
		return nil
	}
	target := matches[0]
	d.TargetContentID = target.ID
	d.BaseRevisionID = target.RevisionID
	d.PublishMode = "undecided"
	p.CategoryID = target.Data.CategoryID
	p.ProductCategoryID = target.Data.ProductCategoryID
	p.VendorProductCategoryID = target.Data.VendorProductCategoryID
	var issues []string
	json.Unmarshal([]byte(d.ReviewIssues), &issues)
	issues = append(issues, "检测到本企业已有同名同型号产品，请明确选择更新现有产品或另建产品")
	d.ReviewIssues = aiJSON(issues)
	d.Data = aiJSON(p)
	return nil
}

// Read-only preflight runs before a publish manifest is frozen, so missing
// categories/images/agreements leave all drafts editable and all business data unchanged.
func (a *App) aiValidateReady(ctx context.Context, tx *gorm.DB, who Account, task AIImportTask, d AIImportDraft, kind string) error {
	var p Payload
	if e := json.Unmarshal([]byte(d.Data), &p); e != nil {
		return serviceFailure{"invalid_request", 400}
	}
	content := Content{Kind: "product", OwnerID: who.ID, VendorID: task.VendorID, HallID: 1, TaxonomyRequired: true}
	if kind == "company" {
		if d.ContentID > 0 {
			return nil
		}
		content.Kind = "vendor"
		var existing Content
		e := tx.Where("owner_id=? AND kind='vendor'", who.ID).First(&existing).Error
		if e == nil {
			var fields []string
			json.Unmarshal([]byte(d.SelectedFields), &fields)
			if len(fields) == 0 {
				return nil
			}
			if existing.DraftRevisionID != d.BaseRevisionID {
				return serviceFailure{"stale_revision", 409}
			}
			var rev Revision
			if e = tx.First(&rev, existing.DraftRevisionID).Error; e != nil {
				return e
			}
			if rev.Status == "pending" {
				return serviceFailure{"pending_review", 409}
			}
			var old Payload
			json.Unmarshal([]byte(rev.Payload), &old)
			p, e = aiMergeCompany(old, p, fields)
			if e != nil {
				return e
			}
			content = existing
			views := []View{view(existing, rev)}
			if e = (&App{DB: tx}).hydrateTaxonomyViews(views); e != nil {
				return e
			}
			if p.PrimaryCategoryID == 0 {
				p.PrimaryCategoryID = views[0].Data.PrimaryCategoryID
			}
		}
		if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		if p.PrimaryCategoryID == 0 {
			return fieldFailure("invalid_category", "primary_category_id", "required")
		}
		if e := checkVendorName(tx, content, p.Name); e != nil {
			return e
		}
	} else {
		if task.VendorID > 0 {
			var vendor Content
			if e := tx.Where("id=? AND owner_id=? AND kind='vendor'", task.VendorID, who.ID).First(&vendor).Error; e != nil {
				return e
			}
			content.HallID = vendor.HallID
		}
		if d.PublishMode == "undecided" {
			return serviceFailure{"ai_update_choice_required", 409}
		}
		if d.PublishMode == "update" {
			var existing Content
			if e := tx.Where("id=? AND owner_id=? AND kind='product'", d.TargetContentID, who.ID).First(&existing).Error; e != nil {
				return e
			}
			if existing.DraftRevisionID != d.BaseRevisionID {
				return serviceFailure{"stale_revision", 409}
			}
			var rev Revision
			tx.First(&rev, existing.DraftRevisionID)
			if rev.Status == "pending" {
				return serviceFailure{"pending_review", 409}
			}
			content = existing
		}
		if len(p.MediaIDs) == 0 {
			return fieldFailure("invalid_request", "media_ids", "required")
		}
		if p.ProductCategoryID == 0 && p.CategoryID == 0 && p.VendorProductCategoryID == 0 {
			return fieldFailure("invalid_category", "product_category_id", "required")
		}
		if p.VendorProductCategoryID > 0 {
			var category VendorProductCategory
			if tx.Where("id=? AND vendor_id=?", p.VendorProductCategoryID, content.VendorID).First(&category).Error != nil {
				return fieldFailure("invalid_category", "vendor_product_category_id", "category_unavailable")
			}
		}
	}
	if e := a.validatePayload(tx, who, content.Kind, &p, p); e != nil {
		return e
	}
	if e := validateContentHall(tx, &content, p); e != nil {
		return e
	}
	if e := validateTaxonomy(tx, content, &p); e != nil {
		return e
	}
	if e := a.aiValidateTaskMedia(tx, task.ID, p); e != nil {
		return e
	}
	for _, mid := range payloadMedia(p) {
		var m Media
		var meta MediaMetadata
		if e := tx.First(&m, mid).Error; e != nil {
			return e
		}
		if tx.First(&meta, "media_id=?", mid).Error == nil && meta.State != "ready" {
			return serviceFailure{"media_not_ready", 409}
		}
		obj, store, e := a.resolveObject(ctx, m.Path)
		if e != nil {
			return serviceFailure{"media_not_ready", 409}
		}
		stat, e := store.Stat(ctx, m.Path)
		if e != nil || stat.Size != obj.Size {
			return serviceFailure{"media_not_ready", 409}
		}
	}
	var vendorPayload *Payload
	if content.Kind == "vendor" {
		vendorPayload = &p
	}
	return publicationCompliance(tx, who.ID, content.VendorID, vendorPayload)
}
