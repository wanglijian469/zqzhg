package main

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type HomeRotationPool struct {
	VendorID  uint64    `gorm:"primaryKey" json:"vendor_id"`
	HallID    uint64    `json:"hall_id"`
	State     string    `json:"state"`
	Reason    string    `json:"reason"`
	UpdatedBy uint64    `json:"updated_by"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (HomeRotationPool) TableName() string { return "home_rotation_pool" }

type HomeRotationDay struct {
	HallID    uint64 `gorm:"primaryKey"`
	Day       string `gorm:"primaryKey"`
	CreatedAt time.Time
}
type HomeRotationSlot struct {
	HallID   uint64 `gorm:"primaryKey" json:"hall_id"`
	Day      string `gorm:"primaryKey" json:"day"`
	Position int    `gorm:"primaryKey" json:"position"`
	VendorID uint64 `json:"vendor_id"`
}

func rotationDay(now time.Time) string {
	return now.In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format("2006-01-02")
}
func completeness(v View) int {
	n := 0
	p := v.Data
	if strings.TrimSpace(p.Description) != "" {
		n++
	}
	if len(p.MediaIDs) > 0 {
		n++
	}
	if p.CompanyProfile != nil {
		if p.CompanyProfile.LogoMediaID > 0 {
			n++
		}
		if p.CompanyProfile.CoverMediaID > 0 {
			n++
		}
		if p.CompanyProfile.Summary != "" {
			n++
		}
	}
	return n
}
func (a *App) dailyHomeVendors(hallID uint64, now time.Time) ([]View, error) {
	selected := []View{}
	day := rotationDay(now)
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		if _, e := lockHomeDisplay(tx, hallID); e != nil {
			return e
		}
		app := &App{DB: tx}
		candidates, e := app.publicViews(app.publicQuery("vendor").Joins("JOIN home_rotation_pool hp ON hp.vendor_id=contents.id AND hp.state='approved'").Where("contents.hall_id=?", hallID).Order("contents.id"))
		if e != nil {
			return e
		}
		byID := map[uint64]View{}
		for _, v := range candidates {
			byID[v.ID] = v
		}
		var prior []HomeRotationSlot
		if e = tx.Where("hall_id=? AND day=?", hallID, day).Order("position").Find(&prior).Error; e != nil {
			return e
		}
		used := map[uint64]bool{}
		cats, regions := map[uint64]int{}, map[uint64]int{}
		for _, slot := range prior {
			if v, ok := byID[slot.VendorID]; ok && !used[v.ID] && len(selected) < 12 {
				selected = append(selected, v)
				used[v.ID] = true
				cats[effectiveViewCategory(v)]++
				regions[v.Data.RegionID]++
			}
		}
		var history []struct {
			VendorID uint64
			LastDay  string
		}
		if e = tx.Model(&HomeRotationSlot{}).Select("vendor_id,MAX(day) AS last_day").Where("hall_id=? AND day<?", hallID, day).Group("vendor_id").Scan(&history).Error; e != nil {
			return e
		}
		last := map[uint64]string{}
		for _, h := range history {
			last[h.VendorID] = h.LastDay
		}
		// Completeness includes reviewed product images and parameters, in one query.
		var productScores []struct {
			VendorID uint64
			Score    int
		}
		if e = app.publicQuery("product").Where("contents.hall_id=?", hallID).Select("contents.vendor_id,MAX(JSON_LENGTH(JSON_EXTRACT(r.payload,'$.media_ids'))>0)+MAX(COALESCE(JSON_UNQUOTE(JSON_EXTRACT(r.payload,'$.parameters')),'')<>'') AS score").Group("contents.vendor_id").Scan(&productScores).Error; e != nil {
			return e
		}
		scores := map[uint64]int{}
		for _, s := range productScores {
			scores[s.VendorID] = s.Score
		}
		for len(selected) < 12 {
			best := -1
			better := func(x, y View) bool {
				if last[x.ID] != last[y.ID] {
					return last[x.ID] < last[y.ID]
				}
				cx, cy := cats[effectiveViewCategory(x)], cats[effectiveViewCategory(y)]
				if cx != cy {
					return cx < cy
				}
				if regions[x.Data.RegionID] != regions[y.Data.RegionID] {
					return regions[x.Data.RegionID] < regions[y.Data.RegionID]
				}
				sx, sy := completeness(x)+scores[x.ID], completeness(y)+scores[y.ID]
				if sx != sy {
					return sx > sy
				}
				return x.ID < y.ID
			}
			for i, v := range candidates {
				if used[v.ID] {
					continue
				}
				if best < 0 || better(v, candidates[best]) {
					best = i
				}
			}
			if best < 0 {
				break
			}
			v := candidates[best]
			selected = append(selected, v)
			used[v.ID] = true
			cats[effectiveViewCategory(v)]++
			regions[v.Data.RegionID]++
		}
		changed := len(prior) != len(selected)
		if !changed {
			for i, v := range selected {
				if prior[i].VendorID != v.ID {
					changed = true
					break
				}
			}
		}
		var count int64
		if e = tx.Model(&HomeRotationDay{}).Where("hall_id=? AND day=?", hallID, day).Count(&count).Error; e != nil {
			return e
		}
		if changed || count == 0 {
			if e = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&HomeRotationDay{hallID, day, now.UTC()}).Error; e != nil {
				return e
			}
			if e = tx.Where("hall_id=? AND day=?", hallID, day).Delete(&HomeRotationSlot{}).Error; e != nil {
				return e
			}
			for i, v := range selected {
				if e = tx.Create(&HomeRotationSlot{hallID, day, i + 1, v.ID}).Error; e != nil {
					return e
				}
			}
		}
		return nil
	})
	return selected, err
}
func (a *App) startHomeRotation(ctx context.Context) {
	go func() {
		timer := time.NewTicker(time.Minute)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				runtime := a.operations()
				if runtime.maintenance.Load() || runtime.pauseWrites.Load() {
					continue
				}
				var halls []Hall
				if a.DB.Where("status='open'").Find(&halls).Error == nil {
					for _, h := range halls {
						_, _ = a.dailyHomeVendors(h.ID, time.Now())
					}
				}
			}
		}
	}()
}
func (a *App) homeRotationRoutes(admin *gin.RouterGroup) {
	admin.GET("/home-rotation", func(c *gin.Context) {
		hall, ok := a.requestHall(c)
		if !ok {
			return
		}
		items := []HomeRotationPool{}
		if e := a.DB.Where("hall_id=?", hall.ID).Order("updated_at DESC,vendor_id").Find(&items).Error; e != nil {
			dbFail(c, e)
			return
		}
		today, e := a.dailyHomeVendors(hall.ID, time.Now())
		if e != nil {
			dbFail(c, e)
			return
		}
		days := []HomeRotationDay{}
		if e = a.DB.Where("hall_id=?", hall.ID).Order("day DESC").Limit(31).Find(&days).Error; e != nil {
			dbFail(c, e)
			return
		}
		slots := []HomeRotationSlot{}
		if len(days) > 0 {
			if e = a.DB.Where("hall_id=? AND day>=?", hall.ID, days[len(days)-1].Day).Order("day DESC,position").Find(&slots).Error; e != nil {
				dbFail(c, e)
				return
			}
		}
		c.JSON(200, gin.H{"items": items, "today": today, "day": rotationDay(time.Now()), "history": slots, "limit": 12})
	})
	admin.PUT("/home-rotation/:id", func(c *gin.Context) {
		var req struct {
			State  string `json:"state"`
			Reason string `json:"reason"`
		}
		if c.ShouldBindJSON(&req) != nil || (req.State != "approved" && req.State != "rejected" && req.State != "removed") || strings.TrimSpace(req.Reason) == "" || len([]rune(req.Reason)) > 1000 {
			fail(c, 400, "invalid_request")
			return
		}
		var hallID uint64
		e := a.DB.Transaction(func(tx *gorm.DB) error {
			var v Content
			if e := tx.First(&v, id(c)).Error; e != nil {
				return e
			}
			if v.Kind != "vendor" {
				return serviceInvalid()
			}
			hallID = v.HallID
			if _, e := lockHomeDisplay(tx, hallID); e != nil {
				return e
			}
			if req.State == "approved" {
				var count int64
				if e := (&App{DB: tx}).publicQuery("vendor").Where("contents.id=?", v.ID).Count(&count).Error; e != nil {
					return e
				}
				if count == 0 {
					return serviceInvalid()
				}
			}
			row := HomeRotationPool{VendorID: v.ID, HallID: hallID, State: req.State, Reason: req.Reason, UpdatedBy: actor(c).ID, UpdatedAt: time.Now().UTC()}
			if e := tx.Save(&row).Error; e != nil {
				return e
			}
			if e := tx.Model(&v).Update("featured", req.State == "approved").Error; e != nil {
				return e
			}
			return recordOperation(tx, actor(c).ID, "home_pool", v.ID, "pool_"+req.State, req.Reason)
		})
		if e != nil {
			dbFail(c, e)
			return
		}
		if _, e = a.dailyHomeVendors(hallID, time.Now()); e != nil {
			dbFail(c, e)
			return
		}
		c.JSON(200, gin.H{"ok": true})
	})
}

// Keep historical free recommendations readable to older management clients.
func syncLegacyHomePool(tx *gorm.DB, content Content, featured bool, actorID uint64) error {
	if content.Kind != "vendor" {
		return nil
	}
	state := "removed"
	if featured {
		state = "approved"
	}
	row := HomeRotationPool{VendorID: content.ID, HallID: content.HallID, State: state, Reason: "后台人工免费推荐", UpdatedBy: actorID, UpdatedAt: time.Now().UTC()}
	return tx.Save(&row).Error
}
