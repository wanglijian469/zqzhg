package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type procurementError struct {
	Code   string
	Status int
}

func (e procurementError) Error() string    { return e.Code }
func procErr(code string, status int) error { return procurementError{code, status} }
func procInvalid() error                    { return procErr("proc_invalid", 400) }
func procMissing() error                    { return procErr("not_found", 404) }
func procConflict() error                   { return procErr("proc_conflict", 409) }
func procurementEnabled() bool              { return false } // This release only provides free information display.
func procResponse(c *gin.Context, value any, err error) {
	if err == nil {
		c.JSON(200, value)
		return
	}
	var failure procurementError
	if errors.As(err, &failure) {
		fail(c, failure.Status, failure.Code)
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		fail(c, 404, "not_found")
		return
	}
	fail(c, 500, "internal_error")
}
func procText(s string, max int) bool { return len([]rune(strings.TrimSpace(s))) <= max }

var procDecimal = regexp.MustCompile(`^(0|[1-9][0-9]{0,13})(\.[0-9]{1,4})?$`)

func procNumber(s string, precision int, positive bool) bool {
	if !procDecimal.MatchString(s) {
		return false
	}
	if dot := strings.IndexByte(s, '.'); dot >= 0 && len(s)-dot-1 > precision {
		return false
	}
	n, ok := new(big.Rat).SetString(s)
	return ok && (!positive || n.Sign() > 0)
}
func procClosed(p ProcurementProject) bool {
	return p.State == "completed" || p.State == "closed" || p.State == "cancelled"
}
func procLockProject(tx *gorm.DB, projectID uint64) (ProcurementProject, error) {
	var p ProcurementProject
	e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&p, projectID).Error
	return p, e
}
func procBuyer(tx *gorm.DB, who Account) (BuyerProfile, error) {
	var b BuyerProfile
	e := tx.First(&b, "account_id=?", who.ID).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return b, procErr("proc_register_buyer", 403)
	}
	if e != nil {
		return b, e
	}
	if b.Suspended {
		return b, procErr("proc_suspended", 403)
	}
	return b, nil
}
func procOwnProject(tx *gorm.DB, who Account, projectID, version uint64) (ProcurementProject, error) {
	p, e := procLockProject(tx, projectID)
	if e != nil {
		return p, e
	}
	if p.BuyerID != who.ID {
		return p, procMissing()
	}
	if _, e = procBuyer(tx, who); e != nil {
		return p, e
	}
	if p.Suspended {
		return p, procErr("proc_suspended", 403)
	}
	if p.Version != version {
		return p, procConflict()
	}
	return p, nil
}
func procTouch(tx *gorm.DB, p *ProcurementProject) error {
	p.Version++
	p.UpdatedAt = time.Now().UTC()
	return tx.Save(p).Error
}

// Each account's mutation is serialized before checking the idempotency record.
// Project row locks are always acquired after the actor lock.
func (a *App) procWrite(c *gin.Context, payload any, action string, fn func(*gorm.DB, Account) (any, uint64, error)) {
	if !procurementEnabled() {
		fail(c, 503, "proc_disabled")
		return
	}
	who := actor(c)
	key := c.GetHeader("Idempotency-Key")
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{8,80}$`).MatchString(key) {
		fail(c, 400, "proc_operation_required")
		return
	}
	if !permit(fmt.Sprintf("proc-write:%d", who.ID), 120, time.Minute) {
		fail(c, 429, "rate_limited")
		return
	}
	raw, _ := json.Marshal(payload)
	fingerprint := digest(c.Request.Method + c.Request.URL.Path + string(raw))
	var result any
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		var current Account
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, who.ID).Error; e != nil {
			return e
		}
		var prior ProcurementEvent
		e := tx.Where("actor_id=? AND operation_id=?", who.ID, key).First(&prior).Error
		if e == nil {
			if prior.Fingerprint != fingerprint {
				return procConflict()
			}
			return json.Unmarshal([]byte(prior.Result), &result)
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		if !platformAccount(current) && !verifiedPhone(current) {
			return procErr("phone_binding_required", 403)
		}
		var projectID uint64
		result, projectID, e = fn(tx, current)
		if e != nil {
			return e
		}
		encoded, e := json.Marshal(result)
		if e != nil {
			return e
		}
		if e = tx.Create(&ProcurementEvent{ProjectID: projectID, ActorID: who.ID, Action: action, OperationID: &key, Fingerprint: fingerprint, Result: string(encoded), CreatedAt: time.Now().UTC()}).Error; e != nil {
			return e
		}
		return recordOperation(tx, who.ID, "procurement", projectID, action, "")
	})
	procResponse(c, result, err)
}
func procBusiness(c *gin.Context) {
	who := actor(c)
	if who.Role != "visitor" && who.Role != "merchant" {
		fail(c, 403, "forbidden")
		return
	}
	c.Next()
}
func (a *App) procurementRoutes(api, pub, admin *gin.RouterGroup) {
	p := api.Group("/procurement", a.requireAuth(""), procBusiness)
	p.GET("/profile", a.procProfile)
	p.PUT("/profile", a.procSaveProfile)
	p.GET("/projects", a.procListProjects)
	p.POST("/projects", a.procCreateProject)
	p.GET("/projects/:id", a.procProjectDetail)
	p.PUT("/projects/:id/draft", a.procSaveProject)
	p.POST("/projects/:id/submit", a.procSubmitProject)
	p.POST("/projects/:id/expand", a.procExpand)
	p.POST("/projects/:id/state", a.procSetState)
	p.POST("/projects/:id/invite", a.procInvite)
	p.GET("/projects/:id/matches", a.procMatches)
	p.POST("/projects/:id/join", a.procJoin)
	p.GET("/projects/:id/quotes", a.procQuotes)
	p.POST("/projects/:id/quotes", a.procSaveQuote)
	p.POST("/projects/:id/quote-withdraw", a.procWithdrawQuote)
	p.POST("/projects/:id/participants/:vendor/state", a.procParticipantState)
	p.GET("/projects/:id/messages/:vendor", a.procMessages)
	p.POST("/projects/:id/messages/:vendor", a.procSendMessage)
	p.POST("/projects/:id/attachments", a.procUploadAttachment)
	p.GET("/attachments/:id", a.procReadAttachment)
	p.HEAD("/attachments/:id", a.procReadAttachment)
	p.GET("/notifications", a.procNotifications)
	p.POST("/notifications/read", a.procReadNotifications)
	p.GET("/preferences", a.procPreferences)
	p.PUT("/preferences", a.procSavePreferences)
	pub.GET("/procurements", a.procPublicProjects)
	pub.GET("/procurements/:id", a.procPublicProject)
	pub.GET("/procurement-vendors", a.procVendorOptions)
	admin.GET("/procurement/projects", a.procAdminProjects)
	admin.GET("/procurement/projects/:id", a.procProjectDetail)
	admin.GET("/procurement/reviews", a.procReviews)
	admin.POST("/procurement/reviews/:id", a.procReview)
	admin.GET("/procurement/projects/:id/quotes", a.procQuotes)
	admin.GET("/procurement/projects/:id/matches", a.procMatches)
	admin.POST("/procurement/projects/:id/invite", a.procInvite)
	admin.POST("/procurement/projects/:id/moderate", a.procModerate)
	admin.POST("/procurement/accounts/:id/restrict", a.procRestrict)
	admin.GET("/procurement/attachments/:id", a.procReadAttachment)
	admin.GET("/procurement/stats", a.procStats)
}

func (a *App) procProfile(c *gin.Context) {
	who := actor(c)
	var profile BuyerProfile
	e := a.DB.First(&profile, "account_id=?", who.ID).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		c.JSON(200, gin.H{"profile": nil, "account": a.accountJSON(who)})
		return
	}
	procResponse(c, gin.H{"profile": profile, "account": a.accountJSON(who)}, e)
}
func (a *App) procSaveProfile(c *gin.Context) {
	var req struct {
		Company     string `json:"company"`
		ContactName string `json:"contact_name"`
		RegionCode  string `json:"region_code"`
		Version     uint64 `json:"version"`
	}
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.ContactName) == "" || !procText(req.ContactName, 100) || !procText(req.Company, 200) || !regexp.MustCompile(`^([0-9]{2,6})?$`).MatchString(req.RegionCode) {
		fail(c, 400, "proc_invalid")
		return
	}
	a.procWrite(c, req, "buyer_profile_saved", func(tx *gorm.DB, who Account) (any, uint64, error) {
		var b BuyerProfile
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&b, "account_id=?", who.ID).Error
		if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
			return nil, 0, e
		}
		if b.Suspended {
			return nil, 0, procErr("proc_suspended", 403)
		}
		if b.Version != req.Version {
			return nil, 0, procConflict()
		}
		b.AccountID = who.ID
		b.Company = strings.TrimSpace(req.Company)
		b.ContactName = strings.TrimSpace(req.ContactName)
		b.RegionCode = req.RegionCode
		b.Version++
		b.UpdatedAt = time.Now().UTC()
		return gin.H{"profile": b}, 0, tx.Save(&b).Error
	})
}

// A single policy protects every private project projection. Public browsing
// deliberately uses a separate query and response type.
func (a *App) procAccess(db *gorm.DB, who Account, p ProcurementProject) (*ProcurementParticipant, bool, error) {
	if who.ID == p.BuyerID || platformAccount(who) {
		return nil, true, nil
	}
	var participant ProcurementParticipant
	e := db.Table("procurement_participants pp").Select("pp.*").Joins("JOIN contents v ON v.id=pp.vendor_id AND v.kind='vendor'").Where("pp.project_id=? AND v.owner_id=? AND pp.state<>'excluded'", p.ID, who.ID).First(&participant).Error
	if e != nil {
		return nil, false, e
	}
	return &participant, false, nil
}
func procDisplayState(p ProcurementProject, r ProcurementRevision) string {
	if p.Suspended {
		return "suspended"
	}
	if procClosed(p) {
		return p.State
	}
	if p.PublishedRevisionID != 0 && time.Now().After(r.Deadline) {
		return "expired"
	}
	if p.PublishedRevisionID == 0 {
		return r.Status
	}
	return "open"
}
func (a *App) procProjectView(db *gorm.DB, who Account, p ProcurementProject, draft bool) (gin.H, error) {
	participant, owner, e := a.procAccess(db, who, p)
	if e != nil {
		return nil, e
	}
	revisionID := p.PublishedRevisionID
	if owner && (draft || revisionID == 0) {
		revisionID = p.DraftRevisionID
	}
	var rev ProcurementRevision
	if e = db.First(&rev, revisionID).Error; e != nil {
		return nil, e
	}
	var items []ProcurementItem
	if e = db.Where("revision_id=?", rev.ID).Order("id").Find(&items).Error; e != nil {
		return nil, e
	}
	attachments, e := procAttachments(db, p.ID, "requirement", rev.ID)
	if e != nil {
		return nil, e
	}
	view := gin.H{"id": p.ID, "hall_id": p.HallID, "state": procDisplayState(p, rev), "audience": p.Audience, "version": p.Version, "suspended": p.Suspended, "published_revision_id": p.PublishedRevisionID, "revision": rev, "items": items, "attachments": attachments, "owner": owner, "created_at": p.CreatedAt, "updated_at": p.UpdatedAt}
	if owner {
		var requestedVendorIDs []uint64
		if e = json.Unmarshal([]byte(rev.VendorIDs), &requestedVendorIDs); e != nil {
			return nil, e
		}
		view["requested_vendor_ids"] = requestedVendorIDs
		view["source_product_id"] = p.SourceProductID
		view["source_vendor_id"] = p.SourceVendorID
		view["draft_revision_id"] = p.DraftRevisionID
		var participants []ProcurementParticipant
		if e = db.Where("project_id=?", p.ID).Order("id").Find(&participants).Error; e != nil {
			return nil, e
		}
		view["participants"] = participants
		var reviews []ProcurementReview
		if e = db.Where("project_id=?", p.ID).Order("id DESC").Find(&reviews).Error; e != nil {
			return nil, e
		}
		view["reviews"] = reviews
		var events []ProcurementEvent
		if e = db.Where("project_id=?", p.ID).Order("id DESC").Limit(100).Find(&events).Error; e != nil {
			return nil, e
		}
		view["events"] = events
		var versions []ProcurementRevision
		if e = db.Where("project_id=?", p.ID).Order("id DESC").Find(&versions).Error; e != nil {
			return nil, e
		}
		view["versions"] = versions
	}
	if owner || participant != nil && participant.Intent {
		view["contact"] = gin.H{"name": rev.ContactName, "phone": rev.Phone}
	} else {
		view["contact"] = nil
	}
	if participant != nil {
		view["participant"] = participant
	}
	return view, nil
}
func (a *App) procProjectDetail(c *gin.Context) {
	var p ProcurementProject
	e := a.DB.First(&p, id(c)).Error
	if e != nil {
		procResponse(c, nil, e)
		return
	}
	selected, parseError := strconv.ParseUint(c.Query("revision_id"), 10, 64)
	historical := false
	currentPublished := p.PublishedRevisionID
	if c.Query("revision_id") != "" {
		if parseError != nil {
			procResponse(c, nil, procMissing())
			return
		}
		_, owner, accessError := a.procAccess(a.DB, actor(c), p)
		if accessError != nil {
			procResponse(c, nil, accessError)
			return
		}
		var revision ProcurementRevision
		if e = a.DB.Where("id=? AND project_id=?", selected, p.ID).First(&revision).Error; e != nil {
			procResponse(c, nil, e)
			return
		}
		if !owner && revision.Status != "approved" {
			procResponse(c, nil, procMissing())
			return
		}
		p.PublishedRevisionID = selected
		p.DraftRevisionID = selected
		historical = true
	}
	result, e := a.procProjectView(a.DB, actor(c), p, c.Query("draft") == "1")
	if e == nil {
		result["historical"] = historical
		result["published_revision_id"] = currentPublished
	}
	procResponse(c, result, e)
}
func procPaging(c *gin.Context) (int, int) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if p < 1 {
		p = 1
	}
	if p > 100000 {
		p = 100000
	}
	return p, 20
}
func (a *App) procListProjects(c *gin.Context) {
	who := actor(c)
	q := a.DB.Model(&ProcurementProject{}).Where("buyer_id=?", who.ID)
	if c.Query("scope") == "supplier" {
		q = a.DB.Model(&ProcurementProject{}).Where("id IN (SELECT pp.project_id FROM procurement_participants pp JOIN contents v ON v.id=pp.vendor_id WHERE v.owner_id=? AND pp.state<>'excluded')", who.ID)
	}
	a.procList(c, q, who)
}
func (a *App) procAdminProjects(c *gin.Context) {
	a.procList(c, a.DB.Model(&ProcurementProject{}), actor(c))
}
func (a *App) procList(c *gin.Context, q *gorm.DB, who Account) {
	page, size := procPaging(c)
	if state := c.Query("state"); state != "" {
		switch state {
		case "pending":
			q = q.Where("EXISTS (SELECT 1 FROM procurement_reviews pr WHERE pr.project_id=procurement_projects.id AND pr.status='pending')")
		case "draft", "rejected":
			q = q.Where("EXISTS (SELECT 1 FROM procurement_revisions pr WHERE pr.id=procurement_projects.draft_revision_id AND pr.status=?)", state)
		case "expired":
			q = q.Where("state='open' AND suspended=0 AND EXISTS (SELECT 1 FROM procurement_revisions pr WHERE pr.id=procurement_projects.published_revision_id AND pr.deadline<=?)", time.Now().UTC())
		case "open":
			q = q.Where("state='open' AND suspended=0 AND EXISTS (SELECT 1 FROM procurement_revisions pr WHERE pr.id=procurement_projects.published_revision_id AND pr.deadline>?)", time.Now().UTC())
		case "suspended":
			q = q.Where("suspended=1")
		default:
			q = q.Where("state=?", state)
		}
	}
	var total int64
	if e := q.Count(&total).Error; e != nil {
		procResponse(c, nil, e)
		return
	}
	var projects []ProcurementProject
	if e := q.Order("updated_at DESC,id DESC").Offset((page - 1) * size).Limit(size).Find(&projects).Error; e != nil {
		procResponse(c, nil, e)
		return
	}
	// Opt in to the Web list contract; legacy App callers retain the detail-shaped response.
	if c.Query("view") == "summary" {
		items, err := a.procProjectSummaries(who, projects)
		if err != nil {
			procResponse(c, nil, err)
			return
		}
		c.JSON(200, gin.H{"items": items, "total": total, "page": page, "page_size": size})
		return
	}
	items := []gin.H{}
	for _, p := range projects {
		v, e := a.procProjectView(a.DB, who, p, false)
		if e != nil {
			procResponse(c, nil, e)
			return
		}
		items = append(items, v)
	}
	c.JSON(200, gin.H{"items": items, "total": total, "page": page, "page_size": size})
}
func (a *App) procPublicProjects(c *gin.Context) {
	a.procPublicProjectsView(c, false)
}

func (a *App) procPublicProject(c *gin.Context) {
	a.procPublicProjectsView(c, true)
}

// Both views use exactly the same public visibility rules and field allowlist.
func (a *App) procPublicProjectsView(c *gin.Context, detail bool) {
	if !procurementEnabled() {
		fail(c, 503, "proc_disabled")
		return
	}
	page, size := procPaging(c)
	q := a.DB.Table("procurement_projects p").Joins("JOIN procurement_revisions r ON r.id=p.published_revision_id AND r.status='approved'").Joins("JOIN halls h ON h.id=p.hall_id AND h.status='open'").Joins("JOIN buyer_profiles b ON b.account_id=p.buyer_id AND b.suspended=0").Where("p.state='open' AND p.audience='public' AND p.suspended=0 AND r.deadline>?", time.Now().UTC())
	if detail {
		q = q.Where("p.id=?", c.Param("id"))
		page, size = 1, 1
	}
	if c.Query("state") == "closing_soon" {
		q = q.Where("r.deadline<=?", time.Now().Add(72*time.Hour))
	} else if c.Query("state") != "" && c.Query("state") != "open" {
		q = q.Where("1=0")
	}
	if hall := c.Query("hall_id"); hall != "" {
		q = q.Where("p.hall_id=?", hall)
	}
	if region := c.Query("region"); region != "" {
		q = q.Where("r.delivery_region LIKE ?", region+"%")
	}
	if category := c.Query("category_id"); category != "" {
		q = q.Where("EXISTS (SELECT 1 FROM procurement_items i WHERE i.revision_id=r.id AND i.category_id IN ?)", a.categoryIDs(category))
	}
	if kw := strings.TrimSpace(c.Query("q")); kw != "" {
		if !procText(kw, 200) {
			fail(c, 400, "proc_invalid")
			return
		}
		q = q.Where("(r.title LIKE ? OR EXISTS (SELECT 1 FROM procurement_items search_item WHERE search_item.revision_id=r.id AND search_item.name LIKE ?))", "%"+kw+"%", "%"+kw+"%")
	}
	var total int64
	if e := q.Count(&total).Error; e != nil {
		procResponse(c, nil, e)
		return
	}
	var projects []ProcurementProject
	if e := q.Select("p.*").Order("p.id DESC").Offset((page - 1) * size).Limit(size).Find(&projects).Error; e != nil {
		procResponse(c, nil, e)
		return
	}
	result := []gin.H{}
	for _, p := range projects {
		var r ProcurementRevision
		var items []ProcurementItem
		if e := a.DB.First(&r, p.PublishedRevisionID).Error; e != nil {
			procResponse(c, nil, e)
			return
		}
		if e := a.DB.Where("revision_id=?", r.ID).Find(&items).Error; e != nil {
			procResponse(c, nil, e)
			return
		}
		lines := []gin.H{}
		categoryIDs := []uint64{}
		for _, i := range items {
			categoryIDs = append(categoryIDs, i.CategoryID)
		}
		var categories []Category
		if len(categoryIDs) > 0 {
			if e := a.DB.Select("id,name,name_en").Where("id IN ?", categoryIDs).Find(&categories).Error; e != nil {
				procResponse(c, nil, e)
				return
			}
		}
		names := map[uint64]Category{}
		for _, category := range categories {
			names[category.ID] = category
		}
		for _, i := range items {
			lines = append(lines, gin.H{"name": i.Name, "category_id": i.CategoryID, "category_name": names[i.CategoryID].Name, "category_name_en": names[i.CategoryID].NameEN, "quantity": i.Quantity, "unit": i.Unit})
		}
		var review ProcurementReview
		if e := a.DB.Where("project_id=? AND revision_id=? AND audience='public' AND status='approved' AND reviewed_at IS NOT NULL", p.ID, r.ID).Order("reviewed_at ASC").Limit(1).Find(&review).Error; e != nil {
			procResponse(c, nil, e)
			return
		}
		result = append(result, gin.H{"id": p.ID, "hall_id": p.HallID, "title": r.Title, "delivery_region": r.DeliveryRegion, "deadline": r.Deadline, "published_at": review.ReviewedAt, "state": "open", "items": lines})
	}
	if detail {
		if len(result) == 0 {
			fail(c, 404, "not_found")
			return
		}
		c.JSON(200, result[0])
		return
	}
	c.JSON(200, gin.H{"items": result, "total": total, "page": page, "page_size": size})
}

func procAttachments(db *gorm.DB, project uint64, scope string, scopeID uint64) ([]ProcurementAttachment, error) {
	items := []ProcurementAttachment{}
	e := db.Where("project_id=? AND scope=? AND scope_id=?", project, scope, scopeID).Order("id").Find(&items).Error
	return items, e
}
func procBindAttachments(tx *gorm.DB, ids []uint64, project, owner uint64, scope string, scopeID uint64) error {
	if len(ids) > 10 {
		return procInvalid()
	}
	seen := map[uint64]bool{}
	for _, id := range ids {
		if seen[id] {
			return procInvalid()
		}
		seen[id] = true
		var f ProcurementAttachment
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND project_id=? AND owner_id=?", id, project, owner).First(&f).Error; e != nil {
			return procMissing()
		}
		if f.Scope != "" && (f.Scope != scope || f.ScopeID != scopeID) {
			f.ID = 0
			f.Scope = scope
			f.ScopeID = scopeID
			if e := tx.Create(&f).Error; e != nil {
				return e
			}
		} else {
			if e := tx.Model(&f).Updates(map[string]any{"scope": scope, "scope_id": scopeID}).Error; e != nil {
				return e
			}
		}
	}
	return nil
}

// All writes use server time. A supplier cannot race a quote against closure.
func procActive(tx *gorm.DB, p ProcurementProject) (ProcurementRevision, error) {
	var r ProcurementRevision
	if p.Suspended || procClosed(p) || p.PublishedRevisionID == 0 {
		return r, procErr("proc_not_open", 409)
	}
	var buyer BuyerProfile
	if e := tx.First(&buyer, "account_id=?", p.BuyerID).Error; e != nil {
		return r, e
	}
	if buyer.Suspended {
		return r, procErr("proc_suspended", 403)
	}
	e := tx.First(&r, p.PublishedRevisionID).Error
	return r, e
}
func procBeforeDeadline(tx *gorm.DB, p ProcurementProject) (ProcurementRevision, error) {
	r, e := procActive(tx, p)
	if e == nil && !r.Deadline.After(time.Now().UTC()) {
		e = procErr("proc_expired", 409)
	}
	return r, e
}
