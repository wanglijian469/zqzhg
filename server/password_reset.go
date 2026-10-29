package main

import (
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"strings"
)

func (a *App) adminPasswordReset(c *gin.Context) {
	var req struct {
		Mode     string `json:"mode"`
		Password string `json:"password"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	password := req.Password
	switch req.Mode {
	case "generate":
		if password != "" {
			dbFail(c, fieldFailure("invalid_request", "password", "generated_password_only"))
			return
		}
		password = "Ad-" + randomToken()[:12] + "!"
	case "manual":
		if !validNewPassword(password) {
			dbFail(c, fieldFailure("invalid_request", "password", "password_length"))
			return
		}
	default:
		dbFail(c, fieldFailure("invalid_request", "mode", "invalid_mode"))
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		dbFail(c, err)
		return
	}
	var account Account
	var revoked int64
	var loginName string
	// The SQL logger must not echo the password hash on errors or slow updates.
	err = a.DB.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Transaction(func(tx *gorm.DB) error {
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND role IN ('merchant','visitor')", id(c)).First(&account).Error; e != nil {
			return e
		}
		if (account.LoginName == nil || strings.TrimSpace(*account.LoginName) == "") && !verifiedPhone(account) {
			return fieldFailure("invalid_request", "account_id", "password_login_unavailable")
		}
		if e := tx.Model(&account).Update("password_hash", string(hash)).Error; e != nil {
			return e
		}
		result := tx.Delete(&Session{}, "account_id=?", account.ID)
		if result.Error != nil {
			return result.Error
		}
		revoked = result.RowsAffected
		if account.LoginName != nil {
			loginName = *account.LoginName
		} else {
			loginName = *account.Phone
		}
		note := loginName
		if verifiedPhone(account) {
			note = phoneMask(*account.Phone)
		}
		return recordOperation(tx, actor(c).ID, "account", account.ID, "account_password_reset", note)
	})
	if err != nil {
		dbFail(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"account_id": account.ID, "login_name": loginName, "password": password, "revoked_sessions": revoked})
}
