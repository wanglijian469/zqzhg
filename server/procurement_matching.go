package main

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type procurementMatch struct {
	ID              uint64 `json:"id"`
	Name            string `json:"name"`
	Score           int    `json:"score"`
	Reason          string `json:"reason"`
	RegionConfirmed bool   `json:"region_confirmed"`
}

func (a *App) procEligibleVendor(tx *gorm.DB, buyer, hall, vendorID uint64) (Content, error) {
	var vendor Content
	q := tx.Where("id=? AND kind='vendor' AND visible=1 AND published_revision_id>0 AND hall_id=? AND owner_id<>?", vendorID, hall, buyer)
	q = q.Where("EXISTS (SELECT 1 FROM halls h WHERE h.id=contents.hall_id AND h.status='open')")
	if e := q.First(&vendor).Error; e != nil {
		return vendor, procMissing()
	}
	var revision Revision
	if e := tx.Where("id=? AND status='approved'", vendor.PublishedRevisionID).First(&revision).Error; e != nil {
		return vendor, procMissing()
	}
	pref, e := procPreference(tx, vendor.OwnerID)
	if e != nil {
		return vendor, e
	}
	if pref.SupplierSuspended {
		return vendor, procErr("proc_suspended", 403)
	}
	return vendor, nil
}
func (a *App) procSupplier(tx *gorm.DB, who Account, p ProcurementProject) (Content, error) {
	if who.Role != "merchant" {
		return Content{}, procErr("proc_supplier_required", 403)
	}
	var vendor Content
	if e := tx.Where("kind='vendor' AND owner_id=?", who.ID).First(&vendor).Error; e != nil {
		return vendor, procErr("proc_supplier_required", 403)
	}
	return a.procEligibleVendor(tx, p.BuyerID, p.HallID, vendor.ID)
}
func (a *App) procFindMatches(tx *gorm.DB, p ProcurementProject) ([]procurementMatch, error) {
	var revision ProcurementRevision
	revisionID := p.PublishedRevisionID
	if revisionID == 0 {
		revisionID = p.DraftRevisionID
	}
	if e := tx.First(&revision, revisionID).Error; e != nil {
		return nil, e
	}
	var lines []ProcurementItem
	if e := tx.Where("revision_id=?", revision.ID).Find(&lines).Error; e != nil {
		return nil, e
	}
	viewApp := &App{DB: tx, Config: a.Config}
	vendors, e := viewApp.publicViews(viewApp.publicQuery("vendor").Where("contents.hall_id=? AND contents.owner_id<>?", p.HallID, p.BuyerID))
	if e != nil {
		return nil, e
	}
	var categories []Category
	if e = tx.Where("hall_id=? AND enabled=1 AND deleted_at IS NULL", p.HallID).Find(&categories).Error; e != nil {
		return nil, e
	}
	byID := map[uint64]Category{}
	for _, cat := range categories {
		byID[cat.ID] = cat
	}
	var products []struct {
		VendorID   uint64
		CategoryID uint64
	}
	if e = viewApp.publicQuery("product").Where("contents.hall_id=?", p.HallID).Select("contents.vendor_id,r.category_id").Scan(&products).Error; e != nil {
		return nil, e
	}
	productCategories := map[uint64][]uint64{}
	for _, product := range products {
		productCategories[product.VendorID] = append(productCategories[product.VendorID], canonicalCategory(tx, product.CategoryID))
	}
	var participants []ProcurementParticipant
	if e = tx.Where("project_id=?", p.ID).Find(&participants).Error; e != nil {
		return nil, e
	}
	already := map[uint64]bool{}
	for _, part := range participants {
		already[part.VendorID] = true
	}
	var prefs []ProcurementPreference
	if e = tx.Where("supplier_suspended=1").Find(&prefs).Error; e != nil {
		return nil, e
	}
	suspended := map[uint64]bool{}
	for _, pref := range prefs {
		suspended[pref.AccountID] = true
	}
	// publicViews removes owner IDs; recover only IDs needed for the suspension check.
	var owners []Content
	if e = tx.Select("id,owner_id").Where("kind='vendor' AND hall_id=?", p.HallID).Find(&owners).Error; e != nil {
		return nil, e
	}
	ownerByVendor := map[uint64]uint64{}
	for _, v := range owners {
		ownerByVendor[v.ID] = v.OwnerID
	}
	ancestor := func(parent, child uint64) bool {
		seen := map[uint64]bool{}
		for child > 0 && !seen[child] {
			if child == parent {
				return true
			}
			seen[child] = true
			child = byID[child].ParentID
		}
		return false
	}
	result := []procurementMatch{}
	for _, vendor := range vendors {
		if suspended[ownerByVendor[vendor.ID]] {
			continue
		}
		capabilities := append([]uint64{}, productCategories[vendor.ID]...)
		regions := []string{}
		if profile := vendor.Data.CompanyProfile; profile != nil {
			capabilities = append(capabilities, profile.SupplyProductCategoryIDs...)
			regions = profile.SupplyRegionCodes
		}
		confirmed := len(regions) > 0
		covered := !confirmed
		for _, region := range regions {
			if strings.HasPrefix(revision.DeliveryRegion, region) {
				covered = true
			}
		}
		if !covered {
			continue
		}
		score := 0
		for _, line := range lines {
			demandCategory := canonicalCategory(tx, line.CategoryID)
			for _, raw := range capabilities {
				cat := canonicalCategory(tx, raw)
				definition, ok := byID[cat]
				if !ok || definition.CategoryKind == "vendor" {
					continue
				}
				if cat == demandCategory {
					score = max(score, 100)
				} else if ancestor(cat, demandCategory) || ancestor(demandCategory, cat) {
					score = max(score, 50)
				}
			}
		}
		if score == 0 {
			continue
		}
		keywords := strings.ToLower(vendor.Data.MainProducts + " " + vendor.Data.Keywords + " " + vendor.Data.Name)
		for _, line := range lines {
			if strings.Contains(keywords, strings.ToLower(strings.TrimSpace(line.Name))) {
				score += 10
			}
		}
		reason := "同展馆、产品分类匹配；供货范围待确认"
		if confirmed {
			score += 5
			reason = "同展馆、产品分类及供货地区匹配"
		}
		if already[vendor.ID] {
			reason += "；已参与/已邀请"
		}
		result = append(result, procurementMatch{vendor.ID, vendor.Data.Name, score, reason, confirmed})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Score == result[j].Score {
			return result[i].ID < result[j].ID
		}
		return result[i].Score > result[j].Score
	})
	return result, nil
}
func (a *App) procMatches(c *gin.Context) {
	var p ProcurementProject
	e := a.DB.First(&p, id(c)).Error
	if e != nil {
		procResponse(c, nil, e)
		return
	}
	if actor(c).ID != p.BuyerID && !platformAccount(actor(c)) {
		procResponse(c, nil, procMissing())
		return
	}
	if c.Query("revision_id") != "" {
		selected, parseError := parseProcID(c.Query("revision_id"))
		var revision ProcurementRevision
		if parseError != nil || a.DB.Where("id=? AND project_id=?", selected, p.ID).First(&revision).Error != nil {
			procResponse(c, nil, procMissing())
			return
		}
		p.PublishedRevisionID = selected
	}
	result, e := a.procFindMatches(a.DB, p)
	procResponse(c, gin.H{"items": result}, e)
}
func (a *App) procAddParticipant(tx *gorm.DB, p ProcurementProject, vendorID uint64, source, reason string, direct bool) error {
	vendor, e := a.procEligibleVendor(tx, p.BuyerID, p.HallID, vendorID)
	if e != nil {
		return e
	}
	var existing ProcurementParticipant
	e = tx.Where("project_id=? AND vendor_id=?", p.ID, vendorID).First(&existing).Error
	if e == nil {
		return nil
	}
	if !errors.Is(e, gorm.ErrRecordNotFound) {
		return e
	}
	if source != "join" {
		var count int64
		if e = tx.Model(&ProcurementParticipant{}).Where("project_id=? AND source<>'join'", p.ID).Count(&count).Error; e != nil {
			return e
		}
		if count >= 100 {
			return procErr("proc_invite_limit", 409)
		}
	}
	state := "invited"
	if source == "join" {
		state = "participating"
	}
	participant := ProcurementParticipant{ProjectID: p.ID, VendorID: vendorID, Source: source, State: state, MatchReason: reason, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if e = tx.Create(&participant).Error; e != nil {
		return e
	}
	return a.procNotice(tx, vendor.OwnerID, p.ID, "invitation", "收到采购询价/邀请", fmt.Sprintf("invite:%d:%d:%d", p.ID, vendorID, p.PublishedRevisionID), !direct)
}
func (a *App) procInvite(c *gin.Context) {
	var req procurementActionInput
	if c.ShouldBindJSON(&req) != nil || len(req.VendorIDs) > 20 {
		fail(c, 400, "proc_invalid")
		return
	}
	a.procWrite(c, req, "procurement_invited", func(tx *gorm.DB, who Account) (any, uint64, error) {
		p, e := procLockProject(tx, id(c))
		if e != nil {
			return nil, 0, e
		}
		if p.Version != req.Version {
			return nil, 0, procConflict()
		}
		if who.ID != p.BuyerID && !platformAccount(who) {
			return nil, 0, procMissing()
		}
		if _, e = procBeforeDeadline(tx, p); e != nil {
			return nil, 0, e
		}
		if p.Audience == "direct" {
			return nil, 0, procErr("proc_review_required", 409)
		}
		ids := req.VendorIDs
		reasons := map[uint64]string{}
		if len(ids) == 0 {
			if p.Audience != "matched" && p.Audience != "public" {
				return nil, 0, procInvalid()
			}
			matches, me := a.procFindMatches(tx, p)
			if me != nil {
				return nil, 0, me
			}
			for _, match := range matches {
				var n int64
				if e = tx.Model(&ProcurementParticipant{}).Where("project_id=? AND vendor_id=?", p.ID, match.ID).Count(&n).Error; e != nil {
					return nil, 0, e
				}
				if n == 0 {
					ids = append(ids, match.ID)
					reasons[match.ID] = match.Reason
				}
				if len(ids) >= 20 {
					break
				}
			}
		}
		for _, vendor := range ids {
			reason := reasons[vendor]
			if reason == "" {
				reason = "指定采购邀请"
			}
			if e = a.procAddParticipant(tx, p, vendor, "manual", reason, false); e != nil {
				return nil, 0, e
			}
		}
		if e = procTouch(tx, &p); e != nil {
			return nil, 0, e
		}
		return gin.H{"id": p.ID, "version": p.Version}, p.ID, nil
	})
}
func (a *App) procJoin(c *gin.Context) {
	var req struct {
		Version uint64 `json:"version"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "proc_invalid")
		return
	}
	a.procWrite(c, req, "procurement_joined", func(tx *gorm.DB, who Account) (any, uint64, error) {
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
		var existing ProcurementParticipant
		e = tx.Where("project_id=? AND vendor_id=?", p.ID, vendor.ID).First(&existing).Error
		if e == nil {
			if existing.State == "excluded" {
				return nil, 0, procMissing()
			}
			return gin.H{"id": p.ID}, p.ID, nil
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return nil, 0, e
		}
		if p.Audience != "public" {
			return nil, 0, procMissing()
		}
		matches, e := a.procFindMatches(tx, p)
		if e != nil {
			return nil, 0, e
		}
		for _, m := range matches {
			if m.ID == vendor.ID {
				e = a.procAddParticipant(tx, p, vendor.ID, "join", m.Reason, false)
				return gin.H{"id": p.ID}, p.ID, e
			}
		}
		return nil, 0, procErr("proc_no_match", 403)
	})
}
func (a *App) procParticipantState(c *gin.Context) {
	var req struct {
		State   string `json:"state"`
		Intent  *bool  `json:"intent"`
		Version uint64 `json:"version"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "proc_invalid")
		return
	}
	a.procWrite(c, req, "procurement_participant_changed", func(tx *gorm.DB, who Account) (any, uint64, error) {
		p, e := procLockProject(tx, id(c))
		if e != nil {
			return nil, 0, e
		}
		if _, e = procActive(tx, p); e != nil {
			return nil, 0, e
		}
		vendorID, _ := parseProcID(c.Param("vendor"))
		var participant ProcurementParticipant
		if e = tx.Where("project_id=? AND vendor_id=? AND state<>'excluded'", p.ID, vendorID).First(&participant).Error; e != nil {
			return nil, 0, e
		}
		if req.Intent != nil {
			if who.ID != p.BuyerID {
				return nil, 0, procMissing()
			}
			if _, e = procBuyer(tx, who); e != nil {
				return nil, 0, e
			}
			if req.Version != p.Version {
				return nil, 0, procConflict()
			}
			participant.Intent = *req.Intent
			var vendor Content
			if e = tx.First(&vendor, vendorID).Error; e != nil {
				return nil, 0, e
			}
			if e = a.procNotice(tx, vendor.OwnerID, p.ID, "intent", "采购商更新了合作意向", fmt.Sprintf("intent:%d:%d:%d", p.ID, vendorID, p.Version), false); e != nil {
				return nil, 0, e
			}
			if e = procTouch(tx, &p); e != nil {
				return nil, 0, e
			}
		} else {
			vendor, e := a.procSupplier(tx, who, p)
			if e != nil {
				return nil, 0, e
			}
			if vendor.ID != vendorID {
				return nil, 0, procMissing()
			}
			if req.State != "declined" && req.State != "participating" {
				return nil, 0, procInvalid()
			}
			participant.State = req.State
			if req.State == "declined" {
				if e = tx.Model(&ProcurementQuote{}).Where("project_id=? AND vendor_id=?", p.ID, vendorID).Update("withdrawn", true).Error; e != nil {
					return nil, 0, e
				}
			}
		}
		participant.UpdatedAt = time.Now().UTC()
		if e = tx.Save(&participant).Error; e != nil {
			return nil, 0, e
		}
		return gin.H{"participant": participant, "version": p.Version}, p.ID, nil
	})
}
func parseProcID(raw string) (uint64, error) {
	id, e := strconv.ParseUint(raw, 10, 64)
	if e != nil || id == 0 {
		return 0, procMissing()
	}
	return id, nil
}
func (a *App) procVendorOptions(c *gin.Context) {
	page, size := procPaging(c)
	q := a.publicQuery("vendor")
	if c.Query("hall_id") != "" {
		q = q.Where("contents.hall_id=?", c.Query("hall_id"))
	}
	if kw := strings.TrimSpace(c.Query("q")); kw != "" {
		if !procText(kw, 200) {
			fail(c, 400, "proc_invalid")
			return
		}
		q = q.Where("r.name LIKE ?", "%"+kw+"%")
	}
	var total int64
	if e := q.Count(&total).Error; e != nil {
		procResponse(c, nil, e)
		return
	}
	items, e := a.publicViews(q.Order("contents.id").Offset((page - 1) * size).Limit(size))
	result := []gin.H{}
	for _, v := range items {
		result = append(result, gin.H{"id": v.ID, "name": v.Data.Name})
	}
	procResponse(c, gin.H{"items": result, "total": total, "page": page, "page_size": size}, e)
}
