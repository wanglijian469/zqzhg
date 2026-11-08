package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const sitemapPageSize = 1000
const sitemapMaxURLs = 50000
const sitemapMaxBytes = 50 * 1024 * 1024
const sitemapNamespace = "http://www.sitemaps.org/schemas/sitemap/0.9"

var sitemapFile = regexp.MustCompile(`^(pages|vendors|products)-([1-9][0-9]{0,6})\.xml$`)

type sitemapLocation struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}
type sitemapIndex struct {
	XMLName xml.Name          `xml:"sitemapindex"`
	XMLNS   string            `xml:"xmlns,attr"`
	Entries []sitemapLocation `xml:"sitemap"`
}
type sitemapURLs struct {
	XMLName xml.Name          `xml:"urlset"`
	XMLNS   string            `xml:"xmlns,attr"`
	Entries []sitemapLocation `xml:"url"`
}

func sitemapError(c *gin.Context, status int) {
	c.Header("Cache-Control", "no-store")
	c.Header("X-Robots-Tag", "noindex")
	code := "not_found"
	if status == 503 {
		code = "database_unavailable"
	}
	fail(c, status, code)
}

func sendSitemap(c *gin.Context, value any) {
	raw, err := xml.Marshal(value)
	if err != nil {
		sitemapError(c, 503)
		return
	}
	sendSitemapXML(c, raw)
}

func sendSitemapXML(c *gin.Context, raw []byte) {
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Type", "application/xml; charset=utf-8")
	if c.Request.Method == "HEAD" {
		c.Status(200)
		return
	}
	c.Data(200, "application/xml; charset=utf-8", append([]byte(xml.Header), raw...))
}

func (a *App) sitemapReady(c *gin.Context) bool {
	if info, err := os.Stat(filepath.Join(a.Config.WebDir, "index.html")); err != nil || info.IsDir() {
		sitemapError(c, 503)
		return false
	}
	return true
}

func (a *App) sitemapIndex(c *gin.Context) {
	if !a.sitemapReady(c) {
		return
	}
	base := a.publicBase(c)
	pages, err := a.sitemapPages(base)
	if err != nil {
		sitemapError(c, 503)
		return
	}
	counts := map[string]int64{"pages": int64(len(pages))}
	total := int64(len(pages))
	for _, kind := range []string{"vendor", "product"} {
		var count int64
		if err := a.publicQuery(kind).Count(&count).Error; err != nil {
			sitemapError(c, 503)
			return
		}
		counts[kind+"s"] = count
		total += count
	}
	if total <= sitemapMaxURLs {
		for _, kind := range []string{"vendor", "product"} {
			items, err := a.sitemapContents(base, kind, 0, sitemapMaxURLs+1-len(pages))
			if err != nil {
				sitemapError(c, 503)
				return
			}
			pages = append(pages, items...)
		}
		if len(pages) <= sitemapMaxURLs {
			raw, err := xml.Marshal(sitemapURLs{XMLNS: sitemapNamespace, Entries: pages})
			if err != nil {
				sitemapError(c, 503)
				return
			}
			if len(raw)+len(xml.Header) <= sitemapMaxBytes {
				sendSitemapXML(c, raw)
				return
			}
		}
	}
	index := sitemapIndex{XMLNS: sitemapNamespace}
	for _, kind := range []string{"pages", "vendors", "products"} {
		for page := int64(1); page <= (counts[kind]+sitemapPageSize-1)/sitemapPageSize; page++ {
			index.Entries = append(index.Entries, sitemapLocation{Loc: fmt.Sprintf("%s/sitemap-%s-%d.xml", base, kind, page)})
		}
	}
	sendSitemap(c, index)
}

func (a *App) sitemapPages(base string) ([]sitemapLocation, error) {
	var halls []Hall
	if err := a.DB.Where("status=?", "open").Order("id ASC").Find(&halls).Error; err != nil {
		return nil, err
	}
	pages := []sitemapLocation{{Loc: base + "/guide"}}
	now := time.Now().UTC()
	for _, key := range serviceDocumentKeys {
		var docs []ServiceDocument
		if err := a.DB.Where("doc_key=? AND status='published' AND effective_at<=?", key, now).Order("version DESC").Limit(1).Find(&docs).Error; err != nil {
			return nil, err
		}
		if len(docs) == 0 {
			continue
		}
		path := "/services/" + key
		if key == "service-info" {
			path = "/services"
		}
		doc := docs[0]
		pages = append(pages, sitemapLocation{Loc: base + path, LastMod: sitemapLastMod(doc.PublishedAt, &doc.EffectiveAt)})
	}
	for _, hall := range halls {
		pages = append(pages, sitemapLocation{Loc: base + hallSEOPath("/", hall.Slug, nil)})
		for _, entry := range []struct{ kind, path string }{{"vendor", "/browse"}, {"product", "/browse/products"}} {
			var ids []uint64
			if err := a.publicQuery(entry.kind).Where("contents.hall_id=?", hall.ID).Limit(1).Pluck("contents.id", &ids).Error; err != nil {
				return nil, err
			}
			if len(ids) == 0 {
				continue
			}
			pages = append(pages, sitemapLocation{Loc: base + hallSEOPath(entry.path, hall.Slug, nil)})
			if entry.kind == "vendor" {
				pages = append(pages, sitemapLocation{Loc: base + hallSEOPath("/regions", hall.Slug, nil)})
			}
			categories, err := a.seoCategoryFacets(hall.ID, entry.kind)
			if err != nil {
				return nil, err
			}
			for _, category := range categories {
				pages = append(pages, sitemapLocation{Loc: base + facetSEOPath(entry.path, hall.Slug, "category_id", strconv.FormatUint(category.ID, 10))})
			}
		}
		active, err := a.seoRegionFacets(hall.ID)
		if err != nil {
			return nil, err
		}
		codes := make([]string, 0, len(active))
		for code := range active {
			codes = append(codes, code)
		}
		slices.Sort(codes)
		for _, code := range codes {
			pages = append(pages, sitemapLocation{Loc: base + facetSEOPath("/regions", hall.Slug, "region_code", code)})
		}
	}
	return pages, nil
}

func sitemapLastMod(times ...*time.Time) string {
	var modified time.Time
	for _, t := range times {
		if t != nil && t.After(modified) {
			modified = *t
		}
	}
	if modified.IsZero() {
		return ""
	}
	return modified.UTC().Format(time.RFC3339)
}

func (a *App) sitemapContents(base, kind string, offset, limit int) ([]sitemapLocation, error) {
	var rows []struct {
		ID           uint64
		ShowroomSlug *string
		Published    *time.Time
		Parent       *time.Time
	}
	fields := "contents.id, contents.showroom_slug, GREATEST(r.created_at, r.updated_at, COALESCE(r.reviewed_at,r.created_at)) AS published"
	if kind == "product" {
		fields += ", GREATEST(vr.created_at, vr.updated_at, COALESCE(vr.reviewed_at,vr.created_at)) AS parent"
	}
	if err := a.publicQuery(kind).Select(fields).Order("contents.id ASC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	entries := make([]sitemapLocation, 0, len(rows))
	for _, item := range rows {
		loc := contentSearchURL(base, Content{ID: item.ID, Kind: kind, ShowroomSlug: item.ShowroomSlug})
		entries = append(entries, sitemapLocation{Loc: loc, LastMod: sitemapLastMod(item.Published, item.Parent)})
	}
	return entries, nil
}

func (a *App) sitemapPage(c *gin.Context) {
	if c.Param("file") == "dead-links.txt" {
		var pages []SearchPage
		if e := a.DB.Where("was_public=1 AND is_public=0").Order("content_id").Limit(10000).Find(&pages).Error; e != nil {
			sitemapError(c, 503)
			return
		}
		lines := []string{}
		for _, p := range pages {
			if strings.HasPrefix(p.URL, searchBase(a)+"/") {
				lines = append(lines, p.URL)
			}
		}
		c.Header("Cache-Control", "no-store")
		c.Data(200, "text/plain; charset=utf-8", []byte(strings.Join(lines, "\n")))
		return
	}
	file := c.Param("file")
	if file == "pages.xml" {
		file = "pages-1.xml"
	}
	match := sitemapFile.FindStringSubmatch(file)
	if match == nil {
		sitemapError(c, 404)
		return
	}
	page, err := strconv.Atoi(match[2])
	if err != nil {
		sitemapError(c, 404)
		return
	}
	if !a.sitemapReady(c) {
		return
	}
	offset := (page - 1) * sitemapPageSize
	base := a.publicBase(c)
	var entries []sitemapLocation
	if match[1] == "pages" {
		entries, err = a.sitemapPages(base)
		if offset < len(entries) {
			entries = entries[offset:min(len(entries), offset+sitemapPageSize)]
		} else {
			entries = nil
		}
	} else {
		entries, err = a.sitemapContents(base, strings.TrimSuffix(match[1], "s"), offset, sitemapPageSize)
	}
	if err != nil {
		sitemapError(c, 503)
		return
	}
	if len(entries) == 0 {
		sitemapError(c, 404)
		return
	}
	sendSitemap(c, sitemapURLs{XMLNS: sitemapNamespace, Entries: entries})
}

func (a *App) publicRobots(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Data(200, "text/plain; charset=utf-8", []byte(robotsPolicy()+"Sitemap: "+a.publicBase(c)+"/sitemap.xml\n"))
}
