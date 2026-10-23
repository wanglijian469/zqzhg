package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type guardConfig struct {
	List, Detail, Total, Media, Vendors, Products, MaxIPs int
	Enabled                                               bool
}

func defaultGuardConfig() guardConfig {
	return guardConfig{settingInt("PUBLIC_LIST_PER_MINUTE", 30), settingInt("PUBLIC_DETAIL_PER_MINUTE", 90), settingInt("PUBLIC_READS_PER_MINUTE", 120), settingInt("PUBLIC_MEDIA_PER_MINUTE", 600), settingInt("PUBLIC_VENDORS_PER_HOUR", 300), settingInt("PUBLIC_PRODUCTS_PER_HOUR", 1200), settingInt("PUBLIC_GUARD_MAX_IPS", 20000), env("PUBLIC_GUARD_ENABLED", "true") != "false"}
}

type guardClient struct {
	Window, Hour, Last, Blocked time.Time
	Counts                      map[string]int
	Vendors, Products           map[string]bool
	Violations                  []time.Time
}
type publicGuard struct {
	mu          sync.Mutex
	cfg         guardConfig
	clients     map[string]*guardClient
	lastCleanup time.Time
	now         func() time.Time
	stats       map[string]uint64
}

func newPublicGuard(cfg guardConfig) *publicGuard {
	return &publicGuard{cfg: cfg, clients: map[string]*guardClient{}, now: time.Now, stats: map[string]uint64{}}
}
func (g *publicGuard) client(ip string, now time.Time) *guardClient {
	if now.Sub(g.lastCleanup) > time.Minute {
		for k, v := range g.clients {
			if now.Sub(v.Last) > time.Hour && now.After(v.Blocked) {
				delete(g.clients, k)
			}
		}
		g.lastCleanup = now
	}
	if x := g.clients[ip]; x != nil {
		x.Last = now
		return x
	}
	if len(g.clients) >= g.cfg.MaxIPs {
		for k, x := range g.clients {
			if now.Sub(x.Last) > time.Hour && now.After(x.Blocked) {
				delete(g.clients, k)
			}
		}
	}
	if len(g.clients) >= g.cfg.MaxIPs {
		return nil
	}
	x := &guardClient{Last: now}
	g.clients[ip] = x
	return x
}
func (g *publicGuard) deny(x *guardClient, now time.Time, reason string) int {
	g.stats[reason]++
	keep := x.Violations[:0]
	for _, t := range x.Violations {
		if now.Sub(t) < 10*time.Minute {
			keep = append(keep, t)
		}
	}
	x.Violations = append(keep, now)
	if len(x.Violations) >= 5 {
		x.Blocked = now.Add(10 * time.Minute)
		x.Violations = nil
		return 600
	}
	return 60
}
func (g *publicGuard) check(ip, kind string, refs map[string]string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	x := g.client(ip, now)
	if x == nil {
		g.stats["capacity"]++
		return 60
	}
	if now.Before(x.Blocked) {
		g.stats["blocked"]++
		return int(x.Blocked.Sub(now).Seconds()) + 1
	}
	if now.Sub(x.Window) >= time.Minute {
		x.Window = now
		x.Counts = map[string]int{}
	}
	if now.Sub(x.Hour) >= time.Hour {
		x.Hour = now
		x.Vendors = map[string]bool{}
		x.Products = map[string]bool{}
	}
	if kind != "" {
		limit := g.cfg.Detail
		if kind == "list" {
			limit = g.cfg.List
		}
		if kind == "media" {
			limit = g.cfg.Media
		}
		if x.Counts[kind] >= limit || (kind != "media" && x.Counts["total"] >= g.cfg.Total) {
			return g.deny(x, now, kind)
		}
		x.Counts[kind]++
		if kind != "media" {
			x.Counts["total"]++
		}
	}
	nv, np := 0, 0
	for key, kind := range refs {
		if kind == "vendor" && !x.Vendors[key] {
			nv++
		}
		if kind == "product" && !x.Products[key] {
			np++
		}
	}
	if len(x.Vendors)+nv > g.cfg.Vendors || len(x.Products)+np > g.cfg.Products {
		return g.deny(x, now, "distinct_content")
	}
	for key, kind := range refs {
		if kind == "vendor" {
			x.Vendors[key] = true
		}
		if kind == "product" {
			x.Products[key] = true
		}
	}
	return 0
}
func guardKind(path string) string {
	if path == "/sitemap.xml" || strings.HasPrefix(path, "/sitemaps/") {
		return "list"
	}
	if path == "/api/v1/favorites" {
		return "list"
	}
	if strings.HasPrefix(path, "/api/v1/media/") || strings.HasPrefix(path, "/media/") && path != "/media/status" {
		return "media"
	}
	p := strings.TrimPrefix(path, "/api/v1/public")
	if p != path {
		if p == "/procurements" || p == "/procurement-vendors" || p == "/industry-map/regions" || p == "/industry-map/vendors" || p == "/vendors" || p == "/products" || p == "/home" || p == "/resources" || p == "/favorites/resolve" || strings.HasSuffix(p, "/resources") {
			return "list"
		}
		if strings.HasPrefix(p, "/vendors/") || strings.HasPrefix(p, "/products/") || strings.HasPrefix(p, "/showrooms/") || strings.HasPrefix(p, "/resources/") {
			return "detail"
		}
		return ""
	}
	if path == "/regions" || path == "/" || path == "/browse" || path == "/browse/products" {
		return "list"
	}
	if strings.HasPrefix(path, "/v/") || strings.HasPrefix(path, "/vendors/") || strings.HasPrefix(path, "/products/") {
		return "detail"
	}
	return ""
}

var deniedAgents = []string{"gptbot", "oai-searchbot", "chatgpt-user", "claudebot", "claude-searchbot", "claude-user", "ccbot", "bytespider", "perplexitybot", "perplexity-user", "meta-externalagent", "meta-externalfetcher", "amazonbot", "google-extended"}

func deniedAgent(ua string) bool {
	ua = strings.ToLower(ua)
	for _, s := range deniedAgents {
		if strings.Contains(ua, s) {
			return true
		}
	}
	return false
}
func robotsPolicy() string {
	// Public runtime APIs are needed when a search crawler renders the React page.
	s := "User-agent: *\nDisallow: /api/\nAllow: /api/v1/public/\nAllow: /api/v1/media/\nDisallow: /admin\nDisallow: /merchant\n\n"
	for _, ua := range deniedAgents {
		s += "User-agent: " + ua + "\nDisallow: /\n\n"
	}
	return s
}

type guardWriter struct {
	gin.ResponseWriter
	buf    bytes.Buffer
	status int
}

func (w *guardWriter) WriteHeader(s int) { w.status = s }
func (w *guardWriter) WriteHeaderNow() {
	if w.status == 0 {
		w.status = 200
	}
}
func (w *guardWriter) Write(b []byte) (int, error)       { w.WriteHeaderNow(); return w.buf.Write(b) }
func (w *guardWriter) WriteString(s string) (int, error) { return w.Write([]byte(s)) }
func (w *guardWriter) Status() int {
	if w.status == 0 {
		return 200
	}
	return w.status
}
func (w *guardWriter) Size() int     { return w.buf.Len() }
func (w *guardWriter) Written() bool { return w.status != 0 }
func collectGuardRefs(v any, refs map[string]string) {
	switch x := v.(type) {
	case []any:
		for _, a := range x {
			collectGuardRefs(a, refs)
		}
	case map[string]any:
		kind, _ := x["kind"].(string)
		if kind == "vendor" || kind == "product" {
			if id, ok := x["id"].(float64); ok {
				refs[fmt.Sprintf("%s:%.0f", kind, id)] = kind
			}
		}
		if vendor, ok := x["vendor"].(map[string]any); ok {
			if id, ok := vendor["id"].(float64); ok {
				refs[fmt.Sprintf("vendor:%.0f", id)] = "vendor"
			}
		}
		for _, a := range x {
			collectGuardRefs(a, refs)
		}
	}
}
func guardFailure(c *gin.Context, seconds int) {
	c.Header("Retry-After", strconv.Itoa(seconds))
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Length", "")
	fail(c, 429, "public_rate_limited")
}
func validPublicQuery(c *gin.Context) bool {
	if utf8.RuneCountInString(c.Query("q")) > 200 || len(c.Request.URL.RawQuery) > 8192 {
		return false
	}
	for _, key := range []string{"page", "page_size"} {
		if s, ok := c.GetQuery(key); ok {
			n, e := strconv.ParseInt(s, 10, 32)
			if e != nil || n < 1 || n > 1000000 || key == "page_size" && n > 100 {
				return false
			}
		}
	}
	return true
}
func (a *App) publicProtection() gin.HandlerFunc {
	a.guardOnce.Do(func() {
		if a.guard == nil {
			a.guard = newPublicGuard(defaultGuardConfig())
		}
	})
	g := a.guard
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		kind := guardKind(path)
		if kind == "" || !g.cfg.Enabled || c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}
		if deniedAgent(c.GetHeader("User-Agent")) {
			g.mu.Lock()
			g.stats["declared_crawler"]++
			g.mu.Unlock()
			fail(c, 403, "automated_collection_denied")
			return
		}
		if !validPublicQuery(c) {
			fail(c, 400, "invalid_request")
			return
		}
		ip := c.ClientIP()
		if wait := g.check(ip, kind, nil); wait > 0 {
			guardFailure(c, wait)
			return
		}
		if kind == "media" {
			c.Next()
			return
		}
		// Buffer only bounded public responses. Streamed media retains its native writer and Range support.
		original := c.Writer
		w := &guardWriter{ResponseWriter: original}
		c.Writer = w
		c.Next()
		c.Writer = original
		if w.Status() < 300 {
			refs := map[string]string{}
			if strings.Contains(w.Header().Get("Content-Type"), "application/json") {
				var data any
				if json.Unmarshal(w.buf.Bytes(), &data) == nil {
					collectGuardRefs(data, refs)
				}
			} else if strings.HasPrefix(path, "/products/") {
				refs["product:"+strings.TrimPrefix(path, "/products/")] = "product"
			} else if strings.HasPrefix(path, "/v/") || strings.HasPrefix(path, "/vendors/") {
				slug := strings.TrimPrefix(path, "/v/")
				if strings.HasPrefix(path, "/vendors/") {
					slug = "v-" + strings.TrimPrefix(path, "/vendors/")
				}
				items, e := a.resolveShowroom(slug)
				if e == nil && len(items) > 0 {
					refs[fmt.Sprintf("vendor:%d", items[0].ID)] = "vendor"
				}
			}
			if wait := g.check(ip, "", refs); wait > 0 {
				guardFailure(c, wait)
				return
			}
		}
		original.WriteHeader(w.Status())
		if c.Request.Method != http.MethodHead {
			original.Write(w.buf.Bytes())
		}
	}
}

func (a *App) protectionStats() gin.H {
	if a.guard == nil {
		return gin.H{}
	}
	g := a.guard
	g.mu.Lock()
	defer g.mu.Unlock()
	stats := map[string]uint64{}
	for k, v := range g.stats {
		stats[k] = v
	}
	return gin.H{"enabled": g.cfg.Enabled, "active_ip_count": len(g.clients), "interceptions": stats}
}
