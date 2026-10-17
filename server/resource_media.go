package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
	pdfmodel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"gorm.io/gorm"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func init() { pdfmodel.ConfigPath = "disable" }

func videoTool(name string) string {
	if value := os.Getenv(strings.ToUpper(name) + "_PATH"); value != "" {
		return value
	}
	if path, e := exec.LookPath(name); e == nil {
		return path
	}
	if runtime.GOOS == "windows" {
		return filepath.Join("..", ".tools", "ffmpeg", "bin", name+".exe")
	}
	return name
}
func pdfPages(path string) (int, error) {
	f, e := os.Open(path)
	if e != nil {
		return 0, e
	}
	defer f.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	doc, e := pdfapi.ReadContext(ctx, f, pdfmodel.NewDefaultConfiguration())
	if e != nil || doc.Encrypt != nil {
		return 0, fmt.Errorf("invalid_pdf")
	}
	if pdfapi.ValidateContext(ctx, doc) != nil {
		return 0, fmt.Errorf("invalid_pdf")
	}
	if doc.PageCount < 1 || doc.PageCount > settingInt("MAX_PDF_PAGES", 200) {
		return 0, fmt.Errorf("invalid_pdf")
	}
	return doc.PageCount, nil
}
func (a *App) uploadResourceMedia(c *gin.Context) {
	kind := c.Query("kind")
	maxBytes := int64(settingInt("MAX_VIDEO_MIB", 200)) << 20
	if kind == "pdf" {
		maxBytes = int64(settingInt("MAX_PDF_MIB", 50)) << 20
	} else if kind != "video" {
		fail(c, 400, "invalid_request")
		return
	}
	c.Request.Body = httpMaxBody(c, maxBytes+(1<<20))
	file, e := c.FormFile("file")
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	if e != nil || file.Size <= 0 || file.Size > maxBytes {
		fail(c, 400, "file_too_large")
		return
	}
	extension := strings.ToLower(filepath.Ext(file.Filename))
	if (kind == "video" && extension != ".mp4" && extension != ".mov") || (kind == "pdf" && extension != ".pdf") {
		fail(c, 400, "unsupported_file")
		return
	}
	src, e := file.Open()
	if e != nil {
		fail(c, 400, "invalid_request")
		return
	}
	defer src.Close()
	key := randomToken()
	path := key + extension
	full := a.workPath(path)
	f, e := os.OpenFile(full, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		fail(c, 500, "internal_error")
		return
	}
	n, e := io.Copy(f, io.LimitReader(src, maxBytes+1))
	closeError := f.Close()
	if e != nil || closeError != nil || n > maxBytes || c.Request.Context().Err() != nil {
		os.Remove(full)
		fail(c, 400, "invalid_request")
		return
	}
	media := Media{OwnerID: actor(c).ID, Path: path}
	if owner, ok := c.Get("upload_owner_id"); ok {
		media.OwnerID = owner.(uint64)
	}
	meta := MediaMetadata{Kind: kind, Size: n, State: "ready"}
	status := 201
	if kind == "pdf" {
		meta.PageCount, e = pdfPages(full)
		if e != nil {
			os.Remove(full)
			fail(c, 400, "invalid_pdf")
			return
		}
	} else {
		meta.State = "queued"
		meta.SourcePath = path
		media.Path = key + "-ready.mp4"
		media.ThumbPath = key + "-cover.jpg"
		status = 202
	}
	defer os.Remove(full)
	if e = a.storeWorkFile(c.Request.Context(), path, full); e != nil {
		fail(c, 503, "storage_unavailable")
		return
	}
	e = a.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Create(&media).Error; e != nil {
			return e
		}
		meta.MediaID = media.ID
		if e := tx.Create(&meta).Error; e != nil {
			return e
		}
		if target, ok := c.Get("upload_resource_id"); ok {
			return recordOperation(tx, actor(c).ID, "media", media.ID, "media_uploaded", fmt.Sprintf("resource=%d", target.(uint64)))
		}
		return recordOperation(tx, actor(c).ID, "media", media.ID, "media_uploaded", "")
	})
	if e != nil {
		os.Remove(full)
		fail(c, 500, "internal_error")
		return
	}
	c.JSON(status, meta)
}
func (a *App) resourceMediaStatus(c *gin.Context) {
	var m Media
	if a.DB.First(&m, id(c)).Error != nil {
		fail(c, 404, "not_found")
		return
	}
	if !a.canReadPrivateMedia(actor(c), m) {
		fail(c, 404, "not_found")
		return
	}
	var meta MediaMetadata
	if a.DB.First(&meta, "media_id=?", m.ID).Error != nil {
		meta = MediaMetadata{MediaID: m.ID, Kind: "image", State: "ready"}
	}
	c.JSON(200, meta)
}
func (a *App) retryResourceMedia(c *gin.Context) {
	var m Media
	if a.DB.First(&m, id(c)).Error != nil || !a.canReadPrivateMedia(actor(c), m) {
		fail(c, 404, "not_found")
		return
	}
	var meta MediaMetadata
	if a.DB.First(&meta, "media_id=?", m.ID).Error != nil || meta.Kind != "video" || meta.State != "failed" {
		fail(c, 409, "invalid_request")
		return
	}
	if _, e := a.objectRecord(meta.SourcePath); e != nil {
		fail(c, 400, "invalid_media")
		return
	}
	if a.DB.Model(&meta).Where("state='failed'").Updates(map[string]any{"state": "queued", "error": ""}).Error != nil {
		fail(c, 500, "internal_error")
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func (a *App) startMediaWorker(ctx context.Context) {
	a.DB.Model(&MediaMetadata{}).Where("state='processing'").Update("state", "queued")
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.processNextVideo(ctx)
			}
		}
	}()
}
func (a *App) processNextVideo(parent context.Context) bool {
	r := a.operations()
	if r.maintenance.Load() || r.pauseWrites.Load() {
		return false
	}
	r.gate.RLock()
	defer r.gate.RUnlock()
	r.writes.RLock()
	defer r.writes.RUnlock()
	var meta MediaMetadata
	job := a.DB.Where("kind='video' AND state='queued'").Order("media_id").Limit(1).Find(&meta)
	if job.Error != nil || job.RowsAffected == 0 {
		return false
	}
	claim := a.DB.Model(&meta).Where("state='queued'").Update("state", "processing")
	if claim.Error != nil || claim.RowsAffected != 1 {
		return false
	}
	var m Media
	if a.DB.First(&m, meta.MediaID).Error != nil {
		return false
	}
	workVideo, workCover := "", ""
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	failure := func(code string) {
		if workVideo != "" {
			os.Remove(workVideo)
		}
		if workCover != "" {
			os.Remove(workCover)
		}
		a.DB.Model(&meta).Updates(map[string]any{"state": "failed", "error": code})
		os.Remove(a.workPath(m.Path))
		os.Remove(a.workPath(m.ThumbPath))
	}
	input, inputErr := a.materializeObject(ctx, meta.SourcePath)
	if inputErr == nil {
		defer os.Remove(input)
	}
	if inputErr != nil {
		failure("invalid_media")
		return true
	}
	out, e := exec.CommandContext(ctx, videoTool("ffprobe"), "-f", "mov", "-protocol_whitelist", "file,pipe", "-v", "error", "-show_entries", "format=duration:stream=codec_type,width,height", "-of", "json", input).Output()
	if e != nil {
		failure("video_probe_failed")
		return true
	}
	var info struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
	}
	if json.Unmarshal(out, &info) != nil {
		failure("unsupported_file")
		return true
	}
	duration, _ := strconv.ParseFloat(info.Format.Duration, 64)
	w, h := 0, 0
	for _, s := range info.Streams {
		if s.CodecType == "video" {
			w = s.Width
			h = s.Height
			break
		}
	}
	if duration <= 0 || duration > float64(settingInt("MAX_VIDEO_SECONDS", 180)) || w < 1 || h < 1 || w > 4096 || h > 4096 {
		failure("video_limit_exceeded")
		return true
	}
	target := a.workPath(m.Path)
	workVideo = a.workPath(randomToken() + "-work.mp4")
	long := settingInt("VIDEO_MAX_EDGE", 1280)
	filter := fmt.Sprintf("scale=w='min(iw,%d)':h='min(ih,%d)':force_original_aspect_ratio=decrease:force_divisible_by=2,setsar=1", long, long)
	e = exec.CommandContext(ctx, videoTool("ffmpeg"), "-nostdin", "-y", "-v", "error", "-protocol_whitelist", "file,pipe", "-f", "mov", "-i", input, "-map", "0:v:0", "-map", "0:a:0?", "-vf", filter, "-c:v", "libx264", "-preset", "fast", "-crf", "23", "-pix_fmt", "yuv420p", "-c:a", "aac", "-b:a", "128k", "-movflags", "+faststart", "-map_metadata", "-1", workVideo).Run()
	if e != nil {
		failure("video_processing_failed")
		return true
	}
	cover := a.workPath(m.ThumbPath)
	workCover = a.workPath(randomToken() + "-work.jpg")
	e = exec.CommandContext(ctx, videoTool("ffmpeg"), "-nostdin", "-y", "-v", "error", "-i", workVideo, "-frames:v", "1", "-vf", "scale=480:480:force_original_aspect_ratio=decrease", workCover).Run()
	if e != nil {
		failure("video_processing_failed")
		return true
	}
	if e = os.Rename(workVideo, target); e != nil {
		failure("video_processing_failed")
		return true
	}
	if e = os.Rename(workCover, cover); e != nil {
		failure("video_processing_failed")
		return true
	}
	st, e := os.Stat(target)
	if e != nil {
		failure("video_processing_failed")
		return true
	}
	defer os.Remove(target)
	defer os.Remove(cover)
	if e = a.storeWorkFile(ctx, m.Path, target); e != nil {
		failure("storage_unavailable")
		return true
	}
	if e = a.storeWorkFile(ctx, m.ThumbPath, cover); e != nil {
		failure("storage_unavailable")
		return true
	}
	scale := min(1.0, float64(long)/float64(max(w, h)))
	e = a.DB.Model(&meta).Updates(map[string]any{"state": "ready", "error": "", "duration": duration, "width": int(float64(w) * scale), "height": int(float64(h) * scale), "size": st.Size(), "source_path": ""}).Error
	if e != nil {
		failure("video_processing_failed")
		return true
	}
	os.Remove(input)
	return true
}

func (a *App) resourceMediaIsPublic(mid uint64) (bool, error) {
	var n int64
	e := a.DB.Table("resource_media_references rm").Joins("JOIN resource_revisions pr ON pr.id=rm.revision_id").Joins("JOIN resources ON resources.published_revision_id=pr.id").Where("rm.media_id=?", mid).Where(resourcePublicSQL).Count(&n).Error
	return n > 0, e
}
