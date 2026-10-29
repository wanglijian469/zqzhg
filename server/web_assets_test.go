package main

import (
	"bytes"
	"compress/gzip"
	"github.com/gin-gonic/gin"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompressedWebAssetsAndCaching(t *testing.T) {
	directory := t.TempDir()
	os.MkdirAll(filepath.Join(directory, "assets"), 0700)
	source := bytes.Repeat([]byte("const example = true;\n"), 100)
	file := filepath.Join(directory, "assets", "index-Ab123456.js")
	os.WriteFile(file, source, 0600)
	var zipped bytes.Buffer
	writer := gzip.NewWriter(&zipped)
	writer.Write(source)
	writer.Close()
	os.WriteFile(file+".gz", zipped.Bytes(), 0600)
	a := &App{}
	r := gin.New()
	r.GET("/asset", func(c *gin.Context) { c.Header("Vary", "Origin"); a.serveWebAsset(c, file) })
	for _, encoding := range []string{"gzip", "br, gzip;q=0.7", "gzip;q=0", "identity", "*;q=0.5, gzip;q=0"} {
		request := httptest.NewRequest("GET", "/asset", nil)
		request.Header.Set("Accept-Encoding", encoding)
		out := httptest.NewRecorder()
		r.ServeHTTP(out, request)
		if out.Code != 200 || out.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
			t.Fatal(out.Code, out.Header())
		}
		if !strings.Contains(strings.Join(out.Header().Values("Vary"), ","), "Accept-Encoding") || !strings.Contains(strings.Join(out.Header().Values("Vary"), ","), "Origin") {
			t.Fatal("vary overwritten")
		}
		result := out.Body.Bytes()
		if acceptsGzip(encoding) {
			if out.Header().Get("Content-Encoding") != "gzip" {
				t.Fatal("not compressed")
			}
			reader, err := gzip.NewReader(bytes.NewReader(result))
			if err != nil {
				t.Fatal(err)
			}
			result, _ = io.ReadAll(reader)
			reader.Close()
		}
		if !bytes.Equal(source, result) {
			t.Fatal("asset bytes differ")
		}
	}
	request := httptest.NewRequest("GET", "/asset", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	request.Header.Set("Range", "bytes=0-4")
	out := httptest.NewRecorder()
	r.ServeHTTP(out, request)
	if out.Code != 206 || out.Header().Get("Content-Encoding") != "" || out.Body.String() != string(source[:5]) {
		t.Fatal("range changed")
	}
}
