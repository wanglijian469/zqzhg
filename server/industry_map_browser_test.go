package main

import (
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Opt-in real browser verification; the HTTP server, DB and media are disposable.
func TestIndustryMapBrowser(t *testing.T) {
	if os.Getenv("RUN_INDUSTRY_MAP_BROWSER") != "1" {
		t.Skip("set RUN_INDUSTRY_MAP_BROWSER=1 with RUN_MYSQL_TESTS=1")
	}
	a := importTestApp(t)
	root, e := filepath.Abs("..")
	if e != nil {
		t.Fatal(e)
	}
	a.Config.WebDir = env("BROWSER_WEB_DIR", filepath.Join(root, "web", "dist"))
	if _, e = os.Stat(filepath.Join(a.Config.WebDir, "index.html")); e != nil {
		t.Fatal("build web before browser verification", e)
	}
	for i := 0; i < 24; i++ {
		industryFixture(t, a, Payload{Name: fmt.Sprintf("产业导航验收企业%02d（仅测试）", i), MainProducts: "农机装备、制造配件（隔离验收）", RegionCode: "130528", CategoryID: 1}, 1)
	}
	industryFixture(t, a, Payload{Name: "市级资料验收企业（仅测试）", RegionCode: "1305", MainProducts: "机械配件"}, 1)
	industryFixture(t, a, Payload{Name: "北京验收企业（仅测试）", RegionCode: "110108"}, 1)
	industryFixture(t, a, Payload{Name: "仙桃验收企业（仅测试）", RegionCode: "429004"}, 1)
	industryFixture(t, a, Payload{Name: "地区待完善验收企业（仅测试）", RegionName: "历史未核实地区（仅测试）"}, 1)
	server := httptest.NewServer(a.router())
	defer server.Close()
	command := exec.CommandContext(t.Context(), "node", filepath.Join(root, "scripts", "verify-industry-map-browser.cjs"), server.URL)
	command.Dir = root
	output, e := command.CombinedOutput()
	t.Log(string(output))
	if e != nil {
		t.Fatal(e)
	}
}

// Explicit opt-in: real source geometry remains in the private ignored cache.
func TestMapPreviewBrowser(t *testing.T) {
	if os.Getenv("RUN_MAP_PREVIEW_BROWSER") != "1" {
		t.Skip("set RUN_MAP_PREVIEW_BROWSER=1 with RUN_MYSQL_TESTS=1")
	}
	a := importTestApp(t)
	root, e := filepath.Abs("..")
	if e != nil {
		t.Fatal(e)
	}
	a.Config.WebDir = env("BROWSER_WEB_DIR", filepath.Join(root, "web", "dist"))
	a.Config.Env = "development"
	a.Config.MapPreviewEnabled = true
	a.Config.MapPreviewDir = filepath.Join(root, ".local", "map-preview")
	for i := 0; i < 24; i++ {
		industryFixture(t, a, Payload{Name: fmt.Sprintf("产业导航验收企业%02d（仅测试）", i), MainProducts: "农机装备（隔离测试）", RegionCode: "130528", CategoryID: 1}, 1)
	}
	industryFixture(t, a, Payload{Name: "市级资料验收企业（仅测试）", RegionCode: "1305"}, 1)
	industryFixture(t, a, Payload{Name: "北京验收企业（仅测试）", RegionCode: "110108"}, 1)
	industryFixture(t, a, Payload{Name: "仙桃验收企业（仅测试）", RegionCode: "429004"}, 1)
	industryFixture(t, a, Payload{Name: "地区待完善验收企业（仅测试）", RegionName: "历史未核实地区（仅测试）"}, 1)
	server := httptest.NewServer(a.router())
	defer server.Close()
	command := exec.CommandContext(t.Context(), "node", filepath.Join(root, "scripts", "verify-map-outline-browser.cjs"), server.URL)
	command.Dir = root
	output, e := command.CombinedOutput()
	t.Log(string(output))
	if e != nil {
		t.Fatal(e)
	}
}
