package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Explicitly enabled harness: all mutations are confined to importTestApp's disposable DB.
func TestV2BrowserHarness(t *testing.T) {
	if os.Getenv("V2_BROWSER_HARNESS") != "1" {
		t.Skip("explicit browser harness; covered separately")
	}
	a := importTestApp(t)
	root, e := filepath.Abs("..")
	if e != nil {
		t.Fatal(e)
	}
	a.Config.WebDir = filepath.Join(root, "web", "dist")
	server := httptest.NewServer(a.router())
	defer server.Close()
	a.Config.PublicBaseURL = server.URL
	token := resourceTestSession(t, a, "admin")
	file := filepath.Join(root, ".local", "p0p1-browser-fixture.json")
	done := filepath.Join(root, ".local", "p0p1-browser-done")
	os.Remove(done)
	defer os.Remove(file)
	defer os.Remove(done)
	data, _ := json.Marshal(map[string]string{"base": server.URL, "token": token})
	if e := os.WriteFile(file, data, 0600); e != nil {
		t.Fatal(e)
	}
	deadline := time.After(10 * time.Minute)
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("browser harness timed out")
		case <-tick.C:
			if _, e := os.Stat(done); e == nil {
				return
			}
		}
	}
}
