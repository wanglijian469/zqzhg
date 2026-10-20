package main

import (
	"errors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"html"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func (a *App) publicBase(c *gin.Context) string {
	cfg, _ := a.systemConfig()
	if cfg.Site.WebURL != "" {
		return strings.TrimRight(cfg.Site.WebURL, "/")
	}
	if a.Config.PublicBaseURL != "" {
		return strings.TrimRight(a.Config.PublicBaseURL, "/")
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}
func canonicalPublic(base, path string, query url.Values) string {
	q := url.Values{}
	company := strings.HasPrefix(path, "/v/")
	keys := []string{"hall", "q", "category_id", "region_id", "vendor_id", "sort", "page"}
	if path == "/regions" {
		keys = []string{"hall", "q", "category_id", "region_code", "unassigned", "page"}
	}
	if company {
		keys = []string{"tab", "q", "category_id", "sort", "page"}
	}
	if company || path == "/regions" || path == "/" || path == "/browse" || path == "/browse/products" {
		for _, k := range keys {
			v := strings.TrimSpace(query.Get(k))
			if k == "tab" && (v == "home" || !strings.Contains("|about|products|videos|catalogs|contact|", "|"+v+"|")) {
				continue
			}
			if v == "" || (k == "hall" && v == "agriculture") || (k == "page" && v == "1") {
				continue
			}
			if k == "page" {
				n, e := strconv.Atoi(v)
				if e != nil || n < 2 {
					continue
				}
				v = strconv.Itoa(n)
			}
			q.Set(k, v)
		}
	}
	suffix := ""
	if len(q) > 0 {
		suffix = "?" + q.Encode()
	}
	return strings.TrimRight(base, "/") + path + suffix
}
func cleanHTMLMetadata(source string) string {
	for _, pattern := range []string{`(?is)<title[^>]*>.*?</title\s*>`, `(?is)<meta\b[^>]*(?:name\s*=\s*["'](?:description|robots|zqzg-public-base)["']|property\s*=\s*["']og:[^"']+["'])[^>]*>`, `(?is)<link\b[^>]*rel\s*=\s*["']canonical["'][^>]*>`} {
		source = regexp.MustCompile(pattern).ReplaceAllString(source, "")
	}
	return source
}
func (a *App) sendPublicHTML(c *gin.Context, index string, status int, title, desc, canonical, robots string) bool {
	source, e := os.ReadFile(index)
	if e != nil {
		return false
	}
	escape := html.EscapeString
	meta := "<title>" + escape(title) + "</title><meta name='description' content='" + escape(desc) + "'><link rel='canonical' href='" + escape(canonical) + "'><meta name='robots' content='" + robots + "'><meta name='zqzg-public-base' content='" + escape(a.publicBase(c)) + "'>"
	for k, v := range map[string]string{"og:title": title, "og:description": desc, "og:url": canonical, "og:type": "website"} {
		meta += "<meta property='" + k + "' content='" + escape(v) + "'>"
	}
	c.Header("Cache-Control", "no-store")
	if robots == "noindex,follow" {
		c.Header("X-Robots-Tag", robots)
	}
	c.Data(status, "text/html; charset=utf-8", []byte(a.runtimeHTML(c, strings.Replace(cleanHTMLMetadata(string(source)), "<head>", "<head>"+meta, 1))))
	return true
}
func (a *App) publicHTML(c *gin.Context, index string) bool {
	path := c.Request.URL.Path
	if path != "/regions" && path != "/" && path != "/browse" && path != "/browse/products" && !strings.HasPrefix(path, "/products/") {
		return false
	}
	title, desc, robots, status := a.brandName(), "汇聚制造企业，展示特色产品，连接采购需求", "index,follow", 200
	canonical := canonicalPublic(a.publicBase(c), path, c.Request.URL.Query())
	if strings.HasPrefix(path, "/products/") {
		id, e := strconv.ParseUint(strings.TrimPrefix(path, "/products/"), 10, 64)
		if e != nil || id == 0 {
			return a.sendPublicHTML(c, index, 404, "产品暂未公开 | "+a.brandName(), "产品不存在、暂未公开或已下架。", canonical, "noindex,follow")
		}
		items, e := a.publicViews(a.publicQuery("product").Where("contents.id=?", id))
		if e != nil {
			return a.sendPublicHTML(c, index, 503, "服务暂不可用 | "+a.brandName(), "请稍后重试。", canonical, "noindex,follow")
		}
		if len(items) == 0 {
			return a.sendPublicHTML(c, index, 404, "产品暂未公开 | "+a.brandName(), "产品不存在、暂未公开或已下架。", canonical, "noindex,follow")
		}
		p := items[0]
		var hall Hall
		if a.DB.First(&hall, p.HallID).Error != nil {
			return a.sendPublicHTML(c, index, 503, title, desc, canonical, "noindex,follow")
		}
		vendors, e := a.publicViews(a.publicQuery("vendor").Where("contents.id=?", p.VendorID))
		if e != nil || len(vendors) == 0 {
			return a.sendPublicHTML(c, index, 404, "产品暂未公开 | "+a.brandName(), "产品不存在、暂未公开或已下架。", canonical, "noindex,follow")
		}
		title = p.Data.Name + " | " + a.brandName() + " · " + hall.Name

		parts := []string{p.Data.Name}
		model := regexp.MustCompile(`(?im)^\s*(?:型号|model)\s*[:：]\s*([^\r\n]+)`).FindStringSubmatch(p.Data.Parameters)
		if len(model) > 1 {
			parts = append(parts, "型号 "+model[1])
		}
		parts = append(parts, vendors[0].Data.Name, hall.Name)
		if strings.TrimSpace(p.Data.Description) != "" {
			parts = append(parts, p.Data.Description)
		}
		desc = strings.Join(parts, " · ")
	} else {
		var hall Hall
		if e := a.DB.Where("slug=?", c.DefaultQuery("hall", "agriculture")).First(&hall).Error; e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				status = 404
			} else {
				status = 503
			}
			return a.sendPublicHTML(c, index, status, "展馆暂不可用 | "+a.brandName(), "展馆不存在或暂不可用。", canonical, "noindex,follow")
		}
		title = a.brandName() + " · " + hall.Name
		if strings.TrimSpace(hall.Description) != "" {
			desc = hall.Description
		}
		if hall.Status != "open" {
			desc = hall.Description
			if strings.TrimSpace(desc) == "" {
				desc = hall.Name + "正在筹建，敬请期待。"
			}
			robots = "noindex,follow"
		} else if path == "/regions" {
			title = "厂商地图 | " + title
			desc = "按行政区划浏览" + hall.Name + "的公开厂商。行政目录覆盖大陆31个省级行政区，版本2023-06-30。"
		} else if path == "/browse" {
			title = "找厂家 | " + title
			desc = "按产品、主营业务和所在地区，快速发现" + hall.Name + "的制造企业。"
		} else if path == "/browse/products" {
			title = "找产品 | " + title
			desc = "按名称、型号、分类和地区，发现" + hall.Name + "的真实产品。"
		}
		for _, k := range []string{"q", "category_id", "region_id", "region_code", "unassigned", "vendor_id", "sort"} {
			if c.Query(k) != "" {
				robots = "noindex,follow"
			}
		}
	}
	runes := []rune(strings.TrimSpace(desc))
	if len(runes) > 160 {
		desc = string(runes[:160])
	}
	return a.sendPublicHTML(c, index, status, title, desc, canonical, robots)
}
