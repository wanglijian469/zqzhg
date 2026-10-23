package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func procPreference(db *gorm.DB, account uint64) (ProcurementPreference, error) {
	pref := ProcurementPreference{AccountID: account, OpportunityMode: "instant"}
	e := db.First(&pref, "account_id=?", account).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return pref, nil
	}
	return pref, e
}
func (a *App) procNotice(tx *gorm.DB, account, project uint64, kind, title, key string, opportunity bool) error {
	now := time.Now().UTC()
	available := now
	if opportunity {
		pref, e := procPreference(tx, account)
		if e != nil {
			return e
		}
		if pref.OpportunityMode == "off" || pref.SupplierSuspended {
			return nil
		}
		if e = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&pref).Error; e != nil {
			return e
		}
		if e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&pref, "account_id=?", account).Error; e != nil {
			return e
		}
		zone := time.FixedZone("Asia/Shanghai", 8*3600)
		local := now.In(zone)
		day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, zone)
		var count int64
		if e = tx.Model(&ProcurementNotification{}).Where("account_id=? AND kind='opportunity' AND created_at>=?", account, day.UTC()).Count(&count).Error; e != nil {
			return e
		}
		kind = "opportunity"
		if pref.OpportunityMode == "daily" || count >= 20 {
			available = day.Add(18 * time.Hour)
			if !available.After(local) {
				available = available.Add(24 * time.Hour)
			}
			key = "digest:" + available.Format("2006-01-02")
			kind = "digest"
			title = "采购商机每日汇总，请到商机中心查看"
			project = 0
		}
	}
	n := ProcurementNotification{AccountID: account, ProjectID: project, EventKey: key, Kind: kind, Title: title, AvailableAt: available.UTC(), CreatedAt: now}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&n).Error
}
func (a *App) procNotifyParticipants(tx *gorm.DB, p ProcurementProject, kind, title, key string) error {
	var parts []ProcurementParticipant
	if e := tx.Where("project_id=? AND state<>'excluded'", p.ID).Find(&parts).Error; e != nil {
		return e
	}
	for _, part := range parts {
		var vendor Content
		if e := tx.First(&vendor, part.VendorID).Error; e != nil {
			return e
		}
		if e := a.procNotice(tx, vendor.OwnerID, p.ID, kind, title, fmt.Sprintf("%s:%d", key, part.VendorID), false); e != nil {
			return e
		}
	}
	return nil
}

// Match the same ownership rule used by TargetScope, including supplier digests
// without a project. Keep the default unscoped for existing clients.
func procNotificationScope(q *gorm.DB, account uint64, scope string) *gorm.DB {
	owned := "EXISTS (SELECT 1 FROM procurement_projects p WHERE p.id=procurement_notifications.project_id AND p.buyer_id=?)"
	if scope == "buyer" {
		return q.Where(owned, account)
	}
	if scope == "supplier" {
		return q.Where("NOT "+owned, account)
	}
	return q
}
func validProcNotificationScope(scope string) bool {
	return scope == "" || scope == "buyer" || scope == "supplier"
}
func (a *App) procNotifications(c *gin.Context) {
	scope := c.Query("scope")
	if !validProcNotificationScope(scope) {
		procResponse(c, nil, procInvalid())
		return
	}
	page, size := procPaging(c)
	now := time.Now().UTC()
	query := func() *gorm.DB {
		return procNotificationScope(a.DB.Model(&ProcurementNotification{}).Where("account_id=? AND available_at<=?", actor(c).ID, now), actor(c).ID, scope)
	}
	q := query()
	var total, unread int64
	var items []ProcurementNotification
	if e := q.Count(&total).Error; e != nil {
		procResponse(c, nil, e)
		return
	}
	if e := query().Where("read_at IS NULL").Count(&unread).Error; e != nil {
		procResponse(c, nil, e)
		return
	}
	e := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error
	if e == nil {
		ids := []uint64{}
		for _, item := range items {
			if item.ProjectID > 0 {
				ids = append(ids, item.ProjectID)
			}
		}
		buyers := map[uint64]uint64{}
		if len(ids) > 0 {
			var projects []ProcurementProject
			if e = a.DB.Select("id,buyer_id").Where("id IN ?", ids).Find(&projects).Error; e != nil {
				procResponse(c, nil, e)
				return
			}
			for _, project := range projects {
				buyers[project.ID] = project.BuyerID
			}
		}
		for i := range items {
			items[i].TargetScope = "supplier"
			if buyers[items[i].ProjectID] == actor(c).ID {
				items[i].TargetScope = "buyer"
			}
		}
	}
	procResponse(c, gin.H{"items": items, "total": total, "page": page, "page_size": size, "unread": unread}, e)
}
func (a *App) procReadNotifications(c *gin.Context) {
	var req struct {
		IDs   []uint64 `json:"ids"`
		Scope string   `json:"scope"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.IDs) > 100 || !validProcNotificationScope(req.Scope) {
		fail(c, 400, "proc_invalid")
		return
	}
	a.procWrite(c, req, "procurement_notifications_read", func(tx *gorm.DB, who Account) (any, uint64, error) {
		q := procNotificationScope(tx.Model(&ProcurementNotification{}).Where("account_id=? AND available_at<=?", who.ID, time.Now().UTC()), who.ID, req.Scope)
		if len(req.IDs) > 0 {
			q = q.Where("id IN ?", req.IDs)
		}
		e := q.Update("read_at", time.Now().UTC()).Error
		return gin.H{"ok": true}, 0, e
	})
}
func (a *App) procPreferences(c *gin.Context) {
	pref, e := procPreference(a.DB, actor(c).ID)
	procResponse(c, pref, e)
}
func (a *App) procSavePreferences(c *gin.Context) {
	var req struct {
		Mode string `json:"opportunity_mode"`
	}
	if c.ShouldBindJSON(&req) != nil || !containsProc([]string{"instant", "daily", "off"}, req.Mode) {
		fail(c, 400, "proc_invalid")
		return
	}
	a.procWrite(c, req, "procurement_preferences_saved", func(tx *gorm.DB, who Account) (any, uint64, error) {
		pref, e := procPreference(tx, who.ID)
		if e != nil {
			return nil, 0, e
		}
		pref.OpportunityMode = req.Mode
		pref.UpdatedAt = time.Now().UTC()
		return pref, 0, tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "account_id"}}, DoUpdates: clause.AssignmentColumns([]string{"opportunity_mode", "updated_at"})}).Create(&pref).Error
	})
}
