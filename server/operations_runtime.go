package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SystemSetting struct {
	SettingKey string `gorm:"primaryKey"`
	Value      string
	UpdatedAt  time.Time
}
type OperationsRuntime struct {
	gate        sync.RWMutex
	writes      sync.RWMutex
	pauseWrites atomic.Bool
	work        sync.Mutex
	keyMu       sync.Mutex
	mu          sync.Mutex
	maintenance atomic.Bool
	dir         string
	jobs        map[string]*OperationJob
	initErr     error
	started     bool
}
type OperationJob struct {
	ID            string          `json:"id"`
	Kind          string          `json:"kind"`
	State         string          `json:"state"`
	Phase         string          `json:"phase"`
	Progress      int64           `json:"progress"`
	Total         int64           `json:"total"`
	Error         string          `json:"error,omitempty"`
	Params        json.RawMessage `json:"-"`
	Result        json.RawMessage `json:"result,omitempty"`
	ActorID       uint64          `json:"actor_id"`
	ActorName     string          `json:"actor_name"`
	ActorDatabase string          `json:"actor_database"`
	Cancelled     bool            `json:"cancelled"`
	TicketHash    string          `json:"-"`
	TicketExpires time.Time       `json:"-"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}
type jobDisk struct {
	Job           *OperationJob   `json:"job"`
	Params        json.RawMessage `json:"params"`
	TicketHash    string          `json:"ticket_hash"`
	TicketExpires time.Time       `json:"ticket_expires"`
}
type ActiveTarget struct {
	Database string `json:"database"`
	MediaDir string `json:"media_dir"`
}
type HostTarget struct {
	Current  ActiveTarget  `json:"current"`
	Previous *ActiveTarget `json:"previous,omitempty"`
	Pending  bool          `json:"pending"`
	Outcome  string        `json:"outcome,omitempty"`
	JobID    string        `json:"job_id"`
}

func controlDirectory(mediaDir string) string {
	if value := os.Getenv("OPERATIONS_DIR"); value != "" {
		return value
	}
	path, _ := filepath.Abs(mediaDir)
	return filepath.Join(filepath.Dir(path), ".operations-"+filepath.Base(path))
}
func atomicJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	temp := path + "." + randomToken()[:12] + ".tmp"
	file, err := os.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	defer os.Remove(temp)
	// Rename replaces atomically on supported platforms. The old complete file is
	// retained when replacement is denied; no delete-before-rename gap is allowed.
	return os.Rename(temp, path)
}
func readHostTarget(mediaDir string) (HostTarget, error) {
	var target HostTarget
	data, err := os.ReadFile(filepath.Join(controlDirectory(mediaDir), "active.json"))
	if os.IsNotExist(err) {
		return target, nil
	}
	if err != nil {
		return target, err
	}
	err = json.Unmarshal(data, &target)
	return target, err
}
func (a *App) operations() *OperationsRuntime {
	a.operationsOnce.Do(func() {
		r := &OperationsRuntime{dir: controlDirectory(a.Config.MediaDir), jobs: map[string]*OperationJob{}}
		a.operationsRuntime = r
		r.initErr = os.MkdirAll(r.dir, 0700)
		if r.initErr != nil {
			return
		}
		os.MkdirAll(filepath.Join(r.dir, "work"), 0700)
		os.MkdirAll(filepath.Join(r.dir, "jobs"), 0700)
		os.MkdirAll(filepath.Join(r.dir, "backups"), 0700)
		entries, _ := os.ReadDir(filepath.Join(r.dir, "jobs"))
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			var disk jobDisk
			data, err := os.ReadFile(filepath.Join(r.dir, "jobs", entry.Name()))
			if err != nil || json.Unmarshal(data, &disk) != nil || disk.Job == nil {
				continue
			}
			disk.Job.Params = disk.Params
			disk.Job.TicketHash = disk.TicketHash
			disk.Job.TicketExpires = disk.TicketExpires
			if disk.Job.State == "running" || disk.Job.State == "queued" {
				disk.Job.State = "interrupted"
				disk.Job.Error = "service_restarted"
			}
			r.jobs[disk.Job.ID] = disk.Job
		}
		if host, err := readHostTargetFrom(r.dir); err == nil && host.JobID != "" {
			if job := r.jobs[host.JobID]; job != nil && job.State == "interrupted" && (job.Kind == "activate" || job.Kind == "rollback") {
				if host.Outcome == "committed" {
					job.State = "succeeded"
					job.Phase = "complete"
					job.Error = ""
					job.Result, _ = json.Marshal(map[string]any{"relogin_required": true, "recovered": true, "active_database": host.Current.Database})
				} else if host.Outcome == "rolled_back" {
					job.State = "failed"
					job.Phase = "rolled_back"
					job.Error = "restore_validation"
				}
				r.persist(job)
			}
		}
	})
	return a.operationsRuntime
}
func (a *App) setting(key, fallback string) string {
	var item SystemSetting
	if a.DB.First(&item, "setting_key=?", key).Error != nil {
		return fallback
	}
	return item.Value
}
func (a *App) setSetting(key, value string) error {
	return a.DB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "setting_key"}}, DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"})}).Create(&SystemSetting{SettingKey: key, Value: value, UpdatedAt: time.Now().UTC()}).Error
}
func (a *App) masterKey() ([]byte, error) {
	r := a.operations()
	r.keyMu.Lock()
	defer r.keyMu.Unlock()
	key := os.Getenv("CONFIG_ENCRYPTION_KEY")
	if key != "" {
		data, err := hex.DecodeString(key)
		if err == nil && len(data) == 32 {
			return data, nil
		}
		data, err = base64.StdEncoding.DecodeString(key)
		if err == nil && len(data) == 32 {
			return data, nil
		}
		return nil, errors.New("invalid_encryption_key")
	}
	if a.Config.Env == "production" {
		return nil, errors.New("encryption_key_required")
	}
	path := filepath.Join(a.operations().dir, "development.key")
	if data, err := os.ReadFile(path); err == nil {
		if len(data) != 32 {
			return nil, errors.New("invalid_encryption_key")
		}
		return data, nil
	}
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if os.IsExist(err) {
		return os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}
	_, err = file.Write(data)
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	return data, err
}
func encryptionGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func (a *App) sealSecret(data []byte) (string, error) {
	key, err := a.masterKey()
	if err != nil {
		return "", err
	}
	gcm, err := encryptionGCM(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, data, []byte("zqzt-config-v1"))), nil
}
func (a *App) openSecret(value string) ([]byte, error) {
	key, err := a.masterKey()
	if err != nil {
		return nil, err
	}
	gcm, err := encryptionGCM(key)
	if err != nil {
		return nil, err
	}
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(data) < gcm.NonceSize()+gcm.Overhead() {
		return nil, errors.New("invalid_encrypted_config")
	}
	return gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], []byte("zqzt-config-v1"))
}
func (a *App) analyticsKey() ([]byte, error) {
	r := a.operations()
	r.keyMu.Lock()
	defer r.keyMu.Unlock()
	// A separate persistent random key keeps anonymous analytics working even
	// before a production storage-encryption key is configured.
	path := filepath.Join(a.operations().dir, "analytics.key")
	if b, e := os.ReadFile(path); e == nil && len(b) == 32 {
		return b, nil
	}
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(e) {
		return os.ReadFile(path)
	}
	if e != nil {
		return nil, e
	}
	_, e = f.Write(b)
	f.Close()
	return b, e
}
func (r *OperationsRuntime) persist(job *OperationJob) error {
	return atomicJSON(filepath.Join(r.dir, "jobs", job.ID+".json"), jobDisk{job, job.Params, job.TicketHash, job.TicketExpires})
}
func (a *App) updateJob(id, phase string, progress, total int64) error {
	r := a.operations()
	r.mu.Lock()
	defer r.mu.Unlock()
	job := r.jobs[id]
	if job == nil {
		return errors.New("job_not_found")
	}
	if job.Cancelled {
		return errors.New("job_cancelled")
	}
	job.Phase = phase
	job.Progress = progress
	job.Total = total
	job.UpdatedAt = time.Now().UTC()
	return r.persist(job)
}
func (a *App) newJob(kind string, params any, actorID uint64) (*OperationJob, string, error) {
	r := a.operations()
	if r.initErr != nil {
		return nil, "", r.initErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, job := range r.jobs {
		if job.State == "queued" || job.State == "running" {
			return nil, "", errors.New("job_busy")
		}
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		return nil, "", err
	}
	name := "系统"
	if actorID > 0 {
		var account Account
		if a.DB.First(&account, actorID).Error == nil {
			name = account.Username
		} else {
			name = fmt.Sprintf("#%d", actorID)
		}
	}
	database := a.currentDatabase()
	ticket := randomToken()
	now := time.Now().UTC()
	job := &OperationJob{ID: randomToken()[:24], Kind: kind, State: "queued", Phase: "queued", Params: encoded, ActorID: actorID, ActorName: name, ActorDatabase: database, CreatedAt: now, UpdatedAt: now, TicketHash: digest(ticket), TicketExpires: now.Add(20 * time.Minute)}
	if err = r.persist(job); err != nil {
		return nil, "", err
	}
	r.jobs[job.ID] = job
	go a.executeJob(job.ID)
	copy := *job
	return &copy, ticket, nil
}
func jobFailure(err error) string {
	for _, code := range []string{"job_busy", "job_cancelled", "cancelled", "encryption_key", "checksum", "invalid_backup", "backup_version", "backup_key", "disk_space", "mysql_tools", "maintenance_credentials", "admin_required", "restore_validation", "storage_private_bucket_required", "storage_unavailable", "object_missing", "not_found", "invalid_media", "restore_not_ready"} {
		if strings.Contains(err.Error(), code) {
			return code
		}
	}
	return "job_failed"
}
func (a *App) executeJob(id string) {
	r := a.operations()
	r.work.Lock()
	defer r.work.Unlock()
	r.mu.Lock()
	job := r.jobs[id]
	if job == nil {
		r.mu.Unlock()
		return
	}
	job.State = "running"
	job.UpdatedAt = time.Now().UTC()
	r.persist(job)
	copy := *job
	r.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	defer cancel()
	var result any
	var err error
	switch copy.Kind {
	case "migration":
		result, err = a.runStorageMigration(ctx, &copy)
	case "backup":
		result, err = a.runBackup(ctx, &copy)
	case "restore":
		result, err = a.runRestore(ctx, &copy)
	case "verify":
		result, err = a.runVerify(ctx, &copy)
	case "activate":
		result, err = a.runActivation(ctx, &copy, false)
	case "rollback":
		result, err = a.runActivation(ctx, &copy, true)
	default:
		err = errors.New("invalid_job")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	job = r.jobs[id]
	if err != nil {
		job.State = "failed"
		job.Error = jobFailure(err)
		if job.Cancelled {
			job.State = "cancelled"
			job.Error = "job_cancelled"
		}
	} else {
		job.State = "succeeded"
		job.Phase = "complete"
		job.Result, _ = json.Marshal(result)
	}
	job.UpdatedAt = time.Now().UTC()
	r.persist(job)
	note, _ := json.Marshal(map[string]any{"job_id": job.ID, "actor_id": copy.ActorID, "actor_name": copy.ActorName, "actor_database": copy.ActorDatabase})
	a.DB.Create(&OperationLog{ActorID: copy.ActorID, TargetKind: "job", Action: copy.Kind + "_" + job.State, Note: string(note), CreatedAt: time.Now().UTC()})
}
func (a *App) jobRoutes(admin *gin.RouterGroup, api *gin.RouterGroup) {
	admin.GET("/jobs", func(c *gin.Context) {
		r := a.operations()
		r.mu.Lock()
		defer r.mu.Unlock()
		items := []OperationJob{}
		for _, job := range r.jobs {
			items = append(items, *job)
		}
		c.JSON(200, gin.H{"items": items})
	})
	admin.GET("/jobs/:id", func(c *gin.Context) {
		r := a.operations()
		r.mu.Lock()
		defer r.mu.Unlock()
		if job := r.jobs[c.Param("id")]; job != nil {
			c.JSON(200, job)
			return
		}
		fail(c, 404, "not_found")
	})
	admin.POST("/jobs/:id/cancel", func(c *gin.Context) {
		r := a.operations()
		r.mu.Lock()
		defer r.mu.Unlock()
		job := r.jobs[c.Param("id")]
		if job == nil {
			fail(c, 404, "not_found")
			return
		}
		if job.Kind == "activate" || job.Kind == "rollback" {
			fail(c, 409, "cannot_cancel_cutover")
			return
		}
		job.Cancelled = true
		r.persist(job)
		c.JSON(200, gin.H{"ok": true})
	})
	admin.POST("/jobs/:id/retry", func(c *gin.Context) {
		r := a.operations()
		r.mu.Lock()
		job := r.jobs[c.Param("id")]
		if job == nil || !(job.State == "failed" || job.State == "interrupted" || job.State == "cancelled") || job.Kind == "activate" || job.Kind == "rollback" {
			r.mu.Unlock()
			fail(c, 409, "invalid_job")
			return
		}
		copy := *job
		r.mu.Unlock()
		a.respondNewJob(c, copy.Kind, json.RawMessage(copy.Params))
	})
	api.GET("/maintenance/:id", func(c *gin.Context) {
		r := a.operations()
		r.mu.Lock()
		defer r.mu.Unlock()
		job := r.jobs[c.Param("id")]
		if job == nil || time.Now().After(job.TicketExpires) || digest(c.GetHeader("X-Maintenance-Token")) != job.TicketHash {
			fail(c, 401, "unauthorized")
			return
		}
		c.Header("Cache-Control", "no-store")
		result := gin.H{}
		if job.State == "succeeded" && (job.Kind == "activate" || job.Kind == "rollback") {
			result["relogin_required"] = true
		}
		c.JSON(200, gin.H{"id": job.ID, "kind": job.Kind, "state": job.State, "phase": job.Phase, "progress": job.Progress, "total": job.Total, "error": job.Error, "created_at": job.CreatedAt, "updated_at": job.UpdatedAt, "result": result})
	})
}
func (a *App) respondNewJob(c *gin.Context, kind string, params any) {
	job, ticket, err := a.newJob(kind, params, actor(c).ID)
	if err != nil {
		fail(c, 409, jobFailure(err))
		return
	}
	c.JSON(202, gin.H{"job": job, "maintenance_token": ticket})
}
func (a *App) operationsGate() gin.HandlerFunc {
	return func(c *gin.Context) {
		r := a.operations()
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/v1/maintenance/") {
			c.Next()
			return
		}
		if r.maintenance.Load() {
			fail(c, 503, "maintenance")
			return
		}
		r.gate.RLock()
		defer r.gate.RUnlock()
		if r.maintenance.Load() {
			fail(c, 503, "maintenance")
			return
		}
		if (c.Request.Method != "GET" && c.Request.Method != "HEAD" && c.Request.Method != "OPTIONS") || strings.HasSuffix(path, "/analytics/export") || (strings.Contains(path, "/admin/backups/") && strings.HasSuffix(path, "/download")) {
			if r.pauseWrites.Load() {
				fail(c, 503, "snapshot_in_progress")
				return
			}
			r.writes.RLock()
			defer r.writes.RUnlock()
		}
		c.Next()
	}
}
func (a *App) startOperations(ctx context.Context) {
	r := a.operations()
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return
	}
	r.started = true
	r.mu.Unlock()
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.operationsTick()
			}
		}
	}()
}
func (a *App) operationsTick() {
	r := a.operations()
	if r.maintenance.Load() {
		return
	}
	r.gate.RLock()
	defer r.gate.RUnlock()
	r.writes.RLock()
	defer r.writes.RUnlock()
	if r.pauseWrites.Load() {
		return
	}
	a.aggregateAnalytics()
	a.scheduledBackup()
}
func (a *App) workPath(key string) string { return filepath.Join(a.operations().dir, "work", key) }
func fileSHA(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), n, err
}
func (a *App) audit(actor uint64, kind string, id uint64, action string) error {
	return recordOperation(a.DB, actor, kind, id, action, "")
}
func objectError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) || os.IsNotExist(err) {
		return fmt.Errorf("object_missing")
	}
	return errors.New("storage_unavailable")
}
