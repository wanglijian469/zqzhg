package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestAIBatchesIndependentCompanyThreeBatchesAndIsolation(t *testing.T) {
	t.Setenv("AI_IMPORT_ENABLED", "true")
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	merchant := tc.login("13900009401")
	other := tc.login("13900009402")
	admin := resourceTestSession(t, a, "admin")
	var session Session
	a.DB.First(&session, "token_hash=?", digest(merchant))
	var who Account
	a.DB.First(&who, session.AccountID)
	task := aiTestTask(t, a, who.ID)
	company := AIImportDraft{TaskID: task.ID, OwnerID: who.ID, TempID: "company", Version: 1, Data: aiJSON(Payload{Name: "独立批次测试企业", RegionName: "河北", Phone: "13900009401", MainProducts: "测试机械"}), SourcePages: "[1]", FieldSources: "{}", ReviewIssues: "[]", SelectedFields: "[]"}
	var category Category
	a.DB.Where("category_kind='vendor' AND enabled=1 AND deleted_at IS NULL AND id NOT IN (SELECT DISTINCT parent_id FROM categories WHERE enabled=1)").First(&category)
	var cp Payload
	json.Unmarshal([]byte(company.Data), &cp)
	cp.PrimaryCategoryID = category.ID
	company.Data = aiJSON(cp)
	a.DB.Table(aiTable("company")).Create(&company)
	page, e := a.aiSaveAsset(context.Background(), who.ID, task.ID, 0, 1, "page", ".png", aiTestPNG(t), nil)
	if e != nil {
		t.Fatal(e)
	}
	photo, e := a.aiCrop(context.Background(), task, page, AIRegion{SourcePage: 1, ImageType: "product_main", Box: []float64{.1, .1, .8, .8}})
	if e != nil {
		t.Fatal(e)
	}
	category = Category{}
	a.DB.Where("category_kind='product' AND enabled=1 AND deleted_at IS NULL AND id NOT IN (SELECT DISTINCT parent_id FROM categories WHERE enabled=1)").First(&category)
	p := Payload{Name: "批次测试产品", CategoryID: category.ID, ProductCategoryID: category.ID, MediaIDs: []uint64{photo.MediaID}, ProductDetails: &ProductDetails{SourcePages: []int{1}, Models: []AIProductModel{{ModelName: "TEST-1", Parameters: []AIParameter{{ParameterName: "压力", RawValue: "10~12", SourcePage: 1, ReviewRequired: true}}}}}}
	drafts := []AIImportDraft{}
	for i := 0; i < 4; i++ {
		row := AIImportDraft{TaskID: task.ID, OwnerID: who.ID, TempID: fmt.Sprint("product-", i), Version: 1, Data: aiJSON(p), SourcePages: "[1]", FieldSources: "{}", ReviewIssues: `["请对照原始参数"]`, SelectedFields: "[]"}
		a.DB.Table(aiTable("product")).Create(&row)
		drafts = append(drafts, row)
	}
	prefix := fmt.Sprintf("/merchant/ai-import/tasks/%d", task.ID)
	refs := serviceTestRefs(t, a, "terms", "privacy", "showroom-agreement")
	request := func(key string, rows []AIImportDraft) gin.H {
		items := []gin.H{}
		for _, row := range rows {
			items = append(items, gin.H{"id": row.ID, "version": row.Version, "resolution": "已对照合成测试参数核对"})
		}
		return gin.H{"idempotency_key": key, "items": items, "agreements": refs}
	}
	tc.call("POST", prefix+"/products/submit", merchant, request("before-company", drafts[:1]), 409)
	companyResult := tc.call("POST", prefix+"/company/submit", merchant, gin.H{"idempotency_key": "company-only", "items": []gin.H{{"id": company.ID, "version": 1}}, "agreements": refs}, 200)
	if companyResult["submitted"] != float64(1) {
		t.Fatal(companyResult)
	}
	var counts int64
	a.DB.Model(&Content{}).Where("kind='product' AND owner_id=?", who.ID).Count(&counts)
	if counts != 0 {
		t.Fatal("company submission created products")
	}
	a.DB.First(&task, task.ID)
	if task.VendorID == 0 {
		t.Fatal("no company id")
	}
	var vendor Content
	a.DB.First(&vendor, task.VendorID)
	var vendorRev Revision
	a.DB.First(&vendorRev, vendor.DraftRevisionID)
	if vendorRev.Status != "pending" {
		t.Fatal(vendorRev.Status)
	}
	tc.call("POST", prefix+"/products/submit", other, request("cross-company", drafts[:1]), 404)
	// No separately reviewed version is required: the button itself records confirmation.
	first := request("batch-one", drafts[:2])
	bad := p
	bad.MediaIDs = nil
	first["items"].([]gin.H)[1]["data"] = bad
	out := tc.call("POST", prefix+"/products/submit", merchant, first, 200)
	if out["submitted"] != float64(1) || out["failed"] != float64(1) {
		t.Fatal(out)
	}
	var failed AIImportDraft
	a.DB.Table(aiTable("product")).First(&failed, drafts[1].ID)
	if failed.ContentID != 0 || failed.ActiveBatchID != 0 || failed.ReviewedVersion != 0 {
		t.Fatal("failed item not editable or was falsely confirmed", failed)
	}
	tc.call("PUT", fmt.Sprintf("/merchant/ai-import/products/%d", failed.ID), merchant, gin.H{"version": failed.Version, "data": p}, 200)
	a.DB.Table(aiTable("product")).First(&failed, failed.ID)
	// Same idempotency key cannot be used for different edits.
	tc.call("POST", prefix+"/products/submit", merchant, request("batch-one", []AIImportDraft{failed}), 409)
	replay := tc.call("POST", prefix+"/products/submit", merchant, first, 200)
	if replay["submitted"] != float64(1) {
		t.Fatal(replay)
	}
	second := request("batch-two", []AIImportDraft{failed, drafts[2]})
	out = tc.call("POST", prefix+"/products/submit", merchant, second, 200)
	if out["submitted"] != float64(2) || out["failed"] != float64(0) {
		t.Fatal(out)
	}
	tc.call("PUT", fmt.Sprintf("/merchant/ai-import/products/%d", drafts[3].ID), merchant, gin.H{"version": 1, "data": p}, 200)
	a.DB.Table(aiTable("product")).First(&drafts[3], drafts[3].ID)
	third := request("batch-three", drafts[3:])
	// Simulate a process stop immediately after reserving the final batch, before creating business content.
	var reservedRequest aiBatchRequest
	json.Unmarshal([]byte(aiJSON(third)), &reservedRequest)
	reserved := AISubmissionBatch{TaskID: task.ID, OwnerID: who.ID, Kind: "product", IdempotencyKey: reservedRequest.Key, Fingerprint: digestText("product" + aiJSON(reservedRequest.Items)), RequestJSON: aiJSON(reservedRequest), Status: "pending"}
	if e = a.DB.Create(&reserved).Error; e != nil {
		t.Fatal(e)
	}
	reservedItem := AISubmissionItem{TaskID: task.ID, OwnerID: who.ID, BatchID: reserved.ID, Kind: "product", DraftID: drafts[3].ID, Version: drafts[3].Version, RequestJSON: aiJSON(reservedRequest.Items[0]), Status: "pending", FieldsJSON: "[]"}
	if e = a.DB.Create(&reservedItem).Error; e != nil {
		t.Fatal(e)
	}
	a.DB.Table(aiTable("product")).Where("id=?", drafts[3].ID).Update("active_batch_id", reserved.ID)
	retryPath := fmt.Sprintf("%s/batches/%d/retry", prefix, reserved.ID)
	tc.call("POST", retryPath, other, gin.H{}, 404)
	out = tc.call("POST", retryPath, merchant, gin.H{}, 200)
	if out["submitted"] != float64(1) {
		t.Fatal(out)
	}
	tc.call("POST", prefix+"/products/submit", merchant, third, 200)
	a.DB.Model(&Content{}).Where("kind='product' AND owner_id=?", who.ID).Count(&counts)
	if counts != 4 {
		t.Fatal("duplicate or missing products", counts)
	}
	a.DB.First(&task, task.ID)
	if task.Status != "completed" {
		t.Fatal(task.Status)
	}
	var current AIImportDraft
	a.DB.Table(aiTable("product")).First(&current, drafts[0].ID)
	tc.call("GET", fmt.Sprintf("/public/products/%d", current.ContentID), "", nil, 404)
	tc.call("POST", fmt.Sprintf("/admin/revisions/%d/review", current.RevisionID), admin, gin.H{"decision": "approve"}, 200)
	tc.call("GET", fmt.Sprintf("/public/products/%d", current.ContentID), "", nil, 404) // New company is still pending.
	tc.call("POST", fmt.Sprintf("/admin/revisions/%d/review", vendorRev.ID), admin, gin.H{"decision": "approve"}, 200)
	tc.call("GET", fmt.Sprintf("/public/products/%d", current.ContentID), "", nil, 200)
	// Concurrent identical requests return the same committed result.
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := strings.NewReader(aiJSON(third))
			r := httptest.NewRequest("POST", "/api/v1"+prefix+"/products/submit", body)
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Authorization", "Bearer "+merchant)
			w := httptest.NewRecorder()
			tc.router.ServeHTTP(w, r)
			if w.Code != 200 {
				t.Errorf("concurrent replay: %d", w.Code)
			}
		}()
	}
	wg.Wait()
}

func TestAIExistingCompanyProductsDoNotRequireCompanySuggestionReview(t *testing.T) {
	t.Setenv("AI_IMPORT_ENABLED", "true")
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	merchant := tc.login("13900009403")
	var session Session
	a.DB.First(&session, "token_hash=?", digest(merchant))
	var who Account
	a.DB.First(&who, session.AccountID)
	vendor := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "现有企业保留测试", RegionName: "河北", Phone: "13900009403", MainProducts: "机械"}}, 201)
	task := aiTestTask(t, a, who.ID)
	task.VendorID = uint64(vendor["id"].(float64))
	a.DB.Save(&task)
	company := AIImportDraft{TaskID: task.ID, OwnerID: who.ID, TempID: "company", Version: 1, Data: aiJSON(Payload{Name: "不能自动覆盖的AI建议"}), SourcePages: "[1]", FieldSources: "{}", ReviewIssues: `["未知企业资料"]`, SelectedFields: "[]", BaseRevisionID: 1}
	a.DB.Table(aiTable("company")).Create(&company)
	page, _ := a.aiSaveAsset(context.Background(), who.ID, task.ID, 0, 1, "page", ".png", aiTestPNG(t), nil)
	photo, _ := a.aiCrop(context.Background(), task, page, AIRegion{SourcePage: 1, ImageType: "product_main", Box: []float64{.1, .1, .8, .8}})
	var cat Category
	a.DB.Where("category_kind='product' AND enabled=1 AND deleted_at IS NULL AND id NOT IN (SELECT DISTINCT parent_id FROM categories WHERE enabled=1)").First(&cat)
	p := Payload{Name: "独立提交测试产品", CategoryID: cat.ID, ProductCategoryID: cat.ID, MediaIDs: []uint64{photo.MediaID}}
	draft := AIImportDraft{TaskID: task.ID, OwnerID: who.ID, TempID: "p1", Version: 1, Data: aiJSON(p), SourcePages: "[1]", FieldSources: "{}", ReviewIssues: "[]", SelectedFields: "[]"}
	a.DB.Table(aiTable("product")).Create(&draft)
	out := tc.call("POST", fmt.Sprintf("/merchant/ai-import/tasks/%d/products/submit", task.ID), merchant, gin.H{"idempotency_key": "without-company-review", "items": []gin.H{{"id": draft.ID, "version": 1}}, "agreements": serviceTestRefs(t, a, "terms", "privacy", "showroom-agreement")}, 200)
	if out["submitted"] != float64(1) {
		t.Fatal(out)
	}
	a.DB.Table(aiTable("company")).First(&company, company.ID)
	if company.ReviewedVersion != 0 || company.ContentID != 0 {
		t.Fatal("company suggestion implicitly confirmed")
	}
	existing := tc.call("GET", fmt.Sprintf("/merchant/contents/%v", vendor["id"]), merchant, nil, 200)
	if existing["data"].(map[string]any)["name"] != "现有企业保留测试" {
		t.Fatal("company overwritten")
	}
	out = tc.call("POST", fmt.Sprintf("/merchant/ai-import/tasks/%d/company/submit", task.ID), merchant, gin.H{"idempotency_key": "keep-existing", "items": []gin.H{{"id": company.ID, "version": 1, "selected_fields": []string{}}}}, 200)
	if out["submitted"] != float64(1) {
		t.Fatal("unused stale suggestion prevented retaining company", out)
	}
}
