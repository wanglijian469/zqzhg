package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

// Synthetic UI fixture, explicitly separate from real brochure acceptance.
func TestAIImportBrowser(t *testing.T) {
	if os.Getenv("AI_IMPORT_BROWSER_TEST") != "1" {
		t.Skip("set AI_IMPORT_BROWSER_TEST=1 for isolated browser acceptance")
	}
	t.Setenv("AI_IMPORT_ENABLED", "true")
	t.Setenv("LOCAL_PREVIEW_SAME_ORIGIN", "true")
	t.Setenv("DASHSCOPE_API_KEY", "synthetic-ui-no-model-call")
	t.Setenv("DASHSCOPE_BASE_URL", "https://example.test/compatible-mode/v1")
	a := importTestApp(t)
	settings := defaultSystemConfig()
	settings.Site.WebURL = "https://saved-primary.invalid"
	settings.Site.APIURL = "https://saved-primary.invalid/api/v1"
	settings.Site.MediaURL = "https://saved-primary.invalid/media"
	if e := a.DB.Create(&SystemSetting{SettingKey: "system_config_v3", Value: aiJSON(settings)}).Error; e != nil {
		t.Fatal(e)
	}
	probe, _ := gin.CreateTestContext(httptest.NewRecorder())
	probe.Request = httptest.NewRequest("GET", "http://127.0.0.1:8094/", nil)
	a.Config.Env = "production"
	if a.effectiveConfig(probe).Site.APIURL != settings.Site.APIURL {
		t.Fatal("preview override affected production")
	}
	a.Config.Env = "development"
	probe.Request = httptest.NewRequest("GET", "http://example.invalid/", nil)
	if a.effectiveConfig(probe).Site.APIURL != settings.Site.APIURL {
		t.Fatal("preview override accepted a non-loopback host")
	}
	web, e := filepath.Abs("../.local/ai-import-batch-web")
	if e != nil {
		t.Fatal(e)
	}
	a.Config.WebDir = web
	tc := testClient{t, a.router()}
	merchant := tc.login("13900009191")
	admin := resourceTestSession(t, a, "admin")
	var session Session
	a.DB.First(&session, "token_hash=?", digest(merchant))
	owner := session.AccountID
	tc.call("POST", "/merchant/service-acceptances", merchant, gin.H{"agreements": serviceTestRefs(t, a, "terms", "privacy", "showroom-agreement"), "source": "web"}, 200)
	vendor := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "界面合成验收企业（非真实画册）", MainProducts: "合成测试设备", RegionName: "河北", Phone: "13900009191"}}, 201)
	tc.call("POST", "/merchant/contents/"+number(vendor["id"])+"/submit", merchant, gin.H{"revision_id": vendor["revision_id"]}, 200)
	tc.call("POST", "/admin/revisions/"+number(vendor["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	task := aiTestTask(t, a, owner)
	task.FileName = "界面合成验收样本（非真实解析）.png"
	task.VendorID = uint64(vendor["id"].(float64))
	a.DB.Save(&task)
	page, e := a.aiSaveAsset(context.Background(), owner, task.ID, 0, 1, "page", ".png", aiTestPNG(t), nil)
	if e != nil {
		t.Fatal(e)
	}
	photo, e := a.aiCrop(context.Background(), task, page, AIRegion{SourcePage: 1, ImageType: "product_main", Box: []float64{0, 0, .7, .7}})
	if e != nil {
		t.Fatal(e)
	}
	diagram, e := a.aiCrop(context.Background(), task, page, AIRegion{SourcePage: 1, ImageType: "dimensions", Box: []float64{.1, .1, .8, .8}})
	if e != nil {
		t.Fatal(e)
	}
	company := AIImportDraft{TaskID: task.ID, OwnerID: owner, TempID: "company", Version: 1, BaseRevisionID: uint64(vendor["revision_id"].(float64)), Data: aiJSON(Payload{Name: "界面合成验收企业（非真实画册）"}), SourcePages: "[1]", FieldSources: "{\"name\":[1]}", ReviewIssues: "[]", SelectedFields: "[]"}
	if e = a.DB.Table(aiTable("company")).Create(&company).Error; e != nil {
		t.Fatal(e)
	}
	var cat Category
	a.DB.Where("category_kind='product' AND enabled=1 AND deleted_at IS NULL AND id NOT IN (SELECT DISTINCT parent_id FROM categories WHERE enabled=1)").Order("id").First(&cat)
	p := Payload{Name: "界面合成产品（非真实解析）", Description: "仅用于浏览器交互和布局验收", ProductCategoryID: cat.ID, CategoryID: cat.ID, MediaIDs: []uint64{photo.MediaID}, ProductDetails: &ProductDetails{SeriesName: "合成系列", Features: []string{}, Applications: []string{}, Models: []AIProductModel{{ModelName: "FIXTURE-A", Parameters: []AIParameter{{ParameterName: "压力", RawValue: "10~12", RawUnit: "MPa", SourcePage: 1, ReviewRequired: true}}}}, Images: []AIProductImage{{MediaID: photo.MediaID, ImageType: "product_main", SourcePage: 1}, {MediaID: diagram.MediaID, ImageType: "dimensions", SourcePage: 1}}, SourcePages: []int{1}}}
	draft := AIImportDraft{TaskID: task.ID, OwnerID: owner, TempID: "product-1", Version: 1, Data: aiJSON(p), SourcePages: "[1]", FieldSources: "{}", ReviewIssues: "[\"请核对合成测试参数\"]", SelectedFields: "[]"}
	if e = a.DB.Table(aiTable("product")).Create(&draft).Error; e != nil {
		t.Fatal(e)
	}
	for i := 2; i <= 7; i++ {
		extra := draft
		extra.ID = 0
		extra.TempID = "ui-product-" + fmt.Sprint(i)
		copy := p
		copy.Name = "界面合成产品 " + fmt.Sprint(i) + "（非真实解析）"
		extra.Data = aiJSON(copy)
		if e = a.DB.Table(aiTable("product")).Create(&extra).Error; e != nil {
			t.Fatal(e)
		}
	}
	srv := httptest.NewServer(a.router())
	defer srv.Close()
	a.Config.PublicBaseURL = srv.URL
	fixture := gin.H{"base": srv.URL, "owner": merchant, "admin": admin, "account": owner, "task": task.ID, "synthetic": true}
	encoded, _ := json.Marshal(fixture)
	cmd := exec.Command("node", "../scripts/verify-ai-import-batches.cjs")
	cmd.Stdin = bytes.NewReader(encoded)
	out, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("browser: %v\n%s", e, out)
	}
	t.Log(string(out))
}
