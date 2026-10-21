package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type testClient struct {
	t      *testing.T
	router http.Handler
}

func (tc testClient) call(method, path, token string, body any, want int) map[string]any {
	tc.t.Helper()
	var b bytes.Buffer
	if body != nil {
		if e := json.NewEncoder(&b).Encode(body); e != nil {
			tc.t.Fatal(e)
		}
	}
	req := httptest.NewRequest(method, "/api/v1"+path, &b)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	tc.router.ServeHTTP(w, req)
	if w.Code != want {
		tc.t.Fatalf("%s %s: wanted %d got %d %s", method, path, want, w.Code, w.Body.String())
	}
	out := map[string]any{}
	json.Unmarshal(w.Body.Bytes(), &out)
	return out
}
func (tc testClient) login(phone string) string {
	tc.t.Helper()
	code := tc.call("POST", "/auth/sms", "", gin.H{"phone": phone}, 200)["development_code"]
	return tc.call("POST", "/auth/phone", "", gin.H{"phone": phone, "code": code}, 200)["token"].(string)
}
func number(v any) string { return strconv.FormatUint(uint64(v.(float64)), 10) }
func TestBusinessAndMediaIsolation(t *testing.T) {
	if os.Getenv("RUN_MYSQL_TESTS") != "1" {
		t.Skip("set RUN_MYSQL_TESTS=1 to run on an isolated MySQL database")
	}
	name := fmt.Sprintf("zqzt_exhibition_test_%d", time.Now().UnixNano())
	db, e := openDB(name)
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		if strings.HasPrefix(name, "zqzt_exhibition_test_") {
			db.Exec("DROP DATABASE `" + name + "`")
		}
	}()
	if e = migrate(db); e != nil {
		t.Fatal(e)
	}
	t.Setenv("ADMIN_PASSWORD", "Integration-Admin-2026!")
	if e = seed(db); e != nil {
		t.Fatal(e)
	}
	a := &App{DB: db, Config: Config{Env: "development", SMSProvider: "development", MediaDir: t.TempDir(), WebDir: t.TempDir()}, SMS: DevelopmentSMS{}}
	tc := testClient{t, a.router()}
	admin := tc.call("POST", "/auth/admin", "", gin.H{"username": "admin", "password": "Integration-Admin-2026!"}, 200)["token"].(string)
	merchant := tc.login("13900000001")
	other := tc.login("13900000002")
	code := tc.call("POST", "/auth/sms", "", gin.H{"phone": "13900000003"}, 200)["development_code"]
	tc.call("POST", "/auth/sms", "", gin.H{"phone": "13900000003"}, 429)
	for i := 0; i < 5; i++ {
		tc.call("POST", "/auth/phone", "", gin.H{"phone": "13900000003", "code": "wrong"}, 401)
	}
	tc.call("POST", "/auth/phone", "", gin.H{"phone": "13900000003", "code": code}, 401)
	home := tc.call("GET", "/public/home", "", nil, 200)
	tc.call("GET", "/public/vendors?q="+strings.Repeat("农", 201), "", nil, 400)
	if home["vendor_count"].(float64) != 0 {
		t.Fatal("new database must be empty")
	}
	upload := func(token string) float64 {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		f, _ := writer.CreateFormFile("file", "test.png")
		png.Encode(f, image.NewRGBA(image.Rect(0, 0, 20, 20)))
		writer.Close()
		req := httptest.NewRequest("POST", "/api/v1/merchant/media", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		tc.router.ServeHTTP(w, req)
		if w.Code != 201 {
			t.Fatal(w.Body.String())
		}
		var result map[string]any
		json.Unmarshal(w.Body.Bytes(), &result)
		return result["id"].(float64)
	}
	media := upload(merchant)
	newMedia := upload(merchant)
	foreignMedia := upload(other)
	mediaPath := "/media/" + number(media)
	tc.call("GET", mediaPath, "", nil, 404)
	tc.call("GET", mediaPath, other, nil, 404)
	tc.call("GET", mediaPath, merchant, nil, 200)
	payload := gin.H{"name": "测试厂家（仅测试）", "name_en": "Test manufacturer", "main_products": "测试配件", "region_name": "测试地区", "phone": "13900000001", "media_ids": []float64{media}}
	vendor := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": payload}, 201)
	vendorID := number(vendor["id"])
	revision := vendor["revision_id"]
	tc.call("GET", "/merchant/contents/"+vendorID, other, nil, 403)
	tc.call("GET", "/public/vendors/"+vendorID, "", nil, 404)
	tc.call("POST", "/merchant/contents/"+vendorID+"/submit", merchant, gin.H{"revision_id": revision}, 200)
	tc.fixtureCall("PUT", "/merchant/contents/"+vendorID+"/draft", merchant, gin.H{"revision_id": revision, "data": payload}, 409)
	childCategory := tc.call("POST", "/admin/categories", admin, gin.H{"name": "测试子分类", "parent_id": 2, "enabled": true}, 200)
	tc.call("PUT", "/admin/categories/2", admin, gin.H{"name": "农机配件", "parent_id": childCategory["id"], "enabled": true}, 400)
	productPayload := gin.H{"name": "测试产品", "name_en": "Test component", "category_id": childCategory["id"], "keywords": "test component", "media_ids": []float64{media}}
	product := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "product", "data": productPayload}, 201)
	productID := number(product["id"])
	tc.call("POST", "/merchant/contents/"+productID+"/submit", merchant, gin.H{"revision_id": product["revision_id"]}, 200)
	tc.call("POST", "/admin/revisions/"+number(product["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	tc.call("GET", "/public/products/"+productID, "", nil, 404)
	hiddenProduct := tc.call("GET", "/merchant/contents/"+productID, merchant, nil, 200)
	if hiddenProduct["public_visible"] != false || hiddenProduct["visible"] != true {
		t.Fatal("an approved product must report unpublished until its booth is public")
	}
	tc.call("POST", "/admin/revisions/"+number(revision)+"/review", admin, gin.H{"decision": "approve"}, 200)
	if tc.call("GET", "/merchant/contents/"+productID, merchant, nil, 200)["public_visible"] != true {
		t.Fatal("an approved product must report published when its booth becomes public")
	}
	tc.call("GET", mediaPath, "", nil, 200)
	tc.call("GET", "/public/products/"+productID, "", nil, 200)
	found := tc.call("GET", "/public/vendors?q=component&category_id=2", "", nil, 200)
	if found["total"].(float64) != 1 {
		t.Fatal("search must find one deduplicated vendor")
	}
	taxonomy := tc.call("GET", "/public/taxonomy", "", nil, 200)
	regions := taxonomy["regions"].([]any)
	if len(regions) != 1 {
		t.Fatal("only regions with public manufacturers should appear")
	}
	regionID := number(regions[0].(map[string]any)["id"])
	regional := tc.call("GET", "/public/vendors?region_id="+regionID+"&category_id=2&q=component", "", nil, 200)
	if regional["total"].(float64) != 1 {
		t.Fatal("combined region, descendant category and English keyword filter failed")
	}
	payload["name"] = "待审新名称"
	payload["media_ids"] = []float64{newMedia}
	draft := tc.fixtureCall("PUT", "/merchant/contents/"+vendorID+"/draft", merchant, gin.H{"revision_id": revision, "data": payload}, 200)
	tc.fixtureCall("PUT", "/merchant/contents/"+vendorID+"/draft", merchant, gin.H{"revision_id": revision, "data": payload}, 409)
	tc.call("POST", "/merchant/contents/"+vendorID+"/submit", merchant, gin.H{"revision_id": draft["revision_id"]}, 200)
	pub := tc.call("GET", "/public/vendors/"+vendorID, "", nil, 200)
	if pub["item"].(map[string]any)["data"].(map[string]any)["name"] != "测试厂家（仅测试）" {
		t.Fatal("pending edit replaced published snapshot")
	}
	tc.call("GET", "/media/"+number(newMedia), "", nil, 404)
	tc.call("GET", mediaPath, "", nil, 200)
	tc.call("POST", "/admin/revisions/"+number(draft["revision_id"])+"/review", admin, gin.H{"decision": "reject"}, 400)
	tc.call("POST", "/admin/revisions/"+number(draft["revision_id"])+"/review", admin, gin.H{"decision": "reject", "note": "请补充说明"}, 200)
	tc.call("POST", "/admin/revisions/"+number(draft["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 409)
	payload["media_ids"] = []float64{foreignMedia}
	tc.fixtureCall("PUT", "/merchant/contents/"+vendorID+"/draft", merchant, gin.H{"revision_id": draft["revision_id"], "data": payload}, 400)
	payload["media_ids"] = []float64{newMedia}
	replacement := tc.fixtureCall("PUT", "/merchant/contents/"+vendorID+"/draft", merchant, gin.H{"revision_id": draft["revision_id"], "data": payload}, 200)
	tc.call("POST", "/merchant/contents/"+vendorID+"/submit", merchant, gin.H{"revision_id": replacement["revision_id"]}, 200)
	tc.call("POST", "/admin/revisions/"+number(replacement["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	tc.call("GET", "/media/"+number(newMedia), "", nil, 200)
	racingDraft := tc.fixtureCall("PUT", "/merchant/contents/"+vendorID+"/draft", merchant, gin.H{"revision_id": replacement["revision_id"], "data": payload}, 200)
	tc.call("POST", "/merchant/contents/"+vendorID+"/submit", merchant, gin.H{"revision_id": racingDraft["revision_id"]}, 200)
	results := make(chan int, 2)
	for _, request := range []struct {
		path string
		data gin.H
	}{
		{"/admin/revisions/" + number(racingDraft["revision_id"]) + "/review", gin.H{"decision": "approve"}},
		{"/admin/contents/" + vendorID + "/offline", gin.H{}},
	} {
		go func(path string, data gin.H) {
			blob, _ := json.Marshal(data)
			req := httptest.NewRequest("POST", "/api/v1"+path, bytes.NewReader(blob))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+admin)
			w := httptest.NewRecorder()
			tc.router.ServeHTTP(w, req)
			results <- w.Code
		}(request.path, request.data)
	}
	for i := 0; i < 2; i++ {
		status := <-results
		if status != 200 && status != 409 {
			t.Fatalf("concurrent review/offline returned %d", status)
		}
	}
	tc.call("GET", "/public/vendors/"+vendorID, "", nil, 404)
	tc.call("GET", "/public/products/"+productID, "", nil, 404)
	tc.call("GET", mediaPath, "", nil, 404)
	tc.call("GET", "/media/"+number(newMedia), "", nil, 404)
	tc.call("POST", "/admin/categories", admin, gin.H{"name": "自引用", "parent_id": 99999, "enabled": true}, 400)
	tc.call("POST", "/auth/logout", merchant, gin.H{}, 200)
	tc.call("GET", "/merchant/contents", merchant, nil, 401)

	// Verify the shipped backup and restore tools against populated, isolated data.
	backupDirectory := filepath.Join(t.TempDir(), "backup")
	backupCommand := exec.Command("pwsh", "-NoProfile", "-File", "../scripts/backup.ps1", "-Database", name, "-OutputDirectory", backupDirectory, "-MediaDirectory", a.Config.MediaDir)
	if output, e := backupCommand.CombinedOutput(); e != nil {
		t.Fatalf("backup failed: %v %s", e, output)
	}
	archive, e := zip.OpenReader(filepath.Join(backupDirectory, "media.zip"))
	if e != nil {
		t.Fatal(e)
	}
	defer archive.Close()
	if len(archive.File) != 6 {
		t.Fatalf("expected 3 originals and 3 thumbnails, got %d", len(archive.File))
	}
	for _, entry := range archive.File {
		reader, e := entry.Open()
		if e != nil {
			t.Fatal(e)
		}
		archived, e := io.ReadAll(reader)
		reader.Close()
		if e != nil {
			t.Fatal(e)
		}
		original, e := os.ReadFile(filepath.Join(a.Config.MediaDir, entry.Name))
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(archived, original) {
			t.Fatal("media backup differs from stored image")
		}
	}
	restoreCommand := exec.Command("node", "../scripts/restore-test.mjs", backupDirectory)
	output, e := restoreCommand.CombinedOutput()
	if e != nil {
		t.Fatalf("restore failed: %v %s", e, output)
	}
	restoredName := strings.TrimSpace(strings.Split(string(output), "\n")[0])
	if !strings.HasPrefix(restoredName, "zqzt_exhibition_restore_") {
		t.Fatal("unexpected restore target")
	}
	defer db.Exec("DROP DATABASE `" + restoredName + "`")
	restoredMediaDir := filepath.Join(backupDirectory, "restored-media-"+restoredName)
	for _, entry := range archive.File {
		original, e := os.ReadFile(filepath.Join(a.Config.MediaDir, entry.Name))
		if e != nil {
			t.Fatal(e)
		}
		restored, e := os.ReadFile(filepath.Join(restoredMediaDir, entry.Name))
		if e != nil || !bytes.Equal(original, restored) {
			t.Fatal("restored media differs from the source")
		}
	}
	for _, table := range []string{"contents", "revisions", "media", "media_references", "review_logs", "operation_logs"} {
		var originalCount, restoredCount int64
		db.Table(table).Count(&originalCount)
		if e := db.Table(restoredName + "." + table).Count(&restoredCount).Error; e != nil {
			t.Fatal(e)
		}
		if originalCount != restoredCount {
			t.Fatalf("restored %s count differs", table)
		}
	}
	restoredDB, e := openDB(restoredName)
	if e != nil {
		t.Fatal(e)
	}
	restoredPool, _ := restoredDB.DB()
	defer restoredPool.Close()
	restoredApp := &App{DB: restoredDB, Config: Config{Env: "development", SMSProvider: "development", MediaDir: restoredMediaDir, WebDir: t.TempDir()}, SMS: DevelopmentSMS{}}
	restoredClient := testClient{t, restoredApp.router()}
	restoredClient.call("GET", "/public/vendors/"+vendorID, "", nil, 404)
	restoredClient.call("GET", "/public/products/"+productID, "", nil, 404)
	restoredClient.call("GET", "/media/"+number(newMedia), "", nil, 404)
	restoredClient.call("GET", "/media/"+number(newMedia), admin, nil, 200)
	restoredClient.call("GET", "/admin/contents/"+vendorID, admin, nil, 200)
	t.Log("Backup and restore verified: checksums, populated isolated database, 3 images and 3 thumbnails restored byte-for-byte, revision and audit counts, restored API and media permissions")
}
