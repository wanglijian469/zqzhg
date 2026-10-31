package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SearchSettings struct {
	ID                     uint64 `gorm:"primaryKey"`
	Payload, SealedSecrets string
	Version                uint64
	UpdatedAt              time.Time
}
type SearchEngineOption struct {
	Enabled    bool `json:"enabled"`
	Automatic  bool `json:"automatic"`
	DailyLimit int  `json:"daily_limit"`
}
type SearchOptions struct {
	Engines        map[string]SearchEngineOption `json:"engines"`
	Verification   map[string]string             `json:"verification"`
	GoogleProperty string                        `json:"google_property"`
}
type SearchSecrets struct {
	BaiduToken         string `json:"baidu_token"`
	IndexNowKey        string `json:"indexnow_key"`
	GoogleClientID     string `json:"google_client_id"`
	GoogleClientSecret string `json:"google_client_secret"`
	GoogleRefreshToken string `json:"google_refresh_token"`
}

var searchEngines = []string{"baidu", "indexnow", "google"}
var verificationNames = map[string]string{"google": "google-site-verification", "baidu": "baidu-site-verification", "bing": "msvalidate.01", "360": "360-site-verification", "sogou": "sogou_site_verification", "shenma": "shenma-site-verification"}

func defaultSearchOptions() SearchOptions {
	return SearchOptions{Engines: map[string]SearchEngineOption{"baidu": {DailyLimit: 100}, "indexnow": {DailyLimit: 200}, "google": {DailyLimit: 2}}, Verification: map[string]string{}}
}
func searchBase(a *App) string {
	cfg, e := a.systemConfig()
	if e == nil && cfg.Site.WebURL != "" {
		return strings.TrimRight(cfg.Site.WebURL, "/")
	}
	return strings.TrimRight(a.Config.PublicBaseURL, "/")
}
func (a *App) searchSettings() (SearchSettings, SearchOptions, SearchSecrets, error) {
	row := SearchSettings{}
	options := defaultSearchOptions()
	secrets := SearchSecrets{}
	e := a.DB.First(&row, 1).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return row, options, secrets, nil
	}
	if e != nil {
		return row, options, secrets, e
	}
	if e = json.Unmarshal([]byte(row.Payload), &options); e != nil {
		return row, options, secrets, e
	}
	if options.Verification == nil {
		options.Verification = map[string]string{}
	}
	if row.SealedSecrets != "" {
		raw, err := a.openSecret(row.SealedSecrets)
		if err != nil {
			return row, options, secrets, err
		}
		if e = json.Unmarshal(raw, &secrets); e != nil {
			return row, options, secrets, e
		}
	}
	return row, options, secrets, nil
}
func searchConfigured(engine string, s SearchSecrets) bool {
	switch engine {
	case "baidu":
		return s.BaiduToken != ""
	case "indexnow":
		return s.IndexNowKey != ""
	case "google":
		return s.GoogleClientID != "" && s.GoogleClientSecret != "" && s.GoogleRefreshToken != ""
	}
	return false
}
func (a *App) searchSettingsJSON() (gin.H, error) {
	r, o, s, e := a.searchSettings()
	if e != nil {
		return nil, e
	}
	configured := map[string]bool{}
	for _, engine := range searchEngines {
		configured[engine] = searchConfigured(engine, s)
	}
	base := searchBase(a)
	return gin.H{"version": r.Version, "options": o, "configured": configured, "base_url": base, "sitemap_url": base + "/sitemap.xml", "indexnow_key_url": base + "/indexnow.txt"}, nil
}
func (a *App) getSearchSettings(c *gin.Context) {
	r, e := a.searchSettingsJSON()
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, r)
}
func (a *App) searchPublicOptions() SearchOptions {
	o := defaultSearchOptions()
	var r SearchSettings
	if a.DB.Select("payload").First(&r, 1).Error == nil {
		_ = json.Unmarshal([]byte(r.Payload), &o)
	}
	return o
}
func validSearchBase(base string) bool {
	u, e := url.Parse(base)
	return e == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.RawQuery == "" && (u.Path == "" || u.Path == "/") && u.Fragment == ""
}
func (a *App) saveSearchSettings(c *gin.Context) {
	var req struct {
		Version          uint64            `json:"version"`
		Options          SearchOptions     `json:"options"`
		Secrets          map[string]string `json:"secrets"`
		ClearSecrets     []string          `json:"clear_secrets"`
		GenerateIndexNow bool              `json:"generate_indexnow"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	base := searchBase(a)
	if !validSearchBase(base) {
		fail(c, 400, "seo_public_https_required")
		return
	}
	if len(req.Options.Engines) != len(searchEngines) {
		fail(c, 400, "invalid_request")
		return
	}
	for _, name := range searchEngines {
		value, ok := req.Options.Engines[name]
		if !ok || value.DailyLimit < 1 || value.DailyLimit > 10000 {
			fail(c, 400, "invalid_request")
			return
		}
	}
	for name, value := range req.Options.Verification {
		if verificationNames[name] == "" || len(value) > 512 || (value != "" && !searchCodePattern.MatchString(value)) {
			fail(c, 400, "invalid_request")
			return
		}
	}
	host, _ := url.Parse(base)
	if req.Options.GoogleProperty != "" && req.Options.GoogleProperty != base+"/" && req.Options.GoogleProperty != "sc-domain:"+host.Hostname() {
		fail(c, 400, "seo_property_mismatch")
		return
	}
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		ta := &App{DB: tx, Config: a.Config}
		row, _, secrets, e := ta.searchSettings()
		if e != nil {
			return e
		}
		if row.ID == 0 {
			raw, _ := json.Marshal(defaultSearchOptions())
			row = SearchSettings{ID: 1, Payload: string(raw), Version: 0, UpdatedAt: time.Now().UTC()}
			if e = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; e != nil {
				return e
			}
		}
		if e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, 1).Error; e != nil {
			return e
		}
		if row.Version != req.Version {
			return fmt.Errorf("stale_revision")
		}
		// Reload encrypted values under the same row lock; never accept a masked placeholder.
		_, _, secrets, e = ta.searchSettings()
		if e != nil {
			return e
		}
		fields := map[string]*string{"baidu_token": &secrets.BaiduToken, "google_client_id": &secrets.GoogleClientID, "google_client_secret": &secrets.GoogleClientSecret, "google_refresh_token": &secrets.GoogleRefreshToken}
		for key, value := range req.Secrets {
			p, ok := fields[key]
			if !ok || len(value) > 4096 || strings.ContainsAny(value, "\r\n\x00") {
				return fmt.Errorf("invalid_request")
			}
			if value != "" {
				*p = value
			}
		}
		for _, key := range req.ClearSecrets {
			if key == "indexnow_key" {
				secrets.IndexNowKey = ""
			} else if p, ok := fields[key]; ok {
				*p = ""
			} else {
				return fmt.Errorf("invalid_request")
			}
		}
		if req.GenerateIndexNow {
			secrets.IndexNowKey = randomToken()
		}
		for name, option := range req.Options.Engines {
			if option.Enabled && !searchConfigured(name, secrets) {
				return fmt.Errorf("seo_credentials_required")
			}
		}
		raw, _ := json.Marshal(secrets)
		sealed, e := a.sealSecret(raw)
		if e != nil {
			return e
		}
		payload, _ := json.Marshal(req.Options)
		if e = tx.Model(&row).Updates(map[string]any{"payload": string(payload), "sealed_secrets": sealed, "version": row.Version + 1, "updated_at": time.Now().UTC()}).Error; e != nil {
			return e
		}
		return recordOperation(tx, actor(c).ID, "seo", 1, "seo_settings_saved", "Search settings updated; credentials omitted")
	})
	if e != nil {
		if e.Error() == "seo_credentials_required" || e.Error() == "seo_property_mismatch" {
			fail(c, 400, e.Error())
		} else {
			dbFail(c, e)
		}
		return
	}
	a.getSearchSettings(c)
}
func (a *App) searchVerificationFile(c *gin.Context) {
	_, _, s, e := a.searchSettings()
	if e != nil || s.IndexNowKey == "" {
		c.Status(404)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(s.IndexNowKey))
}

var searchCodePattern = regexp.MustCompile(`^[A-Za-z0-9._=+/-]{1,512}$`)
