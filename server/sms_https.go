package main

import (
	"github.com/gin-gonic/gin"
	"net"
	"strings"
)

// New OTP secrets must travel over HTTPS. The legacy HTTP listener retains
// browsing/password compatibility; only the local TLS terminator can assert TLS.
func (a *App) smsHTTPS(c *gin.Context) {
	if a.Config.Env != "production" {
		return
	}
	paths := map[string]bool{"/api/v1/auth/sms": true, "/api/v1/auth/phone": true, "/api/v1/auth/phone/code": true, "/api/v1/auth/phone/challenge": true, "/api/v1/auth/phone/register": true, "/api/v1/auth/password": true, "/api/v1/auth/phone/verify": true, "/api/v1/auth/phone/password/reset": true, "/api/v1/auth/phone/login": true, "/api/v1/auth/phone/password/setup": true, "/api/v1/auth/me/phone": true, "/api/v1/auth/me/phone/code": true}
	if !paths[c.Request.URL.Path] {
		return
	}
	if c.Request.TLS != nil {
		return
	}
	peer := net.ParseIP(c.RemoteIP())
	if peer != nil && peer.IsLoopback() && strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		return
	}
	c.Abort()
	fail(c, 426, "https_required")
}
