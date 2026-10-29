package main

import (
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestUnifiedPasswordPolicy(t *testing.T) {
	for _, v := range []string{"12345678", "a!234567", strings.Repeat("A", 20)} {
		if !validNewPassword(v) {
			t.Fatal("valid password rejected")
		}
	}
	for _, v := range []string{"1234567", strings.Repeat("a", 21), "abc defg", "中文密码测试123", "😀abc12345"} {
		if validNewPassword(v) {
			t.Fatal("invalid password accepted")
		}
	}
}
func TestUnifiedRegistrationWelcomeResetAndCompatibility(t *testing.T) {
	a := importTestApp(t)
	r := a.router()
	phone := "13900007770"
	code := phoneFixture(t, a, phone, "register", 0)
	proof := phoneCall(t, r, "POST", "/auth/phone/verify", "", gin.H{"phone": phone, "purpose": "register", "code": code}, 200)["action_token"].(string)
	var count int64
	a.DB.Model(&Account{}).Where("phone=?", phone).Count(&count)
	if count != 0 {
		t.Fatal("verification created an account")
	}
	out := phoneCall(t, r, "POST", "/auth/phone/register", "", gin.H{"phone": phone, "action_token": proof, "password": "12345678", "role": "merchant"}, 201)
	token := out["token"].(string)
	acc := out["account"].(map[string]any)
	if acc["role"] != "visitor" || acc["welcome_pending"] != true {
		t.Fatal(acc)
	}
	phoneCall(t, r, "POST", "/auth/phone/register", "", gin.H{"phone": phone, "action_token": proof, "password": "12345678"}, 400)
	phoneCall(t, r, "POST", "/auth/password", "", gin.H{"username": phone, "password": "12345678"}, 200)
	phoneCall(t, r, "POST", "/auth/me/welcome", token, gin.H{"choice": "products"}, 200)
	if phoneCall(t, r, "GET", "/auth/me", token, nil, 200)["welcome_pending"] != false {
		t.Fatal("welcome repeated")
	}
	phoneCall(t, r, "POST", "/auth/phone/login", "", gin.H{"phone": phone, "code": code}, 410)
	phoneCall(t, r, "POST", "/auth/sms", "", gin.H{"phone": phone}, 410)
	code = phoneFixture(t, a, phone, "reset_password", 0)
	resetProof := phoneCall(t, r, "POST", "/auth/phone/verify", "", gin.H{"phone": phone, "purpose": "reset_password", "code": code}, 200)["action_token"].(string)
	phoneCall(t, r, "POST", "/auth/phone/password/reset", "", gin.H{"phone": phone, "action_token": proof, "password": "New!1234"}, 400)
	phoneCall(t, r, "POST", "/auth/phone/password/reset", "", gin.H{"phone": phone, "action_token": resetProof, "password": "New!1234"}, 200)
	phoneCall(t, r, "GET", "/auth/me", token, nil, 401)
	phoneCall(t, r, "POST", "/auth/password", "", gin.H{"username": phone, "password": "12345678"}, 401)
	phoneCall(t, r, "POST", "/auth/password", "", gin.H{"username": phone, "password": "New!1234"}, 200)
	phoneCall(t, r, "POST", "/auth/phone/password/reset", "", gin.H{"phone": phone, "action_token": resetProof, "password": "New!5678"}, 400)
	ordinary := phoneCall(t, r, "POST", "/auth/merchant/register", "", gin.H{"login_name": "用户名用户", "password": "abcdefgh"}, 201)
	if ordinary["account"].(map[string]any)["role"] != "visitor" {
		t.Fatal("legacy registration granted manufacturer")
	}
	upgraded := phoneCall(t, r, "POST", "/auth/me/merchant-upgrade", ordinary["token"].(string), gin.H{"confirmed": true}, 200)
	if upgraded["account"].(map[string]any)["role"] != "merchant" {
		t.Fatal("username applicant blocked")
	}
	phoneCall(t, r, "POST", "/auth/password", "", gin.H{"username": "用户名用户", "password": "abcdefgh"}, 200)
	// Historical UTF-8/space/long passwords keep their original hash and login.
	oldName := "historical-company"
	oldPassword := "  历史账号-Long-Password-2026!  "
	hash, _ := bcrypt.GenerateFromPassword([]byte(oldPassword), bcrypt.DefaultCost)
	old := Account{Username: "import:historic", LoginName: &oldName, Role: "merchant", PasswordHash: string(hash)}
	a.DB.Create(&old)
	logged := phoneCall(t, r, "POST", "/auth/password", "", gin.H{"username": oldName, "password": oldPassword}, 200)
	if logged["account"].(map[string]any)["welcome_pending"] != false {
		t.Fatal("historical user got welcome")
	}
	a.DB.First(&old, old.ID)
	if old.PasswordHash != string(hash) {
		t.Fatal("historical hash changed")
	}
}
func TestUnifiedProofExpiryAttemptsAndConcurrentConsumption(t *testing.T) {
	a := importTestApp(t)
	r := a.router()
	phone := "13900007771"
	code := phoneFixture(t, a, phone, "register", 0)
	for i := 0; i < 5; i++ {
		phoneCall(t, r, "POST", "/auth/phone/verify", "", gin.H{"phone": phone, "purpose": "register", "code": "000000"}, 400)
	}
	phoneCall(t, r, "POST", "/auth/phone/verify", "", gin.H{"phone": phone, "purpose": "register", "code": code}, 400)
	code = phoneFixture(t, a, phone, "register", 0)
	proof := phoneCall(t, r, "POST", "/auth/phone/verify", "", gin.H{"phone": phone, "purpose": "register", "code": code}, 200)["action_token"].(string)
	a.DB.Model(&PhoneActionProof{}).Where("token_hash=?", digest(proof)).Update("expires_at", time.Now().Add(-time.Second))
	phoneCall(t, r, "POST", "/auth/phone/register", "", gin.H{"phone": phone, "action_token": proof, "password": "12345678"}, 400)
	a.DB.Model(&PhoneActionProof{}).Where("token_hash=?", digest(proof)).Update("expires_at", time.Now().Add(time.Minute))
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e := a.DB.Transaction(func(tx *gorm.DB) error {
				if e := lockPhoneIdentity(tx, phone); e != nil {
					return e
				}
				ok, e := consumePhoneAction(tx, proof, phone, "register")
				if ok {
					mu.Lock()
					success++
					mu.Unlock()
				}
				return e
			})
			if e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	if success != 1 {
		t.Fatalf("proof replay %d", success)
	}
}
