package main

import (
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

var hashedWebAsset = regexp.MustCompile(`-[A-Za-z0-9_-]{8,}\.(js|css)$`)

func appendVary(c *gin.Context, value string) {
	for _, part := range strings.Split(c.Writer.Header().Get("Vary"), ",") {
		if strings.EqualFold(strings.TrimSpace(part), value) {
			return
		}
	}
	c.Writer.Header().Add("Vary", value)
}

func acceptsGzip(header string) bool {
	allowed, wildcard := -1.0, -1.0
	for _, item := range strings.Split(header, ",") {
		parts := strings.Split(item, ";")
		name := strings.TrimSpace(parts[0])
		q := 1.0
		for _, param := range parts[1:] {
			pair := strings.SplitN(strings.TrimSpace(param), "=", 2)
			if len(pair) == 2 && pair[0] == "q" {
				var err error
				q, err = strconv.ParseFloat(pair[1], 64)
				if err != nil {
					q = 0
				}
			}
		}
		if name == "gzip" {
			allowed = q
		}
		if name == "*" {
			wildcard = q
		}
	}
	return allowed > 0 || allowed < 0 && wildcard > 0
}

func (a *App) serveWebAsset(c *gin.Context, path string) {
	c.Header("Cache-Control", "no-cache")
	if filepath.Base(filepath.Dir(path)) == "assets" && hashedWebAsset.MatchString(filepath.Base(path)) {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".js" || ext == ".css" || ext == ".svg" {
		appendVary(c, "Accept-Encoding")
		if c.GetHeader("Range") == "" && acceptsGzip(c.GetHeader("Accept-Encoding")) {
			if info, err := os.Stat(path + ".gz"); err == nil && !info.IsDir() {
				c.Header("Content-Encoding", "gzip")
				if contentType := mime.TypeByExtension(ext); contentType != "" {
					c.Header("Content-Type", contentType)
				}
				c.File(path + ".gz")
				return
			}
		}
	}
	c.File(path)
}
