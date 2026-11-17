// Package bailian implements the server-only, pay-as-you-go Model Studio client.
package bailian

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Key, BaseURL, Model, ReviewModel string
	Timeout                          time.Duration
	Retries                          int
}

func FromEnv() Config {
	seconds, _ := strconv.Atoi(os.Getenv("DASHSCOPE_TIMEOUT_SECONDS"))
	if seconds < 1 {
		seconds = 360
	}
	return Config{os.Getenv("DASHSCOPE_API_KEY"), strings.TrimRight(os.Getenv("DASHSCOPE_BASE_URL"), "/"), fallback(os.Getenv("DASHSCOPE_MODEL"), "qwen3.8-flash"), fallback(os.Getenv("DASHSCOPE_REVIEW_MODEL"), "qwen3.8-flash"), time.Duration(seconds) * time.Second, 2}
}
func fallback(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
func (c Config) Validate() error {
	u, e := url.Parse(c.BaseURL)
	if c.Key == "" || strings.ContainsAny(c.Key, " \t\r\n") || strings.HasPrefix(c.Key, "sk-sp-") || e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !strings.HasSuffix(u.Path, "/compatible-mode/v1") {
		return errors.New("bailian_configuration_required")
	}
	return nil
}

type Client struct {
	Config Config
	HTTP   *http.Client
}
type Part struct {
	Type     string            `json:"type"`
	Text     string            `json:"text,omitempty"`
	File     map[string]string `json:"file,omitempty"`
	ImageURL map[string]string `json:"image_url,omitempty"`
}

func PDF(name string, data []byte) Part {
	return Part{Type: "file", File: map[string]string{"filename": name, "file_data": "data:application/pdf;base64," + base64.StdEncoding.EncodeToString(data)}}
}
func Image(mime string, data []byte) Part {
	return Part{Type: "image_url", ImageURL: map[string]string{"url": "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)}}
}

type Usage struct {
	raw              json.RawMessage
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
	PromptDetails    struct {
		CachedTokens int64 `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	Tools struct {
		PDF struct {
			Count int64 `json:"count"`
		} `json:"pdf_page_parser"`
	} `json:"x_tools"`
}

// Preserve missing provider fields as missing, rather than synthesizing zero usage.
func (u *Usage) UnmarshalJSON(b []byte) error {
	type plain Usage
	var p plain
	if e := json.Unmarshal(b, &p); e != nil {
		return e
	}
	*u = Usage(p)
	u.raw = append([]byte(nil), b...)
	return nil
}
func (u Usage) MarshalJSON() ([]byte, error) {
	if u.raw != nil {
		return u.raw, nil
	}
	type plain Usage
	return json.Marshal(plain(u))
}

type Attempt struct {
	RequestID  string `json:"request_id"`
	Model      string `json:"model"`
	Number     int    `json:"attempt"`
	Status     int    `json:"http_status"`
	Error      string `json:"error,omitempty"`
	Usage      *Usage `json:"usage"`
	DurationMS int64  `json:"duration_ms"`
}
type Result struct {
	Text    string
	Attempt Attempt
}

type wireReply struct {
	ID      string `json:"id"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content string `json:"content"`
		} `json:"message"`
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *Usage `json:"usage"`
}

func readReply(r io.Reader, streaming bool) (wireReply, error) {
	var reply wireReply
	if !streaming {
		data, e := io.ReadAll(io.LimitReader(r, 8<<20))
		if e != nil {
			return reply, e
		}
		e = json.Unmarshal(data, &reply)
		return reply, e
	}
	scanner := bufio.NewScanner(io.LimitReader(r, (8<<20)+1))
	scanner.Buffer(make([]byte, 4096), 2<<20)
	var output strings.Builder
	finished := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk wireReply
		if json.Unmarshal([]byte(data), &chunk) != nil {
			return reply, errors.New("bailian_invalid_stream")
		}
		if chunk.ID != "" {
			reply.ID = chunk.ID
		}
		if chunk.Usage != nil {
			reply.Usage = chunk.Usage
		}
		for _, choice := range chunk.Choices {
			output.WriteString(choice.Delta.Content)
			if choice.FinishReason != "" {
				finished = true
				reply.Choices = chunk.Choices
			}
		}
	}
	if e := scanner.Err(); e != nil {
		return reply, e
	}
	if !finished || len(reply.Choices) == 0 {
		return reply, errors.New("bailian_incomplete_stream")
	}
	reply.Choices[0].Message.Content = output.String()
	return reply, nil
}

// Errors deliberately exclude response bodies, authorization headers and transport URLs.
func (c *Client) Complete(ctx context.Context, model, system string, parts []Part, record func(Attempt) error) (Result, error) {
	if e := c.Config.Validate(); e != nil {
		return Result{}, e
	}
	parts = append(parts, Part{Type: "text", Text: "Return a single valid JSON object. Do not use markdown fences."})
	body, e := json.Marshal(map[string]any{"model": model, "messages": []any{map[string]any{"role": "system", "content": system}, map[string]any{"role": "user", "content": parts}}, "response_format": map[string]string{"type": "json_object"}, "max_tokens": 16000, "include_tool_usage": true, "enable_thinking": false, "stream": true, "stream_options": map[string]bool{"include_usage": true}})
	if e != nil {
		return Result{}, errors.New("bailian_invalid_request")
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: c.Config.Timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	for n := 0; n <= c.Config.Retries; n++ {
		start := time.Now()
		at := Attempt{Model: model, Number: n + 1}
		callCtx, cancel := context.WithTimeout(ctx, c.Config.Timeout)
		req, _ := http.NewRequestWithContext(callCtx, "POST", c.Config.BaseURL+"/chat/completions", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+c.Config.Key)
		req.Header.Set("Content-Type", "application/json")
		res, err := client.Do(req)
		retry := false
		delay := time.Duration(1<<n) * time.Second
		var reply wireReply
		if err != nil {
			at.Error = "bailian_transport_unknown"
			retry = true
		} else {
			at.Status = res.StatusCode
			at.RequestID = res.Header.Get("X-Request-Id")
			var readErr error
			if res.StatusCode == 200 {
				reply, readErr = readReply(res.Body, strings.Contains(res.Header.Get("Content-Type"), "text/event-stream"))
			} else {
				_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 16384))
			}
			res.Body.Close()
			if res.StatusCode == 429 || res.StatusCode >= 500 {
				retry = true
				at.Error = "bailian_temporary_failure"
				if seconds, e := strconv.Atoi(res.Header.Get("Retry-After")); e == nil && seconds > 0 && seconds <= 60 {
					delay = time.Duration(seconds) * time.Second
				}
			} else if res.StatusCode != 200 {
				at.Error = fmt.Sprintf("bailian_http_%d", res.StatusCode)
			} else if readErr != nil || len(reply.Choices) == 0 {
				at.Error = "bailian_invalid_response"
				if readErr != nil && !strings.HasPrefix(readErr.Error(), "bailian_") && !strings.Contains(readErr.Error(), "invalid character") {
					at.Error = "bailian_transport_unknown"
					retry = true
				}
			} else {
				at.Usage = reply.Usage
				if at.RequestID == "" {
					at.RequestID = reply.ID
				}
				if reply.Choices[0].FinishReason == "length" || !json.Valid([]byte(reply.Choices[0].Message.Content)) {
					at.Error = "bailian_invalid_json"
				}
			}
		}
		cancel()
		at.DurationMS = time.Since(start).Milliseconds()
		if record != nil {
			if e := record(at); e != nil {
				return Result{}, errors.New("bailian_usage_record_failed")
			}
		}
		if at.Error == "" {
			return Result{reply.Choices[0].Message.Content, at}, nil
		}
		if !retry || n == c.Config.Retries {
			return Result{}, errors.New(at.Error)
		}
		t := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			t.Stop()
			return Result{}, ctx.Err()
		case <-t.C:
		}
	}
	return Result{}, errors.New("bailian_failed")
}
