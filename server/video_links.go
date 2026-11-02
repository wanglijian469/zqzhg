package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"
	"gorm.io/gorm/clause"
)

const videoLocal = "LOCAL"
const videoDouyin = "DOUYIN"
const videoRightsVersion = "video-rights-v1"

func douyinVideoEnabled() bool    { return env("DOUYIN_VIDEO_ENABLED", "false") == "true" }
func videoV2(c *gin.Context) bool { return c.Query("video_api_version") == "2" }
func videoSource(p ResourcePayload) string {
	if p.VideoSource == "" {
		return videoLocal
	}
	return p.VideoSource
}

type VideoLinkCheck struct {
	Source       string    `gorm:"primaryKey;size:16" json:"source"`
	VideoID      string    `gorm:"primaryKey;size:64" json:"video_id"`
	Title        string    `json:"title"`
	CanonicalURL string    `json:"canonical_url"`
	PlayerURL    string    `json:"player_url,omitempty"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	Status       string    `json:"status"`
	Reason       string    `json:"reason"`
	CheckedAt    time.Time `json:"checked_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

var videoURLPattern = regexp.MustCompile(`https?://[^\s<>"']+`)
var videoIDPattern = regexp.MustCompile(`^[1-9][0-9]{8,23}$`)
var shareVideoPath = regexp.MustCompile(`^/(?:share/)?video/([1-9][0-9]{8,23})/?$`)
var shortVideoPath = regexp.MustCompile(`^/[A-Za-z0-9_-]{3,80}/?$`)
var videoHTMLPattern = regexp.MustCompile(`(?i)<\s*(?:iframe|script|html|object|embed)\b`)

func validVideoHost(host string) bool {
	switch strings.ToLower(host) {
	case "douyin.com", "www.douyin.com", "v.douyin.com", "iesdouyin.com", "www.iesdouyin.com":
		return true
	}
	return false
}
func validateShareURL(raw string) (*url.URL, error) {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Opaque != "" || u.Fragment != "" || !validVideoHost(u.Hostname()) || (u.Port() != "" && u.Port() != "443") || len(raw) > 2048 {
		return nil, fmt.Errorf("invalid_video_link")
	}
	return u, nil
}
func extractVideoShare(text string) (string, error) {
	if len(text) > 8000 || videoHTMLPattern.MatchString(text) {
		return "", fmt.Errorf("invalid_video_link")
	}
	links := videoURLPattern.FindAllString(text, -1)
	if len(links) != 1 {
		return "", fmt.Errorf("one_video_link_required")
	}
	raw := strings.TrimRight(links[0], "，。！？；、）)]】》")
	if _, e := validateShareURL(raw); e != nil {
		return "", e
	}
	return raw, nil
}
func videoIDFromURL(u *url.URL) string {
	if u.Hostname() == "v.douyin.com" {
		return ""
	}
	if match := shareVideoPath.FindStringSubmatch(u.Path); len(match) == 2 {
		return match[1]
	}
	if u.Path == "/" || u.Path == "/user/self" || strings.HasPrefix(u.Path, "/user/") {
		if id := u.Query().Get("modal_id"); videoIDPattern.MatchString(id) {
			return id
		}
	}
	return ""
}

var videoDeniedNetworks = func() []netip.Prefix {
	var result []netip.Prefix
	for _, p := range []string{"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.168.0.0/16", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4", "fc00::/7", "fe80::/10", "2001:db8::/32", "2001::/32", "2002::/16"} {
		result = append(result, netip.MustParsePrefix(p))
	}
	return result
}()

func videoPublicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if ip.Is6() && (!netip.MustParsePrefix("2000::/3").Contains(ip) || netip.MustParsePrefix("2001::/23").Contains(ip) || netip.MustParsePrefix("3fff::/20").Contains(ip)) {
		return false
	}
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, p := range videoDeniedNetworks {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}
func newVideoHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{Proxy: nil, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 5 * time.Second, MaxIdleConns: 8, IdleConnTimeout: 30 * time.Second}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, e := net.SplitHostPort(address)
		if e != nil || port != "443" || !(validVideoHost(host) || host == "open.douyin.com") {
			return nil, fmt.Errorf("blocked_video_destination")
		}
		ips, e := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if e != nil || len(ips) == 0 {
			return nil, fmt.Errorf("video_dns_failed")
		}
		for _, ip := range ips {
			if !videoPublicIP(ip) {
				return nil, fmt.Errorf("blocked_video_destination")
			}
		}
		var last error
		for _, ip := range ips {
			conn, e := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if e == nil {
				return conn, nil
			}
			last = e
		}
		return nil, last
	}
	return &http.Client{Transport: transport, Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
func (a *App) videoClient() *http.Client {
	a.videoClientOnce.Do(func() {
		if a.videoHTTP == nil {
			a.videoHTTP = newVideoHTTPClient()
		}
	})
	return a.videoHTTP
}
func (a *App) resolveVideoShare(ctx context.Context, text string) (string, string, error) {
	original, e := extractVideoShare(text)
	if e != nil {
		return "", "", e
	}
	u, _ := validateShareURL(original)
	if id := videoIDFromURL(u); id != "" {
		return original, id, nil
	}
	if u.Hostname() != "v.douyin.com" {
		return "", "", fmt.Errorf("video_id_not_found")
	}
	if !shortVideoPath.MatchString(u.Path) {
		return "", "", fmt.Errorf("invalid_video_link")
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	for hops := 0; hops <= 3; hops++ {
		if _, e = validateShareURL(u.String()); e != nil {
			return "", "", e
		}
		if id := videoIDFromURL(u); id != "" {
			return original, id, nil
		}
		if hops == 3 {
			break
		}
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		response, e := a.videoClient().Do(request)
		if e != nil {
			if strings.Contains(e.Error(), "blocked_video_destination") {
				return "", "", fmt.Errorf("invalid_video_link")
			}
			var timeout net.Error
			if errors.Is(e, context.DeadlineExceeded) || (errors.As(e, &timeout) && timeout.Timeout()) {
				return "", "", fmt.Errorf("video_network_timeout")
			}
			return "", "", fmt.Errorf("video_link_unreachable")
		}
		location := response.Header.Get("Location")
		response.Body.Close()
		if response.StatusCode < 300 || response.StatusCode > 399 || location == "" {
			return "", "", fmt.Errorf("video_id_not_found")
		}
		next, e := url.Parse(location)
		if e != nil {
			return "", "", fmt.Errorf("invalid_video_link")
		}
		u = u.ResolveReference(next)
	}
	return "", "", fmt.Errorf("video_redirect_limit")
}

// A syntactically valid official short URL may be retained as an unpublished
// draft when the provider cannot resolve it. It cannot pass review without an ID.
func (a *App) inspectVideoShare(ctx context.Context, text string) (string, string, VideoLinkCheck, error) {
	original, id, e := a.resolveVideoShare(ctx, text)
	if e == nil {
		return original, id, a.checkDouyin(ctx, id), nil
	}
	if e.Error() != "video_link_unreachable" && e.Error() != "video_network_timeout" && e.Error() != "video_id_not_found" {
		return "", "", VideoLinkCheck{}, e
	}
	raw, invalid := extractVideoShare(text)
	if invalid != nil {
		return "", "", VideoLinkCheck{}, invalid
	}
	u, _ := validateShareURL(raw)
	if u.Hostname() != "v.douyin.com" || !shortVideoPath.MatchString(u.Path) {
		return "", "", VideoLinkCheck{}, e
	}
	now := time.Now().UTC()
	return raw, "", VideoLinkCheck{Source: videoDouyin, Status: "TEMPORARY_ERROR", Reason: e.Error(), CheckedAt: now, ExpiresAt: now.Add(30 * time.Second)}, nil
}

func safeDouyinPlayer(code, id string) (string, error) {
	if len(code) > 16384 || !videoIDPattern.MatchString(id) {
		return "", fmt.Errorf("invalid_official_player")
	}
	tokenizer := html.NewTokenizer(strings.NewReader(code))
	found := ""
	count := 0
	for {
		kind := tokenizer.Next()
		if kind == html.ErrorToken {
			if tokenizer.Err() != io.EOF {
				return "", fmt.Errorf("invalid_official_player")
			}
			break
		}
		if kind == html.StartTagToken || kind == html.SelfClosingTagToken {
			tag := tokenizer.Token()
			if tag.Data != "iframe" {
				return "", fmt.Errorf("invalid_official_player")
			}
			count++
			for _, attr := range tag.Attr {
				if attr.Key == "src" {
					if found != "" {
						return "", fmt.Errorf("invalid_official_player")
					}
					found = attr.Val
				}
			}
		}
	}
	u, e := url.Parse(found)
	if e != nil || count != 1 || u.Scheme != "https" || u.Host != "open.douyin.com" || u.User != nil || u.Path != "/player/video" || u.Fragment != "" || u.Query().Get("vid") != id {
		return "", fmt.Errorf("invalid_official_player")
	}
	u.RawQuery = url.Values{"vid": {id}, "autoplay": {"0"}}.Encode()
	return u.String(), nil
}

type videoFlight struct {
	mu    sync.Mutex
	users int
}

func (a *App) lockVideoCheck(id string) func() {
	a.videoFlightMu.Lock()
	v, _ := a.videoFlights.LoadOrStore(id, &videoFlight{})
	f := v.(*videoFlight)
	f.users++
	a.videoFlightMu.Unlock()
	f.mu.Lock()
	return func() {
		f.mu.Unlock()
		a.videoFlightMu.Lock()
		f.users--
		if f.users == 0 {
			a.videoFlights.Delete(id)
		}
		a.videoFlightMu.Unlock()
	}
}
func (a *App) checkDouyin(ctx context.Context, id string) VideoLinkCheck {
	now := time.Now().UTC()
	result := VideoLinkCheck{Source: videoDouyin, VideoID: id, CanonicalURL: "https://www.douyin.com/video/" + id, Status: "TEMPORARY_ERROR", Reason: "video_provider_unavailable", CheckedAt: now, ExpiresAt: now.Add(30 * time.Second)}
	if !douyinVideoEnabled() {
		result.Reason = "douyin_video_disabled"
		return result
	}
	if !videoIDPattern.MatchString(id) {
		result.Reason = "invalid_video_link"
		return result
	}
	unlock := a.lockVideoCheck(id)
	defer unlock()
	var cached VideoLinkCheck
	if a.DB.Where("source=? AND video_id=? AND expires_at>?", videoDouyin, id, now).First(&cached).Error == nil {
		return cached
	}
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://open.douyin.com/api/douyin/v1/video/get_iframe_by_video?video_id="+id, nil)
	response, e := a.videoClient().Do(request)
	var timeout net.Error
	if e != nil && (errors.Is(e, context.DeadlineExceeded) || (errors.As(e, &timeout) && timeout.Timeout())) {
		result.Reason = "video_network_timeout"
	}
	if e == nil {
		defer response.Body.Close()
		raw, readErr := io.ReadAll(io.LimitReader(response.Body, 256*1024+1))
		if errors.Is(readErr, context.DeadlineExceeded) {
			result.Reason = "video_network_timeout"
		}
		var data struct {
			ErrNo *int `json:"err_no"`
			Data  struct {
				IframeCode string `json:"iframe_code"`
				Title      string `json:"video_title"`
				Width      int    `json:"video_width"`
				Height     int    `json:"video_height"`
			} `json:"data"`
		}
		if response.StatusCode == 200 && readErr == nil && len(raw) <= 256*1024 && json.Unmarshal(raw, &data) == nil && data.ErrNo != nil {
			if *data.ErrNo == 0 {
				if player, parseErr := safeDouyinPlayer(data.Data.IframeCode, id); parseErr == nil {
					title := []rune(data.Data.Title)
					if len(title) > 200 {
						title = title[:200]
					}
					result.Title = string(title)
					result.PlayerURL = player
					result.Status = "READY"
					result.Reason = ""
					result.ExpiresAt = now.Add(5 * time.Minute)
					if data.Data.Width > 0 && data.Data.Width <= 8192 && data.Data.Height > 0 && data.Data.Height <= 8192 {
						result.Width = data.Data.Width
						result.Height = data.Data.Height
					}
				} else {
					result.Reason = "invalid_official_player"
				}
			} else if *data.ErrNo == 28003004 {
				result.Status = "UNAVAILABLE"
				result.Reason = "video_not_public"
			} else if *data.ErrNo == 28001007 {
				result.Status = "UNAVAILABLE"
				result.Reason = "invalid_video_link"
			}
		}
	}
	a.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&result)
	return result
}
func (a *App) checkVideoLink(c *gin.Context) {
	if !platformAccount(actor(c)) {
		var n int64
		a.DB.Model(&Content{}).Where("owner_id=? AND kind='vendor'", actor(c).ID).Count(&n)
		if n == 0 {
			fail(c, 403, "vendor_required")
			return
		}
	}
	if !douyinVideoEnabled() {
		fail(c, 503, "douyin_video_disabled")
		return
	}
	if !permit(fmt.Sprintf("video-check:%d", actor(c).ID), 20, time.Minute) {
		fail(c, 429, "rate_limited")
		return
	}
	var req struct {
		Text string `json:"text"`
	}
	c.Request.Body = httpMaxBody(c, 10000)
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_video_link")
		return
	}
	original, _, check, e := a.inspectVideoShare(c.Request.Context(), req.Text)
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"original_share_url": original, "check": check})
}
