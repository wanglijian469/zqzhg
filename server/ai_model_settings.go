package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"zqzt/exhibition/internal/bailian"
)

const aiFlashModel = "qwen3.8-flash"
const aiMaxModel = "qwen3.8-max"
const aiModelSettingKey = "ai_bailian_settings_v1"

type AIModelSettings struct {
	Version        uint64 `json:"version"`
	BaseURL        string `json:"base_url"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	AllowManualMax bool   `json:"allow_manual_max"`
	KeyMode        string `json:"key_mode"`
	EncryptedKey   string `json:"encrypted_key,omitempty"`
}

func aiDefaultModelSettings() AIModelSettings {
	c := bailian.FromEnv()
	return AIModelSettings{BaseURL: c.BaseURL, TimeoutSeconds: int(c.Timeout / time.Second), AllowManualMax: true, KeyMode: "environment"}
}
func (a *App) aiModelSettings() (AIModelSettings, error) {
	s := aiDefaultModelSettings()
	var row SystemSetting
	if e := a.DB.First(&row, "setting_key=?", aiModelSettingKey).Error; e != nil {
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return s, nil
		}
		return s, e
	}
	if json.Unmarshal([]byte(row.Value), &s) != nil {
		return s, errors.New("ai_invalid_model_settings")
	}
	return s, nil
}
func (a *App) aiBailianConfig() (bailian.Config, AIModelSettings, error) {
	c := bailian.FromEnv()
	s, e := a.aiModelSettings()
	c.Model = aiFlashModel
	c.ReviewModel = aiFlashModel
	if e != nil {
		return c, s, e
	}
	c.BaseURL = s.BaseURL
	c.Timeout = time.Duration(s.TimeoutSeconds) * time.Second
	switch s.KeyMode {
	case "admin":
		b, e := a.openSecret(s.EncryptedKey)
		if e != nil {
			return c, s, errors.New("ai_model_key_unreadable")
		}
		c.Key = string(b)
	case "disabled":
		c.Key = ""
	case "environment":
	default:
		return c, s, errors.New("ai_invalid_model_settings")
	}
	return c, s, nil
}
func (a *App) aiTaskConfig(t AIImportTask) (bailian.Config, error) {
	c, s, e := a.aiBailianConfig()
	if e != nil {
		return c, e
	}
	if t.ModelMode == "max" {
		if t.MaxConfirmedAt == nil {
			return c, errors.New("ai_max_confirmation_required")
		}
		if !s.AllowManualMax {
			return c, errors.New("ai_max_disabled")
		}
		c.Model = aiMaxModel
		c.ReviewModel = aiMaxModel
	} else if t.ModelMode != "" && t.ModelMode != "flash" {
		return c, errors.New("ai_invalid_model_mode")
	}
	return c, c.Validate()
}
func (a *App) aiModelView() (gin.H, error) {
	c, s, e := a.aiBailianConfig()
	if e != nil && e.Error() != "ai_model_key_unreadable" {
		return nil, e
	}
	code := ""
	if e != nil {
		code = e.Error()
	} else if c.Validate() != nil {
		code = "bailian_configuration_required"
	}
	return gin.H{"version": s.Version, "base_url": s.BaseURL, "timeout_seconds": s.TimeoutSeconds, "allow_manual_max": s.AllowManualMax, "default_model": aiFlashModel, "review_model": aiFlashModel, "advanced_model": aiMaxModel, "key_configured": c.Key != "" && e == nil, "key_source": s.KeyMode, "configured": code == "", "configuration_error": code, "enabled": aiEnabled()}, nil
}
func aiOfficialEndpoint(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "dashscope.aliyuncs.com" || host == "dashscope-intl.aliyuncs.com" || host == "dashscope-us.aliyuncs.com" || strings.HasSuffix(host, ".maas.aliyuncs.com")
}

func (a *App) aiModelSettingsRoutes(admin *gin.RouterGroup) {
	r := admin.Group("/settings/ai-models")
	r.Use(func(c *gin.Context) {
		if actor(c).Role != "admin" {
			fail(c, 403, "forbidden")
			return
		}
		c.Next()
	})
	r.GET("", func(c *gin.Context) {
		v, e := a.aiModelView()
		if e != nil {
			dbFail(c, e)
			return
		}
		c.JSON(200, v)
	})
	r.PUT("", a.aiSaveModelSettings)
	r.POST("/test", a.aiTestModelConnection)
}
func (a *App) aiSaveModelSettings(c *gin.Context) {
	var req struct {
		Version        uint64 `json:"version"`
		BaseURL        string `json:"base_url"`
		TimeoutSeconds int    `json:"timeout_seconds"`
		AllowManualMax bool   `json:"allow_manual_max"`
		KeyAction      string `json:"key_action"`
		APIKey         string `json:"api_key"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	req.BaseURL = strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	check := bailian.Config{Key: "validation-only", BaseURL: req.BaseURL}
	if check.Validate() != nil || !aiOfficialEndpoint(req.BaseURL) || req.TimeoutSeconds < 15 || req.TimeoutSeconds > 600 {
		fail(c, 400, "ai_invalid_model_settings")
		return
	}
	var sealed string
	if req.KeyAction == "replace" {
		req.APIKey = strings.TrimSpace(req.APIKey)
		check.Key = req.APIKey
		if len(req.APIKey) > 4096 || check.Validate() != nil {
			fail(c, 400, "bailian_configuration_required")
			return
		}
		var e error
		sealed, e = a.sealSecret([]byte(req.APIKey))
		if e != nil {
			fail(c, 503, "ai_model_encryption_required")
			return
		}
	}
	if req.KeyAction != "keep" && req.KeyAction != "replace" && req.KeyAction != "environment" && req.KeyAction != "disable" {
		fail(c, 400, "invalid_request")
		return
	}
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		initial := SystemSetting{SettingKey: aiModelSettingKey, Value: aiJSON(aiDefaultModelSettings()), UpdatedAt: time.Now().UTC()}
		if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&initial).Error; e != nil {
			return e
		}
		var row SystemSetting
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "setting_key=?", aiModelSettingKey).Error; e != nil {
			return e
		}
		var s AIModelSettings
		if json.Unmarshal([]byte(row.Value), &s) != nil {
			return errors.New("ai_invalid_model_settings")
		}
		if s.Version != req.Version {
			return serviceFailure{"stale_revision", 409}
		}
		s.BaseURL = req.BaseURL
		s.TimeoutSeconds = req.TimeoutSeconds
		s.AllowManualMax = req.AllowManualMax
		s.Version++
		switch req.KeyAction {
		case "replace":
			s.KeyMode = "admin"
			s.EncryptedKey = sealed
		case "environment":
			s.KeyMode = "environment"
			s.EncryptedKey = ""
		case "disable":
			s.KeyMode = "disabled"
			s.EncryptedKey = ""
		}
		if e := tx.Model(&row).Updates(map[string]any{"value": aiJSON(s), "updated_at": time.Now().UTC()}).Error; e != nil {
			return e
		}
		return recordOperation(tx, actor(c).ID, "ai_models", 0, "ai_models_saved", "default_flash;max_requires_explicit_task_confirmation")
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	v, e := a.aiModelView()
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, v)
}
func (a *App) aiTestModelConnection(c *gin.Context) {
	var req struct {
		Mode    string `json:"model_mode"`
		Confirm bool   `json:"confirm_max"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	t := AIImportTask{ModelMode: req.Mode}
	if req.Mode == "max" && req.Confirm {
		now := time.Now().UTC()
		t.MaxConfirmedAt = &now
	}
	cfg, e := a.aiTaskConfig(t)
	if e != nil {
		fail(c, 400, aiSafeError(e))
		return
	}
	if !permit("ai-model-test", 12, time.Minute) {
		fail(c, 429, "rate_limited")
		return
	}
	client := bailian.Client{Config: cfg}
	ctx, cancel := context.WithTimeout(c.Request.Context(), cfg.Timeout*3+10*time.Second)
	defer cancel()
	result, e := client.Complete(ctx, cfg.Model, "Return JSON only.", []bailian.Part{{Type: "text", Text: `Return {"ok":true}. This is a connectivity check; no brochure is attached.`}}, func(at bailian.Attempt) error {
		price, basis := aiEstimate(cfg.Model, at.Usage)
		return a.DB.Create(&AIImportModelCall{Stage: "admin-connectivity", Fingerprint: strings.Repeat("0", 64), Model: cfg.Model, RequestID: at.RequestID, Attempt: at.Number, HTTPStatus: at.Status, Error: at.Error, UsageJSON: aiJSON(at.Usage), EstimatedCost: price, PriceBasis: basis, DurationMS: at.DurationMS}).Error
	})
	if e != nil {
		fail(c, 502, aiSafeError(e))
		return
	}
	price, basis := aiEstimate(cfg.Model, result.Attempt.Usage)
	c.JSON(200, gin.H{"ok": true, "model": cfg.Model, "request_id": result.Attempt.RequestID, "usage": result.Attempt.Usage, "estimated_cost": price, "price_basis": basis})
}
