package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type previewTransport func(*http.Request) (*http.Response, error)

func (f previewTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func previewSynthetic(code, name string) map[string]any {
	return map[string]any{"type": "Feature", "properties": map[string]any{"adcode": code, "name": name}, "geometry": map[string]any{"type": "Polygon", "coordinates": [][][]float64{{{0, 0}, {1, 0}, {1, 1}, {0, 1}, {0, 0}}}}}
}
func previewTestApp(t *testing.T) (*App, *int) {
	t.Helper()
	loadAdministrativeRegions()
	calls := 0
	files := map[string][]map[string]any{
		"100000_full": {previewSynthetic("130000", "河北省"), previewSynthetic("110000", "北京市"), previewSynthetic("500000", "重庆市"), previewSynthetic("420000", "湖北省"), previewSynthetic("710000", "台湾省"), previewSynthetic("100000_JD", "")},
		"130000_full": {previewSynthetic("130500", "邢台市")}, "130500_full": {previewSynthetic("130528", "宁晋县")}, "130528": {previewSynthetic("130528", "宁晋县")},
		"110000": {previewSynthetic("110000", "北京市")}, "110000_full": {previewSynthetic("110108", "海淀区")},
		"500000_full": {previewSynthetic("500101", "万州区"), previewSynthetic("500229", "城口县")},
		"420000_full": {previewSynthetic("429004", "仙桃市")}, "429004": {previewSynthetic("429004", "仙桃市")},
	}
	app := &App{Config: Config{Env: "development", MapPreviewEnabled: true, MapPreviewDir: t.TempDir(), WebDir: t.TempDir()}}
	app.mapPreviewClient = &http.Client{Transport: previewTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if !strings.HasPrefix(r.URL.String(), previewOrigin) {
			t.Fatalf("unexpected origin: %s", r.URL)
		}
		file := strings.TrimSuffix(strings.TrimPrefix(r.URL.String(), previewOrigin), ".json")
		features, ok := files[file]
		status := 200
		if !ok {
			status = 404
		}
		bytes, _ := json.Marshal(map[string]any{"type": "FeatureCollection", "features": features})
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(string(bytes))), Header: make(http.Header)}, nil
	})}
	return app, &calls
}
func previewRequest(a *App, code, remote, host string, headers map[string]string) *httptest.ResponseRecorder {
	engine := gin.New()
	engine.GET("/config", a.mapPreviewConfiguration)
	engine.GET("/regions", a.mapPreviewRegions)
	target := "/regions?region_code=" + code
	if code == "config" {
		target = "/config"
	}
	request := httptest.NewRequest("GET", "http://"+host+target, nil)
	request.RemoteAddr = remote
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	return response
}
func previewResult(t *testing.T, response *httptest.ResponseRecorder) previewBoundary {
	t.Helper()
	if response.Code != 200 {
		t.Fatalf("status %d: %s", response.Code, response.Body)
	}
	var value previewBoundary
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestMapPreviewIsolation(t *testing.T) {
	for _, test := range []struct {
		name, env, remote, host string
		enabled                 bool
		headers                 map[string]string
		status                  int
	}{
		{"local", "development", "127.0.0.1:3000", "localhost", true, nil, 200},
		{"ipv6", "development", "[::1]:3000", "[::1]", true, nil, 200},
		{"off", "development", "127.0.0.1:3000", "localhost", false, nil, 404},
		{"production", "production", "127.0.0.1:3000", "localhost", true, nil, 404},
		{"lan spoof", "development", "192.168.1.2:3000", "localhost", true, map[string]string{"X-Forwarded-For": "127.0.0.1"}, 404},
		{"proxy", "development", "127.0.0.1:3000", "localhost", true, map[string]string{"Forwarded": "for=198.51.100.2"}, 404},
		{"host", "development", "127.0.0.1:3000", "example.org", true, nil, 404},
	} {
		t.Run(test.name, func(t *testing.T) {
			a, calls := previewTestApp(t)
			a.Config.Env = test.env
			a.Config.MapPreviewEnabled = test.enabled
			for _, code := range []string{"config", ""} {
				response := previewRequest(a, code, test.remote, test.host, test.headers)
				if response.Code != test.status {
					t.Fatalf("status %d != %d", response.Code, test.status)
				}
			}
			if test.status == 404 && *calls != 0 {
				t.Fatal("disabled preview must not fetch resources")
			}
		})
	}
}
func TestMapPreviewExactMappingCacheAndCorruption(t *testing.T) {
	a, calls := previewTestApp(t)
	result := previewResult(t, previewRequest(a, "130528", "127.0.0.1:3000", "localhost", nil))
	if result.Usage != previewUsage || result.RegionCode != "130528" || previewCode(result.Geometry.Features[0]) != "130528" || len(result.Sources) != 4 {
		t.Fatalf("bad result: %+v", result)
	}
	if *calls != 4 {
		t.Fatalf("cold deep link fetched %d resources", *calls)
	}
	previewResult(t, previewRequest(a, "130528", "127.0.0.1:3000", "localhost", nil))
	if *calls != 4 {
		t.Fatal("cache not reused")
	}
	if err := os.WriteFile(filepath.Join(a.Config.MapPreviewDir, "130528.json"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	previewResult(t, previewRequest(a, "130528", "127.0.0.1:3000", "localhost", nil))
	if *calls != 5 {
		t.Fatal("corrupt source not refetched")
	}
	var record previewSource
	bytes, err := os.ReadFile(filepath.Join(a.Config.MapPreviewDir, "130528.metadata.json"))
	if err != nil || json.Unmarshal(bytes, &record) != nil || len(record.SHA256) != 64 || record.AcquiredAt == "" {
		t.Fatal("missing source evidence")
	}
	if response := previewRequest(a, "999999", "127.0.0.1:3000", "localhost", nil); response.Code != 400 {
		t.Fatalf("unknown region status %d", response.Code)
	}
}
func TestMapPreviewSpecialLevelsAndContext(t *testing.T) {
	a, _ := previewTestApp(t)
	for _, test := range []struct {
		code  string
		codes []string
	}{{"11", []string{"1101"}}, {"1101", []string{"110108"}}, {"50", []string{"5001", "5002"}}, {"5001", []string{"500101"}}, {"5002", []string{"500229"}}, {"42", []string{"429004"}}, {"429004", []string{"429004"}}} {
		result := previewResult(t, previewRequest(a, test.code, "127.0.0.1:3000", "localhost", nil))
		if len(result.Geometry.Features) != len(test.codes) {
			t.Fatalf("%s: wrong features: %+v", test.code, result.Geometry.Features)
		}
		for i, code := range test.codes {
			if previewCode(result.Geometry.Features[i]) != code {
				t.Fatalf("%s: wrong mapping", test.code)
			}
		}
	}
	result := previewResult(t, previewRequest(a, "", "127.0.0.1:3000", "localhost", nil))
	for _, feature := range result.Geometry.Features {
		if strings.HasPrefix(previewCode(feature), "context:") && feature.Properties["mapped"] != false {
			t.Fatal("context must not be counted")
		}
	}
	if len(result.MissingCodes) != 27 {
		t.Fatalf("missing directory nodes not reported: %d", len(result.MissingCodes))
	}
	if previewMatch(previewFeature{Properties: map[string]any{"adcode": "999999", "name": "宁晋"}}, []AdministrativeRegion{{Code: "130528", Name: "宁晋县"}}) != nil {
		t.Fatal("fuzzy names must not map")
	}
}
func TestMapPreviewFailureRetryAndPrivateCache(t *testing.T) {
	a, _ := previewTestApp(t)
	original := a.mapPreviewClient.Transport
	a.mapPreviewClient.Transport = previewTransport(func(*http.Request) (*http.Response, error) { return nil, fmt.Errorf("source unavailable") })
	if response := previewRequest(a, "", "127.0.0.1:3000", "localhost", nil); response.Code != 502 {
		t.Fatalf("bad failure status %d", response.Code)
	}
	a.mapPreviewClient.Transport = original
	previewResult(t, previewRequest(a, "", "127.0.0.1:3000", "localhost", nil))
	a.Config.MapPreviewDir = filepath.Join(a.Config.WebDir, "maps")
	if response := previewRequest(a, "", "127.0.0.1:3000", "localhost", nil); response.Code != 503 {
		t.Fatal("must reject a public cache directory")
	}
}
