package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func (a *App) backupDownloadRoutes(admin, api *gin.RouterGroup) {
	admin.POST("/backups/:id/download-ticket", func(c *gin.Context) {
		entry, err := a.backupEntry(c.Param("id"))
		if err != nil {
			fail(c, 404, "not_found")
			return
		}
		key, err := a.masterKey()
		if err != nil {
			fail(c, 503, "encryption_key")
			return
		}
		payload := entry.ID + ":" + strconv.FormatInt(time.Now().Add(2*time.Minute).Unix(), 10) + ":" + randomToken()[:16] + ":" + digest(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
		encoded := base64.RawURLEncoding.EncodeToString([]byte(payload))
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte("backup-download:" + encoded))
		ticket := encoded + "." + hex.EncodeToString(mac.Sum(nil))
		c.SetSameSite(http.SameSiteStrictMode)
		secure := c.Request.TLS != nil || strings.HasPrefix(a.Config.PublicBaseURL, "https://")
		c.SetCookie("zqzt_backup_download", ticket, 120, "/api/v1/backup-download/"+entry.ID, "", secure, true)
		a.audit(actor(c).ID, "backup", 0, "backup_download_authorized")
		c.JSON(200, gin.H{"ok": true})
	})
	api.GET("/backup-download/:id", func(c *gin.Context) {
		ticket, err := c.Cookie("zqzt_backup_download")
		if err != nil {
			fail(c, 401, "unauthorized")
			return
		}
		parts := strings.Split(ticket, ".")
		if len(parts) != 2 {
			fail(c, 401, "unauthorized")
			return
		}
		key, err := a.masterKey()
		if err != nil {
			fail(c, 401, "unauthorized")
			return
		}
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte("backup-download:" + parts[0]))
		signature, e := hex.DecodeString(parts[1])
		if e != nil || !hmac.Equal(signature, mac.Sum(nil)) {
			fail(c, 401, "unauthorized")
			return
		}
		decoded, e := base64.RawURLEncoding.DecodeString(parts[0])
		values := strings.Split(string(decoded), ":")
		if e != nil || len(values) != 4 || values[0] != c.Param("id") {
			fail(c, 401, "unauthorized")
			return
		}
		expires, e := strconv.ParseInt(values[1], 10, 64)
		if e != nil || time.Now().Unix() > expires {
			fail(c, 401, "unauthorized")
			return
		}
		var session Session
		if a.DB.Where("token_hash=? AND expires_at>?", values[3], time.Now().UTC()).First(&session).Error != nil {
			fail(c, 401, "unauthorized")
			return
		}
		var adminAccount Account
		if a.DB.Where("id=? AND role='admin'", session.AccountID).First(&adminAccount).Error != nil {
			fail(c, 401, "unauthorized")
			return
		}
		c.Set("account", adminAccount)
		a.downloadBackup(c)
	})
}
