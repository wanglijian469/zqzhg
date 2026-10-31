package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSearchAdminBrowser(t *testing.T) {
	if os.Getenv("RUN_SEARCH_BROWSER") != "1" {
		t.Skip("enable after candidate Web build")
	}
	a := importTestApp(t)
	root, e := filepath.Abs("..")
	mustSearch(t, e)
	a.Config.WebDir = filepath.Join(root, ".local/search-seo/web")
	a.Config.PublicBaseURL = "https://seo.example"
	v := v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, "SEO自动提交验收厂家（隔离数据）")
	var calls atomic.Int32
	a.searchHTTP = &http.Client{Transport: searchRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"success":1,"remain":99}`)), Request: r}, nil
	})}
	tc, token := searchAdmin(t, a)
	_ = tc
	srv := httptest.NewServer(a.router())
	defer srv.Close()
	fixture := filepath.Join(root, ".local/search-seo/browser-fixture.json")
	raw, _ := json.Marshal(map[string]any{"base": srv.URL, "token": token, "company_url": contentSearchURL(searchBase(a), v)})
	mustSearch(t, os.WriteFile(fixture, raw, 0600))
	defer os.Remove(fixture)
	cmd := exec.CommandContext(t.Context(), "node", filepath.Join(root, "scripts/verify-search-admin.cjs"), fixture)
	cmd.Dir = root
	out, e := cmd.CombinedOutput()
	t.Log(string(out))
	if e != nil {
		t.Fatal(e)
	}
	if calls.Load() != 1 {
		t.Fatal(fmt.Sprintf("Expected one mocked request, got %d", calls.Load()))
	}
}
