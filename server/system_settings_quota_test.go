package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestSettingsQuotaSaveWithCompleteAuditAndSafeDowngrade(t *testing.T) {
	a := importTestAppAtSchema(t, 37)
	pool, err := a.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	pool.SetMaxOpenConns(1)
	if err = a.DB.Exec("SET SESSION sql_mode='STRICT_ALL_TABLES'").Error; err != nil {
		t.Fatal(err)
	}
	tc := testClient{t, a.router()}
	admin := resourceTestSession(t, a, "admin")
	merchant := resourceTestSession(t, a, "merchant")
	result := tc.call("GET", "/admin/settings", admin, nil, 200)
	raw, _ := json.Marshal(result["config"])
	var cfg SystemConfig
	json.Unmarshal(raw, &cfg)
	// Valid long addresses also reproduce the production full-snapshot overflow.
	cfg.Site.MediaURL += "/" + strings.Repeat("a", 700)
	stored, _ := json.Marshal(cfg)
	if err := a.DB.Create(&SystemSetting{SettingKey: "system_config_v3", Value: string(stored)}).Error; err != nil {
		t.Fatal(err)
	}
	cfg.Uploads.DefaultQuotaBytes = 300000000
	tc.call("PUT", "/admin/settings", admin, cfg, 500)
	current, _ := a.systemConfig()
	if current.Version != cfg.Version || current.Uploads.DefaultQuotaBytes != 200000000 {
		t.Fatal("failed audit changed settings")
	}
	if err := migrate(a.DB); err != nil {
		t.Fatal(err)
	}
	tc.call("PUT", "/admin/settings", merchant, cfg, 403)
	tc.call("PUT", "/admin/settings", admin, cfg, 200)
	current, _ = a.systemConfig()
	if current.Version != cfg.Version+1 || current.Uploads.DefaultQuotaBytes != 300000000 {
		t.Fatal("quota was not persisted")
	}
	var log OperationLog
	if err := a.DB.Where("action=?", "system_settings_saved").Order("id DESC").First(&log).Error; err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(log.Note, " -> ", 2)
	if len([]rune(log.Note)) <= 1000 || len(parts) != 2 || !json.Valid([]byte(parts[0])) || !json.Valid([]byte(parts[1])) {
		t.Fatal("full audit snapshots not retained")
	}
	var before, after SystemConfig
	json.Unmarshal([]byte(parts[0]), &before)
	json.Unmarshal([]byte(parts[1]), &after)
	if before.Uploads.DefaultQuotaBytes != 200000000 || after.Uploads.DefaultQuotaBytes != 300000000 || after.Site.MediaURL != cfg.Site.MediaURL {
		t.Fatal("audit values changed")
	}
	tc.call("PUT", "/admin/settings", admin, cfg, 409)
	rollback, err := os.ReadFile("../deploy/linux/rollback-operation-log-038.sql")
	if err != nil {
		t.Fatal(err)
	}
	statements := strings.Split(string(rollback), ";")
	for i, sql := range statements {
		if strings.TrimSpace(sql) == "" {
			continue
		}
		err = a.DB.Exec(sql).Error
		if i == 1 {
			if err == nil {
				t.Fatal("unsafe shrink did not refuse oversized audit history")
			}
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	var retained OperationLog
	a.DB.First(&retained, log.ID)
	if retained.Note != log.Note {
		t.Fatal("failed downgrade truncated audit history")
	}
	// Only this isolated fixture is removed to exercise the permitted shrink path.
	if err = a.DB.Delete(&OperationLog{}, log.ID).Error; err != nil {
		t.Fatal(err)
	}
	for _, sql := range statements {
		if strings.TrimSpace(sql) != "" {
			if err = a.DB.Exec(sql).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	if err = migrate(a.DB); err != nil {
		t.Fatal(err)
	}
}
