package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type videoRoundTrip func(*http.Request) (*http.Response, error)

func (f videoRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func videoResponse(s string) *http.Response {
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(s))}
}
func mockVideoClient(a *App, calls *atomic.Int32) {
	a.videoHTTP = &http.Client{Transport: videoRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		id := r.URL.Query().Get("video_id")
		if id == "7382166514377510182" {
			return videoResponse(`{"err_no":28003004}`), nil
		}
		b, _ := json.Marshal(gin.H{"err_no": 0, "data": gin.H{"video_title": "授权测试视频", "video_width": 1920, "video_height": 1080, "iframe_code": fmt.Sprintf(`<iframe src="https://open.douyin.com/player/video?vid=%s&autoplay=1" allowfullscreen></iframe>`, id)}})
		return videoResponse(string(b)), nil
	})}
}
func TestVideoLinkValidation(t *testing.T) {
	const id = "7690195495465012480"
	a := &App{}
	for _, s := range []string{"https://www.douyin.com/video/" + id, "分享文字 https://www.douyin.com/user/self?modal_id=" + id + " 复制打开", "https://www.iesdouyin.com/share/video/" + id + "。"} {
		_, got, e := a.resolveVideoShare(t.Context(), s)
		if e != nil || got != id {
			t.Fatal(s, got, e)
		}
	}
	for _, s := range []string{"http://www.douyin.com/video/" + id, "https://www.douyin.com.evil.test/video/" + id, "https://127.0.0.1/video/" + id, "https://www.douyin.com:8443/video/" + id, "https://x@www.douyin.com/video/" + id, "<iframe src='https://www.douyin.com/video/" + id + "'></iframe>", "https://www.douyin.com/video/" + id + " https://www.douyin.com/video/" + id, "https://www.kuaishou.com/short-video/123", "https://www.douyin.com/user/self"} {
		if _, _, e := a.resolveVideoShare(t.Context(), s); e == nil {
			t.Fatal("accepted", s)
		}
	}
	for _, ip := range []string{"127.0.0.1", "10.0.0.2", "169.254.169.254", "100.64.0.1", "192.0.2.1", "::1", "::ffff:127.0.0.1", "fd00::1", "2001:db8::1", "2002:7f00:1::1"} {
		if videoPublicIP(netip.MustParseAddr(ip)) {
			t.Fatal("private", ip)
		}
	}
	if !videoPublicIP(netip.MustParseAddr("8.8.8.8")) {
		t.Fatal("public rejected")
	}
	good := `<iframe src="https://open.douyin.com/player/video?vid=` + id + `&amp;autoplay=1"></iframe>`
	if url, e := safeDouyinPlayer(good, id); e != nil || !strings.Contains(url, "autoplay=0") {
		t.Fatal(url, e)
	}
	for _, html := range []string{good + good, `<script>alert(1)</script>` + good, strings.Replace(good, "open.douyin.com", "evil.test", 1), strings.Replace(good, id, "111111111", 1), strings.Replace(good, "https:", "http:", 1)} {
		if _, e := safeDouyinPlayer(html, id); e == nil {
			t.Fatal("iframe accepted", html)
		}
	}
}
func TestVideoLinkShortRedirects(t *testing.T) {
	for _, destination := range []string{"http://127.0.0.1/", "https://127.0.0.1/", "https://evil.test/", "https://www.douyin.com:8080/video/7690195495465012480", "https://v.douyin.com/loop/", "https://www.douyin.com/video/7690195495465012480"} {
		var calls atomic.Int32
		a := &App{videoHTTP: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, Transport: videoRoundTrip(func(*http.Request) (*http.Response, error) {
			calls.Add(1)
			return &http.Response{StatusCode: 302, Header: http.Header{"Location": {destination}}, Body: io.NopCloser(strings.NewReader(""))}, nil
		})}}
		_, id, e := a.resolveVideoShare(t.Context(), "https://v.douyin.com/example/")
		if strings.Contains(destination, "/video/769") && strings.Contains(destination, "com/video") {
			if e != nil || id != "7690195495465012480" {
				t.Fatal(id, e)
			}
		} else if e == nil {
			t.Fatal("redirect accepted", destination)
		}
		if calls.Load() > 3 {
			t.Fatal("redirect overflow")
		}
	}
}
func TestVideoLinksLifecycleV2(t *testing.T) {
	a := importTestApp(t)
	t.Setenv("DOUYIN_VIDEO_ENABLED", "true")
	var calls atomic.Int32
	mockVideoClient(a, &calls)
	tc := testClient{t, a.router()}
	merchant := resourceTestSession(t, a, "merchant")
	admin := resourceTestSession(t, a, "admin")
	other := resourceTestSession(t, a, "merchant")
	vendor := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "视频升级隔离测试", MainProducts: "机械", RegionName: "河北", Phone: "13900009181"}}, 201)
	tc.call("POST", "/merchant/service-acceptances", merchant, gin.H{"agreements": serviceTestRefs(t, a, "terms", "privacy", "showroom-agreement"), "source": "web"}, 200)
	tc.call("POST", "/merchant/contents/"+number(vendor["id"])+"/submit", merchant, gin.H{"revision_id": vendor["revision_id"]}, 200)
	tc.call("POST", "/admin/revisions/"+number(vendor["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	check := tc.call("POST", "/merchant/video-links/check?video_api_version=2", merchant, gin.H{"text": "https://www.douyin.com/user/self?modal_id=7690195495465012480"}, 200)
	if check["check"].(map[string]any)["status"] != "READY" {
		t.Fatal(check)
	}
	p := ResourcePayload{Title: "抖音测试", Format: "video", Purpose: "company", VideoSource: videoDouyin, OriginalShareURL: "https://www.douyin.com/video/7690195495465012480", RightsConfirmed: true}
	tc.call("POST", "/merchant/resources", merchant, gin.H{"kind": "video", "data": p}, 409)
	r := tc.call("POST", "/merchant/resources?video_api_version=2", merchant, gin.H{"kind": "video", "data": p}, 201)
	rid := number(r["id"])
	tc.call("POST", "/merchant/resources?video_api_version=2", merchant, gin.H{"kind": "video", "data": p}, 409)
	tc.call("GET", "/merchant/resources/"+rid+"?video_api_version=2", other, nil, 403)
	tc.call("GET", "/merchant/resources/"+rid+"/playback?video_api_version=2", other, nil, 403)
	tc.call("GET", "/merchant/resources/"+rid, merchant, nil, 409)
	if list := tc.call("GET", "/merchant/resources?kind=video", merchant, nil, 200); list["total"] != float64(0) {
		t.Fatal("old client", list)
	}
	tc.call("GET", "/public/resources/"+rid+"/playback?video_api_version=2", "", nil, 404)
	tc.call("POST", "/merchant/resources/"+rid+"/submit?video_api_version=2", merchant, gin.H{"revision_id": r["revision_id"]}, 200)
	tc.call("POST", "/admin/resource-revisions/"+number(r["revision_id"])+"/review?video_api_version=2", admin, gin.H{"decision": "approve"}, 200)
	tc.call("GET", "/public/resources/"+rid+"/playback?video_api_version=2", "", nil, 200)
	// Rollback disables external playback without removing records or changing local APIs.
	t.Setenv("DOUYIN_VIDEO_ENABLED", "false")
	tc.call("GET", "/public/resources/"+rid+"?video_api_version=2", "", nil, 404)
	tc.call("GET", "/public/resources/"+rid+"/playback?video_api_version=2", "", nil, 503)
	tc.call("POST", "/merchant/video-links/check?video_api_version=2", merchant, gin.H{"text": p.OriginalShareURL}, 503)
	t.Setenv("DOUYIN_VIDEO_ENABLED", "true")
	for _, route := range a.router().Routes() {
		if strings.Contains(route.Path, "/resources/order") || strings.Contains(route.Path, "/resources/cleanup") || strings.Contains(route.Path, "/resources/:id/cleanup") || (route.Method == "DELETE" && strings.HasSuffix(route.Path, "/resources/:id")) {
			t.Fatal("out-of-scope video management route", route.Method, route.Path)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("cache", calls.Load())
	}
	// Changing an already public video creates a draft; the published version remains visible.
	p.OriginalShareURL = "https://www.douyin.com/video/7382166514377510182"
	p.Title = "不可播放草稿"
	draft := tc.call("PUT", "/merchant/resources/"+rid+"/draft?video_api_version=2", merchant, gin.H{"revision_id": r["revision_id"], "data": p}, 200)
	tc.call("POST", "/merchant/resources/"+rid+"/submit?video_api_version=2", merchant, gin.H{"revision_id": draft["revision_id"]}, 409)
	pub := tc.call("GET", "/public/resources/"+rid+"?video_api_version=2", "", nil, 200)
	if pub["data"].(map[string]any)["title"] != "抖音测试" {
		t.Fatal("draft leaked")
	}
	tc.call("POST", "/merchant/resources/"+rid+"/offline?video_api_version=2", merchant, gin.H{}, 200)
	tc.call("GET", "/public/resources/"+rid+"/playback?video_api_version=2", "", nil, 404)
	// Source cannot convert an existing resource, including a historical local video.
	p.VideoSource = videoLocal
	tc.call("PUT", "/merchant/resources/"+rid+"/draft?video_api_version=2", merchant, gin.H{"revision_id": draft["revision_id"], "data": p}, 400)
	if e := a.DB.Exec("DELETE FROM schema_migrations WHERE version=29").Error; e != nil {
		t.Fatal(e)
	}
	if e := migrate(a.DB); e != nil {
		t.Fatal("migration reentry", e)
	}
}
func TestVideoLinkCacheCoalesces(t *testing.T) {
	a := importTestApp(t)
	t.Setenv("DOUYIN_VIDEO_ENABLED", "true")
	var calls atomic.Int32
	mockVideoClient(a, &calls)
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := a.checkDouyin(context.Background(), "7690195495465012480")
			if r.Status != "READY" {
				t.Error(r)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal(calls.Load())
	}
	a.DB.Model(&VideoLinkCheck{}).Where("video_id=?", "7690195495465012480").Update("expires_at", time.Now().Add(-time.Minute))
	a.checkDouyin(t.Context(), "7690195495465012480")
	if calls.Load() != 2 {
		t.Fatal("expiration", calls.Load())
	}
}
