package main

import (
	"regexp"
	"strings"
)

// Keep in sync with React routes. A private shell is still served before login,
// while resource access remains governed by the existing API authorization.
var privateSPARoute = regexp.MustCompile(`^/(?:login|register|account|join|procurement(?:/[1-9][0-9]*)?|buyer(?:/(?:profile|notifications|new|projects/[1-9][0-9]*(?:/edit)?|opportunities(?:/(?:settings|notifications|(?:projects|join)/[1-9][0-9]*))?))?|merchant(?:/(?:verification|login|booth(?:/edit)?|products(?:/(?:new|representatives|[1-9][0-9]*/edit))?|promotion|settings|opportunities(?:/(?:settings|notifications|(?:projects|join)/[1-9][0-9]*))?))?|admin(?:/(?:verifications|service-requests|service-documents|home-rotation|login|dashboard|vendors|products|vendor-reviews|product-reviews|contents/[1-9][0-9]*|videos|catalogs|categories|regions|featured|accounts|logs|system|settings|analytics|operators|storage|backups|procurement(?:/(?:reviews|stats|projects/[1-9][0-9]*))?|extensions/(?:analytics|roles|backups|capture|banners|articles)))?)/?$`)

func privateSPAPath(path string) bool {
	return path == "/admin/seo" || path == "/admin/seo/" || privateSPARoute.MatchString(path)
}

var publicSPARoute = regexp.MustCompile(`^/(?:services(?:/(?:service-info|showroom-agreement|terms|privacy))?|service-requests|browse(?:/products)?|regions|app|products/[1-9][0-9]*|vendors/[1-9][0-9]*|v/[a-z0-9-]+)$`)

func normalizedSPAPath(path string) string {
	trimmed := strings.TrimSuffix(path, "/")
	if trimmed != path && (privateSPAPath(trimmed) || publicSPARoute.MatchString(trimmed)) {
		return trimmed
	}
	return path
}
