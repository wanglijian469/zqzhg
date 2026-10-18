package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"strings"
	"time"
	"unicode/utf8"
)

type HallConfiguration struct {
	Hall
	HeroMediaID uint64    `json:"hero_media_id"`
	Version     uint64    `json:"version"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (a *App) hallConfigurations() ([]HallConfiguration, error) {
	items := []HallConfiguration{}
	e := a.DB.Table("halls h").Select("h.*,s.hero_media_id,s.version,s.updated_at").Joins("JOIN home_display_settings s ON s.id=h.id").Order("h.id").Scan(&items).Error
	return items, e
}
func (a *App) adminHalls(c *gin.Context) { a.publicHalls(c) }
func (a *App) saveHall(c *gin.Context) {
	var req struct {
		Name          string `json:"name"`
		NameEN        string `json:"name_en"`
		Description   string `json:"description"`
		DescriptionEN string `json:"description_en"`
		Status        string `json:"status"`
		Slug          string `json:"slug"`
		HeroMediaID   uint64 `json:"hero_media_id"`
		Version       uint64 `json:"version"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.NameEN = strings.TrimSpace(req.NameEN)
	if req.Name == "" || req.NameEN == "" || utf8.RuneCountInString(req.Name) > 100 || utf8.RuneCountInString(req.NameEN) > 100 || utf8.RuneCountInString(req.Description) > 2000 || utf8.RuneCountInString(req.DescriptionEN) > 2000 || (req.Status != "open" && req.Status != "preparing") {
		fail(c, 400, "invalid_request")
		return
	}
	var saved HallConfiguration
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		settings, e := lockHomeDisplay(tx, id(c))
		if e != nil {
			return e
		}
		if req.Version != settings.Version {
			return fmt.Errorf("stale_revision")
		}
		var hall Hall
		if e = tx.First(&hall, id(c)).Error; e != nil {
			return e
		}
		if req.Slug != "" && req.Slug != hall.Slug {
			return fmt.Errorf("invalid_request")
		}
		if e = validateHomeHero(tx, req.HeroMediaID); e != nil {
			return e
		}
		hall.Name = req.Name
		hall.NameEN = req.NameEN
		hall.Description = req.Description
		hall.DescriptionEN = req.DescriptionEN
		hall.Status = req.Status
		if e = tx.Save(&hall).Error; e != nil {
			return e
		}
		settings.HeroMediaID = req.HeroMediaID
		settings.Version++
		settings.UpdatedBy = actor(c).ID
		settings.UpdatedAt = time.Now().UTC()
		if e = tx.Save(&settings).Error; e != nil {
			return e
		}
		saved = HallConfiguration{hall, settings.HeroMediaID, settings.Version, settings.UpdatedAt}
		return recordOperation(tx, actor(c).ID, "hall", hall.ID, "hall_settings_saved", fmt.Sprintf("slug=%s status=%s hero_media_id=%d", hall.Slug, hall.Status, req.HeroMediaID))
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, saved)
}
func validateHomeHero(tx *gorm.DB, mediaID uint64) error {
	if mediaID == 0 {
		return nil
	}
	var n int64
	if e := tx.Table("home_display_media hm").Joins("JOIN media m ON m.id=hm.media_id").Where("hm.media_id=?", mediaID).Count(&n).Error; e != nil {
		return e
	}
	if n != 1 {
		return fieldFailure("invalid_media", "hero_media_id", "homepage_image_required")
	}
	return nil
}
