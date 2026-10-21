package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"image"
	"image/png"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestHomeDisplayImagePublicationPermissionsAndRollback(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	admin := resourceTestSession(t, a, "admin")
	merchant := resourceTestSession(t, a, "merchant")
	tc.call("GET", "/admin/home-display", "", nil, 401)
	tc.call("GET", "/admin/home-display", merchant, nil, 403)
	tc.call("PUT", "/admin/home-display", merchant, gin.H{"hero_media_id": 0, "version": 1}, 403)
	var photo bytes.Buffer
	png.Encode(&photo, image.NewRGBA(image.Rect(0, 0, 12, 8)))
	resourceUpload(t, a, merchant, "/admin/home-display/media", "hero.png", photo.Bytes(), 403)
	resourceUpload(t, a, admin, "/admin/home-display/media", "broken.png", []byte("not an image"), 400)
	initial := tc.call("GET", "/admin/home-display", admin, nil, 200)
	if initial["hero_media_id"] != float64(0) || initial["vendor_limit"] != float64(12) {
		t.Fatal(initial)
	}
	first := resourceUpload(t, a, admin, "/admin/home-display/media", "hero.png", photo.Bytes(), 201)
	id := number(first["id"])
	tc.call("GET", "/media/"+id, "", nil, 404)
	homeMediaRead(t, a, admin, "/media/"+id, 200)
	foreign := resourceUpload(t, a, merchant, "/merchant/media", "company.png", photo.Bytes(), 201)
	tc.call("PUT", "/admin/home-display", admin, gin.H{"hero_media_id": foreign["id"], "version": 1}, 400)
	saved := tc.call("PUT", "/admin/home-display", admin, gin.H{"hero_media_id": first["id"], "version": 1}, 200)
	homeMediaRead(t, a, "", "/media/"+id, 200)
	homeMediaRead(t, a, "", "/media/"+id+"?thumb=1", 200)
	home := tc.call("GET", "/public/home", "", nil, 200)
	if home["hero"].(map[string]any)["media_id"] != first["id"] {
		t.Fatal("hero missing")
	}
	tc.call("PUT", "/admin/home-display", admin, gin.H{"hero_media_id": 0, "version": 1}, 409)
	second := resourceUpload(t, a, admin, "/admin/home-display/media", "replacement.png", photo.Bytes(), 201)
	updated := tc.call("PUT", "/admin/home-display", admin, gin.H{"hero_media_id": second["id"], "version": saved["version"]}, 200)
	tc.call("GET", "/media/"+id, "", nil, 404)
	homeMediaRead(t, a, admin, "/media/"+id, 200)
	homeMediaRead(t, a, "", "/media/"+number(second["id"]), 200)
	tc.call("PUT", "/admin/home-display", admin, gin.H{"hero_media_id": 0, "version": updated["version"]}, 200)
	tc.call("GET", "/media/"+number(second["id"]), "", nil, 404)
	before, _ := os.ReadDir(a.Config.MediaDir)
	var mediaBefore int64
	a.DB.Model(&Media{}).Count(&mediaBefore)
	a.DB.Callback().Create().Before("gorm:create").Register("home_upload_rollback", func(tx *gorm.DB) {
		if tx.Statement.Table == "home_display_media" {
			tx.AddError(errors.New("isolated insert failure"))
		}
	})
	resourceUpload(t, a, admin, "/admin/home-display/media", "rollback.png", photo.Bytes(), 500)
	a.DB.Callback().Create().Remove("home_upload_rollback")
	after, _ := os.ReadDir(a.Config.MediaDir)
	var mediaAfter int64
	a.DB.Model(&Media{}).Count(&mediaAfter)
	if len(before) != len(after) || mediaBefore != mediaAfter {
		t.Fatal("upload rollback leaked file or media")
	}
	var audit int64
	a.DB.Model(&OperationLog{}).Where("action='home_display_saved'").Count(&audit)
	if audit != 3 {
		t.Fatal("missing settings audit", audit)
	}
}

func TestFeaturedVendorLimitConcurrentAndPublicSnapshots(t *testing.T) {
	a := importTestApp(t)
	r := a.router()
	tc := testClient{t, r}
	admin := resourceTestSession(t, a, "admin")
	var vendors []map[string]any
	for i := 0; i < 14; i++ {
		name := fmt.Sprintf("首页精选测试厂商%02d", i)
		v := tc.fixtureCall("POST", "/admin/vendors", admin, gin.H{"login_name": name, "data": Payload{Name: name, RegionName: "河北", MainProducts: "机械", Phone: "0319-1234567"}}, 201)["item"].(map[string]any)
		tc.call("POST", "/admin/contents/"+number(v["id"])+"/submit", admin, gin.H{"revision_id": v["revision_id"]}, 200)
		tc.call("POST", "/admin/revisions/"+number(v["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
		vendors = append(vendors, v)
	}
	for i, v := range vendors[:11] {
		tc.call("PUT", "/admin/contents/"+number(v["id"])+"/featured", admin, gin.H{"featured": true, "sort_order": i}, 200)
	}
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	start := make(chan struct{})
	for _, v := range vendors[11:13] {
		wg.Add(1)
		go func(v map[string]any) {
			defer wg.Done()
			<-start
			w := homePutRequest(r, "/admin/contents/"+number(v["id"])+"/featured", admin, gin.H{"featured": true, "sort_order": 0})
			codes <- w.Code
		}(v)
	}
	close(start)
	wg.Wait()
	close(codes)
	success, limited := 0, 0
	for code := range codes {
		if code == 200 {
			success++
		} else if code == 409 {
			limited++
		} else {
			t.Fatal(code)
		}
	}
	if success != 1 || limited != 1 {
		t.Fatal("concurrent limit", success, limited)
	}
	stats := tc.call("GET", "/admin/home-display", admin, nil, 200)
	if stats["selected_vendor_count"] != float64(12) {
		t.Fatal(stats)
	}
	tc.call("PUT", "/admin/contents/"+number(vendors[13]["id"])+"/featured", admin, gin.H{"featured": true}, 409)
	v := vendors[0]
	tc.fixtureCall("PUT", "/admin/contents/"+number(v["id"])+"/draft", admin, gin.H{"revision_id": v["revision_id"], "data": Payload{Name: "未审核新名称不能进入首页", RegionName: "河北", MainProducts: "机械", Phone: "0319-1234567"}}, 200)
	home := tc.call("GET", "/public/home", "", nil, 200)
	if len(home["vendors"].([]any)) != 8 {
		t.Fatal("home limit")
	}
	for _, item := range home["vendors"].([]any) {
		if strings.Contains(item.(map[string]any)["data"].(map[string]any)["name"].(string), "未审核") {
			t.Fatal("unreviewed snapshot leaked")
		}
	}
	// A legacy database above the limit remains untouched; removal and reorder still work.
	a.DB.Model(&Content{}).Where("kind='vendor'").Update("featured", true)
	home = tc.call("GET", "/public/home", "", nil, 200)
	if len(home["vendors"].([]any)) != 8 {
		t.Fatal("legacy display limit")
	}
	tc.call("PUT", "/admin/contents/"+number(v["id"])+"/featured", admin, gin.H{"featured": true, "sort_order": -1}, 200)
	tc.call("PUT", "/admin/contents/"+number(vendors[13]["id"])+"/featured", admin, gin.H{"featured": false}, 200)
	tc.call("POST", "/admin/contents/"+number(v["id"])+"/offline", admin, gin.H{}, 200)
	home = tc.call("GET", "/public/home", "", nil, 200)
	for _, row := range home["vendors"].([]any) {
		if row.(map[string]any)["id"] == v["id"] {
			t.Fatal("offline vendor shown")
		}
	}
}

func TestHomeDisplayInterruptedMigrationAndConcurrentVersion(t *testing.T) {
	a := importTestApp(t)
	a.DB.Model(&HomeDisplaySettings{}).Where("id=1").Update("version", 9)
	a.DB.Exec("DELETE FROM schema_migrations WHERE version=7")
	a.DB.Exec("DROP TABLE home_display_media")
	if e := migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	if e := migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	var settings HomeDisplaySettings
	a.DB.First(&settings, 1)
	if settings.Version != 9 {
		t.Fatal("migration replaced settings")
	}
	r := a.router()
	admin := resourceTestSession(t, a, "admin")
	start := make(chan struct{})
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			codes <- homePutRequest(r, "/admin/home-display", admin, gin.H{"hero_media_id": 0, "version": 9}).Code
		}()
	}
	close(start)
	first, second := <-codes, <-codes
	if !((first == 200 && second == 409) || (first == 409 && second == 200)) {
		t.Fatal("concurrent config", strconv.Itoa(first), strconv.Itoa(second))
	}
}

func homePutRequest(r *gin.Engine, path, token string, data any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(data)
	req := httptest.NewRequest("PUT", "/api/v1"+path, bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
func homeMediaRead(t *testing.T, a *App, token, path string, want int) {
	t.Helper()
	for _, method := range []string{"GET", "HEAD"} {
		req := httptest.NewRequest(method, "/api/v1"+path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		a.router().ServeHTTP(w, req)
		if w.Code != want {
			t.Fatal(method, path, w.Code)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("media cache policy")
		}
	}
}
