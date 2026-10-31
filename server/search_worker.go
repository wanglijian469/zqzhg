package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func searchDay(now time.Time) string {
	return now.In(time.FixedZone("Asia/Shanghai", 8*3600)).Format("2006-01-02")
}
func nextSearchDay(now time.Time) time.Time {
	local := now.In(time.FixedZone("Asia/Shanghai", 8*3600))
	return time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, local.Location()).UTC()
}

type searchReply struct {
	State, Result     string
	Status, Remaining int
	RetryAfter        time.Duration
}

func (a *App) searchRequest(ctx context.Context, method, target, contentType string, payload []byte, token string) (int, []byte, http.Header, error) {
	client := http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if a.searchHTTP != nil {
		client.Transport = a.searchHTTP.Transport
	}
	req, e := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(payload))
	if e != nil {
		return 0, nil, nil, e
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("User-Agent", "ZQZH-SearchSubmission/1.0")
	response, e := client.Do(req)
	if e != nil {
		return 0, nil, nil, errors.New("provider_result_unknown")
	}
	defer response.Body.Close()
	body, e := io.ReadAll(io.LimitReader(response.Body, 16385))
	if e != nil || len(body) > 16384 {
		return response.StatusCode, nil, response.Header, errors.New("provider_result_unknown")
	}
	return response.StatusCode, body, response.Header, nil
}
func (a *App) submitSearch(ctx context.Context, job SearchJob, o SearchOptions, s SearchSecrets) searchReply {
	result := searchReply{State: "failed", Result: "provider_rejected", Remaining: -1}
	base := searchBase(a)
	endpoint, typ, method, token := "", "application/json", "POST", ""
	var payload []byte
	switch job.Engine {
	case "baidu":
		endpoint = "https://data.zz.baidu.com/urls?" + url.Values{"site": {base}, "token": {s.BaiduToken}}.Encode()
		typ = "text/plain; charset=utf-8"
		payload = []byte(job.URL + "\n")
	case "indexnow":
		u, _ := url.Parse(base)
		endpoint = "https://api.indexnow.org/indexnow"
		// A root-level proof covers /v/ and /products/; a nested proof would only cover its own directory.
		payload, _ = json.Marshal(gin.H{"host": u.Host, "key": s.IndexNowKey, "keyLocation": base + "/indexnow.txt", "urlList": []string{job.URL}})
	case "google":
		form := url.Values{"grant_type": {"refresh_token"}, "client_id": {s.GoogleClientID}, "client_secret": {s.GoogleClientSecret}, "refresh_token": {s.GoogleRefreshToken}}
		status, body, _, e := a.searchRequest(ctx, "POST", "https://oauth2.googleapis.com/token", "application/x-www-form-urlencoded", []byte(form.Encode()), "")
		if e != nil {
			return searchReply{State: "unknown", Result: "oauth_result_unknown", Remaining: -1}
		}
		if status != 200 {
			return searchReply{State: "blocked", Result: "google_authorization_required", Status: status, Remaining: -1}
		}
		var auth struct {
			AccessToken string `json:"access_token"`
		}
		if json.Unmarshal(body, &auth) != nil || auth.AccessToken == "" {
			return searchReply{State: "blocked", Result: "google_authorization_required", Status: status, Remaining: -1}
		}
		token = auth.AccessToken
		property := o.GoogleProperty
		if property == "" {
			property = base + "/"
		}
		endpoint = "https://www.googleapis.com/webmasters/v3/sites/" + url.PathEscape(property) + "/sitemaps/" + url.PathEscape(base+"/sitemap.xml")
		method = "PUT"
	default:
		return searchReply{State: "blocked", Result: "unsupported_engine", Remaining: -1}
	}
	status, body, headers, e := a.searchRequest(ctx, method, endpoint, typ, payload, token)
	result.Status = status
	if e != nil {
		result.State = "unknown"
		result.Result = "provider_result_unknown"
		return result
	}
	if status == 429 || status >= 500 {
		result.Result = "provider_temporary_failure"
		result.RetryAfter = 5 * time.Minute
		if n, e := strconv.Atoi(headers.Get("Retry-After")); e == nil && n > 0 && n <= 86400 {
			result.RetryAfter = time.Duration(n) * time.Second
		}
		return result
	}
	if status == 401 || status == 403 {
		result.State = "blocked"
		result.Result = "authorization_or_ownership_required"
		return result
	}
	switch job.Engine {
	case "indexnow":
		if status == 200 {
			result.State = "accepted"
			result.Result = "received_not_indexed"
		} else if status == 202 {
			result.State = "verification_pending"
			result.Result = "received_key_verification_pending"
		}
	case "google":
		if status == 204 || status == 200 {
			result.State = "accepted"
			result.Result = "sitemap_received_not_indexed"
		}
	case "baidu":
		var response struct {
			Success int      `json:"success"`
			Remain  *int     `json:"remain"`
			Error   int      `json:"error"`
			Invalid []string `json:"not_valid"`
			Other   []string `json:"not_same_site"`
		}
		if status == 200 && json.Unmarshal(body, &response) == nil {
			if response.Remain != nil {
				result.Remaining = *response.Remain
			}
			if response.Success == 1 && len(response.Invalid) == 0 && len(response.Other) == 0 && response.Error == 0 {
				result.State = "accepted"
				result.Result = "received_not_indexed"
			} else if response.Error == 401 || response.Error == 400 {
				result.State = "blocked"
				result.Result = "baidu_configuration_required"
			} else if response.Remain != nil && *response.Remain == 0 {
				result.State = "blocked"
				result.Result = "provider_quota_exhausted"
			}
		}
	}
	return result
}
func (a *App) finishSearch(job SearchJob, state, result string, status int, next time.Time) error {
	return a.DB.Model(&SearchJob{}).Where("id=? AND lease=? AND state='processing'", job.ID, job.Lease).Updates(map[string]any{"state": state, "result": result, "http_status": status, "not_before": next, "locked_until": nil, "lease": "", "updated_at": time.Now().UTC()}).Error
}
func (a *App) processSearchJob(ctx context.Context) (bool, error) {
	now := time.Now().UTC()
	_, options, secrets, e := a.searchSettings()
	if e != nil {
		return false, e
	}
	// Expired in-flight sends are uncertain, never automatically replayed after a crash.
	if e = a.DB.Transaction(func(tx *gorm.DB) error {
		var expired []SearchJob
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("state='processing' AND locked_until<?", now).Limit(100).Find(&expired).Error; e != nil {
			return e
		}
		for _, j := range expired {
			if e := tx.Model(&SearchAttempt{}).Where("job_id=? AND state='started'", j.ID).Updates(map[string]any{"state": "unknown", "result": "worker_interrupted"}).Error; e != nil {
				return e
			}
			if e := tx.Model(&j).Updates(map[string]any{"state": "unknown", "result": "worker_interrupted", "lease": "", "locked_until": nil}).Error; e != nil {
				return e
			}
		}
		return nil
	}); e != nil {
		return false, e
	}
	enabled := []string{}
	for _, engine := range searchEngines {
		if options.Engines[engine].Enabled {
			enabled = append(enabled, engine)
		}
	}
	if len(enabled) == 0 {
		return false, nil
	}
	automatic := []string{}
	for _, name := range enabled {
		if options.Engines[name].Automatic {
			automatic = append(automatic, name)
		}
	}
	var job SearchJob
	e = a.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("engine IN ? AND state='pending' AND not_before<=?", enabled, now).Where("source='manual' OR engine IN ?", automatic).Order("id").First(&job).Error; e != nil {
			return e
		}
		until := now.Add(2 * time.Minute)
		job.Lease = randomToken()
		job.LockedUntil = &until
		job.State = "processing"
		return tx.Model(&job).Updates(map[string]any{"state": "processing", "lease": job.Lease, "locked_until": until, "updated_at": now}).Error
	})
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	finish := func(state, result string, status int, next time.Time) (bool, error) {
		return true, a.finishSearch(job, state, result, status, next)
	}
	if !searchConfigured(job.Engine, secrets) {
		return finish("blocked", "credentials_required", 0, now)
	}
	base := searchBase(a)
	if !validSearchBase(base) {
		return finish("blocked", "public_https_required", 0, now)
	}
	if job.Engine == "google" {
		if job.URL != base+"/sitemap.xml" {
			return finish("skipped", "canonical_site_changed", 0, now)
		}
	} else if job.Action == "remove" {
		b, _ := url.Parse(base)
		u, _ := url.Parse(job.URL)
		if u == nil || u.Host != b.Host || u.Scheme != b.Scheme {
			return finish("skipped", "canonical_site_changed", 0, now)
		}
		c, public, e := a.searchContent(job.ContentID)
		if e != nil {
			return finish("blocked", "database_unavailable", 0, now)
		}
		if public && contentSearchURL(base, c) == job.URL {
			return finish("skipped", "page_is_public_again", 0, now)
		}
	} else {
		_, fp, e := a.validSearchURL(job.URL)
		if e != nil {
			if e.Error() != "seo_url_not_public" {
				return finish("blocked", "database_unavailable", 0, now)
			}
			return finish("skipped", "not_current_public_url", 0, now)
		}
		if job.ContentID != 0 && fp != job.Fingerprint {
			return finish("skipped", "newer_public_revision", 0, now)
		}
	}
	day := searchDay(now)
	attempt := SearchAttempt{JobID: job.ID, Engine: job.Engine, URL: job.URL, Action: job.Action, State: "started", CreatedAt: now}
	e = a.DB.Transaction(func(tx *gorm.DB) error {
		q := SearchQuota{Engine: job.Engine, Day: day, RemoteRemaining: -1}
		if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&q).Error; e != nil {
			return e
		}
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&q, "engine=? AND day=?", job.Engine, day).Error; e != nil {
			return e
		}
		if q.Used >= options.Engines[job.Engine].DailyLimit || q.RemoteRemaining == 0 {
			return fmt.Errorf("quota_exhausted")
		}
		changes := map[string]any{"used": q.Used + 1}
		if q.RemoteRemaining > 0 {
			changes["remote_remaining"] = q.RemoteRemaining - 1
		}
		if e := tx.Model(&q).Updates(changes).Error; e != nil {
			return e
		}
		if e := tx.Create(&attempt).Error; e != nil {
			return e
		}
		return tx.Model(&SearchJob{}).Where("id=? AND lease=?", job.ID, job.Lease).Update("attempts", gorm.Expr("attempts+1")).Error
	})
	if e != nil {
		if e.Error() == "quota_exhausted" {
			return finish("pending", "daily_limit_waiting", 0, nextSearchDay(now))
		}
		return finish("blocked", "database_unavailable", 0, now)
	}
	result := a.submitSearch(ctx, job, options, secrets)
	next := now
	state := result.State
	if result.State == "failed" && result.RetryAfter > 0 && job.Attempts < 4 {
		state = "pending"
		next = now.Add(result.RetryAfter * time.Duration(job.Attempts+1))
	}
	return true, a.DB.Transaction(func(tx *gorm.DB) error {
		if result.Remaining >= 0 {
			if e := tx.Model(&SearchQuota{}).Where("engine=? AND day=?", job.Engine, day).Update("remote_remaining", gorm.Expr("CASE WHEN remote_remaining<0 THEN ? ELSE LEAST(remote_remaining,?) END", result.Remaining, result.Remaining)).Error; e != nil {
				return e
			}
		}
		if e := tx.Model(&attempt).Updates(map[string]any{"state": result.State, "result": result.Result, "http_status": result.Status}).Error; e != nil {
			return e
		}
		ta := &App{DB: tx, Config: a.Config}
		return ta.finishSearch(job, state, result.Result, result.Status, next)
	})
}
func (a *App) runSearchQueue(c *gin.Context) {
	ok, e := a.processSearchJob(c.Request.Context())
	if e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"processed": ok, "message": "check_job_result_not_index_status"})
}

// No response URL, provider error text, OAuth response, or secret is stored in logs.
