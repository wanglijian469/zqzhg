package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SearchDirty struct {
	ContentID uint64 `gorm:"primaryKey"`
	UpdatedAt time.Time
}

func (SearchDirty) TableName() string { return "search_dirty" }

type SearchPage struct {
	WasPublic   bool      `json:"-"`
	ContentID   uint64    `gorm:"primaryKey" json:"content_id"`
	Kind        string    `json:"kind"`
	URL         string    `json:"url"`
	Fingerprint string    `json:"-"`
	IsPublic    bool      `json:"is_public"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type SearchJob struct {
	Source      string     `json:"source"`
	ID          uint64     `json:"id"`
	Engine      string     `json:"engine"`
	ContentID   uint64     `json:"content_id"`
	URL         string     `json:"url"`
	Action      string     `json:"action"`
	Fingerprint string     `json:"-"`
	DedupKey    string     `json:"-"`
	State       string     `json:"state"`
	Attempts    int        `json:"attempts"`
	NotBefore   time.Time  `json:"not_before"`
	Lease       string     `json:"-"`
	LockedUntil *time.Time `json:"-"`
	HTTPStatus  int        `json:"http_status"`
	Result      string     `json:"result"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
type SearchAttempt struct {
	ID         uint64    `json:"id"`
	JobID      uint64    `json:"job_id"`
	Engine     string    `json:"engine"`
	URL        string    `json:"url"`
	Action     string    `json:"action"`
	State      string    `json:"state"`
	HTTPStatus int       `json:"http_status"`
	Result     string    `json:"result"`
	CreatedAt  time.Time `json:"created_at"`
}
type SearchQuota struct {
	Engine          string `gorm:"primaryKey" json:"engine"`
	Day             string `gorm:"primaryKey" json:"day"`
	Used            int    `json:"used"`
	RemoteRemaining int    `json:"remote_remaining"`
}

func (SearchQuota) TableName() string { return "search_quotas" }

func sortSearchContents(items []Content) {
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
}
func (a *App) searchFingerprint(c Content, public bool, target string) string {
	parent := uint64(0)
	if c.Kind == "product" {
		var v Content
		a.DB.First(&v, c.VendorID)
		parent = v.Epoch
	}
	return digest(fmt.Sprintf("%s|%d|%d|%d|%t", target, c.PublishedRevisionID, c.Epoch, parent, public))
}
func markSearchDirty(tx *gorm.DB, kind string, target uint64) error {
	if (kind != "vendor" && kind != "product" && kind != "hall") || !tx.Migrator().HasTable(&SearchDirty{}) {
		return nil
	}
	query := "id=? OR vendor_id=?"
	if kind == "hall" {
		query = "hall_id=? OR vendor_id IN (SELECT h.id FROM (SELECT id FROM contents WHERE hall_id=?) h)"
	}
	return tx.Exec("INSERT INTO search_dirty(content_id,updated_at) SELECT id,UTC_TIMESTAMP(3) FROM contents WHERE "+query+" ON DUPLICATE KEY UPDATE updated_at=VALUES(updated_at)", target, target).Error
}
func (a *App) searchContent(id uint64) (Content, bool, error) {
	var content Content
	e := a.DB.First(&content, id).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return content, false, nil
	}
	if e != nil {
		return content, false, e
	}
	if content.Kind != "vendor" && content.Kind != "product" {
		return content, false, nil
	}
	var count int64
	e = a.publicQuery(content.Kind).Where("contents.id=?", id).Count(&count).Error
	return content, count == 1, e
}
func contentSearchURL(base string, c Content) string {
	if c.Kind == "vendor" {
		return base + "/v/" + showroomSlug(c)
	}
	return fmt.Sprintf("%s/products/%d", base, c.ID)
}
func (a *App) enqueueSearch(engine string, id uint64, target, action, fingerprint string, manual ...bool) error {
	if engine == "google" {
		target = searchBase(a) + "/sitemap.xml"
		action = "sitemap"
		fingerprint = time.Now().UTC().Format("2006-01-02")
	}
	source := "automatic"
	if len(manual) > 0 && manual[0] {
		source = "manual"
	}
	now := time.Now().UTC()
	job := SearchJob{Source: source, Engine: engine, ContentID: id, URL: target, Action: action, Fingerprint: fingerprint, DedupKey: digest(engine + "\n" + target + "\n" + action + "\n" + fingerprint), State: "pending", NotBefore: now, CreatedAt: now, UpdatedAt: now}
	if engine == "baidu" && action == "remove" {
		job.State = "manual_required"
		job.Result = "baidu_dead_link_console_required"
	}
	conflict := clause.OnConflict{DoNothing: true}
	if source == "manual" {
		conflict = clause.OnConflict{Columns: []clause.Column{{Name: "dedup_key"}}, DoUpdates: clause.Assignments(map[string]any{"source": "manual"})}
	}
	return a.DB.Clauses(conflict).Create(&job).Error
}
func (a *App) syncSearchContent(id uint64) error {
	return a.DB.Transaction(func(tx *gorm.DB) error {
		ta := &App{DB: tx, Config: a.Config}
		base := searchBase(ta)
		if !validSearchBase(base) {
			return nil
		}
		_, options, _, e := ta.searchSettings()
		if e != nil {
			return e
		}
		var dirty SearchDirty
		de := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&dirty, "content_id=?", id).Error
		if de != nil && !errors.Is(de, gorm.ErrRecordNotFound) {
			return de
		}
		content, public, e := ta.searchContent(id)
		if e != nil {
			return e
		}
		var old SearchPage
		e = tx.First(&old, "content_id=?", id).Error
		if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		if content.ID == 0 || content.Kind != "vendor" && content.Kind != "product" {
			if old.ContentID == 0 {
				return tx.Where("content_id=?", id).Delete(&SearchDirty{}).Error
			}
			content.Kind = old.Kind
			content.ID = id
		}
		target := contentSearchURL(base, content)
		if !public && old.WasPublic && old.URL != "" {
			target = old.URL
		}
		fingerprint := ta.searchFingerprint(content, public, target)
		if public && (old.Fingerprint != fingerprint || !old.IsPublic) {
			for _, engine := range searchEngines {
				o := options.Engines[engine]
				if o.Enabled && o.Automatic {
					if e = ta.enqueueSearch(engine, id, target, "upsert", fingerprint); e != nil {
						return e
					}
				}
			}
		}
		if old.IsPublic && (!public || old.URL != target) {
			for _, engine := range searchEngines {
				o := options.Engines[engine]
				if o.Enabled && o.Automatic {
					if e = ta.enqueueSearch(engine, id, old.URL, "remove", fingerprint); e != nil {
						return e
					}
				}
			}
		}
		state := SearchPage{ContentID: id, Kind: content.Kind, URL: target, Fingerprint: fingerprint, IsPublic: public, WasPublic: public || old.WasPublic, UpdatedAt: time.Now().UTC()}
		if e = tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&state).Error; e != nil {
			return e
		}
		return tx.Where("content_id=?", id).Delete(&SearchDirty{}).Error
	})
}
func (a *App) reconcileSearch() error {
	var dirty []SearchDirty
	if e := a.DB.Order("updated_at,content_id").Limit(100).Find(&dirty).Error; e != nil {
		return e
	}
	for _, d := range dirty {
		if e := a.syncSearchContent(d.ContentID); e != nil {
			return e
		}
	}
	// A bounded durable cursor also recovers legacy/imported changes and hard deletions.
	var cursor SystemSetting
	a.DB.First(&cursor, "setting_key=?", "seo_scan_cursor")
	after, _ := strconv.ParseUint(cursor.Value, 10, 64)
	var contents []Content
	if e := a.DB.Where("id>? AND kind IN ?", after, []string{"vendor", "product"}).Order("id").Limit(100).Find(&contents).Error; e != nil {
		return e
	}
	for _, c := range contents {
		if e := a.syncSearchContent(c.ID); e != nil {
			return e
		}
		after = c.ID
	}
	if len(contents) < 100 {
		after = 0
	}
	if e := a.setSetting("seo_scan_cursor", strconv.FormatUint(after, 10)); e != nil {
		return e
	}
	var missing []SearchPage
	if e := a.DB.Where("is_public=1 AND NOT EXISTS (SELECT 1 FROM contents c WHERE c.id=search_pages.content_id)").Limit(100).Find(&missing).Error; e != nil {
		return e
	}
	for _, p := range missing {
		if e := a.syncSearchContent(p.ContentID); e != nil {
			return e
		}
	}
	return nil
}
func (a *App) validSearchURL(raw string) (uint64, string, error) {
	base := searchBase(a)
	u, e := url.Parse(raw)
	if e != nil || !validSearchBase(base) || u.User != nil || u.Fragment != "" {
		return 0, "", fmt.Errorf("seo_url_not_public")
	}
	b, _ := url.Parse(base)
	if u.Scheme != b.Scheme || u.Host != b.Host {
		return 0, "", fmt.Errorf("seo_url_not_public")
	}
	if strings.HasPrefix(u.Path, "/products/") || strings.HasPrefix(u.Path, "/v/") {
		var c Content
		if strings.HasPrefix(u.Path, "/products/") {
			n, _ := strconv.ParseUint(strings.TrimPrefix(u.Path, "/products/"), 10, 64)
			e = a.DB.First(&c, n).Error
		} else {
			slug := strings.TrimPrefix(u.Path, "/v/")
			if strings.HasPrefix(slug, "v-") {
				n, _ := strconv.ParseUint(strings.TrimPrefix(slug, "v-"), 10, 64)
				e = a.DB.First(&c, n).Error
			} else {
				e = a.DB.Where("showroom_slug=? AND kind='vendor'", slug).First(&c).Error
			}
		}
		if e != nil {
			if !errors.Is(e, gorm.ErrRecordNotFound) {
				return 0, "", e
			}
			return 0, "", fmt.Errorf("seo_url_not_public")
		}
		_, public, e := a.searchContent(c.ID)
		if e != nil {
			return 0, "", e
		}
		canonical := contentSearchURL(base, c)
		if !public || raw != canonical {
			return 0, "", fmt.Errorf("seo_url_not_public")
		}
		return c.ID, a.searchFingerprint(c, true, canonical), nil
	}
	allowed := u.Path == "/" || u.Path == "/browse" || u.Path == "/browse/products" || u.Path == "/regions"
	if !allowed {
		return 0, "", fmt.Errorf("seo_url_not_public")
	}
	var hall Hall
	e = a.DB.Where("slug=? AND status='open'", u.Query().Get("hall")).First(&hall).Error
	if u.Query().Get("hall") == "" {
		e = a.DB.Where("slug='agriculture' AND status='open'").First(&hall).Error
	}
	if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
		return 0, "", e
	}
	if e != nil || raw != canonicalPublic(base, u.Path, u.Query()) {
		return 0, "", fmt.Errorf("seo_url_not_public")
	}
	for k := range u.Query() {
		if k != "hall" {
			return 0, "", fmt.Errorf("seo_url_not_public")
		}
	}
	return 0, digest(raw), nil
}
func (a *App) enqueueSearchAdmin(c *gin.Context) {
	var req struct {
		Engine string   `json:"engine"`
		URLs   []string `json:"urls"`
		Scope  string   `json:"scope"`
		Cursor uint64   `json:"cursor"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.URLs) > 500 || (req.Scope != "" && req.Scope != "urls" && req.Scope != "public") || (req.Scope == "public" && len(req.URLs) != 0) {
		fail(c, 400, "invalid_request")
		return
	}
	_, options, _, e := a.searchSettings()
	if e != nil {
		dbFail(c, e)
		return
	}
	o, ok := options.Engines[req.Engine]
	if !ok || !o.Enabled {
		fail(c, 400, "seo_engine_disabled")
		return
	}
	next := uint64(0)
	more := false
	if req.Scope == "public" {
		var items []Content
		e = a.publicQuery("vendor").Where("contents.id>?", req.Cursor).Select("contents.*").Order("contents.id").Limit(201).Find(&items).Error
		var products []Content
		if e == nil {
			e = a.publicQuery("product").Where("contents.id>?", req.Cursor).Select("contents.*").Order("contents.id").Limit(201).Find(&products).Error
		}
		if e != nil {
			dbFail(c, e)
			return
		}
		items = append(items, products...)
		sortSearchContents(items)
		if len(items) > 200 {
			more = true
			items = items[:200]
		}
		for _, item := range items {
			req.URLs = append(req.URLs, contentSearchURL(searchBase(a), item))
			next = item.ID
		}
	}
	count := 0
	e = a.DB.Transaction(func(tx *gorm.DB) error {
		ta := &App{DB: tx, Config: a.Config}
		for _, raw := range req.URLs {
			id, fp, err := ta.validSearchURL(raw)
			if err != nil {
				return err
			}
			if err = ta.enqueueSearch(req.Engine, id, raw, "upsert", fp, true); err != nil {
				return err
			}
			count++
		}
		return recordOperation(tx, actor(c).ID, "seo", 0, "seo_manual_queued", fmt.Sprintf("engine=%s validated_urls=%d", req.Engine, count))
	})
	if e != nil {
		if e.Error() == "seo_url_not_public" {
			fail(c, 400, e.Error())
		} else {
			dbFail(c, e)
		}
		return
	}
	c.JSON(200, gin.H{"validated_urls": count, "next_cursor": next, "has_more": more, "message": "queued_not_submitted"})
}
func (a *App) searchOverview(c *gin.Context) {
	stats := []struct {
		Engine string `json:"engine"`
		State  string `json:"state"`
		Count  int64  `json:"count"`
	}{}
	if e := a.DB.Model(&SearchJob{}).Select("engine,state,COUNT(*) AS count").Group("engine,state").Scan(&stats).Error; e != nil {
		dbFail(c, e)
		return
	}
	day := searchDay(time.Now())
	quotas := []SearchQuota{}
	if e := a.DB.Where("day=?", day).Find(&quotas).Error; e != nil {
		dbFail(c, e)
		return
	}
	var counts = map[string]int64{}
	for _, kind := range []string{"vendor", "product"} {
		var n int64
		if e := a.publicQuery(kind).Count(&n).Error; e != nil {
			dbFail(c, e)
			return
		}
		counts[kind] = n
	}
	c.JSON(200, gin.H{"stats": stats, "quotas": quotas, "day": day, "public_counts": counts, "sitemap_url": searchBase(a) + "/sitemap.xml", "sitemap_mode": "live_query", "message": "accepted_is_not_indexed"})
}
func (a *App) searchLogs(c *gin.Context) {
	p, n := page(c)
	q := a.DB.Model(&SearchJob{})
	for _, key := range []string{"engine", "state"} {
		if v := c.Query(key); v != "" {
			q = q.Where(key+"=?", v)
		}
	}
	var total int64
	var items []SearchJob
	if e := q.Count(&total).Error; e != nil {
		dbFail(c, e)
		return
	}
	if e := q.Order("id DESC").Offset((p - 1) * n).Limit(n).Find(&items).Error; e != nil {
		dbFail(c, e)
		return
	}
	if items == nil {
		items = []SearchJob{}
	}
	c.JSON(200, gin.H{"items": items, "total": total, "page": p, "page_size": n})
}
func (a *App) searchAttempts(c *gin.Context) {
	rows := []SearchAttempt{}
	if e := a.DB.Where("job_id=?", id(c)).Order("id DESC").Limit(50).Find(&rows).Error; e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"items": rows})
}
func (a *App) refreshSearchSitemap(c *gin.Context) {
	if e := a.reconcileSearch(); e != nil {
		dbFail(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true, "sitemap_url": searchBase(a) + "/sitemap.xml", "mode": "live_query", "message": "sitemap_already_tracks_public_data"})
}
func (a *App) retrySearchJob(c *gin.Context) {
	var req struct {
		ConfirmUnknown bool `json:"confirm_unknown"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, 400, "invalid_request")
		return
	}
	e := a.DB.Transaction(func(tx *gorm.DB) error {
		var job SearchJob
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&job, id(c)).Error; e != nil {
			return e
		}
		if job.State == "unknown" && !req.ConfirmUnknown {
			return fmt.Errorf("seo_uncertain_confirmation_required")
		}
		if job.State != "unknown" && job.State != "failed" && job.State != "blocked" {
			return fmt.Errorf("invalid_request")
		}
		return tx.Model(&job).Updates(map[string]any{"state": "pending", "attempts": 0, "not_before": time.Now().UTC(), "result": "manual_retry"}).Error
	})
	if e != nil {
		fail(c, 400, e.Error())
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func (a *App) startSearchWorker(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = a.reconcileSearch()
				for i := 0; i < 5; i++ {
					if ok, e := a.processSearchJob(ctx); e != nil || !ok {
						break
					}
				}
			}
		}
	}()
}
