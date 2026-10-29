package main

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"regexp"
	"strings"
	"time"
)

var mainlandPhone = regexp.MustCompile(`^1[3-9][0-9]{9}$`)

type PhoneIdentityLock struct {
	Phone string `gorm:"primaryKey"`
}
type PhoneVerification struct {
	Phone     string `gorm:"primaryKey"`
	Purpose   string `gorm:"primaryKey"`
	AccountID uint64 `gorm:"primaryKey;autoIncrement:false"`
	Digest    string
	SendID    string
	Salt      string
	ExpiresAt time.Time
	SentAt    time.Time
	Attempts  int
	Used      bool
}

func phoneAccount(a Account) bool { return a.Role == "merchant" || a.Role == "visitor" }
func verifiedPhone(a Account) bool {
	return a.Phone != nil && mainlandPhone.MatchString(*a.Phone) && a.PhoneVerifiedAt != nil
}
func lockPhoneIdentity(tx *gorm.DB, phone string) error {
	if !mainlandPhone.MatchString(phone) {
		return fieldFailure("invalid_phone", "phone", "invalid")
	}
	if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&PhoneIdentityLock{phone}).Error; e != nil {
		return e
	}
	var row PhoneIdentityLock
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "phone=?", phone).Error
}
func phoneCollision(tx *gorm.DB, phone string, except uint64) (bool, error) {
	var n int64
	e := tx.Model(&Account{}).Where("id<>? AND (phone=? OR username=? OR login_name=?)", except, phone, phone, phone).Count(&n).Error
	return n > 0, e
}
func phonePasswordFields(phone, password string) error {
	fields := []FieldIssue{}
	if !mainlandPhone.MatchString(phone) {
		fields = append(fields, FieldIssue{Field: "phone", Code: "invalid"})
	}
	if !validNewPassword(password) {
		fields = append(fields, FieldIssue{Field: "password", Code: "password_length"})
	}
	if len(fields) > 0 {
		return &FieldValidationError{Code: "invalid_request", Fields: fields}
	}
	return nil
}
func phoneMask(phone string) string {
	if len(phone) == 11 {
		return phone[:3] + "****" + phone[7:]
	}
	return ""
}
func (a *App) phoneSession(tx *gorm.DB, account Account, revoke bool) (gin.H, error) {
	if revoke {
		if e := tx.Delete(&Session{}, "account_id=?", account.ID).Error; e != nil {
			return nil, e
		}
	}
	out, e := newSessionIn(tx, account)
	if e == nil {
		out["account"] = a.accountJSON(account)
		var welcome AccountWelcome
		if tx.Where("account_id=?", account.ID).First(&welcome).Error == nil {
			out["account"].(gin.H)["welcome_pending"] = welcome.Pending
		}
	}
	return out, e
}
func (a *App) phoneCode(c *gin.Context)                           { a.sendPhoneVerification(c, false) }
func (a *App) bindingPhoneCode(c *gin.Context)                    { a.sendPhoneVerification(c, true) }
func (a *App) sendPhoneVerification(c *gin.Context, binding bool) { a.sendSMSCode(c, binding, false) }

// Invalid attempts return normally so counters commit; application errors roll
// back successful code consumption and account changes together.
func verifyPhoneCode(tx *gorm.DB, phone, purpose, code string, accountID uint64) (bool, error) {
	var row PhoneVerification
	e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "phone=? AND purpose=? AND account_id=?", phone, purpose, accountID).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	if row.SendID != "" {
		var attempt SMSSendAttempt
		if e := tx.First(&attempt, "id=?", row.SendID).Error; e != nil {
			return false, e
		}
		if attempt.State != "accepted" && attempt.State != "unknown" && !(attempt.State == "sending" && time.Since(attempt.CreatedAt) > 15*time.Second) {
			return false, nil
		}
	}
	if row.Used || row.Attempts >= 5 || !time.Now().UTC().Before(row.ExpiresAt) {
		return false, nil
	}
	q := tx.Model(&PhoneVerification{}).Where("phone=? AND purpose=? AND account_id=?", phone, purpose, accountID)
	if e := q.Update("attempts", row.Attempts+1).Error; e != nil {
		return false, e
	}
	if len(code) != 6 || subtle.ConstantTimeCompare([]byte(smsCodeDigest(row.Salt, phone, purpose, code)), []byte(row.Digest)) != 1 {
		return false, nil
	}
	if e := q.Update("used", true).Error; e != nil {
		return false, e
	}
	return true, nil
}

type phoneRegistration struct {
	Agreements  []AgreementRef `json:"agreements"`
	Source      string         `json:"source"`
	Phone       string         `json:"phone"`
	Code        string         `json:"code"`
	Password    string         `json:"password"`
	Role        string         `json:"role"`
	ActionToken string         `json:"action_token"`
}

func (a *App) phoneRegister(c *gin.Context)      { a.completePhoneRegistration(c, false, false) }
func (a *App) setupPhonePassword(c *gin.Context) { a.completePhoneRegistration(c, true, false) }
func (a *App) bindPhone(c *gin.Context)          { a.completePhoneRegistration(c, false, true) }
func (a *App) completePhoneRegistration(c *gin.Context, setup, binding bool) {
	if a.Config.SMSProvider == "disabled" {
		fail(c, 503, "sms_disabled")
		return
	}
	if !permit("phone-register:"+c.ClientIP(), 10, time.Minute) {
		fail(c, 429, "rate_limited")
		return
	}
	var req phoneRegistration
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	req.Phone = strings.TrimSpace(req.Phone)
	if binding {
		if !mainlandPhone.MatchString(req.Phone) {
			dbFail(c, fieldFailure("invalid_phone", "phone", "invalid"))
			return
		}
		if !phoneAccount(actor(c)) {
			fail(c, 403, "forbidden")
			return
		}
	} else {
		if e := phonePasswordFields(req.Phone, req.Password); e != nil {
			dbFail(c, e)
			return
		}
		if !setup && req.Role != "" && req.Role != "merchant" && req.Role != "visitor" {
			dbFail(c, fieldFailure("invalid_request", "role", "invalid"))
			return
		}
	}
	var hashed []byte
	var e error
	if !binding {
		hashed, e = bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if e != nil {
			dbFail(c, e)
			return
		}
	}
	var out gin.H
	valid := false
	e = a.DB.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Transaction(func(tx *gorm.DB) error {
		if e := lockPhoneIdentity(tx, req.Phone); e != nil {
			return e
		}
		purpose := "register"
		var subject uint64
		if setup {
			purpose = "setup_password"
		}
		if binding {
			purpose = "bind"
			subject = actor(c).ID
		}
		var ok bool
		var e error
		if req.ActionToken != "" && !setup && !binding {
			ok, e = consumePhoneAction(tx, req.ActionToken, req.Phone, "register")
		} else {
			ok, e = verifyPhoneCode(tx, req.Phone, purpose, req.Code, subject)
		}
		if e != nil || !ok {
			return e
		}
		valid = true
		var account Account
		now := time.Now().UTC()
		action := "phone_registered"
		if binding {
			if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&account, subject).Error; e != nil {
				return e
			}
			if !phoneAccount(account) {
				return fmt.Errorf("forbidden")
			}
			if account.Phone != nil && *account.Phone != "" && *account.Phone != req.Phone {
				return fieldFailure("phone_already_bound", "phone", "already_used")
			}
		} else if setup {
			if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("username=? AND role='merchant' AND password_hash=''", req.Phone).First(&account).Error; e != nil {
				if errors.Is(e, gorm.ErrRecordNotFound) {
					return fieldFailure("password_setup_unavailable", "phone", "invalid")
				}
				return e
			}
			if account.Phone != nil && *account.Phone != "" && *account.Phone != req.Phone {
				return fieldFailure("phone_already_bound", "phone", "already_used")
			}
			action = "phone_password_configured"
		}
		collision, e := phoneCollision(tx, req.Phone, account.ID)
		if e != nil {
			return e
		}
		if collision {
			return fieldFailure("phone_exists", "phone", "already_used")
		}
		if !setup && !binding {
			account = Account{Username: "phone:" + randomToken()[:24], Role: "visitor", Phone: &req.Phone, PhoneVerifiedAt: &now, PasswordHash: string(hashed)}
			if e := tx.Create(&account).Error; e != nil {
				return e
			}
			if e := markWelcomePending(tx, account.ID); e != nil {
				return e
			}
			if e := acceptServiceDocuments(tx, account.ID, req.Agreements, req.Source, []string{"terms", "privacy"}); e != nil {
				return e
			}
		} else {
			updates := map[string]any{"phone": req.Phone, "phone_verified_at": now}
			if setup {
				updates["password_hash"] = string(hashed)
			} else {
				action = "phone_bound"
			}
			if e := tx.Model(&account).Updates(updates).Error; e != nil {
				return e
			}
			account.Phone = &req.Phone
			account.PhoneVerifiedAt = &now
			if setup {
				account.PasswordHash = string(hashed)
			}
		}
		if e := recordOperation(tx, account.ID, "account", account.ID, action, phoneMask(req.Phone)); e != nil {
			return e
		}
		out, e = a.phoneSession(tx, account, setup || binding)
		return e
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	if !valid {
		dbFail(c, fieldFailure("invalid_code", "code", "invalid"))
		return
	}
	c.Header("Cache-Control", "no-store")
	status := 200
	if !setup && !binding {
		status = 201
	}
	c.JSON(status, out)
}
func (a *App) phonePasswordLogin(c *gin.Context) {
	if !permit("phone-password:"+c.ClientIP(), 10, time.Minute) {
		fail(c, 429, "rate_limited")
		return
	}
	var req struct {
		Phone    string `json:"phone"`
		Password string `json:"password"`
	}
	if c.ShouldBindJSON(&req) != nil || !mainlandPhone.MatchString(strings.TrimSpace(req.Phone)) || len(req.Password) > 72 {
		fail(c, 400, "invalid_request")
		return
	}
	req.Phone = strings.TrimSpace(req.Phone)
	if !permit("phone-password-number:"+digest(req.Phone), 20, time.Minute) {
		fail(c, 429, "rate_limited")
		return
	}
	var out gin.H
	e := a.DB.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Transaction(func(tx *gorm.DB) error {
		var account Account
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("phone=? AND phone_verified_at IS NOT NULL AND role IN ('merchant','visitor')", req.Phone).First(&account).Error
		if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		hash := account.PasswordHash
		if hash == "" {
			hash = missingAccountHash
		}
		matched := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) == nil
		if e != nil || account.PasswordHash == "" || !matched {
			return nil
		}
		out, e = a.phoneSession(tx, account, false)
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
	c.Header("Cache-Control", "no-store")
	c.JSON(200, out)
}
func (a *App) upgradePhoneMerchant(c *gin.Context) {
	var req struct {
		Confirmed bool `json:"confirmed"`
	}
	if c.ShouldBindJSON(&req) != nil || !req.Confirmed {
		fail(c, 400, "confirmation_required")
		return
	}
	var out gin.H
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		var account Account
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&account, actor(c).ID).Error; e != nil {
			return e
		}
		if !phoneAccount(account) {
			return fmt.Errorf("forbidden")
		}

		if account.Role == "visitor" {
			if e := tx.Model(&account).Update("role", "merchant").Error; e != nil {
				return e
			}
			account.Role = "merchant"
			if e := recordOperation(tx, account.ID, "account", account.ID, "visitor_upgraded_to_merchant", accountContactMask(account)); e != nil {
				return e
			}
		}
		var e error
		out, e = a.phoneSession(tx, account, true)
		return e
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, out)
}
