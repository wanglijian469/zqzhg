package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAIExplicitUpdateVersioningAndRemainingReview(t *testing.T) {
	t.Setenv("AI_IMPORT_ENABLED", "true")
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	merchant := tc.login("13900009211")
	admin := resourceTestSession(t, a, "admin")
	var session Session
	a.DB.First(&session, "token_hash=?", digest(merchant))
	var who Account
	a.DB.First(&who, session.AccountID)
	vendor := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "AI更新隔离验收", MainProducts: "设备", RegionName: "河北", Phone: "13900009211"}}, 201)
	tc.call("POST", "/merchant/service-acceptances", merchant, gin.H{"agreements": serviceTestRefs(t, a, "terms", "privacy", "showroom-agreement"), "source": "web"}, 200)
	tc.call("POST", "/merchant/contents/"+number(vendor["id"])+"/submit", merchant, gin.H{"revision_id": vendor["revision_id"]}, 200)
	tc.call("POST", "/admin/revisions/"+number(vendor["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	task := aiTestTask(t, a, who.ID)
	task.VendorID = uint64(vendor["id"].(float64))
	a.DB.Save(&task)
	page, e := a.aiSaveAsset(context.Background(), who.ID, task.ID, 0, 1, "page", ".png", aiTestPNG(t), nil)
	if e != nil {
		t.Fatal(e)
	}
	photo, e := a.aiCrop(context.Background(), task, page, AIRegion{SourcePage: 1, ImageType: "product_main", Box: []float64{.1, .1, .8, .8}})
	if e != nil {
		t.Fatal(e)
	}
	var cat Category
	a.DB.Where("category_kind='product' AND enabled=1 AND deleted_at IS NULL AND id NOT IN (SELECT DISTINCT parent_id FROM categories WHERE enabled=1)").Order("id").First(&cat)
	p := Payload{Name: "原产品", CategoryID: cat.ID, ProductCategoryID: cat.ID, MediaIDs: []uint64{photo.MediaID}, ProductDetails: &ProductDetails{SourcePages: []int{1}, Models: []AIProductModel{{ModelName: "X-1", Variant: "结构甲", Parameters: []AIParameter{{ParameterName: "压力", RawValue: "10~12", SourcePage: 1}}}}}}
	old := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "product", "data": p}, 201)
	tc.call("POST", "/merchant/contents/"+number(old["id"])+"/submit", merchant, gin.H{"revision_id": old["revision_id"]}, 200)
	tc.call("POST", "/admin/revisions/"+number(old["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	company := AIImportDraft{TaskID: task.ID, OwnerID: who.ID, TempID: "company", Version: 1, BaseRevisionID: uint64(vendor["revision_id"].(float64)), Data: aiJSON(Payload{Name: "AI更新隔离验收"}), SourcePages: "[1]", FieldSources: "{}", ReviewIssues: "[]", SelectedFields: "[]"}
	if e = a.DB.Table(aiTable("company")).Create(&company).Error; e != nil {
		t.Fatal(e)
	}
	draft := AIImportDraft{TaskID: task.ID, OwnerID: who.ID, TempID: "product-1", Version: 1, Data: aiJSON(p), SourcePages: "[1]", FieldSources: "{}", ReviewIssues: "[]", SelectedFields: "[]"}
	if e = a.aiSuggestProductUpdate(task, &p, &draft); e != nil {
		t.Fatal(e)
	}
	if draft.TargetContentID != uint64(old["id"].(float64)) || draft.PublishMode != "undecided" {
		t.Fatal("update was not an explicit suggestion", draft)
	}
	if e = a.aiStoreDraft(context.Background(), task, "product", &draft); e != nil {
		t.Fatal(e)
	}
	path := fmt.Sprintf("/merchant/ai-import/tasks/%d", task.ID)
	review := func(version uint64) {
		tc.call("POST", path+"/review", merchant, gin.H{"items": []gin.H{{"kind": "company", "id": company.ID, "version": 1}, {"kind": "product", "id": draft.ID, "version": version, "resolution": "已比对原产品，明确更新此版本"}}}, 200)
	}
	review(1)
	publish := gin.H{"idempotency_key": "explicit-update", "company_version": 1, "products": []gin.H{{"id": draft.ID, "version": 1}}}
	tc.call("POST", path+"/publish", merchant, publish, 409)
	p.ProductDetails.Models[0].Parameters[0].RawValue = "13~15"
	save := gin.H{"version": 1, "data": p, "publish_mode": "update"}
	tc.call("PUT", fmt.Sprintf("/merchant/ai-import/products/%d", draft.ID), merchant, save, 200)
	tc.call("PUT", fmt.Sprintf("/merchant/ai-import/products/%d", draft.ID), merchant, save, 409)
	review(2)
	// An unselected draft can be reviewed in a later batch without unlocking the submitted item.
	remaining := AIImportDraft{TaskID: task.ID, OwnerID: who.ID, TempID: "remaining", Version: 1, Data: aiJSON(p), SourcePages: "[1]", FieldSources: "{}", ReviewIssues: "[]", SelectedFields: "[]"}
	a.DB.Table(aiTable("product")).Create(&remaining)
	publish["products"] = []gin.H{{"id": draft.ID, "version": 2}}
	result := tc.call("POST", path+"/publish", merchant, publish, 200)
	if result["submitted"] != float64(1) || result["failed"] != float64(0) {
		t.Fatal(result)
	}
	var count int64
	a.DB.Model(&Content{}).Where("owner_id=? AND kind='product'", who.ID).Count(&count)
	if count != 1 {
		t.Fatal("update created a duplicate")
	}
	var current Content
	a.DB.First(&current, uint64(old["id"].(float64)))
	var public Revision
	a.DB.First(&public, current.PublishedRevisionID)
	var before Payload
	json.Unmarshal([]byte(public.Payload), &before)
	if before.ProductDetails.Models[0].Parameters[0].RawValue != "10~12" {
		t.Fatal("unapproved update changed public version")
	}
	tc.call("POST", path+"/continue-review", merchant, gin.H{}, 200)
	tc.call("PUT", fmt.Sprintf("/merchant/ai-import/products/%d", remaining.ID), merchant, gin.H{"version": 1, "data": p}, 200)
	tc.call("PUT", fmt.Sprintf("/merchant/ai-import/products/%d", draft.ID), merchant, gin.H{"version": 2, "data": p}, 409)
	var two Payload
	json.Unmarshal([]byte(aiJSON(p)), &two)
	two.ProductDetails.Models = append(two.ProductDetails.Models, AIProductModel{ModelName: "X-1", Variant: "结构乙", Parameters: []AIParameter{{ParameterName: "压力", RawValue: "16~18", SourcePage: 1}}})
	tc.call("PUT", fmt.Sprintf("/merchant/ai-import/products/%d", remaining.ID), merchant, gin.H{"version": 2, "data": two}, 200)
	var left, right Payload
	json.Unmarshal([]byte(aiJSON(two)), &left)
	json.Unmarshal([]byte(aiJSON(two)), &right)
	left.Name = "结构甲产品"
	right.Name = "结构乙产品"
	left.ProductDetails.Models = left.ProductDetails.Models[:1]
	right.ProductDetails.Models = nil
	split := fmt.Sprintf("/merchant/ai-import/products/%d/split", remaining.ID)
	tc.call("POST", split, merchant, gin.H{"version": 3, "products": []Payload{left, right}}, 400)
	right.ProductDetails.Models = []AIProductModel{two.ProductDetails.Models[1]}
	tc.call("POST", split, merchant, gin.H{"version": 3, "products": []Payload{left, right}}, 200)
	var children []AIImportDraft
	a.DB.Table(aiTable("product")).Where("task_id=? AND temp_id LIKE 'split-%' AND deleted=0", task.ID).Find(&children)
	if len(children) != 2 {
		t.Fatal("split did not produce two drafts")
	}
	tc.call("POST", path+"/products/merge", merchant, gin.H{"name": "两种结构", "products": []gin.H{{"id": children[0].ID, "version": 1}, {"id": children[1].ID, "version": 1}}}, 200)
	var merged AIImportDraft
	a.DB.Table(aiTable("product")).Where("task_id=? AND temp_id LIKE 'merge-%' AND deleted=0", task.ID).First(&merged)
	var mergedData Payload
	json.Unmarshal([]byte(merged.Data), &mergedData)
	if len(mergedData.ProductDetails.Models) != 2 || mergedData.ProductDetails.Models[0].Variant == mergedData.ProductDetails.Models[1].Variant {
		t.Fatal("same-model structure variants were lost")
	}
}
