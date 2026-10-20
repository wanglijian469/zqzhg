package main

import (
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// All account writes in this browser check use a disposable MySQL database.
func TestMerchantEntryBrowser(t *testing.T) {
	if os.Getenv("RUN_MERCHANT_ENTRY_BROWSER") != "1" {
		t.Skip("set RUN_MERCHANT_ENTRY_BROWSER=1 with RUN_MYSQL_TESTS=1")
	}
	a := importTestApp(t)
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	a.Config.WebDir = filepath.Join(root, "web", "dist")
	if _, err := os.Stat(filepath.Join(a.Config.WebDir, "index.html")); err != nil {
		t.Fatal("build web first", err)
	}
	server := httptest.NewServer(a.router())
	defer server.Close()
	command := exec.CommandContext(t.Context(), "node", filepath.Join(root, "scripts", "verify-merchant-entry-browser.cjs"), server.URL)
	command.Dir = root
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := a.DB.Model(&Content{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("entry navigation must not create a company: count=%d err=%v", count, err)
	}
	if err := a.DB.Model(&Account{}).Where("role = ?", "merchant").Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("expected one company account and one independent phone account: count=%d err=%v", count, err)
	}
}
