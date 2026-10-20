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

func TestCompanyOnboardingBrowserHarness(t *testing.T) {
	if os.Getenv("COMPANY_BROWSER_HARNESS") != "1" {
		t.Skip("explicit disposable browser harness")
	}
	a := importTestApp(t)
	root, _ := filepath.Abs("..")
	a.Config.WebDir = filepath.Join(root, "web", "dist")
	admin := resourceTestSession(t, a, "admin")
	account := Account{Username: "legacy-company-" + randomToken()[:12], Role: "merchant"}
	a.DB.Create(&account)
	session, e := a.newSession(account)
	if e != nil {
		t.Fatal(e)
	}
	merchant := session["token"].(string)
	tc := testClient{t, a.router()}
	legacy := tc.call("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "历史资料验收企业（隔离测试）", MainProducts: "测试机械", RegionName: "旧地区文字（测试）", Phone: "0319-1234567", NameEN: "Legacy company test", Description: "已有真实资料回填测试", CompanyProfile: &CompanyProfile{Summary: "原宣传摘要", SummaryEN: "Existing short summary", Contacts: []CompanyContact{{Area: "测试区域", AreaEN: "Test area", Name: "测试联系人", NameEN: "Test contact", Phone: "0319-7654321"}}}}}, 201)
	tc.call("POST", "/merchant/contents/"+number(legacy["id"])+"/submit", merchant, gin.H{"revision_id": legacy["revision_id"]}, 200)
	tc.call("POST", "/admin/revisions/"+number(legacy["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	pic := image.NewRGBA(image.Rect(0, 0, 640, 480))
	draw.Draw(pic, pic.Bounds(), &image.Uniform{C: color.RGBA{243, 247, 251, 255}}, image.Point{}, draw.Src)
	draw.Draw(pic, image.Rect(140, 100, 500, 380), &image.Uniform{C: color.RGBA{48, 106, 161, 255}}, image.Point{}, draw.Src)
	var pngData bytes.Buffer
	png.Encode(&pngData, pic)
	sample := filepath.Join(root, ".local", "company-isolated-image.png")
	os.WriteFile(sample, pngData.Bytes(), 0600)
	defer os.Remove(sample)
	srv := httptest.NewServer(a.router())
	defer srv.Close()
	a.Config.PublicBaseURL = srv.URL
	file := filepath.Join(root, ".local", "company-browser-fixture.json")
	done := filepath.Join(root, ".local", "company-browser-done")
	os.Remove(done)
	defer os.Remove(file)
	defer os.Remove(done)
	blob, _ := json.Marshal(map[string]any{"base": srv.URL, "admin_token": admin, "legacy_token": merchant, "legacy_account_id": account.ID, "legacy_id": legacy["id"], "sample_path": sample})
	if e := os.WriteFile(file, blob, 0600); e != nil {
		t.Fatal(e)
	}
	if os.Getenv("RUN_AUTOMATED_UPGRADE_BROWSER") == "1" {
		command := exec.CommandContext(t.Context(), "node", filepath.Join(root, "scripts", "verify-company-onboarding.cjs"))
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
			t.Fatal("company browser harness timed out")
		case <-tick.C:
			if _, e := os.Stat(done); e == nil {
				return
			}
		}
	}
}
