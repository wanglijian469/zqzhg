package main

import (
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (a *App) createOwnedDraft(tx *gorm.DB, content *Content, who Account, p *Payload) error {
	if e := a.validateDraftPayload(tx, who, content.Kind, p); e != nil {
		return e
	}
	if content.Kind == "vendor" {
		hall := Hall{}
		hid := content.HallID
		if hid == 0 {
			hid = 1
		}
		if tx.First(&hall, hid).Error != nil || hall.Status != "open" {
			return fmt.Errorf("invalid_request")
		}
		content.VendorOwnerKey = &who.ID
		var n int64
		tx.Model(&Content{}).Where("vendor_owner_key = ?", who.ID).Count(&n)
		if n > 0 {
			return fmt.Errorf("vendor_required")
		}
	} else {
		var e error
		content.VendorID, e = productSlot(tx, who.ID)
		if e != nil {
			return e
		}
	}
	if e := tx.Create(content).Error; e != nil {
		return e
	}
	if e := a.makeDraft(tx, content, *p); e != nil {
		return e
	}

	return nil
}
func (a *App) submitOwnedDraft(tx *gorm.DB, who Account, contentID, revisionID uint64) error {
	content, e := loadOwned(tx, contentID, who, true)
	if e != nil {
		return e
	}
	if content.DraftRevisionID != revisionID {
		return fmt.Errorf("stale_revision")
	}
	var rev Revision
	if e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&rev, revisionID).Error; e != nil {
		return e
	}
	if rev.Status != "draft" {
		return fmt.Errorf("pending_review")
	}
	if content.Epoch != rev.BaseEpoch {
		return fmt.Errorf("stale_revision")
	}
	var publicationData Payload
	if e = json.Unmarshal([]byte(rev.Payload), &publicationData); e != nil {
		return e
	}
	var vendorData *Payload
	if content.Kind == "vendor" {
		vendorData = &publicationData
	}
	if e = publicationCompliance(tx, content.OwnerID, content.VendorID, vendorData); e != nil {
		return e
	}
	if e = a.validatePayload(tx, Account{ID: content.OwnerID}, content.Kind, &publicationData, publicationData); e != nil {
		return e
	}
	if content.Kind == "product" && len(publicationData.MediaIDs) == 0 {
		return fieldFailure("invalid_request", "media_ids", "required")
	}
	if e = submitTaxonomy(tx, content, rev); e != nil {
		return e
	}
	if e = tx.Model(&rev).Update("status", "pending").Error; e != nil {
		return e
	}
	return recordReview(tx, ReviewLog{ContentID: content.ID, RevisionID: rev.ID, ActorID: who.ID, Action: "submit"})
}

func (a *App) saveOwnedDraft(tx *gorm.DB, target *Content, who Account, contentID, revisionID uint64, p *Payload) error {
	var content Content
	var e error
	content, e = loadOwned(tx, contentID, who, true)
	if e != nil {
		return e
	}
	if content.DraftRevisionID != revisionID {
		return fmt.Errorf("stale_revision")
	}
	var old Revision
	if e = tx.First(&old, content.DraftRevisionID).Error; e != nil {
		return e
	}
	if old.Status == "pending" {
		return fmt.Errorf("pending_review")
	}
	var previousTaxonomy Payload
	if e = json.Unmarshal([]byte(old.Payload), &previousTaxonomy); e != nil {
		return e
	}
	oldView := []View{view(content, old)}
	// Use the transaction to avoid a pool read outside the locked content.
	if e = (&App{DB: tx}).hydrateTaxonomyViews(oldView); e != nil {
		return e
	}
	previousTaxonomy.PrimaryCategoryID = oldView[0].Data.PrimaryCategoryID
	previousTaxonomy.SecondaryCategoryIDs = oldView[0].Data.SecondaryCategoryIDs
	previousTaxonomy.ProductCategoryID = oldView[0].Data.ProductCategoryID
	previousTaxonomy.VendorProductCategoryID = oldView[0].Data.VendorProductCategoryID
	legacyChanged := (p.taxonomyFields["product_category_id"] && canonicalCategory(tx, p.ProductCategoryID) != previousTaxonomy.ProductCategoryID) ||
		(p.taxonomyFields["category_id"] && canonicalCategory(tx, p.CategoryID) != previousTaxonomy.CategoryID)
	mergeTaxonomyFields(p, previousTaxonomy)
	if !p.taxonomyFields["product_details"] {
		p.ProductDetails = previousTaxonomy.ProductDetails
	}
	if !p.taxonomyFields["vendor_product_category_id"] && legacyChanged {
		p.VendorProductCategoryID = 0
	}
	if !content.TaxonomyRequired && ((p.PrimaryCategoryID > 0 && p.PrimaryCategoryID != previousTaxonomy.PrimaryCategoryID) || (p.ProductCategoryID > 0 && p.ProductCategoryID != previousTaxonomy.ProductCategoryID)) {
		content.TaxonomyRequired = true
		if e = tx.Model(&content).Update("taxonomy_required", true).Error; e != nil {
			return e
		}
	}
	if canonicalCategory(tx, p.CategoryID) != canonicalCategory(tx, previousTaxonomy.CategoryID) {
		if !p.taxonomyFields["primary_category_id"] {
			p.PrimaryCategoryID = 0
		}
		if !p.taxonomyFields["product_category_id"] {
			p.ProductCategoryID = 0
		}
	}
	if content.Kind == "vendor" {
		var previous Payload
		if e = json.Unmarshal([]byte(old.Payload), &previous); e != nil {
			return e
		}
		mergePortalProfile(p, previous)
	}
	if e = a.validateDraftPayload(tx, Account{ID: content.OwnerID}, content.Kind, p, previousTaxonomy); e != nil {
		return e
	}
	if e = a.makeDraft(tx, &content, *p); e != nil {
		return e
	}
	*target = content
	if platformAccount(who) {
		return recordOperation(tx, who.ID, content.Kind, content.ID, "draft_saved", "")
	}
	return nil
}
