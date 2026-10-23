package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

type SiteConfig struct {
	Name     string `json:"name"`
	NameEN   string `json:"name_en"`
	WebURL   string `json:"web_url"`
	APIURL   string `json:"api_url"`
	MediaURL string `json:"media_url"`
}
type UploadConfig struct {
	ImageMiB          int   `json:"image_mib"`
	VideoMiB          int   `json:"video_mib"`
	PDFMiB            int   `json:"pdf_mib"`
	DefaultQuotaBytes int64 `json:"default_quota_bytes"`
}
type DiskConfig struct {
	WarnPercent     int    `json:"warn_percent"`
	CriticalPercent int    `json:"critical_percent"`
	MinFreeBytes    uint64 `json:"min_free_bytes"`
}
type SystemConfig struct {
	Watermark        WatermarkConfig `json:"watermark"`
	watermarkPresent bool
	Version          uint64       `json:"version"`
	Site             SiteConfig   `json:"site"`
	Uploads          UploadConfig `json:"uploads"`
	Disk             DiskConfig   `json:"disk"`
	ProbeToken       string       `json:"probe_token,omitempty"`
}

func defaultSystemConfig() SystemConfig {
	return SystemConfig{Watermark: defaultWatermarkConfig(), Version: 1, Site: SiteConfig{Name: "中企展馆", NameEN: "ZQZG", WebURL: "", APIURL: "", MediaURL: ""}, Uploads: UploadConfig{ImageMiB: settingInt("MAX_IMAGE_MIB", 10), VideoMiB: settingInt("MAX_VIDEO_MIB", 200), PDFMiB: settingInt("MAX_PDF_MIB", 50), DefaultQuotaBytes: 2 << 30}, Disk: DiskConfig{WarnPercent: 80, CriticalPercent: 90, MinFreeBytes: 1 << 30}}
}
func (a *App) systemConfig() (SystemConfig, error) { return systemConfigDB(a.DB) }
func systemConfigDB(db *gorm.DB) (SystemConfig, error) {
	cfg := defaultSystemConfig()
	var row SystemSetting
	e := db.First(&row, "setting_key=?", "system_config_v3").Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return cfg, nil
	}
	if e != nil {
		return cfg, e
	}
	if json.Unmarshal([]byte(row.Value), &cfg) != nil {
		return cfg, errors.New("invalid_system_settings")
	}
	return cfg, nil
}
func (a *App) effectiveConfig(c *gin.Context) SystemConfig {
	cfg, _ := a.systemConfig()
	base := strings.TrimRight(a.Config.PublicBaseURL, "/")
	if base == "" {
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		base = scheme + "://" + c.Request.Host
	}
	unsavedWebsite := cfg.Site.WebURL == ""
	if unsavedWebsite {
		cfg.Site.WebURL = base
	}
	serviceBase := cfg.Site.WebURL
	if unsavedWebsite && a.Config.Env != "production" {
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		serviceBase = scheme + "://" + c.Request.Host
	}
	if cfg.Site.APIURL == "" {
		cfg.Site.APIURL = serviceBase + "/api/v1"
	}
	if cfg.Site.MediaURL == "" {
		cfg.Site.MediaURL = serviceBase + "/media"
	}
	return cfg
}
func (a *App) uploadConfig() UploadConfig { cfg, _ := a.systemConfig(); return cfg.Uploads }
func (a *App) siteIdentity() string {
	var row SystemSetting
	if a.DB.First(&row, "setting_key=?", "site_identity").Error == nil {
		return row.Value
	}
	row = SystemSetting{SettingKey: "site_identity", Value: randomToken(), UpdatedAt: time.Now().UTC()}
	a.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	a.DB.First(&row, "setting_key=?", "site_identity")
	return row.Value
}
func (a *App) publicRuntimeConfig(c *gin.Context) {
	cfg := a.effectiveConfig(c)
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"version": cfg.Version, "site": cfg.Site, "uploads": cfg.Uploads, "watermark": cfg.Watermark, "site_id": a.siteIdentity(), "capabilities": a.publicCapabilities(), "demo_accounts": a.procurementDemoAccounts()})
}
func (a *App) publicMediaStatus(c *gin.Context) { c.JSON(200, gin.H{"site_id": a.siteIdentity()}) }
func normalizeConfig(cfg *SystemConfig) {
	cfg.Site.Name = strings.TrimSpace(cfg.Site.Name)
	cfg.Site.NameEN = strings.TrimSpace(cfg.Site.NameEN)
	cfg.Site.WebURL = strings.TrimRight(strings.TrimSpace(cfg.Site.WebURL), "/")
	cfg.Site.APIURL = strings.TrimRight(strings.TrimSpace(cfg.Site.APIURL), "/")
	cfg.Site.MediaURL = strings.TrimRight(strings.TrimSpace(cfg.Site.MediaURL), "/")
	cfg.ProbeToken = strings.TrimSpace(cfg.ProbeToken)
	normalizeWatermark(&cfg.Watermark)
}
func validateSystemConfig(cfg SystemConfig) error {
	if cfg.watermarkPresent || cfg.Watermark != (WatermarkConfig{}) {
		if e := validateWatermark(cfg.Watermark); e != nil {
			return e
		}
	}
	if cfg.Site.Name == "" || len([]rune(cfg.Site.Name)) > 100 || len([]rune(cfg.Site.NameEN)) > 100 {
		return errors.New("invalid_system_settings")
	}
	for i, s := range []string{cfg.Site.WebURL, cfg.Site.APIURL, cfg.Site.MediaURL} {
		u, e := url.Parse(s)
		if e != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") || len(s) > 1000 {
			return errors.New("invalid_system_settings")
		}
		if i == 0 && u.Path != "" {
			return errors.New("invalid_system_settings")
		}
		if strings.HasPrefix(cfg.Site.WebURL, "https:") && u.Scheme != "https" {
			return errors.New("mixed_content_address")
		}
	}
	if cfg.Uploads.ImageMiB < 1 || cfg.Uploads.ImageMiB > settingInt("IMAGE_UPLOAD_HARD_MAX_MIB", 20) || cfg.Uploads.VideoMiB < 1 || cfg.Uploads.VideoMiB > settingInt("VIDEO_UPLOAD_HARD_MAX_MIB", 1024) || cfg.Uploads.PDFMiB < 1 || cfg.Uploads.PDFMiB > settingInt("PDF_UPLOAD_HARD_MAX_MIB", 200) || cfg.Uploads.DefaultQuotaBytes < 1<<20 || cfg.Uploads.DefaultQuotaBytes > 1<<50 {
		return errors.New("invalid_system_settings")
	}
	if cfg.Disk.WarnPercent < 1 || cfg.Disk.CriticalPercent <= cfg.Disk.WarnPercent || cfg.Disk.CriticalPercent > 99 || cfg.Disk.MinFreeBytes < 64<<20 || cfg.Disk.MinFreeBytes > 1<<50 {
		return errors.New("invalid_system_settings")
	}
	return nil
}

type addressProof struct {
	Site    SiteConfig `json:"site"`
	Version uint64     `json:"version"`
	Expires int64      `json:"expires"`
}

func sameAddresses(a, b SiteConfig) bool {
	return a.WebURL == b.WebURL && a.APIURL == b.APIURL && a.MediaURL == b.MediaURL
}
func (a *App) safeProbe(ctx context.Context, raw, currentOrigin string) (map[string]any, error) {
	u, e := url.Parse(raw)
	if e != nil || u.User != nil || u.Fragment != "" || u.RawQuery != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("address_probe_denied")
	}
	host := u.Hostname()
	ips, e := net.DefaultResolver.LookupIPAddr(ctx, host)
	if e != nil || len(ips) == 0 {
		return nil, errors.New("address_probe_failed")
	}
	current, _ := url.Parse(currentOrigin)
	configured, _ := url.Parse(a.Config.PublicBaseURL)
	for _, ip := range ips {
		blocked := false
		for _, cidr := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "2001:db8::/32"} {
			_, network, _ := net.ParseCIDR(cidr)
			if network.Contains(ip.IP) {
				blocked = true
			}
		}
		safe := !blocked && ip.IP.IsGlobalUnicast() && !ip.IP.IsPrivate() && !ip.IP.IsLoopback() && !ip.IP.IsLinkLocalUnicast() && !ip.IP.IsLinkLocalMulticast()
		local := a.Config.Env != "production" && localServerIP(ip.IP) && (current != nil && u.Host == current.Host || configured != nil && u.Host == configured.Host)
		if !safe && !local {
			return nil, errors.New("address_probe_denied")
		}
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	target := net.JoinHostPort(ips[0].IP.String(), port)
	tr := &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, target)
	}}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, e := http.NewRequestWithContext(ctx, "GET", raw, nil)
	if e != nil {
		return nil, e
	}
	res, e := client.Do(req)
	if e != nil {
		return nil, errors.New("address_probe_failed")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, errors.New("address_probe_failed")
	}
	data, e := io.ReadAll(io.LimitReader(res.Body, 65537))
	if e != nil || len(data) > 65536 {
		return nil, errors.New("address_probe_failed")
	}
	var info map[string]any
	if json.Unmarshal(data, &info) != nil || info["site_id"] != a.siteIdentity() {
		return nil, errors.New("address_site_mismatch")
	}
	return info, nil
}
func (a *App) settingsRoutes(admin, pub *gin.RouterGroup) {
	a.watermarkRoutes(admin)
	pub.GET("/config", a.publicRuntimeConfig)
	admin.GET("/settings", func(c *gin.Context) {
		cfg := a.effectiveConfig(c)
		c.JSON(200, gin.H{"config": cfg, "paths": a.storagePaths(), "roots": a.localRoots(), "hard_limits": gin.H{"image_mib": settingInt("IMAGE_UPLOAD_HARD_MAX_MIB", 20), "video_mib": settingInt("VIDEO_UPLOAD_HARD_MAX_MIB", 1024), "pdf_mib": settingInt("PDF_UPLOAD_HARD_MAX_MIB", 200)}})
	})
	admin.POST("/settings/test-addresses", func(c *gin.Context) {
		var cfg SystemConfig
		if c.ShouldBindJSON(&cfg) != nil {
			fail(c, 400, "invalid_request")
			return
		}
		if !cfg.watermarkPresent {
			previous, err := a.systemConfig()
			if err != nil {
				dbFail(c, err)
				return
			}
			cfg.Watermark = previous.Watermark
		}
		normalizeConfig(&cfg)
		if e := validateSystemConfig(cfg); e != nil {
			fail(c, 400, e.Error())
			return
		}
		if !permit(fmt.Sprintf("site-probe:%d", actor(c).ID), 20, time.Hour) {
			fail(c, 429, "rate_limited")
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 25*time.Second)
		defer cancel()
		old := a.effectiveConfig(c)
		probeOrigin := old.Site.WebURL
		if a.Config.Env != "production" {
			scheme := "http"
			if c.Request.TLS != nil {
				scheme = "https"
			}
			probeOrigin = scheme + "://" + c.Request.Host
		}
		for _, target := range []string{cfg.Site.WebURL + "/api/v1/public/config", cfg.Site.APIURL + "/public/config", cfg.Site.MediaURL + "/status"} {
			if _, e := a.safeProbe(ctx, target, probeOrigin); e != nil {
				fail(c, 400, e.Error())
				return
			}
		}
		raw, _ := json.Marshal(addressProof{Site: cfg.Site, Version: cfg.Version, Expires: time.Now().Add(10 * time.Minute).Unix()})
		proof, e := a.sealSecret(raw)
		if e != nil {
			fail(c, 400, jobFailure(e))
			return
		}
		c.JSON(200, gin.H{"ok": true, "probe_token": proof})
	})
	admin.PUT("/settings", func(c *gin.Context) {
		var cfg SystemConfig
		if c.ShouldBindJSON(&cfg) != nil {
			fail(c, 400, "invalid_request")
			return
		}
		if !cfg.watermarkPresent {
			previous, err := a.systemConfig()
			if err != nil {
				dbFail(c, err)
				return
			}
			cfg.Watermark = previous.Watermark
		}
		normalizeConfig(&cfg)
		if e := validateSystemConfig(cfg); e != nil {
			fail(c, 400, e.Error())
			return
		}
		old := a.effectiveConfig(c)
		if !sameAddresses(cfg.Site, old.Site) {
			raw, e := a.openSecret(cfg.ProbeToken)
			var proof addressProof
			if e != nil || json.Unmarshal(raw, &proof) != nil || !sameAddresses(proof.Site, cfg.Site) || proof.Version != cfg.Version || proof.Expires < time.Now().Unix() {
				fail(c, 400, "address_probe_required")
				return
			}
		}
		cfg.ProbeToken = ""
		e := a.DB.Transaction(func(tx *gorm.DB) error {
			initial := defaultSystemConfig()
			raw, _ := json.Marshal(initial)
			row := SystemSetting{SettingKey: "system_config_v3", Value: string(raw), UpdatedAt: time.Now().UTC()}
			if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; e != nil {
				return e
			}
			if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "setting_key=?", "system_config_v3").Error; e != nil {
				return e
			}
			var previous SystemConfig
			if json.Unmarshal([]byte(row.Value), &previous) != nil {
				return errors.New("invalid_system_settings")
			}
			if previous.Version != cfg.Version {
				return errors.New("stale_revision")
			}
			if old.Site.WebURL != "" && old.Site.WebURL != cfg.Site.WebURL {
				var history []string
				var past SystemSetting
				tx.First(&past, "setting_key=?", "previous_site_origins")
				json.Unmarshal([]byte(past.Value), &history)
				history = append(history, old.Site.WebURL)
				if len(history) > 10 {
					history = history[len(history)-10:]
				}
				v, _ := json.Marshal(history)
				if e := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&SystemSetting{SettingKey: "previous_site_origins", Value: string(v), UpdatedAt: time.Now().UTC()}).Error; e != nil {
					return e
				}
			}
			cfg.Version++
			raw, _ = json.Marshal(cfg)
			if e := tx.Model(&row).Updates(map[string]any{"value": string(raw), "updated_at": time.Now().UTC()}).Error; e != nil {
				return e
			}
			before, _ := json.Marshal(previous)
			return recordOperation(tx, actor(c).ID, "settings", 0, "system_settings_saved", string(before)+" -> "+string(raw))
		})
		if e != nil {
			dbFail(c, e)
			return
		}
		c.JSON(200, gin.H{"config": cfg})
	})
}
func (a *App) storagePaths() gin.H {
	return gin.H{"media": a.Config.MediaDir, "temp": a.mediaTempDir(), "backup": a.localBackupDir(), "operations": a.operations().dir}
}
func (a *App) mediaTempDir() string {
	if p := strings.TrimSpace(env("MEDIA_TEMP_DIR", "")); p != "" {
		return filepath.Clean(p)
	}
	return filepath.Join(a.operations().dir, "work")
}
func (a *App) localBackupDir() string {
	if p := strings.TrimSpace(env("BACKUP_DIR", "")); p != "" {
		return filepath.Clean(p)
	}
	return filepath.Join(a.operations().dir, "backups")
}
func (a *App) mediaAddress(c *gin.Context, id uint64, query string) string {
	return a.effectiveConfig(c).Site.MediaURL + fmt.Sprintf("/%d", id) + query
}
func (a *App) brandName() string { cfg, _ := a.systemConfig(); return cfg.Site.Name }
func (a *App) runtimeHTML(c *gin.Context, page string) string {
	cfg := a.effectiveConfig(c)
	raw, _ := json.Marshal(gin.H{"version": cfg.Version, "site": cfg.Site, "uploads": cfg.Uploads, "watermark": cfg.Watermark, "capabilities": a.publicCapabilities(), "demo_accounts": a.procurementDemoAccounts()})
	return strings.Replace(page, "<head>", "<head><script id=\"zqzg-runtime-config\" type=\"application/json\">"+string(raw)+"</script>", 1)
}
func (a *App) allowedSiteOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	if origin == a.Config.CORS {
		return true
	}
	cfg, _ := a.systemConfig()
	orig := func(s string) string {
		u, e := url.Parse(s)
		if e != nil {
			return ""
		}
		return u.Scheme + "://" + u.Host
	}
	if origin == orig(cfg.Site.WebURL) || origin == orig(a.Config.PublicBaseURL) {
		return true
	}
	var history []string
	json.Unmarshal([]byte(a.setting("previous_site_origins", "[]")), &history)
	for _, s := range history {
		if origin == orig(s) {
			return true
		}
	}
	return false
}

func localServerIP(ip net.IP) bool {
	if ip.IsLoopback() {
		return true
	}
	addrs, e := net.InterfaceAddrs()
	if e != nil {
		return false
	}
	for _, addr := range addrs {
		if network, ok := addr.(*net.IPNet); ok && network.IP.Equal(ip) {
			return true
		}
	}
	return false
}
