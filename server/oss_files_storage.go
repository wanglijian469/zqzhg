package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type cloudFilesRuntime struct {
	mu         sync.Mutex
	processMu  sync.Mutex
	workerOnce sync.Once
	clients    map[string]*oss.Client
	provider   credentials.CredentialsProvider // Tests may inject a local provider; never set by HTTP input.
	gcAfter    string
}

func (a *App) cloudFiles() *cloudFilesRuntime {
	a.cloudOnce.Do(func() { a.cloudState = &cloudFilesRuntime{clients: map[string]*oss.Client{}} })
	return a.cloudState
}
func cloudFilesEnabled() bool { return env("OSS_ENABLED", "false") == "true" }
func (a *App) cloudStorageInitialized() bool {
	var n int64
	return a.DB.Model(&SystemSetting{}).Where("setting_key='oss_quota_300mib_v1'").Count(&n).Error != nil || n > 0
}

type cloudFileConfig struct {
	Region        string `json:"region"`
	Endpoint      string `json:"endpoint"`
	PrivateBucket string `json:"private_bucket"`
	PublicBucket  string `json:"public_bucket"`
	UploadBase    string `json:"upload_base_url"`
	PublicBase    string `json:"public_base_url"`
	Role          string `json:"ecs_role_name"`
}

func cloudConfig() cloudFileConfig {
	return cloudFileConfig{env("OSS_REGION", "cn-beijing"), env("OSS_INTERNAL_ENDPOINT", "https://oss-cn-beijing-internal.aliyuncs.com"), env("OSS_PRIVATE_BUCKET", "zqzhg-private-data"), env("OSS_PUBLIC_BUCKET", "zqzhg-public-media"), strings.TrimRight(env("OSS_PRIVATE_UPLOAD_BASE_URL", "https://upload.zqzhg.com"), "/"), strings.TrimRight(env("OSS_PUBLIC_BASE_URL", "https://img.zqzhg.com"), "/"), env("OSS_ECS_ROLE_NAME", "ZQZHG-SMS-Role")}
}
func (cfg cloudFileConfig) digest() string {
	b, _ := json.Marshal(cfg)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
func (cfg cloudFileConfig) validate() error {
	if cfg.Region != "cn-beijing" || cfg.Role == "" || cfg.PrivateBucket == "" || cfg.PublicBucket == "" || cfg.PrivateBucket == cfg.PublicBucket || cfg.PrivateBucket == "zqzhg-backup" || cfg.PublicBucket == "zqzhg-backup" {
		return errors.New("invalid_oss_configuration")
	}
	for _, address := range []string{cfg.Endpoint, cfg.UploadBase, cfg.PublicBase} {
		u, e := url.Parse(address)
		if e != nil || u.Scheme != "https" || u.User != nil || u.Hostname() == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return errors.New("invalid_oss_configuration")
		}
	}
	if cfg.Endpoint != "https://oss-cn-beijing-internal.aliyuncs.com" {
		return errors.New("invalid_oss_configuration")
	}
	return nil
}
func (a *App) initializeCloudFiles() error {
	if !cloudFilesEnabled() {
		return nil
	}
	cfg := cloudConfig()
	if e := cfg.validate(); e != nil {
		return e
	}
	for _, kind := range []string{"private", "public"} {
		bucket, base := cfg.PrivateBucket, cfg.UploadBase
		if kind == "public" {
			bucket, base = cfg.PublicBucket, cfg.PublicBase
		}
		p := StorageProfile{Name: "Enterprise OSS " + kind, Kind: "oss", Region: cfg.Region, Endpoint: cfg.Endpoint, Bucket: bucket, AuthMode: "ecs_role", RoleName: cfg.Role, Purpose: "enterprise_" + kind, PublicBaseURL: base, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
		var previous StorageProfile
		e := a.DB.Where("auth_mode=? AND purpose=? AND bucket=? AND region=? AND endpoint=? AND role_name=? AND public_base_url=?", p.AuthMode, p.Purpose, p.Bucket, p.Region, p.Endpoint, p.RoleName, p.PublicBaseURL).First(&previous).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			if e = a.DB.Create(&p).Error; e != nil {
				return e
			}
		} else if e != nil {
			return e
		}
	}
	// One-time policy update preserves all explicit merchant quota overrides.
	return a.DB.Transaction(func(tx *gorm.DB) error {
		var marker SystemSetting
		if e := tx.Where("setting_key=?", "oss_quota_300mib_v1").First(&marker).Error; e == nil {
			return nil
		} else if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		cfg, e := systemConfigDB(tx)
		if e != nil {
			return e
		}
		cfg.Uploads.DefaultQuotaBytes = enterpriseOSSQuota
		cfg.Version++
		cfg.ProbeToken = ""
		b, _ := json.Marshal(cfg)
		if e = tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&SystemSetting{SettingKey: "system_config_v3", Value: string(b), UpdatedAt: time.Now().UTC()}).Error; e != nil {
			return e
		}
		if e = publishCloudStorageNotice(tx); e != nil {
			return e
		}
		return tx.Create(&SystemSetting{SettingKey: "oss_quota_300mib_v1", Value: "314572800", UpdatedAt: time.Now().UTC()}).Error
	})
}
func (a *App) cloudProfile(kind string) (StorageProfile, error) {
	cfg := cloudConfig()
	bucket, base := cfg.PrivateBucket, cfg.UploadBase
	if kind == "public" {
		bucket, base = cfg.PublicBucket, cfg.PublicBase
	}
	var p StorageProfile
	e := a.DB.Where("auth_mode='ecs_role' AND purpose=? AND bucket=? AND region=? AND endpoint=? AND role_name=? AND public_base_url=?", "enterprise_"+kind, bucket, cfg.Region, cfg.Endpoint, cfg.Role, base).Order("id DESC").First(&p).Error
	return p, e
}
func (a *App) cloudClient(p StorageProfile, external bool) (*oss.Client, error) {
	if p.Bucket == operatorBackupBucket {
		return nil, errors.New("oss_operator_backup_required")
	}
	if p.Kind != "oss" || p.AuthMode != "ecs_role" || p.RoleName == "" {
		return nil, errors.New("invalid_storage_profile")
	}
	endpoint := p.Endpoint
	if external {
		endpoint = p.PublicBaseURL
	}
	u, e := url.Parse(endpoint)
	if e != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return nil, errors.New("invalid_oss_endpoint")
	}
	if a.Config.Env == "production" && u.Scheme != "https" {
		return nil, errors.New("invalid_oss_endpoint")
	}
	r := a.cloudFiles()
	r.mu.Lock()
	defer r.mu.Unlock()
	key := fmt.Sprintf("%d|%s|%s|%s", p.ID, endpoint, p.Bucket, p.RoleName)
	if c := r.clients[key]; c != nil {
		return c, nil
	}
	if r.provider == nil {
		r.provider = credentials.NewEcsRoleCredentialsProvider(credentials.EcsRamRole(p.RoleName))
	}
	cfg := oss.LoadDefaultConfig().WithRegion(p.Region).WithEndpoint(endpoint).WithSignatureVersion(oss.SignatureVersionV4).WithCredentialsProvider(r.provider).WithRetryMaxAttempts(3)
	if external {
		cfg = cfg.WithUseCName(true).WithAdditionalHeaders([]string{"content-length"})
	}
	if a.Config.Env != "production" && u.Scheme == "http" {
		cfg = cfg.WithDisableSSL(true)
		if !external {
			cfg = cfg.WithUsePathStyle(true)
		}
	}
	c := oss.NewClient(cfg)
	r.clients[key] = c
	return c, nil
}
func cloudError(e error) error {
	if e == nil {
		return nil
	}
	var s *oss.ServiceError
	if errors.As(e, &s) {
		switch s.StatusCode {
		case 404:
			return errors.New("oss_object_missing")
		case 401, 403:
			return errors.New("oss_permission_denied")
		case 409:
			return errors.New("oss_object_conflict")
		case 429:
			return errors.New("oss_rate_limited")
		}
	}
	return errors.New("oss_unavailable") // SDK errors may contain signed URLs: never propagate them.
}
func (a *App) enterpriseID(owner uint64) uint64 {
	var v Content
	a.DB.Select("id").Where("kind='vendor' AND owner_id=?", owner).First(&v)
	return v.ID
}
func (a *App) mediaObjectKey(owner uint64, purpose, id, extension string) string {
	if !cloudFilesEnabled() {
		return id + extension
	}
	root := fmt.Sprintf("companies/%d/drafts", a.enterpriseID(owner))
	if a.enterpriseID(owner) == 0 {
		root = fmt.Sprintf("upload-temp/accounts/%d", owner)
	}
	if purpose == "ai_original" || purpose == "ai_asset" {
		root = fmt.Sprintf("companies/%d/ai-import", a.enterpriseID(owner))
		if a.enterpriseID(owner) == 0 {
			root = fmt.Sprintf("upload-temp/accounts/%d/ai-import", owner)
		}
	}
	return root + "/" + id + extension
}
func safeUploadFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	r := []rune(name)
	if len(r) > 200 {
		r = r[:200]
	}
	if len(r) == 0 {
		return "file"
	}
	return string(r)
}
func (a *App) cloudReady(public bool) bool {
	return a.cloudReadyDB(a.DB, public)
}
func (a *App) cloudReadyDB(db *gorm.DB, public bool) bool {
	if !cloudFilesEnabled() {
		return false
	}
	if a.Config.Env != "production" && env("OSS_TEST_ASSUME_READY", "false") == "true" {
		return true
	}
	var row SystemSetting
	if db.Where("setting_key=?", "oss_readiness_v1").First(&row).Error != nil {
		return false
	}
	var v struct {
		ConfigHash string `json:"config_hash"`
		Upload     bool   `json:"upload"`
		Public     bool   `json:"public"`
	}
	if json.Unmarshal([]byte(row.Value), &v) != nil || v.ConfigHash != cloudConfig().digest() {
		return false
	}
	return v.Upload && (!public || v.Public)
}
func (a *App) cloudDirectEnabled() bool {
	return env("OSS_UPLOADS_ENABLED", "true") == "true" && env("OSS_DIRECT_UPLOAD_ENABLED", "true") == "true" && a.cloudReady(false)
}
func (a *App) objectPinned(tx *gorm.DB, key string) bool {
	if !tx.Migrator().HasTable(&FileBackupPin{}) {
		return false
	}
	var n int64
	e := tx.Model(&FileBackupPin{}).Where("object_key=? AND expires_at>?", key, time.Now().UTC()).Count(&n).Error
	return e != nil || n > 0
}
func (a *App) registerEnterpriseFile(tx *gorm.DB, m Media, kind, name, mimeType string, business uint64) error {
	if actual := mime.TypeByExtension(filepath.Ext(m.Path)); actual != "" {
		mimeType = actual
	}
	var vendor Content
	tx.Select("id").Where("owner_id=? AND kind='vendor'", m.OwnerID).First(&vendor)
	filename := ""
	if name != "" {
		filename = safeUploadFilename(name)
	}
	f := EnterpriseFile{MediaID: m.ID, OwnerID: m.OwnerID, EnterpriseID: vendor.ID, BusinessType: kind, BusinessID: business, OriginalFilename: filename, MimeType: mimeType, Visibility: "private", FileStatus: "ready", Version: 1, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&f).Error; e != nil {
		return e
	}
	for index, key := range []string{m.Path, m.ThumbPath} {
		if key == "" {
			continue
		}
		var o StoredObject
		if tx.First(&o, "object_key=?", key).Error != nil {
			continue
		}
		var p StorageProfile
		if o.StoreID > 0 {
			tx.First(&p, o.StoreID)
		}
		role := "display"
		if index == 1 {
			role = "thumbnail"
		}
		physical := o.RemoteKey
		if physical == "" {
			physical = key
		}
		row := EnterpriseFileObject{ObjectKey: key, PhysicalKey: physical, MediaID: m.ID, OwnerID: m.OwnerID, EnterpriseID: f.EnterpriseID, StoreID: o.StoreID, Bucket: p.Bucket, ObjectRole: role, BusinessType: kind, FileSize: o.Size, SHA256: o.SHA256, ObjectState: "active", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
		if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; e != nil {
			return e
		}
	}
	return nil
}
func (a *App) objectStore(o StoredObject) (ObjectStore, error) {
	s, e := a.store(o.StoreID)
	if e != nil {
		return nil, e
	}
	if c, ok := s.(*ossObjectStore); ok && o.RemoteKey != "" {
		c.LogicalKey = o.Key
		c.PhysicalKey = o.RemoteKey
	}
	return s, nil
}
func (a *App) publicObjectURL(p StorageProfile, key string) (string, error) {
	if p.Purpose != "enterprise_public" || p.PublicBaseURL == "" || !safeObjectKey(key) {
		return "", errors.New("invalid_object_key")
	}
	parts := strings.Split(key, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.TrimRight(p.PublicBaseURL, "/") + "/" + strings.Join(parts, "/"), nil
}
func (a *App) signedObject(ctx context.Context, p StorageProfile, key string, method string, size int64, mimeType string) (*oss.PresignResult, error) {
	c, e := a.cloudClient(p, true)
	if e != nil {
		return nil, e
	}
	var request any = &oss.GetObjectRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(key)}
	if method == "PUT" {
		request = &oss.PutObjectRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(key), ContentLength: oss.Ptr(size), ContentType: oss.Ptr(mimeType), ForbidOverwrite: oss.Ptr("true")}
	}
	r, e := c.Presign(ctx, request, oss.PresignExpires(cloudAuthorizationLifetime()))
	return r, cloudError(e)
}

func cloudAuthorizationLifetime() time.Duration {
	seconds := settingInt("OSS_UPLOAD_TIMEOUT", 300)
	if seconds < 30 || seconds > 900 {
		seconds = 300
	}
	return time.Duration(seconds) * time.Second
}

func associateEnterpriseFiles(tx *gorm.DB, v Content) error {
	if v.Kind != "vendor" || v.ID == 0 || !tx.Migrator().HasTable(&EnterpriseFile{}) {
		return nil
	}
	for _, model := range []any{&EnterpriseFile{}, &EnterpriseFileObject{}, &FileUploadSession{}} {
		if e := tx.Model(model).Where("owner_id=? AND enterprise_id=0", v.OwnerID).Update("enterprise_id", v.ID).Error; e != nil {
			return e
		}
	}
	return nil
}
