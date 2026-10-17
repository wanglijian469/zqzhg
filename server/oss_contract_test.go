package main

import (
	"bytes"
	"context"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestOSSOfficialSDKPrivateObjectAndRangeContract(t *testing.T) {
	data := []byte("%PDF-1.7 sample media object")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "OSS4-HMAC-SHA256 ") {
			t.Error("SDK did not authenticate request")
		}
		if r.URL.Path != "/test-bucket/private/sample.pdf" {
			t.Errorf("wrong object key: %s", r.URL.Path)
		}
		switch r.Method {
		case "PUT":
			if r.Header.Get("x-oss-object-acl") != "private" || r.Header.Get("x-oss-forbid-overwrite") != "true" {
				t.Error("object is not private and immutable")
			}
			value, _ := io.ReadAll(r.Body)
			if !bytes.Equal(value, data) {
				t.Error("upload differs")
			}
			w.Header().Set("ETag", "\"test-etag\"")
			w.WriteHeader(200)
		case "GET", "HEAD":
			w.Header().Set("Content-Type", "application/pdf")
			http.ServeContent(w, r, "sample.pdf", time.Unix(1000000000, 0), bytes.NewReader(data))
		case "DELETE":
			w.WriteHeader(204)
		}
	}))
	defer server.Close()
	cfg := oss.LoadDefaultConfig().WithRegion("cn-beijing").WithEndpoint(server.URL).WithDisableSSL(true).WithUsePathStyle(true).WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test-key", "test-secret")).WithRetryMaxAttempts(1)
	store := &ossObjectStore{oss.NewClient(cfg), "test-bucket", "private"}
	ctx := context.Background()
	if err := store.Put(ctx, "sample.pdf", bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatal(err)
	}
	if err := verifyObject(ctx, store, "sample.pdf", digest(string(data)), int64(len(data))); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Any("/media", func(c *gin.Context) { serveOSSObject(c, store, "sample.pdf", int64(len(data))) })
	for _, test := range []struct {
		method, rangeValue string
		status, size       int
	}{{"GET", "bytes=0-3", 206, 4}, {"HEAD", "", 200, 0}, {"GET", "bytes=900-999", 416, 0}, {"GET", "", 200, len(data)}} {
		req := httptest.NewRequest(test.method, "/media", nil)
		req.Header.Set("Range", test.rangeValue)
		out := httptest.NewRecorder()
		router.ServeHTTP(out, req)
		if out.Code != test.status || out.Body.Len() != test.size {
			t.Fatalf("%s %s: %d len=%d", test.method, test.rangeValue, out.Code, out.Body.Len())
		}
		if test.status == 206 && out.Header().Get("Content-Range") == "" {
			t.Fatal("missing Content-Range")
		}
	}
	if err := store.Delete(ctx, "sample.pdf"); err != nil {
		t.Fatal(err)
	}
}

// Run only against an administrator-provided isolated private bucket. No cloud
// credentials, bucket names or signed URLs are written to test output.
func TestRealOSSPrivateStorage(t *testing.T) {
	if os.Getenv("RUN_OSS_TESTS") != "1" {
		t.Skip("isolated real OSS credentials not supplied")
	}
	for _, key := range []string{"OSS_TEST_REGION", "OSS_TEST_BUCKET", "OSS_TEST_ACCESS_KEY_ID", "OSS_TEST_ACCESS_KEY_SECRET"} {
		if os.Getenv(key) == "" {
			t.Fatalf("missing %s", key)
		}
	}
	a := importTestApp(t)
	secret, err := a.sealSecret([]byte(`{"access_key_id":"` + os.Getenv("OSS_TEST_ACCESS_KEY_ID") + `","access_key_secret":"` + os.Getenv("OSS_TEST_ACCESS_KEY_SECRET") + `"}`))
	if err != nil {
		t.Fatal("test secret encryption failed")
	}
	profile := StorageProfile{Name: "OSS isolated acceptance", Kind: "oss", Region: os.Getenv("OSS_TEST_REGION"), Endpoint: os.Getenv("OSS_TEST_ENDPOINT"), Bucket: os.Getenv("OSS_TEST_BUCKET"), Prefix: "zqzt-acceptance/" + randomToken()[:24], Credentials: secret}
	if err := a.testStore(context.Background(), profile); err != nil {
		t.Fatal(jobFailure(err))
	}
}
