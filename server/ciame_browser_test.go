package main

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCIAMEBrowserAcceptance(t *testing.T) {
	if os.Getenv("CIAME_BROWSER") != "1" {
		t.Skip("explicit isolated browser acceptance")
	}
	t.Setenv("PROCUREMENT_ENABLED", "false")
	a := importTestApp(t)
	root, _ := filepath.Abs("..")
	a.Config.WebDir = filepath.Join(root, "web", "dist")
	admin := resourceTestSession(t, a, "admin")
	merchant := resourceTestSession(t, a, "merchant")
	tc := testClient{t, a.router()}
	v := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "CIAME隔离验收厂商（测试）", MainProducts: "耕整地机械及零部件", RegionName: "河北省 邢台市 宁晋县", Phone: "0319-1234567", Description: "用于本次本机验收的隔离样本。"}}, 201)
	tc.call("POST", "/merchant/contents/"+number(v["id"])+"/submit", merchant, gin.H{"revision_id": v["revision_id"]}, 200)
	tc.call("POST", "/admin/revisions/"+number(v["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	p := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "product", "vendor_id": v["id"], "data": Payload{Name: "CIAME验收产品（测试）", Description: "隔离产品资料", Parameters: "验收型号 T-01"}}, 201)
	tc.call("POST", "/merchant/contents/"+number(p["id"])+"/submit", merchant, gin.H{"revision_id": p["revision_id"]}, 200)
	tc.call("POST", "/admin/revisions/"+number(p["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	srv := httptest.NewServer(a.router())
	defer srv.Close()
	a.Config.PublicBaseURL = srv.URL
	fixture := filepath.Join(root, ".local", "ciame-browser-fixture.json")
	blob, _ := json.Marshal(gin.H{"base": srv.URL, "admin_token": admin, "vendor_id": v["id"], "product_id": p["id"]})
	if e := os.WriteFile(fixture, blob, 0600); e != nil {
		t.Fatal(e)
	}
	defer os.Remove(fixture)
	script := "verify-ciame-upgrade.cjs"
	if os.Getenv("CIAME_COMPACT_LAYOUT") == "1" {
		script = "verify-compact-layout.cjs"
	}
	cmd := exec.CommandContext(t.Context(), "node", filepath.Join(root, "scripts", script))
	cmd.Dir = root
	output, e := cmd.CombinedOutput()
	t.Log(string(output))
	if e != nil {
		t.Fatal(e)
	}
}
