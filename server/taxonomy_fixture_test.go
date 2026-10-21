package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
)

var taxonomyFixtureDefaults sync.Map

// fixtureCall supplies an explicit valid classification to older feature fixtures.
// Tests of optional drafts, historical records and classification validation use
// call directly, so this helper never changes the API behavior under test.
func (tc testClient) fixtureCall(method, path, token string, body any, want int) map[string]any {
	encoded, _ := json.Marshal(body)
	var request map[string]any
	if json.Unmarshal(encoded, &request) != nil {
		return tc.call(method, path, token, body, want)
	}
	data, ok := request["data"].(map[string]any)
	if !ok {
		return tc.call(method, path, token, body, want)
	}
	kind, _ := request["kind"].(string)
	if kind == "" && strings.HasSuffix(path, "/draft") {
		req := httptest.NewRequest("GET", "/api/v1"+strings.TrimSuffix(path, "/draft"), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		out := httptest.NewRecorder()
		tc.router.ServeHTTP(out, req)
		var item map[string]any
		_ = json.Unmarshal(out.Body.Bytes(), &item)
		if out.Code != 200 {
			return tc.call(method, path, token, body, want)
		}
		kind, _ = item["kind"].(string)
	} else if kind == "" {
		kind = "vendor"
		if strings.HasSuffix(path, "/products") {
			kind = "product"
		}
	}
	if hall, ok := request["hall_id"].(float64); ok && hall != 0 && hall != 1 {
		return tc.call(method, path, token, body, want)
	}
	fields := map[string]string{"vendor": "primary_category_id", "product": "product_category_id"}
	field := fields[kind]
	if field == "" {
		return tc.call(method, path, token, body, want)
	}
	if value, ok := data[field].(float64); ok && value > 0 {
		return tc.call(method, path, token, body, want)
	}
	cached, loaded := taxonomyFixtureDefaults.Load(tc.router)
	if !loaded {
		ids := map[string]float64{}
		for _, purpose := range []string{"vendor", "product"} {
			tax := tc.call("GET", "/public/taxonomy?category_kind="+purpose, "", nil, 200)
			cats := tax["categories"].([]any)
			for _, raw := range cats {
				cat := raw.(map[string]any)
				leaf := true
				if purpose == "product" {
					for _, child := range cats {
						if child.(map[string]any)["parent_id"] == cat["id"] {
							leaf = false
							break
						}
					}
				}
				if leaf {
					ids[purpose] = cat["id"].(float64)
					break
				}
			}
		}
		taxonomyFixtureDefaults.Store(tc.router, ids)
		tc.t.Cleanup(func() { taxonomyFixtureDefaults.Delete(tc.router) })
		cached = ids
	}
	data[field] = cached.(map[string]float64)[kind]
	return tc.call(method, path, token, request, want)
}
