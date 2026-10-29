package main

import (
	"bytes"
	"context"
	"image"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

func TestDisplayImagePrivateOSSAndIntegrity(t *testing.T) {
	data := imageTestPNG(t, true)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "OSS4-HMAC-SHA256 ") {
			t.Error("unsigned private object request")
		}
		if r.URL.Path != "/test-bucket/private/marked.png" {
			t.Error("wrong source object", r.URL.Path)
		}
		requests.Add(1)
		w.Header().Set("Content-Type", "image/png")
		w.Write(data)
	}))
	defer server.Close()
	cfg := oss.LoadDefaultConfig().WithRegion("cn-beijing").WithEndpoint(server.URL).WithDisableSSL(true).WithUsePathStyle(true).WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test-key", "test-secret")).WithRetryMaxAttempts(1)
	store := &ossObjectStore{oss.NewClient(cfg), "test-bucket", "private"}
	cache := newTestImageCache(t)
	generate := func(ctx context.Context) ([]byte, error) {
		return loadDisplayImage(ctx, store, "marked.png", imageDigest(data), 240)
	}
	first, err := cache.variant(context.Background(), "oss", generate)
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.variant(context.Background(), "oss", generate)
	if err != nil || !bytes.Equal(first, second) || requests.Load() != 1 {
		t.Fatal("OSS cache miss", err, requests.Load())
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(first))
	if err != nil || config.Width != 240 || format != "png" {
		t.Fatal(config, format, err)
	}
	_, err = loadDisplayImage(context.Background(), store, "marked.png", strings.Repeat("0", 64), 240)
	if err == nil {
		t.Fatal("changed source hash accepted")
	}
}
