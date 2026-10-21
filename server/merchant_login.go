package main

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Provision a named login on the imported account without changing content,
// media ownership, source identity, or the separate administrator account.
func (a *App) setMerchantLogin(sourceKey, name, password string) (Account, error) {
	var account Account
	name = strings.TrimSpace(name)
	if !regexp.MustCompile(`^V[0-9]{4,12}$`).MatchString(sourceKey) || name == "" || utf8.RuneCountInString(name) > 100 || len(password) < 12 || len(password) > 72 {
		return account, fmt.Errorf("login name and 12–72 byte password required")
	}
	hash, e := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if e != nil {
		return account, e
	}
	e = a.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("username = ? AND role = 'merchant'", "import:"+sourceKey).First(&account).Error; e != nil {
			return e
		}
		var vendor Content
		if e := tx.Where("kind='vendor' AND owner_id=?", account.ID).First(&vendor).Error; e != nil {
			return e
		}
		var collision int64
		if e := tx.Model(&Account{}).Where("id<>? AND (login_name=? OR username=? OR phone=?)", account.ID, name, name, name).Count(&collision).Error; e != nil {
			return e
		}
		if collision > 0 {
			return fmt.Errorf("login name is already in use")
		}
		if e := tx.Model(&account).Updates(map[string]any{"login_name": name, "password_hash": string(hash)}).Error; e != nil {
			return e
		}
		if e := tx.Delete(&Session{}, "account_id=?", account.ID).Error; e != nil {
			return e
		}
		var admin Account
		if e := tx.Where("role='admin'").Order("id").First(&admin).Error; e != nil {
			return e
		}
		if e := recordReview(tx, ReviewLog{ContentID: vendor.ID, ActorID: admin.ID, Action: "account_login_configured", Note: name}); e != nil {
			return e
		}
		account.LoginName = &name
		return nil
	})
	return account, e
}

func (a *App) merchantLogin(c *gin.Context) {
	a.passwordLogin(c, "merchant")
}
func (a *App) visitorLogin(c *gin.Context) {
	a.passwordLogin(c, "visitor")
}
func (a *App) passwordLogin(c *gin.Context, role string) {
	if !permit(role+"-password:"+c.ClientIP(), 10, time.Minute) {
		fail(c, 429, "rate_limited")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if c.ShouldBindJSON(&req) != nil || utf8.RuneCountInString(req.Username) > 100 || len(req.Password) > 72 {
		fail(c, 400, "invalid_request")
		return
	}
	var session gin.H
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		var account Account
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("(login_name = ? OR (phone=? AND phone_verified_at IS NOT NULL)) AND role = ?", strings.TrimSpace(req.Username), strings.TrimSpace(req.Username), role).First(&account).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		hash := account.PasswordHash
		if hash == "" {
			hash = missingAccountHash
		}
		matched := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) == nil
		if err != nil || account.PasswordHash == "" || !matched {
			return nil
		}
		session, err = a.phoneSession(tx, account, false)
		return err
	})
	if e != nil {
		fail(c, 500, "internal_error")
		return
	}
	if session == nil {
		fail(c, 401, "invalid_credentials")
		return
	}
	c.JSON(200, session)
}

var missingAccountHash = func() string {
	h, _ := bcrypt.GenerateFromPassword([]byte(randomToken()), bcrypt.DefaultCost)
	return string(h)
}()
