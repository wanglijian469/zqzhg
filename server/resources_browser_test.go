package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

// Explicit opt-in creates a persistent isolated fixture for real browser verification.
func TestResourceBrowserFixture(t *testing.T) {
	if os.Getenv("RESOURCE_BROWSER_FIXTURE") != "1" {
		t.Skip("browser fixture is opt-in")
	}
	name := fmt.Sprintf("zqzt_exhibition_test_resource_browser_%d", time.Now().UnixNano())
	db, e := openDB(name)
	if e != nil {
		t.Fatal(e)
	}
	if e = migrate(db); e != nil {
		t.Fatal(e)
	}
	t.Setenv("ADMIN_PASSWORD", "Browser-Only-Resource-2026!")
	if e = seed(db); e != nil {
		t.Fatal(e)
	}
	root, _ := filepath.Abs("..")
	media := filepath.Join(root, ".local", "resource-browser-media")
	os.MkdirAll(media, 0700)
	a := &App{DB: db, Config: Config{Env: "development", SMSProvider: "development", MediaDir: media, WebDir: filepath.Join(root, "web", "dist")}, SMS: DevelopmentSMS{}}
	tc := testClient{t, a.router()}
	merchant := resourceTestSession(t, a, "merchant")
	admin := resourceTestSession(t, a, "admin")
	vendor := tc.call("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "多媒体验收厂商（仅测试）", NameEN: "Multimedia QA", MainProducts: "测试农机", RegionName: "河北", Phone: "13900009910", Description: "这是独立测试库中的企业介绍。", CompanyProfile: &CompanyProfile{Capacity: "每月100台（仅测试）", Equipment: "测试生产设备", Established: "2020", Staff: "50人"}}}, 201)
	product := tc.call("POST", "/merchant/contents", merchant, gin.H{"kind": "product", "data": Payload{Name: "测试农机展品", CategoryID: 1}}, 201)
	for _, item := range []map[string]any{vendor, product} {
		tc.call("POST", "/merchant/contents/"+number(item["id"])+"/submit", merchant, gin.H{"revision_id": item["revision_id"]}, 200)
		tc.call("POST", "/admin/revisions/"+number(item["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	}
	var images []uint64
	for _, col := range []color.Color{color.RGBA{R: 30, G: 120, B: 60, A: 255}, color.RGBA{R: 50, G: 90, B: 160, A: 255}} {
		img := image.NewRGBA(image.Rect(0, 0, 300, 420))
		for y := 0; y < 420; y++ {
			for x := 0; x < 300; x++ {
				img.Set(x, y, col)
			}
		}
		var b bytes.Buffer
		png.Encode(&b, img)
		r := resourceUpload(t, a, merchant, "/merchant/media", "page.png", b.Bytes(), 201)
		images = append(images, uint64(r["id"].(float64)))
	}
	pdf := resourceUpload(t, a, merchant, "/merchant/resource-media?kind=pdf", "catalog.pdf", tinyPDF(), 201)
	input := filepath.Join(media, "fixture.mov")
	if output, e := exec.Command(videoTool("ffmpeg"), "-nostdin", "-y", "-v", "error", "-f", "lavfi", "-i", "testsrc=size=320x180:rate=15:duration=2", "-c:v", "libx264", "-pix_fmt", "yuv420p", input).CombinedOutput(); e != nil {
		t.Fatal(e, string(output))
	}
	video, _ := os.ReadFile(input)
	uploaded := resourceUpload(t, a, merchant, "/merchant/resource-media?kind=video", "factory.mov", video, 202)
	a.processNextVideo(t.Context())
	os.Remove(input)
	ids := map[string]uint64{}
	for _, v := range []struct {
		kind, key string
		p         ResourcePayload
	}{{"video", "video", ResourcePayload{Title: "生产现场视频（仅测试）", Purpose: "product", ProductID: uint64(product["id"].(float64)), FileMediaID: uint64(uploaded["media_id"].(float64))}}, {"catalog", "pdf", ResourcePayload{Title: "PDF电子图册（仅测试）", Format: "pdf", FileMediaID: uint64(pdf["media_id"].(float64)), ProductID: uint64(product["id"].(float64))}}, {"catalog", "images", ResourcePayload{Title: "图片电子图册（仅测试）", Format: "images", PageMediaIDs: images}}} {
		r := tc.call("POST", "/merchant/resources", merchant, gin.H{"kind": v.kind, "data": v.p}, 201)
		tc.call("POST", "/merchant/resources/"+number(r["id"])+"/submit", merchant, gin.H{"revision_id": r["revision_id"]}, 200)
		tc.call("POST", "/admin/resource-revisions/"+number(r["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
		ids[v.key] = uint64(r["id"].(float64))
	}
	fixture := gin.H{"database": name, "media_directory": media, "base": "http://localhost:8081", "merchant_token": merchant, "admin_token": admin, "resources": ids, "vendor_id": vendor["id"], "product_id": product["id"]}
	data, _ := json.Marshal(fixture)
	if e = os.WriteFile(filepath.Join(root, ".local", "resource-browser-fixture.json"), data, 0600); e != nil {
		t.Fatal(e)
	}
}

func TestCleanupResourceBrowserFixture(t *testing.T) {
	if os.Getenv("RESOURCE_BROWSER_CLEANUP") != "1" {
		t.Skip("fixture cleanup is opt-in")
	}
	data, e := os.ReadFile(filepath.Join("..", ".local", "resource-browser-fixture.json"))
	if e != nil {
		t.Fatal(e)
	}
	var f struct {
		Database string `json:"database"`
	}
	if json.Unmarshal(data, &f) != nil || !regexp.MustCompile(`^zqzt_exhibition_test_resource_browser_[0-9]+$`).MatchString(f.Database) {
		t.Fatal("invalid isolated fixture name")
	}
	db, e := openDB(f.Database)
	if e != nil {
		t.Fatal(e)
	}
	if e = db.Exec("DROP DATABASE `" + f.Database + "`").Error; e != nil {
		t.Fatal(e)
	}
}
