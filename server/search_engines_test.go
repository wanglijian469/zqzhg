package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type searchRoundTrip func(*http.Request) (*http.Response, error)

func (f searchRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func searchFixture(t *testing.T, engine string, automatic bool) (*App, SearchOptions, SearchSecrets) {
	t.Helper()
	a := importTestApp(t)
	a.Config.PublicBaseURL = "https://seo.example"
	o := defaultSearchOptions()
	v := o.Engines[engine]
	v.Enabled = true
	v.Automatic = automatic
	o.Engines[engine] = v
	s := SearchSecrets{BaiduToken: "fixture-baidu-private-credential", IndexNowKey: strings.Repeat("a", 64), GoogleClientID: "fixture-client", GoogleClientSecret: "fixture-client-secret", GoogleRefreshToken: "fixture-private-refresh"}
	searchStore(t, a, o, s)
	return a, o, s
}
func searchStore(t *testing.T, a *App, o SearchOptions, s SearchSecrets) {
	t.Helper()
	raw, _ := json.Marshal(s)
	sealed, e := a.sealSecret(raw)
	if e != nil {
		t.Fatal(e)
	}
	payload, _ := json.Marshal(o)
	if e = a.DB.Save(&SearchSettings{ID: 1, Payload: string(payload), SealedSecrets: sealed, Version: 1, UpdatedAt: time.Now().UTC()}).Error; e != nil {
		t.Fatal(e)
	}
}
func mockSearch(t *testing.T, a *App, status int, response string, requests *atomic.Int32) {
	t.Helper()
	a.searchHTTP = &http.Client{Transport: searchRoundTrip(func(r *http.Request) (*http.Response, error) {
		requests.Add(1)
		if r.URL.Scheme != "https" {
			t.Errorf("insecure provider request")
		}
		return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(response)), Request: r}, nil
	})}
}
func mustSearch(t *testing.T, e error) {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
}
func searchAdmin(t *testing.T, a *App) (testClient, string) {
	t.Helper()
	tc := testClient{t, a.router()}
	token := tc.call("POST", "/auth/admin", "", gin.H{"username": "admin", "password": "Import-Test-Admin-2026!"}, 200)["token"].(string)
	return tc, token
}

func TestSearchSettingsSecretsAndVerification(t *testing.T) {
	a := importTestApp(t)
	a.Config.PublicBaseURL = "https://seo.example"
	seoTestIndex(t, a)
	adminPage := seoGet(a.router(), "/admin/seo")
	if adminPage.Code != 200 || !strings.Contains(adminPage.Body.String(), "noindex,follow") {
		t.Fatal("SEO administration must serve a private, non-indexable SPA shell")
	}
	tc, token := searchAdmin(t, a)
	r := tc.call("GET", "/admin/seo/settings", token, nil, 200)
	o := defaultSearchOptions()
	x := o.Engines["baidu"]
	x.Enabled = true
	o.Engines["baidu"] = x
	o.Verification["google"] = "fixture-public-verification-code"
	tc.call("PUT", "/admin/seo/settings", token, gin.H{"version": r["version"], "options": o, "secrets": gin.H{"baidu_token": "do-not-expose-this-token"}, "generate_indexnow": true}, 200)
	r = tc.call("GET", "/admin/seo/settings", token, nil, 200)
	raw, _ := json.Marshal(r)
	if strings.Contains(string(raw), "do-not-expose") || strings.Contains(string(raw), "sealed_secrets") {
		t.Fatal("credentials leaked")
	}
	var row SearchSettings
	mustSearch(t, a.DB.First(&row, 1).Error)
	if strings.Contains(row.SealedSecrets, "do-not-expose") {
		t.Fatal("plaintext persisted")
	}
	w := seoGet(a.router(), "/")
	if !strings.Contains(w.Body.String(), "google-site-verification") || !strings.Contains(w.Body.String(), "fixture-public-verification-code") || strings.Contains(w.Body.String(), "do-not-expose") {
		t.Fatal("verification tags or secret isolation failed")
	}
	key := seoGet(a.router(), "/indexnow.txt")
	if key.Code != 200 || len(key.Body.String()) != 64 {
		t.Fatal("ownership proof unavailable")
	}
	if seoGet(a.router(), "/search-verification/../settings.json").Code == 200 {
		t.Fatal("arbitrary file served")
	}
	tc.call("PUT", "/admin/seo/settings", token, gin.H{"version": 0, "options": o}, 409)
	tc.call("PUT", "/admin/seo/settings", token, gin.H{"version": 1, "options": o, "secrets": gin.H{}}, 200)
	_, _, s, e := a.searchSettings()
	mustSearch(t, e)
	if s.BaiduToken != "do-not-expose-this-token" {
		t.Fatal("blank edit destroyed credential")
	}
	o.Verification["google"] = "<script>alert(1)</script>"
	tc.call("PUT", "/admin/seo/settings", token, gin.H{"version": 2, "options": o}, 400)
	tc.call("GET", "/admin/seo/settings", "", nil, 401)
	reader := Account{Username: "seo-reader", Role: "operator", CreatedAt: time.Now().UTC()}
	mustSearch(t, a.DB.Create(&reader).Error)
	mustSearch(t, a.DB.Create(&PlatformOperator{AccountID: reader.ID, Enabled: true, Permissions: `["seo.read"]`}).Error)
	mustSearch(t, a.DB.Create(&Session{TokenHash: digest(strings.Repeat("e", 64)), AccountID: reader.ID, ExpiresAt: time.Now().Add(time.Hour)}).Error)
	tc.call("GET", "/admin/seo/settings", strings.Repeat("e", 64), nil, 200)
	tc.call("POST", "/admin/seo/process", strings.Repeat("e", 64), gin.H{}, 403)
	perms, e := normalizePermissions([]string{"seo.manage"})
	mustSearch(t, e)
	if !strings.Contains(strings.Join(perms, ","), "seo.read") {
		t.Fatal("manage lacks read")
	}
}

func TestSearchPublicationPrivacyCascadeAndDedup(t *testing.T) {
	a, _, _ := searchFixture(t, "indexnow", true)
	v := v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, "公开企业")
	p := v2Published(t, a, "product", v.ID, 1, 1, 1, false, 0, "公开产品")
	mustSearch(t, a.syncSearchContent(v.ID))
	mustSearch(t, a.syncSearchContent(p.ID))
	mustSearch(t, a.syncSearchContent(p.ID))
	var count int64
	a.DB.Model(&SearchJob{}).Count(&count)
	if count != 2 {
		t.Fatal("duplicate publication jobs", count)
	}
	// A later draft is not a public revision and must not leak or generate another notification.
	draft := Revision{ContentID: p.ID, Status: "draft", Name: "待审秘密", Payload: `{"name":"待审秘密"}`}
	mustSearch(t, a.DB.Create(&draft).Error)
	a.DB.Model(&p).Update("draft_revision_id", draft.ID)
	mustSearch(t, a.syncSearchContent(p.ID))
	a.DB.Model(&SearchJob{}).Count(&count)
	if count != 2 {
		t.Fatal("draft generated submission")
	}
	mustSearch(t, a.DB.Model(&v).Update("visible", false).Error)
	mustSearch(t, markSearchDirty(a.DB, "vendor", v.ID))
	mustSearch(t, a.reconcileSearch())
	a.DB.Model(&SearchJob{}).Where("action='remove'").Count(&count)
	if count != 2 {
		t.Fatal("parent unpublish did not include products", count)
	}
	var calls atomic.Int32
	mockSearch(t, a, 200, "", &calls)
	for i := 0; i < 5; i++ {
		_, e := a.processSearchJob(context.Background())
		mustSearch(t, e)
	}
	if calls.Load() != 2 {
		t.Fatal("obsolete private upserts sent or removal missing", calls.Load())
	}
	var skipped int64
	a.DB.Model(&SearchJob{}).Where("state='skipped'").Count(&skipped)
	if skipped != 2 {
		t.Fatal("stale tasks not skipped", skipped)
	}
}

func TestSearchManualURLAndPreparingHallBoundaries(t *testing.T) {
	a, _, _ := searchFixture(t, "indexnow", false)
	v := v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, "公开厂家")
	p := v2Published(t, a, "product", v.ID, 1, 1, 1, false, 0, "产品")
	tc, token := searchAdmin(t, a)
	for _, raw := range []string{"https://evil.example/v/v-1", "https://seo.example/admin", "https://seo.example/login", "https://seo.example/service-requests?token=private", "https://seo.example/?hall=cable", "https://seo.example/products/999999", "https://seo.example/?q=private", "https://seo.example/#private"} {
		tc.call("POST", "/admin/seo/submit", token, gin.H{"engine": "indexnow", "urls": []string{raw}}, 400)
	}
	tc.call("POST", "/admin/seo/submit", token, gin.H{"engine": "indexnow", "urls": []string{contentSearchURL(searchBase(a), v), contentSearchURL(searchBase(a), p)}}, 200)
	tc.call("POST", "/admin/seo/submit", token, gin.H{"engine": "indexnow", "scope": "public", "urls": []string{contentSearchURL(searchBase(a), v)}}, 400)
	tc.call("POST", "/admin/seo/submit", token, gin.H{"engine": "indexnow", "scope": "unknown"}, 400)
	var calls atomic.Int32
	a.searchHTTP = &http.Client{Transport: searchRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		var payload struct {
			KeyLocation string   `json:"keyLocation"`
			URLs        []string `json:"urlList"`
		}
		mustSearch(t, json.NewDecoder(r.Body).Decode(&payload))
		if payload.KeyLocation != searchBase(a)+"/indexnow.txt" || len(payload.URLs) != 1 {
			t.Fatal("IndexNow proof must cover all public URL directories")
		}
		return &http.Response{StatusCode: 202, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})}
	_, e := a.processSearchJob(context.Background())
	mustSearch(t, e)
	var job SearchJob
	a.DB.Order("id").First(&job)
	if job.State != "verification_pending" {
		t.Fatal(job.State)
	}
	seoTestIndex(t, a)
	w := seoGet(a.router(), "/?hall=cable")
	if !strings.Contains(w.Body.String(), "<h1>") || !strings.Contains(w.Body.String(), "noindex,follow") {
		t.Fatal("preparing hall not readable or indexable by mistake")
	}
}

func TestSearchAtomicQuotaAndConcurrentReplay(t *testing.T) {
	a, o, s := searchFixture(t, "indexnow", false)
	v := o.Engines["indexnow"]
	v.DailyLimit = 1
	o.Engines["indexnow"] = v
	searchStore(t, a, o, s)
	for i := 0; i < 2; i++ {
		v := v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, fmt.Sprint("企业", i))
		mustSearch(t, a.enqueueSearch("indexnow", v.ID, contentSearchURL(searchBase(a), v), "upsert", a.searchFingerprint(v, true, contentSearchURL(searchBase(a), v)), true))
	}
	var calls atomic.Int32
	mockSearch(t, a, 200, "", &calls)
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := a.processSearchJob(context.Background())
			if e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("quota or lease race", calls.Load())
	}
	var quota SearchQuota
	mustSearch(t, a.DB.First(&quota, "engine='indexnow'").Error)
	if quota.Used != 1 {
		t.Fatal("quota reservation", quota.Used)
	}
	var pending SearchJob
	mustSearch(t, a.DB.Where("state='pending'").First(&pending).Error)
	if !pending.NotBefore.After(time.Now()) {
		t.Fatal("quota wait has no next-day backoff")
	}
}

func TestSearchTimeoutAndCrashDoNotReplay(t *testing.T) {
	a, _, _ := searchFixture(t, "baidu", true)
	v := v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, "企业")
	mustSearch(t, a.syncSearchContent(v.ID))
	var calls atomic.Int32
	a.searchHTTP = &http.Client{Transport: searchRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, errors.New("timeout do-not-log-token")
	})}
	_, e := a.processSearchJob(context.Background())
	mustSearch(t, e)
	_, e = a.processSearchJob(context.Background())
	mustSearch(t, e)
	if calls.Load() != 1 {
		t.Fatal("uncertain request replayed")
	}
	var job SearchJob
	mustSearch(t, a.DB.First(&job).Error)
	if job.State != "unknown" || strings.Contains(job.Result, "do-not-log") {
		t.Fatal(job)
	}
	tc, token := searchAdmin(t, a)
	tc.call("POST", fmt.Sprintf("/admin/seo/jobs/%d/retry", job.ID), token, gin.H{}, 400)
	expired := time.Now().Add(-time.Minute)
	a.DB.Model(&job).Updates(map[string]any{"state": "processing", "locked_until": expired, "lease": "abandoned"})
	_, e = a.processSearchJob(context.Background())
	mustSearch(t, e)
	a.DB.First(&job)
	if job.State != "unknown" || calls.Load() != 1 {
		t.Fatal("restart replayed a request")
	}
	logs := tc.call("GET", "/admin/seo/jobs", token, nil, 200)
	raw, _ := json.Marshal(logs)
	if strings.Contains(string(raw), "fixture-baidu-private") || strings.Contains(string(raw), "do-not-log") {
		t.Fatal("provider credential leaked into logs")
	}
}

func TestSearchGoogleUsesOnlySitemapAPI(t *testing.T) {
	a, _, _ := searchFixture(t, "google", true)
	for i := 0; i < 2; i++ {
		v := v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, fmt.Sprint("企业", i))
		mustSearch(t, a.syncSearchContent(v.ID))
	}
	var jobs int64
	a.DB.Model(&SearchJob{}).Count(&jobs)
	if jobs != 1 {
		t.Fatal("Google sitemap notifications not coalesced", jobs)
	}
	var calls atomic.Int32
	a.searchHTTP = &http.Client{Transport: searchRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		code, body := 204, ""
		if r.URL.Host == "oauth2.googleapis.com" {
			code = 200
			body = `{"access_token":"private-access-token"}`
		} else if r.URL.Host != "www.googleapis.com" || !strings.Contains(r.URL.Path, "/sitemaps/") || r.Method != "PUT" || r.Header.Get("Authorization") != "Bearer private-access-token" {
			t.Error("not a Search Console sitemap request", r.URL.Host, r.URL.Path)
		}
		return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}, Request: r}, nil
	})}
	_, e := a.processSearchJob(context.Background())
	mustSearch(t, e)
	if calls.Load() != 2 {
		t.Fatal("OAuth and sitemap calls missing")
	}
	var job SearchJob
	a.DB.First(&job)
	if job.State != "accepted" || job.Action != "sitemap" {
		t.Fatal(job)
	}
}

func TestSearchAutomaticSwitchAndTransactionRollback(t *testing.T) {
	a, o, s := searchFixture(t, "indexnow", true)
	v := v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, "企业")
	mustSearch(t, a.syncSearchContent(v.ID))
	opt := o.Engines["indexnow"]
	opt.Automatic = false
	o.Engines["indexnow"] = opt
	searchStore(t, a, o, s)
	var calls atomic.Int32
	mockSearch(t, a, 200, "", &calls)
	ok, e := a.processSearchJob(context.Background())
	mustSearch(t, e)
	if ok || calls.Load() != 0 {
		t.Fatal("automatic switch did not pause queued automatic jobs")
	}
	e = a.DB.Transaction(func(tx *gorm.DB) error {
		if e := recordReview(tx, ReviewLog{ContentID: v.ID, RevisionID: v.PublishedRevisionID, ActorID: 1, Action: "approve"}); e != nil {
			return e
		}
		return errors.New("rollback")
	})
	if e == nil {
		t.Fatal("test rollback missing")
	}
	var n int64
	a.DB.Model(&SearchDirty{}).Where("content_id=?", v.ID).Count(&n)
	if n != 0 {
		t.Fatal("outbox survived rolled back publication")
	}
}

func TestSearchBaiduQuotaFailureAndDeadLinks(t *testing.T) {
	a, _, _ := searchFixture(t, "baidu", true)
	v := v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, "公开企业")
	p := v2Published(t, a, "product", v.ID, 1, 1, 1, false, 0, "产品")
	private := v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, "非公开企业")
	a.DB.Model(&private).Update("visible", false)
	for _, id := range []uint64{v.ID, p.ID, private.ID} {
		mustSearch(t, a.syncSearchContent(id))
	}
	var calls atomic.Int32
	mockSearch(t, a, 200, `{"success":1,"remain":0}`, &calls)
	_, e := a.processSearchJob(context.Background())
	mustSearch(t, e)
	_, e = a.processSearchJob(context.Background())
	mustSearch(t, e)
	if calls.Load() != 1 {
		t.Fatal("provider remaining quota ignored")
	}
	a.DB.Model(&v).Update("visible", false)
	mustSearch(t, markSearchDirty(a.DB, "vendor", v.ID))
	mustSearch(t, a.reconcileSearch())
	var manual int64
	a.DB.Model(&SearchJob{}).Where("state='manual_required' AND action='remove'").Count(&manual)
	if manual != 2 {
		t.Fatal("Baidu removal falsely treated as normal push")
	}
	feed := seoGet(a.router(), "/sitemaps/dead-links.txt")
	if feed.Code != 200 || !strings.Contains(feed.Body.String(), contentSearchURL(searchBase(a), v)) || !strings.Contains(feed.Body.String(), contentSearchURL(searchBase(a), p)) || strings.Contains(feed.Body.String(), contentSearchURL(searchBase(a), private)) {
		t.Fatal("dead links exposed private or missed public history", feed.Body.String())
	}
}

func TestSearchBackoffAndPermanentFailure(t *testing.T) {
	a, _, _ := searchFixture(t, "indexnow", true)
	v := v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, "企业")
	mustSearch(t, a.syncSearchContent(v.ID))
	var calls atomic.Int32
	mockSearch(t, a, 503, "temporary", &calls)
	_, e := a.processSearchJob(context.Background())
	mustSearch(t, e)
	var job SearchJob
	a.DB.First(&job)
	if job.State != "pending" || !job.NotBefore.After(time.Now()) {
		t.Fatal("no bounded backoff")
	}
	_, e = a.processSearchJob(context.Background())
	mustSearch(t, e)
	if calls.Load() != 1 {
		t.Fatal("immediate retry storm")
	}
	a.DB.Model(&job).Update("not_before", time.Now().Add(-time.Second))
	mockSearch(t, a, 403, "secret-must-not-be-logged", &calls)
	_, e = a.processSearchJob(context.Background())
	mustSearch(t, e)
	a.DB.First(&job)
	if job.State != "blocked" || strings.Contains(job.Result, "secret") {
		t.Fatal("authorization failure replayed or exposed response")
	}
}
