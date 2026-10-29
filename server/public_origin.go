package main

import (
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/url"
	"strings"
	"time"
)

// Explicit host-operator configuration command, used after TLS verification.
func (a *App) setPublicOrigin(origin string) error {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("https_origin_required")
	}
	origin = strings.TrimRight(origin, "/")
	return a.DB.Transaction(func(tx *gorm.DB) error {
		var current SystemSetting
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("setting_key='system_config_v3'").Find(&current).Error; e != nil {
			return e
		}
		cfg, e := systemConfigDB(tx)
		if e != nil {
			return e
		}
		if cfg.Site.WebURL != "" && cfg.Site.WebURL != origin {
			var past SystemSetting
			var history []string
			tx.Where("setting_key='previous_site_origins'").Find(&past)
			_ = json.Unmarshal([]byte(past.Value), &history)
			history = append(history, cfg.Site.WebURL)
			if len(history) > 10 {
				history = history[len(history)-10:]
			}
			data, _ := json.Marshal(history)
			if e := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&SystemSetting{SettingKey: "previous_site_origins", Value: string(data), UpdatedAt: time.Now().UTC()}).Error; e != nil {
				return e
			}
		}
		cfg.Version++
		cfg.ProbeToken = ""
		cfg.Site.WebURL = origin
		cfg.Site.APIURL = origin + "/api/v1"
		cfg.Site.MediaURL = origin + "/api/v1/media"
		if cfg.Site.ICPNumber == "" {
			cfg.Site.ICPNumber = "冀ICP备2026047360号-2"
		}
		raw, e := json.Marshal(cfg)
		if e != nil {
			return e
		}
		row := SystemSetting{SettingKey: "system_config_v3", Value: string(raw), UpdatedAt: time.Now().UTC()}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "setting_key"}}, DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"})}).Create(&row).Error
	})
}
