package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWatermarkUploadSnapshotLegacyAndQuota(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	token := tc.login("13900007773")
	admin := resourceTestSession(t, a, "admin")
	source := watermarkUpload(t, a, token, "/merchant/media")
	cfg, _ := a.systemConfig()
	cfg.Watermark.Enabled = false
	cfg.Watermark.Opacity = 39
	raw, _ := json.Marshal(cfg)
	a.DB.Exec("INSERT INTO system_settings(setting_key,value,updated_at) VALUES('system_config_v3',?,UTC_TIMESTAMP(3)) ON DUPLICATE KEY UPDATE value=VALUES(value)", string(raw))
	saved := watermarkVendorFixture(t, a, token, []uint64{source})
	marked := uint64(saved["data"].(map[string]any)["media_ids"].([]any)[0].(float64))
	var wm MediaWatermark
	a.DB.First(&wm, "media_id=?", marked)
	var frozen WatermarkConfig
	json.Unmarshal([]byte(wm.ConfigJSON), &frozen)
	if !frozen.Enabled || frozen.Opacity != 15 {
		t.Fatal("upload settings were not frozen", wm)
	}
	// A pre-Schema20 media row is never rewritten, even with the feature enabled.
	legacy := watermarkUpload(t, a, token, "/merchant/media")
	a.DB.Where("media_id=?", legacy).Delete(&MediaWatermark{})
	cfg.Watermark = defaultWatermarkConfig()
	raw, _ = json.Marshal(cfg)
	a.DB.Exec("UPDATE system_settings SET value=? WHERE setting_key='system_config_v3'", string(raw))
	p := Payload{Name: "水印验收农机有限公司", MainProducts: "农机", RegionName: "河北", Phone: "0319-1234567", MediaIDs: []uint64{legacy}}
	next := tc.fixtureCall("PUT", "/merchant/contents/"+number(saved["id"])+"/draft", token, gin.H{"revision_id": saved["revision_id"], "data": p}, 200)
	if next["data"].(map[string]any)["media_ids"].([]any)[0] != float64(legacy) {
		t.Fatal("historical media changed")
	}
	fresh := watermarkUpload(t, a, token, "/merchant/media")
	var media Media
	a.DB.First(&media, fresh)
	var usage int64
	a.DB.Model(&MediaObjectOwner{}).Where("owner_id=?", media.OwnerID).Select("COALESCE(SUM(size),0)").Scan(&usage)
	quota := MerchantStorageQuota{AccountID: media.OwnerID, QuotaBytes: &usage}
	a.DB.Save(&quota)
	p.MediaIDs = []uint64{fresh}
	tc.fixtureCall("PUT", "/merchant/contents/"+number(saved["id"])+"/draft", token, gin.H{"revision_id": next["revision_id"], "data": p}, 409)
	var aliases int64
	a.DB.Model(&MediaWatermark{}).Where("source_media_id=?", fresh).Count(&aliases)
	if aliases != 0 {
		t.Fatal("quota failure left a variant")
	}
	// A disabled snapshot still stays disabled after the switch is enabled again.
	cfg.Watermark.Enabled = false
	raw, _ = json.Marshal(cfg)
	a.DB.Exec("UPDATE system_settings SET value=? WHERE setting_key='system_config_v3'", string(raw))
	a.DB.Where("account_id=?", media.OwnerID).Delete(&MerchantStorageQuota{})
	off := watermarkUpload(t, a, token, "/merchant/media")
	cfg.Watermark.Enabled = true
	raw, _ = json.Marshal(cfg)
	a.DB.Exec("UPDATE system_settings SET value=? WHERE setting_key='system_config_v3'", string(raw))
	p.MediaIDs = []uint64{off}
	next = tc.fixtureCall("PUT", "/merchant/contents/"+number(saved["id"])+"/draft", token, gin.H{"revision_id": next["revision_id"], "data": p}, 200)
	if next["data"].(map[string]any)["media_ids"].([]any)[0] != float64(off) {
		t.Fatal("disabled snapshot changed")
	}
	tc.call("POST", "/merchant/contents/"+number(saved["id"])+"/submit", token, gin.H{"revision_id": next["revision_id"]}, 200)
	tc.call("POST", "/admin/revisions/"+number(next["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
}

func TestWatermarkPublicNameAndOldSourceURL(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	token := tc.login("13900007774")
	admin := resourceTestSession(t, a, "admin")
	vendor := watermarkVendorFixture(t, a, token, nil)
	tc.call("POST", "/merchant/contents/"+number(vendor["id"])+"/submit", token, gin.H{"revision_id": vendor["revision_id"]}, 200)
	tc.call("POST", "/admin/revisions/"+number(vendor["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	source := watermarkUpload(t, a, token, "/merchant/media")
	p := Payload{Name: "尚未审核的新企业名称", MainProducts: "农机", RegionName: "河北", Phone: "0319-1234567", MediaIDs: []uint64{source}, CompanyProfile: &CompanyProfile{ShortName: "尚未审核简称"}}
	next := tc.fixtureCall("PUT", "/merchant/contents/"+number(vendor["id"])+"/draft", token, gin.H{"revision_id": vendor["revision_id"], "data": p}, 200)
	marked := uint64(next["data"].(map[string]any)["media_ids"].([]any)[0].(float64))
	var wm MediaWatermark
	a.DB.First(&wm, "media_id=?", marked)
	if wm.VendorText != "水印农机" {
		t.Fatal("draft replaced public watermark identity", wm)
	}
	request := func(id uint64) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api/v1/media/"+fmt.Sprint(id)+"?thumb=1&download=1&original=1", nil)
		w := httptest.NewRecorder()
		a.router().ServeHTTP(w, r)
		return w
	}
	if request(source).Code != 404 {
		t.Fatal("unpublished source became anonymous")
	}
	tc.call("POST", "/merchant/contents/"+number(vendor["id"])+"/submit", token, gin.H{"revision_id": next["revision_id"]}, 200)
	tc.call("POST", "/admin/revisions/"+number(next["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	alias, original := request(marked), request(source)
	if alias.Code != 200 || original.Code != 200 || string(alias.Body.Bytes()) != string(original.Body.Bytes()) {
		t.Fatal("cached legacy source URL does not resolve to marked display", alias.Code, original.Code)
	}
	var beforeRevision Revision
	a.DB.First(&beforeRevision, vendor["revision_id"])
	var old Payload
	json.Unmarshal([]byte(beforeRevision.Payload), &old)
	if len(old.MediaIDs) != 0 {
		t.Fatal("old snapshot overwritten")
	}
}

func TestWatermarkBrowserHarness(t *testing.T) {
	if os.Getenv("WATERMARK_BROWSER") != "1" {
		t.Skip("explicit disposable browser harness")
	}
	a := importTestApp(t)
	root, _ := filepath.Abs("..")
	a.Config.WebDir = filepath.Join(root, ".local", "watermark-dist")
	admin := resourceTestSession(t, a, "admin")
	tc := testClient{t, a.router()}
	for i, short := range []string{"轻水印农机", "", "宁晋县专用农业机械及零部件制造企业长简称"} {
		token := tc.login("1390000778" + string(rune('0'+i)))
		tc.fixtureCall("POST", "/merchant/contents", token, gin.H{"kind": "vendor", "data": Payload{Name: "水印预览测试企业有限公司" + short, RegionName: "河北", MainProducts: "测试", Phone: "0319-1234567", CompanyProfile: &CompanyProfile{ShortName: short}}}, 201)
	}
	srv := httptest.NewServer(a.router())
	defer srv.Close()
	a.Config.PublicBaseURL = srv.URL
	blob, _ := json.Marshal(gin.H{"base": srv.URL, "admin_token": admin})
	fixture := filepath.Join(root, ".local", "watermark-browser-fixture.json")
	os.WriteFile(fixture, blob, 0600)
	defer os.Remove(fixture)
	cmd := exec.CommandContext(t.Context(), "node", filepath.Join(root, "scripts", "verify-watermark.cjs"))
	cmd.Dir = root
	out, e := cmd.CombinedOutput()
	t.Log(string(out))
	if e != nil {
		t.Fatal(e)
	}
}
