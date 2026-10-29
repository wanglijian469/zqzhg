package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestProcurementBrowser(t *testing.T) {
	if os.Getenv("RUN_PROCUREMENT_BROWSER") != "1" {
		t.Skip("set RUN_PROCUREMENT_BROWSER=1 and RUN_MYSQL_TESTS=1 after building web")
	}
	a := importTestApp(t)
	root, e := filepath.Abs("..")
	if e != nil {
		t.Fatal(e)
	}
	a.Config.WebDir = filepath.Join(root, "web", "dist")
	if dir := os.Getenv("TEST_WEB_DIR"); dir != "" {
		a.Config.WebDir = dir
	}
	buyer, buyerToken := procurementTestAccount(t, a, "visitor")
	cat := procurementCategory(t, a)
	s1, t1, v1, product := procurementSupplier(t, a, cat)
	s2, t2, v2, _ := procurementSupplier(t, a, cat)
	admin := resourceTestSession(t, a, "admin")
	srv := httptest.NewServer(a.router())
	defer srv.Close()
	a.Config.PublicBaseURL = srv.URL
	fixture := gin.H{"base": srv.URL, "buyer": gin.H{"token": buyerToken, "id": buyer.ID, "phone": buyer.Phone}, "supplier1": gin.H{"token": t1, "id": s1.ID, "vendor_id": v1.ID}, "supplier2": gin.H{"token": t2, "id": s2.ID, "vendor_id": v2.ID}, "admin": admin, "product_id": product.ID}
	raw, _ := json.Marshal(fixture)
	path := filepath.Join(root, ".local", "procurement-browser-fixture.json")
	if e = os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	defer os.Remove(path)
	cmd := exec.CommandContext(t.Context(), "node", filepath.Join(root, "scripts", "verify-procurement.cjs"))
	cmd.Dir = root
	output, e := cmd.CombinedOutput()
	t.Log(string(output))
	if e != nil {
		t.Fatal(e)
	}
}
