package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdmissionGuideHTMLAndSitemap(t *testing.T) {
	a := importTestApp(t)
	seoTestIndex(t, a)
	a.Config.PublicBaseURL = "https://guide.example"
	router := a.router()
	for _, method := range []string{"GET", "HEAD"} {
		for _, path := range []string{"/guide", "/guide?hall=cable&utm_source=test"} {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(method, path, nil))
			if w.Code != 200 || !strings.Contains(w.Body.String(), "厂商入驻与使用指南") || !strings.Contains(w.Body.String(), "href='https://guide.example/guide'") || !strings.Contains(w.Body.String(), "content='index,follow'") {
				t.Fatal(method, path, w.Code, w.Body.String())
			}
		}
	}
	if w := seoGet(router, "/guide/unknown"); w.Code != 404 {
		t.Fatal("unknown guide route must remain 404", w.Code)
	}
	if w := seoGet(router, "/sitemaps/pages.xml"); w.Code != 200 || !strings.Contains(w.Body.String(), "https://guide.example/guide</loc>") {
		t.Fatal(w.Code, w.Body.String())
	}
}

// All account, content and review writes are confined to importTestApp's disposable database.
// Screenshot capture is explicit; no real SMS provider or production server is used.
func TestAdmissionGuideBrowser(t *testing.T) {
	if os.Getenv("RUN_GUIDE_BROWSER") != "1" {
		t.Skip("set RUN_GUIDE_BROWSER=1 with RUN_MYSQL_TESTS=1")
	}
	a := importTestApp(t)
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	a.Config.WebDir = filepath.Join(root, "web", "dist")
	admin := resourceTestSession(t, a, "admin")
	srv := httptest.NewServer(a.router())
	defer srv.Close()
	a.Config.PublicBaseURL = srv.URL
	fixture := filepath.Join(root, ".local", "guide", "fixture.json")
	if err = os.MkdirAll(filepath.Dir(fixture), 0700); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"base": srv.URL, "admin_token": admin})
	if err = os.WriteFile(fixture, raw, 0600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(fixture)
	cmd := exec.CommandContext(t.Context(), "node", filepath.Join(root, "scripts", "verify-admission-guide.cjs"))
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
}
