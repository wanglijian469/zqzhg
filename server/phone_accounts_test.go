package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"strings"
	"sync"
	"testing"
	"time"
)

func phoneFixture(t *testing.T, a *App, phone, purpose string, id uint64) string {
	t.Helper()
	code := "246810"
	salt := randomToken()
	a.DB.Delete(&PhoneVerification{}, "phone=? AND purpose=? AND account_id=?", phone, purpose, id)
	if e := a.DB.Create(&PhoneVerification{Phone: phone, Purpose: purpose, AccountID: id, Salt: salt, Digest: smsCodeDigest(salt, phone, purpose, code), SentAt: time.Now().Add(-61 * time.Second), ExpiresAt: time.Now().Add(5 * time.Minute)}).Error; e != nil {
		t.Fatal(e)
	}
	return code
}
func phoneCall(t *testing.T, r *gin.Engine, method, path, token string, data gin.H, status int) map[string]any {
	t.Helper()
	clearRegistrationLimit()
	if (path == "/auth/phone/register" || path == "/auth/merchant/register" || path == "/auth/visitor/register") && data != nil && data["agreements"] == nil {
		data["agreements"] = (testClient{t, r}).registrationAgreements()
		data["source"] = "web"
	}
	return (testClient{t, r}).call(method, path, token, data, status)
}
func TestPhoneRegistrationLoginUpgradeAndMigration(t *testing.T) {
	a := importTestApp(t)
	r := a.router()
	pw := "  Phone-Pass-2026!  "
	phone := "13900001111"
	code := phoneFixture(t, a, phone, "register", 0)
	out := phoneCall(t, r, "POST", "/auth/phone/register", "", gin.H{"phone": phone, "code": code, "password": pw, "role": "visitor"}, 201)
	account := out["account"].(map[string]any)
	id := uint64(account["id"].(float64))
	token := out["token"].(string)
	if account["phone"] != phone || account["phone_verified"] != true || account["requires_phone_binding"] != false {
		t.Fatal(account)
	}
	var stored Account
	a.DB.First(&stored, id)
	if stored.Role != "visitor" || stored.LoginName != nil || bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte(pw)) != nil {
		t.Fatal("account fields")
	}
	phoneCall(t, r, "POST", "/auth/phone/register", "", gin.H{"phone": phone, "code": code, "password": pw, "role": "merchant"}, 400)
	phoneFixture(t, a, phone, "register", 0)
	phoneCall(t, r, "POST", "/auth/phone/register", "", gin.H{"phone": phone, "code": code, "password": pw, "role": "merchant"}, 409)
	phoneCall(t, r, "POST", "/auth/phone/password", "", gin.H{"phone": phone, "password": "incorrect"}, 401)
	phoneCall(t, r, "POST", "/auth/visitor", "", gin.H{"username": phone, "password": pw}, 200)
	phoneCall(t, r, "POST", "/auth/phone/password", "", gin.H{"phone": phone, "password": pw}, 200)
	a.DB.Create(&Favorite{AccountID: id, Kind: "vendor", ContentID: 999, Active: true, CollectedAt: time.Now(), UpdatedAt: time.Now()})
	phoneCall(t, r, "POST", "/auth/me/merchant-upgrade", token, gin.H{}, 400)
	upgraded := phoneCall(t, r, "POST", "/auth/me/merchant-upgrade", token, gin.H{"confirmed": true}, 200)
	if upgraded["account"].(map[string]any)["id"] != float64(id) || upgraded["account"].(map[string]any)["role"] != "merchant" {
		t.Fatal(upgraded)
	}
	phoneCall(t, r, "GET", "/auth/me", token, nil, 401)
	var n int64
	a.DB.Model(&Favorite{}).Where("account_id=? AND active=1", id).Count(&n)
	if n != 1 {
		t.Fatal("favorites lost")
	}
	a.DB.Model(&Content{}).Where("owner_id=?", id).Count(&n)
	if n != 0 {
		t.Fatal("automatic showroom")
	}
	phoneCall(t, r, "POST", "/auth/merchant", "", gin.H{"username": phone, "password": pw}, 200)
	login := phoneCall(t, r, "POST", "/auth/phone/password", "", gin.H{"phone": phone, "password": pw}, 200)
	if login["account"].(map[string]any)["role"] != "merchant" {
		t.Fatal(login)
	}
	// DDL retry recovers partially completed migration without changing identities.
	a.DB.Exec("DELETE FROM schema_migrations WHERE version=19")
	if e := migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	if e := migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	a.DB.First(&stored, id)
	if !verifiedPhone(stored) {
		t.Fatal("phone lost after migration retry")
	}
	var audit []OperationLog
	a.DB.Where("actor_id=?", id).Find(&audit)
	raw, _ := json.Marshal(audit)
	if string(raw) == "[]" || containsSecret(string(raw), pw, stored.PasswordHash, phone, code) {
		t.Fatal("missing or sensitive audit")
	}
}
func containsSecret(s string, values ...string) bool {
	for _, v := range values {
		if len(v) > 0 && strings.Contains(s, v) {
			return true
		}
	}
	return false
}

func TestPhoneCodePurposesAttemptsExpiryAndCooldown(t *testing.T) {
	a := importTestApp(t)
	r := a.router()
	phone := "13900002222"
	pw := "Phone-Password-2026!"
	out := phoneCall(t, r, "POST", "/auth/phone/code", "", gin.H{"phone": phone, "purpose": "register"}, 200)
	if len(out["development_code"].(string)) != 6 || out["expires_in"] != float64(300) {
		t.Fatal(out)
	}
	phoneCall(t, r, "POST", "/auth/phone/code", "", gin.H{"phone": phone, "purpose": "setup_password"}, 409)
	phoneCall(t, r, "POST", "/auth/sms", "", gin.H{"phone": phone}, 409)
	code := phoneFixture(t, a, phone, "register", 0)
	phoneCall(t, r, "POST", "/auth/phone/password/setup", "", gin.H{"phone": phone, "code": code, "password": pw}, 400)
	for i := 0; i < 5; i++ {
		phoneCall(t, r, "POST", "/auth/phone/register", "", gin.H{"phone": phone, "code": "000000", "password": pw, "role": "visitor"}, 400)
	}
	phoneCall(t, r, "POST", "/auth/phone/register", "", gin.H{"phone": phone, "code": code, "password": pw, "role": "visitor"}, 400)
	var row PhoneVerification
	a.DB.First(&row, "phone=? AND purpose='register'", phone)
	if row.Attempts != 5 || row.Used {
		t.Fatal(row.Attempts, row.Used)
	}
	phoneFixture(t, a, phone, "register", 0)
	a.DB.Model(&PhoneVerification{}).Where("phone=?", phone).Update("expires_at", time.Now().Add(-time.Second))
	phoneCall(t, r, "POST", "/auth/phone/register", "", gin.H{"phone": phone, "code": code, "password": pw, "role": "visitor"}, 400)
	phoneCall(t, r, "POST", "/auth/phone/code", "", gin.H{"phone": "12345", "purpose": "register"}, 400)
	phoneCall(t, r, "POST", "/auth/phone/code", "", gin.H{"phone": phone, "purpose": "bind"}, 400)
	// The development adapter cannot send in a production environment.
	a.Config.Env = "production"
	other := "13900002223"
	out = phoneCall(t, r, "POST", "/auth/phone/code", "", gin.H{"phone": other, "purpose": "register"}, 426)
	if _, ok := out["development_code"]; ok {
		t.Fatal("code exposed")
	}
}
func TestPhoneLegacyBindingSetupAndRollback(t *testing.T) {
	a := importTestApp(t)
	r := a.router()
	pw := "Legacy-Password-2026!"
	phone := "13900003333"
	out := phoneCall(t, r, "POST", "/auth/visitor/register", "", gin.H{"login_name": "original-account", "password": pw}, 201)
	id := uint64(out["account"].(map[string]any)["id"].(float64))
	token := out["token"].(string)
	phoneCall(t, r, "POST", "/auth/me/merchant-upgrade", token, gin.H{"confirmed": true}, 400)
	code := phoneFixture(t, a, phone, "bind", id)
	// Code cannot be used by another account or public registration.
	phoneCall(t, r, "POST", "/auth/phone/register", "", gin.H{"phone": phone, "code": code, "password": pw, "role": "visitor"}, 400)
	a.DB.Callback().Create().Before("gorm:create").Register("phone_audit_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "operation_logs" {
			tx.AddError(errors.New("test rollback"))
		}
	})
	phoneCall(t, r, "PUT", "/auth/me/phone", token, gin.H{"phone": phone, "code": code}, 500)
	a.DB.Callback().Create().Remove("phone_audit_failure")
	var old Account
	a.DB.First(&old, id)
	if old.Phone != nil {
		t.Fatal("binding rollback failed")
	}
	bound := phoneCall(t, r, "PUT", "/auth/me/phone", token, gin.H{"phone": phone, "code": code}, 200)
	if bound["account"].(map[string]any)["id"] != float64(id) {
		t.Fatal("new account on bind")
	}
	phoneCall(t, r, "GET", "/auth/me", token, nil, 401)
	phoneCall(t, r, "POST", "/auth/visitor", "", gin.H{"username": "original-account", "password": pw}, 200)
	other := phoneCall(t, r, "POST", "/auth/merchant/register", "", gin.H{"login_name": "other-account", "password": pw}, 201)
	otherID := uint64(other["account"].(map[string]any)["id"].(float64))
	phoneFixture(t, a, phone, "bind", otherID)
	phoneCall(t, r, "PUT", "/auth/me/phone", other["token"].(string), gin.H{"phone": phone, "code": code}, 409)
	smsPhone := "13900003334"
	sms := Account{Username: smsPhone, Role: "merchant"}
	a.DB.Create(&sms)
	oldSession, _ := a.newSession(sms)
	phoneFixture(t, a, smsPhone, "setup_password", 0)
	setup := phoneCall(t, r, "POST", "/auth/phone/password/setup", "", gin.H{"phone": smsPhone, "code": code, "password": pw}, 200)
	if setup["account"].(map[string]any)["id"] != float64(sms.ID) {
		t.Fatal("SMS setup changed ID")
	}
	phoneCall(t, r, "GET", "/auth/me", oldSession["token"].(string), nil, 401)
	phoneFixture(t, a, smsPhone, "setup_password", 0)
	phoneCall(t, r, "POST", "/auth/phone/password/setup", "", gin.H{"phone": smsPhone, "code": code, "password": "Changed-Password-2026!"}, 400)
	phoneCall(t, r, "POST", "/auth/phone/password", "", gin.H{"phone": smsPhone, "password": pw}, 200)
}
func TestPhoneConcurrentRegistration(t *testing.T) {
	a := importTestApp(t)
	r := a.router()
	phone := "13900004444"
	code := phoneFixture(t, a, phone, "register", 0)
	clearRegistrationLimit()
	var wg sync.WaitGroup
	refs := serviceTestRefs(t, a, "terms", "privacy")
	statuses := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := resetRequest(r, "/auth/phone/register", "", gin.H{"phone": phone, "code": code, "password": "Phone-Password-2026!", "role": "visitor", "agreements": refs, "source": "web"})
			statuses <- w.Code
		}()
	}
	wg.Wait()
	close(statuses)
	accepted := 0
	for status := range statuses {
		if status == 201 {
			accepted++
		} else if status != 400 {
			t.Fatal(fmt.Sprint(status))
		}
	}
	if accepted != 1 {
		t.Fatal("duplicate registration")
	}
	var n int64
	a.DB.Model(&Account{}).Where("phone=?", phone).Count(&n)
	if n != 1 {
		t.Fatal(n)
	}
}
func TestPhoneIdentityConflictsAndAdminReset(t *testing.T) {
	a := importTestApp(t)
	r := a.router()
	pw := "Phone-Password-2026!"
	phone := "13900005555"
	admin := resourceTestSession(t, a, "admin")
	code := phoneFixture(t, a, phone, "register", 0)
	out := phoneCall(t, r, "POST", "/auth/phone/register", "", gin.H{"phone": phone, "code": code, "password": pw, "role": "visitor"}, 201)
	id := out["account"].(map[string]any)["id"]
	token := out["token"].(string)
	phoneCall(t, r, "POST", "/auth/merchant/register", "", gin.H{"login_name": phone, "password": pw}, 409)
	listing := phoneCall(t, r, "GET", "/admin/accounts?q="+phone, admin, nil, 200)
	if listing["total"] != float64(1) {
		t.Fatal("verified phone not found in admin query")
	}
	reset := phoneCall(t, r, "POST", "/admin/accounts/"+number(id)+"/password-reset", admin, gin.H{"mode": "generate"}, 200)
	if reset["login_name"] != phone {
		t.Fatal(reset)
	}
	phoneCall(t, r, "GET", "/auth/me", token, nil, 401)
	phoneCall(t, r, "POST", "/auth/phone/password", "", gin.H{"phone": phone, "password": reset["password"]}, 200)
	var log OperationLog
	a.DB.Where("target_id=? AND action='account_password_reset'", id).First(&log)
	if strings.Contains(log.Note, phone) {
		t.Fatal("phone exposed in reset audit")
	}
	// Numeric legacy usernames reserve a number until the administrator resolves it.
	reserved := "13900005556"
	phoneCall(t, r, "POST", "/auth/merchant/register", "", gin.H{"login_name": reserved, "password": pw}, 201)
	code = phoneFixture(t, a, reserved, "register", 0)
	phoneCall(t, r, "POST", "/auth/phone/register", "", gin.H{"phone": reserved, "code": code, "password": pw, "role": "merchant"}, 409)
	a.Config.SMSProvider = "disabled"
	phoneCall(t, r, "POST", "/auth/phone/code", "", gin.H{"phone": phone, "purpose": "register"}, 503)
	for _, bad := range []string{"12345678901", strings.Repeat(" ", 12), strings.Repeat("😀", 19)} {
		phoneCall(t, r, "POST", "/auth/phone/register", "", gin.H{"phone": "13900005557", "code": "123456", "password": bad, "role": "visitor"}, 503)
	}
	a.Config.SMSProvider = "development"
	for _, bad := range []string{"12345678901", strings.Repeat(" ", 12), strings.Repeat("😀", 19)} {
		phoneCall(t, r, "POST", "/auth/phone/register", "", gin.H{"phone": "13900005557", "code": "123456", "password": bad, "role": "visitor"}, 400)
	}
}
