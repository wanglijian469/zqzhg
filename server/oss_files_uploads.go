package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const cloudPartBytes int64 = 8 << 20

type cloudUploadRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
	Kind           string `json:"kind"`
	Purpose        string `json:"purpose"`
	BusinessKind   string `json:"business_kind"`
	BusinessID     uint64 `json:"business_id"`
	EnterpriseID   uint64 `json:"enterprise_id"`
	Filename       string `json:"filename"`
	Size           int64  `json:"size"`
	MimeType       string `json:"mime_type"`
}

func cloudFail(c *gin.Context, e error) {
	code := e.Error()
	status := 400
	switch code {
	case "record not found", "not_found":
		code = "not_found"
		status = 404
	case "stale_revision", "file_upload_conflict", "file_upload_not_ready", "file_upload_completed", "file_upload_cancelled", "storage_quota_exceeded", "media_in_use", "job_busy":
		status = 409
	case "oss_disabled", "oss_configuration_required", "oss_permission_denied", "oss_unavailable", "oss_unverified", "oss_public_unverified", "oss_company_public_unverified", "oss_public_bucket_prefix_required", "disk_space_insufficient", "oss_writes_disabled":
		status = 503
	}
	if strings.ContainsAny(code, "\r\n? ") {
		code = "oss_unavailable"
		status = 503
	}
	fail(c, status, code)
}
func (a *App) cloudUploadOwner(c *gin.Context, req cloudUploadRequest) (uint64, uint64, error) {
	who := actor(c)
	owner := who.ID
	vendor := a.enterpriseID(owner)
	if req.BusinessID > 0 {
		switch req.BusinessKind {
		case "content":
			var v Content
			if a.DB.First(&v, req.BusinessID).Error != nil {
				return 0, 0, errors.New("not_found")
			}
			owner = v.OwnerID
			if v.Kind == "vendor" {
				vendor = v.ID
			} else {
				vendor = v.VendorID
			}
		case "resource":
			var v Resource
			if a.DB.First(&v, req.BusinessID).Error != nil {
				return 0, 0, errors.New("not_found")
			}
			owner = v.OwnerID
			vendor = v.VendorID
		default:
			return 0, 0, errors.New("invalid_request")
		}
		if who.Role == "merchant" && owner != who.ID {
			return 0, 0, errors.New("not_found")
		}
	}
	if who.Role != "merchant" && req.EnterpriseID > 0 {
		var v Content
		if a.DB.Where("id=? AND kind='vendor'", req.EnterpriseID).First(&v).Error != nil {
			return 0, 0, errors.New("not_found")
		}
		if req.BusinessID > 0 && owner != v.OwnerID {
			return 0, 0, errors.New("not_found")
		}
		owner = v.OwnerID
		vendor = v.ID
	}
	if who.Role == "merchant" && req.EnterpriseID > 0 && req.EnterpriseID != vendor {
		return 0, 0, errors.New("not_found")
	}
	if who.Role != "merchant" && vendor == 0 && req.Purpose != "category_image" && req.Purpose != "home_display" {
		return 0, 0, errors.New("vendor_required")
	}
	return owner, vendor, nil
}
func (a *App) validateCloudUpload(req *cloudUploadRequest) (string, error) {
	req.Filename = safeUploadFilename(req.Filename)
	ext := strings.ToLower(filepath.Ext(req.Filename))
	if len(req.IdempotencyKey) < 8 || len(req.IdempotencyKey) > 100 || strings.ContainsAny(req.IdempotencyKey, "\r\n\x00") {
		return "", errors.New("invalid_client_request_id")
	}
	max := int64(a.uploadConfig().ImageMiB) << 20
	mimeType := ""
	switch req.Kind {
	case "image", "technical":
		switch ext {
		case ".jpg", ".jpeg":
			mimeType = "image/jpeg"
		case ".png":
			mimeType = "image/png"
		case ".webp":
			mimeType = "image/webp"
		}
		if req.Kind == "image" && !validImagePurpose(req.Purpose) {
			return "", errors.New("invalid_image_purpose")
		}
	case "pdf":
		max = int64(a.uploadConfig().PDFMiB) << 20
		if ext == ".pdf" {
			mimeType = "application/pdf"
		}
	case "video":
		max = int64(a.uploadConfig().VideoMiB) << 20
		if ext == ".mp4" {
			mimeType = "video/mp4"
		} else if ext == ".mov" {
			mimeType = "video/quicktime"
		}
	case "ai_original":
		max = int64(settingInt("AI_IMPORT_MAX_FILE_BYTES", 30000000))
		switch ext {
		case ".pdf":
			mimeType = "application/pdf"
		case ".png":
			mimeType = "image/png"
		case ".jpg", ".jpeg":
			mimeType = "image/jpeg"
		}
		if ext != ".pdf" && max > int64(a.uploadConfig().ImageMiB)<<20 {
			max = int64(a.uploadConfig().ImageMiB) << 20
		}
	default:
		return "", errors.New("unsupported_file")
	}
	if req.Size < 1 || req.Size > max {
		return "", errors.New("file_too_large")
	}
	if mimeType == "" {
		return "", errors.New("unsupported_file")
	}
	if req.MimeType != "" && req.MimeType != mimeType && req.MimeType != "application/octet-stream" {
		return "", errors.New("invalid_file_type")
	}
	req.MimeType = mimeType
	return ext, nil
}
func (a *App) createCloudUpload(c *gin.Context, req cloudUploadRequest) (FileUploadSession, error) {
	var s FileUploadSession
	if env("OSS_UPLOADS_ENABLED", "true") != "true" {
		return s, errors.New("oss_writes_disabled")
	}
	ext, e := a.validateCloudUpload(&req)
	if e != nil {
		return s, e
	}
	owner, vendor, e := a.cloudUploadOwner(c, req)
	if e != nil {
		return s, e
	}
	p, e := a.cloudProfile("private")
	if e != nil {
		return s, errors.New("oss_configuration_required")
	}
	raw, _ := json.Marshal(req)
	fingerprint := digestText(string(raw))
	now := time.Now().UTC()
	e = a.DB.Transaction(func(tx *gorm.DB) error {
		var acc Account
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&acc, owner).Error; e != nil {
			return e
		}
		e := tx.Where("owner_id=? AND idempotency_key=?", owner, req.IdempotencyKey).First(&s).Error
		if e == nil {
			if s.Fingerprint != fingerprint {
				return errors.New("file_upload_conflict")
			}
			return nil
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		if e = a.syncOwnerInventory(tx, owner); e != nil {
			return e
		}
		used, reserved, quota, e := a.ownerUsage(tx, owner)
		if e != nil {
			return e
		}
		if phoneAccount(acc) && (req.Size > quota-used-reserved) {
			return errors.New("storage_quota_exceeded")
		}
		id := randomToken()
		key := fmt.Sprintf("companies/%d/drafts/%s%s", vendor, randomToken(), ext)
		if vendor == 0 {
			key = fmt.Sprintf("upload-temp/accounts/%d/%s%s", owner, randomToken(), ext)
		}
		if req.Kind == "ai_original" && vendor > 0 {
			key = fmt.Sprintf("companies/%d/ai-import/%s%s", vendor, randomToken(), ext)
		}
		mode := "single"
		if req.Size >= 32<<20 {
			mode = "multipart"
		}
		s = FileUploadSession{ID: id, OwnerID: owner, ActorID: actor(c).ID, EnterpriseID: vendor, BusinessID: req.BusinessID, BusinessType: req.Kind, BusinessScope: req.BusinessKind, Purpose: req.Purpose, OriginalFilename: req.Filename, MimeType: req.MimeType, ExpectedBytes: req.Size, StoreID: p.ID, ObjectKey: key, IdempotencyKey: req.IdempotencyKey, Fingerprint: fingerprint, UploadMode: mode, UploadState: "uploading", ExpiresAt: now.Add(2 * time.Hour), SignatureExpiresAt: now, CreatedAt: now, UpdatedAt: now}
		if e = tx.Create(&s).Error; e != nil {
			return e
		}
		return tx.Create(&MediaUploadReservation{ID: s.ID, OwnerID: owner, Bytes: req.Size, ExpiresAt: s.ExpiresAt, HoldUntilReclaimed: true}).Error
	})
	return s, e
}
func (a *App) ownedUpload(c *gin.Context) (FileUploadSession, error) {
	var s FileUploadSession
	e := a.DB.First(&s, "id=?", c.Param("id")).Error
	if e != nil {
		return s, errors.New("not_found")
	}
	if actor(c).Role == "merchant" && s.OwnerID != actor(c).ID {
		return s, errors.New("not_found")
	}
	return s, nil
}
func (a *App) uploadAuthorization(ctx context.Context, s *FileUploadSession) (gin.H, error) {
	if s.UploadState != "uploading" {
		return gin.H{"session": s}, nil
	}
	if !s.ExpiresAt.After(time.Now()) {
		return nil, errors.New("file_upload_cancelled")
	}
	p, e := a.storageProfile(s.StoreID)
	if e != nil {
		return nil, e
	}
	// Recover a successful PUT/Complete whose HTTP response was lost. Never ask
	// clients to overwrite an immutable object on retry.
	if s.SignatureExpiresAt.After(s.CreatedAt) {
		client, e := a.cloudClient(p, false)
		if e != nil {
			return nil, e
		}
		head, e := client.HeadObject(ctx, &oss.HeadObjectRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(s.ObjectKey)})
		if e == nil {
			if head.ContentLength != s.ExpectedBytes || oss.ToString(head.ContentType) != s.MimeType {
				return nil, errors.New("file_size_mismatch")
			}
			if e = a.DB.Model(&FileUploadSession{}).Where("id=? AND upload_state='uploading'", s.ID).Updates(map[string]any{"upload_state": "validating", "updated_at": time.Now().UTC()}).Error; e != nil {
				return nil, e
			}
			s.UploadState = "validating"
			return gin.H{"session": s}, nil
		}
		if !objectMissing(e) {
			return nil, cloudError(e)
		}
	}
	if s.UploadMode == "multipart" {
		if s.MultipartID == "" {
			client, e := a.cloudClient(p, false)
			if e != nil {
				return nil, e
			}
			r, e := client.InitiateMultipartUpload(ctx, &oss.InitiateMultipartUploadRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(s.ObjectKey), ContentType: oss.Ptr(s.MimeType)})
			if e != nil {
				return nil, cloudError(e)
			}
			s.MultipartID = oss.ToString(r.UploadId)
			result := a.DB.Model(&FileUploadSession{}).Where("id=? AND multipart_id='' AND upload_state='uploading'", s.ID).Update("multipart_id", s.MultipartID)
			if result.Error != nil || result.RowsAffected != 1 {
				_, _ = client.AbortMultipartUpload(ctx, &oss.AbortMultipartUploadRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(s.ObjectKey), UploadId: oss.Ptr(s.MultipartID)})
				a.DB.First(s, "id=?", s.ID)
				if result.Error != nil {
					return nil, errors.New("oss_unavailable")
				}
			}
		}
		client, e := a.cloudClient(p, false)
		if e != nil {
			return nil, e
		}
		list, e := client.ListParts(ctx, &oss.ListPartsRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(s.ObjectKey), UploadId: oss.Ptr(s.MultipartID)})
		if e != nil {
			return nil, cloudError(e)
		}
		ready := []int32{}
		count := (s.ExpectedBytes + cloudPartBytes - 1) / cloudPartBytes
		for _, part := range list.Parts {
			size := cloudPartBytes
			if int64(part.PartNumber) == count {
				size = s.ExpectedBytes - (count-1)*cloudPartBytes
			}
			if part.PartNumber > 0 && int64(part.PartNumber) <= count && part.Size == size {
				ready = append(ready, part.PartNumber)
			}
		}
		return gin.H{"session": s, "mode": "multipart", "part_size": cloudPartBytes, "part_count": int(math.Ceil(float64(s.ExpectedBytes) / float64(cloudPartBytes))), "parallel_parts": 2, "uploaded_parts": ready}, nil
	}
	r, e := a.signedObject(ctx, p, s.ObjectKey, "PUT", s.ExpectedBytes, s.MimeType)
	if e != nil {
		return nil, e
	}
	result := a.DB.Model(&FileUploadSession{}).Where("id=? AND upload_state='uploading'", s.ID).Updates(map[string]any{"signature_expires_at": gorm.Expr("GREATEST(signature_expires_at, ?)", r.Expiration), "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, errors.New("file_upload_cancelled")
	}
	return gin.H{"session": s, "mode": "single", "method": r.Method, "url": r.URL, "headers": r.SignedHeaders, "expires_at": r.Expiration}, nil
}
func (a *App) cloudPartAuthorization(c *gin.Context) {
	if !a.cloudDirectEnabled() {
		cloudFail(c, errors.New("oss_unverified"))
		return
	}
	s, e := a.ownedUpload(c)
	if e != nil {
		cloudFail(c, e)
		return
	}
	var req struct {
		Parts []int32 `json:"parts"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.Parts) < 1 || len(req.Parts) > 2 {
		fail(c, 400, "invalid_request")
		return
	}
	if s.UploadMode != "multipart" || s.UploadState != "uploading" || s.MultipartID == "" || !s.ExpiresAt.After(time.Now()) {
		cloudFail(c, errors.New("file_upload_not_ready"))
		return
	}
	p, e := a.storageProfile(s.StoreID)
	if e != nil {
		cloudFail(c, e)
		return
	}
	client, e := a.cloudClient(p, true)
	if e != nil {
		cloudFail(c, e)
		return
	}
	urls := []gin.H{}
	expiry := time.Now().UTC()
	for _, part := range req.Parts {
		count := (s.ExpectedBytes + cloudPartBytes - 1) / cloudPartBytes
		if part < 1 || int64(part) > count {
			fail(c, 400, "invalid_request")
			return
		}
		size := cloudPartBytes
		if int64(part) == count {
			size = s.ExpectedBytes - (count-1)*cloudPartBytes
		}
		r, e := client.Presign(c.Request.Context(), &oss.UploadPartRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(s.ObjectKey), UploadId: oss.Ptr(s.MultipartID), PartNumber: part, ContentLength: oss.Ptr(size)}, oss.PresignExpires(cloudAuthorizationLifetime()))
		if e != nil {
			cloudFail(c, cloudError(e))
			return
		}
		if r.Expiration.After(expiry) {
			expiry = r.Expiration
		}
		urls = append(urls, gin.H{"part_number": part, "offset": int64(part-1) * cloudPartBytes, "size": size, "url": r.URL, "headers": r.SignedHeaders, "expires_at": r.Expiration})
	}
	result := a.DB.Model(&FileUploadSession{}).Where("id=? AND upload_state='uploading'", s.ID).Update("signature_expires_at", gorm.Expr("GREATEST(signature_expires_at, ?)", expiry))
	if result.Error != nil || result.RowsAffected != 1 {
		cloudFail(c, errors.New("file_upload_cancelled"))
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"items": urls})
}
func (a *App) completeCloudUpload(c *gin.Context) {
	s, e := a.ownedUpload(c)
	if e != nil {
		cloudFail(c, e)
		return
	}
	if s.UploadState == "completed" || s.UploadState == "validating" {
		c.JSON(200, gin.H{"session": s})
		return
	}
	if s.UploadState != "uploading" || !s.ExpiresAt.After(time.Now()) {
		cloudFail(c, errors.New("file_upload_cancelled"))
		return
	}
	p, e := a.storageProfile(s.StoreID)
	if e != nil {
		cloudFail(c, e)
		return
	}
	client, e := a.cloudClient(p, false)
	if e != nil {
		cloudFail(c, e)
		return
	}
	if s.UploadMode == "multipart" {
		if s.MultipartID == "" {
			cloudFail(c, errors.New("file_upload_not_ready"))
			return
		}
		parts, e := client.ListParts(c.Request.Context(), &oss.ListPartsRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(s.ObjectKey), UploadId: oss.Ptr(s.MultipartID)})
		if e != nil { // Completion may have succeeded before a lost response.
			if _, he := client.HeadObject(c.Request.Context(), &oss.HeadObjectRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(s.ObjectKey)}); he != nil {
				cloudFail(c, cloudError(e))
				return
			}
		} else {
			want := (s.ExpectedBytes + cloudPartBytes - 1) / cloudPartBytes
			if int64(len(parts.Parts)) != want {
				cloudFail(c, errors.New("file_upload_not_ready"))
				return
			}
			ready := []oss.UploadPart{}
			for i, part := range parts.Parts {
				size := cloudPartBytes
				if int64(i+1) == want {
					size = s.ExpectedBytes - int64(i)*cloudPartBytes
				}
				if part.PartNumber != int32(i+1) || part.Size != size {
					cloudFail(c, errors.New("file_size_mismatch"))
					return
				}
				ready = append(ready, oss.UploadPart{PartNumber: part.PartNumber, ETag: part.ETag})
			}
			_, e = client.CompleteMultipartUpload(c.Request.Context(), &oss.CompleteMultipartUploadRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(s.ObjectKey), UploadId: oss.Ptr(s.MultipartID), CompleteMultipartUpload: &oss.CompleteMultipartUpload{Parts: ready}, ForbidOverwrite: oss.Ptr("true")})
			if e != nil {
				cloudFail(c, cloudError(e))
				return
			}
		}
	}
	head, e := client.HeadObject(c.Request.Context(), &oss.HeadObjectRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(s.ObjectKey)})
	if e != nil {
		cloudFail(c, cloudError(e))
		return
	}
	if head.ContentLength != s.ExpectedBytes || oss.ToString(head.ContentType) != s.MimeType {
		cloudFail(c, errors.New("file_size_mismatch"))
		return
	}
	r := a.DB.Model(&FileUploadSession{}).Where("id=? AND upload_state='uploading'", s.ID).Updates(map[string]any{"upload_state": "validating", "updated_at": time.Now().UTC(), "error": ""})
	if r.Error != nil {
		cloudFail(c, r.Error)
		return
	}
	a.DB.First(&s, "id=?", s.ID)
	c.JSON(202, gin.H{"session": s})
}
func (a *App) bindCloudUpload(tx *gorm.DB, c *gin.Context, mediaID uint64) error {
	id, ok := c.Get("oss_session_id")
	if !ok {
		return nil
	}
	token, _ := c.Get("oss_lease_token")
	r := tx.Model(&FileUploadSession{}).Where("id=? AND media_id=0 AND lease_token=? AND upload_state='validating'", id, token).Update("media_id", mediaID)
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return errors.New("file_upload_conflict")
	}
	return nil
}
func (a *App) cloudFileRoutes(admin, merchant *gin.RouterGroup) {
	a.cloudJobRoutes(admin, merchant)
	for _, group := range []*gin.RouterGroup{merchant, admin.Group("/storage")} {
		r := group.Group("/files")
		r.POST("/uploads", func(c *gin.Context) {
			if !a.cloudDirectEnabled() {
				cloudFail(c, errors.New("oss_unverified"))
				return
			}
			var req cloudUploadRequest
			if c.ShouldBindJSON(&req) != nil {
				fail(c, 400, "invalid_request")
				return
			}
			s, e := a.createCloudUpload(c, req)
			if e != nil {
				cloudFail(c, e)
				return
			}
			v, e := a.uploadAuthorization(c.Request.Context(), &s)
			if e != nil {
				cloudFail(c, e)
				return
			}
			c.Header("Cache-Control", "no-store")
			c.JSON(201, v)
		})
		r.GET("/uploads/:id", func(c *gin.Context) {
			s, e := a.ownedUpload(c)
			if e != nil {
				cloudFail(c, e)
				return
			}
			c.Header("Cache-Control", "no-store")
			c.JSON(200, gin.H{"session": s})
		})
		r.POST("/uploads/:id/authorization", func(c *gin.Context) {
			s, e := a.ownedUpload(c)
			if e != nil {
				cloudFail(c, e)
				return
			}
			if !a.cloudDirectEnabled() {
				cloudFail(c, errors.New("oss_unverified"))
				return
			}
			v, e := a.uploadAuthorization(c.Request.Context(), &s)
			if e != nil {
				cloudFail(c, e)
				return
			}
			c.Header("Cache-Control", "no-store")
			c.JSON(200, v)
		})
		r.POST("/uploads/:id/parts", a.cloudPartAuthorization)
		r.POST("/uploads/:id/complete", a.completeCloudUpload)
		r.POST("/uploads/:id/cancel", func(c *gin.Context) {
			s, e := a.ownedUpload(c)
			if e != nil {
				cloudFail(c, e)
				return
			}
			if s.MediaID > 0 {
				cloudFail(c, errors.New("file_upload_completed"))
				return
			}
			if s.UploadState == "validating" && s.LeaseUntil != nil && s.LeaseUntil.After(time.Now()) {
				cloudFail(c, errors.New("job_busy"))
				return
			}
			result := a.DB.Model(&FileUploadSession{}).Where("id=? AND media_id=0 AND (lease_until IS NULL OR lease_until<=?)", s.ID, time.Now().UTC()).Updates(map[string]any{"upload_state": "cancelled", "updated_at": time.Now().UTC()})
			if result.Error != nil || result.RowsAffected != 1 {
				cloudFail(c, errors.New("job_busy"))
				return
			}
			c.JSON(202, gin.H{"state": "cancelled", "reclaim_pending": true})
		})
		r.POST("/uploads/:id/retry", func(c *gin.Context) {
			s, e := a.ownedUpload(c)
			if e != nil {
				cloudFail(c, e)
				return
			}
			if s.UploadState != "failed" {
				cloudFail(c, errors.New("file_upload_conflict"))
				return
			}
			if !s.ExpiresAt.After(time.Now()) && s.MediaID == 0 {
				cloudFail(c, errors.New("file_upload_cancelled"))
				return
			}
			if e = a.DB.Model(&s).Where("upload_state='failed'").Updates(map[string]any{"upload_state": "validating", "error": "", "lease_token": "", "lease_until": nil}).Error; e != nil {
				cloudFail(c, e)
				return
			}
			c.JSON(202, gin.H{"id": s.ID})
		})
		r.GET("", a.listCloudFiles)
		r.GET("/:id", a.cloudFileDetail)
		r.GET("/:id/access", a.cloudFileAccess)
		r.DELETE("/:id", func(c *gin.Context) {
			m, e := a.ownedCloudMedia(c)
			if e != nil {
				cloudFail(c, e)
				return
			}
			_ = m
			a.deleteUnusedMedia(c)
		})
	}
	admin.GET("/storage/oss", func(c *gin.Context) {
		c.JSON(200, gin.H{"enabled": cloudFilesEnabled(), "config": cloudConfig(), "direct_upload_ready": a.cloudReady(false), "public_delivery_ready": a.cloudReady(true), "default_quota_bytes": enterpriseOSSQuota})
	})
	admin.POST("/storage/oss/check", a.checkCloudFiles)
}
func (a *App) ownedCloudMedia(c *gin.Context) (Media, error) {
	id, e := strconv.ParseUint(c.Param("id"), 10, 64)
	if e != nil {
		return Media{}, errors.New("not_found")
	}
	var m Media
	if a.DB.First(&m, id).Error != nil {
		return m, errors.New("not_found")
	}
	if !a.canReadPrivateMedia(actor(c), m) {
		return m, errors.New("not_found")
	}
	return m, nil
}
func (a *App) listCloudFiles(c *gin.Context) {
	p, n := page(c)
	query := a.DB.Model(&EnterpriseFile{})
	query = query.Where("file_status<>'deleted'")
	if actor(c).Role == "merchant" {
		query = query.Where("owner_id=?", actor(c).ID)
	} else if id := uintQuery(c, "enterprise_id"); id > 0 {
		query = query.Where("enterprise_id=?", id)
	}
	if kind := c.Query("kind"); kind != "" {
		query = query.Where("business_type=?", kind)
	}
	var total int64
	var rows []EnterpriseFile
	if e := query.Count(&total).Error; e != nil {
		cloudFail(c, e)
		return
	}
	if e := query.Order("media_id DESC").Offset((p - 1) * n).Limit(n).Find(&rows).Error; e != nil {
		cloudFail(c, e)
		return
	}
	c.JSON(200, gin.H{"items": rows, "total": total, "page": p, "page_size": n})
}
func (a *App) cloudFileDetail(c *gin.Context) {
	m, e := a.ownedCloudMedia(c)
	if e != nil {
		cloudFail(c, e)
		return
	}
	var f EnterpriseFile
	if a.DB.First(&f, "media_id=?", m.ID).Error != nil {
		f = EnterpriseFile{MediaID: m.ID, EnterpriseID: a.enterpriseID(m.OwnerID), FileStatus: "legacy"}
	}
	var rows []EnterpriseFileObject
	a.DB.Where("media_id=?", m.ID).Find(&rows)
	c.JSON(200, gin.H{"file": f, "objects": rows})
}
func (a *App) cloudFileAccess(c *gin.Context) {
	m, e := a.ownedCloudMedia(c)
	if e != nil {
		cloudFail(c, e)
		return
	}
	var meta MediaMetadata
	if a.DB.First(&meta, "media_id=?", m.ID).Error == nil && meta.State != "ready" {
		cloudFail(c, errors.New("file_upload_not_ready"))
		return
	}
	key := m.Path
	if c.Query("thumb") == "1" {
		key = m.ThumbPath
	}
	if key == "" {
		cloudFail(c, errors.New("not_found"))
		return
	}
	o, e := a.objectRecord(key)
	if e != nil {
		cloudFail(c, e)
		return
	}
	p, e := a.storageProfile(o.StoreID)
	if e != nil {
		cloudFail(c, e)
		return
	}
	c.Header("Cache-Control", "no-store")
	if p.AuthMode != "ecs_role" {
		c.JSON(200, gin.H{"url": a.mediaAddress(c, m.ID, func() string {
			if c.Query("thumb") == "1" {
				return "?thumb=1"
			}
			return ""
		}()), "direct": false, "size": o.Size})
		return
	}
	if o.RemoteKey != "" {
		key = o.RemoteKey
	}
	r, e := a.signedObject(c.Request.Context(), p, key, "GET", 0, "")
	if e != nil {
		cloudFail(c, e)
		return
	}
	c.JSON(200, gin.H{"url": r.URL, "expires_at": r.Expiration, "direct": true, "size": o.Size})
}
