package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type smsBridgeConfig struct{ Database, User, Password, HashKey string }

// Explicit, temporary synthetic fixture. The account accepts only IPv4 loopback,
// has access to this new DB only, and is revoked on stop or after 90 minutes.
// No production data or production credentials are read by this fixture.
func TestSMSTemporaryAcceptanceBridge(t *testing.T) {
	if os.Getenv("SMS_PREPARE_ACCEPTANCE") != "1" {
		t.Skip("operator-only temporary acceptance")
	}
	private := filepath.Join("..", ".local", "sms")
	if err := os.MkdirAll(private, 0700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(private, "bridge.json")
	stopPath := filepath.Join(private, "bridge.stop")
	if _, err := os.Stat(configPath); err == nil {
		t.Fatal("an acceptance bridge already exists; clean it up first")
	}
	_ = os.Remove(stopPath)
	cfg := smsBridgeConfig{Database: "zqzt_sms_acceptance_local_" + randomToken()[:12], User: "smsa_" + randomToken()[:10], Password: randomToken(), HashKey: randomToken()}
	root, err := openExistingDB("", env("DB_USER", "root"), env("DB_PASSWORD", "root"))
	if err != nil {
		t.Fatal("local operator connection unavailable")
	}
	defer func() { pool, _ := root.DB(); _ = pool.Close() }()
	t.Cleanup(func() {
		// Names are generated here, contain only ASCII hex, and cannot name business DBs.
		root.Exec("DROP USER IF EXISTS '" + cfg.User + "'@'127.0.0.1'")
		root.Exec("DROP DATABASE IF EXISTS `" + cfg.Database + "`")
		_ = os.Remove(configPath)
		_ = os.Remove(stopPath)
	})
	db, err := openDB(cfg.Database)
	if err != nil {
		t.Fatal("synthetic database creation failed")
	}
	defer func() { pool, _ := db.DB(); _ = pool.Close() }()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADMIN_PASSWORD", randomToken())
	if err = seed(db); err != nil {
		t.Fatal(err)
	}
	if err = root.Exec("CREATE USER '" + cfg.User + "'@'127.0.0.1' IDENTIFIED BY '" + cfg.Password + "' WITH MAX_USER_CONNECTIONS 8").Error; err != nil {
		t.Fatal("temporary loopback identity failed")
	}
	if err = root.Exec("GRANT ALL PRIVILEGES ON `" + cfg.Database + "`.* TO '" + cfg.User + "'@'127.0.0.1'").Error; err != nil {
		t.Fatal("temporary isolated grant failed")
	}
	saved := defaultSystemConfig()
	saved.Site.Name = "中企展馆（短信隔离验收）"
	saved.Site.WebURL = "http://127.0.0.1:18091"
	saved.Site.APIURL = "http://127.0.0.1:18091/api/v1"
	saved.Site.MediaURL = "http://127.0.0.1:18091/api/v1/media"
	raw, _ := json.Marshal(saved)
	if err = db.Create(&SystemSetting{SettingKey: "system_config_v3", Value: string(raw), UpdatedAt: time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(cfg)
	if err = os.WriteFile(configPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(90 * time.Minute)
	for time.Now().Before(deadline) {
		if _, err = os.Stat(stopPath); err == nil {
			return
		}
		time.Sleep(time.Second)
	}
}

// Recovery command if the operator session is interrupted before normal cleanup.
func TestSMSCleanupTemporaryAcceptance(t *testing.T) {
	if os.Getenv("SMS_CLEANUP_ACCEPTANCE") != "1" {
		t.Skip("operator-only acceptance cleanup")
	}
	path := filepath.Join("..", ".local", "sms", "bridge.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg smsBridgeConfig
	if json.Unmarshal(raw, &cfg) != nil || !strings.HasPrefix(cfg.Database, "zqzt_sms_acceptance_local_") || !strings.HasPrefix(cfg.User, "smsa_") || strings.ContainsAny(cfg.Database+cfg.User, "`' ;\\\n") {
		t.Fatal("refusing cleanup outside the isolated fixture")
	}
	root, err := openExistingDB("", env("DB_USER", "root"), env("DB_PASSWORD", "root"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { pool, _ := root.DB(); _ = pool.Close() }()
	if err = root.Exec("DROP USER IF EXISTS '" + cfg.User + "'@'127.0.0.1'").Error; err != nil {
		t.Fatal("identity cleanup failed")
	}
	if err = root.Exec("DROP DATABASE IF EXISTS `" + cfg.Database + "`").Error; err != nil {
		t.Fatal("database cleanup failed")
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
}
