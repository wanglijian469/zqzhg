package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"image"
	"image/png"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func registerStorageTestRoot(t *testing.T, root string) {
	t.Helper()
	var roots []LocalRoot
	json.Unmarshal([]byte(os.Getenv("LOCAL_STORAGE_ROOTS_JSON")), &roots)
	roots = append(roots, LocalRoot{Key: randomToken(), Name: "Isolated test", Path: root})
	raw, _ := json.Marshal(roots)
	t.Setenv("LOCAL_STORAGE_ROOTS_JSON", string(raw))
}
func TestStorageSettingsPermissionsAndVersions(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	admin := resourceTestSession(t, a, "admin")
	merchant := resourceTestSession(t, a, "merchant")
	tc.call("GET", "/admin/settings", merchant, nil, 403)
	result := tc.call("GET", "/admin/settings", admin, nil, 200)
	raw, _ := json.Marshal(result["config"])
	var cfg SystemConfig
	json.Unmarshal(raw, &cfg)
	if cfg.Uploads.DefaultQuotaBytes != 200000000 || cfg.Uploads.ImageMiB != 10 || cfg.Uploads.PDFMiB != 50 {
		t.Fatal(cfg)
	}
	public := tc.call("GET", "/public/config", "", nil, 200)
	if public["paths"] != nil || public["roots"] != nil || public["credentials"] != nil {
		t.Fatal("private configuration leaked")
	}
	cfg.Site.Name = "隔离设置测试"
	tc.call("PUT", "/admin/settings", admin, cfg, 200)
	tc.call("PUT", "/admin/settings", admin, cfg, 409)
	cfg.Version++
	cfg.Site.WebURL = "http://127.0.0.1:9"
	tc.call("PUT", "/admin/settings", admin, cfg, 400)
	if err := migrate(a.DB); err != nil {
		t.Fatal("migration retry", err)
	}
}
func TestStorageQuotaConcurrentReservationsAndRelease(t *testing.T) {
	a := importTestApp(t)
	acc := Account{Username: randomToken(), Role: "merchant"}
	a.DB.Create(&acc)
	limit := int64(100)
	a.DB.Create(&MerchantStorageQuota{AccountID: acc.ID, Initialized: true, QuotaBytes: &limit})
	var wg sync.WaitGroup
	var mu sync.Mutex
	var leases []*UploadLease
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l, e := a.reserveUpload(acc.ID, 60)
			if e == nil {
				mu.Lock()
				leases = append(leases, l)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(leases) != 1 {
		t.Fatalf("concurrent quota admitted %d", len(leases))
	}
	leases[0].Release()
	l, e := a.reserveUpload(acc.ID, 100)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Release()
	key := "nested/quota.bin"
	if e := a.storeWorkFile(context.Background(), key, writeStorageFixture(t, []byte("abc"))); e != nil {
		t.Fatal(e)
	}
	if e = l.Commit(key, key); e != nil {
		t.Fatal(e)
	}
	used, reserved, cap, e := a.ownerUsage(a.DB, acc.ID)
	if e != nil || used != 3 || reserved != 0 || cap != 100 {
		t.Fatal(used, reserved, cap, e)
	}
	limit = 2
	a.DB.Model(&MerchantStorageQuota{}).Where("account_id=?", acc.ID).Update("quota_bytes", limit)
	if _, e = a.reserveUpload(acc.ID, 1); e == nil {
		t.Fatal("historical overquota upload allowed")
	}
	if _, _, e = a.resolveObject(context.Background(), key); e != nil {
		t.Fatal("historical overquota file unreadable", e)
	}
}
func writeStorageFixture(t *testing.T, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fixture")
	if e := os.WriteFile(p, data, 0600); e != nil {
		t.Fatal(e)
	}
	return p
}
func TestStorageVerifiedReplicaFallbackAndNestedInventory(t *testing.T) {
	a := importTestApp(t)
	ctx := context.Background()
	data := []byte("verified content")
	key := "companies/10/gallery/file.jpg"
	p, _ := localObjectPath(a.Config.MediaDir, key)
	os.MkdirAll(filepath.Dir(p), 0700)
	os.WriteFile(p, data, 0600)
	objects, e := a.inventory()
	if e != nil || len(objects) != 1 {
		t.Fatal(objects, e)
	}
	original := objects[0]
	if original.Key != key {
		t.Fatal(original)
	}
	root := t.TempDir()
	registerStorageTestRoot(t, root)
	profile := StorageProfile{Name: "Replica test", Kind: "local", LocalRoot: root}
	a.DB.Create(&profile)
	if e = a.registerReplica(a.DB, original, a.Config.MediaDir); e != nil {
		t.Fatal(e)
	}
	a.DB.Model(&StoredObject{}).Where("object_key=?", key).Update("store_id", profile.ID)
	_, store, e := a.resolveObject(ctx, key)
	if e != nil {
		t.Fatal(e)
	}
	body, e := store.Open(ctx, key)
	if e != nil {
		t.Fatal(e)
	}
	defer body.Close()
	os.WriteFile(p, bytes.Repeat([]byte("x"), len(data)), 0600)
	future := time.Now().Add(time.Second)
	os.Chtimes(p, future, future)
	if _, _, e = a.resolveObject(ctx, key); e == nil {
		t.Fatal("corrupt fallback accepted")
	}
	if _, e = a.registeredRoot("", t.TempDir()); e == nil {
		t.Fatal("arbitrary root allowed")
	}
}
func TestStorageAddressProbeSafetyAndIdentity(t *testing.T) {
	a := importTestApp(t)
	id := a.siteIdentity()
	srv := httptest.NewServer(httpHandler(func(c *gin.Context) { c.JSON(200, gin.H{"site_id": id}) }))
	defer srv.Close()
	if _, e := a.safeProbe(context.Background(), srv.URL, srv.URL); e != nil {
		t.Fatal(e)
	}
	a.Config.Env = "production"
	if _, e := a.safeProbe(context.Background(), srv.URL, srv.URL); e == nil {
		t.Fatal("production loopback allowed")
	}
	a.Config.Env = "development"
	for _, target := range []string{"http://169.254.169.254/latest/meta-data/", "http://10.0.0.1/", "http://user:pass@127.0.0.1/", "file:///etc/passwd"} {
		if _, e := a.safeProbe(context.Background(), target, srv.URL); e == nil {
			t.Fatal(target)
		}
	}
	cfg := defaultSystemConfig()
	cfg.Site = SiteConfig{Name: "test", WebURL: "https://example.com", APIURL: "http://example.com/api/v1", MediaURL: "https://example.com/media"}
	if validateSystemConfig(cfg) == nil {
		t.Fatal("mixed content accepted")
	}
}
func httpHandler(f gin.HandlerFunc) *gin.Engine { r := gin.New(); r.GET("/", f); return r }
func TestStorageDiskGuardAndMediaAliases(t *testing.T) {
	a := importTestApp(t)
	cfg := defaultSystemConfig()
	cfg.Disk.MinFreeBytes = 1 << 60
	raw, _ := json.Marshal(cfg)
	a.DB.Create(&SystemSetting{SettingKey: "system_config_v3", Value: string(raw), UpdatedAt: time.Now()})
	if e := a.checkUploadDisks(1); e == nil || !strings.Contains(e.Error(), "disk_space") {
		t.Fatal(e)
	}
	r := a.router()
	for _, path := range []string{"/media/999999", "/api/v1/media/999999"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("HEAD", path, nil))
		if w.Code != 404 {
			t.Fatal(path, w.Code)
		}
	}
}
func TestStorageQuotaDefault200MBAndExplicitDelete(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	admin := resourceTestSession(t, a, "admin")
	merchant := resourceTestSession(t, a, "merchant")
	var acc Account
	a.DB.Where("role='merchant'").First(&acc)
	l, e := a.reserveUpload(acc.ID, 200000000)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = a.reserveUpload(acc.ID, 1); e == nil {
		t.Fatal("200 MB boundary exceeded")
	}
	l.Release()
	var imageBytes bytes.Buffer
	png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	m := resourceUpload(t, a, merchant, "/merchant/media", "isolated.png", imageBytes.Bytes(), 201)
	id := uint64(m["id"].(float64))
	if e = a.DB.Exec("INSERT INTO media_references (revision_id,media_id) VALUES (?,?)", 999999, id).Error; e != nil {
		t.Fatal(e)
	}
	tc.call("DELETE", "/admin/media/"+number(m["id"]), admin, gin.H{"confirm": true}, 409)
	a.DB.Exec("DELETE FROM media_references WHERE revision_id=?", 999999)
	tc.call("DELETE", "/admin/media/"+number(m["id"]), merchant, gin.H{"confirm": true}, 403)
	tc.call("DELETE", "/admin/media/"+number(m["id"]), admin, gin.H{"confirm": true}, 200)
	var count int64
	a.DB.Model(&Media{}).Where("id=?", id).Count(&count)
	if count != 0 {
		t.Fatal("unreferenced media retained")
	}
	_, reserved, _, e := a.ownerUsage(a.DB, acc.ID)
	if e != nil || reserved != 0 {
		t.Fatal(reserved, e)
	}
}
func TestStorageDomainChangeKeepsApprovedOldOrigin(t *testing.T) {
	a := importTestApp(t)
	a.Config.PublicBaseURL = "https://old.example"
	tc := testClient{t, a.router()}
	admin := resourceTestSession(t, a, "admin")
	result := tc.call("GET", "/admin/settings", admin, nil, 200)
	raw, _ := json.Marshal(result["config"])
	var cfg SystemConfig
	json.Unmarshal(raw, &cfg)
	cfg.Site.WebURL = "https://new.example"
	cfg.Site.APIURL = "https://api.new.example/api/v1"
	cfg.Site.MediaURL = "https://media.new.example/media"
	proof, _ := json.Marshal(addressProof{Site: cfg.Site, Version: cfg.Version, Expires: time.Now().Add(time.Minute).Unix()})
	cfg.ProbeToken, _ = a.sealSecret(proof)
	tc.call("PUT", "/admin/settings", admin, cfg, 200)
	a.Config.PublicBaseURL = ""
	if !a.allowedSiteOrigin("https://old.example") || !a.allowedSiteOrigin("https://new.example") || a.allowedSiteOrigin("https://unconfirmed.example") {
		t.Fatal("origin allowlist incorrect")
	}
	public := tc.call("GET", "/public/config", "", nil, 200)
	if public["site"].(map[string]any)["web_url"] != "https://new.example" {
		t.Fatal(public)
	}
	cfg.Version++
	cfg.Site.WebURL = "https://unconfirmed.example"
	tc.call("PUT", "/admin/settings", admin, cfg, 400)
}
