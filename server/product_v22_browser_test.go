package main

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestProductV22BrowserHarness(t *testing.T) {
	if os.Getenv("V22_BROWSER_HARNESS") != "1" {
		t.Skip("explicit disposable browser harness")
	}
	a := importTestApp(t)
	root, _ := filepath.Abs("..")
	a.Config.WebDir = filepath.Join(root, "web", "dist")
	merchantAccount := Account{Username: "v22-browser-" + randomToken()[:12], Role: "merchant"}
	if e := a.DB.Create(&merchantAccount).Error; e != nil {
		t.Fatal(e)
	}
	session, e := a.newSession(merchantAccount)
	if e != nil {
		t.Fatal(e)
	}
	merchant := session["token"].(string)
	admin := resourceTestSession(t, a, "admin")
	tc := testClient{t, a.router()}
	vendor := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "V2.2隔离验收企业（测试）", MainProducts: "测试产品", RegionName: "测试地区", Phone: "0319-1234567"}}, 201)
	tc.call("POST", "/merchant/contents/"+number(vendor["id"])+"/submit", merchant, gin.H{"revision_id": vendor["revision_id"]}, 200)
	tc.call("POST", "/admin/revisions/"+number(vendor["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	var mids []uint64
	for _, c := range []color.RGBA{{45, 105, 160, 255}, {125, 160, 185, 255}} {
		pic := image.NewRGBA(image.Rect(0, 0, 640, 480))
		draw.Draw(pic, pic.Bounds(), &image.Uniform{C: color.RGBA{245, 248, 252, 255}}, image.Point{}, draw.Src)
		draw.Draw(pic, image.Rect(160, 120, 480, 360), &image.Uniform{C: c}, image.Point{}, draw.Src)
		var buf bytes.Buffer
		png.Encode(&buf, pic)
		m := resourceUpload(t, a, merchant, "/merchant/media", "isolated-sample.png", buf.Bytes(), 201)
		mids = append(mids, uint64(m["id"].(float64)))
	}
	product := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "product", "data": Payload{Name: "旧产品回填测试 V22-440", NameEN: "Legacy test product V22-440", CategoryID: 1, Description: "已有产品说明（隔离测试）", DescriptionEN: "Existing test description", Parameters: "自定义参数=保留原文\n型号：V22-440", MediaIDs: mids}}, 201)
	srv := httptest.NewServer(a.router())
	defer srv.Close()
	a.Config.PublicBaseURL = srv.URL
	file := filepath.Join(root, ".local", "v22-browser-fixture.json")
	done := filepath.Join(root, ".local", "v22-browser-done")
	os.Remove(done)
	defer os.Remove(file)
	defer os.Remove(done)
	blob, _ := json.Marshal(map[string]any{"base": srv.URL, "token": merchant, "account_id": merchantAccount.ID, "admin_token": admin, "product_id": product["id"], "media_ids": mids})
	if e := os.WriteFile(file, blob, 0600); e != nil {
		t.Fatal(e)
	}
	if os.Getenv("RUN_AUTOMATED_UPGRADE_BROWSER") == "1" {
		command := exec.CommandContext(t.Context(), "node", filepath.Join(root, "scripts", "verify-product-v22.cjs"))
		command.Dir = root
		output, err := command.CombinedOutput()
		t.Log(string(output))
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	deadline := time.After(12 * time.Minute)
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("V22 browser harness timed out")
		case <-tick.C:
			if _, e := os.Stat(done); e == nil {
				return
			}
		}
	}
}
