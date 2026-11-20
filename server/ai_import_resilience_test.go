package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAIStandardPresentationKeepsTechnicalEvidence(t *testing.T) {
	x := AIProductExtraction{ProductName: "设备", Description: "产品介绍\n**画册来源**：PDF第2页", SourcePages: []int{2}, Models: []AIProductModel{{ModelName: "X-01", Parameters: []AIParameter{{ParameterName: "压力", RawValue: "1.2300e2", RawUnit: "MPa", SourcePage: 2}}}}}
	p := aiProductPayload(x)
	if strings.Contains(p.Parameters, "PDF") || strings.Contains(p.Description, "画册来源") || !strings.Contains(p.Parameters, "1.2300e2 MPa") || p.ProductDetails.Models[0].Parameters[0].SourcePage != 2 {
		t.Fatal(p)
	}
	original := p
	original.Parameters += "（PDF第2页）"
	clean := standardAIProductPayload(original)
	if clean.Parameters == original.Parameters || original.ProductDetails.Models[0].Parameters[0].SourcePage != 2 {
		t.Fatal("evidence or raw input mutated")
	}
	if got := aiCleanPresentation("PDF涂层设备\n压力：10~12 MPa（PDF第2页）"); got != "PDF涂层设备\n压力：10~12 MPa" {
		t.Fatal(got)
	}
	var review AIReviewResult
	if aiDecode(`{}`, &review) == nil || aiDecode(`[]`, &review) == nil || aiDecode(`{"review_issues":[]} {}`, &review) == nil {
		t.Fatal("invalid review accepted")
	}
	if e := aiDecode(`{"review_issues":[{"message":"第2页的型号适用范围需核对"}]}`, &review); e != nil || len(review.Issues) != 1 {
		t.Fatal(e, review)
	}
	if aiDecode(`{"source_pages":[1],"models":[{"model_name":"A","parameters":[{"raw_value":{"value":12}}]}]}`, &x) == nil {
		t.Fatal("compound parameter silently coerced")
	}
}
func TestAIPipelineContinuesAfterOneProductFailsAndProtectsHumanEdits(t *testing.T) {
	t.Setenv("AI_IMPORT_ENABLED", "true")
	t.Setenv("DASHSCOPE_API_KEY", "synthetic-pipeline-test")
	t.Setenv("DASHSCOPE_BASE_URL", "https://model.invalid/compatible-mode/v1")
	a := importTestApp(t)
	var owner Account
	a.DB.Where("role='admin'").First(&owner)
	task := aiTestTask(t, a, owner.ID)
	task.Total = 2
	task.Status = "processing"
	task.Lease = "pipeline-lease"
	task.PromptVersionID = 2
	ids := []uint64{}
	for page := 1; page <= 2; page++ {
		v, e := a.aiSaveAsset(context.Background(), owner.ID, task.ID, 0, page, "original", ".png", aiTestPNG(t), nil)
		if e != nil {
			t.Fatal(e)
		}
		ids = append(ids, v.ID)
	}
	task.SourceIDs = aiJSON(ids)
	a.DB.Save(&task)
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	firstOK := false
	firstCalls, secondCalls := 0, 0
	http.DefaultTransport = aiFixtureTransport(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		var req struct {
			Messages []struct {
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(b, &req)
		var sys string
		_ = json.Unmarshal(req.Messages[0].Content, &sys)
		text := string(req.Messages[1].Content)
		content := "{}"
		switch {
		case strings.Contains(sys, "Identify every separately"):
			title := "甲设备"
			if strings.Contains(text, "source_page=2") {
				title = "乙设备"
			}
			content = fmt.Sprintf(`{"company_text":"合成企业 联系电话13900000000","blocks":[{"title":%q,"series_name":"","kind":"product"}],"logo_regions":[]}`, title)
		case strings.Contains(sys, "Build the complete inventory"):
			content = `{"products":[{"product_name":"甲设备","block_ids":["p1-b1"]},{"product_name":"乙设备","block_ids":["p2-b1"]}],"unassigned_support_blocks":[],"review_issues":[]}`
		case strings.Contains(sys, "Extract company facts"):
			content = `{"data":{"name":"合成企业"},"source_pages":[1],"field_sources":{"name":[1]},"review_issues":[]}`
		case strings.Contains(sys, "Independently compare"):
			content = `{"review_issues":[]}`
		default:
			page := 2
			name := "乙设备"
			if strings.Contains(text, "product-1") {
				firstCalls++
				page = 1
				name = "甲设备"
				if !firstOK {
					break
				}
			} else {
				secondCalls++
			}
			content = fmt.Sprintf(`{"product_name":%q,"description":"真实产品介绍","source_pages":[%d],"features":[],"applications":[],"models":[{"model_name":"S-01","parameters":[{"parameter_name":"压力","raw_value":"10~12","raw_unit":"MPa","source_page":%d,"review_required":true}]}],"images":[],"review_issues":[]}`, name, page, page)
		}
		wire, _ := json.Marshal(gin.H{"choices": []any{gin.H{"finish_reason": "stop", "message": gin.H{"content": content}}}})
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(string(wire)))}, nil
	})
	if e := a.aiRun(context.Background(), task); e != nil {
		t.Fatal(e)
	}
	var drafts []AIImportDraft
	a.DB.Table(aiTable("product")).Where("task_id=?", task.ID).Order("id").Find(&drafts)
	if len(drafts) != 2 {
		t.Fatal("later product discarded", len(drafts))
	}
	byName := map[string]AIImportDraft{}
	for _, d := range drafts {
		var p Payload
		_ = json.Unmarshal([]byte(d.Data), &p)
		byName[p.Name] = d
	}
	if byName["甲设备"].GenerationState != "incomplete" || byName["乙设备"].GenerationState != "ready" {
		t.Fatal(byName)
	}
	if firstCalls != 2 || secondCalls != 1 {
		t.Fatal("initial retry or continuation", firstCalls, secondCalls)
	}
	// The user edits a successful draft while another unit is pending; retry cannot replace it.
	good := byName["乙设备"]
	var p Payload
	_ = json.Unmarshal([]byte(good.Data), &p)
	p.Description = "厂商人工修改"
	a.DB.Table(aiTable("product")).Where("id=?", good.ID).Updates(map[string]any{"data": aiJSON(p), "version": 2})
	firstOK = true
	a.DB.Model(&AIImportWorkItem{}).Where("task_id=? AND status='waiting'", task.ID).Update("next_retry_at", time.Now().Add(-time.Minute))
	if e := a.aiRun(context.Background(), task); e != nil {
		t.Fatal(e)
	}
	if firstCalls != 3 || secondCalls != 1 {
		t.Fatal("completed product re-read", firstCalls, secondCalls)
	}
	a.DB.Table(aiTable("product")).First(&good, good.ID)
	_ = json.Unmarshal([]byte(good.Data), &p)
	if p.Description != "厂商人工修改" || good.Version != 2 {
		t.Fatal("human edits lost")
	}
	var repaired AIImportDraft
	a.DB.Table(aiTable("product")).First(&repaired, byName["甲设备"].ID)
	if repaired.GenerationState != "ready" || repaired.ID != byName["甲设备"].ID {
		t.Fatal("recovery duplicated or failed", repaired)
	}
}
func TestAIPromptVersionsPermissionAndOptimisticSave(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	admin := resourceTestSession(t, a, "admin")
	merchant := tc.login("13900009751")
	path := "/admin/settings/ai-prompts"
	tc.call("GET", path, merchant, nil, 403)
	old := tc.call("GET", path, admin, nil, 200)
	if old["version"] != float64(2) {
		t.Fatal(old)
	}
	rules := aiDefaultPromptRules()
	rules.Product = "只使用可核实产品事实，按简介、特点和参数编排。"
	saved := tc.call("PUT", path, admin, gin.H{"version": 2, "rules": rules, "note": "合成配置验收"}, 200)
	tc.call("PUT", path, admin, gin.H{"version": 2, "rules": rules}, 409)
	tc.call("POST", path+"/preview", admin, gin.H{"rules": rules}, 200)
	tc.call("PUT", path, admin, gin.H{"version": saved["version"], "rules": aiDefaultPromptRules(), "note": "恢复默认"}, 200)
	var prior AIImportPromptVersion
	a.DB.First(&prior, 2)
	if prior.Rules != aiJSON(aiDefaultPromptRules()) {
		t.Fatal("historical rules overwritten")
	}
}
func TestAIUnitBudgetRecoveryAndPromptSnapshot(t *testing.T) {
	t.Setenv("DASHSCOPE_API_KEY", "synthetic-budget-test")
	t.Setenv("DASHSCOPE_BASE_URL", "https://model.invalid/compatible-mode/v1")
	a := importTestApp(t)
	var owner Account
	a.DB.Where("role='admin'").First(&owner)
	task := aiTestTask(t, a, owner.ID)
	task.Status = "processing"
	task.Lease = "unit-lease"
	task.PromptVersionID = 2
	a.DB.Save(&task)
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	calls := 0
	valid := false
	systems := []string{}
	http.DefaultTransport = aiFixtureTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Messages []struct {
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(body, &req)
		systems = append(systems, string(req.Messages[0].Content))
		content := `{}`
		if valid {
			content = `{"product_name":"X","source_pages":[1],"models":[],"review_issues":[],"images":[]}`
		}
		wire, _ := json.Marshal(gin.H{"choices": []any{gin.H{"finish_reason": "stop", "message": gin.H{"content": content}}}})
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(string(wire)))}, nil
	})
	var out AIProductExtraction
	if e := a.aiModel(context.Background(), task, "product-1-detail-0", aiFlashModel, "fixture", nil, &out); e != errAIRetryScheduled {
		t.Fatal(e)
	}
	if calls != 2 {
		t.Fatal("schema repair budget", calls)
	}
	a.DB.Model(&AIImportWorkItem{}).Where("task_id=?", task.ID).Update("next_retry_at", time.Now().Add(-time.Minute))
	if e := a.aiModel(context.Background(), task, "product-1-detail-0", aiFlashModel, "fixture", nil, &out); e == nil {
		t.Fatal("empty result succeeded")
	}
	if calls != 3 {
		t.Fatal("nested retries exceeded actual request budget", calls)
	}
	if e := a.aiModel(context.Background(), task, "product-1-detail-0", aiFlashModel, "fixture", nil, &out); e == nil || calls != 3 {
		t.Fatal("terminal failure retried itself", e, calls)
	}
	a.DB.Model(&AIImportWorkItem{}).Where("task_id=?", task.ID).Updates(map[string]any{"status": "queued", "attempts": 0, "error": "", "next_retry_at": nil})
	valid = true
	if e := a.aiModel(context.Background(), task, "product-1-detail-0", aiFlashModel, "fixture", nil, &out); e != nil {
		t.Fatal(e)
	}
	if e := a.aiModel(context.Background(), task, "product-1-detail-0", aiFlashModel, "fixture", nil, &out); e != nil || calls != 4 {
		t.Fatal("checkpoint called again", e, calls)
	}
	if !strings.Contains(systems[0], "标准简介") {
		t.Fatal("task rules missing", systems[0])
	}
}
func TestAIRetryUnitsAppliesLatestOnlyToSelectedUneditedItems(t *testing.T) {
	t.Setenv("AI_IMPORT_ENABLED", "true")
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	merchant := tc.login("13900009761")
	other := tc.login("13900009762")
	var session Session
	a.DB.First(&session, "token_hash=?", digest(merchant))
	task := aiTestTask(t, a, session.AccountID)
	task.PromptVersionID = 2
	task.ExtractionModel = aiFlashModel
	task.ReviewModel = aiFlashModel
	task.RecognitionOutcome = "partial"
	a.DB.Save(&task)
	one := AIImportWorkItem{TaskID: task.ID, Stage: "product-1-detail-0", Status: "failed", PromptVersionID: 2, Attempts: 3, Error: "ai_invalid_schema", UpdatedAt: time.Now().UTC()}
	two := one
	two.Stage = "product-2-detail-0"
	a.DB.Create(&one)
	a.DB.Create(&two)
	human := AIImportDraft{TaskID: task.ID, OwnerID: task.OwnerID, TempID: "product-2", Version: 2, GenerationState: "incomplete", Data: aiJSON(Payload{Name: "人工修改保留"}), SourcePages: "[1]", FieldSources: "{}", ReviewIssues: "[]", SelectedFields: "[]"}
	a.DB.Table(aiTable("product")).Create(&human)
	newer := AIImportPromptVersion{Rules: aiJSON(aiDefaultPromptRules()), Note: "新规则", CreatedAt: time.Now().UTC()}
	a.DB.Create(&newer)
	a.DB.Model(&SystemSetting{}).Where("setting_key=?", aiPromptActiveKey).Update("value", aiJSON(newer.ID))
	path := fmt.Sprintf("/merchant/ai-import/tasks/%d/retry", task.ID)
	request := gin.H{"unit_ids": []uint64{one.ID}, "prompt_policy": "latest", "prompt_version_id": newer.ID, "model_mode": "max"}
	tc.call("POST", path, other, request, 404)
	tc.call("POST", path, merchant, gin.H{"unit_ids": []uint64{one.ID}, "prompt_policy": "latest", "prompt_version_id": 2}, 409)
	tc.call("POST", path, merchant, gin.H{"unit_ids": []uint64{two.ID}}, 409)
	tc.call("POST", path, merchant, request, 200)
	a.DB.First(&one, one.ID)
	a.DB.First(&two, two.ID)
	a.DB.First(&task, task.ID)
	if one.PromptVersionID != newer.ID || one.Attempts != 0 || one.Status != "queued" || two.PromptVersionID != 2 || two.Attempts != 3 || task.PromptVersionID != 2 || task.ExtractionModel != aiFlashModel || task.ReviewModel != aiFlashModel {
		t.Fatal("snapshot, budget or protected item changed", one, two, task)
	}
}
func TestAICopyCleanupCreatesOnlyPrivateUpdateDraftAndIsIdempotent(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	merchant := tc.login("13900009771")
	admin := resourceTestSession(t, a, "admin")
	vendor := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "整理草稿验收企业", RegionName: "河北", MainProducts: "机械", Phone: "13900009771"}}, 201)
	refs := serviceTestRefs(t, a, "terms", "privacy", "showroom-agreement")
	tc.call("POST", "/merchant/contents/"+number(vendor["id"])+"/submit", merchant, gin.H{"revision_id": vendor["revision_id"], "agreements": refs}, 200)
	tc.call("POST", "/admin/revisions/"+number(vendor["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	var session Session
	a.DB.First(&session, "token_hash=?", digest(merchant))
	task := aiTestTask(t, a, session.AccountID)
	page, e := a.aiSaveAsset(context.Background(), session.AccountID, task.ID, 0, 1, "page", ".png", aiTestPNG(t), nil)
	if e != nil {
		t.Fatal(e)
	}
	photo, e := a.aiCrop(context.Background(), task, page, AIRegion{SourcePage: 1, ImageType: "product_main", Box: []float64{.1, .1, .8, .8}})
	if e != nil {
		t.Fatal(e)
	}
	payload := Payload{Name: "整理测试产品", Description: "根据画册介绍，真实机械产品。", Parameters: "压力：10~12 MPa（PDF第1页）", MediaIDs: []uint64{photo.MediaID}, ProductDetails: &ProductDetails{SourcePages: []int{1}, Models: []AIProductModel{{ModelName: "X-01", Parameters: []AIParameter{{ParameterName: "压力", RawValue: "10~12", RawUnit: "MPa", SourcePage: 1}}}}}}
	product := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "product", "data": payload}, 201)
	cid := uint64(product["id"].(float64))
	rid := uint64(product["revision_id"].(float64))
	tc.call("POST", "/merchant/contents/"+number(product["id"])+"/submit", merchant, gin.H{"revision_id": rid, "agreements": refs}, 200)
	tc.call("POST", fmt.Sprintf("/admin/revisions/%d/review", rid), admin, gin.H{"decision": "approve"}, 200)
	d := AIImportDraft{TaskID: task.ID, OwnerID: task.OwnerID, TempID: "product-1", Version: 1, ContentID: cid, RevisionID: rid, Data: aiJSON(payload), SourcePages: "[1]", FieldSources: "{}", ReviewIssues: "[]", SelectedFields: "[]"}
	a.DB.Table(aiTable("product")).Create(&d)
	rows, e := a.aiCleanupCopies(false)
	if e != nil || len(rows) != 1 || rows[0]["status"] != "suggested" {
		t.Fatal(e, rows)
	}
	var unchanged Content
	a.DB.First(&unchanged, cid)
	if unchanged.DraftRevisionID != rid {
		t.Fatal("dry run wrote a draft")
	}
	rows, e = a.aiCleanupCopies(true)
	if e != nil || rows[0]["status"] != "draft_created" {
		t.Fatal(e, rows)
	}
	a.DB.First(&unchanged, cid)
	if unchanged.PublishedRevisionID != rid || unchanged.DraftRevisionID == rid {
		t.Fatal("public version changed")
	}
	var rev Revision
	a.DB.First(&rev, unchanged.DraftRevisionID)
	var cleaned Payload
	_ = json.Unmarshal([]byte(rev.Payload), &cleaned)
	if rev.Status != "draft" || strings.Contains(cleaned.Parameters, "PDF") || cleaned.Description != "真实机械产品。" || cleaned.ProductDetails.Models[0].Parameters[0].SourcePage != 1 {
		t.Fatal(rev.Status, cleaned)
	}
	rows, e = a.aiCleanupCopies(true)
	if e != nil || rows[0]["status"] != "existing_draft_kept" {
		t.Fatal("cleanup was not idempotent", e, rows)
	}
}

func TestAIPartialTaskCanCropOriginalsWithoutPublishing(t *testing.T) {
	t.Setenv("AI_IMPORT_ENABLED", "true")
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	token := tc.login("13900009781")
	other := tc.login("13900009782")
	var session Session
	a.DB.First(&session, "token_hash=?", digest(token))
	task := aiTestTask(t, a, session.AccountID)
	task.Status = "failed"
	a.DB.Save(&task)
	source, err := a.aiSaveAsset(context.Background(), task.OwnerID, task.ID, 0, 1, "page", ".png", aiTestPNG(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/merchant/ai-import/assets/%d/crops", source.ID)
	request := gin.H{"source_page": 1, "image_type": "product_main", "crop_region": []float64{.1, .1, .8, .8}, "panel": "whole"}
	tc.call("POST", path, other, request, 404)
	tc.call("POST", path, token, request, 201)
	a.DB.First(&task, task.ID)
	if task.Status != "failed" {
		t.Fatal("crop changed task state")
	}
	task.PublishKey = "legacy-locked"
	a.DB.Save(&task)
	tc.call("POST", path, token, request, 409)
}
