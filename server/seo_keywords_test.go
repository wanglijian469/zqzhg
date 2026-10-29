package main

import (
	"encoding/xml"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSEOKeywordNormalization(t *testing.T) {
	if got := seoKeywords(" 企业展厅，企业名单; 企业展厅 | Tractor\ntractor", "  河北   农机  ", strings.Repeat("长", 81)); got != "企业展厅,企业名单,Tractor,河北 农机" {
		t.Fatal(got)
	}
	if got := seoKeywords(strings.Repeat("a,", 30)); got != "a" {
		t.Fatal(got)
	}
	terms := []string{}
	for i := 0; i < 30; i++ {
		terms = append(terms, string(rune('一'+i)))
	}
	if len(strings.Split(seoKeywords(terms...), ",")) != 24 {
		t.Fatal("keyword count is unbounded")
	}
	vendor := Payload{Name: "河北冀农农机具有限公司", MainProducts: "农机具", RegionName: "河北省", Keywords: "农机具,耕地机械", CompanyProfile: &CompanyProfile{ShortName: "冀农农机"}}
	for _, word := range []string{vendor.Name, "冀农农机", "企业展厅", "河北省", "耕地机械"} {
		if !strings.Contains(vendorSEOKeywords(vendor), word) {
			t.Fatal("missing company keyword", word)
		}
	}
	if strings.Contains(vendorSEOKeywords(vendor), "内卷网") {
		t.Fatal("unrelated platform search terms added to a company")
	}
	if title := showroomSEOTitle(vendor.Name, "about", "中企展馆"); title != vendor.Name+" · 企业展厅 · 企业介绍 | 中企展馆" {
		t.Fatal(title)
	}
}

func TestSEOLandingSearchTermsAndStaticSitemap(t *testing.T) {
	a := importTestApp(t)
	seoTestIndex(t, a)
	a.Config.PublicBaseURL = "https://seo.example"
	router := a.router()
	for _, path := range []string{"/?hall=agriculture", "/browse", "/browse/products"} {
		w := seoGet(router, path)
		raw := w.Body.String()
		if w.Code != 200 || !strings.Contains(raw, "data-seo-content") || strings.Contains(raw, "stale-") {
			t.Fatal(path, w.Code, raw)
		}
		if path != "/browse/products" {
			for _, word := range append(publicSEOCopy.TargetTerms, "企业名单", "企业展厅") {
				if !strings.Contains(raw, word) {
					t.Fatal("missing landing term", path, word)
				}
			}
		}
		if strings.Contains(raw, "alternateName") {
			t.Fatal("search terms represented as brand aliases")
		}
	}
	for _, path := range []string{"/?hall=cable", "/browse?hall=missing", "/products/999999999"} {
		w := seoGet(router, path)
		if !strings.Contains(w.Body.String(), "noindex,follow") || strings.Contains(w.Body.String(), "name='keywords'") {
			t.Fatal("unavailable page promoted", path)
		}
	}
	w := seoGet(router, "/sitemaps/pages.xml")
	var urls sitemapURLs
	if w.Code != 200 || xml.Unmarshal(w.Body.Bytes(), &urls) != nil || len(urls.Entries) != 8 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, entry := range urls.Entries {
		if strings.Contains(entry.Loc, "cable") || strings.Contains(entry.Loc, "merchant") || !strings.HasPrefix(entry.Loc, a.Config.PublicBaseURL) {
			t.Fatal("incorrect public sitemap page", entry.Loc)
		}
	}
	for _, ua := range []string{"Baiduspider/2.0", "bingbot/2.0", "Googlebot/2.1"} {
		for _, path := range []string{"/", "/robots.txt", "/sitemap.xml", "/sitemaps/pages.xml"} {
			req := httptest.NewRequest("GET", path, nil)
			req.Header.Set("User-Agent", ua)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != 200 {
				t.Fatal("search crawler cannot access public entry", ua, path, response.Code)
			}
		}
	}
}
