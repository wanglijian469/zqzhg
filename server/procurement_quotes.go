package main

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type procurementQuoteInput struct {
	Version          uint64                 `json:"version"`
	DemandRevisionID uint64                 `json:"demand_revision_id"`
	Submit           bool                   `json:"submit"`
	TaxMode          string                 `json:"tax_mode"`
	FreightMode      string                 `json:"freight_mode"`
	Freight          string                 `json:"freight"`
	ValidUntil       time.Time              `json:"valid_until"`
	Notes            string                 `json:"notes"`
	Items            []ProcurementQuoteItem `json:"items"`
	AttachmentIDs    []uint64               `json:"attachment_ids"`
}

func (a *App) procSaveQuote(c *gin.Context) {
	var req procurementQuoteInput
	if c.ShouldBindJSON(&req) != nil || len(req.Items) > 30 || !procText(req.Notes, 10000) || !procNumber(req.Freight, 2, false) || !containsProc([]string{"included", "excluded", "negotiable"}, req.TaxMode) || !containsProc([]string{"included", "fixed", "collect", "negotiable"}, req.FreightMode) {
		fail(c, 400, "proc_invalid")
		return
	}
	a.procWrite(c, req, "procurement_quoted", func(tx *gorm.DB, who Account) (any, uint64, error) {
		p, e := procLockProject(tx, id(c))
		if e != nil {
			return nil, 0, e
		}
		demand, e := procBeforeDeadline(tx, p)
		if e != nil {
			return nil, 0, e
		}
		if demand.ID != req.DemandRevisionID {
			return nil, 0, procErr("proc_demand_changed", 409)
		}
		vendor, e := a.procSupplier(tx, who, p)
		if e != nil {
			return nil, 0, e
		}
		var participant ProcurementParticipant
		if e = tx.Where("project_id=? AND vendor_id=? AND state IN ('invited','participating')", p.ID, vendor.ID).First(&participant).Error; e != nil {
			return nil, 0, procMissing()
		}
		var quote ProcurementQuote
		e = tx.Where("project_id=? AND vendor_id=?", p.ID, vendor.ID).First(&quote).Error
		if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
			return nil, 0, e
		}
		if quote.Version != req.Version {
			return nil, 0, procConflict()
		}
		if quote.ID == 0 {
			quote = ProcurementQuote{ProjectID: p.ID, VendorID: vendor.ID}
			if e = tx.Create(&quote).Error; e != nil {
				return nil, 0, e
			}
		}
		if req.ValidUntil.IsZero() {
			req.ValidUntil = time.Now().UTC().Add(7 * 24 * time.Hour)
		}
		if req.Submit && !req.ValidUntil.After(time.Now().UTC()) {
			return nil, 0, procInvalid()
		}
		var demandItems []ProcurementItem
		if e = tx.Where("revision_id=?", demand.ID).Find(&demandItems).Error; e != nil {
			return nil, 0, e
		}
		validIDs := map[uint64]bool{}
		for _, i := range demandItems {
			validIDs[i.ID] = true
		}
		seen := map[uint64]bool{}
		quoted := 0
		for _, line := range req.Items {
			if !validIDs[line.ItemID] || seen[line.ItemID] || !containsProc([]string{"quoted", "unavailable", "unquoted"}, line.Status) || !procText(line.Notes, 1000) || line.DeliveryDays > 3650 {
				return nil, 0, procInvalid()
			}
			seen[line.ItemID] = true
			if line.Status == "quoted" {
				if !procNumber(line.UnitPrice, 4, true) || !procNumber(line.MinimumQuantity, 3, false) {
					return nil, 0, procInvalid()
				}
				quoted++
			}
		}
		if req.Submit && quoted == 0 {
			return nil, 0, procErr("proc_quote_empty", 400)
		}
		status := "draft"
		if req.Submit {
			status = "submitted"
		}
		revision := ProcurementQuoteRevision{QuoteID: quote.ID, DemandRevisionID: demand.ID, Status: status, TaxMode: req.TaxMode, FreightMode: req.FreightMode, Freight: req.Freight, ValidUntil: req.ValidUntil, Notes: req.Notes, CreatedAt: time.Now().UTC()}
		if e = tx.Create(&revision).Error; e != nil {
			return nil, 0, e
		}
		for _, line := range req.Items {
			line.ID = 0
			line.QuoteRevisionID = revision.ID
			if line.Status != "quoted" {
				line.UnitPrice = "0"
				line.MinimumQuantity = "0"
			}
			if e = tx.Create(&line).Error; e != nil {
				return nil, 0, e
			}
		}
		if e = procBindAttachments(tx, req.AttachmentIDs, p.ID, who.ID, "quote", revision.ID); e != nil {
			return nil, 0, e
		}
		quote.DraftRevisionID = revision.ID
		quote.Version++
		quote.UpdatedAt = time.Now().UTC()
		if req.Submit {
			quote.SubmittedRevisionID = revision.ID
			quote.Withdrawn = false
			participant.State = "participating"
			participant.UpdatedAt = time.Now().UTC()
			if e = tx.Save(&participant).Error; e != nil {
				return nil, 0, e
			}
			if e = a.procNotice(tx, p.BuyerID, p.ID, "quote", "收到新的厂商报价", fmt.Sprintf("quote:%d", revision.ID), false); e != nil {
				return nil, 0, e
			}
		}
		if e = tx.Save(&quote).Error; e != nil {
			return nil, 0, e
		}
		return gin.H{"quote": quote, "revision": revision}, p.ID, nil
	})
}
func containsProc(values []string, value string) bool {
	for _, v := range values {
		if value == v {
			return true
		}
	}
	return false
}
func (a *App) procWithdrawQuote(c *gin.Context) {
	var req struct {
		Version uint64 `json:"version"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "proc_invalid")
		return
	}
	a.procWrite(c, req, "procurement_quote_withdrawn", func(tx *gorm.DB, who Account) (any, uint64, error) {
		p, e := procLockProject(tx, id(c))
		if e != nil {
			return nil, 0, e
		}
		if _, e = procBeforeDeadline(tx, p); e != nil {
			return nil, 0, e
		}
		vendor, e := a.procSupplier(tx, who, p)
		if e != nil {
			return nil, 0, e
		}
		var quote ProcurementQuote
		if e = tx.Where("project_id=? AND vendor_id=?", p.ID, vendor.ID).First(&quote).Error; e != nil {
			return nil, 0, e
		}
		if quote.Version != req.Version {
			return nil, 0, procConflict()
		}
		quote.Withdrawn = true
		quote.Version++
		quote.UpdatedAt = time.Now().UTC()
		if e = tx.Save(&quote).Error; e != nil {
			return nil, 0, e
		}
		e = a.procNotice(tx, p.BuyerID, p.ID, "quote", "厂商撤回了报价", fmt.Sprintf("withdraw:%d:%d", quote.ID, quote.Version), false)
		return gin.H{"quote": quote}, p.ID, e
	})
}
func (a *App) procQuotes(c *gin.Context) {
	var p ProcurementProject
	e := a.DB.First(&p, id(c)).Error
	if e != nil {
		procResponse(c, nil, e)
		return
	}
	participant, owner, e := a.procAccess(a.DB, actor(c), p)
	if e != nil {
		procResponse(c, nil, e)
		return
	}
	q := a.DB.Where("project_id=?", p.ID)
	if !owner {
		q = q.Where("vendor_id=?", participant.VendorID)
	} else {
		q = q.Where("submitted_revision_id>0")
	}
	var quotes []ProcurementQuote
	if e = q.Order("updated_at DESC,id DESC").Find(&quotes).Error; e != nil {
		procResponse(c, nil, e)
		return
	}
	result := []gin.H{}
	for _, quote := range quotes {
		history := []gin.H{}
		var revisions []ProcurementQuoteRevision
		query := a.DB.Where("quote_id=?", quote.ID)
		if owner {
			query = query.Where("status='submitted'")
		}
		if e = query.Order("id DESC").Find(&revisions).Error; e != nil {
			procResponse(c, nil, e)
			return
		}
		for _, r := range revisions {
			var lines []ProcurementQuoteItem
			var demand []ProcurementItem
			if e = a.DB.Where("quote_revision_id=?", r.ID).Order("id").Find(&lines).Error; e != nil {
				procResponse(c, nil, e)
				return
			}
			if e = a.DB.Where("revision_id=?", r.DemandRevisionID).Find(&demand).Error; e != nil {
				procResponse(c, nil, e)
				return
			}
			for i := range lines {
				if lines[i].Status != "quoted" {
					lines[i].UnitPrice = ""
					lines[i].MinimumQuantity = ""
				}
			}
			files, fe := procAttachments(a.DB, p.ID, "quote", r.ID)
			if fe != nil {
				procResponse(c, nil, fe)
				return
			}
			history = append(history, gin.H{"revision": r, "items": lines, "demand_items": demand, "attachments": files, "stale": r.DemandRevisionID != p.PublishedRevisionID, "expired": !r.ValidUntil.After(time.Now().UTC()), "comparable_total": procComparableTotal(quote, r, lines, demand, p.PublishedRevisionID)})
		}
		var vendor Content
		var vendorRev Revision
		if e = a.DB.First(&vendor, quote.VendorID).Error; e != nil {
			procResponse(c, nil, e)
			return
		}
		if e = a.DB.First(&vendorRev, vendor.PublishedRevisionID).Error; e != nil {
			procResponse(c, nil, e)
			return
		}
		entry := gin.H{"quote": quote, "vendor_name": vendorRev.Name, "history": history}
		result = append(result, entry)
	}
	c.JSON(200, gin.H{"items": result})
}
func procComparableTotal(q ProcurementQuote, r ProcurementQuoteRevision, lines []ProcurementQuoteItem, demand []ProcurementItem, current uint64) any {
	if q.Withdrawn || q.Excluded || r.Status != "submitted" || r.DemandRevisionID != current || !r.ValidUntil.After(time.Now().UTC()) || r.TaxMode != "included" || !containsProc([]string{"included", "fixed"}, r.FreightMode) {
		return nil
	}
	byID := map[uint64]ProcurementQuoteItem{}
	for _, line := range lines {
		byID[line.ItemID] = line
	}
	total := new(big.Rat)
	for _, item := range demand {
		line, ok := byID[item.ID]
		if !ok || line.Status != "quoted" {
			return nil
		}
		price, ok := new(big.Rat).SetString(line.UnitPrice)
		if !ok {
			return nil
		}
		quantity, ok := new(big.Rat).SetString(item.Quantity)
		if !ok {
			return nil
		}
		minimum, ok := new(big.Rat).SetString(line.MinimumQuantity)
		if !ok || minimum.Cmp(quantity) > 0 {
			return nil
		}
		total.Add(total, new(big.Rat).Mul(price, quantity))
	}
	if r.FreightMode == "fixed" {
		freight, ok := new(big.Rat).SetString(r.Freight)
		if !ok {
			return nil
		}
		total.Add(total, freight)
	}
	return total.FloatString(2)
}

func (a *App) procConversation(db *gorm.DB, who Account, p ProcurementProject, vendorID uint64) (ProcurementParticipant, error) {
	var part ProcurementParticipant
	if e := db.Where("project_id=? AND vendor_id=? AND state<>'excluded'", p.ID, vendorID).First(&part).Error; e != nil {
		return part, e
	}
	if who.ID == p.BuyerID {
		return part, nil
	}
	var vendor Content
	if e := db.First(&vendor, vendorID).Error; e != nil {
		return part, e
	}
	if vendor.OwnerID != who.ID {
		return part, procMissing()
	}
	return part, nil
}
func (a *App) procMessages(c *gin.Context) {
	vendorID, e := parseProcID(c.Param("vendor"))
	if e != nil {
		procResponse(c, nil, e)
		return
	}
	var p ProcurementProject
	if e = a.DB.First(&p, id(c)).Error; e != nil {
		procResponse(c, nil, e)
		return
	}
	part, e := a.procConversation(a.DB, actor(c), p, vendorID)
	if e != nil {
		procResponse(c, nil, e)
		return
	}
	page, size := procPaging(c)
	var messages []ProcurementMessage
	q := a.DB.Where("participant_id=?", part.ID)
	var total int64
	if e = q.Model(&ProcurementMessage{}).Count(&total).Error; e != nil {
		procResponse(c, nil, e)
		return
	}
	if e = q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&messages).Error; e != nil {
		procResponse(c, nil, e)
		return
	}
	result := []gin.H{}
	for _, msg := range messages {
		files, fe := procAttachments(a.DB, p.ID, "message", msg.ID)
		if fe != nil {
			procResponse(c, nil, fe)
			return
		}
		result = append(result, gin.H{"message": msg, "attachments": files, "mine": msg.AuthorID == actor(c).ID})
	}
	c.JSON(200, gin.H{"items": result, "total": total, "page": page, "page_size": size})
}
func (a *App) procSendMessage(c *gin.Context) {
	var req struct {
		Body          string   `json:"body"`
		AttachmentIDs []uint64 `json:"attachment_ids"`
	}
	if c.ShouldBindJSON(&req) != nil || !procText(req.Body, 5000) || (req.Body == "" && len(req.AttachmentIDs) == 0) {
		fail(c, 400, "proc_invalid")
		return
	}
	a.procWrite(c, req, "procurement_message_sent", func(tx *gorm.DB, who Account) (any, uint64, error) {
		p, e := procLockProject(tx, id(c))
		if e != nil {
			return nil, 0, e
		}
		if _, e = procActive(tx, p); e != nil {
			return nil, 0, e
		}
		vendorID, e := parseProcID(c.Param("vendor"))
		if e != nil {
			return nil, 0, e
		}
		part, e := a.procConversation(tx, who, p, vendorID)
		if e != nil {
			return nil, 0, e
		}
		msg := ProcurementMessage{ProjectID: p.ID, ParticipantID: part.ID, AuthorID: who.ID, Body: req.Body, CreatedAt: time.Now().UTC()}
		if e = tx.Create(&msg).Error; e != nil {
			return nil, 0, e
		}
		if e = procBindAttachments(tx, req.AttachmentIDs, p.ID, who.ID, "message", msg.ID); e != nil {
			return nil, 0, e
		}
		recipient := p.BuyerID
		if who.ID == p.BuyerID {
			var vendor Content
			if e = tx.First(&vendor, vendorID).Error; e != nil {
				return nil, 0, e
			}
			recipient = vendor.OwnerID
		}
		e = a.procNotice(tx, recipient, p.ID, "message", "收到采购洽谈消息", fmt.Sprintf("message:%d", msg.ID), false)
		return gin.H{"message": msg}, p.ID, e
	})
}
