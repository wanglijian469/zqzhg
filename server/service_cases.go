package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BusinessVerification struct {
	AccountID       uint64    `gorm:"primaryKey" json:"account_id"`
	CompanyName     string    `json:"company_name"`
	CreditCode      string    `json:"credit_code"`
	Representative  string    `json:"representative"`
	AgentName       string    `json:"agent_name"`
	Contact         string    `json:"contact"`
	LicenseID       uint64    `json:"license_id"`
	AuthorizationID uint64    `json:"authorization_id"`
	Status          string    `json:"status"`
	Version         uint64    `json:"version"`
	ReviewNote      string    `json:"review_note"`
	ReviewedBy      uint64    `json:"-"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
type ServiceEvidence struct {
	ID             uint64    `json:"id"`
	OwnerID        uint64    `json:"-"`
	CredentialHash string    `json:"-"`
	Purpose        string    `json:"purpose"`
	ObjectKey      string    `json:"-"`
	Name           string    `json:"name"`
	Mime           string    `json:"mime"`
	Size           int64     `json:"size"`
	CreatedAt      time.Time `json:"created_at"`
}

func (ServiceEvidence) TableName() string { return "service_evidence" }

type ServiceRequest struct {
	ID              uint64    `json:"id"`
	OwnerID         uint64    `json:"owner_id"`
	Kind            string    `json:"kind"`
	TargetURL       string    `json:"target_url"`
	Subject         string    `json:"subject"`
	Description     string    `json:"description"`
	Contact         string    `json:"contact"`
	Attachments     string    `json:"attachments"`
	Status          string    `json:"status"`
	Reply           string    `json:"reply"`
	Version         uint64    `json:"version"`
	CredentialHash  string    `json:"-"`
	IdempotencyHash string    `json:"-"`
	UpdatedBy       uint64    `json:"-"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

var privateCredentialPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func validCreditCode(code string) bool {
	const alphabet = "0123456789ABCDEFGHJKLMNPQRTUWXY"
	weights := []int{1, 3, 9, 27, 19, 26, 16, 17, 20, 29, 25, 13, 8, 24, 10, 30, 28}
	if len(code) != 18 {
		return false
	}
	sum := 0
	for i := 0; i < 17; i++ {
		n := strings.IndexByte(alphabet, code[i])
		if n < 0 {
			return false
		}
		sum += n * weights[i]
	}
	return code[17] == alphabet[(31-sum%31)%31]
}
func checkEvidence(tx *gorm.DB, ids []uint64, owner uint64, credential, purpose string) error {
	if len(ids) > 5 {
		return serviceInvalid()
	}
	for _, id := range ids {
		var f ServiceEvidence
		e := tx.First(&f, id).Error
		if e != nil {
			return serviceFailure{"invalid_evidence", 400}
		}
		if f.Purpose != purpose || (owner == 0 && f.CredentialHash != digestText(credential)) || (owner > 0 && f.OwnerID != owner) {
			return serviceFailure{"invalid_evidence", 400}
		}
	}
	return nil
}
func (a *App) serviceCaseRoutes(pub, merchant, admin *gin.RouterGroup) {
	pub.POST("/service-evidence", a.uploadServiceEvidence)
	pub.GET("/service-evidence/:id", a.readServiceEvidence)
	pub.POST("/service-requests", a.createServiceRequest)
	pub.POST("/service-requests/lookup", a.lookupServiceRequest)
	pub.POST("/service-requests/:id/supplement", a.supplementServiceRequest)
	merchant.GET("/service-requests", func(c *gin.Context) {
		items := []ServiceRequest{}
		p, n := page(c)
		var total int64
		q := a.DB.Model(&ServiceRequest{}).Where("owner_id=?", actor(c).ID)
		if e := q.Count(&total).Error; e != nil {
			dbFail(c, e)
			return
		}
		if e := q.Order("id DESC").Offset((p - 1) * n).Limit(n).Find(&items).Error; e != nil {
			dbFail(c, e)
			return
		}
		c.JSON(200, gin.H{"items": items, "total": total, "page": p, "page_size": n})
	})
	merchant.GET("/verification", a.companyAgreementState)
	merchant.PUT("/verification", verificationMerged)
	merchant.POST("/verification/submit", verificationMerged)
	admin.GET("/verifications", func(c *gin.Context) {
		p, n := page(c)
		c.JSON(200, gin.H{"items": []gin.H{}, "total": 0, "page": p, "page_size": n, "company_verification_mode": companyVerificationMode})
	})
	admin.POST("/verifications/:id/review", verificationMerged)
	admin.GET("/service-requests", func(c *gin.Context) {
		p, n := page(c)
		q := a.DB.Model(&ServiceRequest{})
		if state := c.Query("status"); state != "" {
			q = q.Where("status=?", state)
		}
		var total int64
		items := []ServiceRequest{}
		if e := q.Count(&total).Error; e != nil {
			dbFail(c, e)
			return
		}
		if e := q.Order("id DESC").Offset((p - 1) * n).Limit(n).Find(&items).Error; e != nil {
			dbFail(c, e)
			return
		}
		c.JSON(200, gin.H{"items": items, "total": total, "page": p, "page_size": n})
	})
	admin.PUT("/service-requests/:id", a.reviewServiceRequest)
}
func (a *App) uploadServiceEvidence(c *gin.Context) {
	if !permit("service-evidence:"+c.ClientIP(), 12, time.Hour) {
		fail(c, 429, "rate_limited")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 11000000)
	purpose := c.PostForm("purpose")
	if purpose == "verification" {
		if c.Request.MultipartForm != nil {
			defer c.Request.MultipartForm.RemoveAll()
		}
		verificationMerged(c)
		return
	}
	file, header, e := c.Request.FormFile("file")
	if e != nil {
		fail(c, 400, "invalid_evidence")
		return
	}
	defer file.Close()
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	credential := c.GetHeader("X-Service-Credential")
	account, authErr := a.readAccount(c)
	owner := uint64(0)
	if authErr == nil && account.Role == "merchant" {
		owner = account.ID
	}
	if (purpose != "verification" && purpose != "request") || (purpose == "verification" && owner == 0) || (owner == 0 && !privateCredentialPattern.MatchString(credential)) {
		fail(c, 403, "forbidden")
		return
	}
	if header.Size <= 0 || header.Size > 10000000 {
		fail(c, 400, "invalid_evidence")
		return
	}
	if e = a.checkUploadDisks(uint64(header.Size)); e != nil {
		dbFail(c, e)
		return
	}
	head := make([]byte, 512)
	n, _ := io.ReadFull(file, head)
	head = head[:n]
	mime := http.DetectContentType(head)
	if mime != "image/jpeg" && mime != "image/png" && mime != "image/webp" && mime != "application/pdf" {
		fail(c, 400, "invalid_evidence")
		return
	}
	temp, e := os.CreateTemp(a.mediaTempDir(), "evidence-*")
	if e != nil {
		dbFail(c, e)
		return
	}
	defer os.Remove(temp.Name())
	_, e = temp.Write(head)
	if e == nil {
		_, e = io.Copy(temp, io.LimitReader(file, 10000000))
	}
	closeErr := temp.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		dbFail(c, e)
		return
	}
	info, e := os.Stat(temp.Name())
	if e != nil || info.Size() != header.Size {
		fail(c, 400, "invalid_evidence")
		return
	}
	key := "private/service/" + randomToken()
	if e = a.storeWorkFile(c.Request.Context(), key, temp.Name()); e != nil {
		dbFail(c, e)
		return
	}
	name := filepath.Base(strings.ReplaceAll(header.Filename, "\\", "/"))
	if utf8.RuneCountInString(name) > 200 {
		name = "evidence"
	}
	row := ServiceEvidence{OwnerID: owner, CredentialHash: digestText(credential), Purpose: purpose, ObjectKey: key, Name: name, Mime: mime, Size: info.Size(), CreatedAt: time.Now().UTC()}
	if e = a.DB.Create(&row).Error; e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(201, row)
}
func (a *App) readServiceEvidence(c *gin.Context) {
	if !permit("service-evidence-read:"+c.ClientIP(), 120, time.Minute) {
		fail(c, 429, "rate_limited")
		return
	}
	var f ServiceEvidence
	if a.DB.First(&f, id(c)).Error != nil {
		fail(c, 404, "not_found")
		return
	}
	account, e := a.readAccount(c)
	allowed := e == nil && (account.ID == f.OwnerID && f.OwnerID > 0 || a.hasPermission(account, map[string]string{"verification": "compliance.review", "request": "service.manage"}[f.Purpose]))
	credential := c.GetHeader("X-Service-Credential")
	if !allowed && f.Purpose == "request" && privateCredentialPattern.MatchString(credential) && f.CredentialHash == digestText(credential) {
		allowed = true
	}
	if !allowed {
		fail(c, 404, "not_found")
		return
	}
	if e == nil && platformAccount(account) {
		a.audit(account.ID, "service_evidence", f.ID, "private_evidence_read")
	}
	c.Header("Content-Type", f.Mime)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"evidence-%d\"", f.ID))
	c.Header("Cache-Control", "no-store")
	a.serveObject(c, f.ObjectKey)
}
func (a *App) createServiceRequest(c *gin.Context) {
	if !permit("service-request:"+c.ClientIP(), 8, time.Hour) {
		fail(c, 429, "rate_limited")
		return
	}
	var req struct {
		Kind        string   `json:"kind"`
		TargetURL   string   `json:"target_url"`
		Subject     string   `json:"subject"`
		Description string   `json:"description"`
		Contact     string   `json:"contact"`
		Attachments []uint64 `json:"attachments"`
		Credential  string   `json:"credential"`
		RequestID   string   `json:"request_id"`
	}
	if c.ShouldBindJSON(&req) != nil || !privateCredentialPattern.MatchString(req.Credential) || !privateCredentialPattern.MatchString(req.RequestID) || strings.TrimSpace(req.Subject) == "" || strings.TrimSpace(req.Description) == "" || utf8.RuneCountInString(req.Subject) > 200 || utf8.RuneCountInString(req.Description) > 10000 || utf8.RuneCountInString(req.Contact) > 200 || len(req.TargetURL) > 1000 {
		fail(c, 400, "invalid_request")
		return
	}
	valid := false
	for _, k := range []string{"complaint", "appeal", "export", "unpublish", "withdraw", "account_close", "storage"} {
		if req.Kind == k {
			valid = true
		}
	}
	if !valid {
		fail(c, 400, "invalid_request")
		return
	}
	owner := uint64(0)
	if account, e := a.readAccount(c); e == nil && account.Role == "merchant" {
		owner = account.ID
	}
	if req.Kind != "complaint" && owner == 0 {
		fail(c, 401, "unauthorized")
		return
	}
	target := req.TargetURL
	if req.Kind != "complaint" {
		var v Content
		if e := a.DB.Where("owner_id=? AND kind='vendor'", owner).First(&v).Error; e != nil && req.Kind != "storage" {
			dbFail(c, e)
			return
		}
		if v.ID > 0 {
			target = "/v/" + showroomSlug(v)
		}
	}
	if e := checkEvidence(a.DB, req.Attachments, owner, req.Credential, "request"); e != nil {
		dbFail(c, e)
		return
	}
	hash := digestText(fmt.Sprintf("%d:%s:%s", owner, req.Credential, req.RequestID))
	var existing ServiceRequest
	if e := a.DB.Where("idempotency_hash=?", hash).First(&existing).Error; e == nil {
		c.JSON(200, existing)
		return
	} else if !errors.Is(e, gorm.ErrRecordNotFound) {
		dbFail(c, e)
		return
	}
	now := time.Now().UTC()
	attachmentBytes, _ := json.Marshal(req.Attachments)
	attachments := string(attachmentBytes)
	row := ServiceRequest{OwnerID: owner, Kind: req.Kind, TargetURL: target, Subject: strings.TrimSpace(req.Subject), Description: strings.TrimSpace(req.Description), Contact: strings.TrimSpace(req.Contact), Attachments: attachments, Status: "received", Version: 1, Reply: "", CredentialHash: digestText(req.Credential), IdempotencyHash: hash, CreatedAt: now, UpdatedAt: now}
	if e := a.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; e != nil {
		dbFail(c, e)
		return
	}
	if row.ID == 0 {
		if e := a.DB.Where("idempotency_hash=?", hash).First(&row).Error; e != nil {
			dbFail(c, e)
			return
		}
	}
	c.JSON(201, row)
}
func (a *App) lookupServiceRequest(c *gin.Context) {
	if !permit("service-lookup:"+c.ClientIP(), 30, time.Minute) {
		fail(c, 429, "rate_limited")
		return
	}
	var req struct {
		ID         uint64 `json:"id"`
		Credential string `json:"credential"`
	}
	if c.ShouldBindJSON(&req) != nil || !privateCredentialPattern.MatchString(req.Credential) {
		fail(c, 404, "not_found")
		return
	}
	var row ServiceRequest
	if a.DB.Where("id=? AND credential_hash=?", req.ID, digestText(req.Credential)).First(&row).Error != nil {
		fail(c, 404, "not_found")
		return
	}
	c.JSON(200, row)
}
func (a *App) supplementServiceRequest(c *gin.Context) {
	if !permit("service-supplement:"+c.ClientIP(), 15, time.Hour) {
		fail(c, 429, "rate_limited")
		return
	}
	var req struct {
		Credential  string `json:"credential"`
		Description string `json:"description"`
		Version     uint64 `json:"version"`
	}
	if c.ShouldBindJSON(&req) != nil || !privateCredentialPattern.MatchString(req.Credential) || strings.TrimSpace(req.Description) == "" || utf8.RuneCountInString(req.Description) > 2000 {
		fail(c, 400, "invalid_request")
		return
	}
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		var row ServiceRequest
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND credential_hash=?", id(c), digestText(req.Credential)).First(&row).Error; e != nil {
			return e
		}
		if row.Version != req.Version || row.Status == "closed" {
			return serviceFailure{"stale_revision", 409}
		}
		text := row.Description + "\n[" + time.Now().UTC().Format(time.RFC3339) + "] " + req.Description
		if utf8.RuneCountInString(text) > 20000 {
			return serviceInvalid()
		}
		return tx.Model(&row).Updates(map[string]any{"description": text, "status": "received", "version": row.Version + 1, "updated_at": time.Now().UTC()}).Error
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func (a *App) reviewServiceRequest(c *gin.Context) {
	var req struct {
		Status  string `json:"status"`
		Reply   string `json:"reply"`
		Version uint64 `json:"version"`
	}
	if c.ShouldBindJSON(&req) != nil || (req.Status != "received" && req.Status != "processing" && req.Status != "needs_info" && req.Status != "closed") || utf8.RuneCountInString(req.Reply) > 5000 || ((req.Status == "closed" || req.Status == "needs_info") && strings.TrimSpace(req.Reply) == "") {
		fail(c, 400, "invalid_request")
		return
	}
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		var row ServiceRequest
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, id(c)).Error; e != nil {
			return e
		}
		if row.Version != req.Version {
			return serviceFailure{"stale_revision", 409}
		}
		if e := tx.Model(&row).Updates(map[string]any{"status": req.Status, "reply": req.Reply, "version": row.Version + 1, "updated_by": actor(c).ID, "updated_at": time.Now().UTC()}).Error; e != nil {
			return e
		}
		return recordOperation(tx, actor(c).ID, "service_request", row.ID, "request_"+req.Status, req.Reply)
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
