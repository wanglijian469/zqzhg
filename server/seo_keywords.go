package main

import (
	_ "embed"
	"encoding/json"
	"strings"
	"unicode"
)

// The browser imports this same copy; public search terms are not brand aliases.
//
//go:embed seo-copy.json
var seoCopyJSON []byte

type seoLanguageCopy struct {
	HomeTitle, HomeHeading, HomeIntro, DirectoryTitle, DirectoryDescription string
	ProductsTitle, ProductsDescription, Showroom                            string
	DirectoryTerms, ProductTerms                                            []string
}

var publicSEOCopy = func() struct {
	TargetTerms []string
	ZH, EN      seoLanguageCopy
} {
	var copy struct {
		TargetTerms []string
		ZH, EN      seoLanguageCopy
	}
	if err := json.Unmarshal(seoCopyJSON, &copy); err != nil {
		panic(err)
	}
	return copy
}()

func seoKeywords(values ...string) string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		for _, term := range strings.FieldsFunc(value, func(r rune) bool { return strings.ContainsRune(",，、;；|\n\r", r) }) {
			term = strings.Join(strings.FieldsFunc(term, unicode.IsSpace), " ")
			key := strings.ToLower(term)
			if term == "" || seen[key] || len([]rune(term)) > 80 {
				continue
			}
			seen[key] = true
			result = append(result, term)
			if len(result) == 24 {
				return strings.Join(result, ",")
			}
		}
	}
	return strings.Join(result, ",")
}

func vendorSEOKeywords(data Payload) string {
	values := []string{data.Name}
	if data.CompanyProfile != nil {
		values = append(values, data.CompanyProfile.ShortName)
	}
	values = append(values, data.MainProducts, data.RegionName)
	values = append(values, publicSEOCopy.ZH.DirectoryTerms...)
	values = append(values, data.Keywords)
	return seoKeywords(values...)
}

func productSEOKeywords(data, vendor Payload) string {
	values := []string{data.Name, seoProductModel(data), vendor.Name, data.Brand, data.RegionName}
	values = append(values, publicSEOCopy.ZH.ProductTerms...)
	values = append(values, data.Keywords)
	return seoKeywords(values...)
}

func showroomSEOTitle(name, tab, brand string) string {
	labels := map[string]string{"about": "企业介绍", "products": "产品中心", "videos": "视频展示", "catalogs": "电子画册", "contact": "联系企业"}
	title := name + " · " + publicSEOCopy.ZH.Showroom
	if label := labels[tab]; label != "" {
		title += " · " + label
	}
	return title + " | " + brand
}
