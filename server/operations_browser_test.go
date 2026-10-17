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
	"path/filepath"
	"testing"
	"time"
)

func TestOperationsBrowserFixture(t *testing.T) {
	if os.Getenv("OPERATIONS_BROWSER_FIXTURE") != "1" {
		t.Skip("explicit isolated browser fixture only")
	}
	name := fmt.Sprintf("zqzt_exhibition_test_operations_browser_%d", time.Now().UnixNano())
	db, err := openDB(name)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { pool, _ := db.DB(); pool.Close() }()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADMIN_PASSWORD", "Browser-Only-Operations-2026!")
	if err = seed(db); err != nil {
		t.Fatal(err)
	}
	root, _ := filepath.Abs("..")
	dir := filepath.Join(root, ".local", "operations-browser-"+randomToken()[:10])
	media := filepath.Join(dir, "media")
	os.MkdirAll(media, 0700)
	a := &App{DB: db, Config: Config{Env: "development", MediaDir: media, WebDir: filepath.Join(root, "web", "dist"), SMSProvider: "development"}, SMS: DevelopmentSMS{}}
	tc := testClient{t, a.router()}
	login := tc.call("POST", "/auth/admin", "", gin.H{"username": "admin", "password": "Browser-Only-Operations-2026!"}, 200)
	admin := login["token"].(string)
	merchant := resourceTestSession(t, a, "merchant")
	vendor := tc.call("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "运营升级验收厂商（仅测试）", MainProducts: "测试机械", RegionName: "河北", Phone: "0319-1234567", Wechat: "fixture-only"}}, 201)
	var photo bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 180, 120))
	for y := 0; y < 120; y++ {
		for x := 0; x < 180; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 150, 255})
		}
	}
	png.Encode(&photo, img)
	upload := resourceUpload(t, a, merchant, "/merchant/media", "fixture.png", photo.Bytes(), 201)
	product := tc.call("POST", "/merchant/contents", merchant, gin.H{"kind": "product", "data": Payload{Name: "运营测试旋耕机", CategoryID: 1, MediaIDs: []uint64{uint64(upload["id"].(float64))}}}, 201)
	for _, item := range []map[string]any{vendor, product} {
		tc.call("POST", "/merchant/contents/"+number(item["id"])+"/submit", merchant, gin.H{"revision_id": item["revision_id"]}, 200)
		tc.call("POST", "/admin/revisions/"+number(item["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	}
	pdf := resourceUpload(t, a, merchant, "/merchant/resource-media?kind=pdf", "catalog.pdf", tinyPDF(), 201)
	resource := tc.call("POST", "/merchant/resources", merchant, gin.H{"kind": "catalog", "data": ResourcePayload{Title: "运营测试图册", Format: "pdf", FileMediaID: uint64(pdf["media_id"].(float64))}}, 201)
	tc.call("POST", "/merchant/resources/"+number(resource["id"])+"/submit", merchant, gin.H{"revision_id": resource["revision_id"]}, 200)
	tc.call("POST", "/admin/resource-revisions/"+number(resource["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	fixture := gin.H{"base": "http://127.0.0.1:18096", "database": name, "media_directory": media, "control_directory": a.operations().dir, "admin_token": admin, "merchant_token": merchant, "vendor_id": vendor["id"], "product_id": product["id"], "resource_id": resource["id"], "image_id": upload["id"]}
	data, _ := json.Marshal(fixture)
	if err = os.WriteFile(filepath.Join(root, ".local", "operations-browser-fixture.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}
