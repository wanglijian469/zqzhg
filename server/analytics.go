package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AnalyticsEvent struct {
	EventID     string    `gorm:"primaryKey" json:"event_id"`
	VisitorHash string    `json:"-"`
	SessionHash string    `json:"-"`
	Source      string    `json:"source"`
	EventName   string    `json:"event_name"`
	Page        string    `json:"page"`
	TargetKind  string    `json:"target_kind"`
	TargetID    uint64    `json:"target_id"`
	Keyword     string    `json:"keyword"`
	Referrer    string    `json:"referrer"`
	Device      string    `json:"device"`
	OccurredAt  time.Time `json:"occurred_at"`
}
type AnalyticsDaily struct {
	Day      string `gorm:"primaryKey;type:date" json:"day"`
	Source   string `gorm:"primaryKey" json:"source"`
	PV       int64  `json:"pv"`
	UV       int64  `json:"uv"`
	Visits   int64  `json:"visits"`
	Contacts int64  `json:"contacts"`
	Searches int64  `json:"searches"`
}

func (AnalyticsDaily) TableName() string { return "analytics_daily" }

type AnalyticsInput struct {
	EventID    string    `json:"event_id"`
	VisitorID  string    `json:"visitor_id"`
	SessionID  string    `json:"session_id"`
	Source     string    `json:"source"`
	EventName  string    `json:"event_name"`
	Page       string    `json:"page"`
	TargetKind string    `json:"target_kind"`
	TargetID   uint64    `json:"target_id"`
	Keyword    string    `json:"keyword"`
	Referrer   string    `json:"referrer"`
	Device     string    `json:"device"`
	OccurredAt time.Time `json:"occurred_at"`
}

var analyticID = regexp.MustCompile(`^[a-zA-Z0-9_-]{16,80}$`)
var eventIDPattern = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)
var analyticPage = regexp.MustCompile(`^(/|/browse(?:/products)?|/app|/vendors/[0-9]+|/products/[0-9]+|/v/[a-z0-9-]+|/resources/[0-9]+)$`)
var sensitiveSearch = regexp.MustCompile(`(?i)(1[3-9][0-9]{9}|[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}|(?:password|token|secret|access.?key)\s*[:=])`)

func cleanSearch(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if sensitiveSearch.MatchString(value) {
		return "[已脱敏]"
	}
	runes := []rune(value)
	if len(runes) > 80 {
		runes = runes[:80]
	}
	return string(runes)
}
func analyticHash(key []byte, source, value string) string {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(source + ":" + value))
	return hex.EncodeToString(h.Sum(nil))
}
func (a *App) analyticsRoutes(pub, admin *gin.RouterGroup) {
	pub.POST("/analytics/events", a.collectAnalytics)
	admin.GET("/analytics", a.analyticsReport)
	admin.GET("/analytics/export", a.analyticsExport)
}
func (a *App) collectAnalytics(c *gin.Context) {
	if !permit("analytics:"+c.ClientIP(), 120, time.Minute) {
		fail(c, 429, "rate_limited")
		return
	}
	var req struct {
		Events []AnalyticsInput `json:"events"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.Events) == 0 || len(req.Events) > 50 {
		fail(c, 400, "invalid_request")
		return
	}
	key, err := a.analyticsKey()
	if err != nil {
		fail(c, 503, "analytics_unavailable")
		return
	}
	now := time.Now().UTC()
	events := []AnalyticsEvent{}
	allowed := map[string]bool{"page_view": true, "search": true, "vendor_view": true, "product_view": true, "resource_view": true, "contact_phone": true, "contact_wechat": true}
	for _, input := range req.Events {
		if !eventIDPattern.MatchString(input.EventID) || !analyticID.MatchString(input.VisitorID) || !analyticID.MatchString(input.SessionID) || (input.Source != "web" && input.Source != "android") || !allowed[input.EventName] || !analyticPage.MatchString(input.Page) {
			continue
		}
		if input.TargetKind != "" && input.TargetKind != "vendor" && input.TargetKind != "product" && input.TargetKind != "resource" {
			continue
		}
		if input.EventName == "vendor_view" && (input.TargetKind != "vendor" || input.TargetID == 0) {
			continue
		}
		if input.EventName == "resource_view" && (input.TargetKind != "resource" || input.TargetID == 0) {
			continue
		}
		if input.EventName == "product_view" && (input.TargetKind != "product" || input.TargetID == 0) {
			continue
		}
		if (strings.HasPrefix(input.EventName, "contact_") || input.TargetID > 0) && input.TargetKind == "" {
			continue
		}
		if input.TargetKind != "" && input.TargetID > 0 {
			var count int64
			query := a.publicQuery(input.TargetKind).Where("contents.id=?", input.TargetID)
			if input.TargetKind == "resource" {
				query = a.resourceQuery().Where("resources.id=?", input.TargetID)
			}
			if query.Count(&count).Error != nil || count == 0 {
				continue
			}
		}
		occurred := input.OccurredAt.UTC()
		if occurred.IsZero() || occurred.After(now.Add(5*time.Minute)) {
			occurred = now
		}
		if occurred.Before(now.Add(-24 * time.Hour)) {
			continue
		}
		referrer := ""
		if parsed, e := url.Parse(input.Referrer); e == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.User == nil {
			referrer = parsed.Hostname()
			if len(referrer) > 253 {
				referrer = ""
			}
		}
		device := input.Device
		if device != "mobile" && device != "desktop" && device != "tablet" {
			device = "unknown"
		}
		keyword := ""
		if input.EventName == "search" {
			keyword = cleanSearch(input.Keyword)
		}
		events = append(events, AnalyticsEvent{EventID: strings.ToLower(input.EventID), VisitorHash: analyticHash(key, input.Source, input.VisitorID), SessionHash: analyticHash(key, input.Source, input.SessionID), Source: input.Source, EventName: input.EventName, Page: input.Page, TargetKind: input.TargetKind, TargetID: input.TargetID, Keyword: keyword, Referrer: referrer, Device: device, OccurredAt: occurred})
	}
	if len(events) > 0 {
		if err = a.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&events).Error; err != nil {
			dbFail(c, err)
			return
		}
	}
	c.JSON(202, gin.H{"accepted": len(events)})
}

type analyticsWindow struct {
	From, Until time.Time
	Source      string
}

var shanghai = time.FixedZone("Asia/Shanghai", 8*60*60)

func analyticsDates(c *gin.Context) (analyticsWindow, error) {
	now := time.Now().In(shanghai)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, shanghai)
	from := today.AddDate(0, 0, -6)
	to := today
	var err error
	if value := c.Query("from"); value != "" {
		from, err = time.ParseInLocation("2006-01-02", value, shanghai)
		if err != nil {
			return analyticsWindow{}, err
		}
	}
	if value := c.Query("to"); value != "" {
		to, err = time.ParseInLocation("2006-01-02", value, shanghai)
		if err != nil {
			return analyticsWindow{}, err
		}
	}
	source := c.Query("source")
	if source != "" && source != "web" && source != "android" {
		return analyticsWindow{}, fmt.Errorf("invalid_source")
	}
	if to.Before(from) || to.Sub(from) > 730*24*time.Hour || to.After(today) || from.Before(today.AddDate(-2, 0, 0)) {
		return analyticsWindow{}, fmt.Errorf("invalid_range")
	}
	return analyticsWindow{from.UTC(), to.AddDate(0, 0, 1).UTC(), source}, nil
}
func analyticFilter(db *gorm.DB, w analyticsWindow) *gorm.DB {
	q := db.Model(&AnalyticsEvent{}).Where("occurred_at>=? AND occurred_at<?", w.From, w.Until)
	if w.Source != "" {
		q = q.Where("source=?", w.Source)
	}
	return q
}

type AnalyticsSummary struct {
	PV           int64  `json:"pv"`
	UV           *int64 `json:"uv"`
	Visits       *int64 `json:"visits"`
	Contacts     int64  `json:"contacts"`
	Searches     int64  `json:"searches"`
	VendorViews  int64  `json:"vendor_views"`
	ProductViews int64  `json:"product_views"`
}
type analyticRanking struct {
	Name  string `json:"name"`
	ID    uint64 `json:"id"`
	Count int64  `json:"count"`
}

func (a *App) analyticsData(w analyticsWindow) (gin.H, error) {
	detailed := !w.From.Before(time.Now().UTC().Add(-90 * 24 * time.Hour))
	summary := AnalyticsSummary{}
	daily := []AnalyticsDaily{}
	rankings := map[string][]analyticRanking{}
	sources := []analyticRanking{}
	devices := []analyticRanking{}
	referrers := []analyticRanking{}
	if detailed {
		var row struct{ PV, UV, Visits, Contacts, Searches, VendorViews, ProductViews int64 }
		err := analyticFilter(a.DB, w).Select("COALESCE(SUM(event_name='page_view'),0) pv, COUNT(DISTINCT visitor_hash) uv, COUNT(DISTINCT session_hash) visits, COALESCE(SUM(event_name IN ('contact_phone','contact_wechat')),0) contacts, COALESCE(SUM(event_name='search'),0) searches, COALESCE(SUM(event_name='vendor_view'),0) vendor_views, COALESCE(SUM(event_name='product_view'),0) product_views").Scan(&row).Error
		if err != nil {
			return nil, err
		}
		summary = AnalyticsSummary{row.PV, &row.UV, &row.Visits, row.Contacts, row.Searches, row.VendorViews, row.ProductViews}
		err = analyticFilter(a.DB, w).Select("DATE_FORMAT(DATE_ADD(occurred_at,INTERVAL 8 HOUR),'%Y-%m-%d') day, source, COALESCE(SUM(event_name='page_view'),0) pv, COUNT(DISTINCT visitor_hash) uv, COUNT(DISTINCT session_hash) visits, COALESCE(SUM(event_name IN ('contact_phone','contact_wechat')),0) contacts, COALESCE(SUM(event_name='search'),0) searches").Group("day,source").Order("day,source").Scan(&daily).Error
		if err != nil {
			return nil, err
		}
		for _, part := range []struct{ Key, Event, Kind string }{{"vendors", "vendor_view", "vendor"}, {"products", "product_view", "product"}} {
			rows := []analyticRanking{}
			err = analyticFilter(a.DB, w).Where("event_name=?", part.Event).Select("target_id id,COUNT(*) count").Group("target_id").Order("count DESC,id").Limit(20).Scan(&rows).Error
			if err != nil {
				return nil, err
			}
			for i := range rows {
				var item Content
				var rev Revision
				if a.DB.First(&item, rows[i].ID).Error == nil && a.DB.First(&rev, item.PublishedRevisionID).Error == nil {
					rows[i].Name = rev.Name
				}
			}
			rankings[part.Key] = rows
		}
		keywords := []analyticRanking{}
		if err = analyticFilter(a.DB, w).Where("event_name='search' AND keyword<>''").Select("keyword name,COUNT(*) count").Group("keyword").Order("count DESC").Limit(20).Scan(&keywords).Error; err != nil {
			return nil, err
		}
		rankings["keywords"] = keywords
		contactRows := []analyticRanking{}
		if err = analyticFilter(a.DB, w).Where("event_name IN ('contact_phone','contact_wechat')").Select("event_name name,COUNT(*) count").Group("event_name").Scan(&contactRows).Error; err != nil {
			return nil, err
		}
		rankings["contacts"] = contactRows
		resourceRows := []analyticRanking{}
		if err = analyticFilter(a.DB, w).Where("event_name='resource_view'").Select("target_id id,COUNT(*) count").Group("target_id").Order("count DESC,id").Limit(20).Scan(&resourceRows).Error; err != nil {
			return nil, err
		}
		for i := range resourceRows {
			var resource Resource
			var rev ResourceRevision
			if a.DB.First(&resource, resourceRows[i].ID).Error == nil && a.DB.First(&rev, resource.PublishedRevisionID).Error == nil {
				resourceRows[i].Name = rev.Title
			}
		}
		rankings["resources"] = resourceRows
		for _, part := range []struct {
			Field  string
			Target *[]analyticRanking
		}{{"source", &sources}, {"device", &devices}, {"referrer", &referrers}} {
			if err = analyticFilter(a.DB, w).Select(part.Field + " name,COUNT(*) count").Where("event_name='page_view'").Group(part.Field).Order("count DESC").Limit(20).Scan(part.Target).Error; err != nil {
				return nil, err
			}
		}
	} else {
		q := a.DB.Model(&AnalyticsDaily{}).Where("day>=? AND day<?", w.From.In(shanghai).Format("2006-01-02"), w.Until.In(shanghai).Format("2006-01-02"))
		if w.Source != "" {
			q = q.Where("source=?", w.Source)
		}
		if err := q.Order("day,source").Find(&daily).Error; err != nil {
			return nil, err
		}
		for _, day := range daily {
			summary.PV += day.PV
			summary.Contacts += day.Contacts
			summary.Searches += day.Searches
		}
	}
	return gin.H{"summary": summary, "daily": daily, "rankings": rankings, "sources": sources, "devices": devices, "referrers": referrers, "detail_available": detailed, "timezone": "Asia/Shanghai", "from": w.From.In(shanghai).Format("2006-01-02"), "to": w.Until.Add(-time.Second).In(shanghai).Format("2006-01-02")}, nil
}
func (a *App) analyticsReport(c *gin.Context) {
	window, err := analyticsDates(c)
	if err != nil {
		fail(c, 400, "invalid_date_range")
		return
	}
	data, err := a.analyticsData(window)
	if err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(200, data)
}
func csvSafe(value string) string {
	if value != "" && strings.ContainsAny(value[:1], "=+-@\t\r") {
		return "'" + value
	}
	return value
}
func (a *App) analyticsExport(c *gin.Context) {
	window, err := analyticsDates(c)
	if err != nil {
		fail(c, 400, "invalid_date_range")
		return
	}
	data, err := a.analyticsData(window)
	if err != nil {
		dbFail(c, err)
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=analytics.csv")
	writer := csv.NewWriter(c.Writer)
	c.Writer.Write([]byte{0xef, 0xbb, 0xbf})
	writer.Write([]string{"section", "date", "source", "metric", "name", "id", "value"})
	summary := data["summary"].(AnalyticsSummary)
	for _, row := range []struct {
		Key   string
		Value int64
	}{{"pv", summary.PV}, {"contact_clicks", summary.Contacts}, {"searches", summary.Searches}} {
		writer.Write([]string{"summary", "", window.Source, row.Key, "", "", strconv.FormatInt(row.Value, 10)})
	}
	if summary.UV != nil {
		writer.Write([]string{"summary", "", window.Source, "anonymous_uv", "", "", strconv.FormatInt(*summary.UV, 10)})
	}
	if summary.Visits != nil {
		writer.Write([]string{"summary", "", window.Source, "visits", "", "", strconv.FormatInt(*summary.Visits, 10)})
	}
	for _, day := range data["daily"].([]AnalyticsDaily) {
		for _, row := range []struct {
			Key   string
			Value int64
		}{{"pv", day.PV}, {"daily_anonymous_uv", day.UV}, {"daily_visits", day.Visits}, {"contact_clicks", day.Contacts}, {"searches", day.Searches}} {
			writer.Write([]string{"daily", day.Day, day.Source, row.Key, "", "", strconv.FormatInt(row.Value, 10)})
		}
	}
	rankings := data["rankings"].(map[string][]analyticRanking)
	for _, kind := range []string{"vendors", "products", "resources", "keywords", "contacts", "sources", "devices", "referrers"} {
		rows := rankings[kind]
		if rows == nil {
			if values, ok := data[kind].([]analyticRanking); ok {
				rows = values
			}
		}
		for _, row := range rows {
			writer.Write([]string{"ranking", "", window.Source, kind, csvSafe(row.Name), strconv.FormatUint(row.ID, 10), strconv.FormatInt(row.Count, 10)})
		}
	}
	writer.Flush()
	a.audit(actor(c).ID, "analytics", 0, "analytics_exported")
}
func (a *App) aggregateAnalytics() {
	// Recompute the current and previous day for delayed event delivery, and seal
	// every day about to leave the detail-retention window before deletion.
	now := time.Now().In(shanghai)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, shanghai)
	last := a.setting("analytics_aggregated_day", "")
	from := time.Date(2000, 1, 1, 0, 0, 0, 0, shanghai)
	if parsed, e := time.ParseInLocation("2006-01-02", last, shanghai); e == nil && parsed.After(from) {
		from = parsed.AddDate(0, 0, -1)
	}
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		rows := []AnalyticsDaily{}
		if e := tx.Model(&AnalyticsEvent{}).Where("occurred_at>=? AND occurred_at<?", from.UTC(), today.AddDate(0, 0, 1).UTC()).Select("DATE_FORMAT(DATE_ADD(occurred_at,INTERVAL 8 HOUR),'%Y-%m-%d') day, source, COALESCE(SUM(event_name='page_view'),0) pv, COUNT(DISTINCT visitor_hash) uv, COUNT(DISTINCT session_hash) visits, COALESCE(SUM(event_name IN ('contact_phone','contact_wechat')),0) contacts, COALESCE(SUM(event_name='search'),0) searches").Group("day,source").Scan(&rows).Error; e != nil {
			return e
		}
		if len(rows) > 0 {
			if e := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&rows).Error; e != nil {
				return e
			}
		}
		if e := tx.Where("occurred_at<?", today.AddDate(0, 0, -90).UTC()).Delete(&AnalyticsEvent{}).Error; e != nil {
			return e
		}
		return tx.Where("day<?", today.AddDate(-2, 0, 0).Format("2006-01-02")).Delete(&AnalyticsDaily{}).Error
	})
	if err == nil {
		a.setSetting("analytics_aggregated_day", today.Format("2006-01-02"))
	}
}
