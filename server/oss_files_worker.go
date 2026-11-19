package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (a *App) startCloudFileWorker(ctx context.Context) {
	if !cloudFilesEnabled() {
		return
	}
	a.cloudFiles().workerOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if !a.operations().pauseWrites.Load() {
						a.operations().gate.RLock()
						a.operations().writes.RLock()
						a.processNextCloudUpload(ctx)
						a.reclaimCloudUpload(ctx)
						a.processNextFileJob(ctx)
						a.operations().writes.RUnlock()
						a.operations().gate.RUnlock()
					}
				}
			}
		}()
	})
}
func (a *App) processNextCloudUpload(parent context.Context) bool {
	return a.withCloudWorkerLock(parent, a.processNextCloudUploadLocked)
}
func (a *App) processNextCloudUploadLocked(parent context.Context) bool {
	r := a.cloudFiles()
	r.processMu.Lock()
	defer r.processMu.Unlock()
	var s FileUploadSession
	now := time.Now().UTC()
	token := randomToken()
	until := now.Add(2 * time.Minute)
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("upload_state='validating' AND (lease_until IS NULL OR lease_until<?)", now).Order("created_at").First(&s).Error; e != nil {
			return e
		}
		return tx.Model(&s).Updates(map[string]any{"lease_token": token, "lease_until": until, "updated_at": now}).Error
	})
	if e != nil {
		return false
	}
	s.LeaseToken = token
	a.setCloudUploadProgress(s.ID, "checking_file", nil)
	defer a.clearCloudUploadProgress(s.ID)
	ctx, cancel := context.WithTimeout(parent, 10*time.Minute)
	defer cancel()
	ctx, stop := a.cloudLeaseContext(ctx, "upload", s.ID, token)
	defer stop()
	e = a.processCloudUpload(ctx, &s)
	if e != nil {
		a.DB.Model(&FileUploadSession{}).Where("id=? AND lease_token=? AND upload_state='validating'", s.ID, token).Updates(map[string]any{"upload_state": "failed", "error": cloudProcessingError(e), "lease_token": "", "lease_until": nil, "updated_at": time.Now().UTC()})
	}
	return true
}
func cloudProcessingError(e error) string {
	if e == nil {
		return ""
	}
	switch e.Error() {
	case "invalid_image", "invalid_pdf", "file_size_mismatch", "unsupported_file", "file_too_large", "storage_quota_exceeded", "disk_space_insufficient", "file_upload_conflict", "video_probe_failed", "unsupported_video_codec", "video_limit_exceeded", "video_processing_failed", "video_processing_timeout", "video_conversion_unavailable", "checksum_mismatch", "file_backup_pinned", "oss_permission_denied", "oss_unavailable", "not_found", "media_not_ready", "watermark_not_ready":
		return e.Error()
	}
	return "oss_unavailable"
}
func (a *App) downloadCloudUpload(ctx context.Context, s FileUploadSession) (string, string, error) {
	budget := int64(settingInt("OSS_TEMP_BUDGET_MIB", 512)) << 20
	if s.ExpectedBytes > budget/2 {
		return "", "", errors.New("file_too_large")
	}
	if e := a.checkUploadDisks(uint64(s.ExpectedBytes * 2)); e != nil {
		return "", "", e
	}
	store, e := a.store(s.StoreID)
	if e != nil {
		return "", "", e
	}
	body, e := store.Open(ctx, s.ObjectKey)
	if e != nil {
		return "", "", cloudError(e)
	}
	defer body.Close()
	p := a.workPath("oss-" + s.ID + filepath.Ext(s.OriginalFilename))
	f, e := os.OpenFile(p, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if e != nil {
		return "", "", e
	}
	h := sha256.New()
	n, e := io.Copy(io.MultiWriter(f, h), io.LimitReader(&contextReader{ctx, body}, s.ExpectedBytes+1))
	closeError := f.Close()
	if e == nil {
		e = closeError
	}
	if e == nil && n != s.ExpectedBytes {
		e = errors.New("file_size_mismatch")
	}
	if e != nil {
		os.Remove(p)
		return "", "", e
	}
	return p, hex.EncodeToString(h.Sum(nil)), nil
}
func (a *App) processCloudUpload(ctx context.Context, s *FileUploadSession) error {
	if s.MediaID > 0 {
		return a.finishBoundCloudUpload(*s)
	}
	p, sha, e := a.downloadCloudUpload(ctx, *s)
	if e != nil {
		return e
	}
	defer os.Remove(p)
	if s.BusinessType == "video" {
		return a.processCloudVideo(ctx, *s, p, sha)
	}
	if s.BusinessType == "image" {
		f, e := os.Open(p)
		if e != nil {
			return e
		}
		cfg, format, e := image.DecodeConfig(f)
		f.Close()
		if e != nil || cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > 36000000 || (s.MimeType == "image/jpeg" && format != "jpeg") || (s.MimeType == "image/png" && format != "png") || (s.MimeType == "image/webp" && format != "webp") {
			return errors.New("invalid_image")
		}
		// The mature image/watermark pipeline is shared, including its transactional audit.
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, e := writer.CreateFormFile("file", s.OriginalFilename)
		if e != nil {
			return e
		}
		f, e = os.Open(p)
		if e != nil {
			return e
		}
		_, e = io.Copy(part, f)
		f.Close()
		if e != nil {
			return e
		}
		writer.WriteField("purpose", s.Purpose)
		writer.Close()
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest("POST", "/internal/oss-image", &body).WithContext(ctx)
		c.Request.Header.Set("Content-Type", writer.FormDataContentType())
		var who Account
		if e = a.DB.First(&who, s.ActorID).Error; e != nil {
			return e
		}
		c.Set("account", who)
		c.Set("upload_owner_id", s.OwnerID)
		c.Set("oss_session_id", s.ID)
		c.Set("oss_lease_token", s.LeaseToken)
		c.Set("oss_internal_processing", true)
		if s.Purpose == "home_display" {
			c.Set("upload_home_display", true)
		}
		if s.Purpose == "category_image" {
			c.Set("upload_category_media", true)
		}
		if s.BusinessScope == "content" && s.BusinessID > 0 {
			c.Set("upload_content_id", s.BusinessID)
		}
		if s.BusinessScope == "resource" && s.BusinessID > 0 {
			c.Set("upload_resource_id", s.BusinessID)
		}
		a.upload(c)
		if rec.Code != 201 {
			var failure struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			json.Unmarshal(rec.Body.Bytes(), &failure)
			if failure.Error.Code != "" {
				return errors.New(failure.Error.Code)
			}
			return errors.New("invalid_image")
		}
		if e = a.DB.First(s, "id=?", s.ID).Error; e != nil {
			return e
		}
		return a.finishBoundCloudUpload(*s)
	}
	pages := 0
	if s.MimeType == "application/pdf" {
		pages, e = pdfPages(p)
		if e != nil {
			return errors.New("invalid_pdf")
		}
		if s.BusinessType == "ai_original" && pages > settingInt("AI_IMPORT_MAX_PAGES", 100) {
			return errors.New("invalid_pdf")
		}
	}
	if strings.HasPrefix(s.MimeType, "image/") {
		f, e := os.Open(p)
		if e != nil {
			return e
		}
		cfg, format, e := image.DecodeConfig(f)
		f.Close()
		if e != nil || cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > 36000000 {
			return errors.New("invalid_image")
		}
		if (s.MimeType == "image/png" && format != "png") || (s.MimeType == "image/jpeg" && format != "jpeg") || (s.MimeType == "image/webp" && format != "webp") {
			return errors.New("invalid_image")
		}
		f, e = os.Open(p)
		if e != nil {
			return e
		}
		_, _, e = image.Decode(f)
		f.Close()
		if e != nil {
			return errors.New("invalid_image")
		}
	}
	return a.commitOriginalCloudUpload(*s, sha, pages)
}
func (a *App) commitOriginalCloudUpload(s FileUploadSession, sha string, pages int) error {
	return a.DB.Transaction(func(tx *gorm.DB) error {
		var acc Account
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&acc, s.OwnerID).Error; e != nil {
			return e
		}
		var current FileUploadSession
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, "id=?", s.ID).Error; e != nil {
			return e
		}
		if current.MediaID > 0 {
			return nil
		}
		if current.UploadState != "validating" || current.LeaseToken != s.LeaseToken {
			return errors.New("file_upload_conflict")
		}
		if e := tx.Where("id=?", s.ID).Delete(&MediaUploadReservation{}).Error; e != nil {
			return e
		}
		used, reserved, limit, e := a.ownerUsage(tx, s.OwnerID)
		if e != nil {
			return e
		}
		if phoneAccount(acc) && used+reserved+s.ExpectedBytes > limit {
			return errors.New("storage_quota_exceeded")
		}
		o := StoredObject{Key: s.ObjectKey, StoreID: s.StoreID, Size: s.ExpectedBytes, SHA256: sha, UpdatedAt: time.Now().UTC()}
		if e = tx.Create(&o).Error; e != nil {
			return e
		}
		if e = tx.Create(&MediaObjectOwner{ObjectKey: s.ObjectKey, OwnerID: s.OwnerID, Size: s.ExpectedBytes}).Error; e != nil {
			return e
		}
		m := Media{OwnerID: s.OwnerID, Path: s.ObjectKey}
		if e = tx.Create(&m).Error; e != nil {
			return e
		}
		kind := s.BusinessType
		if kind == "ai_original" {
			if s.MimeType == "application/pdf" {
				kind = "pdf"
			} else {
				kind = "image"
			}
		}
		if kind == "technical" {
			kind = "image"
		}
		meta := MediaMetadata{MediaID: m.ID, Kind: kind, State: "ready", Size: s.ExpectedBytes, PageCount: pages}
		if kind == "video" {
			meta.State = "queued"
			meta.SourcePath = s.ObjectKey
		}
		if e = tx.Create(&meta).Error; e != nil {
			return e
		}
		if e = a.registerEnterpriseFile(tx, m, s.BusinessType, s.OriginalFilename, s.MimeType, s.BusinessID); e != nil {
			return e
		}
		if e = tx.Model(&FileUploadSession{}).Where("id=?", s.ID).Updates(map[string]any{"media_id": m.ID, "upload_state": "completed", "error": "", "lease_until": nil, "lease_token": "", "updated_at": time.Now().UTC()}).Error; e != nil {
			return e
		}
		return recordOperation(tx, s.ActorID, "media", m.ID, "oss_upload_completed", s.BusinessType)
	})
}
func (a *App) finishBoundCloudUpload(s FileUploadSession) error {
	if s.MediaID == 0 {
		return errors.New("file_upload_conflict")
	}
	return a.DB.Transaction(func(tx *gorm.DB) error {
		var m Media
		if e := tx.First(&m, s.MediaID).Error; e != nil {
			return e
		}
		if m.OwnerID != s.OwnerID {
			return errors.New("file_upload_conflict")
		}
		if e := a.registerEnterpriseFile(tx, m, s.BusinessType, s.OriginalFilename, s.MimeType, s.BusinessID); e != nil {
			return e
		}
		return tx.Model(&FileUploadSession{}).Where("id=? AND lease_token=?", s.ID, s.LeaseToken).Updates(map[string]any{"upload_state": "completed", "error": "", "lease_until": nil, "lease_token": "", "updated_at": time.Now().UTC()}).Error
	})
}
func (a *App) reclaimCloudUpload(ctx context.Context) bool {
	var s FileUploadSession
	q := a.DB.Where("signature_expires_at<? AND (lease_until IS NULL OR lease_until<?) AND (upload_state='cancelled' OR (upload_state IN ('failed','uploading') AND expires_at<?) OR (upload_state='completed' AND id IN (SELECT id FROM media_upload_reservations WHERE hold_until_reclaimed=1)))", time.Now().UTC().Add(-30*time.Second), time.Now().UTC(), time.Now().UTC()).Order("updated_at")
	if q.First(&s).Error != nil {
		return false
	}
	if s.UploadState == "failed" && s.ExpiresAt.After(time.Now()) {
		return false
	}
	if s.UploadState == "failed" && s.MediaID > 0 {
		if e := a.finishBoundCloudUpload(s); e != nil {
			return false
		}
		s.UploadState = "completed"
	}
	var uses int64
	a.DB.Model(&Media{}).Where("path=? OR thumb_path=?", s.ObjectKey, s.ObjectKey).Count(&uses)
	var more int64
	a.DB.Model(&MediaMetadata{}).Where("source_path=?", s.ObjectKey).Count(&more)
	if uses+more > 0 {
		return false
	}
	if a.objectPinned(a.DB, s.ObjectKey) {
		return false
	}
	p, e := a.storageProfile(s.StoreID)
	if e != nil {
		return false
	}
	client, e := a.cloudClient(p, false)
	if e != nil {
		return false
	}
	if s.UploadMode == "multipart" {
		list, e := client.ListMultipartUploads(ctx, &oss.ListMultipartUploadsRequest{Bucket: oss.Ptr(p.Bucket), Prefix: oss.Ptr(s.ObjectKey)})
		if e != nil {
			return false
		}
		for _, v := range list.Uploads {
			if oss.ToString(v.Key) == s.ObjectKey {
				if _, e = client.AbortMultipartUpload(ctx, &oss.AbortMultipartUploadRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(s.ObjectKey), UploadId: v.UploadId}); e != nil {
					return false
				}
			}
		}
	}
	if _, e = client.DeleteObject(ctx, &oss.DeleteObjectRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(s.ObjectKey)}); e != nil {
		return false
	}
	if _, e = client.HeadObject(ctx, &oss.HeadObjectRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(s.ObjectKey)}); e == nil {
		return false
	} else {
		var se *oss.ServiceError
		if !errors.As(e, &se) || se.StatusCode != 404 {
			return false
		}
	}
	return a.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Where("id=? AND hold_until_reclaimed=1", s.ID).Delete(&MediaUploadReservation{}).Error; e != nil {
			return e
		}
		state := "reclaimed"
		if s.UploadState == "completed" {
			state = "completed"
		}
		return tx.Model(&FileUploadSession{}).Where("id=?", s.ID).Updates(map[string]any{"upload_state": state, "updated_at": time.Now().UTC()}).Error
	}) == nil
}
func (a *App) checkCloudFiles(c *gin.Context) {
	if !cloudFilesEnabled() {
		cloudFail(c, errors.New("oss_disabled"))
		return
	}
	var req struct {
		Confirm bool `json:"confirm"`
		Public  bool `json:"confirm_public_probe"`
	}
	if c.ShouldBindJSON(&req) != nil || !req.Confirm {
		fail(c, 400, "confirmation_required")
		return
	}
	state := map[string]any{"config_hash": cloudConfig().digest(), "upload": false, "public": false, "checked_at": time.Now().UTC()}
	save := func() {
		b, _ := json.Marshal(state)
		a.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&SystemSetting{SettingKey: "oss_readiness_v1", Value: string(b), UpdatedAt: time.Now().UTC()})
	}
	save()
	p, e := a.cloudProfile("private")
	if e != nil {
		cloudFail(c, errors.New("oss_configuration_required"))
		return
	}
	client, e := a.cloudClient(p, false)
	if e != nil {
		cloudFail(c, e)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()
	key := "upload-temp/system-preflight/" + randomToken() + ".bin"
	data := []byte("ZQZHG synthetic storage connectivity probe " + randomToken())
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		client.DeleteObject(cleanup, &oss.DeleteObjectRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(key)})
	}()
	auth, e := a.signedObject(ctx, p, key, "PUT", int64(len(data)), "application/octet-stream")
	if e != nil {
		cloudFail(c, e)
		return
	}
	transport := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	origin := a.effectiveConfig(c).Site.WebURL
	if origin != "https://zqzhg.com" {
		cloudFail(c, errors.New("oss_cors_origin_required"))
		return
	}
	preflight, _ := http.NewRequestWithContext(ctx, "OPTIONS", auth.URL, nil)
	preflight.Header.Set("Origin", origin)
	preflight.Header.Set("Access-Control-Request-Method", "PUT")
	preflight.Header.Set("Access-Control-Request-Headers", "content-type,x-oss-forbid-overwrite")
	response, e := transport.Do(preflight)
	if e != nil {
		cloudFail(c, errors.New("oss_unavailable"))
		return
	}
	io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	response.Body.Close()
	if response.StatusCode >= 400 || response.Header.Get("Access-Control-Allow-Origin") != origin {
		cloudFail(c, errors.New("oss_cors_required"))
		return
	}
	put, _ := http.NewRequestWithContext(ctx, "PUT", auth.URL, bytes.NewReader(data))
	for k, v := range auth.SignedHeaders {
		put.Header.Set(k, v)
	}
	put.ContentLength = int64(len(data))
	response, e = transport.Do(put)
	if e != nil {
		cloudFail(c, errors.New("oss_unavailable"))
		return
	}
	io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	response.Body.Close()
	if response.StatusCode >= 300 {
		cloudFail(c, errors.New("oss_permission_denied"))
		return
	}
	sum := sha256.Sum256(data)
	store := &ossObjectStore{Client: client, Bucket: p.Bucket, InheritACL: true}
	if e = verifyObject(ctx, store, key, hex.EncodeToString(sum[:]), int64(len(data))); e != nil {
		cloudFail(c, errors.New("oss_unavailable"))
		return
	}
	read, e := a.signedObject(ctx, p, key, "GET", 0, "")
	if e != nil {
		cloudFail(c, e)
		return
	}
	getRange, _ := http.NewRequestWithContext(ctx, "GET", read.URL, nil)
	getRange.Header.Set("Origin", origin)
	getRange.Header.Set("Range", "bytes=0-7")
	rangeResponse, e := transport.Do(getRange)
	if e != nil {
		cloudFail(c, errors.New("oss_unavailable"))
		return
	}
	rangeBody, _ := io.ReadAll(io.LimitReader(rangeResponse.Body, 9))
	rangeResponse.Body.Close()
	if rangeResponse.StatusCode != 206 || !bytes.Equal(rangeBody, data[:8]) || rangeResponse.Header.Get("Access-Control-Allow-Origin") != origin {
		cloudFail(c, errors.New("oss_cors_required"))
		return
	}
	if !cloudCORSExposes(rangeResponse.Header, "ETag", "Content-Length", "Content-Range", "Accept-Ranges") {
		cloudFail(c, errors.New("oss_cors_expose_headers_required"))
		return
	}
	anonymous, _ := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(p.PublicBaseURL, "/")+"/"+key, nil)
	anonymousResponse, e := transport.Do(anonymous)
	if e != nil {
		cloudFail(c, errors.New("oss_unavailable"))
		return
	}
	io.Copy(io.Discard, io.LimitReader(anonymousResponse.Body, 4096))
	anonymousResponse.Body.Close()
	if anonymousResponse.StatusCode != 403 && anonymousResponse.StatusCode != 404 {
		cloudFail(c, errors.New("oss_private_bucket_required"))
		return
	}
	partKey := strings.TrimSuffix(key, ".bin") + "-parts.bin"
	mp, e := client.InitiateMultipartUpload(ctx, &oss.InitiateMultipartUploadRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(partKey)})
	if e != nil {
		cloudFail(c, cloudError(e))
		return
	}
	defer client.AbortMultipartUpload(context.Background(), &oss.AbortMultipartUploadRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(partKey), UploadId: mp.UploadId})
	if _, e = client.UploadPart(ctx, &oss.UploadPartRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(partKey), UploadId: mp.UploadId, PartNumber: 1, Body: bytes.NewReader(data), ContentLength: oss.Ptr(int64(len(data)))}); e != nil {
		cloudFail(c, cloudError(e))
		return
	}
	if _, e = client.ListParts(ctx, &oss.ListPartsRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(partKey), UploadId: mp.UploadId}); e != nil {
		cloudFail(c, cloudError(e))
		return
	}
	if _, e = client.ListMultipartUploads(ctx, &oss.ListMultipartUploadsRequest{Bucket: oss.Ptr(p.Bucket), Prefix: oss.Ptr("upload-temp/system-preflight/")}); e != nil {
		cloudFail(c, cloudError(e))
		return
	}
	if _, e = client.AbortMultipartUpload(ctx, &oss.AbortMultipartUploadRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(partKey), UploadId: mp.UploadId}); e != nil {
		cloudFail(c, cloudError(e))
		return
	}
	if _, e = client.DeleteObject(ctx, &oss.DeleteObjectRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(key)}); e != nil {
		cloudFail(c, cloudError(e))
		return
	}
	state["upload"] = true
	if req.Public {
		pub, e := a.cloudProfile("public")
		if e != nil {
			cloudFail(c, e)
			return
		}
		if e = a.verifyPublicFileScopes(ctx, pub, origin, data, transport); e != nil {
			save()
			cloudFail(c, e)
			return
		}
		state["public"] = true
	}
	save()
	c.JSON(200, state)
}

// A successful CORS status alone does not let browser PDF/video clients inspect
// range support. Require explicit exposed headers; do not accept a wildcard.
func cloudCORSExposes(headers http.Header, required ...string) bool {
	exposed := map[string]bool{}
	for _, line := range headers.Values("Access-Control-Expose-Headers") {
		for _, name := range strings.Split(line, ",") {
			exposed[strings.ToLower(strings.TrimSpace(name))] = true
		}
	}
	for _, name := range required {
		if !exposed[strings.ToLower(name)] {
			return false
		}
	}
	return true
}
func (a *App) uploadLegacyCloudFile(c *gin.Context, kind string) {
	if !a.cloudReady(false) {
		cloudFail(c, errors.New("oss_unverified"))
		return
	}
	max := int64(a.uploadConfig().ImageMiB) << 20
	if kind == "pdf" {
		max = int64(a.uploadConfig().PDFMiB) << 20
	}
	if kind == "video" {
		max = int64(a.uploadConfig().VideoMiB) << 20
	}
	if max > int64(settingInt("OSS_TEMP_BUDGET_MIB", 512))<<19 {
		cloudFail(c, errors.New("file_too_large"))
		return
	}
	c.Request.Body = httpMaxBody(c, max+(2<<20))
	reader, e := c.Request.MultipartReader()
	if e != nil {
		cloudFail(c, errors.New("invalid_request"))
		return
	}
	var filename, path string
	fields := url.Values{}
	defer func() {
		if path != "" {
			os.Remove(path)
		}
	}()
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			cloudFail(c, errors.New("invalid_request"))
			return
		}
		if part.FileName() == "" {
			b, _ := io.ReadAll(io.LimitReader(part, 4096))
			fields.Set(part.FormName(), string(b))
			continue
		}
		if filename != "" || part.FormName() != "file" {
			cloudFail(c, errors.New("invalid_request"))
			return
		}
		filename = safeUploadFilename(part.FileName())
		path = a.workPath("legacy-" + randomToken() + filepath.Ext(filename))
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			cloudFail(c, errors.New("disk_space_insufficient"))
			return
		}
		n, err := io.Copy(f, io.LimitReader(part, max+1))
		ce := f.Close()
		if err != nil || ce != nil || n < 1 || n > max {
			cloudFail(c, errors.New("file_too_large"))
			return
		}
	}
	if path == "" {
		cloudFail(c, errors.New("invalid_request"))
		return
	}
	info, _ := os.Stat(path)
	c.Request.PostForm = fields
	c.Request.Form = fields
	purpose := "resource_file"
	if kind == "image" {
		purpose, e = uploadImagePurpose(c)
		if e != nil {
			cloudFail(c, e)
			return
		}
	}
	req := cloudUploadRequest{IdempotencyKey: randomToken(), Kind: kind, Purpose: purpose, Filename: filename, Size: info.Size()}
	if v, ok := c.Get("upload_content_id"); ok {
		req.BusinessKind = "content"
		req.BusinessID = v.(uint64)
	}
	if v, ok := c.Get("upload_resource_id"); ok {
		req.BusinessKind = "resource"
		req.BusinessID = v.(uint64)
	}
	if owner, ok := c.Get("upload_owner_id"); ok {
		req.EnterpriseID = a.enterpriseID(owner.(uint64))
	}
	s, e := a.createCloudUpload(c, req)
	if e != nil {
		cloudFail(c, e)
		return
	}
	store, e := a.store(s.StoreID)
	if e != nil {
		cloudFail(c, e)
		return
	}
	f, e := os.Open(path)
	if e != nil {
		cloudFail(c, e)
		return
	}
	e = store.Put(c.Request.Context(), s.ObjectKey, f, s.ExpectedBytes)
	f.Close()
	os.Remove(path)
	path = ""
	if e != nil {
		a.DB.Model(&s).Updates(map[string]any{"upload_state": "cancelled"})
		cloudFail(c, cloudError(e))
		return
	}
	token := randomToken()
	until := time.Now().Add(15 * time.Minute)
	a.DB.Model(&s).Updates(map[string]any{"upload_state": "validating", "lease_token": token, "lease_until": until})
	s.UploadState = "validating"
	s.LeaseToken = token
	a.cloudFiles().processMu.Lock()
	e = a.processCloudUpload(c.Request.Context(), &s)
	a.cloudFiles().processMu.Unlock()
	if e != nil {
		a.DB.Model(&s).Updates(map[string]any{"upload_state": "failed", "error": cloudProcessingError(e), "lease_token": "", "lease_until": nil})
		cloudFail(c, errors.New(cloudProcessingError(e)))
		return
	}
	a.DB.First(&s, "id=?", s.ID)
	if kind == "image" {
		c.JSON(201, gin.H{"id": s.MediaID, "url": a.mediaAddress(c, s.MediaID, ""), "thumb_url": a.mediaAddress(c, s.MediaID, "?thumb=1")})
		return
	}
	var meta MediaMetadata
	a.DB.First(&meta, "media_id=?", s.MediaID)
	code := 201
	if meta.State != "ready" {
		code = 202
	}
	c.JSON(code, meta)
}

// A common worker entry point is also used by CLI retries and isolated tests.
func (a *App) processNextFileJob(ctx context.Context) bool { return a.runNextStorageFileJob(ctx) }

var _ = fmt.Sprintf
