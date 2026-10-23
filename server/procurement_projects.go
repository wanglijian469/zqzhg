package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type procurementDemandInput struct {
	Version          uint64            `json:"version"`
	HallID           uint64            `json:"hall_id"`
	SourceProductID  uint64            `json:"source_product_id"`
	Title            string            `json:"title"`
	DeliveryRegion   string            `json:"delivery_region"`
	ExpectedDelivery string            `json:"expected_delivery"`
	Deadline         time.Time         `json:"deadline"`
	Notes            string            `json:"notes"`
	ContactName      string            `json:"contact_name"`
	Items            []ProcurementItem `json:"items"`
	AttachmentIDs    []uint64          `json:"attachment_ids"`
	Audience         string            `json:"audience"`
	VendorIDs        []uint64          `json:"vendor_ids"`
}
type procurementActionInput struct {
	Version   uint64   `json:"version"`
	Audience  string   `json:"audience"`
	VendorIDs []uint64 `json:"vendor_ids"`
	State     string   `json:"state"`
	Reason    string   `json:"reason"`
}

func procValidRegion(code string) bool {
	loadAdministrativeRegions()
	_, ok := administrativeByCode[code]
	return ok
}
func (a *App) procValidateDemand(tx *gorm.DB, p ProcurementProject, r ProcurementRevision, items []ProcurementItem, submitting bool) error {
	if !procText(r.Title, 200) || !procText(r.Notes, 10000) || !procText(r.ContactName, 100) || !procText(r.ExpectedDelivery, 100) || len(items) > 30 {
		return procInvalid()
	}
	if r.DeliveryRegion != "" && !procValidRegion(r.DeliveryRegion) {
		return procInvalid()
	}
	if submitting && (strings.TrimSpace(r.Title) == "" || r.DeliveryRegion == "" || len(items) == 0 || !r.Deadline.After(time.Now().UTC()) || r.ContactName == "") {
		return procInvalid()
	}
	for _, i := range items {
		if !procText(i.Name, 200) || !procText(i.Specification, 5000) || !procText(i.Unit, 30) || !procNumber(i.Quantity, 3, true) {
			return procInvalid()
		}
		if submitting && (strings.TrimSpace(i.Name) == "" || strings.TrimSpace(i.Unit) == "" || i.CategoryID == 0) {
			return procInvalid()
		}
		if i.CategoryID > 0 && !selectableCategory(tx, i.CategoryID, "product", p.HallID, 0, false) {
			return procErr("proc_category_invalid", 400)
		}
	}
	var hall Hall
	if e := tx.Where("id=? AND status='open'", p.HallID).First(&hall).Error; e != nil {
		return procInvalid()
	}
	return nil
}
func (a *App) procNewDraft(tx *gorm.DB, who Account, p *ProcurementProject, req procurementDemandInput) (ProcurementRevision, error) {
	r := ProcurementRevision{ProjectID: p.ID, Status: "draft", Title: strings.TrimSpace(req.Title), DeliveryRegion: req.DeliveryRegion, ExpectedDelivery: strings.TrimSpace(req.ExpectedDelivery), Deadline: req.Deadline, Notes: strings.TrimSpace(req.Notes), ContactName: strings.TrimSpace(req.ContactName), CreatedAt: time.Now().UTC()}
	r.RequestedAudience = req.Audience
	if r.RequestedAudience == "" {
		r.RequestedAudience = p.Audience
		if p.PublishedRevisionID == 0 && p.SourceProductID == 0 {
			r.RequestedAudience = "public"
		}
	}
	if !procAudience(r.RequestedAudience) || len(req.VendorIDs) > 20 || r.RequestedAudience == "direct" && p.SourceProductID == 0 {
		return r, procInvalid()
	}
	vendorIDs, _ := json.Marshal(req.VendorIDs)
	r.VendorIDs = string(vendorIDs)
	if who.Phone != nil {
		r.Phone = *who.Phone
	}
	if r.Deadline.IsZero() {
		r.Deadline = time.Now().UTC().Add(7 * 24 * time.Hour)
	}
	if r.ContactName == "" {
		var buyer BuyerProfile
		tx.First(&buyer, "account_id=?", who.ID)
		r.ContactName = buyer.ContactName
	}
	if e := a.procValidateDemand(tx, *p, r, req.Items, false); e != nil {
		return r, e
	}
	if p.DraftRevisionID > 0 && p.DraftRevisionID != p.PublishedRevisionID {
		if e := tx.Model(&ProcurementRevision{}).Where("id=? AND status IN ('draft','pending','rejected')", p.DraftRevisionID).Update("status", "superseded").Error; e != nil {
			return r, e
		}
	}
	if e := tx.Model(&ProcurementReview{}).Where("project_id=? AND status='pending'", p.ID).Update("status", "superseded").Error; e != nil {
		return r, e
	}
	if e := tx.Create(&r).Error; e != nil {
		return r, e
	}
	for _, i := range req.Items {
		i.ID = 0
		i.RevisionID = r.ID
		i.CategoryID = canonicalCategory(tx, i.CategoryID)
		if e := tx.Create(&i).Error; e != nil {
			return r, e
		}
	}
	if e := procBindAttachments(tx, req.AttachmentIDs, p.ID, who.ID, "requirement", r.ID); e != nil {
		return r, e
	}
	p.DraftRevisionID = r.ID
	return r, procTouch(tx, p)
}
func (a *App) procCreateProject(c *gin.Context) {
	var req procurementDemandInput
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "proc_invalid")
		return
	}
	a.procWrite(c, req, "procurement_created", func(tx *gorm.DB, who Account) (any, uint64, error) {
		if _, e := procBuyer(tx, who); e != nil {
			return nil, 0, e
		}
		p := ProcurementProject{BuyerID: who.ID, HallID: req.HallID, State: "draft", Audience: "designated", Version: 0}
		input := req
		if req.SourceProductID > 0 {
			viewApp := &App{DB: tx, Config: a.Config}
			items, e := viewApp.publicViews(viewApp.publicQuery("product").Where("contents.id=?", req.SourceProductID))
			if e != nil {
				return nil, 0, e
			}
			if len(items) != 1 {
				return nil, 0, procMissing()
			}
			product := items[0]
			var vendor Content
			if e = tx.First(&vendor, product.VendorID).Error; e != nil {
				return nil, 0, e
			}
			if vendor.OwnerID == who.ID {
				return nil, 0, procErr("proc_self_inquiry", 400)
			}
			p.SourceProductID = product.ID
			p.SourceVendorID = product.VendorID
			p.HallID = product.HallID
			p.Audience = "direct"
			if input.Title == "" {
				input.Title = product.Data.Name + " · 采购询价"
			}
			if len(input.Items) == 0 {
				input.Items = []ProcurementItem{{CategoryID: effectiveViewCategory(product), Name: product.Data.Name, Specification: product.Data.Parameters, Quantity: "1", Unit: "件"}}
			}
		}
		if p.HallID == 0 {
			p.HallID = 1
		}
		if e := tx.Create(&p).Error; e != nil {
			return nil, 0, e
		}
		_, e := a.procNewDraft(tx, who, &p, input)
		if e != nil {
			return nil, 0, e
		}
		return gin.H{"id": p.ID, "version": p.Version}, p.ID, nil
	})
}
func (a *App) procSaveProject(c *gin.Context) {
	var req procurementDemandInput
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "proc_invalid")
		return
	}
	a.procWrite(c, req, "procurement_draft_saved", func(tx *gorm.DB, who Account) (any, uint64, error) {
		p, e := procOwnProject(tx, who, id(c), req.Version)
		if e != nil {
			return nil, 0, e
		}
		if procClosed(p) {
			return nil, 0, procErr("proc_not_open", 409)
		}
		_, e = a.procNewDraft(tx, who, &p, req)
		return gin.H{"id": p.ID, "version": p.Version}, p.ID, e
	})
}
func procAudience(s string) bool {
	return s == "direct" || s == "matched" || s == "public" || s == "designated"
}
func (a *App) procSubmitProject(c *gin.Context) { a.procSubmit(c, false) }
func (a *App) procExpand(c *gin.Context)        { a.procSubmit(c, true) }
func (a *App) procSubmit(c *gin.Context, expand bool) {
	var req procurementActionInput
	if c.ShouldBindJSON(&req) != nil || !procAudience(req.Audience) || len(req.VendorIDs) > 20 {
		fail(c, 400, "proc_invalid")
		return
	}
	a.procWrite(c, req, "procurement_submitted", func(tx *gorm.DB, who Account) (any, uint64, error) {
		p, e := procOwnProject(tx, who, id(c), req.Version)
		if e != nil {
			return nil, 0, e
		}
		if procClosed(p) {
			return nil, 0, procErr("proc_not_open", 409)
		}
		revisionID := p.DraftRevisionID
		action := "publish"
		if expand {
			revisionID = p.PublishedRevisionID
			action = "expand"
			if revisionID == 0 || req.Audience == "direct" {
				return nil, 0, procInvalid()
			}
		}
		var r ProcurementRevision
		if e = tx.First(&r, revisionID).Error; e != nil {
			return nil, 0, e
		}
		if !expand && r.Status != "draft" && r.Status != "rejected" {
			return nil, 0, procConflict()
		}
		var lines []ProcurementItem
		if e = tx.Where("revision_id=?", r.ID).Find(&lines).Error; e != nil {
			return nil, 0, e
		}
		if e = a.procValidateDemand(tx, p, r, lines, true); e != nil {
			return nil, 0, e
		}
		if req.Audience == "direct" && p.SourceVendorID == 0 {
			return nil, 0, procInvalid()
		}
		if req.Audience == "designated" && len(req.VendorIDs) == 0 {
			return nil, 0, procInvalid()
		}
		for _, vendor := range req.VendorIDs {
			if _, e = a.procEligibleVendor(tx, who.ID, p.HallID, vendor); e != nil {
				return nil, 0, e
			}
		}
		var pending int64
		if e = tx.Model(&ProcurementReview{}).Where("project_id=? AND status='pending'", p.ID).Count(&pending).Error; e != nil {
			return nil, 0, e
		}
		if pending > 0 {
			return nil, 0, procConflict()
		}
		if !expand {
			r.RequestedAudience = req.Audience
			vendorIDs, _ := json.Marshal(req.VendorIDs)
			r.VendorIDs = string(vendorIDs)
		}
		// A first product inquiry reaches its supplier immediately. Any wider
		// invitation request reviews expansion of that same published version.
		firstInquiry := !expand && p.SourceProductID > 0 && p.PublishedRevisionID == 0
		if firstInquiry {
			r.Status = "approved"
			if e = tx.Save(&r).Error; e != nil {
				return nil, 0, e
			}
			p.PublishedRevisionID = r.ID
			p.State = "open"
			p.Audience = "direct"
			if e = a.procAddParticipant(tx, p, p.SourceVendorID, "direct", "产品所属厂商", true); e != nil {
				return nil, 0, e
			}
			action = "expand"
		}
		if !firstInquiry || req.Audience != "direct" {
			vendors, _ := json.Marshal(req.VendorIDs)
			review := ProcurementReview{ProjectID: p.ID, RevisionID: r.ID, Action: action, Audience: req.Audience, VendorIDs: string(vendors), Status: "pending", CreatedAt: time.Now().UTC()}
			if e = tx.Create(&review).Error; e != nil {
				return nil, 0, e
			}
			if !expand && !firstInquiry {
				r.Status = "pending"
				if e = tx.Save(&r).Error; e != nil {
					return nil, 0, e
				}
			}
		}
		if e = procTouch(tx, &p); e != nil {
			return nil, 0, e
		}
		return gin.H{"id": p.ID, "version": p.Version}, p.ID, nil
	})
}
func (a *App) procSetState(c *gin.Context) {
	var req procurementActionInput
	if c.ShouldBindJSON(&req) != nil || (req.State != "closed" && req.State != "cancelled" && req.State != "completed") {
		fail(c, 400, "proc_invalid")
		return
	}
	a.procWrite(c, req, "procurement_ended", func(tx *gorm.DB, who Account) (any, uint64, error) {
		p, e := procOwnProject(tx, who, id(c), req.Version)
		if e != nil {
			return nil, 0, e
		}
		if procClosed(p) {
			return nil, 0, procConflict()
		}
		p.State = req.State
		if e = procTouch(tx, &p); e != nil {
			return nil, 0, e
		}
		if e = tx.Model(&ProcurementReview{}).Where("project_id=? AND status='pending'", p.ID).Update("status", "superseded").Error; e != nil {
			return nil, 0, e
		}
		if e = a.procNotifyParticipants(tx, p, "ended", "项目已结束", fmt.Sprintf("ended:%d:%d", p.ID, p.Version)); e != nil {
			return nil, 0, e
		}
		return gin.H{"id": p.ID, "version": p.Version}, p.ID, nil
	})
}
func (a *App) procReviews(c *gin.Context) {
	page, size := procPaging(c)
	q := a.DB.Model(&ProcurementReview{})
	if status := c.DefaultQuery("status", "pending"); status != "all" {
		q = q.Where("status=?", status)
	}
	var total int64
	var items []ProcurementReview
	e := q.Count(&total).Error
	if e == nil {
		e = q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error
	}
	procResponse(c, gin.H{"items": items, "total": total, "page": page, "page_size": size}, e)
}
func (a *App) procReview(c *gin.Context) {
	var req struct {
		Approved   bool   `json:"approved"`
		Reason     string `json:"reason"`
		RevisionID uint64 `json:"revision_id"`
	}
	if c.ShouldBindJSON(&req) != nil || !procText(req.Reason, 1000) || !req.Approved && strings.TrimSpace(req.Reason) == "" {
		fail(c, 400, "proc_invalid")
		return
	}
	a.procWrite(c, req, "procurement_reviewed", func(tx *gorm.DB, who Account) (any, uint64, error) {
		var review ProcurementReview
		if e := tx.First(&review, id(c)).Error; e != nil {
			return nil, 0, e
		}
		p, e := procLockProject(tx, review.ProjectID)
		if e != nil {
			return nil, 0, e
		}
		// Re-read under the project lock: another reviewer may already have acted.
		if e = tx.First(&review, id(c)).Error; e != nil {
			return nil, 0, e
		}
		if review.Status != "pending" || review.RevisionID != req.RevisionID || procClosed(p) || p.Suspended {
			return nil, 0, procConflict()
		}
		if review.Action == "publish" && p.DraftRevisionID != review.RevisionID || review.Action == "expand" && p.PublishedRevisionID != review.RevisionID {
			return nil, 0, procConflict()
		}
		now := time.Now().UTC()
		review.ReviewerID = who.ID
		review.ReviewedAt = &now
		review.Reason = strings.TrimSpace(req.Reason)
		review.Status = "rejected"
		var r ProcurementRevision
		if e = tx.First(&r, review.RevisionID).Error; e != nil {
			return nil, 0, e
		}
		if req.Approved {
			var lines []ProcurementItem
			if e = tx.Where("revision_id=?", r.ID).Find(&lines).Error; e != nil {
				return nil, 0, e
			}
			if e = a.procValidateDemand(tx, p, r, lines, true); e != nil {
				return nil, 0, e
			}
			var buyer Account
			if e = tx.First(&buyer, p.BuyerID).Error; e != nil {
				return nil, 0, e
			}
			if _, e = procBuyer(tx, buyer); e != nil {
				return nil, 0, e
			}
			review.Status = "approved"
			previousRevision := p.PublishedRevisionID
			if review.Action == "publish" {
				r.Status = "approved"
				if e = tx.Save(&r).Error; e != nil {
					return nil, 0, e
				}
				p.PublishedRevisionID = r.ID
				p.State = "open"
			}
			p.Audience = review.Audience
			if review.Action == "publish" && previousRevision > 0 {
				if e = a.procNotifyParticipants(tx, p, "updated", "采购需求已更新，请按新版本重新报价", fmt.Sprintf("approved:%d:%d", p.ID, review.ID)); e != nil {
					return nil, 0, e
				}
			}
			if p.SourceVendorID > 0 {
				if e = a.procAddParticipant(tx, p, p.SourceVendorID, "direct", "产品所属厂商", true); e != nil {
					return nil, 0, e
				}
			}
			var ids []uint64
			if e = json.Unmarshal([]byte(review.VendorIDs), &ids); e != nil {
				return nil, 0, e
			}
			reasons := map[uint64]string{}
			var existing []ProcurementParticipant
			if e = tx.Where("project_id=?", p.ID).Find(&existing).Error; e != nil {
				return nil, 0, e
			}
			seen := map[uint64]bool{}
			pushCount := 0
			for _, part := range existing {
				seen[part.VendorID] = true
				if part.Source != "join" {
					pushCount++
				}
			}
			if p.Audience == "matched" || p.Audience == "public" {
				matches, me := a.procFindMatches(tx, p)
				if me != nil {
					return nil, 0, me
				}
				for _, m := range matches {
					if len(ids) >= 20 || pushCount+len(ids) >= 100 {
						break
					}
					if seen[m.ID] {
						continue
					}
					ids = append(ids, m.ID)
					reasons[m.ID] = m.Reason
					seen[m.ID] = true
				}
			}
			for _, vendor := range ids {
				source, reason := "auto", reasons[vendor]
				if reason == "" {
					source = "manual"
					reason = "审核后的指定采购邀请"
				}
				if e = a.procAddParticipant(tx, p, vendor, source, reason, false); e != nil {
					return nil, 0, e
				}
			}
		} else if review.Action == "publish" {
			r.Status = "rejected"
			if e = tx.Save(&r).Error; e != nil {
				return nil, 0, e
			}
		}
		if e = tx.Save(&review).Error; e != nil {
			return nil, 0, e
		}
		if e = procTouch(tx, &p); e != nil {
			return nil, 0, e
		}
		if e = a.procNotice(tx, p.BuyerID, p.ID, "review", "采购需求审核结果", fmt.Sprintf("review:%d", review.ID), false); e != nil {
			return nil, 0, e
		}
		return gin.H{"review": review, "version": p.Version}, p.ID, nil
	})
}

func (a *App) procModerate(c *gin.Context) {
	var req struct {
		Version   uint64 `json:"version"`
		Suspended bool   `json:"suspended"`
		QuoteID   uint64 `json:"quote_id"`
		Excluded  bool   `json:"excluded"`
		VendorID  uint64 `json:"vendor_id"`
		Reason    string `json:"reason"`
	}
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Reason) == "" || !procText(req.Reason, 1000) {
		fail(c, 400, "proc_invalid")
		return
	}
	a.procWrite(c, req, "procurement_moderated", func(tx *gorm.DB, who Account) (any, uint64, error) {
		p, e := procLockProject(tx, id(c))
		if e != nil {
			return nil, 0, e
		}
		if p.Version != req.Version {
			return nil, 0, procConflict()
		}
		if req.QuoteID > 0 {
			if e = tx.Model(&ProcurementQuote{}).Where("id=? AND project_id=?", req.QuoteID, p.ID).Update("excluded", req.Excluded).Error; e != nil {
				return nil, 0, e
			}
		} else if req.VendorID > 0 {
			state := "invited"
			if req.Excluded {
				state = "excluded"
			}
			if e = tx.Model(&ProcurementParticipant{}).Where("project_id=? AND vendor_id=?", p.ID, req.VendorID).Updates(map[string]any{"state": state, "intent": false}).Error; e != nil {
				return nil, 0, e
			}
		} else {
			p.Suspended = req.Suspended
		}
		if e = procTouch(tx, &p); e != nil {
			return nil, 0, e
		}
		if e = recordOperation(tx, who.ID, "procurement", p.ID, "procurement_moderation_reason", req.Reason); e != nil {
			return nil, 0, e
		}
		return gin.H{"id": p.ID, "version": p.Version}, p.ID, nil
	})
}
func (a *App) procRestrict(c *gin.Context) {
	var req struct {
		Suspended bool   `json:"suspended"`
		Supplier  bool   `json:"supplier"`
		Reason    string `json:"reason"`
	}
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Reason) == "" || !procText(req.Reason, 1000) {
		fail(c, 400, "proc_invalid")
		return
	}
	a.procWrite(c, req, "procurement_account_restricted", func(tx *gorm.DB, who Account) (any, uint64, error) {
		var e error
		if req.Supplier {
			var target Account
			if e = tx.Where("id=? AND role IN ('merchant','visitor')", id(c)).First(&target).Error; e != nil {
				return nil, 0, e
			}
			pref, pe := procPreference(tx, id(c))
			if pe != nil {
				return nil, 0, pe
			}
			pref.SupplierSuspended = req.Suspended
			pref.UpdatedAt = time.Now().UTC()
			e = tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "account_id"}}, DoUpdates: clause.AssignmentColumns([]string{"supplier_suspended", "updated_at"})}).Create(&pref).Error
		} else {
			var target BuyerProfile
			if e = tx.First(&target, "account_id=?", id(c)).Error; e != nil {
				return nil, 0, e
			}
			e = tx.Model(&BuyerProfile{}).Where("account_id=?", id(c)).Update("suspended", req.Suspended).Error
		}
		if e != nil {
			return nil, 0, e
		}
		e = recordOperation(tx, who.ID, "account", id(c), "procurement_restriction_reason", req.Reason)
		return gin.H{"ok": true}, 0, e
	})
}
func (a *App) procStats(c *gin.Context) {
	counts := map[string]int64{}
	for _, state := range []string{"draft", "open", "closed", "cancelled", "completed"} {
		var n int64
		if e := a.DB.Model(&ProcurementProject{}).Where("state=?", state).Count(&n).Error; e != nil {
			procResponse(c, nil, e)
			return
		}
		counts[state] = n
	}
	var quotes, invitations, pending int64
	for _, job := range []struct {
		model any
		where string
		out   *int64
	}{{&ProcurementQuote{}, "submitted_revision_id>0", &quotes}, {&ProcurementParticipant{}, "1=1", &invitations}, {&ProcurementReview{}, "status='pending'", &pending}} {
		if e := a.DB.Model(job.model).Where(job.where).Count(job.out).Error; e != nil {
			procResponse(c, nil, e)
			return
		}
	}
	c.JSON(200, gin.H{"projects": counts, "quotes": quotes, "invitations": invitations, "pending_reviews": pending})
}
