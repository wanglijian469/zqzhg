package main

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Explicit local-only fixture, backed by the disposable test database.
func TestService104BrowserFixture(t *testing.T) {
	if os.Getenv("SERVICE104_BROWSER_FIXTURE") != "1" {
		t.Skip("operator-only browser fixture")
	}
	a := importTestApp(t)
	web, err := filepath.Abs("../web/dist")
	if err != nil {
		t.Fatal(err)
	}
	a.Config.WebDir = web
	a.Config.PublicBaseURL = "http://127.0.0.1:18094"
	admin := resourceTestSession(t, a, "admin")
	merchant := resourceTestSession(t, a, "merchant")
	owner := serviceTestOwner(t, a, merchant)
	tc := testClient{t, a.router()}
	vendor := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "界面验收制造企业（仅隔离测试）", MainProducts: "耕整地机械", RegionName: "河北省 邢台市 宁晋县", Phone: "0319-1234567"}}, 201)
	_ = recordOperation(a.DB, owner, "account", owner, "phone_password_login", "***1234")
	stop := filepath.Join("..", ".local", "sms", "service104-browser.stop")
	_ = os.Remove(stop)
	info := filepath.Join("..", ".local", "sms", "service104-browser.json")
	b, _ := json.Marshal(gin.H{"base": "http://127.0.0.1:18094", "admin_token": admin, "merchant_token": merchant, "owner": owner, "vendor_id": vendor["id"]})
	if err = os.WriteFile(info, b, 0600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(info)
	defer os.Remove(stop)
	listener, err := net.Listen("tcp", "127.0.0.1:18094")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: a.router(), ReadHeaderTimeout: 10 * time.Second}
	defer server.Shutdown(context.Background())
	go server.Serve(listener)
	deadline := time.Now().Add(60 * time.Minute)
	for time.Now().Before(deadline) {
		if _, err = os.Stat(stop); err == nil {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
}
