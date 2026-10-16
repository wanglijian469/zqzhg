package main

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"strings"
	"sync"
	"testing"
	"time"
)

func clearRegistrationLimit() {
	authLimits.Lock()
	authLimits.entries = map[string][]time.Time{}
	authLimits.Unlock()
}
func TestMerchantRegistrationAndExistingLogin(t *testing.T) {
	a := importTestApp(t)
	clearRegistrationLimit()
	r := a.router()
	tc := testClient{t, r}
	password := "  注册密码😀ABC  "
	out := resetRequest(r, "/auth/merchant/register", "", gin.H{"login_name": "  新厂商😀  ", "password": password, "role": "admin"})
	if out.Code != 201 || out.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(out.Code, out.Body.String())
	}
	var result map[string]any
	json.Unmarshal(out.Body.Bytes(), &result)
	token := result["token"].(string)
	var account Account
	a.DB.Where("login_name=?", "新厂商😀").First(&account)
	if account.Role != "merchant" || bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(password)) != nil {
		t.Fatal("invalid account")
	}
	if strings.Contains(out.Body.String(), password) || strings.Contains(out.Body.String(), account.PasswordHash) {
		t.Fatal("secret in response")
	}
	tc.call("GET", "/admin/accounts", token, nil, 403)
	draft := tc.call("POST", "/merchant/contents", token, gin.H{"kind": "vendor", "data": Payload{Name: "注册厂商", RegionName: "河北", MainProducts: "液压机械", Phone: "0319-1234567"}}, 201)
	tc.call("POST", "/merchant/contents/"+number(draft["id"])+"/submit", token, gin.H{"revision_id": draft["revision_id"]}, 200)
	existing := tc.call("POST", "/auth/merchant", "", gin.H{"username": "新厂商😀", "password": password}, 200)
	tc.call("GET", "/merchant/contents/"+number(draft["id"]), existing["token"].(string), nil, 200)
	tc.call("POST", "/auth/merchant", "", gin.H{"username": "新厂商😀", "password": strings.TrimSpace(password)}, 401)
	tc.call("POST", "/auth/merchant/register", "", gin.H{"login_name": "新厂商😀", "password": password}, 409)
	var audit OperationLog
	if a.DB.Where("action='merchant_registered' AND actor_id=?", account.ID).First(&audit).Error != nil {
		t.Fatal("missing audit")
	}
	raw, _ := json.Marshal(audit)
	if strings.Contains(string(raw), password) || strings.Contains(string(raw), account.PasswordHash) {
		t.Fatal("secret in audit")
	}
}
func TestMerchantRegistrationBoundariesRateAndRollback(t *testing.T) {
	a := importTestApp(t)
	r := a.router()
	tc := testClient{t, r}
	clearRegistrationLimit()
	for _, input := range []gin.H{{"login_name": "", "password": "123456789012"}, {"login_name": strings.Repeat("😀", 101), "password": "123456789012"}, {"login_name": "short", "password": "12345678901"}, {"login_name": "space", "password": strings.Repeat(" ", 12)}, {"login_name": "long", "password": strings.Repeat("😀", 18) + "a"}} {
		tc.call("POST", "/auth/merchant/register", "", input, 400)
	}
	tc.call("POST", "/auth/merchant/register", "", gin.H{"login_name": "rate", "password": "123456789012"}, 429)
	clearRegistrationLimit()
	tc.call("POST", "/auth/merchant/register", "", gin.H{"login_name": strings.Repeat("😀", 100), "password": strings.Repeat("😀", 18)}, 201)
	tc.call("POST", "/auth/merchant/register", "", gin.H{"login_name": "中文边界", "password": "中中中中"}, 201)
	clearRegistrationLimit()
	a.DB.Callback().Create().Before("gorm:create").Register("registration_rollback", func(tx *gorm.DB) {
		if tx.Statement.Table == "operation_logs" {
			tx.AddError(errors.New("isolated audit failure"))
		}
	})
	out := resetRequest(r, "/auth/merchant/register", "", gin.H{"login_name": "事务失败厂商", "password": "123456789012"})
	if out.Code != 500 {
		t.Fatal(out.Code)
	}
	a.DB.Callback().Create().Remove("registration_rollback")
	var count int64
	a.DB.Model(&Account{}).Where("login_name=?", "事务失败厂商").Count(&count)
	if count != 0 {
		t.Fatal("account survived rollback")
	}
	var sessions int64
	a.DB.Model(&Session{}).Count(&sessions)
	if sessions != 2 {
		t.Fatal("session survived rollback", sessions)
	}
}
func TestConcurrentMerchantRegistrationUniqueName(t *testing.T) {
	a := importTestApp(t)
	clearRegistrationLimit()
	r := a.router()
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			codes <- resetRequest(r, "/auth/merchant/register", "", gin.H{"login_name": "同名并发厂商", "password": "Concurrent-password-2026"}).Code
		}()
	}
	close(start)
	wg.Wait()
	close(codes)
	successes, duplicates := 0, 0
	for code := range codes {
		if code == 201 {
			successes++
		} else if code == 409 {
			duplicates++
		} else {
			t.Fatal(code)
		}
	}
	if successes != 1 || duplicates != 1 {
		t.Fatal(successes, duplicates)
	}
	var count int64
	a.DB.Model(&Account{}).Where("login_name=?", "同名并发厂商").Count(&count)
	if count != 1 {
		t.Fatal("duplicate accounts", count)
	}
}
