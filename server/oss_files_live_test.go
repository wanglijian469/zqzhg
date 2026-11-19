package main

// Operator-invoked acceptance only. Creates a new, generated test database and
// synthetic cloud objects. Never clones, migrates or deletes production media.
import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/gin-gonic/gin"
)

func TestCloudLiveSyntheticAcceptance(t *testing.T) {
	if os.Getenv("RUN_REAL_OSS_ACCEPTANCE") != "1" || os.Getenv("CONFIRM_SYNTHETIC_CLOUD_OBJECTS") != "1" {
		t.Skip("requires explicit operator authorization and ECS role")
	}
	if cloudConfig().validate() != nil || cloudConfig().Role != "ZQZHG-SMS-Role" {
		t.Fatal("unexpected cloud target")
	}
	a := importTestApp(t)
	database := a.currentDatabase()
	if !strings.HasPrefix(database, "zqzt_exhibition_test_") {
		t.Fatal("not an isolated database")
	}
	a.Config.Env = "production"
	t.Setenv("OSS_ENABLED", "true")
	t.Setenv("OSS_UPLOADS_ENABLED", "true")
	t.Setenv("OSS_DIRECT_UPLOAD_ENABLED", "true")
	t.Setenv("OSS_TEST_ASSUME_READY", "false")
	t.Setenv("OSS_UPLOAD_TIMEOUT", "30")
	t.Setenv("OPERATIONS_DIR", t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	report := map[string]any{"isolated_database": database, "production_database_changed": false, "existing_production_files_changed": false, "role": cloudConfig().Role, "region": "cn-beijing", "checks": []string{}, "passed": false}
	checks := []string{}
	mark := func(name string) { checks = append(checks, name); report["checks"] = checks; t.Log("verified", name) }
	defer func() {
		if target := os.Getenv("OSS_ACCEPTANCE_REPORT"); target != "" {
			raw, _ := json.MarshalIndent(report, "", "  ")
			if e := os.WriteFile(target, raw, 0600); e != nil {
				t.Error("cannot write acceptance report")
			}
		}
	}()
	fatalCloud := func(api, bucket, key string, e error) {
		d := cloudLiveFailure(api, bucket, key, e)
		report["failure"] = d
		raw, _ := json.Marshal(d)
		t.Fatal(string(raw))
	}
	if e := a.initializeCloudFiles(); e != nil {
		t.Fatal("isolated configuration failed")
	}
	cfg, e := a.systemConfig()
	if e != nil || cfg.Uploads.DefaultQuotaBytes != 314572800 {
		t.Fatal("default quota must be 300 MiB")
	}
	cfg.Site.WebURL = "https://zqzhg.com"
	cfg.Site.APIURL = "https://zqzhg.com/api/v1"
	cfg.Site.MediaURL = "https://zqzhg.com/api/v1/media"
	raw, _ := json.Marshal(cfg)
	if e = a.DB.Model(&SystemSetting{}).Where("setting_key='system_config_v3'").Update("value", string(raw)).Error; e != nil {
		t.Fatal("isolated site configuration failed")
	}
	mark("default_quota_314572800_bytes")
	private, _ := a.cloudProfile("private")
	public, _ := a.cloudProfile("public")
	// Every cleanup target comes from this freshly created test database or the
	// explicit random probe key below. No bucket-wide cleanup or local media walk.
	t.Cleanup(func() {
		cleanupOK := true
		cleanupError := func(message string) { cleanupOK = false; t.Error(message) }
		var sessions []FileUploadSession
		if a.DB.Find(&sessions).Error != nil {
			cleanupError("cleanup inventory unavailable")
			return
		}
		latest := time.Now()
		for _, s := range sessions {
			if s.SignatureExpiresAt.Add(31 * time.Second).After(latest) {
				latest = s.SignatureExpiresAt.Add(31 * time.Second)
			}
		}
		for time.Now().Before(latest) {
			time.Sleep(time.Second)
		}
		cleanup, stop := context.WithTimeout(context.Background(), 90*time.Second)
		defer stop()
		targets := map[string]uint64{}
		for _, s := range sessions {
			if s.ObjectKey != "" {
				targets[s.ObjectKey] = s.StoreID
			}
			if s.MultipartID != "" {
				p, _ := a.storageProfile(s.StoreID)
				client, e := a.cloudClient(p, false)
				if e != nil {
					cleanupError("cleanup client failed")
					continue
				}
				_, e = client.AbortMultipartUpload(cleanup, &oss.AbortMultipartUploadRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(s.ObjectKey), UploadId: oss.Ptr(s.MultipartID)})
				var se *oss.ServiceError
				if e != nil && (!errors.As(e, &se) || se.StatusCode != 404) {
					report["cleanup_abort_error"] = cloudLiveFailure("AbortMultipartUpload", p.Bucket, s.ObjectKey, e)
					cleanupError("synthetic multipart cleanup failed")
				}
			}
		}
		var objects []StoredObject
		var copies []EnterpriseFileObject
		var copyItems []FileStorageJobItem
		if a.DB.Find(&objects).Error != nil || a.DB.Find(&copies).Error != nil || a.DB.Find(&copyItems).Error != nil {
			cleanupError("cleanup inventory unavailable")
			return
		}
		for _, o := range objects {
			key := o.RemoteKey
			if key == "" {
				key = o.Key
			}
			if o.StoreID > 0 {
				targets[key] = o.StoreID
			}
		}
		for _, o := range copies {
			if o.StoreID > 0 {
				key := o.PhysicalKey
				if key == "" {
					key = o.ObjectKey
				}
				targets[key] = o.StoreID
			}
		}
		for _, item := range copyItems {
			if item.TargetStoreID > 0 && item.TargetKey != "" {
				targets[item.TargetKey] = item.TargetStoreID
			}
		}
		for key, storeID := range targets {
			p, e := a.storageProfile(storeID)
			if e != nil || (p.Bucket != private.Bucket && p.Bucket != public.Bucket) || !safeObjectKey(key) {
				cleanupError("unsafe synthetic cleanup target")
				continue
			}
			client, e := a.cloudClient(p, false)
			if e != nil {
				cleanupError("cleanup client failed")
				continue
			}
			if _, e = client.DeleteObject(cleanup, &oss.DeleteObjectRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(key)}); e != nil {
				report["cleanup_error"] = cloudLiveFailure("DeleteObject", p.Bucket, key, e)
				cleanupError("synthetic object cleanup failed")
				continue
			}
			_, e = client.HeadObject(cleanup, &oss.HeadObjectRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(key)})
			var se *oss.ServiceError
			if !errors.As(e, &se) || se.StatusCode != 404 {
				cleanupError("synthetic deletion not confirmed")
			}
		}
		report["synthetic_cleanup_completed"] = cleanupOK
		if !cleanupOK {
			report["passed"] = false
		}
		if target := os.Getenv("OSS_ACCEPTANCE_REPORT"); target != "" {
			raw, _ := json.MarshalIndent(report, "", "  ")
			os.WriteFile(target, raw, 0600)
		}
	})
	// Independently exercise all three newly granted actions on both buckets.
	for _, p := range []StorageProfile{private, public} {
		client, e := a.cloudClient(p, false)
		if e != nil {
			t.Fatal("ECS role unavailable")
		}
		key := "upload-temp/system-preflight/" + randomToken() + "-multipart.bin"
		_, e = client.ListMultipartUploads(ctx, &oss.ListMultipartUploadsRequest{Bucket: oss.Ptr(p.Bucket), Prefix: oss.Ptr(key), MaxUploads: 1})
		if e != nil {
			fatalCloud("ListMultipartUploads", p.Bucket, key, e)
		}
		mp, e := client.InitiateMultipartUpload(ctx, &oss.InitiateMultipartUploadRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(key)})
		if e != nil {
			fatalCloud("InitiateMultipartUpload", p.Bucket, key, e)
		}
		_, e = client.ListParts(ctx, &oss.ListPartsRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(key), UploadId: mp.UploadId})
		if e != nil {
			client.AbortMultipartUpload(ctx, &oss.AbortMultipartUploadRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(key), UploadId: mp.UploadId})
			fatalCloud("ListParts", p.Bucket, key, e)
		}
		_, e = client.AbortMultipartUpload(ctx, &oss.AbortMultipartUploadRequest{Bucket: oss.Ptr(p.Bucket), Key: oss.Ptr(key), UploadId: mp.UploadId})
		if e != nil {
			fatalCloud("AbortMultipartUpload", p.Bucket, key, e)
		}
		mark(p.Bucket + ": ListMultipartUploads + ListParts + AbortMultipartUpload")
	}
	router := a.router()
	call := func(method, path, token string, payload any, want int) map[string]any {
		var body bytes.Buffer
		if payload != nil {
			json.NewEncoder(&body).Encode(payload)
		}
		req := httptest.NewRequest(method, "https://zqzhg.com/api/v1"+path, &body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		out := httptest.NewRecorder()
		router.ServeHTTP(out, req)
		var result map[string]any
		json.Unmarshal(out.Body.Bytes(), &result)
		if out.Code != want {
			report["api_failure"] = map[string]any{"method": method, "path": path, "status": out.Code, "error": result["error"]}
			t.Fatalf("API %s %s expected %d got %d", method, path, want, out.Code)
		}
		return result
	}
	admin := resourceTestSession(t, a, "admin")
	call("POST", "/admin/storage/oss/check", admin, gin.H{"confirm": true, "confirm_public_probe": false}, 200)
	mark("readiness: signed PUT, private rejection, Range, exact-origin CORS")
	merchant := resourceTestSession(t, a, "merchant")
	other := resourceTestSession(t, a, "merchant")
	var owner Account
	a.DB.Where("role='merchant'").Order("id").First(&owner)
	vendor := Content{OwnerID: owner.ID, Kind: "vendor", Visible: false}
	if a.DB.Create(&vendor).Error != nil {
		t.Fatal("isolated enterprise failed")
	}
	var pngData bytes.Buffer
	png.Encode(&pngData, image.NewNRGBA(image.Rect(0, 0, 96, 64)))
	videoPath := filepath.Join(t.TempDir(), "synthetic.mp4")
	command := exec.CommandContext(ctx, env("FFMPEG_PATH", "ffmpeg"), "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=blue:s=160x120:r=10", "-t", "1", "-c:v", "libx264", "-threads", "1", "-pix_fmt", "yuv420p", "-movflags", "+faststart", "-an", "-y", videoPath)
	if command.Run() != nil {
		t.Fatal("synthetic H264 fixture generation failed")
	}
	video, _ := os.ReadFile(videoPath)
	httpClient := &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	putSigned := func(address string, headers map[string]any, data []byte) {
		req, _ := http.NewRequestWithContext(ctx, "PUT", address, bytes.NewReader(data))
		req.ContentLength = int64(len(data))
		for k, v := range headers {
			req.Header.Set(k, fmt.Sprint(v))
		}
		response, e := httpClient.Do(req)
		if e != nil {
			t.Fatal("signed PUT network failure")
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			body, _ := io.ReadAll(io.LimitReader(response.Body, 16384))
			report["failure"] = cloudLiveHTTPFailure("signed PUT", private.Bucket, response, body)
			t.Fatal("signed PUT rejected; see acceptance report")
		}
	}
	for _, f := range []struct {
		kind, name, purpose string
		data                []byte
	}{{"image", "synthetic-photo.png", "product_image", pngData.Bytes()}, {"technical", "synthetic-drawing.png", "dimensions", pngData.Bytes()}, {"pdf", "synthetic-catalog.pdf", "", tinyPDF()}, {"video", "synthetic-video.mp4", "", video}, {"pdf", "synthetic-multipart.pdf", "", cloudLiveLargePDF(33 << 20)}} {
		created := call("POST", "/merchant/files/uploads", merchant, gin.H{"kind": f.kind, "filename": f.name, "purpose": f.purpose, "size": len(f.data), "idempotency_key": randomToken()}, 201)
		session := created["session"].(map[string]any)
		sid := session["id"].(string)
		authorization := call("POST", "/merchant/files/uploads/"+sid+"/authorization", merchant, gin.H{}, 200)
		if session["upload_mode"] == "multipart" || len(f.data) >= 32<<20 {
			for part := 1; int64(part-1)*cloudPartBytes < int64(len(f.data)); part++ {
				result := call("POST", "/merchant/files/uploads/"+sid+"/parts", merchant, gin.H{"parts": []int{part}}, 200)
				p := result["items"].([]any)[0].(map[string]any)
				offset := int(p["offset"].(float64))
				size := int(p["size"].(float64))
				putSigned(p["url"].(string), p["headers"].(map[string]any), f.data[offset:offset+size])
			}
			resume := call("POST", "/merchant/files/uploads/"+sid+"/authorization", merchant, gin.H{}, 200)
			uploadedParts, ok := resume["uploaded_parts"].([]any)
			expectedParts := (int64(len(f.data)) + cloudPartBytes - 1) / cloudPartBytes
			if !ok || int64(len(uploadedParts)) != expectedParts {
				t.Fatal("resume omitted uploaded parts")
			}
			report["multipart_resume_part_count"] = len(uploadedParts)
		} else {
			putSigned(authorization["url"].(string), authorization["headers"].(map[string]any), f.data)
		}
		call("POST", "/merchant/files/uploads/"+sid+"/complete", merchant, gin.H{}, 202)
		if !a.processNextCloudUpload(ctx) {
			t.Fatal("no upload claimed")
		}
		var saved FileUploadSession
		a.DB.First(&saved, "id=?", sid)
		if saved.UploadState != "completed" {
			report["processing_failure"] = saved.Error
			t.Fatal("synthetic file processing failed", f.kind, saved.Error)
		}
		access := call("GET", fmt.Sprintf("/merchant/files/%d/access", saved.MediaID), merchant, nil, 200)
		call("GET", fmt.Sprintf("/merchant/files/%d/access", saved.MediaID), other, nil, 404)
		read, _ := http.NewRequestWithContext(ctx, "GET", access["url"].(string), nil)
		read.Header.Set("Range", "bytes=0-7")
		response, e := httpClient.Do(read)
		if e != nil {
			t.Fatal("private read failed")
		}
		io.Copy(io.Discard, response.Body)
		response.Body.Close()
		if response.StatusCode != 206 {
			t.Fatal("private Range failed")
		}
		var media Media
		a.DB.First(&media, saved.MediaID)
		if f.kind == "image" && !strings.HasSuffix(media.Path, ".webp") {
			t.Fatal("photo was not WebP")
		}
		if f.kind == "technical" {
			o, _ := a.objectRecord(media.Path)
			if o.SHA256 != digestText(string(f.data)) {
				t.Fatal("technical original changed")
			}
		}
		object, _ := a.objectRecord(media.Path)
		job := FileStorageJob{ID: randomToken(), OwnerID: owner.ID, EnterpriseID: vendor.ID}
		if e = a.publishFileObject(ctx, job, media, object); e != nil {
			fatalCloud("CopyObject/checksum", public.Bucket, media.Path, e)
		}
		if e = a.publishFileObject(ctx, job, media, object); e != nil {
			t.Fatal("public-copy retry failed")
		}
		mark(f.name + ": upload + validate + private Range + isolation + public copy + idempotent retry")
	}
	used, reserved, _, e := a.ownerUsage(a.DB, owner.ID)
	if e != nil || used <= 0 {
		t.Fatal("physical accounting unavailable")
	}
	report["usage_bytes"] = used
	report["reserved_bytes"] = reserved
	// A request exceeding the effective remaining budget must be rejected before
	// any object is created. This quota override changes the isolated account only.
	if e = a.DB.Model(&MerchantStorageQuota{}).Where("account_id=?", owner.ID).Update("quota_bytes", used+reserved+512).Error; e != nil {
		t.Fatal("isolated quota setup failed")
	}
	call("POST", "/merchant/files/uploads", merchant, gin.H{"kind": "pdf", "filename": "over-quota.pdf", "size": 4096, "idempotency_key": randomToken()}, 409)
	mark("quota exhaustion rejected before upload")
	call("POST", "/admin/storage/oss/check", admin, gin.H{"confirm": true, "confirm_public_probe": true}, 200)
	mark("public prefixes: platform and company readable; upload-temp denied; exact-origin CORS")
	report["passed"] = true
}

func cloudLiveFailure(api, bucket, key string, e error) map[string]any {
	d := map[string]any{"api": api, "resource": "acs:oss:*:*:" + bucket + "/" + key, "role": "ZQZHG-SMS-Role", "error": cloudError(e).Error()}
	var se *oss.ServiceError
	if errors.As(e, &se) {
		d["error_code"] = se.Code
		d["http_status"] = se.StatusCode
		d["ec"] = se.EC
		d["request_id"] = se.RequestID
	}
	return d
}
func cloudLiveHTTPFailure(api, bucket string, response *http.Response, body []byte) map[string]any {
	var parsed struct {
		Code      string `xml:"Code"`
		RequestID string `xml:"RequestId"`
		EC        string `xml:"EC"`
	}
	xml.Unmarshal(body, &parsed)
	return map[string]any{"api": api, "bucket": bucket, "http_status": response.StatusCode, "error_code": parsed.Code, "ec": parsed.EC, "request_id": parsed.RequestID, "role": "ZQZHG-SMS-Role"}
}
func cloudLiveLargePDF(size int) []byte {
	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R] /Count 1 >>", "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 400] >>", fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", size, strings.Repeat("X", size))}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i, o := range objects {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return b.Bytes()
}
