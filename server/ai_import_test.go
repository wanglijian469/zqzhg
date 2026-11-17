package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestAIRegionAndSourceValidation(t *testing.T) {
	good := AIRegion{SourcePage: 1, ImageType: "dimensions", Box: []float64{.1, .2, .7, .8}}
	if !aiRegionValid(good, 2) {
		t.Fatal("valid crop rejected")
	}
	good.Box[3] = 1.1
	if aiRegionValid(good, 2) {
		t.Fatal("out of bounds accepted")
	}
	x := AIProductExtraction{SourcePages: []int{1}, Models: []AIProductModel{{ModelName: "A-01", Parameters: []AIParameter{{ParameterName: "压力", RawValue: "10~12", RawUnit: "MPa", SourcePage: 2}}}}}
	if aiValidateExtraction(x, 3, []int{1}) == nil {
		t.Fatal("foreign page accepted")
	}
	old := Payload{Name: "old", Phone: "123", Address: "keep"}
	next, e := aiMergeCompany(old, Payload{Name: "new"}, []string{"name"})
	if e != nil || next.Name != "new" || next.Phone != "123" || next.Address != "keep" {
		t.Fatal("unselected company fields overwritten")
	}
}
func aiTestTask(t *testing.T, a *App, owner uint64) AIImportTask {
	t.Helper()
	r := AIImportTask{OwnerID: owner, RequestID: randomToken(), FileHash: randomToken(), FileName: "fixture.png", SourceIDs: "[]", Status: "review_pending", Stage: "review_pending", Total: 1, Checkpoint: "{}", PublishManifest: "[]"}
	if e := a.DB.Create(&r).Error; e != nil {
		t.Fatal(e)
	}
	return r
}
func aiTestPNG(t *testing.T) []byte {
	t.Helper()
	im := image.NewRGBA(image.Rect(0, 0, 240, 160))
	for y := 0; y < 160; y++ {
		for x := 0; x < 240; x++ {
			im.Set(x, y, color.RGBA{uint8(x), uint8(y), 33, 255})
		}
	}
	var b bytes.Buffer
	if e := png.Encode(&b, im); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestAIIsolationCropIntegrityReviewAndSubmission(t *testing.T) {
	t.Setenv("AI_IMPORT_ENABLED", "true")
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	merchant := tc.login("13900009181")
	other := tc.login("13900009182")
	admin := resourceTestSession(t, a, "admin")
	var who Account
	tokenAccount := func(token string) Account {
		var s Session
		a.DB.First(&s, "token_hash=?", digest(token))
		var ac Account
		a.DB.First(&ac, s.AccountID)
		return ac
	}
	who = tokenAccount(merchant)
	vendor := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "AI隔离验收企业", RegionName: "河北", MainProducts: "机械", Phone: "13900009181"}}, 201)
	tc.call("POST", "/merchant/service-acceptances", merchant, gin.H{"agreements": serviceTestRefs(t, a, "terms", "privacy", "showroom-agreement"), "source": "web"}, 200)
	tc.call("POST", "/merchant/contents/"+number(vendor["id"])+"/submit", merchant, gin.H{"revision_id": vendor["revision_id"]}, 200)
	task := aiTestTask(t, a, who.ID)
	task.VendorID = uint64(vendor["id"].(float64))
	a.DB.Save(&task)
	tc.call("GET", fmt.Sprintf("/merchant/ai-import/tasks/%d/result", task.ID), other, nil, 404)
	tc.call("GET", fmt.Sprintf("/merchant/ai-import/tasks/%d", task.ID), "", nil, 401)
	page, e := a.aiSaveAsset(context.Background(), who.ID, task.ID, 0, 1, "page", ".png", aiTestPNG(t), nil)
	if e != nil {
		t.Fatal(e)
	}
	crop, e := a.aiCrop(context.Background(), task, page, AIRegion{SourcePage: 1, ImageType: "dimensions", Box: []float64{.1, .1, .9, .9}})
	if e != nil {
		t.Fatal(e)
	}
	data, e := a.aiAssetBytes(context.Background(), crop)
	if e != nil {
		t.Fatal(e)
	}
	im, _, e := image.Decode(bytes.NewReader(data))
	if e != nil || im.Bounds().Dx() != 192 || im.Bounds().Dy() != 128 {
		t.Fatal("crop changed pixels or bounds", e)
	}
	r, g, _, _ := im.At(0, 0).RGBA()
	if r>>8 != 24 || g>>8 != 16 {
		t.Fatal("diagram pixels were modified")
	}
	tc.call("GET", fmt.Sprintf("/merchant/ai-import/assets/%d", crop.ID), other, nil, 404)
	tc.call("DELETE", fmt.Sprintf("/admin/media/%d", crop.MediaID), admin, gin.H{"confirm": true}, 409)
	if e = validateAIPayloadAssets(a.DB, Payload{MediaIDs: []uint64{page.MediaID}}); e == nil {
		t.Fatal("whole page accepted")
	}
	photo, e := a.aiCrop(context.Background(), task, page, AIRegion{SourcePage: 1, ImageType: "product_main", Box: []float64{.05, .1, .8, .8}})
	if e != nil {
		t.Fatal(e)
	}
	otherTask := aiTestTask(t, a, who.ID)
	if a.aiValidateTaskMedia(a.DB, otherTask.ID, Payload{MediaIDs: []uint64{photo.MediaID}}) == nil {
		t.Fatal("another task's AI asset was accepted")
	}
	company := AIImportDraft{TaskID: task.ID, OwnerID: who.ID, TempID: "company", Version: 1, BaseRevisionID: uint64(vendor["revision_id"].(float64)), Data: aiJSON(Payload{Name: "AI隔离验收企业"}), SourcePages: "[1]", FieldSources: "{}", ReviewIssues: "[]", SelectedFields: "[]"}
	if e = a.DB.Table(aiTable("company")).Create(&company).Error; e != nil {
		t.Fatal(e)
	}
	p := Payload{Name: "真实参数保护测试产品", MediaIDs: []uint64{photo.MediaID}, ProductDetails: &ProductDetails{SeriesName: "测试系列", Models: []AIProductModel{{ModelName: "X-1", Parameters: []AIParameter{{ParameterName: "压力", RawValue: "10~12", RawUnit: "MPa", SourcePage: 1, ReviewRequired: true}}}}, Images: []AIProductImage{{MediaID: crop.MediaID, ImageType: "dimensions", SourcePage: 1}}, SourcePages: []int{1}}}
	var cat Category
	a.DB.Where("category_kind='product' AND enabled=1 AND deleted_at IS NULL AND id NOT IN (SELECT DISTINCT parent_id FROM categories WHERE enabled=1)").Order("id").First(&cat)
	p.ProductCategoryID = cat.ID
	p.CategoryID = cat.ID
	product := AIImportDraft{TaskID: task.ID, OwnerID: who.ID, TempID: "product-1", Version: 1, Data: aiJSON(p), SourcePages: "[1]", FieldSources: "{}", ReviewIssues: "[\"核对原参数\"]", SelectedFields: "[]"}
	a.DB.Table(aiTable("product")).Create(&product)
	publish := gin.H{"idempotency_key": "fixture-key", "company_version": 1, "products": []gin.H{{"id": product.ID, "version": 1}}}
	path := fmt.Sprintf("/merchant/ai-import/tasks/%d", task.ID)
	tc.call("POST", path+"/publish", merchant, publish, 409)
	review := gin.H{"items": []gin.H{{"kind": "product", "id": product.ID, "version": 1}}}
	tc.call("POST", path+"/review", merchant, review, 400)
	review = gin.H{"items": []gin.H{{"kind": "company", "id": company.ID, "version": 1}, {"kind": "product", "id": product.ID, "version": 1, "resolution": "逐项比对原图，保留10~12 MPa原值"}}}
	tc.call("POST", path+"/review", merchant, review, 200)
	// A reviewed but incomplete item must fail before freezing a submission batch.
	incomplete := p
	incomplete.MediaIDs = nil
	a.DB.Table(aiTable("product")).Where("id=?", product.ID).Update("data", aiJSON(incomplete))
	tc.call("POST", path+"/publish", merchant, publish, 400)
	var unlocked AIImportTask
	a.DB.First(&unlocked, task.ID)
	if unlocked.PublishKey != "" {
		t.Fatal("invalid publish locked an editable task")
	}
	a.DB.Table(aiTable("product")).Where("id=?", product.ID).Update("data", aiJSON(aiReviewedPayload(p)))
	tc.call("PUT", fmt.Sprintf("/merchant/ai-import/products/%d", product.ID), other, gin.H{"version": 1, "data": p}, 404)
	out := tc.call("POST", path+"/publish", merchant, publish, 200)
	if out["failed"] != float64(0) {
		t.Fatal(out)
	}
	again := tc.call("POST", path+"/publish", merchant, publish, 200)
	if again["submitted"] != float64(1) {
		t.Fatal(again)
	}
	a.DB.Table(aiTable("product")).First(&product, product.ID)
	var ct Content
	var rev Revision
	a.DB.First(&ct, product.ContentID)
	a.DB.First(&rev, ct.DraftRevisionID)
	if ct.Visible || rev.Status != "pending" {
		t.Fatal("AI bypassed platform review")
	}
	var count int64
	a.DB.Model(&Content{}).Where("owner_id=? AND kind='product'", who.ID).Count(&count)
	if count != 1 {
		t.Fatal("duplicate product")
	}
	tc.call("GET", fmt.Sprintf("/public/products/%d", ct.ID), "", nil, 404)
	tc.call("POST", "/admin/revisions/"+number(vendor["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	tc.call("POST", fmt.Sprintf("/admin/revisions/%d/review", rev.ID), admin, gin.H{"decision": "approve"}, 200)
	tc.call("GET", fmt.Sprintf("/public/products/%d", ct.ID), "", nil, 200)
	// Old clients omit the new field; a later draft must retain structured models.
	tc.call("PUT", fmt.Sprintf("/merchant/contents/%d/draft", ct.ID), merchant, map[string]any{"revision_id": ct.DraftRevisionID, "data": map[string]any{"name": p.Name, "media_ids": p.MediaIDs}}, 200)
	a.DB.First(&ct, ct.ID)
	a.DB.First(&rev, ct.DraftRevisionID)
	var retained Payload
	json.Unmarshal([]byte(rev.Payload), &retained)
	if retained.ProductDetails == nil || retained.ProductDetails.Models[0].Parameters[0].RawValue != "10~12" {
		t.Fatal("legacy edit lost model detail")
	}
}
func TestAIMigrationResumeAndQueueLease(t *testing.T) {
	t.Setenv("AI_IMPORT_ENABLED", "true")
	a := importTestAppAtSchema(t, 31)
	if e := migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	if e := migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	var admin Account
	a.DB.Where("role='admin'").First(&admin)
	task := aiTestTask(t, a, admin.ID)
	task.Status = "queued"
	a.DB.Save(&task)
	claimed, e := a.aiClaim(context.Background())
	if e != nil || claimed.ID != task.ID || claimed.Lease == "" {
		t.Fatal(e)
	}
	if _, e = a.aiClaim(context.Background()); e != gorm.ErrRecordNotFound {
		t.Fatal("duplicate task claim", e)
	}
	a.DB.Model(&AIImportTask{}).Where("id=?", task.ID).Update("locked_until", time.Now().UTC().Add(-time.Minute))
	next, e := a.aiClaim(context.Background())
	if e != nil || next.Lease == claimed.Lease {
		t.Fatal("lease not recovered", e)
	}
	checkpoint := aiImportCheckpoint{Models: map[string]aiModelCheckpoint{
		"product-1-detail-0": {Fingerprint: "fixture", Response: json.RawMessage(`{}`)},
		"product-1-review-0": {Fingerprint: "fixture", Response: json.RawMessage(`{}`)},
		"product-2-detail-0": {Fingerprint: "other", Response: json.RawMessage(`{}`)},
	}}
	a.DB.Model(&AIImportTask{}).Where("id=?", next.ID).Update("checkpoint", aiJSON(checkpoint))
	a.DB.Create(&AIImportModelCall{TaskID: next.ID, Stage: "product-1-review-0", Fingerprint: strings.Repeat("a", 64), UsageJSON: "null", PriceBasis: "unknown", ResponseJSON: `{"review_issues":[]}`})
	a.aiInvalidateFailedResponse(next, errors.New("ai_invalid_crop"))
	remainingCache := a.aiReadCheckpoint(next.ID)
	if len(remainingCache.Models) != 1 || remainingCache.Models["product-2-detail-0"].Fingerprint != "other" {
		t.Fatal("semantic retry retained a poisoned checkpoint or removed unrelated stages")
	}
	for _, file := range []string{"../deploy/linux/rollback-ai-import-034.sql", "../deploy/linux/rollback-ai-import-033.sql", "../deploy/linux/rollback-ai-import-032.sql"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, statement := range strings.Split(string(data), ";") {
			if strings.TrimSpace(statement) != "" {
				if err = a.DB.Exec(statement).Error; err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if a.DB.Migrator().HasTable(&AIImportTask{}) {
		t.Fatal("rollback retained AI tables")
	}
	if !a.DB.Migrator().HasTable(&Content{}) {
		t.Fatal("rollback touched business content")
	}
	if e = migrate(a.DB); e != nil {
		t.Fatal(e)
	}
}

func TestAIUploadContentValidationAndDuplicate(t *testing.T) {
	t.Setenv("AI_IMPORT_ENABLED", "true")
	t.Setenv("DASHSCOPE_API_KEY", "synthetic-no-call")
	t.Setenv("DASHSCOPE_BASE_URL", "https://example.test/compatible-mode/v1")
	a := importTestApp(t)
	router := a.router()
	tc := testClient{t, router}
	merchant := tc.login("13900009212")
	upload := func(name string, data []byte, want int) map[string]any {
		var body bytes.Buffer
		w := multipart.NewWriter(&body)
		part, _ := w.CreateFormFile("files", name)
		part.Write(data)
		w.WriteField("request_id", randomToken())
		w.Close()
		r := httptest.NewRequest("POST", "/api/v1/merchant/ai-import/tasks", &body)
		r.Header.Set("Content-Type", w.FormDataContentType())
		r.Header.Set("Authorization", "Bearer "+merchant)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, r)
		if rec.Code != want {
			t.Fatalf("upload %s: %d %s", name, rec.Code, rec.Body.String())
		}
		var out map[string]any
		json.Unmarshal(rec.Body.Bytes(), &out)
		return out
	}
	upload("fake.pdf", []byte("not a pdf"), 400)
	data := aiTestPNG(t)
	upload("wrong.jpg", data, 400)
	upload("truncated.png", data[:40], 400)
	t.Setenv("AI_IMPORT_MAX_FILE_BYTES", "100")
	upload("oversize.png", data, 400)
	t.Setenv("AI_IMPORT_MAX_FILE_BYTES", "30000000")
	upload("valid.png", data, 202)
	if upload("same.png", data, 200)["duplicate"] != true {
		t.Fatal("same file not detected")
	}
}
