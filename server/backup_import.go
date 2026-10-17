package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const backupUploadChunk = 8 << 20

type BackupUpload struct {
	ID        string    `json:"id"`
	Size      int64     `json:"size"`
	Offset    int64     `json:"offset"`
	ActorID   uint64    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
	Completed bool      `json:"completed"`
}

func (a *App) uploadMetadata(id string) (BackupUpload, error) {
	var upload BackupUpload
	if len(id) != 24 || strings.Trim(id, "0123456789abcdef") != "" {
		return upload, errors.New("invalid_upload")
	}
	data, err := os.ReadFile(filepath.Join(a.operations().dir, "imports", id+".json"))
	if err != nil {
		return upload, err
	}
	err = json.Unmarshal(data, &upload)
	return upload, err
}
func (a *App) beginBackupImport(c *gin.Context) {
	var req struct {
		Size int64 `json:"size"`
	}
	limit := int64(settingInt("MAX_BACKUP_GIB", 20)) << 30
	if c.ShouldBindJSON(&req) != nil || req.Size < 56 || req.Size > limit {
		fail(c, 400, "invalid_backup_size")
		return
	}
	dir := filepath.Join(a.operations().dir, "imports")
	if os.MkdirAll(dir, 0700) != nil {
		fail(c, 500, "internal_error")
		return
	}
	free, err := diskFreeBytes(dir)
	if err != nil || free < uint64(req.Size) {
		fail(c, 400, "disk_space_insufficient")
		return
	}
	upload := BackupUpload{ID: randomToken()[:24], Size: req.Size, ActorID: actor(c).ID, CreatedAt: time.Now().UTC()}
	file, err := os.OpenFile(filepath.Join(dir, upload.ID+".part"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		fail(c, 500, "internal_error")
		return
	}
	file.Close()
	if atomicJSON(filepath.Join(dir, upload.ID+".json"), upload) != nil {
		fail(c, 500, "internal_error")
		return
	}
	c.JSON(201, gin.H{"upload": upload, "chunk_size": backupUploadChunk, "max_bytes": limit})
}
func (a *App) uploadBackupChunk(c *gin.Context) {
	r := a.operations()
	r.mu.Lock()
	defer r.mu.Unlock()
	upload, err := a.uploadMetadata(c.Param("id"))
	if err != nil || upload.ActorID != actor(c).ID || upload.Completed || time.Since(upload.CreatedAt) > 24*time.Hour {
		fail(c, 404, "not_found")
		return
	}
	index, err := strconv.ParseInt(c.Param("index"), 10, 64)
	if err != nil || index*backupUploadChunk != upload.Offset {
		fail(c, 409, "invalid_upload_offset")
		return
	}
	want := min(int64(backupUploadChunk), upload.Size-upload.Offset)
	if want <= 0 {
		fail(c, 409, "invalid_upload_offset")
		return
	}
	data, err := io.ReadAll(io.LimitReader(c.Request.Body, want+1))
	if err != nil || int64(len(data)) != want {
		fail(c, 400, "invalid_upload_chunk")
		return
	}
	if index == 0 && (len(data) < 8 || string(data[:8]) != backupMagic) {
		fail(c, 400, "invalid_backup")
		return
	}
	path := filepath.Join(r.dir, "imports", upload.ID+".part")
	file, err := os.OpenFile(path, os.O_WRONLY, 0600)
	if err != nil {
		fail(c, 500, "internal_error")
		return
	}
	_, err = file.WriteAt(data, upload.Offset)
	if err == nil {
		err = file.Sync()
	}
	file.Close()
	if err != nil {
		fail(c, 500, "internal_error")
		return
	}
	upload.Offset += want
	if atomicJSON(filepath.Join(r.dir, "imports", upload.ID+".json"), upload) != nil {
		fail(c, 500, "internal_error")
		return
	}
	c.JSON(200, upload)
}
func (a *App) finishBackupImport(c *gin.Context) {
	r := a.operations()
	if !r.work.TryLock() {
		fail(c, 409, "job_busy")
		return
	}
	defer r.work.Unlock()
	upload, err := a.uploadMetadata(c.Param("id"))
	if err != nil || upload.ActorID != actor(c).ID || upload.Completed || upload.Offset != upload.Size {
		fail(c, 409, "invalid_upload_offset")
		return
	}
	source := filepath.Join(r.dir, "imports", upload.ID+".part")
	hash, size, err := fileSHA(source)
	if err != nil || size != upload.Size {
		fail(c, 400, "checksum_mismatch")
		return
	}
	objectKey := "archives/" + upload.ID + ".zqbak"
	store, _ := a.backupStore(0)
	body, err := os.Open(source)
	if err != nil {
		fail(c, 500, "internal_error")
		return
	}
	if _, e := store.Stat(c.Request.Context(), objectKey); e == nil {
		err = verifyObject(c.Request.Context(), store, objectKey, hash, size)
	} else {
		err = store.Put(context.Background(), objectKey, body, size)
	}
	body.Close()
	if err != nil {
		fail(c, 500, "storage_unavailable")
		return
	}
	entries, err := a.backupEntries()
	if err != nil {
		fail(c, 500, "internal_error")
		return
	}
	entry := BackupEntry{ID: upload.ID, Key: objectKey, Size: size, SHA256: hash, CreatedAt: time.Now().UTC()}
	exists := false
	for _, old := range entries {
		if old.ID == entry.ID {
			if old.SHA256 != entry.SHA256 {
				fail(c, 409, "checksum_mismatch")
				return
			}
			exists = true
		}
	}
	if !exists {
		entries = append(entries, entry)
	}
	if a.saveBackupEntries(entries) != nil {
		fail(c, 500, "internal_error")
		return
	}
	upload.Completed = true
	atomicJSON(filepath.Join(r.dir, "imports", upload.ID+".json"), upload)
	os.Remove(source)
	a.audit(actor(c).ID, "backup", 0, "backup_imported")
	c.JSON(201, entry)
}
