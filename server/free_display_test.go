package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func serviceTestRefs(t *testing.T, a *App, keys ...string) []AgreementRef {
	t.Helper()
	docs, e := activeServiceDocuments(a.DB)
	if e != nil {
		t.Fatal(e)
	}
	refs := []AgreementRef{}
	for _, d := range docs {
		for _, k := range keys {
			if d.DocKey == k {
				refs = append(refs, AgreementRef{d.DocKey, d.Version, d.Digest})
			}
		}
	}
	return refs
}
func serviceTestOwner(t *testing.T, a *App, token string) uint64 {
	t.Helper()
	var s Session
	if e := a.DB.Where("token_hash=?", digest(token)).First(&s).Error; e != nil {
		t.Fatal(e)
	}
	return s.AccountID
}
func serviceTestCreditCode() string {
	code := "91130100MA0000000"
	alphabet := "0123456789ABCDEFGHJKLMNPQRTUWXY"
	weights := []int{1, 3, 9, 27, 19, 26, 16, 17, 20, 29, 25, 13, 8, 24, 10, 30, 28}
	sum := 0
	for i := 0; i < 17; i++ {
		sum += strings.IndexByte(alphabet, code[i]) * weights[i]
	}
	return code + string(alphabet[(31-sum%31)%31])
}
func serviceTestEvidence(t *testing.T, a *App, token, credential, purpose string) uint64 {
	t.Helper()
	var imageBytes bytes.Buffer
	_ = png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	var data bytes.Buffer
	writer := multipart.NewWriter(&data)
	f, _ := writer.CreateFormFile("file", "evidence.png")
	_, _ = f.Write(imageBytes.Bytes())
	_ = writer.WriteField("purpose", purpose)
	_ = writer.Close()
	request := httptest.NewRequest("POST", "/api/v1/public/service-evidence", &data)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	request.Header.Set("X-Service-Credential", credential)
	request.RemoteAddr = "127.0.0.9:40001"
	response := httptest.NewRecorder()
	a.router().ServeHTTP(response, request)
	if response.Code != 201 {
		t.Fatal(response.Code, response.Body.String())
	}
	var result ServiceEvidence
	if e := json.Unmarshal(response.Body.Bytes(), &result); e != nil {
		t.Fatal(e)
	}
	return result.ID
}
func serviceTestReadEvidence(a *App, id uint64, token, credential string) int {
	request := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/public/service-evidence/%d", id), nil)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	request.Header.Set("X-Service-Credential", credential)
	response := httptest.NewRecorder()
	a.router().ServeHTTP(response, request)
	return response.Code
}

func TestFreeDisplayMigrationAndStorage(t *testing.T) {
	a := importTestAppAtSchema(t, 22)
	cfg := defaultSystemConfig()
	cfg.Uploads.DefaultQuotaBytes = 2 << 30
	raw, _ := json.Marshal(cfg)
	if e := a.DB.Create(&SystemSetting{SettingKey: "system_config_v3", Value: string(raw), UpdatedAt: time.Now()}).Error; e != nil {
		t.Fatal(e)
	}
	normal := Account{Username: "default-quota-company", Role: "merchant"}
	custom := Account{Username: "custom-quota-company", Role: "merchant"}
	a.DB.Create(&normal)
	a.DB.Create(&custom)
	quota := int64(300000000)
	a.DB.Create(&MerchantStorageQuota{AccountID: normal.ID, Initialized: true})
	a.DB.Create(&MerchantStorageQuota{AccountID: custom.ID, QuotaBytes: &quota, Initialized: true})
	a.DB.Create(&MediaObjectOwner{ObjectKey: "existing-lawful-original", OwnerID: normal.ID, Size: 220000000})
	old := v2Published(t, a, "vendor", 0, 1, 0, 0, true, 0, "历史免费展示企业")
	if e := migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	if e := migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	saved, e := a.systemConfig()
	if e != nil || saved.Uploads.DefaultQuotaBytes != 200000000 {
		t.Fatal(saved, e)
	}
	usage, e := a.usageJSON(normal.ID)
	if e != nil || usage["used_bytes"] != int64(220000000) || usage["quota_bytes"] != int64(200000000) || usage["remaining_bytes"] != int64(0) || usage["state"] != "exceeded" {
		t.Fatal(usage, e)
	}
	customUsage, e := a.usageJSON(custom.ID)
	if e != nil || customUsage["quota_bytes"] != quota {
		t.Fatal(customUsage, e)
	}
	if _, e = a.reserveUpload(normal.ID, 1); e == nil || e.Error() != "storage_quota_exceeded" {
		t.Fatal("over-quota upload admitted", e)
	}
	var count int64
	a.DB.Model(&MediaObjectOwner{}).Where("owner_id=?", normal.ID).Count(&count)
	if count != 1 {
		t.Fatal("historical material lost")
	}
	var pool HomeRotationPool
	if e = a.DB.First(&pool, "vendor_id=?", old.ID).Error; e != nil || pool.State != "approved" {
		t.Fatal(pool, e)
	}
	a.DB.Create(&MediaObjectOwner{ObjectKey: "warning-object", OwnerID: custom.ID, Size: 270000000})
	u, e := a.usageJSON(custom.ID)
	if e != nil || u["state"] != "warning" || u["remaining_bytes"] != int64(30000000) {
		t.Fatal(u, e)
	}
	a.DB.Delete(&MediaObjectOwner{}, "object_key=?", "warning-object")
	var leases []*UploadLease
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lease, e := a.reserveUpload(custom.ID, 100000000)
			if e == nil {
				mu.Lock()
				leases = append(leases, lease)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(leases) != 3 {
		t.Fatal("reservation concurrency", len(leases))
	}
	for _, lease := range leases {
		lease.Release()
	}
	u, _ = a.usageJSON(custom.ID)
	if u["reserved_bytes"] != int64(0) {
		t.Fatal(u)
	}
}
func TestFreeDisplayVerificationAgreementsAndPublication(t *testing.T) {
	a := importTestApp(t)
	admin := resourceTestSession(t, a, "admin")
	merchant := resourceTestSession(t, a, "merchant")
	owner := serviceTestOwner(t, a, merchant)
	tc := testClient{t, a.router()}
	company := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "简化入驻验收企业", MainProducts: "耕整地机械", RegionName: "河北省 邢台市 宁晋县", Phone: "0319-1234567"}}, 201)
	vid, rid := number(company["id"]), company["revision_id"]
	product := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "product", "vendor_id": company["id"], "data": Payload{Name: "简化入驻验收产品"}}, 201)
	pid, prid := number(product["id"]), product["revision_id"]
	for _, prefix := range []string{"merchant", "admin"} {
		tok := merchant
		if prefix == "admin" {
			tok = admin
		}
		tc.call("POST", "/"+prefix+"/contents/"+vid+"/submit", tok, gin.H{"revision_id": rid}, 409)
		tc.call("POST", "/"+prefix+"/contents/"+pid+"/submit", tok, gin.H{"revision_id": prid}, 409)
	}
	tc.call("PUT", "/merchant/verification", merchant, gin.H{"representative": "discard", "license_id": 123}, 410)
	tc.call("POST", "/merchant/verification/submit", merchant, gin.H{}, 410)
	tc.call("POST", fmt.Sprintf("/admin/verifications/%d/review", owner), admin, gin.H{"decision": "approve"}, 410)
	refs := serviceTestRefs(t, a, "terms", "privacy", "showroom-agreement")
	bad := append([]AgreementRef{}, refs...)
	bad[0].Digest = "forged"
	tc.call("POST", "/merchant/service-acceptances", merchant, gin.H{"agreements": bad, "source": "android"}, 409)
	tc.call("POST", "/merchant/service-acceptances", merchant, gin.H{"agreements": refs, "source": "android"}, 200)
	tc.call("POST", "/merchant/contents/"+vid+"/submit", merchant, gin.H{"revision_id": rid}, 200)
	tc.call("POST", "/admin/contents/"+pid+"/submit", admin, gin.H{"revision_id": prid}, 200)
	tc.call("POST", "/admin/revisions/"+number(prid)+"/review", admin, gin.H{"decision": "approve"}, 200)
	tc.call("GET", "/public/products/"+pid, "", nil, 404)
	tc.call("POST", "/admin/revisions/"+number(rid)+"/review", admin, gin.H{"decision": "approve"}, 200)
	tc.call("GET", "/public/products/"+pid, "", nil, 200)
	docs, _ := activeServiceDocuments(a.DB)
	var original ServiceDocument
	for _, d := range docs {
		if d.DocKey == "showroom-agreement" {
			original = d
		}
	}
	next := tc.call("POST", "/admin/service-documents", admin, gin.H{"key": original.DocKey, "title": original.Title, "title_en": original.TitleEN, "body": original.Body + " update", "body_en": original.BodyEN + " update", "effective_at": time.Now().UTC().Add(-time.Second)}, 200)
	tc.call("POST", "/admin/service-documents/"+number(next["id"])+"/publish", admin, gin.H{}, 200)
	var p Payload
	jsonRoundTrip(company["data"], &p)
	p.Name = "更名后重新提交内容审核"
	draft := tc.call("PUT", "/merchant/contents/"+vid+"/draft", merchant, gin.H{"revision_id": rid, "data": p}, 200)
	tc.call("POST", "/merchant/contents/"+vid+"/submit", merchant, gin.H{"revision_id": draft["revision_id"]}, 409)
	tc.call("POST", "/merchant/service-acceptances", merchant, gin.H{"agreements": serviceTestRefs(t, a, "terms", "privacy", "showroom-agreement"), "source": "web"}, 200)
	tc.call("POST", "/merchant/contents/"+vid+"/submit", merchant, gin.H{"revision_id": draft["revision_id"]}, 200)
	tc.call("GET", "/public/vendors/"+vid, "", nil, 200)
	var n int64
	a.DB.Model(&BusinessVerification{}).Where("account_id=?", owner).Count(&n)
	if n != 0 {
		t.Fatal("independent verification unexpectedly created")
	}
}

func serviceTestLegacyEvidence(t *testing.T, a *App, token string) uint64 {
	t.Helper()
	id := serviceTestEvidence(t, a, token, "", "request")
	if e := a.DB.Model(&ServiceEvidence{}).Where("id=?", id).Update("purpose", "verification").Error; e != nil {
		t.Fatal(e)
	}
	return id
}
func jsonRoundTrip(value any, target any) {
	raw, _ := json.Marshal(value)
	_ = json.Unmarshal(raw, target)
}

func TestFreeDisplayResourceGateAndPrivateBackupInventory(t *testing.T) {
	a := importTestApp(t)
	ownerToken := resourceTestSession(t, a, "merchant")
	admin := resourceTestSession(t, a, "admin")
	operator := resourceTestSession(t, a, "operator")
	if e := a.DB.Create(&PlatformOperator{AccountID: serviceTestOwner(t, a, operator), Enabled: true, Permissions: "[]", DisplayName: "无合规权限操作员"}).Error; e != nil {
		t.Fatal(e)
	}
	owner := serviceTestOwner(t, a, ownerToken)
	tc := testClient{t, a.router()}
	vendor := v2Published(t, a, "vendor", 0, 1, 0, 0, false, 0, "历史公开企业")
	if e := a.DB.Model(&vendor).Update("owner_id", owner).Error; e != nil {
		t.Fatal(e)
	}
	resource := Resource{OwnerID: owner, VendorID: vendor.ID, Kind: "video", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if e := a.DB.Create(&resource).Error; e != nil {
		t.Fatal(e)
	}
	revision := ResourceRevision{ResourceID: resource.ID, Payload: "{}", Status: "draft", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if e := a.DB.Create(&revision).Error; e != nil {
		t.Fatal(e)
	}
	a.DB.Model(&resource).Update("draft_revision_id", revision.ID)
	for _, who := range []struct{ prefix, token string }{{"merchant", ownerToken}, {"admin", admin}} {
		tc.call("POST", fmt.Sprintf("/%s/resources/%d/submit", who.prefix, resource.ID), who.token, gin.H{"revision_id": revision.ID}, 409)
	}
	for _, path := range []string{"/verifications", "/service-requests", "/service-documents"} {
		tc.call("GET", "/admin"+path, operator, nil, 403)
	}
	evidence := serviceTestLegacyEvidence(t, a, ownerToken)
	var file ServiceEvidence
	a.DB.First(&file, evidence)
	var objects []StoredObject
	if e := a.DB.Find(&objects).Error; e != nil {
		t.Fatal(e)
	}
	if e := validateMediaInventory(a.DB, objects); e != nil {
		t.Fatal(e)
	}
	filtered := []StoredObject{}
	for _, o := range objects {
		if o.Key != file.ObjectKey {
			filtered = append(filtered, o)
		}
	}
	if e := validateMediaInventory(a.DB, filtered); e == nil || e.Error() != "object_missing_service_evidence" {
		t.Fatal("missing private evidence accepted by backup verifier", e)
	}
	var accepted int64
	a.DB.Model(&ServiceAcceptance{}).Where("account_id=?", owner).Count(&accepted)
	if accepted != 0 {
		t.Fatal("historical enterprise consent was fabricated")
	}
	tc.call("GET", fmt.Sprintf("/public/vendors/%d", vendor.ID), "", nil, 200)
}
func TestFreeDisplayRegistrationConsentAndFixedCapabilities(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	tc.call("POST", "/auth/merchant/register", "", gin.H{"login_name": "free-company", "password": "Free-Service-2026!"}, 409)
	tc.call("POST", "/auth/merchant/register", "", gin.H{"login_name": "free-company", "password": "Free-Service-2026!", "agreements": serviceTestRefs(t, a, "terms", "privacy"), "source": "web"}, 201)
	t.Setenv("PROCUREMENT_ENABLED", "true")
	if procurementEnabled() {
		t.Fatal("legacy switch re-enabled procurement")
	}
	for _, path := range []string{"/public/procurements", "/procurement/projects", "/admin/procurement/reviews"} {
		tc.call("GET", path, "", nil, 503)
	}
	config := tc.call("GET", "/public/config", "", nil, 200)
	if config["service_mode"] != "free_display" || config["uploads"].(map[string]any)["default_quota_bytes"] != float64(200000000) {
		t.Fatal(config)
	}
	site := config["site"].(map[string]any)
	if site["app_number"] != "" || site["police_number"] != "" {
		t.Fatal("invented filing number", site)
	}
}
func TestFreeDisplayRequestsAndPrivateEvidence(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	admin := resourceTestSession(t, a, "admin")
	credential := strings.Repeat("a", 64)
	evidence := serviceTestEvidence(t, a, "", credential, "request")
	if serviceTestReadEvidence(a, evidence, "", strings.Repeat("b", 64)) != 404 || serviceTestReadEvidence(a, evidence, "", credential) != 200 {
		t.Fatal("anonymous evidence credential failure")
	}
	request := gin.H{"kind": "complaint", "subject": "侵权资料投诉（隔离样本）", "description": "请核对图片权利。", "credential": credential, "request_id": strings.Repeat("b", 64), "attachments": []uint64{evidence}}
	first := tc.call("POST", "/public/service-requests", "", request, 201)
	again := tc.call("POST", "/public/service-requests", "", request, 200)
	if first["id"] != again["id"] {
		t.Fatal("duplicate submission created another request")
	}
	if _, ok := first["credential_hash"]; ok {
		t.Fatal("credential hash leaked")
	}
	tc.call("POST", "/public/service-requests/lookup", "", gin.H{"id": first["id"], "credential": strings.Repeat("c", 64)}, 404)
	tc.call("PUT", "/admin/service-requests/"+number(first["id"]), admin, gin.H{"status": "closed", "reply": "", "version": 1}, 400)
	tc.call("PUT", "/admin/service-requests/"+number(first["id"]), admin, gin.H{"status": "needs_info", "reply": "请补充权利证明。", "version": 1}, 200)
	tc.call("POST", "/public/service-requests/"+number(first["id"])+"/supplement", "", gin.H{"credential": credential, "description": "补充说明", "version": 2}, 200)
	tc.call("PUT", "/admin/service-requests/"+number(first["id"]), admin, gin.H{"status": "closed", "reply": "已核查并反馈处理结果。", "version": 3}, 200)
	lookup := tc.call("POST", "/public/service-requests/lookup", "", gin.H{"id": first["id"], "credential": credential}, 200)
	if lookup["reply"] != "已核查并反馈处理结果。" {
		t.Fatal(lookup)
	}
	request["kind"] = "export"
	request["request_id"] = strings.Repeat("d", 64)
	tc.call("POST", "/public/service-requests", "", request, 401)
	merchant := resourceTestSession(t, a, "merchant")
	owner := serviceTestOwner(t, a, merchant)
	vendor := v2Published(t, a, "vendor", 0, 1, 0, 0, false, 0, "申请导出企业")
	a.DB.Model(&vendor).Update("owner_id", owner)
	request["attachments"] = []uint64{}
	request["target_url"] = "/v/another-company"
	owned := tc.call("POST", "/public/service-requests", merchant, request, 201)
	if owned["owner_id"] != float64(owner) || owned["target_url"] != "/v/"+showroomSlug(vendor) {
		t.Fatal("rights request ownership not bound", owned)
	}
}
func TestFreeDisplayDailyRotation(t *testing.T) {
	a := importTestApp(t)
	admin := resourceTestSession(t, a, "admin")
	tc := testClient{t, a.router()}
	now := time.Now().UTC()
	vendors := []Content{}
	for i := 0; i < 26; i++ {
		v := v2Published(t, a, "vendor", 0, 1, 0, uint64(i%3), false, 0, fmt.Sprintf("轮换验收企业%02d", i))
		vendors = append(vendors, v)
		if e := a.DB.Create(&HomeRotationPool{VendorID: v.ID, HallID: 1, State: "approved", Reason: "隔离审核", UpdatedAt: now}).Error; e != nil {
			t.Fatal(e)
		}
	}
	first, e := a.dailyHomeVendors(1, now)
	if e != nil || len(first) != 12 {
		t.Fatal(len(first), e)
	}
	same, e := a.dailyHomeVendors(1, now.Add(time.Minute))
	if e != nil {
		t.Fatal(e)
	}
	for i, v := range first {
		if same[i].ID != v.ID {
			t.Fatal("same-day order changed")
		}
	}
	second, e := a.dailyHomeVendors(1, now.Add(24*time.Hour))
	if e != nil || len(second) != 12 {
		t.Fatal(second, e)
	}
	seen := map[uint64]bool{}
	for _, v := range first {
		seen[v.ID] = true
	}
	for _, v := range second {
		if seen[v.ID] {
			t.Fatal("recent company selected before an undisplayed eligible company")
		}
	}
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			items, e := a.dailyHomeVendors(1, now.Add(48*time.Hour))
			if e == nil && len(items) != 12 {
				e = fmt.Errorf("wrong slot count")
			}
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	a.DB.Model(&Content{}).Where("id=?", first[0].ID).Update("visible", false)
	remaining, e := a.dailyHomeVendors(1, now)
	if e != nil || len(remaining) != 12 {
		t.Fatal(remaining, e)
	}
	for _, v := range remaining {
		if v.ID == first[0].ID {
			t.Fatal("offline company still selected")
		}
	}
	tc.call("PUT", fmt.Sprintf("/admin/home-rotation/%d", remaining[0].ID), admin, gin.H{"state": "removed", "reason": "隔离测试移出"}, 200)
	home := tc.call("GET", "/public/home", "", nil, 200)
	if len(home["vendors"].([]any)) != 12 {
		t.Fatal(home)
	}
	tc.call("GET", "/public/vendors", "", nil, 200)
	if rotationDay(time.Date(2026, 10, 28, 16, 0, 0, 0, time.UTC)) != "2026-10-29" {
		t.Fatal("wrong China day boundary")
	}
	// A fresh process reads the same durable snapshot; a small pool is never padded.
	restarted := &App{DB: a.DB}
	stable, e := restarted.dailyHomeVendors(1, now)
	if e != nil || len(stable) != 12 {
		t.Fatal(stable, e)
	}
	a.DB.Model(&HomeRotationPool{}).Where("vendor_id NOT IN ?", []uint64{vendors[24].ID, vendors[25].ID}).Update("state", "removed")
	small, e := restarted.dailyHomeVendors(1, now)
	if e != nil || len(small) != 2 {
		t.Fatal(small, e)
	}
}
