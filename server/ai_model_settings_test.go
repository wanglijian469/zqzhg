package main

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAIModelPolicyEncryptedSettingsAndPermissions(t *testing.T) {
	t.Setenv("AI_IMPORT_ENABLED", "true")
	t.Setenv("DASHSCOPE_API_KEY", "synthetic-environment-key")
	t.Setenv("DASHSCOPE_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1")
	t.Setenv("DASHSCOPE_MODEL", aiMaxModel)
	t.Setenv("DASHSCOPE_REVIEW_MODEL", aiMaxModel)
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	admin := resourceTestSession(t, a, "admin")
	merchant := tc.login("13900009331")
	operator := resourceTestSession(t, a, "operator")
	var operatorSession Session
	a.DB.First(&operatorSession, "token_hash=?", digest(operator))
	a.DB.Create(&PlatformOperator{AccountID: operatorSession.AccountID, Enabled: true, Permissions: `["system.read"]`})
	path := "/admin/settings/ai-models"
	tc.call("GET", path, "", nil, 401)
	tc.call("GET", path, merchant, nil, 403)
	tc.call("GET", path, operator, nil, 403)
	view := tc.call("GET", path, admin, nil, 200)
	if view["default_model"] != aiFlashModel || view["review_model"] != aiFlashModel || strings.Contains(aiJSON(view), "synthetic-environment-key") {
		t.Fatal("environment model escalated or secret returned")
	}
	cfg, e := a.aiTaskConfig(AIImportTask{})
	if e != nil || cfg.Model != aiFlashModel || cfg.ReviewModel != aiFlashModel {
		t.Fatal("default task not all flash", e)
	}
	if _, e = a.aiTaskConfig(AIImportTask{ModelMode: "max"}); e == nil {
		t.Fatal("max allowed without consent")
	}
	secret := "synthetic-business-secret-0987"
	req := gin.H{"version": 0, "base_url": "https://dashscope.aliyuncs.com/compatible-mode/v1", "timeout_seconds": 90, "allow_manual_max": true, "key_action": "replace", "api_key": secret}
	view = tc.call("PUT", path, admin, req, 200)
	if strings.Contains(aiJSON(view), secret) || strings.Contains(aiJSON(view), "encrypted_key") {
		t.Fatal("secret/ciphertext returned")
	}
	var row SystemSetting
	a.DB.First(&row, "setting_key=?", aiModelSettingKey)
	if strings.Contains(row.Value, secret) {
		t.Fatal("plaintext key stored")
	}
	reloaded := &App{DB: a.DB, Config: a.Config}
	cfg, _, e = reloaded.aiBailianConfig()
	if e != nil || cfg.Key != secret {
		t.Fatal("encrypted configuration did not survive application reload", e)
	}
	tc.call("PUT", path, admin, req, 409)
	req["version"] = 1
	req["api_key"] = "sk-sp-not-allowed"
	tc.call("PUT", path, admin, req, 400)
	req["key_action"] = "keep"
	delete(req, "api_key")
	req["allow_manual_max"] = false
	tc.call("PUT", path, admin, req, 200)
	now := time.Now().UTC()
	if _, e = a.aiTaskConfig(AIImportTask{ModelMode: "max", MaxConfirmedAt: &now}); e != nil {
		t.Fatal("old confirmed task model changed")
	}
	public := tc.call("GET", "/merchant/ai-import/config", merchant, nil, 200)
	if strings.Contains(aiJSON(public), secret) || public["review_model"] != aiFlashModel {
		t.Fatal("merchant config leaked key or auto max")
	}
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	called := []string{}
	http.DefaultTransport = aiFixtureTransport(func(r *http.Request) (*http.Response, error) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		called = append(called, body["model"].(string))
		status := 200
		if len(called) == 1 {
			status = 500
		}
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"id":"synthetic","choices":[{"finish_reason":"stop","message":{"content":"{\"ok\":true}"}}]}`))}, nil
	})
	tc.call("POST", path+"/test", admin, gin.H{}, 200)
	if len(called) != 2 || called[0] != aiFlashModel || called[1] != aiFlashModel {
		t.Fatal("retry automatically escalated", called)
	}
	tc.call("POST", path+"/test", admin, gin.H{"model_mode": "max", "confirm_max": true}, 200)
	req["version"] = 2
	req["allow_manual_max"] = true
	tc.call("PUT", path, admin, req, 200)
	tc.call("POST", path+"/test", admin, gin.H{"model_mode": "max"}, 400)
	tc.call("POST", path+"/test", admin, gin.H{"model_mode": "max", "confirm_max": true}, 200)
	if called[len(called)-1] != aiMaxModel {
		t.Fatal("explicit max was not used")
	}
	req["version"] = 3
	req["key_action"] = "environment"
	tc.call("PUT", path, admin, req, 200)
	cfg, _, e = a.aiBailianConfig()
	if e != nil || cfg.Key != "synthetic-environment-key" {
		t.Fatal("environment fallback failed")
	}
	req["version"] = 4
	req["key_action"] = "disable"
	tc.call("PUT", path, admin, req, 200)
	cfg, _, e = a.aiBailianConfig()
	if e != nil || cfg.Key != "" {
		t.Fatal("disabled key fell back to environment")
	}
}

func TestAIUploadUsesAdministratorSnapshots(t *testing.T) {
	t.Setenv("AI_IMPORT_ENABLED", "true")
	t.Setenv("DASHSCOPE_API_KEY", "synthetic-no-call")
	t.Setenv("DASHSCOPE_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1")
	a := importTestApp(t)
	router := a.router()
	tc := testClient{t, router}
	merchant := tc.login("13900009332")
	upload := func(mode, confirmation string, want int) map[string]any {
		var b bytes.Buffer
		w := multipart.NewWriter(&b)
		part, _ := w.CreateFormFile("files", "fixture.png")
		part.Write(aiTestPNG(t))
		w.WriteField("request_id", randomToken())
		w.WriteField("agreements", aiJSON(serviceTestRefs(t, a, "terms", "privacy", "showroom-agreement")))
		w.WriteField("model_mode", mode)
		w.WriteField("confirm_max", confirmation)
		w.Close()
		r := httptest.NewRequest("POST", "/api/v1/merchant/ai-import/tasks", &b)
		r.Header.Set("Content-Type", w.FormDataContentType())
		r.Header.Set("Authorization", "Bearer "+merchant)
		out := httptest.NewRecorder()
		router.ServeHTTP(out, r)
		if out.Code != want {
			t.Fatalf("upload status %d want %d: %s", out.Code, want, out.Body.String())
		}
		var data map[string]any
		json.Unmarshal(out.Body.Bytes(), &data)
		return data
	}
	for _, mode := range []string{"max", "automatic", "flash", ""} {
		out := upload(mode, "true", 202)
		task := out["task"].(map[string]any)
		if task["extraction_model"] != aiFlashModel || task["review_model"] != aiFlashModel || task["model_mode"] != "admin" {
			t.Fatal("merchant overrode administrator models", task)
		}
		a.DB.Model(&AIImportTask{}).Where("id=?", task["id"]).Updates(map[string]any{"status": "failed", "file_hash": randomToken()})
	}
	admin := resourceTestSession(t, a, "admin")
	req := gin.H{"version": 0, "base_url": "https://dashscope.aliyuncs.com/compatible-mode/v1", "timeout_seconds": 90, "key_action": "keep", "extraction_model": aiMaxModel, "review_model": aiFlashModel}
	tc.call("PUT", "/admin/settings/ai-models", admin, req, 400)
	req["confirm_max"] = true
	tc.call("PUT", "/admin/settings/ai-models", admin, req, 200)
	out := upload("flash", "", 202)
	task := out["task"].(map[string]any)
	if task["extraction_model"] != aiMaxModel || task["review_model"] != aiFlashModel {
		t.Fatal(task)
	}
	req["version"] = 1
	req["extraction_model"] = aiFlashModel
	req["confirm_max"] = false
	tc.call("PUT", "/admin/settings/ai-models", admin, req, 200)
	var persisted AIImportTask
	a.DB.First(&persisted, task["id"])
	cfg, e := a.aiTaskConfig(persisted)
	if e != nil || cfg.Model != aiMaxModel || cfg.ReviewModel != aiFlashModel {
		t.Fatal("old snapshot changed", e)
	}
}
