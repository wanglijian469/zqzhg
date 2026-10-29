package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/url"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// Only an empty application root is replaced. React owns it after mounting.
var seoRoot = regexp.MustCompile(`<div\b[^>]*\bid=["']root["'][^>]*>\s*</div>`)
var seoModel = regexp.MustCompile(`(?im)^\s*(?:型号|model)\s*[:：]\s*([^\r\n]+)`)

type seoContent struct {
	Body       string
	Image      string
	Keywords   string
	Structured map[string]any
}

type seoImage struct{ URL, Alt string }
type seoLink struct{ URL, Name string }
type seoPage struct {
	Company  bool
	Data     Payload
	Model    string
	Images   []seoImage
	Products []seoLink
	Vendor   seoLink
}

var seoBody = template.Must(template.New("public-content").Funcs(template.FuncMap{"previewImage": func(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	query := u.Query()
	query.Set("w", "480")
	u.RawQuery = query.Encode()
	return u.String()
}}).Parse(`<main class="container section seo-content" data-seo-content>
<article><h1>{{.Data.Name}}</h1>
{{range .Images}}<img src="{{previewImage .URL}}" alt="{{.Alt}}" style="max-width:100%;object-fit:contain" width="320" height="240" loading="lazy" decoding="async">{{end}}
{{if .Company}}
{{with .Data.MainProducts}}<section><h2>主营产品</h2><p>{{.}}</p></section>{{end}}
{{with .Data.CompanyProfile}}{{with .Summary}}<section><h2>企业简介</h2><p style="white-space:pre-wrap;overflow-wrap:anywhere">{{.}}</p></section>{{end}}{{end}}
{{end}}
{{with .Data.Description}}<section><h2>{{if $.Company}}企业介绍{{else}}产品介绍{{end}}</h2><p style="white-space:pre-wrap;overflow-wrap:anywhere">{{.}}</p></section>{{end}}
{{if not .Company}}
{{with .Model}}<p>型号：{{.}}</p>{{end}}{{with .Data.Brand}}<p>品牌：{{.}}</p>{{end}}
{{with .Data.Parameters}}<section><h2>产品参数</h2><p style="white-space:pre-wrap;overflow-wrap:anywhere">{{.}}</p></section>{{end}}
<p>所属厂商：<a href="{{.Vendor.URL}}">{{.Vendor.Name}}</a></p>
{{else}}
{{with .Data.RegionName}}<p>所在地区：{{.}}</p>{{end}}
<section><h2>联系企业</h2>
{{with .Data.Address}}<p>地址：{{.}}</p>{{end}}
{{with .Data.Phone}}<p>电话：<a href="tel:{{.}}">{{.}}</a></p>{{end}}
{{with .Data.Wechat}}<p>微信：{{.}}</p>{{end}}
{{with .Data.CompanyProfile}}
{{with .ContactName}}<p>联系人：{{.}}</p>{{end}}
{{with .Website}}<p>企业网站：<a href="{{.}}">{{.}}</a></p>{{end}}
{{range .Contacts}}<p>{{.Area}} {{.Name}} {{with .Phone}}<a href="tel:{{.}}">{{.}}</a>{{end}} {{with .Wechat}}微信：{{.}}{{end}}</p>{{end}}
{{end}}</section>
{{if .Products}}<section><h2>主营产品展示</h2><ul>{{range .Products}}<li><a href="{{.URL}}">{{.Name}}</a></li>{{end}}</ul></section>{{end}}
{{end}}
</article></main>`))

func seoProductModel(p Payload) string {
	if model := strings.TrimSpace(p.Model); model != "" {
		return model
	}
	if matches := seoModel.FindStringSubmatch(p.Parameters); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func renderSEO(page seoPage, structured map[string]any) (seoContent, error) {
	var body bytes.Buffer
	if err := seoBody.Execute(&body, page); err != nil {
		return seoContent{}, err
	}
	result := seoContent{Body: body.String(), Structured: structured}
	if len(page.Images) > 0 {
		result.Image = page.Images[0].URL
	}
	return result, nil
}

func (a *App) vendorSEO(c *gin.Context, item View, products []View, canonical string) (seoContent, error) {
	d := item.Data
	page := seoPage{Company: true, Data: d}
	org := map[string]any{"@context": "https://schema.org", "@type": "Organization", "name": d.Name, "url": canonical}
	for k, v := range map[string]string{"telephone": d.Phone, "address": d.Address, "description": d.Description} {
		if strings.TrimSpace(v) != "" {
			org[k] = v
		}
	}
	cover := uint64(0)
	if p := d.CompanyProfile; p != nil {
		cover = p.CoverMediaID
		if p.Summary != "" {
			org["description"] = p.Summary
		}
		if p.LogoMediaID > 0 {
			org["logo"] = a.mediaAddress(c, p.LogoMediaID, "")
		}
	}
	if cover == 0 && len(d.MediaIDs) > 0 {
		cover = d.MediaIDs[0]
	}
	if cover > 0 {
		image := a.mediaAddress(c, cover, "")
		page.Images = []seoImage{{image, d.Name}}
		org["image"] = image
	}
	for _, product := range products {
		page.Products = append(page.Products, seoLink{fmt.Sprintf("/products/%d", product.ID), product.Data.Name})
	}
	result, err := renderSEO(page, org)
	result.Keywords = vendorSEOKeywords(d)
	return result, err
}

func (a *App) productSEO(c *gin.Context, item, vendor View, canonical string) (seoContent, error) {
	d := item.Data
	vendorPath := "/v/" + vendor.ShowroomSlug
	page := seoPage{Data: d, Model: seoProductModel(d), Vendor: seoLink{vendorPath, vendor.Data.Name}}
	product := map[string]any{
		"@context": "https://schema.org", "@type": "Product", "name": d.Name, "url": canonical,
		"manufacturer": map[string]string{"@type": "Organization", "name": vendor.Data.Name, "url": a.publicBase(c) + vendorPath},
	}
	if d.Description != "" {
		product["description"] = d.Description
	}
	if page.Model != "" {
		product["model"] = page.Model
	}
	if d.Brand != "" {
		product["brand"] = map[string]string{"@type": "Brand", "name": d.Brand}
	}
	images := []string{}
	for _, id := range d.MediaIDs {
		image := a.mediaAddress(c, id, "")
		images = append(images, image)
		page.Images = append(page.Images, seoImage{image, d.Name})
	}
	if len(images) > 0 {
		product["image"] = images
	}
	result, err := renderSEO(page, product)
	result.Keywords = productSEOKeywords(d, vendor.Data)
	return result, err
}

var seoLandingBody = template.Must(template.New("public-landing").Parse(`<main class="container section seo-content" data-seo-content><h1>{{.Title}}</h1>{{if not .Home}}<p>{{.Description}}</p>{{end}}<p><a href="{{.Vendors}}">企业名单与厂家展厅</a> · <a href="{{.Products}}">产品展示</a></p></main>`))

func landingSEO(title, description, hall string, home bool) (seoContent, error) {
	vendors, products := "/browse", "/browse/products"
	if hall != "agriculture" {
		vendors += "?hall=" + url.QueryEscape(hall)
		products += "?hall=" + url.QueryEscape(hall)
	}
	var body bytes.Buffer
	err := seoLandingBody.Execute(&body, struct {
		Title, Description, Vendors, Products string
		Home                                  bool
	}{title, description, vendors, products, home})
	return seoContent{Body: body.String()}, err
}
