package main

import (
	"errors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"strings"
	"time"
)

// Unlike the legacy merchant SMS endpoint, this never creates an account or
// claims a company by its login name. Only an explicitly verified phone logs in.
func (a *App) phoneSMSLogin(c *gin.Context) {
	if a.Config.SMSProvider == "disabled" {
		fail(c, 503, "sms_disabled")
		return
	}
	var req struct {
		Phone string `json:"phone"`
		Code  string `json:"code"`
	}
	if c.ShouldBindJSON(&req) != nil || !mainlandPhone.MatchString(strings.TrimSpace(req.Phone)) {
		fail(c, 400, "invalid_phone")
		return
	}
	req.Phone = strings.TrimSpace(req.Phone)
	if !permit("phone-sms-login-ip:"+c.ClientIP(), 30, time.Minute) || !permit("phone-sms-login-number:"+digest(req.Phone), 20, time.Minute) {
		fail(c, 429, "rate_limited")
		return
	}
	var out gin.H
	unregistered := false
	e := a.DB.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Transaction(func(tx *gorm.DB) error {
		if e := lockPhoneIdentity(tx, req.Phone); e != nil {
			return e
		}
		valid, e := verifyPhoneCode(tx, req.Phone, "login", req.Code, 0)
		if e != nil || !valid {
			return e
		}
		var account Account
		e = tx.Where("phone=? AND phone_verified_at IS NOT NULL AND role IN ('merchant','visitor')", req.Phone).First(&account).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			unregistered = true
			return nil
		}
		if e != nil {
			return e
		}
		out, e = a.phoneSession(tx, account, false)
		if e != nil {
			return e
		}
		return recordOperation(tx, account.ID, "account", account.ID, "phone_sms_login", phoneMask(req.Phone))
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	if unregistered {
		fail(c, 401, "phone_login_unavailable")
		return
	}
	if out == nil {
		fail(c, 401, "invalid_code")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, out)
}
