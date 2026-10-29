package main

import (
	"github.com/gin-gonic/gin"
	"time"
)

// Explicit operator-selected demonstration mode. Source metadata is preserved;
// this is not an approved-resource record and does not invent a map review ID.
const mapDemoUsage = "development-demonstration"

func (a *App) mapDemoConfiguration(c *gin.Context) {
	if !a.Config.MapDemoEnabled {
		fail(c, 404, "not_found")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"enabled": true, "usage": mapDemoUsage, "source": "DataV / 高德开放平台", "source_terms": "https://help.aliyun.com/zh/datav/datav-7-0/user-guide/datav-geoatlas-widgets/", "publication_approval_claimed": false})
}
func (a *App) mapDemoRegions(c *gin.Context) {
	if !a.Config.MapDemoEnabled {
		fail(c, 404, "not_found")
		return
	}
	if !permit("map-demo:"+c.ClientIP(), 90, time.Minute) {
		fail(c, 429, "rate_limited")
		return
	}
	node, ok := industryRegion(c)
	if !ok {
		return
	}
	a.mapPreviewMu.Lock()
	defer a.mapPreviewMu.Unlock()
	directoryApp := &App{Config: Config{MapPreviewDir: a.Config.MapDemoDir, WebDir: a.Config.WebDir}}
	directory, err := directoryApp.mapPreviewDirectory()
	if err != nil {
		fail(c, 503, "map_boundary_unavailable")
		return
	}
	resolver := previewResolver{app: a, ctx: c.Request.Context(), sources: map[string]previewSource{}, directory: directory}
	result, err := resolver.boundary(node)
	if err != nil {
		fail(c, 502, "map_boundary_unavailable")
		return
	}
	result.Usage = mapDemoUsage
	c.Header("Cache-Control", "no-store")
	c.JSON(200, result)
}
