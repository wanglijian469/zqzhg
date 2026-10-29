package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"strings"
	"sync"
	"time"
)

func randomToken() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func digest(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func (a *App) readAccount(c *gin.Context) (Account, error) {
	token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	if token == "" || len(token) != 64 {
		return Account{}, fmt.Errorf("unauthorized")
	}
	var s Session
	if e := a.DB.Where("token_hash = ? AND expires_at > ?", digest(token), time.Now().UTC()).First(&s).Error; e != nil {
		return Account{}, e
	}
	var account Account
	e := a.DB.First(&account, s.AccountID).Error
	if e == nil && account.Role == "operator" {
		var op PlatformOperator
		if a.DB.First(&op, "account_id=? AND enabled=1", account.ID).Error != nil {
			return Account{}, fmt.Errorf("unauthorized")
		}
	}
	return account, e
}
func (a *App) requireAuth(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		account, e := a.readAccount(c)
		if e != nil {
			fail(c, 401, "unauthorized")
			return
		}
		if role != "" && account.Role != role && !(role == "platform" && platformAccount(account)) {
			fail(c, 403, "forbidden")
			return
		}
		c.Set("account", account)
		c.Next()
	}
}
func (a *App) newSession(account Account) (gin.H, error) {
	return newSessionIn(a.DB, account)
}
func newSessionIn(db *gorm.DB, account Account) (gin.H, error) {
	token := randomToken()
	e := db.Create(&Session{TokenHash: digest(token), AccountID: account.ID, ExpiresAt: time.Now().UTC().Add(30 * 24 * time.Hour)}).Error
	return gin.H{"token": token, "account": account}, e
}

var authLimits = struct {
	sync.Mutex
	entries map[string][]time.Time
}{entries: map[string][]time.Time{}}

func permit(key string, max int, window time.Duration) bool {
	authLimits.Lock()
	defer authLimits.Unlock()
	now := time.Now()
	items := authLimits.entries[key]
	keep := items[:0]
	for _, t := range items {
		if now.Sub(t) < window {
			keep = append(keep, t)
		}
	}
	if len(keep) >= max {
		return false
	}
	authLimits.entries[key] = append(keep, now)
	if len(authLimits.entries) > 20000 {
		for k, v := range authLimits.entries {
			if len(v) == 0 || now.Sub(v[len(v)-1]) > time.Hour {
				delete(authLimits.entries, k)
			}
		}
	}
	return true
}

// Legacy clients may sign in only; these aliases never create accounts.
func (a *App) sendCode(c *gin.Context)   { a.sendSMSCode(c, false, true) }
func (a *App) phoneLogin(c *gin.Context) { a.phoneSMSLogin(c) }
func (a *App) adminLogin(c *gin.Context) {
	if !permit("admin:"+c.ClientIP(), 10, time.Minute) {
		fail(c, 429, "rate_limited")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	var account Account
	if a.DB.Where("username = ? AND role IN ?", req.Username, []string{"admin", "operator"}).First(&account).Error != nil || bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(req.Password)) != nil {
		fail(c, 401, "invalid_credentials")
		return
	}
	if account.Role == "operator" {
		var op PlatformOperator
		if a.DB.First(&op, "account_id=? AND enabled=1", account.ID).Error != nil {
			fail(c, 401, "invalid_credentials")
			return
		}
	}
	session, e := a.newSession(account)
	if e != nil {
		fail(c, 500, "internal_error")
		return
	}
	session["account"] = a.accountJSON(account)
	c.JSON(200, session)
}
func (a *App) logout(c *gin.Context) {
	token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	if a.DB.Delete(&Session{}, "token_hash = ?", digest(token)).Error != nil {
		fail(c, 500, "internal_error")
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

type SMSProvider interface {
	Send(phone, code string) error
}
type DevelopmentSMS struct{}

type DisabledSMS struct{}

func (DisabledSMS) Send(phone, code string) error { return fmt.Errorf("sms_disabled") }

func validateSMSMode(environment, provider string) error {
	switch provider {
	case "aliyun", "disabled":
		return nil
	case "development":
		if environment != "production" {
			return nil
		}
	}
	return fmt.Errorf("SMS_PROVIDER must be aliyun or disabled in production; development is permitted only outside production")
}

func (DevelopmentSMS) Send(phone, code string) error { return nil }
