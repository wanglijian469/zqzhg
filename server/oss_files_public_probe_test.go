package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

func TestCloudPublicProbeRejectsMissingCompanyScopeAndWholeBucket(t *testing.T) {
	a := importTestApp(t)
	a.cloudFiles().provider = credentials.NewStaticCredentialsProvider("fixture", "fixture", "fixture")
	var mu sync.Mutex
	objects := map[string][]byte{}
	denyCompany, allowTemp := false, false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		key := strings.TrimPrefix(r.URL.Path, "/zqzhg-public-media")
		switch r.Method {
		case "PUT":
			objects[key], _ = io.ReadAll(r.Body)
			w.WriteHeader(200)
		case "DELETE":
			delete(objects, key)
			w.WriteHeader(204)
		case "HEAD":
			if _, ok := objects[key]; !ok {
				w.WriteHeader(404)
				return
			}
			w.WriteHeader(200)
		case "GET":
			if (strings.HasPrefix(key, "/companies/") && denyCompany) || (strings.HasPrefix(key, "/upload-temp/") && !allowTemp) {
				w.WriteHeader(403)
				return
			}
			data, ok := objects[key]
			if !ok {
				w.WriteHeader(404)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", "https://zqzhg.com")
			w.Header().Set("Access-Control-Expose-Headers", "ETag, Content-Length, Content-Range, Accept-Ranges")
			w.Header().Set("Content-Length", fmt.Sprint(len(data)))
			w.Write(data)
		}
	}))
	defer server.Close()
	profile := StorageProfile{ID: 1, Kind: "oss", AuthMode: "ecs_role", RoleName: "fixture", Region: "cn-beijing", Endpoint: server.URL, Bucket: "zqzhg-public-media", Purpose: "enterprise_public", PublicBaseURL: server.URL}
	for _, test := range []struct {
		name                   string
		denyCompany, allowTemp bool
		want                   string
	}{
		{"complete scoped policy", false, false, ""},
		{"platform-only policy", true, false, "oss_company_public_unverified"},
		{"whole bucket publicly readable", false, true, "oss_public_bucket_prefix_required"},
	} {
		mu.Lock()
		denyCompany, allowTemp = test.denyCompany, test.allowTemp
		mu.Unlock()
		err := a.verifyPublicFileScopes(context.Background(), profile, "https://zqzhg.com", []byte("synthetic policy probe"), server.Client())
		if test.want == "" {
			if err != nil {
				t.Fatal(test.name, err)
			}
		} else if err == nil || err.Error() != test.want {
			t.Fatal(test.name, err)
		}
		mu.Lock()
		left := len(objects)
		mu.Unlock()
		if left != 0 {
			t.Fatal("synthetic object was not reclaimed", test.name)
		}
	}
}
