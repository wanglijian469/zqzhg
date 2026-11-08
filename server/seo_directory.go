package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
)

func hallSEOPath(path, hall string, values url.Values) string {
	if values == nil {
		values = url.Values{}
	}
	values.Set("hall", hall)
	return canonicalPublic("", path, values)
}

func facetSEOPath(path, hall, key, value string) string {
	return hallSEOPath(path, hall, url.Values{key: {value}})
}

// Catalog and membership use the same public revision, category graph and
// administrative snapshots as the interactive directory.
func (a *App) seoCategoryFacets(hall uint64, kind string) ([]Category, error) {
	var categories []Category
	var aliases []CategoryAlias
	var unavailable []uint64
	var assigned []uint64
	if err := a.DB.Order("sort_order,id").Find(&categories).Error; err != nil {
		return nil, err
	}
	if err := a.DB.Find(&aliases).Error; err != nil {
		return nil, err
	}
	if err := a.DB.Model(&CategorySuggestion{}).Where("status <> 'approved'").Pluck("category_id", &unavailable).Error; err != nil {
		return nil, err
	}
	if err := a.publicQuery(kind).Where("contents.hall_id=?", hall).
		Joins("JOIN revision_categories seo_rc ON seo_rc.revision_id=r.id").
		Distinct("seo_rc.category_id").Pluck("seo_rc.category_id", &assigned).Error; err != nil {
		return nil, err
	}
	excluded, used := map[uint64]bool{}, map[uint64]bool{}
	for _, alias := range aliases {
		excluded[alias.SourceID] = true
	}
	for _, id := range unavailable {
		excluded[id] = true
	}
	for _, id := range assigned {
		used[id] = true
	}
	var result []Category
	for _, category := range categories {
		if category.HallID != hall || category.CategoryKind != kind ||
			excluded[category.ID] || !categoryBranchEnabled(categories, category.ID) {
			continue
		}
		for _, id := range categoryGraphMembers(categories, aliases, []uint64{category.ID}) {
			if used[id] {
				result = append(result, category)
				break
			}
		}
	}
	return result, nil
}

func (a *App) seoRegionFacets(hall uint64) (map[string]bool, error) {
	loadAdministrativeRegions()
	var rows []struct{ Province, City, District string }
	err := a.publicQuery("vendor").Where("contents.hall_id=?", hall).
		Distinct("COALESCE(r.province_code,'') AS province, COALESCE(r.city_code,'') AS city, COALESCE(r.district_code,'') AS district").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	active := map[string]bool{}
	for _, row := range rows {
		parent := ""
		for _, code := range []string{row.Province, row.City, row.District} {
			node, ok := administrativeByCode[code]
			if !ok || node.ParentCode != parent {
				break
			}
			active[code] = true
			parent = code
		}
	}
	return active, nil
}

func cleanDirectoryVariant(c *gin.Context, facet string) bool {
	if p := c.Query("page"); p != "" && p != "1" {
		return false
	}
	for _, key := range []string{"q", "category_id", "region_code", "region_id", "unassigned", "vendor_id", "sort", "business_nature", "industrial_belt_id"} {
		if key != facet && c.Query(key) != "" {
			return false
		}
	}
	return true
}

type directorySEOPage struct {
	Title, Description, Vendors, Products, Regions string
	Entries, Facets, Pages                         []seoLink
	Home                                           bool
}

var seoLandingBody = template.Must(template.New("public-landing").Parse(`<main class="container section seo-content" data-seo-content>
<h1>{{.Title}}</h1><p>{{.Description}}</p>
<nav aria-label="展馆目录"><a href="{{.Vendors}}">企业名单与厂家展厅</a> · <a href="{{.Products}}">产品展示</a> · <a href="{{.Regions}}">按地区找厂商</a></nav>
{{if .Facets}}<nav aria-label="分类目录"><ul>{{range .Facets}}<li><a href="{{.URL}}">{{.Name}}</a></li>{{end}}</ul></nav>{{end}}
{{if .Entries}}<section><h2>{{if .Home}}推荐企业展厅{{else}}公开目录{{end}}</h2><ul>{{range .Entries}}<li><a href="{{.URL}}">{{.Name}}</a></li>{{end}}</ul></section>{{else if not .Home}}<p>暂无符合条件的公开资料。</p>{{end}}
{{if .Pages}}<nav aria-label="分页">{{range .Pages}}<a href="{{.URL}}">{{.Name}}</a> {{end}}</nav>{{end}}
</main>`))

func (a *App) directorySEO(c *gin.Context, hall Hall, title, desc string) (seoContent, string, string, bool, error) {
	path := c.Request.URL.Path
	page := directorySEOPage{Title: title, Description: desc, Home: path == "/",
		Vendors: hallSEOPath("/browse", hall.Slug, nil), Products: hallSEOPath("/browse/products", hall.Slug, nil),
		Regions: hallSEOPath("/regions", hall.Slug, nil)}
	kind, facet, value, name := "vendor", "category_id", c.Query("category_id"), ""
	if path == "/browse/products" {
		kind = "product"
	}
	var items []View
	indexable := page.Home && cleanDirectoryVariant(c, "")
	var err error
	if page.Home {
		items, err = a.homeVendors(hall.ID)
	} else {
		q := a.publicQuery(kind).WithContext(c.Request.Context()).Where("contents.hall_id=?", hall.ID)
		order := publicOrder(c, kind)
		size := 20
		if kind == "product" {
			size = 24
		}
		if path == "/regions" {
			facet, value = "region_code", c.Query("region_code")
			active, e := a.seoRegionFacets(hall.ID)
			if e != nil {
				return seoContent{}, title, desc, false, e
			}
			q = a.filterVendors(c, q)
			order = "contents.featured DESC, contents.sort_order ASC, contents.id DESC"
			node, valid := administrativeByCode[value]
			if value != "" && !valid {
				q = q.Where("1=0")
			}
			if valid {
				q = industryRegionQuery(q, &node)
				if active[value] {
					name = node.FullName
				}
				page.Facets = append(page.Facets, seoLink{facetSEOPath(path, hall.Slug, facet, node.ParentCode), "返回上一级地区"})
			}
			if c.Query("unassigned") == "1" {
				var selected *AdministrativeRegion
				if valid {
					selected = &node
				}
				q = industryUnassigned(q, selected)
			}
			for _, child := range administrativeChildren[value] {
				if active[child.Code] {
					page.Facets = append(page.Facets, seoLink{facetSEOPath(path, hall.Slug, facet, child.Code), child.Name})
				}
			}
		} else {
			categories, e := a.seoCategoryFacets(hall.ID, kind)
			if e != nil {
				return seoContent{}, title, desc, false, e
			}
			selected := positiveFilterID(value)
			for _, category := range categories {
				if category.ID == selected && value == strconv.FormatUint(category.ID, 10) {
					name = category.Name
					parent := ""
					if category.ParentID != 0 {
						parent = strconv.FormatUint(category.ParentID, 10)
					}
					page.Facets = append(page.Facets, seoLink{facetSEOPath(path, hall.Slug, facet, parent), "返回上一级分类"})
				}
				if (value == "" && category.ParentID == 0) || category.ParentID == selected {
					page.Facets = append(page.Facets, seoLink{facetSEOPath(path, hall.Slug, facet, strconv.FormatUint(category.ID, 10)), category.Name})
				}
			}
			if kind == "product" {
				q = a.filterProducts(c, q)
			} else {
				q = a.filterDirectoryVendors(c, q)
			}
		}
		var total int64
		if err = q.Count(&total).Error; err != nil {
			return seoContent{}, title, desc, false, err
		}
		p, _ := publicPage(c, size)
		last := (total + int64(size) - 1) / int64(size)
		if int64(p) <= last {
			items, err = a.publicViews(q.Order(order).Offset((p - 1) * size).Limit(size))
		}
		indexable = cleanDirectoryVariant(c, facet) && (value == "" || name != "") && len(items) > 0
		if name != "" {
			page.Title = name + " | " + title
			page.Description = "浏览" + name + "的公开" + map[string]string{"vendor": "企业展厅", "product": "产品"}[kind] + "。"
		}
		for _, offset := range []int{-1, 1} {
			n := p + offset
			if n < 1 || int64(n) > last {
				continue
			}
			query := c.Request.URL.Query()
			query.Set("page", strconv.Itoa(n))
			page.Pages = append(page.Pages, seoLink{hallSEOPath(path, hall.Slug, query), fmt.Sprintf("第 %d 页", n)})
		}
	}
	if err != nil {
		return seoContent{}, title, desc, false, err
	}
	for _, item := range items {
		page.Entries = append(page.Entries, seoLink{contentSearchURL("", item.Content), item.Data.Name})
	}
	var body bytes.Buffer
	err = seoLandingBody.Execute(&body, page)
	return seoContent{Body: body.String()}, page.Title, page.Description, indexable, err
}
