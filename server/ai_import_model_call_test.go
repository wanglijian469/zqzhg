package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type aiFixtureTransport func(*http.Request) (*http.Response, error)

func (f aiFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAIEmptyModelResponseIsRepairedAndAudited(t *testing.T) {
	t.Setenv("DASHSCOPE_API_KEY", "synthetic-repair-test")
	t.Setenv("DASHSCOPE_BASE_URL", "https://model.invalid/compatible-mode/v1")
	a := importTestApp(t)
	var owner Account
	a.DB.Where("role='admin'").First(&owner)
	task := aiTestTask(t, a, owner.ID)
	task.Status = "processing"
	task.Lease = "fixture-lease"
	a.DB.Save(&task)
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	requests := 0
	http.DefaultTransport = aiFixtureTransport(func(*http.Request) (*http.Response, error) {
		requests++
		content := `{}`
		if requests == 2 {
			content = `{"product_name":"样本","source_pages":[1],"models":[],"images":[],"review_issues":[]}`
		}
		if requests >= 3 {
			content = `{"product_name":"样本","source_pages":[1],"models":[],"images":[{"source_page":1,"image_type":"product_main","crop_region":[0,0,1,1]}],"review_issues":[]}`
		}
		wire, _ := json.Marshal(map[string]any{"id": "synthetic", "choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": content}}}})
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(string(wire)))}, nil
	})
	var out AIProductExtraction
	if e := a.aiModel(context.Background(), task, "product-empty", "fixture", "fixture extraction", nil, &out); e != nil {
		t.Fatal(e)
	}
	if requests != 2 || out.ProductName != "样本" {
		t.Fatal("empty response was not repaired", requests, out)
	}
	if e := a.aiModel(context.Background(), task, "product-empty", "fixture", "fixture extraction", nil, &out); e != nil {
		t.Fatal(e)
	}
	if requests != 2 {
		t.Fatal("successful checkpoint was called again")
	}
	var bad int64
	a.DB.Model(&AIImportModelCall{}).Where("task_id=? AND error='ai_invalid_schema'", task.ID).Count(&bad)
	if bad != 1 {
		t.Fatal("original empty answer not retained in audit")
	}
	page, e := a.aiProductPage(context.Background(), task, "unsafe-image", "fixture", "fixture extraction", nil, []int{1})
	if e != nil || len(page.Images) != 0 || len(page.ReviewIssues) == 0 || requests != 4 {
		t.Fatal("bad photo blocked all products or was silently accepted", e, page, requests)
	}
}
