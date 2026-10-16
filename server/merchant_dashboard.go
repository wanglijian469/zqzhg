package main

import (
	"github.com/gin-gonic/gin"
	"net/url"
	"strings"
)

// The same publication predicate is used for filters and full-account counts.
const contentPublicSQL = "contents.visible=1 AND EXISTS (SELECT 1 FROM revisions pr WHERE pr.id=contents.published_revision_id AND pr.status='approved') AND (contents.kind='vendor' OR EXISTS (SELECT 1 FROM contents pv JOIN revisions pvr ON pvr.id=pv.published_revision_id AND pvr.status='approved' WHERE pv.id=contents.vendor_id AND pv.kind='vendor' AND pv.visible=1))"

func (a *App) merchantDashboard(c *gin.Context) {
	var counts struct {
		Total    int64 `json:"total"`
		Public   int64 `json:"public"`
		Draft    int64 `json:"draft"`
		Pending  int64 `json:"pending"`
		Rejected int64 `json:"rejected"`
	}
	q := a.DB.Model(&Content{}).Joins("JOIN revisions dr ON dr.id=contents.draft_revision_id").Where("contents.owner_id=? AND contents.kind='product'", actor(c).ID)
	err := q.Select("COUNT(*) AS total, COALESCE(SUM(" + contentPublicSQL + "),0) AS public, COALESCE(SUM(dr.status='draft'),0) AS draft, COALESCE(SUM(dr.status='pending'),0) AS pending, COALESCE(SUM(dr.status='rejected'),0) AS rejected").Scan(&counts).Error
	if err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(200, gin.H{"products": counts})
}

func (a *App) publicApp(c *gin.Context) {
	base := strings.TrimRight(env("APP_DOWNLOAD_BASE_URL", a.Config.PublicBaseURL), "/")
	if validatePublicBase(base) != nil {
		base = ""
	}
	if base == "" {
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		base = scheme + "://" + c.Request.Host
	}
	u, err := url.Parse(base)
	if err != nil {
		fail(c, 500, "internal_error")
		return
	}
	host := u.Hostname()
	local := host == "localhost" || host == "127.0.0.1" || host == "::1"
	qr := base + "/downloads/dalunongji.apk"
	if local {
		qr = ""
	}
	c.JSON(200, gin.H{"android_url": "/downloads/dalunongji.apk", "qr_url": qr, "base_url": base, "ios_available": false})
}
