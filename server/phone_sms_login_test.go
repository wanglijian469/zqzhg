package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"testing"
	"time"
)

func TestBusinessSMSLoginIsolationAndCodeLifecycle(t *testing.T) {
	a := importTestApp(t)
	r := a.router()
	account, _ := procurementTestAccount(t, a, "visitor")
	out := phoneCall(t, r, "POST", "/auth/phone/code", "", gin.H{"phone": *account.Phone, "purpose": "login"}, 200)
	code := out["development_code"].(string)
	phoneCall(t, r, "POST", "/auth/phone/login", "", gin.H{"phone": *account.Phone, "code": "wrong"}, 401)
	session := phoneCall(t, r, "POST", "/auth/phone/login", "", gin.H{"phone": *account.Phone, "code": code}, 200)
	if procID(session["account"].(map[string]any)["id"]) != account.ID || session["account"].(map[string]any)["role"] != "visitor" {
		t.Fatal("identity changed")
	}
	phoneCall(t, r, "POST", "/auth/phone/login", "", gin.H{"phone": *account.Phone, "code": code}, 401)
	phoneCall(t, r, "POST", "/auth/phone/code", "", gin.H{"phone": *account.Phone, "purpose": "login"}, 429)
	var vendors int64
	a.DB.Model(&Content{}).Where("owner_id=? AND kind='vendor'", account.ID).Count(&vendors)
	if vendors != 0 {
		t.Fatal("SMS login created a supplier")
	}
	code = phoneFixture(t, a, *account.Phone, "login", 0)
	a.DB.Model(&PhoneVerification{}).Where("phone=? AND purpose='login'", *account.Phone).Update("expires_at", time.Now().Add(-time.Minute))
	phoneCall(t, r, "POST", "/auth/phone/login", "", gin.H{"phone": *account.Phone, "code": code}, 401)
	code = phoneFixture(t, a, *account.Phone, "login", 0)
	for i := 0; i < 5; i++ {
		phoneCall(t, r, "POST", "/auth/phone/login", "", gin.H{"phone": *account.Phone, "code": "bad"}, 401)
	}
	phoneCall(t, r, "POST", "/auth/phone/login", "", gin.H{"phone": *account.Phone, "code": code}, 401)
	unknown := "13900009099"
	code = phoneFixture(t, a, unknown, "login", 0)
	phoneCall(t, r, "POST", "/auth/phone/login", "", gin.H{"phone": unknown, "code": code}, 401)
	var n int64
	a.DB.Model(&Account{}).Where("phone=?", unknown).Count(&n)
	if n != 0 {
		t.Fatal("unregistered phone created an account")
	}
	admin, _ := procurementTestAccount(t, a, "admin")
	code = phoneFixture(t, a, *admin.Phone, "login", 0)
	phoneCall(t, r, "POST", "/auth/phone/login", "", gin.H{"phone": *admin.Phone, "code": code}, 401)
	company, _ := procurementTestAccount(t, a, "merchant")
	code = phoneFixture(t, a, *company.Phone, "login", 0)
	result := phoneCall(t, r, "POST", "/auth/phone/login", "", gin.H{"phone": *company.Phone, "code": code}, 200)
	if result["account"].(map[string]any)["role"] != "merchant" {
		t.Fatal("supplier lost identity")
	}
	a.Config.SMSProvider = "aliyun" // An adapter test double avoids any actual network send.
	a.SMS = DevelopmentSMS{}
	smsFixtureAccount(t, a, "13900009098", "visitor")
	out = phoneCall(t, r, "POST", "/auth/phone/code", "", gin.H{"phone": "13900009098", "purpose": "login"}, 200)
	if out["development_code"] != nil {
		t.Fatal("provider mode leaked code")
	}
	a.Config.SMSProvider = "disabled"
	phoneCall(t, r, "POST", "/auth/phone/login", "", gin.H{}, 503)
}
func TestProcurementDemoGuardsAndIdempotentSeed(t *testing.T) {
	t.Setenv("DB_HOST", "127.0.0.1")
	valid := Config{Env: "development", SMSProvider: "development", Addr: "127.0.0.1:8081", ProcurementDemo: true}
	if e := validateProcurementDemo(valid, "zqzt_procurement_demo"); e != nil {
		t.Fatal(e)
	}
	for _, test := range []struct{ env, provider, addr, db string }{{"production", "development", "127.0.0.1:8081", "zqzt_procurement_demo"}, {"development", "aliyun", "127.0.0.1:8081", "zqzt_procurement_demo"}, {"development", "development", "0.0.0.0:8081", "zqzt_procurement_demo"}, {"development", "development", "127.0.0.1:8081", "zqzt_exhibition"}} {
		cfg := valid
		cfg.Env = test.env
		cfg.SMSProvider = test.provider
		cfg.Addr = test.addr
		if validateProcurementDemo(cfg, test.db) == nil {
			t.Fatal("unsafe demo config accepted", test)
		}
	}
	a := importTestApp(t)
	if e := a.seedProcurementDemo(); e != nil {
		t.Fatal(e)
	}
	var before int64
	a.DB.Model(&ProcurementProject{}).Count(&before)
	if before != 2 {
		t.Fatal(before)
	}
	var project ProcurementProject
	a.DB.Where("audience='direct'").First(&project)
	a.DB.Model(&project).Update("state", "closed")
	if e := a.seedProcurementDemo(); e != nil {
		t.Fatal(e)
	}
	var after int64
	a.DB.Model(&ProcurementProject{}).Count(&after)
	a.DB.First(&project, project.ID)
	if after != before || project.State != "closed" {
		t.Fatal("seed overwrote demo work")
	}
	var buyer Account
	a.DB.Where("phone=?", demoPhones[0].Phone).First(&buyer)
	session, e := a.newSession(buyer)
	if e != nil {
		t.Fatal(e)
	}
	r := a.router()
	public := procCall(t, r, "GET", "/public/procurements", "", nil, 200)
	if public["total"] != float64(1) {
		t.Fatal("demo public request missing", public)
	}
	var pub ProcurementProject
	a.DB.Where("audience='public'").First(&pub)
	quotes := procCall(t, r, "GET", fmt.Sprintf("/procurement/projects/%d/quotes", pub.ID), session["token"].(string), nil, 200)
	if len(quotes["items"].([]any)) != 2 {
		t.Fatal("demo comparison missing")
	}
	raw, _ := json.Marshal(public)
	if len(raw) == 0 {
		t.Fatal("missing public data")
	}
	if a.procurementDemoAccounts() != nil {
		t.Fatal("demo accounts exposed on primary config")
	}
}
