package bailian

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestStreamingReplyAndUnknownUsage(t *testing.T) {
	wire := "data: {\"id\":\"s1\",\"choices\":[{\"delta\":{\"content\":\"{\\\"ok\\\":\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"true}\"},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: {\"choices\":[],\"usage\":{\"total_tokens\":9}}\n\ndata: [DONE]\n"
	r, e := readReply(strings.NewReader(wire), true)
	if e != nil || r.Choices[0].Message.Content != "{\"ok\":true}" || r.Usage == nil {
		t.Fatal(e, r)
	}
	b, _ := json.Marshal(r.Usage)
	if string(b) != "{\"total_tokens\":9}" {
		t.Fatal("fabricated missing usage", string(b))
	}
	if _, e = readReply(strings.NewReader("data: {\"choices\":[{\"delta\":{\"content\":\"{}\"}}]}\n"), true); e == nil {
		t.Fatal("incomplete stream accepted")
	}
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestClientUsageAndRedaction(t *testing.T) {
	cfg := Config{Key: "private-test-key", BaseURL: "https://example.test/compatible-mode/v1", Model: "test", Timeout: time.Second, Retries: 0}
	c := Client{Config: cfg, HTTP: &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer private-test-key" {
			t.Fatal("authorization")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"id":"r1","choices":[{"finish_reason":"stop","message":{"content":"{\"ok\":true}"}}],"usage":{"prompt_tokens":12,"completion_tokens":3,"total_tokens":15,"x_tools":{"pdf_page_parser":{"count":2}}}}`))}, nil
	})}}
	count := 0
	r, e := c.Complete(context.Background(), "test", "extract", []Part{{Type: "text", Text: "fixture"}}, func(a Attempt) error {
		count++
		if a.Usage == nil || a.Usage.Tools.PDF.Count != 2 {
			t.Fatal("missing usage")
		}
		return nil
	})
	if e != nil || count != 1 || r.Attempt.RequestID != "r1" {
		t.Fatal(e, r)
	}
	c.HTTP.Transport = transportFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 401, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("private-test-key"))}, nil
	})
	_, e = c.Complete(context.Background(), "test", "", nil, nil)
	if e == nil || strings.Contains(e.Error(), cfg.Key) {
		t.Fatal("unsafe error")
	}
}
func TestInvalidJSONAndCodingKey(t *testing.T) {
	c := Client{Config: Config{Key: "sk-sp-invalid", BaseURL: "https://example.test/compatible-mode/v1", Timeout: time.Second}}
	if _, e := c.Complete(context.Background(), "test", "", nil, nil); e == nil {
		t.Fatal("coding key accepted")
	}
	c.Config.Key = "test"
	c.HTTP = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"length","message":{"content":"{}"}}]}`))}, nil
	})}
	if _, e := c.Complete(context.Background(), "test", "", nil, nil); e == nil {
		t.Fatal("truncated result accepted")
	}
}

func TestTemporaryRetryRecordsEachAttempt(t *testing.T) {
	n := 0
	c := Client{Config: Config{Key: "private-test-key", BaseURL: "https://example.test/compatible-mode/v1", Timeout: time.Second, Retries: 1}, HTTP: &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		n++
		if n == 1 {
			return &http.Response{StatusCode: 429, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("private-test-key"))}, nil
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"stop","message":{"content":"{}"}}]}`))}, nil
	})}}
	attempts := []Attempt{}
	_, e := c.Complete(context.Background(), "fixture", "", nil, func(a Attempt) error { attempts = append(attempts, a); return nil })
	if e != nil || len(attempts) != 2 || attempts[0].Error != "bailian_temporary_failure" || attempts[0].Usage != nil || attempts[1].Usage != nil {
		t.Fatal("retry or unknown usage not recorded", e, attempts)
	}
}
