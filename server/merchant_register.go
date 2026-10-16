package main

import (
	"errors"
	"github.com/gin-gonic/gin"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"strings"
	"time"
	"unicode/utf8"
)

func (a *App) merchantRegister(c *gin.Context) {
	a.registerAccount(c, "merchant")
}
func (a *App) visitorRegister(c *gin.Context) {
	a.registerAccount(c, "visitor")
}
func (a *App) registerAccount(c *gin.Context, role string) {
	if !permit(role+"-register:"+c.ClientIP(), 5, time.Minute) {
		fail(c, 429, "rate_limited")
		return
	}
	var req struct {
		LoginName string `json:"login_name"`
		Password  string `json:"password"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	req.LoginName = strings.TrimSpace(req.LoginName)
	var fields []FieldIssue
	if req.LoginName == "" {
		fields = append(fields, FieldIssue{Field: "login_name", Code: "required"})
	} else if utf8.RuneCountInString(req.LoginName) > 100 {
		fields = append(fields, FieldIssue{Field: "login_name", Code: "too_long", Limit: 100, Unit: "characters"})
	}
	if len(req.Password) < 12 || len(req.Password) > 72 || strings.TrimSpace(req.Password) == "" {
		fields = append(fields, FieldIssue{Field: "password", Code: "password_length"})
	}
	if len(fields) > 0 {
		dbFail(c, &FieldValidationError{Code: "invalid_request", Fields: fields})
		return
	}
	hash, e := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if e != nil {
		dbFail(c, e)
		return
	}
	var result gin.H
	e = a.DB.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Transaction(func(tx *gorm.DB) error {
		var count int64
		if e := tx.Model(&Account{}).Where("login_name=? OR username=?", req.LoginName, req.LoginName).Count(&count).Error; e != nil {
			return e
		}
		if count > 0 {
			return fieldFailure("login_name_exists", "login_name", "already_used")
		}
		account := Account{Username: "self:" + randomToken()[:24], LoginName: &req.LoginName, Role: role, PasswordHash: string(hash)}
		if e := tx.Create(&account).Error; e != nil {
			var duplicate *mysqlDriver.MySQLError
			if errors.As(e, &duplicate) && duplicate.Number == 1062 {
				return fieldFailure("login_name_exists", "login_name", "already_used")
			}
			return e
		}
		var e error
		result, e = newSessionIn(tx, account)
		if e != nil {
			return e
		}
		return recordOperation(tx, account.ID, "account", account.ID, role+"_registered", req.LoginName)
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(201, result)
}
