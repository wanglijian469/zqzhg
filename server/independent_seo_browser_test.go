package main

import (
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIndependentSEOBrowser(t *testing.T) {
	if os.Getenv("RUN_SEO_BROWSER") != "1" {
		t.Skip("set RUN_SEO_BROWSER=1 with RUN_MYSQL_TESTS=1 after building web")
	}
	a := importTestApp(t)
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	a.Config.WebDir = filepath.Join(root, "web", "dist")
	if _, err = os.Stat(filepath.Join(a.Config.WebDir, "index.html")); err != nil {
		t.Fatal("build web first", err)
	}
	v := v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, "SEO验收企业（隔离测试）")
	a.DB.Model(&v).Update("showroom_slug", "seo-browser")
	seoSetPayload(t, a, v, Payload{Name: "SEO验收企业（隔离测试）", MainProducts: "验收机械", RegionName: "河北省邢台市", Address: "验收工业园", Phone: "0319-1234567", CompanyProfile: &CompanyProfile{Summary: "无需执行脚本也能读取的企业介绍"}})
	p := v2Published(t, a, "product", v.ID, 1, 1, 1, false, 0, "SEO验收产品（隔离测试）")
	seoSetPayload(t, a, p, Payload{Name: "SEO验收产品（隔离测试）", Model: "SEO-800", Parameters: "型号：SEO-800\n功率：80千瓦", Description: "无需执行脚本也能读取的产品介绍"})
	srv := httptest.NewServer(a.router())
	defer srv.Close()
	a.Config.PublicBaseURL = srv.URL
	cmd := exec.CommandContext(t.Context(), "node", filepath.Join(root, "scripts", "verify-independent-seo.cjs"), srv.URL, fmt.Sprintf("/products/%d", p.ID))
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
}
