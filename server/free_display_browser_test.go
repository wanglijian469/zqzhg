package main

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestFreeDisplayBrowserAcceptance(t *testing.T) {
	if os.Getenv("FREE_SERVICE_BROWSER") != "1" {
		t.Skip("explicit isolated free-service browser acceptance")
	}
	a := importTestApp(t)
	root, _ := filepath.Abs("..")
	a.Config.WebDir = filepath.Join(root, "web", "dist")
	admin := resourceTestSession(t, a, "admin")
	for i := 0; i < 26; i++ {
		v := v2Published(t, a, "vendor", 0, 1, 0, 0, false, 0, fmt.Sprintf("免费轮换隔离验收企业%02d", i))
		if e := a.DB.Create(&HomeRotationPool{VendorID: v.ID, HallID: 1, State: "approved", Reason: "隔离浏览器验收", UpdatedAt: time.Now().UTC()}).Error; e != nil {
			t.Fatal(e)
		}
	}
	server := httptest.NewServer(a.router())
	defer server.Close()
	a.Config.PublicBaseURL = server.URL
	fixture := filepath.Join(root, ".local", "free-service-browser-fixture.json")
	blob, _ := json.Marshal(map[string]any{"base": server.URL, "admin_token": admin, "credit_code": serviceTestCreditCode()})
	if e := os.WriteFile(fixture, blob, 0600); e != nil {
		t.Fatal(e)
	}
	defer os.Remove(fixture)
	command := exec.CommandContext(t.Context(), "node", filepath.Join(root, "scripts", "verify-free-service.cjs"))
	command.Dir = root
	output, e := command.CombinedOutput()
	t.Log(string(output))
	if e != nil {
		t.Fatal(e)
	}
}
