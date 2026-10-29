package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSPARouteClassification(t *testing.T) {
	for _, p := range []string{"/buyer", "/buyer/projects/24/edit", "/buyer/opportunities/projects/24", "/buyer/opportunities/join/24", "/merchant/opportunities/join/24", "/merchant/products/24/edit", "/admin/procurement/reviews", "/admin/procurement/projects/1", "/admin/extensions/roles", "/merchant/login", "/admin/"} {
		if !privateSPAPath(p) {
			t.Error("legal deep link rejected", p)
		}
	}
	for _, p := range []string{"/buyer-extra", "/buyer/unknown", "/admin/not-a-page", "/merchant/products/0/edit", "/merchant/opportunities/projects/1/other", "/missing.js", "/unknown"} {
		if privateSPAPath(p) {
			t.Error("unknown path accepted", p)
		}
	}
}

func TestSPARouteHTTP(t *testing.T) {
	a := importTestApp(t)
	if err := os.WriteFile(filepath.Join(a.Config.WebDir, "index.html"), []byte(`<html><head><title>旧标题</title></head><body><div id="root"></div></body></html>`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.Config.WebDir, "asset.js"), []byte(`console.log('static')`), 0600); err != nil {
		t.Fatal(err)
	}
	r := a.router()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/browse/products/?page=2&hall=cable", nil))
	if w.Code != 301 || w.Header().Get("Location") != "/browse/products?page=2&hall=cable" {
		t.Fatal("trailing slash redirect lost route or query")
	}
	for _, tc := range []struct {
		path    string
		status  int
		private bool
	}{{"/admin/login", 200, true}, {"/admin/procurement/projects/1?tab=x", 200, true}, {"/merchant/opportunities/projects/1", 200, true}, {"/buyer/projects/1/edit", 200, true}, {"/buyer/opportunities/settings?x=1", 200, true}, {"/no-such-page", 404, true}, {"/admin/not-a-page", 404, true}, {"/buyer-extra", 404, true}, {"/app", 200, false}, {"/asset.js", 200, false}, {"/missing.js", 404, true}, {"/api/v1/missing", 404, false}} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != tc.status {
			t.Errorf("%s: %d", tc.path, w.Code)
		}
		if tc.private && (w.Header().Get("X-Robots-Tag") != "noindex,follow" || !strings.Contains(w.Body.String(), "noindex,follow") || w.Header().Get("Cache-Control") != "no-store") {
			t.Errorf("missing private metadata %s", tc.path)
		}
	}
}
