package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestContentReviewMigrationPrivacyAndLegacyEndpoints(t *testing.T) {
	a := importTestAppAtSchema(t, 25)
	ownerToken := resourceTestSession(t, a, "merchant")
	other := resourceTestSession(t, a, "merchant")
	admin := resourceTestSession(t, a, "admin")
	owner := serviceTestOwner(t, a, ownerToken)
	evidence := serviceTestLegacyEvidence(t, a, ownerToken)
	old := BusinessVerification{AccountID: owner, CompanyName: "历史企业", CreditCode: serviceTestCreditCode(), Representative: "历史敏感姓名", AgentName: "历史经办姓名", Contact: "private", LicenseID: evidence, Status: "revoked", Version: 7, ReviewNote: "archived", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := a.DB.Create(&old).Error; err != nil {
		t.Fatal(err)
	}
	oldRefs := serviceTestRefs(t, a, "terms", "privacy", "showroom-agreement")
	if err := a.DB.Transaction(func(tx *gorm.DB) error {
		return acceptServiceDocuments(tx, owner, oldRefs, "web", []string{"terms", "privacy", "showroom-agreement"})
	}); err != nil {
		t.Fatal(err)
	}
	var beforeDocs, beforeFiles int64
	a.DB.Model(&ServiceDocument{}).Count(&beforeDocs)
	a.DB.Model(&ServiceEvidence{}).Count(&beforeFiles)
	if err := migrate(a.DB); err != nil {
		t.Fatal(err)
	}
	if err := migrate(a.DB); err != nil {
		t.Fatal(err)
	}
	if err := migrateContentReview(a.DB); err != nil {
		t.Fatal(err)
	}
	var afterDocs, afterFiles int64
	a.DB.Model(&ServiceDocument{}).Count(&afterDocs)
	a.DB.Model(&ServiceEvidence{}).Count(&afterFiles)
	if afterDocs != beforeDocs+4 || afterFiles != beforeFiles {
		t.Fatal("migration was destructive or repeated", beforeDocs, afterDocs)
	}
	var restored BusinessVerification
	a.DB.First(&restored, owner)
	if restored.Representative != old.Representative || restored.Status != "revoked" || restored.Version != 7 {
		t.Fatal("historical record changed")
	}
	tc := testClient{t, a.router()}
	response := tc.call("GET", "/merchant/verification", ownerToken, nil, 200)
	raw, _ := json.Marshal(response)
	if response["company_verification_mode"] != "content_review" || strings.Contains(string(raw), "历史敏感姓名") || strings.Contains(string(raw), "license_id") {
		t.Fatal("legacy read exposes identity", string(raw))
	}
	tc.call("PUT", "/merchant/verification", ownerToken, gin.H{"representative": "new private name", "license_id": evidence}, 410)
	tc.call("POST", "/merchant/verification/submit", ownerToken, gin.H{}, 410)
	tc.call("POST", fmt.Sprintf("/admin/verifications/%d/review", owner), admin, gin.H{}, 410)
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	_ = form.WriteField("purpose", "verification")
	f, _ := form.CreateFormFile("file", "licence.png")
	_, _ = f.Write([]byte("unused"))
	_ = form.Close()
	req := httptest.NewRequest("POST", "/api/v1/public/service-evidence", &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	w := httptest.NewRecorder()
	a.router().ServeHTTP(w, req)
	if w.Code != 410 {
		t.Fatal(w.Code, w.Body.String())
	}
	if serviceTestReadEvidence(a, evidence, ownerToken, "") != 200 || serviceTestReadEvidence(a, evidence, admin, "") != 200 || serviceTestReadEvidence(a, evidence, other, "") != 404 || serviceTestReadEvidence(a, evidence, "", "") != 404 {
		t.Fatal("historical evidence permissions changed")
	}
	if err := publicationCompliance(a.DB, owner, 0, nil); err == nil || err.Error() != "agreement_required" {
		t.Fatal("old agreement unexpectedly current", err)
	}
	tc.call("POST", "/merchant/service-acceptances", ownerToken, gin.H{"agreements": serviceTestRefs(t, a, "terms", "privacy", "showroom-agreement"), "source": "android"}, 200)
	if err := publicationCompliance(a.DB, owner, 0, nil); err != nil {
		t.Fatal("archived revoked verification still blocks content review", err)
	}
	var acceptances int64
	a.DB.Model(&ServiceAcceptance{}).Where("account_id=?", owner).Count(&acceptances)
	if acceptances != 6 {
		t.Fatal("old confirmations not retained", acceptances)
	}
	cfg := tc.call("GET", "/public/config", "", nil, 200)
	active, err := activeServiceDocuments(a.DB)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range active {
		if d.DocKey == "privacy" && (!strings.Contains(d.Body, "注册及登录处理") || !strings.Contains(d.BodyEN, "Registration and sign-in process")) {
			t.Fatal("account privacy disclosure missing")
		}
	}
	if cfg["company_verification_mode"] != "content_review" || cfg["capabilities"].(map[string]any)["company_verification"] != false {
		t.Fatal(cfg)
	}
}
