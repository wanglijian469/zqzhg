package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Permission struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	NameEN string `json:"name_en"`
}

var permissionCatalog = []Permission{
	{"vendor.read", "查看厂商", "View manufacturers"}, {"vendor.write", "编辑厂商", "Edit manufacturers"}, {"vendor.review", "审核厂商", "Review manufacturers"}, {"vendor.offline", "下架厂商", "Unpublish manufacturers"},
	{"product.read", "查看产品", "View products"}, {"product.write", "编辑产品", "Edit products"}, {"product.review", "审核产品", "Review products"}, {"product.offline", "下架产品", "Unpublish products"},
	{"resource.read", "查看视频和图册", "View media resources"}, {"resource.write", "编辑视频和图册", "Edit media resources"}, {"resource.review", "审核视频和图册", "Review media resources"}, {"resource.offline", "下架视频和图册", "Unpublish media resources"},
	{"taxonomy.manage", "管理分类与地区", "Manage categories and regions"}, {"home.manage", "管理首页展示", "Manage homepage"}, {"analytics.read", "查看访问分析", "View analytics"}, {"analytics.export", "导出访问分析", "Export analytics"}, {"audit.read", "查看操作日志", "View audit logs"}, {"system.read", "查看系统状态", "View system status"},
}

type PlatformOperator struct {
	AccountID   uint64    `gorm:"primaryKey" json:"account_id"`
	DisplayName string    `json:"display_name"`
	Enabled     bool      `json:"enabled"`
	Permissions string    `json:"-"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func platformAccount(account Account) bool {
	return account.Role == "admin" || account.Role == "operator"
}
func normalizePermissions(values []string) ([]string, error) {
	set := map[string]bool{}
	for _, key := range values {
		found := false
		for _, p := range permissionCatalog {
			if p.Key == key {
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("invalid_permission")
		}
		set[key] = true
		parts := strings.Split(key, ".")
		if len(parts) == 2 && (parts[1] == "write" || parts[1] == "review" || parts[1] == "offline") {
			set[parts[0]+".read"] = true
		}
		if key == "home.manage" {
			set["vendor.read"] = true
			set["product.read"] = true
		}
		if key == "analytics.export" {
			set["analytics.read"] = true
		}
		if key == "product.write" || key == "resource.write" {
			set["vendor.read"] = true
		}
		if key == "resource.write" {
			set["product.read"] = true
		}
	}
	out := []string{}
	for _, p := range permissionCatalog {
		if set[p.Key] {
			out = append(out, p.Key)
		}
	}
	return out, nil
}
func (a *App) permissions(account Account) []string {
	if account.Role == "admin" {
		out := []string{}
		for _, p := range permissionCatalog {
			out = append(out, p.Key)
		}
		return out
	}
	if account.Role != "operator" {
		return []string{}
	}
	var operator PlatformOperator
	if a.DB.First(&operator, "account_id=? AND enabled=1", account.ID).Error != nil {
		return []string{}
	}
	values := []string{}
	_ = json.Unmarshal([]byte(operator.Permissions), &values)
	return values
}
func (a *App) hasPermission(account Account, key string) bool {
	if account.Role == "admin" {
		return true
	}
	if account.Role == "operator" && key == "platform.dashboard" {
		return true
	}
	if account.Role == "operator" && (key == "platform.taxonomy" || key == "platform.contents") {
		for _, p := range a.permissions(account) {
			if strings.HasSuffix(p, ".read") || p == "taxonomy.manage" || p == "home.manage" {
				return true
			}
		}
		return false
	}
	for _, p := range a.permissions(account) {
		if p == key {
			return true
		}
	}
	return false
}
func (a *App) accountJSON(account Account) gin.H {
	return gin.H{"id": account.ID, "username": account.Username, "login_name": account.LoginName, "role": account.Role, "enabled": true, "created_at": account.CreatedAt, "permissions": a.permissions(account)}
}
func (a *App) platformPolicy() gin.HandlerFunc {
	return func(c *gin.Context) {
		who := actor(c)
		if who.Role == "admin" {
			c.Next()
			return
		}
		if who.Role != "operator" {
			fail(c, 403, "forbidden")
			return
		}
		key := a.permissionForRequest(c)
		if key == "" || !a.hasPermission(who, key) {
			fail(c, 403, "forbidden")
			return
		}
		c.Next()
	}
}
func (a *App) permissionForRequest(c *gin.Context) string {
	path := strings.TrimPrefix(c.Request.URL.Path, "/api/v1/admin/")
	method := c.Request.Method
	switch {
	case path == "dashboard":
		return "platform.dashboard"
	case strings.HasPrefix(path, "analytics"):
		if strings.HasSuffix(path, "export") {
			return "analytics.export"
		}
		return "analytics.read"
	case path == "operation-logs":
		return "audit.read"
	case path == "system":
		return "system.read"
	case strings.HasPrefix(path, "home-display") || path == "halls" || strings.HasPrefix(path, "halls/"):
		return "home.manage"
	case path == "taxonomy" && method == "GET":
		return "platform.taxonomy"
	case strings.HasPrefix(path, "categories") || strings.HasPrefix(path, "regions") || strings.HasPrefix(path, "category-suggestions"):
		return "taxonomy.manage"
	case strings.HasPrefix(path, "resources") || strings.HasPrefix(path, "resource-media") || strings.HasPrefix(path, "resource-revisions"):
		action := "read"
		if method != "GET" && method != "HEAD" {
			action = "write"
		}
		if strings.HasSuffix(path, "/review") {
			action = "review"
		}
		if strings.HasSuffix(path, "/offline") {
			action = "offline"
		}
		return "resource." + action
	case strings.HasPrefix(path, "vendors"):
		if strings.HasSuffix(path, "/products") {
			return "product.write"
		}
		if path == "vendors" {
			return ""
		}
		return "vendor.write"
	case path == "contents":
		kind := c.Query("kind")
		if kind != "vendor" && kind != "product" {
			return "platform.contents"
		}
		return kind + ".read"
	case strings.HasPrefix(path, "contents/"):
		var item Content
		if a.DB.First(&item, id(c)).Error != nil {
			return "platform.missing"
		}
		action := "read"
		if method != "GET" && method != "HEAD" {
			action = "write"
		}
		if strings.HasSuffix(path, "/offline") {
			action = "offline"
		}
		if strings.HasSuffix(path, "/featured") {
			return "home.manage"
		}
		return item.Kind + "." + action
	case strings.HasPrefix(path, "revisions/"):
		var rev Revision
		var item Content
		if a.DB.First(&rev, id(c)).Error != nil || a.DB.First(&item, rev.ContentID).Error != nil {
			return "platform.missing"
		}
		return item.Kind + ".review"
	}
	return ""
}
func (a *App) canReadPrivateMedia(account Account, m Media) bool {
	if account.Role == "admin" || (account.ID == m.OwnerID && account.Role != "operator") {
		return true
	}
	if account.Role != "operator" {
		return false
	}
	// Newly uploaded drafts have no revision reference until the editor saves.
	// The audit row binds the uploader to the explicit content/resource scope.
	var uploads []OperationLog
	a.DB.Where("actor_id=? AND target_kind=? AND target_id=? AND action=?", account.ID, "media", m.ID, "media_uploaded").Find(&uploads)
	for _, upload := range uploads {
		var contentID, resourceID uint64
		if _, e := fmt.Sscanf(upload.Note, "content=%d", &contentID); e == nil {
			var content Content
			if a.DB.First(&content, contentID).Error == nil && a.hasPermission(account, content.Kind+".write") {
				return true
			}
		}
		if _, e := fmt.Sscanf(upload.Note, "resource=%d", &resourceID); e == nil && a.hasPermission(account, "resource.write") {
			var resource Resource
			if a.DB.First(&resource, resourceID).Error == nil {
				return true
			}
		}
	}
	for _, kind := range []string{"vendor", "product"} {
		if !a.hasPermission(account, kind+".read") {
			continue
		}
		var n int64
		a.DB.Table("media_references mr").Joins("JOIN revisions r ON r.id=mr.revision_id").Joins("JOIN contents ct ON ct.id=r.content_id").Where("mr.media_id=? AND ct.kind=?", m.ID, kind).Count(&n)
		if n > 0 {
			return true
		}
	}
	if a.hasPermission(account, "resource.read") {
		var n int64
		a.DB.Table("resource_media_references").Where("media_id=?", m.ID).Count(&n)
		if n > 0 {
			return true
		}
	}
	if a.hasPermission(account, "home.manage") {
		var n int64
		a.DB.Model(&HomeDisplayMedia{}).Where("media_id=?", m.ID).Count(&n)
		if n > 0 {
			return true
		}
	}
	return false
}
func (a *App) operatorRoutes(admin *gin.RouterGroup) {
	admin.GET("/permissions", func(c *gin.Context) { c.JSON(200, gin.H{"items": permissionCatalog}) })
	admin.GET("/operators", a.listOperators)
	admin.POST("/operators", a.saveOperator)
	admin.PUT("/operators/:id", a.saveOperator)
	admin.POST("/operators/:id/password-reset", a.resetOperatorPassword)
}
func (a *App) listOperators(c *gin.Context) {
	var operators []PlatformOperator
	if e := a.DB.Order("account_id DESC").Find(&operators).Error; e != nil {
		dbFail(c, e)
		return
	}
	items := []gin.H{}
	for _, op := range operators {
		var account Account
		if a.DB.First(&account, op.AccountID).Error != nil {
			continue
		}
		var permissions []string
		json.Unmarshal([]byte(op.Permissions), &permissions)
		items = append(items, gin.H{"id": account.ID, "username": account.Username, "display_name": op.DisplayName, "enabled": op.Enabled, "permissions": permissions, "updated_at": op.UpdatedAt})
	}
	c.JSON(200, gin.H{"items": items})
}
func (a *App) saveOperator(c *gin.Context) {
	var req struct {
		Username    string   `json:"username"`
		DisplayName string   `json:"display_name"`
		Enabled     bool     `json:"enabled"`
		Permissions []string `json:"permissions"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	grants, e := normalizePermissions(req.Permissions)
	if e != nil {
		fail(c, 400, "invalid_permission")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if len(req.DisplayName) > 100 {
		fail(c, 400, "invalid_request")
		return
	}
	var account Account
	password := ""
	isNew := c.Request.Method == http.MethodPost
	e = a.DB.Transaction(func(tx *gorm.DB) error {
		if isNew {
			if len(req.Username) < 3 || len(req.Username) > 100 || strings.ContainsAny(req.Username, "\r\n\t ") {
				return errors.New("invalid_request")
			}
			password = randomToken()[:24]
			hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			account = Account{Username: req.Username, Role: "operator", PasswordHash: string(hash)}
			if err = tx.Create(&account).Error; err != nil {
				return errors.New("login_name_exists")
			}
		} else {
			if err := tx.Where("role='operator'").First(&account, id(c)).Error; err != nil {
				return gorm.ErrRecordNotFound
			}
		}
		data, _ := json.Marshal(grants)
		op := PlatformOperator{AccountID: account.ID, DisplayName: strings.TrimSpace(req.DisplayName), Enabled: req.Enabled, Permissions: string(data), UpdatedAt: time.Now().UTC()}
		if err := tx.Save(&op).Error; err != nil {
			return err
		}
		if err := tx.Where("account_id=?", account.ID).Delete(&Session{}).Error; err != nil {
			return err
		}
		return recordOperation(tx, actor(c).ID, "operator", account.ID, "operator_saved", "")
	})
	if e != nil {
		if errors.Is(e, gorm.ErrRecordNotFound) {
			fail(c, 404, "not_found")
		} else if e.Error() == "login_name_exists" || e.Error() == "invalid_request" {
			fail(c, 400, e.Error())
		} else {
			dbFail(c, e)
		}
		return
	}
	status := 200
	if isNew {
		status = 201
	}
	c.JSON(status, gin.H{"id": account.ID, "username": account.Username, "initial_password": password, "permissions": grants})
}
func (a *App) resetOperatorPassword(c *gin.Context) {
	var req struct {
		Password string `json:"password"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	password := req.Password
	if password == "" {
		password = randomToken()[:24]
	}
	if len(password) < 12 || len(password) > 72 {
		fail(c, 400, "invalid_password")
		return
	}
	hash, e := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if e != nil {
		dbFail(c, e)
		return
	}
	accountID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	e = a.DB.Transaction(func(tx *gorm.DB) error {
		var account Account
		if err := tx.Where("role='operator'").First(&account, accountID).Error; err != nil {
			return err
		}
		if err := tx.Model(&account).Update("password_hash", string(hash)).Error; err != nil {
			return err
		}
		if err := tx.Where("account_id=?", account.ID).Delete(&Session{}).Error; err != nil {
			return err
		}
		return recordOperation(tx, actor(c).ID, "operator", account.ID, "operator_password_reset", "")
	})
	if e != nil {
		fail(c, 404, "not_found")
		return
	}
	c.JSON(200, gin.H{"id": accountID, "password": password})
}
