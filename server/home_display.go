package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

const homeVendorLimit = 12

type HomeDisplaySettings struct {
	ID          uint64    `gorm:"primaryKey" json:"-"`
	HeroMediaID uint64    `json:"hero_media_id"`
	Version     uint64    `json:"version"`
	UpdatedBy   uint64    `json:"-"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (HomeDisplaySettings) TableName() string { return "home_display_settings" }

type HomeDisplayMedia struct {
	MediaID   uint64 `gorm:"primaryKey"`
	CreatedAt time.Time
}

func (HomeDisplayMedia) TableName() string { return "home_display_media" }
func lockHomeDisplay(tx *gorm.DB) (HomeDisplaySettings, error) {
	var s HomeDisplaySettings
	e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&s, 1).Error
	return s, e
}
func (a *App) homeDisplay(c *gin.Context) {
	var settings HomeDisplaySettings
	if e := a.DB.First(&settings, 1).Error; e != nil {
		dbFail(c, e)
		return
	}
	var count int64
	if e := a.publicQuery("vendor").Where("contents.featured=1").Count(&count).Error; e != nil {
		dbFail(c, e)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"hero_media_id": settings.HeroMediaID, "version": settings.Version, "vendor_limit": homeVendorLimit, "selected_vendor_count": count, "updated_at": settings.UpdatedAt})
}
func (a *App) homeDisplayUpload(c *gin.Context) { c.Set("upload_home_display", true); a.upload(c) }
func (a *App) saveHomeDisplay(c *gin.Context) {
	var req struct {
		HeroMediaID uint64 `json:"hero_media_id"`
		Version     uint64 `json:"version"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	var saved HomeDisplaySettings
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		settings, e := lockHomeDisplay(tx)
		if e != nil {
			return e
		}
		if req.Version != settings.Version {
			return fmt.Errorf("stale_revision")
		}
		if req.HeroMediaID > 0 {
			var count int64
			if e := tx.Table("home_display_media hm").Joins("JOIN media m ON m.id=hm.media_id").Where("hm.media_id=?", req.HeroMediaID).Count(&count).Error; e != nil {
				return e
			}
			if count != 1 {
				return fieldFailure("invalid_media", "hero_media_id", "homepage_image_required")
			}
		}
		settings.HeroMediaID = req.HeroMediaID
		settings.Version++
		settings.UpdatedBy = actor(c).ID
		settings.UpdatedAt = time.Now().UTC()
		if e := tx.Save(&settings).Error; e != nil {
			return e
		}
		saved = settings
		return recordOperation(tx, actor(c).ID, "home_display", 1, "home_display_saved", fmt.Sprintf("hero_media_id=%d", req.HeroMediaID))
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, saved)
}
