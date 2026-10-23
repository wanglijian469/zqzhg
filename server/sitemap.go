package main

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
)

const sitemapPageSize = 1000
const sitemapNamespace = "http://www.sitemaps.org/schemas/sitemap/0.9"

var sitemapFile = regexp.MustCompile(`^(vendors|products)-([1-9][0-9]{0,6})\.xml$`)

type sitemapLocation struct {
	Loc string `xml:"loc"`
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
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Type", "application/xml; charset=utf-8")
	if c.Request.Method == "HEAD" {
		c.Status(200)
		return
	}
	c.Data(200, "application/xml; charset=utf-8", append([]byte(xml.Header), raw...))
}

func (a *App) sitemapIndex(c *gin.Context) {
	index := sitemapIndex{XMLNS: sitemapNamespace}
	base := a.publicBase(c)
	for _, kind := range []string{"vendor", "product"} {
		var total int64
		if err := a.publicQuery(kind).Count(&total).Error; err != nil {
			sitemapError(c, 503)
			return
		}
		for page := int64(1); page <= (total+sitemapPageSize-1)/sitemapPageSize; page++ {
			index.Entries = append(index.Entries, sitemapLocation{fmt.Sprintf("%s/sitemaps/%ss-%d.xml", base, kind, page)})
		}
	}
	sendSitemap(c, index)
}

func (a *App) sitemapPage(c *gin.Context) {
	match := sitemapFile.FindStringSubmatch(c.Param("file"))
	if match == nil {
		sitemapError(c, 404)
		return
	}
	page, err := strconv.Atoi(match[2])
	if err != nil {
		sitemapError(c, 404)
		return
	}
	kind := "vendor"
	if match[1] == "products" {
		kind = "product"
	}
	var items []Content
	if err = a.publicQuery(kind).Select("contents.id, contents.showroom_slug").Order("contents.id ASC").Offset((page - 1) * sitemapPageSize).Limit(sitemapPageSize).Find(&items).Error; err != nil {
		sitemapError(c, 503)
		return
	}
	if len(items) == 0 {
		sitemapError(c, 404)
		return
	}
	urls := sitemapURLs{XMLNS: sitemapNamespace}
	base := a.publicBase(c)
	for _, item := range items {
		path := fmt.Sprintf("/products/%d", item.ID)
		if kind == "vendor" {
			// The narrow SELECT above deliberately omits kind and private revision data.
			item.Kind = "vendor"
			path = "/v/" + showroomSlug(item)
		}
		urls.Entries = append(urls.Entries, sitemapLocation{base + path})
	}
	sendSitemap(c, urls)
}

func (a *App) publicRobots(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Data(200, "text/plain; charset=utf-8", []byte(robotsPolicy()+"Sitemap: "+a.publicBase(c)+"/sitemap.xml\n"))
}
