package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	mysqldriver "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func recordOperation(tx *gorm.DB, actorID uint64, kind string, target uint64, action, note string) error {
	return tx.Create(&OperationLog{ActorID: actorID, TargetKind: kind, TargetID: target, Action: action, Note: note}).Error
}
func recordReview(tx *gorm.DB, log ReviewLog) error {
	if e := tx.Create(&log).Error; e != nil {
		return e
	}
	var content Content
	if e := tx.First(&content, log.ContentID).Error; e != nil {
		return e
	}
	return tx.Create(&OperationLog{ActorID: log.ActorID, TargetKind: content.Kind, TargetID: log.ContentID, Action: log.Action, Note: log.Note, SourceReviewLogID: &log.ID, CreatedAt: log.CreatedAt}).Error
}
func (a *App) adminCreateVendor(c *gin.Context) {
	var req struct {
		AccountID uint64  `json:"account_id"`
		LoginName string  `json:"login_name"`
		Data      Payload `json:"data"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	req.LoginName = strings.TrimSpace(req.LoginName)
	var account Account
	var content Content
	var password string
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		if req.AccountID != 0 {
			if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND role='merchant'", req.AccountID).First(&account).Error; e != nil {
				return e
			}
		} else {
			if req.LoginName == "" || utf8.RuneCountInString(req.LoginName) > 100 {
				return fieldFailure("invalid_request", "login_name", "login_name_length")
			}
			var count int64
			if e := tx.Model(&Account{}).Where("login_name=? OR username=?", req.LoginName, req.LoginName).Count(&count).Error; e != nil {
				return e
			}
			if count > 0 {
				return fieldFailure("login_name_exists", "login_name", "already_used")
			}
			password = "Ad-" + randomToken()[:24] + "!"
			hash, e := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if e != nil {
				return e
			}
			account = Account{Username: "managed:" + randomToken()[:24], LoginName: &req.LoginName, Role: "merchant", PasswordHash: string(hash)}
			if e = tx.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Create(&account).Error; e != nil {
				var duplicate *mysqldriver.MySQLError
				if errors.As(e, &duplicate) && duplicate.Number == 1062 {
					return fieldFailure("login_name_exists", "login_name", "already_used")
				}
				return e
			}
		}
		var existing int64
		if e := tx.Model(&Content{}).Where("kind='vendor' AND owner_id=?", account.ID).Count(&existing).Error; e != nil {
			return e
		}
		if existing > 0 {
			return fieldFailure("vendor_required", "account_id", "account_has_booth")
		}
		if e := a.validatePayload(tx, account, "vendor", &req.Data); e != nil {
			return e
		}
		content = Content{Kind: "vendor", OwnerID: account.ID, VendorOwnerKey: &account.ID}
		if e := tx.Create(&content).Error; e != nil {
			return e
		}
		if e := makeDraft(tx, &content, req.Data); e != nil {
			return e
		}
		if e := recordOperation(tx, actor(c).ID, "account", account.ID, func() string {
			if req.AccountID == 0 {
				return "account_created"
			}
			return "account_linked"
		}(), ""); e != nil {
			return e
		}
		return recordOperation(tx, actor(c).ID, "vendor", content.ID, "content_created", "")
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	var rev Revision
	a.DB.First(&rev, content.DraftRevisionID)
	c.Header("Cache-Control", "no-store")
	result := gin.H{"item": a.privateView(content, rev), "account_id": account.ID}
	if password != "" {
		result["initial_password"] = password
		result["login_name"] = req.LoginName
	}
	c.JSON(201, result)
}
func (a *App) adminCreateProduct(c *gin.Context) {
	var req struct {
		Data Payload `json:"data"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	var content Content
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		vendor, e := loadOwned(tx, id(c), actor(c), true)
		if e != nil {
			return e
		}
		if vendor.Kind != "vendor" {
			return fmt.Errorf("invalid_request")
		}
		owner := Account{ID: vendor.OwnerID}
		if e = a.validatePayload(tx, owner, "product", &req.Data); e != nil {
			return e
		}
		if _, e = productSlot(tx, vendor.OwnerID); e != nil {
			return e
		}
		content = Content{Kind: "product", VendorID: vendor.ID, OwnerID: vendor.OwnerID}
		if e = tx.Create(&content).Error; e != nil {
			return e
		}
		if e = makeDraft(tx, &content, req.Data); e != nil {
			return e
		}
		return recordOperation(tx, actor(c).ID, "product", content.ID, "content_created", "")
	})
	if e != nil {
		dbFail(c, e)
		return
	}
	var rev Revision
	a.DB.First(&rev, content.DraftRevisionID)
	c.JSON(201, a.privateView(content, rev))
}
func (a *App) adminUpload(c *gin.Context) {
	content, e := loadOwned(a.DB, id(c), actor(c), false)
	if e != nil {
		dbFail(c, e)
		return
	}
	c.Set("upload_owner_id", content.OwnerID)
	c.Set("upload_content_id", content.ID)
	a.upload(c)
}
func (a *App) adminEnrich(items []View) error {
	ids := []uint64{}
	for _, item := range items {
		if item.Kind == "product" {
			ids = append(ids, item.VendorID)
		} else {
			ids = append(ids, item.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	var vendors []struct {
		ID      uint64
		Payload string
	}
	if e := a.DB.Model(&Content{}).Joins("JOIN revisions r ON r.id=contents.draft_revision_id").Where("contents.id IN ? AND contents.kind='vendor'", ids).Select("contents.id,r.payload").Scan(&vendors).Error; e != nil {
		return e
	}
	summaries := map[uint64]*VendorSummary{}
	for _, row := range vendors {
		var p Payload
		if e := json.Unmarshal([]byte(row.Payload), &p); e != nil {
			return e
		}
		summaries[row.ID] = &VendorSummary{ID: row.ID, Name: p.Name, NameEN: p.NameEN, RegionID: p.RegionID, RegionName: p.RegionName}
	}
	var counts []struct {
		VendorID uint64
		Total    int64
	}
	if e := a.DB.Model(&Content{}).Where("kind='product' AND vendor_id IN ?", ids).Select("vendor_id,COUNT(*) AS total").Group("vendor_id").Scan(&counts).Error; e != nil {
		return e
	}
	byVendor := map[uint64]int64{}
	for _, row := range counts {
		byVendor[row.VendorID] = row.Total
	}
	pubIDs := []uint64{}
	for _, item := range items {
		if item.PublishedRevisionID > 0 {
			pubIDs = append(pubIDs, item.PublishedRevisionID)
		}
	}
	var published []Revision
	if len(pubIDs) > 0 {
		if e := a.DB.Where("id IN ?", pubIDs).Find(&published).Error; e != nil {
			return e
		}
	}
	pubs := map[uint64]Revision{}
	for _, r := range published {
		pubs[r.ID] = r
	}
	for i := range items {
		if items[i].Kind == "product" {
			items[i].Vendor = summaries[items[i].VendorID]
		} else {
			items[i].ProductCount = byVendor[items[i].ID]
		}
		if r, ok := pubs[items[i].PublishedRevisionID]; ok {
			v := view(items[i].Content, r)
			items[i].Published = &v
		}
	}
	return nil
}
func (a *App) adminDashboard(c *gin.Context) {
	counts := map[string]int64{}
	for _, kind := range []string{"vendor", "product"} {
		if actor(c).Role == "operator" && !a.hasPermission(actor(c), kind+".read") {
			continue
		}
		var total, pending, public int64
		if e := a.DB.Model(&Content{}).Where("kind=?", kind).Count(&total).Error; e != nil {
			dbFail(c, e)
			return
		}
		if e := a.DB.Model(&Content{}).Joins("JOIN revisions r ON r.id=contents.draft_revision_id").Where("contents.kind=? AND r.status='pending'", kind).Count(&pending).Error; e != nil {
			dbFail(c, e)
			return
		}
		if e := a.publicQuery(kind).Count(&public).Error; e != nil {
			dbFail(c, e)
			return
		}
		counts[kind+"_total"] = total
		counts[kind+"_pending"] = pending
		counts[kind+"_public"] = public
	}
	var logs []OperationLog
	if e := a.DB.Where(func() string {
		if a.hasPermission(actor(c), "audit.read") {
			return "1=1"
		}
		return "1=0"
	}()).Order("id DESC").Limit(8).Find(&logs).Error; e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"counts": counts, "recent_operations": logs, "account": a.accountJSON(actor(c))})
}

type AdminAccountView struct {
	ID            uint64    `json:"id"`
	Username      string    `json:"username"`
	LoginName     *string   `json:"login_name"`
	CreatedAt     time.Time `json:"created_at"`
	VendorID      uint64    `json:"vendor_id"`
	VendorName    string    `json:"vendor_name"`
	ProductCount  int64     `json:"product_count"`
	PasswordLogin bool      `json:"password_login"`
	PhoneLogin    bool      `json:"phone_login"`
}

func (a *App) adminAccounts(c *gin.Context) {
	p, n := page(c)
	q := a.DB.Model(&Account{}).Where("accounts.role='merchant'")
	if value := c.Query("account_id"); value != "" {
		target, err := strconv.ParseUint(value, 10, 64)
		if err != nil || target == 0 {
			fail(c, 400, "invalid_request")
			return
		}
		q = q.Where("accounts.id=?", target)
	}
	if s := strings.TrimSpace(c.Query("q")); s != "" {
		k := keyword(c)
		q = q.Where("accounts.username LIKE ? OR accounts.login_name LIKE ?", k, k)
	}
	if c.Query("unassigned") == "1" {
		q = q.Where("NOT EXISTS (SELECT 1 FROM contents v WHERE v.owner_id=accounts.id AND v.kind='vendor')")
	}
	var total int64
	if e := q.Count(&total).Error; e != nil {
		dbFail(c, e)
		return
	}
	var accounts []Account
	if e := q.Order("accounts.id DESC").Offset((p - 1) * n).Limit(n).Find(&accounts).Error; e != nil {
		dbFail(c, e)
		return
	}
	items := []AdminAccountView{}
	for _, account := range accounts {
		row := AdminAccountView{ID: account.ID, Username: account.Username, LoginName: account.LoginName, CreatedAt: account.CreatedAt, PasswordLogin: account.LoginName != nil && account.PasswordHash != "", PhoneLogin: regexp.MustCompile(`^1[3-9][0-9]{9}$`).MatchString(account.Username)}
		var content Content
		e := a.DB.Where("kind='vendor' AND owner_id=?", account.ID).First(&content).Error
		if e == nil {
			row.VendorID = content.ID
			var rev Revision
			if e = a.DB.First(&rev, content.DraftRevisionID).Error; e != nil {
				dbFail(c, e)
				return
			}
			row.VendorName = rev.Name
			if e = a.DB.Model(&Content{}).Where("kind='product' AND vendor_id=?", content.ID).Count(&row.ProductCount).Error; e != nil {
				dbFail(c, e)
				return
			}
		} else if !errors.Is(e, gorm.ErrRecordNotFound) {
			dbFail(c, e)
			return
		}
		items = append(items, row)
	}
	c.JSON(200, gin.H{"items": items, "total": total, "page": p, "page_size": n})
}
func (a *App) adminLogs(c *gin.Context) {
	p, n := page(c)
	q := a.DB.Model(&OperationLog{})
	for _, key := range []string{"actor_id", "target_kind", "target_id", "action"} {
		if value := c.Query(key); value != "" {
			q = q.Where(key+" = ?", value)
		}
	}
	for _, part := range []struct{ Key, Op string }{{"from", ">="}, {"to", "<="}} {
		if value := c.Query(part.Key); value != "" {
			parsed, e := time.Parse(time.RFC3339, value)
			if e != nil {
				fail(c, 400, "invalid_request")
				return
			}
			q = q.Where("created_at "+part.Op+" ?", parsed.UTC())
		}
	}
	var total int64
	if e := q.Count(&total).Error; e != nil {
		dbFail(c, e)
		return
	}
	var logs []OperationLog
	if e := q.Order("created_at DESC,id DESC").Offset((p - 1) * n).Limit(n).Find(&logs).Error; e != nil {
		dbFail(c, e)
		return
	}
	ids := []uint64{}
	for _, log := range logs {
		ids = append(ids, log.ActorID)
	}
	var actors []Account
	if len(ids) > 0 {
		if e := a.DB.Where("id IN ?", ids).Select("id,username,login_name").Find(&actors).Error; e != nil {
			dbFail(c, e)
			return
		}
	}
	names := map[uint64]string{}
	for _, account := range actors {
		names[account.ID] = account.Username
		if account.LoginName != nil {
			names[account.ID] = *account.LoginName
		}
	}
	items := []gin.H{}
	for _, log := range logs {
		actorName := names[log.ActorID]
		if log.TargetKind == "job" {
			var snapshot struct {
				ActorName string `json:"actor_name"`
			}
			if json.Unmarshal([]byte(log.Note), &snapshot) == nil && snapshot.ActorName != "" {
				actorName = snapshot.ActorName
			}
		}
		items = append(items, gin.H{"id": log.ID, "actor_id": log.ActorID, "actor_name": actorName, "target_kind": log.TargetKind, "target_id": log.TargetID, "action": log.Action, "note": log.Note, "created_at": log.CreatedAt})
	}
	c.JSON(200, gin.H{"items": items, "total": total, "page": p, "page_size": n})
}
func (a *App) adminSystem(c *gin.Context) {
	pool, e := a.DB.DB()
	dbOK := e == nil && pool.Ping() == nil
	mediaOK := false
	var files int64
	var bytes int64
	entries, e := os.ReadDir(a.Config.MediaDir)
	if e == nil {
		mediaOK = true
		for _, entry := range entries {
			if !entry.IsDir() {
				info, e := entry.Info()
				if e == nil {
					files++
					bytes += info.Size()
				}
			}
		}
	}
	configured := true
	for _, key := range []string{"ALIYUN_ACCESS_KEY_ID", "ALIYUN_ACCESS_KEY_SECRET", "ALIYUN_SMS_SIGN_NAME", "ALIYUN_SMS_TEMPLATE_CODE"} {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			configured = false
		}
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"service": "ok", "environment": a.Config.Env, "database_ok": dbOK, "media_ok": mediaOK, "media_files": files, "media_bytes": bytes, "sms_provider": a.Config.SMSProvider, "aliyun_configured": configured, "checked_at": time.Now().UTC(), "product_limit": maxVendorProducts})
}
func uintQuery(c *gin.Context, key string) uint64 {
	v, _ := strconv.ParseUint(c.Query(key), 10, 64)
	return v
}
