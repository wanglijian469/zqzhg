package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	driver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openExistingDB(name, user, password string) (*gorm.DB, error) {
	if name != "" && !regexp.MustCompile(`^[a-zA-Z0-9_]{1,64}$`).MatchString(name) {
		return nil, errors.New("invalid_database_name")
	}
	cfg := driver.NewConfig()
	cfg.User = user
	cfg.Passwd = password
	cfg.Net = "tcp"
	cfg.Addr = env("DB_HOST", "127.0.0.1") + ":" + env("DB_PORT", "13306")
	cfg.DBName = name
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	cfg.Timeout = 5 * time.Second
	cfg.Params = map[string]string{"charset": "utf8mb4"}
	cfg.AllowNativePasswords = true
	cfg.AllowAllFiles = false
	return gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
}
func (a *App) createRestoreDatabase(ctx context.Context, name, sqlPath string) (*gorm.DB, error) {
	if !regexp.MustCompile(`^zqzt_exhibition_restore_[a-f0-9]{16}$`).MatchString(name) {
		return nil, errors.New("invalid_database_name")
	}
	maintUser := os.Getenv("DB_MAINTENANCE_USER")
	maintPassword := os.Getenv("DB_MAINTENANCE_PASSWORD")
	if maintUser == "" {
		if a.Config.Env == "production" {
			return nil, errors.New("maintenance_credentials_required")
		}
		maintUser = env("DB_USER", "root")
		maintPassword = env("DB_PASSWORD", "root")
	}
	maintenance, err := openExistingDB("", maintUser, maintPassword)
	if err != nil {
		return nil, errors.New("maintenance_credentials_unavailable")
	}
	pool, _ := maintenance.DB()
	defer pool.Close()
	if err = maintenance.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci").Error; err != nil {
		return nil, errors.New("maintenance_credentials_or_database_exists")
	}
	limitedUser := "zqr_" + randomToken()[:12]
	limitedPassword := randomToken()
	if err = maintenance.Exec("CREATE USER '" + limitedUser + "'@'%' IDENTIFIED BY '" + limitedPassword + "'").Error; err != nil {
		return nil, errors.New("maintenance_credentials_create_user")
	}
	defer maintenance.Exec("DROP USER IF EXISTS '" + limitedUser + "'@'%'")
	if err = maintenance.Exec("GRANT ALL PRIVILEGES ON `" + name + "`.* TO '" + limitedUser + "'@'%'").Error; err != nil {
		return nil, errors.New("maintenance_credentials_grant")
	}
	tool, err := mysqlTool("mysql")
	if err != nil {
		return nil, err
	}
	input, err := os.Open(sqlPath)
	if err != nil {
		return nil, err
	}
	args := append(mysqlArgs(limitedUser), "--binary-mode=1", "--local-infile=0", name)
	command := exec.CommandContext(ctx, tool, args...)
	command.Env = mysqlEnvironment(limitedPassword)
	command.Stdin = input
	err = command.Run()
	input.Close()
	if err != nil {
		return nil, errors.New("restore_validation_sql_import")
	}
	var currentUser string
	if err = a.DB.Raw("SELECT CURRENT_USER()").Scan(&currentUser).Error; err != nil {
		return nil, err
	}
	parts := strings.SplitN(currentUser, "@", 2)
	safe := regexp.MustCompile(`^[a-zA-Z0-9_.%:\-]+$`)
	if len(parts) != 2 || !safe.MatchString(parts[0]) || !safe.MatchString(parts[1]) {
		return nil, errors.New("maintenance_credentials_invalid_application_user")
	}
	if err = maintenance.Exec("GRANT ALL PRIVILEGES ON `" + name + "`.* TO '" + parts[0] + "'@'" + parts[1] + "'").Error; err != nil {
		return nil, errors.New("maintenance_credentials_application_grant")
	}
	return openExistingDB(name, env("DB_USER", "root"), env("DB_PASSWORD", "root"))
}
func bootstrapTarget(mediaDir, defaultDatabase string) (ActiveTarget, error) {
	target := ActiveTarget{defaultDatabase, mediaDir}
	host, err := readHostTarget(mediaDir)
	if err != nil {
		return target, err
	}
	if host.Current.Database == "" {
		return target, nil
	}
	if host.Pending {
		if host.Previous == nil {
			return target, errors.New("invalid_pending_restore")
		}
		host.Current = *host.Previous
		host.Previous = nil
		host.Pending = false
		host.Outcome = "rolled_back"
		if err = atomicJSON(filepath.Join(controlDirectory(mediaDir), "active.json"), host); err != nil {
			return target, err
		}
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9_]{1,64}$`).MatchString(host.Current.Database) || !filepath.IsAbs(host.Current.MediaDir) {
		return target, errors.New("invalid_active_target")
	}
	return host.Current, nil
}
