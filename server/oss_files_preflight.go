package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"os"
	"time"
)

// No database, uploads, ACL changes or public object probes. Safe to run before
// installation to identify missing role permissions without reading file data.
func cloudReadPreflight(ctx context.Context) error {
	cfg := cloudConfig()
	if e := cfg.validate(); e != nil {
		return e
	}
	a := &App{Config: Config{Env: "production"}}
	items := []map[string]any{}
	for index, bucket := range []string{cfg.PrivateBucket, cfg.PublicBucket} {
		p := StorageProfile{ID: uint64(index + 1), Kind: "oss", AuthMode: "ecs_role", RoleName: cfg.Role, Endpoint: cfg.Endpoint, Region: cfg.Region, Bucket: bucket}
		client, e := a.cloudClient(p, false)
		if e != nil {
			return e
		}
		_, e = client.ListMultipartUploads(ctx, &oss.ListMultipartUploadsRequest{Bucket: oss.Ptr(bucket), Prefix: oss.Ptr("upload-temp/system-preflight/"), MaxUploads: 1})
		item := map[string]any{"bucket": bucket, "api": "ListMultipartUploads", "resource": "acs:oss:*:*:" + bucket, "list_multipart_uploads": e == nil}
		if e != nil {
			item["error"] = cloudError(e).Error()
			var service *oss.ServiceError
			if errors.As(e, &service) {
				item["error_code"] = service.Code
				item["http_status"] = service.StatusCode
				item["request_id"] = service.RequestID
				item["ec"] = service.EC
			}
		}
		items = append(items, item)
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"checked_at": time.Now().UTC(), "read_only": true, "role": cfg.Role, "endpoint": cfg.Endpoint, "results": items, "objects_created": 0, "database_changed": false})
}
