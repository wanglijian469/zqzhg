package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"strings"
	"testing"
)

func TestNamedMerchantLoginOwnershipAndEditing(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	account := Account{Username: "import:V9090", Role: "merchant"}
	if e := a.DB.Create(&account).Error; e != nil {
		t.Fatal(e)
	}
	session, e := a.newSession(account)
	if e != nil {
		t.Fatal(e)
	}
	oldToken := session["token"].(string)
	vendorData := Payload{Name: "厂商账号测试厂家（仅测试）", MainProducts: "测试农机", RegionName: "测试地区", Phone: "13900009090"}
	vendor := tc.fixtureCall("POST", "/merchant/contents", oldToken, gin.H{"kind": "vendor", "data": vendorData}, 201)
	productData := Payload{Name: "账号编辑测试产品", CategoryID: 1, Parameters: "测试原始参数"}
	product := tc.fixtureCall("POST", "/merchant/contents", oldToken, gin.H{"kind": "product", "data": productData}, 201)
	admin := tc.call("POST", "/auth/admin", "", gin.H{"username": "admin", "password": "Import-Test-Admin-2026!"}, 200)["token"].(string)
	for _, item := range []map[string]any{vendor, product} {
		tc.call("POST", "/merchant/contents/"+number(item["id"])+"/submit", oldToken, gin.H{"revision_id": item["revision_id"]}, 200)
		tc.call("POST", "/admin/revisions/"+number(item["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	}
	if _, e = a.setMerchantLogin("V9090", "admin", "Named-Merchant-Test-2026!"); e == nil {
		t.Fatal("admin name collision must be rejected")
	}
	alias, password := "测试企业专用账号", "Named-Merchant-Test-2026!"
	configured, e := a.setMerchantLogin("V9090", alias, password)
	if e != nil {
		t.Fatal(e)
	}
	if configured.ID != account.ID {
		t.Fatal("provisioning changed the owner account")
	}
	var stored Account
	a.DB.First(&stored, account.ID)
	if stored.PasswordHash == password || bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte(password)) != nil {
		t.Fatal("password must be hashed")
	}
	tc.call("GET", "/merchant/contents", oldToken, nil, 401)
	tc.call("POST", "/auth/merchant", "", gin.H{"username": alias, "password": "wrong-password"}, 401)
	tc.call("POST", "/auth/merchant", "", gin.H{"username": "admin", "password": "Import-Test-Admin-2026!"}, 401)
	tc.call("POST", "/auth/admin", "", gin.H{"username": alias, "password": password}, 401)
	login := tc.call("POST", "/auth/merchant", "", gin.H{"username": alias, "password": password}, 200)
	token := login["token"].(string)
	if strings.Contains(fmt.Sprint(login), password) || strings.Contains(fmt.Sprint(login), stored.PasswordHash) {
		t.Fatal("login leaked credential material")
	}
	items := tc.call("GET", "/merchant/contents", token, nil, 200)
	if items["total"].(float64) != 2 {
		t.Fatal("named login cannot see original booth and product")
	}
	tc.call("GET", "/admin/contents", token, nil, 403)
	other := tc.login("13900009091")
	foreign := tc.fixtureCall("POST", "/merchant/contents", other, gin.H{"kind": "vendor", "data": Payload{Name: "另一个测试厂商", MainProducts: "测试产品", RegionName: "测试地区", Phone: "13900009091"}}, 201)
	tc.call("GET", "/merchant/contents/"+number(foreign["id"]), token, nil, 403)
	vendorData.Description = "厂商账号修改的测试介绍"
	tc.fixtureCall("PUT", "/merchant/contents/"+number(vendor["id"])+"/draft", token, gin.H{"revision_id": vendor["revision_id"], "data": vendorData}, 200)
	productData.Parameters = "厂商账号修改的测试参数"
	tc.fixtureCall("PUT", "/merchant/contents/"+number(product["id"])+"/draft", token, gin.H{"revision_id": product["revision_id"], "data": productData}, 200)
	published := tc.call("GET", "/public/products/"+number(product["id"]), "", nil, 200)
	if published["item"].(map[string]any)["data"].(map[string]any)["parameters"] != "测试原始参数" {
		t.Fatal("saving a merchant draft changed public content")
	}
	// Recover a MySQL ALTER committed just before its version record was written.
	if e = a.DB.Exec("DELETE FROM schema_migrations WHERE version=2").Error; e != nil {
		t.Fatal(e)
	}
	if e = migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	if e = migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	tc.call("POST", "/auth/merchant", "", gin.H{"username": alias, "password": password}, 200)
	tc.call("POST", "/auth/logout", token, gin.H{}, 200)
	tc.call("GET", "/merchant/contents", token, nil, 401)
}
