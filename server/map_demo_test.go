package main

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestMapDemoProductionProxyAndSourceIdentity(t *testing.T) {
	a, _ := previewTestApp(t)
	a.Config.MapDemoDir = a.Config.MapPreviewDir
	// The same resolver/cache/encoding must produce identical demo geometry.
	original := previewResult(t, previewRequest(a, "130528", "127.0.0.1:1", "localhost", nil))
	a.Config.Env = "production"
	a.Config.MapDemoEnabled = true
	engine := gin.New()
	engine.GET("/demo", a.mapDemoRegions)
	request := httptest.NewRequest("GET", "https://zqzhg.com/demo?region_code=130528", nil)
	request.Header.Set("X-Forwarded-For", "198.51.100.10")
	request.Header.Set("X-Forwarded-Proto", "https")
	request.RemoteAddr = "127.0.0.1:3000"
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	value := previewResult(t, response)
	if value.SHA256 != original.SHA256 || value.Usage != mapDemoUsage || len(value.Sources) == 0 || value.Sources[0].Usage != previewUsage {
		t.Fatal("demo geometry or original provenance changed")
	}
	var fields map[string]any
	_ = json.Unmarshal(response.Body.Bytes(), &fields)
	if fields["approved"] != nil || fields["review_number"] != nil {
		t.Fatal("demo fabricated approval metadata")
	}
	if status := previewRequest(a, "config", "127.0.0.1:1", "localhost", nil).Code; status != 404 {
		t.Fatal("private development endpoint was enabled in production")
	}
	a.Config.MapDemoEnabled = false
	response = httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != 404 {
		t.Fatal("disabled demo remained public")
	}
}
