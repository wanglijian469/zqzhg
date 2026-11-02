package main

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"image"
	"image/png"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAppFlowCreateIdempotencyAndDraftValidation(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	token := resourceTestSession(t, a, "merchant")
	request := gin.H{"kind": "vendor", "client_request_id": "vendor-1", "data": gin.H{"name": " 云草稿 "}}
	first := tc.call("POST", "/merchant/contents", token, request, 201)
	replay := tc.call("POST", "/merchant/contents", token, gin.H{"kind": "vendor", "client_request_id": "vendor-1", "hall_id": 1, "data": gin.H{"name": "云草稿", "media_ids": []uint64{}}}, 200)
	if first["id"] != replay["id"] {
		t.Fatal("retry created another content", first, replay)
	}
	edited := tc.call("PUT", "/merchant/contents/"+number(first["id"])+"/draft", token, gin.H{"revision_id": first["revision_id"], "data": gin.H{"name": "继续编辑"}}, 200)
	replay = tc.call("POST", "/merchant/contents", token, request, 200)
	if replay["revision_id"] != edited["revision_id"] || replay["data"].(map[string]any)["name"] != "继续编辑" {
		t.Fatal("replay must return current private draft", replay)
	}
	conflict := tc.call("POST", "/merchant/contents", token, gin.H{"kind": "vendor", "client_request_id": "vendor-1", "data": gin.H{"name": "冲突名称"}}, 409)
	if conflict["error"].(map[string]any)["code"] != "client_request_conflict" {
		t.Fatal(conflict)
	}
	tc.call("POST", "/merchant/contents", token, gin.H{"kind": "product", "client_request_id": "invalid key", "data": gin.H{"name": "产品"}}, 400)
	for _, data := range []gin.H{{"name": " "}, {"name": strings.Repeat("长", 201)}, {"name": "媒体", "media_ids": []uint64{999999}}, {"name": "分类", "product_category_id": 999999}} {
		tc.call("POST", "/merchant/contents", token, gin.H{"kind": "product", "data": data}, 400)
	}
	p := tc.call("POST", "/merchant/contents", token, gin.H{"kind": "product", "data": gin.H{"name": "无图草稿"}}, 201)
	tc.call("POST", "/merchant/service-acceptances", token, gin.H{"agreements": serviceTestRefs(t, a, "terms", "privacy", "showroom-agreement")}, 200)
	tc.call("POST", "/merchant/contents/"+number(edited["id"])+"/submit", token, gin.H{"revision_id": edited["revision_id"]}, 400)
	result := tc.call("POST", "/merchant/contents/"+number(p["id"])+"/submit", token, gin.H{"revision_id": p["revision_id"]}, 400)
	if result["error"].(map[string]any)["fields"].([]any)[0].(map[string]any)["field"] != "media_ids" {
		t.Fatal(result)
	}
	tc.call("POST", "/merchant/contents", resourceTestSession(t, a, "merchant"), request, 201)
}

func TestAppFlowConcurrentCreateRetries(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	token := resourceTestSession(t, a, "merchant")
	tc.call("POST", "/merchant/contents", token, gin.H{"kind": "vendor", "data": gin.H{"name": "并发企业"}}, 201)
	const workers = 8
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan *httptest.ResponseRecorder, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			raw, _ := json.Marshal(gin.H{"kind": "product", "client_request_id": "concurrent-product", "data": gin.H{"name": "同一产品"}})
			req := httptest.NewRequest("POST", "/api/v1/merchant/contents", bytes.NewReader(raw))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			tc.router.ServeHTTP(w, req)
			results <- w
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	var contentID float64
	created := 0
	for w := range results {
		if w.Code != 200 && w.Code != 201 {
			t.Fatal(w.Code, w.Body.String())
		}
		if w.Code == 201 {
			created++
		}
		var body map[string]any
		json.Unmarshal(w.Body.Bytes(), &body)
		if contentID != 0 && contentID != body["id"].(float64) {
			t.Fatal("duplicate", body)
		}
		contentID = body["id"].(float64)
	}
	if created != 1 {
		t.Fatal("expected one creation", created)
	}
	var count int64
	a.DB.Model(&Content{}).Where("kind='product'").Count(&count)
	if count != 1 {
		t.Fatal(count)
	}
}

func TestAppFlowMissingAgreementRefsKeepAudit(t *testing.T) {
	a := importTestApp(t)
	token := resourceTestSession(t, a, "merchant")
	owner := serviceTestOwner(t, a, token)
	refs := serviceTestRefs(t, a, "terms", "privacy", "showroom-agreement")
	if err := a.DB.Transaction(func(tx *gorm.DB) error {
		return acceptServiceDocuments(tx, owner, refs[:2], "ios", []string{refs[0].Key, refs[1].Key})
	}); err != nil {
		t.Fatal(err)
	}
	var before []ServiceAcceptance
	a.DB.Where("account_id=?", owner).Order("id").Find(&before)
	tc := testClient{t, a.router()}
	tc.call("POST", "/merchant/service-acceptances", token, gin.H{"agreements": refs[2:], "source": "android"}, 200)
	tc.call("POST", "/merchant/service-acceptances", token, gin.H{"agreements": []AgreementRef{}}, 200)
	var after []ServiceAcceptance
	a.DB.Where("account_id=?", owner).Order("id").Find(&after)
	if len(after) != 3 {
		t.Fatal(after)
	}
	for i := range before {
		if !before[i].AcceptedAt.Equal(after[i].AcceptedAt) || before[i].Source != after[i].Source {
			t.Fatal("audit rewritten", before, after)
		}
	}
	d := ServiceDocument{DocKey: "terms", Version: 99, Title: "new", Body: "changed", Digest: strings.Repeat("a", 64), Status: "published", EffectiveAt: time.Now().Add(-time.Minute), CreatedAt: time.Now()}
	if err := a.DB.Create(&d).Error; err != nil {
		t.Fatal(err)
	}
	tc.call("POST", "/merchant/service-acceptances", token, gin.H{"agreements": []AgreementRef{}}, 409)
}

func TestAppFlowSchema30PreservesLegacyPublicationAndPendingApproval(t *testing.T) {
	a := importTestAppAtSchema(t, 29)
	token := resourceTestSession(t, a, "merchant")
	admin := resourceTestSession(t, a, "admin")
	owner := serviceTestOwner(t, a, token)
	vendor := Content{OwnerID: owner, Kind: "vendor", HallID: 1}
	if e := a.DB.Create(&vendor).Error; e != nil {
		t.Fatal(e)
	}
	product := Content{OwnerID: owner, VendorID: vendor.ID, Kind: "product", HallID: 1}
	if e := a.DB.Create(&product).Error; e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(Payload{Name: "旧版待审核无图产品"})
	rev := Revision{ContentID: product.ID, Status: "pending", Name: "旧版待审核无图产品", Payload: string(raw)}
	if e := a.DB.Create(&rev).Error; e != nil {
		t.Fatal(e)
	}
	a.DB.Model(&product).Update("draft_revision_id", rev.ID)
	published := Content{OwnerID: owner, VendorID: vendor.ID, Kind: "product", HallID: 1, Visible: true, PublishedRevisionID: rev.ID, Epoch: 7}
	a.DB.Create(&published)
	if e := migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	if e := migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	if !a.DB.Migrator().HasTable("content_create_requests") {
		t.Fatal("schema30 mapping absent")
	}
	var saved Content
	a.DB.First(&saved, published.ID)
	if !saved.Visible || saved.PublishedRevisionID != published.PublishedRevisionID || saved.Epoch != 7 {
		t.Fatal("migration changed published content", saved)
	}
	tc := testClient{t, a.router()}
	tc.call("POST", "/admin/revisions/"+number(float64(rev.ID))+"/review", admin, gin.H{"decision": "approve"}, 200)
	saved = Content{}
	a.DB.First(&saved, product.ID)
	if !saved.Visible || saved.PublishedRevisionID != rev.ID {
		t.Fatal("legacy pending approval blocked", saved)
	}
}

func TestAppFlowCreateFailureRollsBackKeyAndLegacyWithoutKey(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	token := resourceTestSession(t, a, "merchant")
	tc.call("POST", "/merchant/contents", token, gin.H{"kind": "vendor", "client_request_id": "retry-after-invalid", "data": gin.H{"name": " "}}, 400)
	tc.call("POST", "/merchant/contents", token, gin.H{"kind": "vendor", "client_request_id": "retry-after-invalid", "data": gin.H{"name": "修复草稿"}}, 201)
	request := gin.H{"kind": "product", "data": gin.H{"name": "无键产品"}}
	first := tc.call("POST", "/merchant/contents", token, request, 201)
	second := tc.call("POST", "/merchant/contents", token, request, 201)
	if first["id"] == second["id"] {
		t.Fatal("legacy requests unexpectedly deduplicated")
	}
	request["client_request_id"] = "case-key"
	one := tc.call("POST", "/merchant/contents", token, request, 201)
	request["client_request_id"] = "CASE-KEY"
	two := tc.call("POST", "/merchant/contents", token, request, 201)
	if one["id"] == two["id"] {
		t.Fatal("case sensitive keys collided")
	}
	cfg := tc.call("GET", "/public/config", "", nil, 200)
	if cfg["capabilities"].(map[string]any)["content_create_idempotency"] != true {
		t.Fatal("capability missing", cfg)
	}
}

func TestAppFlowProductSubmissionNeedsImageAndLeafCategory(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	token := resourceTestSession(t, a, "merchant")
	tc.call("POST", "/merchant/contents", token, gin.H{"kind": "vendor", "data": gin.H{"name": "图片提交流程企业"}}, 201)
	tc.call("POST", "/merchant/service-acceptances", token, gin.H{"agreements": serviceTestRefs(t, a, "terms", "privacy", "showroom-agreement")}, 200)
	var buf bytes.Buffer
	if e := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 8))); e != nil {
		t.Fatal(e)
	}
	media := resourceUpload(t, a, token, "/merchant/media", "valid.png", buf.Bytes(), 201)
	product := tc.call("POST", "/merchant/contents", token, gin.H{"kind": "product", "data": gin.H{"name": "图片但缺分类", "media_ids": []any{media["id"]}}}, 201)
	missing := tc.call("POST", "/merchant/contents/"+number(product["id"])+"/submit", token, gin.H{"revision_id": product["revision_id"]}, 400)
	if missing["error"].(map[string]any)["code"] != "invalid_category" {
		t.Fatal(missing)
	}
	leaf := tc.fixtureCall("PUT", "/merchant/contents/"+number(product["id"])+"/draft", token, gin.H{"revision_id": product["revision_id"], "data": gin.H{"name": "完整产品无视频", "media_ids": []any{media["id"]}}}, 200)
	tc.call("POST", "/merchant/contents/"+number(product["id"])+"/submit", token, gin.H{"revision_id": leaf["revision_id"]}, 200)
	tc.call("PUT", "/merchant/contents/"+number(product["id"])+"/draft", token, gin.H{"revision_id": leaf["revision_id"], "data": gin.H{"name": "审核中修改"}}, 409)
}

func appFlowProductImage(t *testing.T, a *App, token string) uint64 {
	t.Helper()
	var buf bytes.Buffer
	if e := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 8))); e != nil {
		t.Fatal(e)
	}
	media := resourceUpload(t, a, token, "/merchant/media", "product.png", buf.Bytes(), 201)
	return uint64(media["id"].(float64))
}
