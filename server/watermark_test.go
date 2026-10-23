package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"image"
	"image/color"
	"image/png"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWatermarkRenderingAndAlpha(t *testing.T) {
	cfg := defaultWatermarkConfig()
	if e := validateWatermark(cfg); e != nil {
		t.Fatal(e)
	}
	for _, dark := range []bool{false, true} {
		for _, position := range []string{"top_left", "top_right", "bottom_left", "bottom_right"} {
			cfg.Position = position
			src := watermarkSample(dark, 960, 600)
			out, e := renderWatermark(src, cfg, "宁晋宇浩农机厂")
			if e != nil {
				t.Fatal(e)
			}
			changed := 0
			for y := 0; y < 600; y++ {
				for x := 0; x < 960; x++ {
					a := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
					b := color.NRGBAModel.Convert(out.At(x, y)).(color.NRGBA)
					if a.A != b.A {
						t.Fatal("alpha changed")
					}
					if a != b {
						changed++
						if strings.HasPrefix(position, "bottom") && y < 500 || strings.HasPrefix(position, "top") && y > 100 || strings.HasSuffix(position, "right") && x < 650 || strings.HasSuffix(position, "left") && x > 310 {
							t.Fatal("watermark outside corner")
						}
						if absInt(int(a.R)-int(b.R)) > 39 {
							t.Fatal("watermark exceeded opacity")
						}
					}
				}
			}
			if changed == 0 {
				t.Fatal("watermark missing")
			}
		}
	}
	for _, size := range []image.Point{{80, 50}, {240, 900}, {300, 300}} {
		src := image.NewNRGBA(image.Rect(0, 0, size.X, size.Y))
		for y := 0; y < size.Y; y++ {
			for x := 0; x < size.X; x++ {
				src.SetNRGBA(x, y, color.NRGBA{220, 220, 220, uint8(x % 256)})
			}
		}
		out, e := renderWatermark(src, cfg, strings.Repeat("长名称", 70))
		if e != nil {
			t.Fatal(e)
		}
		for y := 0; y < size.Y; y++ {
			for x := 0; x < size.X; x++ {
				if color.NRGBAModel.Convert(out.At(x, y)).(color.NRGBA).A != src.NRGBAAt(x, y).A {
					t.Fatal("PNG transparency changed")
				}
			}
		}
	}
	if os.Getenv("WATERMARK_CAPTURE") == "1" {
		for _, dark := range []bool{false, true} {
			name := "light"
			if dark {
				name = "dark"
			}
			img, e := renderWatermark(watermarkSample(dark, 1600, 1000), defaultWatermarkConfig(), "宁晋宇浩农机厂")
			if e != nil {
				t.Fatal(e)
			}
			dir := filepath.Join("..", "docs", "screenshots", "watermark")
			os.MkdirAll(dir, 0755)
			f, e := os.Create(filepath.Join(dir, "effect-"+name+".png"))
			if e != nil {
				t.Fatal(e)
			}
			png.Encode(f, img)
			f.Close()
		}
		for _, sample := range []struct {
			name, vendor string
			w, h         int
			transparent  bool
		}{
			{"short", "宇浩农机", 1600, 1000, false}, {"long", "宁晋县农业机械及专用零部件制造有限公司超长名称", 1600, 1000, false},
			{"full-name", "未填简称的农机制造有限公司", 1600, 1000, false}, {"portrait", "示例农机厂", 600, 1000, false},
			{"small", "示例农机厂", 120, 80, false}, {"thumbnail", "示例农机厂", 480, 300, false}, {"transparent", "示例农机厂", 960, 600, true},
		} {
			src := watermarkSample(false, sample.w, sample.h)
			if sample.transparent {
				n := src.(*image.NRGBA)
				for y := 0; y < sample.h; y++ {
					for x := 0; x < sample.w; x++ {
						v := n.NRGBAAt(x, y)
						if y < sample.h/4 || y > sample.h*4/5 {
							v.A = 100
							n.SetNRGBA(x, y, v)
						}
					}
				}
			}
			out, e := renderWatermark(src, defaultWatermarkConfig(), sample.vendor)
			if e != nil {
				t.Fatal(e)
			}
			f, e := os.Create(filepath.Join("..", "docs", "screenshots", "watermark", "sample-"+sample.name+".png"))
			if e != nil {
				t.Fatal(e)
			}
			if e = png.Encode(f, out); e != nil {
				t.Fatal(e)
			}
			f.Close()
		}

	}
}
func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
func watermarkUpload(t *testing.T, a *App, token, path string) uint64 {
	t.Helper()
	var imageBytes bytes.Buffer
	png.Encode(&imageBytes, watermarkSample(false, 960, 600))
	r := resourceUpload(t, a, token, path, "fixture.png", imageBytes.Bytes(), 201)
	return uint64(r["id"].(float64))
}
func watermarkVendorFixture(t *testing.T, a *App, token string, ids []uint64) map[string]any {
	return (testClient{t, a.router()}).fixtureCall("POST", "/merchant/contents", token, gin.H{"kind": "vendor", "data": Payload{Name: "水印验收农机有限公司", RegionName: "河北", MainProducts: "农机", Phone: "0319-1234567", MediaIDs: ids, CompanyProfile: &CompanyProfile{ShortName: "水印农机"}}}, 201)
}
func TestWatermarkSnapshotVariantsAndPublicProtection(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	token := tc.login("13900007771")
	admin := resourceTestSession(t, a, "admin")
	source := watermarkUpload(t, a, token, "/merchant/media")
	var clean Media
	a.DB.First(&clean, source)
	before, _, _ := fileSHA(filepath.Join(a.Config.MediaDir, clean.Path))
	p := Payload{Name: "水印验收农机有限公司", RegionName: "河北", MainProducts: "农机", Phone: "0319-1234567", MediaIDs: []uint64{source}, CompanyProfile: &CompanyProfile{ShortName: "水印农机", LogoMediaID: source, CoverMediaID: source}}
	saved := tc.fixtureCall("POST", "/merchant/contents", token, gin.H{"kind": "vendor", "data": p}, 201)
	data := saved["data"].(map[string]any)
	marked := uint64(data["media_ids"].([]any)[0].(float64))
	if marked == source {
		t.Fatal("no independent variant")
	}
	profile := data["company_profile"].(map[string]any)
	if profile["logo_media_id"] != float64(source) || profile["cover_media_id"] != float64(marked) {
		t.Fatal("mixed role resolution", profile)
	}
	var info MediaWatermark
	a.DB.First(&info, "media_id=?", marked)
	if info.VendorText != "水印农机" || info.State != "ready" {
		t.Fatal(info)
	}
	var rendered Media
	a.DB.First(&rendered, marked)
	after, _, _ := fileSHA(filepath.Join(a.Config.MediaDir, clean.Path))
	derived, _, _ := fileSHA(filepath.Join(a.Config.MediaDir, rendered.Path))
	if before != after || before == derived {
		t.Fatal("source overwritten or watermark missing")
	}
	tc.call("POST", "/merchant/contents/"+number(saved["id"])+"/submit", token, gin.H{"revision_id": saved["revision_id"]}, 200)
	tc.call("POST", "/admin/revisions/"+number(saved["revision_id"])+"/review", admin, gin.H{"decision": "approve", "note": "isolated watermark"}, 200)
	request := httptest.NewRequest("GET", "/api/v1/media/"+fmt.Sprint(marked)+"?original=1", nil)
	w := httptest.NewRecorder()
	tc.router.ServeHTTP(w, request)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	originalBytes, _ := os.ReadFile(filepath.Join(a.Config.MediaDir, clean.Path))
	if bytes.Equal(w.Body.Bytes(), originalBytes) {
		t.Fatal("query bypassed watermark")
	}
	// Re-saving the same source keeps the same variant and frozen vendor text.
	p.CompanyProfile.ShortName = "新简称"
	again := tc.fixtureCall("PUT", "/merchant/contents/"+number(saved["id"])+"/draft", token, gin.H{"revision_id": saved["revision_id"], "data": p}, 200)
	if again["data"].(map[string]any)["media_ids"].([]any)[0] != float64(marked) {
		t.Fatal("duplicate watermark")
	}
	cfg, _ := a.systemConfig()
	cfg.Watermark.Enabled = false
	raw, _ := json.Marshal(cfg)
	a.DB.Exec("INSERT INTO system_settings(setting_key,value,updated_at) VALUES('system_config_v3',?,UTC_TIMESTAMP(3)) ON DUPLICATE KEY UPDATE value=VALUES(value)", string(raw))
	fresh := watermarkUpload(t, a, token, "/merchant/media")
	p.MediaIDs = []uint64{fresh}
	p.CompanyProfile.CoverMediaID = 0
	disabled := tc.fixtureCall("PUT", "/merchant/contents/"+number(saved["id"])+"/draft", token, gin.H{"revision_id": again["revision_id"], "data": p}, 200)
	if disabled["data"].(map[string]any)["media_ids"].([]any)[0] != float64(fresh) {
		t.Fatal("disabled upload marked")
	}
	// Source files remain protected while referenced by derivatives.
	tc.call("DELETE", "/admin/media/"+fmt.Sprint(source), admin, gin.H{"confirm": true}, 409)
}
func TestWatermarkSettingsPreviewCompatibilityAndRollback(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	admin := resourceTestSession(t, a, "admin")
	merchant := tc.login("13900007772")
	settings := tc.call("GET", "/admin/settings", admin, nil, 200)["config"].(map[string]any)
	wm := settings["watermark"].(map[string]any)
	if wm["opacity"] != float64(15) || wm["enabled"] != true {
		t.Fatal(wm)
	}
	wm["opacity"] = float64(12)
	tc.call("PUT", "/admin/settings", admin, settings, 200)
	current := tc.call("GET", "/admin/settings", admin, nil, 200)["config"].(map[string]any)
	delete(current, "watermark")
	tc.call("PUT", "/admin/settings", admin, current, 200)
	cfg, _ := a.systemConfig()
	if cfg.Watermark.Opacity != 12 {
		t.Fatal("old request cleared settings")
	}
	tc.call("POST", "/admin/settings/watermark/preview", merchant, gin.H{"watermark": defaultWatermarkConfig()}, 403)
	out := resetRequest(a.router(), "/admin/settings/watermark/preview", admin, gin.H{"watermark": defaultWatermarkConfig(), "dark": true})
	if out.Code != 200 || !strings.HasPrefix(out.Header().Get("Content-Type"), "image/png") {
		t.Fatal(out.Code, out.Body.String())
	}
	if _, e := png.Decode(bytes.NewReader(out.Body.Bytes())); e != nil {
		t.Fatal(e)
	}
	bad := defaultWatermarkConfig()
	bad.Opacity = 41
	tc.call("POST", "/admin/settings/watermark/preview", admin, gin.H{"watermark": bad}, 400)
	source := watermarkUpload(t, a, merchant, "/merchant/media")
	var before int64
	a.DB.Model(&StoredObject{}).Count(&before)
	a.DB.Callback().Create().Before("gorm:create").Register("watermark_save_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "revisions" {
			tx.AddError(gorm.ErrInvalidData)
		}
	})
	tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "故障测试", MainProducts: "农机", RegionName: "河北", Phone: "13900007772", MediaIDs: []uint64{source}}}, 500)
	a.DB.Callback().Create().Remove("watermark_save_failure")
	var after int64
	a.DB.Model(&StoredObject{}).Count(&after)
	if before != after {
		t.Fatal("object DB rollback failed")
	}
	entries, _ := os.ReadDir(a.Config.MediaDir)
	if len(entries) != int(before) {
		t.Fatal("orphaned derivative files", len(entries), before)
	}
	if e := migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	a.DB.Exec("DELETE FROM schema_migrations WHERE version=20")
	if e := migrate(a.DB); e != nil {
		t.Fatal(e)
	}
}
