package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
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
	return account, e
}
func (a *App) requireAuth(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		account, e := a.readAccount(c)
		if e != nil {
			fail(c, 401, "unauthorized")
			return
		}
		if role != "" && account.Role != role {
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
func (a *App) sendCode(c *gin.Context) {
	if a.Config.SMSProvider == "disabled" {
		fail(c, 503, "sms_disabled")
		return
	}
	if !permit("sms:"+c.ClientIP(), 20, time.Hour) {
		fail(c, 429, "rate_limited")
		return
	}
	var req struct {
		Phone string `json:"phone"`
	}
	if c.ShouldBindJSON(&req) != nil || !regexp.MustCompile(`^1[3-9][0-9]{9}$`).MatchString(req.Phone) {
		fail(c, 400, "invalid_phone")
		return
	}
	n, e := rand.Int(rand.Reader, big.NewInt(1000000))
	if e != nil {
		fail(c, 500, "internal_error")
		return
	}
	code := fmt.Sprintf("%06d", n.Int64())
	salt := randomToken()
	now := time.Now().UTC()
	e = a.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&SMSCode{Phone: req.Phone, Digest: "", Salt: "", ExpiresAt: now, SentAt: time.Unix(0, 0).UTC()}).Error; e != nil {
			return e
		}
		var s SMSCode
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&s, "phone = ?", req.Phone).Error; e != nil {
			return e
		}
		if now.Sub(s.SentAt) < 60*time.Second {
			return fmt.Errorf("rate_limited")
		}
		if e := a.SMS.Send(req.Phone, code); e != nil {
			return fmt.Errorf("sms_unavailable")
		}
		return tx.Model(&s).Updates(map[string]any{"digest": digest(salt + req.Phone + code), "salt": salt, "expires_at": now.Add(5 * time.Minute), "sent_at": now, "attempts": 0, "used": false}).Error
	})
	if e != nil {
		if e.Error() == "rate_limited" {
			fail(c, 429, "rate_limited")
		} else {
			fail(c, 503, "sms_unavailable")
		}
		return
	}
	response := gin.H{"sent": true, "expires_in": 300}
	if a.Config.Env != "production" && a.Config.SMSProvider == "development" {
		response["development_code"] = code
	}
	c.JSON(200, response)
}
func (a *App) phoneLogin(c *gin.Context) {
	if a.Config.SMSProvider == "disabled" {
		fail(c, 503, "sms_disabled")
		return
	}
	if !permit("phone:"+c.ClientIP(), 30, time.Minute) {
		fail(c, 429, "rate_limited")
		return
	}
	var req struct {
		Phone string `json:"phone"`
		Code  string `json:"code"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	valid := false
	var account Account
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		var s SMSCode
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&s, "phone = ?", req.Phone).Error; e != nil {
			if e == gorm.ErrRecordNotFound {
				return nil
			}
			return e
		}
		if s.Used || s.Attempts >= 5 || time.Now().UTC().After(s.ExpiresAt) {
			return nil
		}
		if e := tx.Model(&s).Update("attempts", s.Attempts+1).Error; e != nil {
			return e
		}
		if subtle.ConstantTimeCompare([]byte(digest(s.Salt+req.Phone+req.Code)), []byte(s.Digest)) != 1 {
			return nil
		}
		if e := tx.Model(&s).Update("used", true).Error; e != nil {
			return e
		}
		if e := tx.Where("username = ?", req.Phone).FirstOrCreate(&account, Account{Username: req.Phone, Role: "merchant"}).Error; e != nil {
			return e
		}
		if account.Role != "merchant" {
			return nil
		}
		valid = true
		return nil
	})
	if e != nil {
		fail(c, 500, "internal_error")
		return
	}
	if !valid {
		fail(c, 401, "invalid_code")
		return
	}
	session, e := a.newSession(account)
	if e != nil {
		fail(c, 500, "internal_error")
		return
	}
	c.JSON(200, session)
}
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
	if a.DB.Where("username = ? AND role = ?", req.Username, "admin").First(&account).Error != nil || bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(req.Password)) != nil {
		fail(c, 401, "invalid_credentials")
		return
	}
	session, e := a.newSession(account)
	if e != nil {
		fail(c, 500, "internal_error")
		return
	}
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

type AliyunSMS struct {
	KeyID, Secret, Sign, Template string
	Client                        *http.Client
}

func NewAliyunSMS() (AliyunSMS, error) {
	s := AliyunSMS{KeyID: os.Getenv("ALIYUN_ACCESS_KEY_ID"), Secret: os.Getenv("ALIYUN_ACCESS_KEY_SECRET"), Sign: os.Getenv("ALIYUN_SMS_SIGN_NAME"), Template: os.Getenv("ALIYUN_SMS_TEMPLATE_CODE"), Client: &http.Client{Timeout: 15 * time.Second}}
	if s.KeyID == "" || s.Secret == "" || s.Sign == "" || s.Template == "" {
		return s, fmt.Errorf("Aliyun SMS credentials, sign and template are required")
	}
	return s, nil
}
func rpcEscape(s string) string { return strings.ReplaceAll(url.QueryEscape(s), "+", "%20") }
func (s AliyunSMS) Send(phone, code string) error {
	params := map[string]string{"Action": "SendSms", "Version": "2017-05-25", "Format": "JSON", "AccessKeyId": s.KeyID, "SignatureMethod": "HMAC-SHA1", "SignatureVersion": "1.0", "SignatureNonce": randomToken(), "Timestamp": time.Now().UTC().Format("2006-01-02T15:04:05Z"), "RegionId": "cn-hangzhou", "PhoneNumbers": phone, "SignName": s.Sign, "TemplateCode": s.Template, "TemplateParam": `{"code":"` + code + `"}`}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, rpcEscape(k)+"="+rpcEscape(params[k]))
	}
	mac := hmac.New(sha1.New, []byte(s.Secret+"&"))
	mac.Write([]byte("POST&%2F&" + rpcEscape(strings.Join(parts, "&"))))
	params["Signature"] = base64.StdEncoding.EncodeToString(mac.Sum(nil))
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	res, e := s.Client.PostForm("https://dysmsapi.aliyuncs.com/", form)
	if e != nil {
		return e
	}
	defer res.Body.Close()
	var result struct{ Code string }
	if e = json.NewDecoder(http.MaxBytesReader(nil, res.Body, 1<<20)).Decode(&result); e != nil {
		return e
	}
	if res.StatusCode != 200 || result.Code != "OK" {
		return fmt.Errorf("SMS delivery failed")
	}
	return nil
}
