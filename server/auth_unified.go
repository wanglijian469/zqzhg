package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

//go:embed password_policy.json
var passwordPolicyJSON []byte
var passwordPolicy = func() map[string]any { var p map[string]any; _ = json.Unmarshal(passwordPolicyJSON, &p); return p }()
var newPasswordPattern = regexp.MustCompile(passwordPolicy["pattern"].(string))

func validNewPassword(value string) bool { return newPasswordPattern.MatchString(value) }

type PhoneActionProof struct {
	TokenHash      string `gorm:"primaryKey"`
	Phone, Purpose string
	ExpiresAt      time.Time
	Used           bool
	CreatedAt      time.Time
}
type AccountWelcome struct {
	AccountID uint64 `gorm:"primaryKey"`
	Pending   bool
	Choice    string
	UpdatedAt time.Time
}

func markWelcomePending(tx *gorm.DB, id uint64) error {
	return tx.Create(&AccountWelcome{AccountID: id, Pending: true, UpdatedAt: time.Now().UTC()}).Error
}
func (a *App) welcomePending(id uint64) bool {
	var w AccountWelcome
	return a.DB.Where("account_id=?", id).First(&w).Error == nil && w.Pending
}
func (a *App) completeWelcome(c *gin.Context) {
	var req struct {
		Choice string `json:"choice"`
	}
	if c.ShouldBindJSON(&req) != nil || !map[string]bool{"manufacturer": true, "products": true, "browse": true, "skip": true}[req.Choice] {
		fail(c, 400, "invalid_request")
		return
	}
	if e := a.DB.Model(&AccountWelcome{}).Where("account_id=? AND pending=1", actor(c).ID).Updates(map[string]any{"pending": false, "choice": req.Choice, "updated_at": time.Now().UTC()}).Error; e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"welcome_pending": false})
}

func (a *App) unifiedPasswordLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if c.ShouldBindJSON(&req) != nil || utf8.RuneCountInString(req.Username) > 100 || len(req.Password) > 72 {
		fail(c, 400, "invalid_request")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if !permit("unified-password-ip:"+c.ClientIP(), 10, time.Minute) || !permit("unified-password-name:"+digest(req.Username), 20, time.Minute) {
		fail(c, 429, "rate_limited")
		return
	}
	var out gin.H
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		var rows []Account
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("role IN ('visitor','merchant') AND (login_name=? OR (phone=? AND phone_verified_at IS NOT NULL))", req.Username, req.Username).Limit(2).Find(&rows).Error; e != nil {
			return e
		}
		hash := missingAccountHash
		if len(rows) == 1 && rows[0].PasswordHash != "" {
			hash = rows[0].PasswordHash
		}
		matched := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) == nil
		if len(rows) != 1 || rows[0].PasswordHash == "" || !matched {
			return nil
		}
		var e error
		out, e = a.phoneSession(tx, rows[0], false)
		return e
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	if out == nil {
		fail(c, 401, "invalid_credentials")
		return
	}
	c.JSON(200, out)
}

// Verification creates only a restricted proof, never an account or session.
func (a *App) verifyPhoneAction(c *gin.Context) {
	var req struct {
		Phone   string `json:"phone"`
		Purpose string `json:"purpose"`
		Code    string `json:"code"`
	}
	if c.ShouldBindJSON(&req) != nil || !mainlandPhone.MatchString(req.Phone) || (req.Purpose != "register" && req.Purpose != "reset_password") {
		fail(c, 400, "invalid_request")
		return
	}
	if !permit("phone-action:"+c.ClientIP(), 20, time.Minute) {
		fail(c, 429, "rate_limited")
		return
	}
	token := randomToken()
	valid := false
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		if e := lockPhoneIdentity(tx, req.Phone); e != nil {
			return e
		}
		if e := a.smsEligible(tx, req.Phone, req.Purpose, 0); e != nil {
			return e
		}
		var e error
		valid, e = verifyPhoneCode(tx, req.Phone, req.Purpose, req.Code, 0)
		if e != nil || !valid {
			return e
		}
		if e := tx.Model(&PhoneActionProof{}).Where("phone=? AND purpose=? AND used=0", req.Phone, req.Purpose).Update("used", true).Error; e != nil {
			return e
		}
		now := time.Now().UTC()
		return tx.Create(&PhoneActionProof{TokenHash: digest(token), Phone: req.Phone, Purpose: req.Purpose, CreatedAt: now, ExpiresAt: now.Add(5 * time.Minute)}).Error
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	if !valid {
		fail(c, 400, "invalid_code")
		return
	}
	c.JSON(200, gin.H{"action_token": token, "expires_in": 300})
}
func consumePhoneAction(tx *gorm.DB, token, phone, purpose string) (bool, error) {
	if len(token) != 64 {
		return false, nil
	}
	var p PhoneActionProof
	e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_hash=?", digest(token)).First(&p).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	if p.Used || p.Phone != phone || p.Purpose != purpose || !time.Now().UTC().Before(p.ExpiresAt) {
		return false, nil
	}
	e = tx.Model(&p).Update("used", true).Error
	return e == nil, e
}
func (a *App) resetPhonePassword(c *gin.Context) {
	var req struct {
		Phone       string `json:"phone"`
		ActionToken string `json:"action_token"`
		Password    string `json:"password"`
	}
	if c.ShouldBindJSON(&req) != nil || !mainlandPhone.MatchString(req.Phone) {
		fail(c, 400, "invalid_phone")
		return
	}
	if !validNewPassword(req.Password) {
		dbFail(c, fieldFailure("invalid_request", "password", "password_length"))
		return
	}
	if !permit("phone-reset:"+c.ClientIP(), 10, time.Minute) {
		fail(c, 429, "rate_limited")
		return
	}
	hashed, e := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if e != nil {
		dbFail(c, e)
		return
	}
	valid := false
	e = a.DB.Transaction(func(tx *gorm.DB) error {
		if e := lockPhoneIdentity(tx, req.Phone); e != nil {
			return e
		}
		var account Account
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("phone=? AND phone_verified_at IS NOT NULL AND role IN ('merchant','visitor')", req.Phone).First(&account).Error; e != nil {
			return e
		}
		var e error
		valid, e = consumePhoneAction(tx, req.ActionToken, req.Phone, "reset_password")
		if e != nil || !valid {
			return e
		}
		if e := tx.Model(&account).Update("password_hash", string(hashed)).Error; e != nil {
			return e
		}
		if e := tx.Where("account_id=?", account.ID).Delete(&Session{}).Error; e != nil {
			return e
		}
		if e := tx.Model(&PhoneActionProof{}).Where("phone=? AND purpose='reset_password'", req.Phone).Update("used", true).Error; e != nil {
			return e
		}
		return recordOperation(tx, account.ID, "account", account.ID, "phone_password_reset", phoneMask(req.Phone))
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	if !valid {
		fail(c, 400, "phone_action_expired")
		return
	}
	c.JSON(200, gin.H{"reset": true})
}

func accountContactMask(a Account) string {
	if a.Phone != nil {
		return phoneMask(*a.Phone)
	}
	return "username-account"
}
