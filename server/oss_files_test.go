package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestCloudUploadContentAndSafetyRules(t *testing.T) {
	a := importTestApp(t)
	for _, req := range []cloudUploadRequest{
		{Kind: "image", Purpose: "product_image", Filename: "x.svg", Size: 100, IdempotencyKey: "test-request-id"},
		{Kind: "image", Purpose: "product_image", Filename: "x.png", MimeType: "text/html", Size: 100, IdempotencyKey: "test-request-id"},
		{Kind: "ai_original", Filename: "x.pdf", Size: 30000001, IdempotencyKey: "test-request-id"},
	} {
		if _, e := a.validateCloudUpload(&req); e == nil {
			t.Fatalf("unsafe request accepted: %s", req.Filename)
		}
	}
	if safeUploadFilename("../../a\n.jpg") != "a.jpg" {
		t.Fatal("unsafe filename")
	}
	if cloudError(fmt.Errorf("url https://upload.invalid/?security-token=SECRET")).Error() != "oss_unavailable" {
		t.Fatal("raw SDK error exposed")
	}
	for _, bad := range []string{"http://upload.zqzhg.com", "https://u:p@upload.zqzhg.com", "https://upload.zqzhg.com/private"} {
		cfg := cloudConfig()
		cfg.UploadBase = bad
		if cfg.validate() == nil {
			t.Fatal("unsafe cloud config accepted")
		}
	}
}
func TestCloudVideoFastStartParser(t *testing.T) {
	box := func(name string) []byte {
		b := make([]byte, 8)
		binary.BigEndian.PutUint32(b, 8)
		copy(b[4:], name)
		return b
	}
	for _, test := range []struct {
		data []byte
		want bool
	}{{append(box("ftyp"), append(box("moov"), box("mdat")...)...), true}, {append(box("ftyp"), append(box("mdat"), box("moov")...)...), false}, {[]byte("invalid"), false}} {
		p := filepath.Join(t.TempDir(), "test.mp4")
		os.WriteFile(p, test.data, 0600)
		if mp4FastStart(p) != test.want {
			t.Fatal("wrong mp4 ordering decision")
		}
	}
}

func TestCloudCORSExposureForBrowserRange(t *testing.T) {
	for _, test := range []struct {
		exposed string
		want    bool
	}{
		{"ETag, x-oss-request-id", false},
		{"*", false},
		{"ETag, Content-Length, Content-Range, Accept-Ranges", true},
		{"etag, content-length, content-range, accept-ranges, x-oss-request-id", true},
	} {
		h := http.Header{"Access-Control-Expose-Headers": []string{test.exposed}}
		if cloudCORSExposes(h, "ETag", "Content-Length", "Content-Range", "Accept-Ranges") != test.want {
			t.Fatal("browser range exposure was incorrectly accepted", test.exposed)
		}
	}
}

func TestCloudFileSessionsIsolationAccountingAndMigration(t *testing.T) {
	a := importTestApp(t)
	t.Setenv("OSS_ENABLED", "true")
	t.Setenv("OSS_TEST_ASSUME_READY", "true")
	t.Setenv("OSS_UPLOAD_TIMEOUT", "30")
	if e := a.initializeCloudFiles(); e != nil {
		t.Fatal(e)
	}
	cfg, e := a.systemConfig()
	if e != nil || cfg.Uploads.DefaultQuotaBytes != enterpriseOSSQuota {
		t.Fatal("wrong MiB policy", e)
	}
	provider := credentials.NewStaticCredentialsProvider("fixture-id", "fixture-secret", "fixture-token")
	a.cloudFiles().provider = provider
	var mu sync.Mutex
	objects := map[string][]byte{}
	types := map[string]string{}
	failCopy := false
	parts := map[string]map[int][]byte{}
	uploadKeys := map[string]string{}
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		key := r.URL.Path
		if !strings.HasPrefix(r.Header.Get("Authorization"), "OSS4-HMAC-SHA256 ") {
			t.Error("missing V4 signature")
		}

		query := r.URL.Query()
		if r.Method == "POST" && query.Has("uploads") {
			uploadID := "fixture-" + strconv.Itoa(len(uploadKeys)+1)
			uploadKeys[uploadID] = key
			parts[uploadID] = map[int][]byte{}
			types[key] = r.Header.Get("Content-Type")
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, "<InitiateMultipartUploadResult><Bucket>zqzhg-backup</Bucket><Key>%s</Key><UploadId>%s</UploadId></InitiateMultipartUploadResult>", strings.TrimPrefix(key, "/"), uploadID)
			return
		}
		if uploadID := query.Get("uploadId"); uploadID != "" {
			w.Header().Set("Content-Type", "application/xml")
			switch r.Method {
			case "PUT":
				part, _ := strconv.Atoi(query.Get("partNumber"))
				data, _ := io.ReadAll(r.Body)
				parts[uploadID][part] = data
				w.Header().Set("ETag", fmt.Sprintf("\"etag-%d\"", part))
				return
			case "POST":
				var complete struct {
					Parts []struct {
						Number int `xml:"PartNumber"`
					} `xml:"Part"`
				}
				xml.NewDecoder(r.Body).Decode(&complete)
				var data []byte
				for _, part := range complete.Parts {
					data = append(data, parts[uploadID][part.Number]...)
				}
				objects[key] = data
				delete(parts, uploadID)
				delete(uploadKeys, uploadID)
				io.WriteString(w, "<CompleteMultipartUploadResult><ETag>etag</ETag></CompleteMultipartUploadResult>")
				return
			case "GET":
				io.WriteString(w, "<ListPartsResult>")
				for n := 1; n <= len(parts[uploadID]); n++ {
					fmt.Fprintf(w, "<Part><PartNumber>%d</PartNumber><Size>%d</Size><ETag>etag-%d</ETag></Part>", n, len(parts[uploadID][n]), n)
				}
				io.WriteString(w, "</ListPartsResult>")
				return
			case "DELETE":
				delete(parts, uploadID)
				delete(uploadKeys, uploadID)
				w.WriteHeader(204)
				return
			}
		}
		if r.Method == "GET" && query.Has("uploads") {
			io.WriteString(w, "<ListMultipartUploadsResult>")
			for uploadID, key := range uploadKeys {
				if strings.HasPrefix(key, "/"+strings.TrimPrefix(r.URL.Path, "/")+"/"+query.Get("prefix")) {
					fmt.Fprintf(w, "<Upload><Key>%s</Key><UploadId>%s</UploadId></Upload>", strings.TrimPrefix(key, strings.TrimRight(r.URL.Path, "/")+"/"), uploadID)
				}
			}
			io.WriteString(w, "</ListMultipartUploadsResult>")
			return
		}
		switch r.Method {
		case "PUT":
			if r.Header.Get("x-oss-object-acl") != "" {
				t.Error("managed role upload tried ACL mutation")
			}
			if source := r.Header.Get("x-oss-copy-source"); source != "" {
				if failCopy {
					w.WriteHeader(503)
					io.WriteString(w, "<Error><Code>ServiceUnavailable</Code></Error>")
					return
				}
				objects[key] = append([]byte{}, objects[source]...)
				types[key] = types[source]
				w.Header().Set("Content-Type", "application/xml")
				io.WriteString(w, "<CopyObjectResult><ETag>etag</ETag><LastModified>2026-11-18T00:00:00.000Z</LastModified></CopyObjectResult>")
				return
			}
			data, _ := io.ReadAll(r.Body)
			objects[key] = data
			types[key] = r.Header.Get("Content-Type")
			w.Header().Set("ETag", "\"etag\"")
		case "GET", "HEAD":
			data, ok := objects[key]
			if !ok {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(404)
				if r.Method != "HEAD" {
					io.WriteString(w, "<Error><Code>NoSuchKey</Code></Error>")
				}
				return
			}
			w.Header().Set("Content-Type", types[key])
			http.ServeContent(w, r, "file", time.Unix(1700000000, 0), bytes.NewReader(data))
		case "DELETE":
			delete(objects, key)
			delete(types, key)
			w.WriteHeader(204)
		}
	}))
	defer cloud.Close()
	var profiles []StorageProfile
	a.DB.Where("auth_mode='ecs_role'").Find(&profiles)
	for _, p := range profiles {
		client := oss.NewClient(oss.LoadDefaultConfig().WithRegion(p.Region).WithEndpoint(cloud.URL).WithDisableSSL(true).WithUsePathStyle(true).WithCredentialsProvider(provider).WithRetryMaxAttempts(1))
		a.cloudFiles().clients[fmt.Sprintf("%d|%s|%s|%s", p.ID, p.Endpoint, p.Bucket, p.RoleName)] = client
	}
	tc := testClient{t, a.router()}
	token := tc.login("13900006111")
	other := tc.login("13900006112")
	who := tc.call("GET", "/auth/me", token, nil, 200)
	owner := uint64(who["id"].(float64))
	req := gin.H{"kind": "technical", "purpose": "dimensions", "filename": "drawing.png", "size": 100, "idempotency_key": "fixture-upload-request"}
	init := tc.call("POST", "/merchant/files/uploads", token, req, 201)
	session := init["session"].(map[string]any)
	sid := session["id"].(string)
	if strings.Contains(init["url"].(string), "fixture-secret") || !strings.Contains(init["url"].(string), "upload.zqzhg.com/") {
		t.Fatal("bad CNAME signing")
	}
	signed := init["headers"].(map[string]any)
	if signed["Content-Length"] != "100" && signed["content-length"] != "100" {
		t.Fatal("length not bound", signed)
	}
	tc.call("GET", "/merchant/files/uploads/"+sid, other, nil, 404)
	again := tc.call("POST", "/merchant/files/uploads", token, req, 201)
	if again["session"].(map[string]any)["id"] != sid {
		t.Fatal("duplicate upload session")
	}
	req["size"] = 101
	tc.call("POST", "/merchant/files/uploads", token, req, 409)
	usage := tc.call("GET", "/merchant/storage/usage", token, nil, 200)
	if usage["reserved_bytes"] != float64(100) {
		t.Fatal("wrong reservation")
	}
	tc.call("POST", "/merchant/files/uploads/"+sid+"/cancel", token, gin.H{}, 202)
	usage = tc.call("GET", "/merchant/storage/usage", token, nil, 200)
	if usage["reserved_bytes"] != float64(100) {
		t.Fatal("cancel freed capacity while signature valid")
	}
	a.DB.Model(&FileUploadSession{}).Where("id=?", sid).Update("signature_expires_at", time.Now().Add(-time.Hour))
	if !a.reclaimCloudUpload(context.Background()) {
		t.Fatal("cancel cleanup failed")
	}
	usage = tc.call("GET", "/merchant/storage/usage", token, nil, 200)
	if usage["reserved_bytes"] != float64(0) {
		t.Fatal("confirmed deletion not freed")
	}

	// Multipart authorization and completion are checked against actual server state.
	multipart := tc.call("POST", "/merchant/files/uploads", token, gin.H{"kind": "video", "filename": "clip.mp4", "size": 33 << 20, "purpose": "resource_file", "idempotency_key": "fixture-multipart-request"}, 201)
	msid := multipart["session"].(map[string]any)["id"].(string)
	tc.call("POST", "/merchant/files/uploads/"+msid+"/parts", other, gin.H{"parts": []int{1}}, 404)
	tc.call("POST", "/merchant/files/uploads/"+msid+"/parts", token, gin.H{"parts": []int{1, 2, 3}}, 400)
	partAuth := tc.call("POST", "/merchant/files/uploads/"+msid+"/parts", token, gin.H{"parts": []int{5}}, 200)
	part := partAuth["items"].([]any)[0].(map[string]any)
	partExpiry, expiryError := time.Parse(time.RFC3339Nano, part["expires_at"].(string))
	if expiryError != nil || partExpiry.After(time.Now().Add(31*time.Second)) || partExpiry.Before(time.Now().Add(20*time.Second)) {
		t.Fatal("multipart authorization ignored configured lifetime")
	}
	if part["size"] != float64(1<<20) || part["offset"] != float64(32<<20) {
		t.Fatal("last part bounds wrong")
	}
	tc.call("POST", "/merchant/files/uploads/"+msid+"/complete", token, gin.H{}, 409)
	tc.call("POST", "/merchant/files/uploads/"+msid+"/cancel", token, gin.H{}, 202)
	a.DB.Model(&FileUploadSession{}).Where("id=?", msid).Update("signature_expires_at", time.Now().Add(-time.Hour))
	if !a.reclaimCloudUpload(context.Background()) {
		t.Fatal("multipart abort cleanup failed")
	}
	// Content must decode successfully before a Media row is created.
	var imageBytes bytes.Buffer
	png.Encode(&imageBytes, image.NewNRGBA(image.Rect(0, 0, 8, 8)))
	data := imageBytes.Bytes()
	req = gin.H{"kind": "technical", "purpose": "dimensions", "filename": "drawing.png", "size": len(data), "idempotency_key": "fixture-valid-drawing"}
	result := tc.call("POST", "/merchant/files/uploads", token, req, 201)
	sid = result["session"].(map[string]any)["id"].(string)
	var upload FileUploadSession
	a.DB.First(&upload, "id=?", sid)
	store, _ := a.store(upload.StoreID)
	if e := store.Put(context.Background(), upload.ObjectKey, bytes.NewReader(data), int64(len(data))); e != nil {
		t.Fatal(e)
	}
	recovered := tc.call("POST", "/merchant/files/uploads/"+sid+"/authorization", token, gin.H{}, 200)
	if recovered["session"].(map[string]any)["state"] != "validating" {
		t.Fatal("lost PUT response not recovered")
	}
	tc.call("POST", "/merchant/files/uploads/"+sid+"/complete", token, gin.H{}, 200)
	if !a.processNextCloudUpload(context.Background()) {
		t.Fatal("worker did not claim")
	}
	a.DB.First(&upload, "id=?", sid)
	if upload.UploadState != "completed" || upload.MediaID == 0 {
		t.Fatal("upload not validated", upload.Error)
	}
	tc.call("GET", fmt.Sprintf("/merchant/files/%d/access", upload.MediaID), other, nil, 404)
	tc.call("GET", fmt.Sprintf("/merchant/files/%d/access", upload.MediaID), token, nil, 200)
	// The OSS-compatible rollback baseline pauses writes while retaining reads.
	t.Setenv("OSS_ENABLED", "false")
	tc.call("GET", fmt.Sprintf("/merchant/files/%d/access", upload.MediaID), token, nil, 200)
	tc.call("POST", "/merchant/files/uploads", token, gin.H{"kind": "technical", "purpose": "dimensions", "filename": "paused.png", "size": len(data), "idempotency_key": "paused-upload-fixture"}, 503)
	t.Setenv("OSS_ENABLED", "true")
	var count int64
	a.DB.Model(&Media{}).Where("owner_id=?", owner).Count(&count)
	a.processNextCloudUpload(context.Background())
	var count2 int64
	a.DB.Model(&Media{}).Where("owner_id=?", owner).Count(&count2)
	if count != count2 {
		t.Fatal("worker created duplicate media")
	}
	var media Media
	a.DB.First(&media, upload.MediaID)
	object, _ := a.objectRecord(media.Path)
	job := FileStorageJob{ID: randomToken(), OwnerID: owner, EnterpriseID: 67}
	if e := a.publishFileObject(context.Background(), job, media, object); e != nil {
		t.Fatal("public copy", e)
	}
	used, reserved, _, e := a.ownerUsage(a.DB, owner)
	if e != nil || used != 2*int64(len(data)) || reserved != 0 {
		t.Fatal("physical public copy not accounted", used, reserved, e)
	}
	if e := a.publishFileObject(context.Background(), job, media, object); e != nil {
		t.Fatal(e)
	}
	used, _, _, _ = a.ownerUsage(a.DB, owner)
	if used != 2*int64(len(data)) {
		t.Fatal("copy retry double charged")
	}
	// Pins prevent deletion of both canonical and public objects.
	a.DB.Create(&FileBackupPin{SnapshotID: "fixture", ObjectKey: media.Path, ExpiresAt: time.Now().Add(time.Hour)})
	a.DB.Model(&FileUploadSession{}).Where("media_id=?", media.ID).Update("signature_expires_at", time.Now().Add(-time.Hour))
	deleted := tc.call("DELETE", fmt.Sprintf("/merchant/files/%d", media.ID), token, gin.H{"confirm": true}, 409)
	if deleted["error"].(map[string]any)["code"] != "file_backup_pinned" {
		t.Fatal("backup pin did not protect file")
	}
	a.DB.Where("snapshot_id='fixture'").Delete(&FileBackupPin{})

	// A historic logical path remains stable after verified remote mapping.
	localKey := "legacy-drawing.png"
	os.WriteFile(filepath.Join(a.Config.MediaDir, localKey), data, 0600)
	legacy := StoredObject{Key: localKey, StoreID: 0, Size: int64(len(data)), SHA256: digestText(string(data)), UpdatedAt: time.Now().UTC()}
	a.DB.Create(&legacy)
	m := Media{OwnerID: owner, Path: localKey}
	a.DB.Create(&m)
	a.DB.Create(&MediaObjectOwner{ObjectKey: localKey, OwnerID: owner, Size: legacy.Size})
	p, _ := a.cloudProfile("private")
	migration := FileStorageJob{ID: randomToken(), JobKind: "migrate", OwnerID: owner, EnterpriseID: 67}
	a.DB.Create(&migration)
	a.DB.Create(&FileStorageJobItem{JobID: migration.ID, SourceKey: localKey, TargetKey: "companies/67/drafts/legacy.png", TargetStoreID: p.ID, MediaID: m.ID, FileSize: legacy.Size, SHA256: legacy.SHA256, Status: "queued"})
	if e := a.executeFileMigration(context.Background(), migration); e != nil {
		t.Fatal("migration failed", e)
	}
	o, mapped, e := a.resolveObject(context.Background(), localKey)
	if e != nil || o.RemoteKey == "" {
		t.Fatal("legacy read mapping failed", e)
	}
	if e = verifyObject(context.Background(), mapped, localKey, legacy.SHA256, legacy.Size); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(a.Config.MediaDir, localKey)); e != nil {
		t.Fatal("migration removed historical source")
	}

	// Failed public copy must leave the previous public business revision intact.
	content := Content{Kind: "vendor", OwnerID: owner, HallID: 1, Visible: true, Epoch: 9}
	if e := a.DB.Create(&content).Error; e != nil {
		t.Fatal(e)
	}
	old := Revision{ContentID: content.ID, Status: "approved", Payload: aiJSON(Payload{Name: "old"})}
	a.DB.Create(&old)
	next := Revision{ContentID: content.ID, Status: "approved", Payload: aiJSON(Payload{Name: "new", MediaIDs: []uint64{m.ID}})}
	a.DB.Create(&next)
	a.DB.Model(&content).Updates(map[string]any{"published_revision_id": old.ID, "draft_revision_id": next.ID})
	if e := a.DB.Transaction(func(tx *gorm.DB) error {
		queued, e := a.queueFilePublication(tx, "content", content.ID, next.ID, 9, owner, content.ID, []uint64{m.ID})
		if !queued && e == nil {
			t.Error("public copy not queued")
		}
		return e
	}); e != nil {
		t.Fatal(e)
	}
	mu.Lock()
	failCopy = true
	mu.Unlock()
	a.runNextStorageFileJob(context.Background())
	a.DB.First(&content, content.ID)
	if content.PublishedRevisionID != old.ID {
		t.Fatal("failed copy changed published revision")
	}
	var publish FileStorageJob
	a.DB.Where("job_kind='publish' AND resource_id=?", content.ID).First(&publish)
	if publish.Status != "failed" {
		t.Fatal("copy failure not recoverable", publish.Status)
	}
	mu.Lock()
	failCopy = false
	mu.Unlock()
	a.DB.Model(&publish).Updates(map[string]any{"status": "queued"})
	a.runNextStorageFileJob(context.Background())
	a.DB.First(&content, content.ID)
	if content.PublishedRevisionID != next.ID || content.Epoch != 10 {
		t.Fatal("verified copy did not switch revision")
	}
	var public EnterpriseFileObject
	a.DB.Where("source_key=? AND object_role='public' AND object_state='active'", m.Path).First(&public)
	if public.ObjectKey == "" {
		t.Fatal("public copy missing")
	}
	a.DB.Model(&content).Updates(map[string]any{"visible": false, "epoch": 11})
	for n := 0; n < 4; n++ {
		a.reclaimPublicFile(context.Background())
	}
	a.DB.First(&public, "object_key=?", public.ObjectKey)
	if public.ObjectState != "deleted" {
		t.Fatal("offline copy retained without use")
	}

	// Normal product photos produce WebP + thumbnail; disguised file formats fail.
	for _, test := range []struct {
		name string
		want string
	}{{"photo.png", "completed"}, {"disguised.jpg", "failed"}} {
		created := tc.call("POST", "/merchant/files/uploads", token, gin.H{"kind": "image", "purpose": "product_image", "filename": test.name, "size": len(data), "idempotency_key": "image-fixture-" + test.name}, 201)
		imageSessionID := created["session"].(map[string]any)["id"].(string)
		var session FileUploadSession
		a.DB.First(&session, "id=?", imageSessionID)
		store, _ := a.store(session.StoreID)
		if e := store.Put(context.Background(), session.ObjectKey, bytes.NewReader(data), int64(len(data))); e != nil {
			t.Fatal(e)
		}
		tc.call("POST", "/merchant/files/uploads/"+imageSessionID+"/complete", token, gin.H{}, 202)
		a.processNextCloudUpload(context.Background())
		a.DB.First(&session, "id=?", imageSessionID)
		if session.UploadState != test.want {
			t.Fatal("photo validation", test.name, session.UploadState, session.Error)
		}
		if test.want == "completed" {
			var photo Media
			a.DB.First(&photo, session.MediaID)
			if !strings.HasSuffix(photo.Path, ".webp") || !strings.HasSuffix(photo.ThumbPath, ".webp") {
				t.Fatal("photo derivatives not WebP")
			}
		}
	}
	// Backup also preserves unregistered files privately without inventing ownership.
	os.WriteFile(filepath.Join(a.Config.MediaDir, "unregistered.pdf"), []byte("private unregistered fixture"), 0600)
	// Backup sends SQL through authenticated encryption and deduplicates immutable objects.
	private, _ := a.cloudProfile("private")
	backupClient, _ := a.cloudClient(private, false)
	encryption := bytes.Repeat([]byte{9}, 32)
	if e := a.createCloudBackup(context.Background(), backupClient, encryption); e != nil {
		t.Fatal("stream backup", e)
	}
	snapshot := ""
	mu.Lock()
	for key := range objects {
		if strings.HasPrefix(key, "/zqzhg-backup/snapshots/") && strings.HasSuffix(key, "/manifest.json.zqenc") {
			snapshot = strings.Split(key, "/")[3]
		}
	}
	mu.Unlock()
	manifest, e := readCloudBackupManifest(context.Background(), backupClient, encryption, snapshot)
	if e != nil {
		t.Fatal("backup manifest", e)
	}
	if e = verifyCloudSQL(context.Background(), backupClient, encryption, manifest, io.Discard); e != nil {
		t.Fatal("encrypted sql", e)
	}
	for _, item := range manifest.Objects {
		if e := verifyObject(context.Background(), &ossObjectStore{Client: backupClient, Bucket: operatorBackupBucket}, item.BackupKey, item.Object.SHA256, item.Object.Size); e != nil {
			t.Fatal("backup object", e)
		}
	}

	output := filepath.Join(t.TempDir(), "restore-result.json")
	out, e := os.OpenFile(output, os.O_CREATE|os.O_WRONLY, 0600)
	if e != nil {
		t.Fatal(e)
	}
	savedStdout := os.Stdout
	os.Stdout = out
	e = a.restoreCloudBackup(context.Background(), backupClient, encryption, manifest)
	os.Stdout = savedStdout
	out.Close()
	if e != nil {
		t.Fatal("isolated restore", e)
	}
	raw, e := os.ReadFile(output)
	if e != nil {
		t.Fatal(e)
	}
	var restoredResult map[string]any
	if e = json.Unmarshal(raw, &restoredResult); e != nil {
		t.Fatal(e)
	}
	restoredName, _ := restoredResult["isolated_database"].(string)
	if !strings.HasPrefix(restoredName, "zqzt_exhibition_restore_") || len(restoredName) != len("zqzt_exhibition_restore_")+16 {
		t.Fatal("unsafe restored database name")
	}
	defer a.DB.Exec("DROP DATABASE `" + restoredName + "`")
	restoredDB, e := openExistingDB(restoredName, env("DB_USER", "root"), env("DB_PASSWORD", "root"))
	if e != nil {
		t.Fatal(e)
	}
	restoredPool, _ := restoredDB.DB()
	defer restoredPool.Close()
	var activeRestored int64
	restoredDB.Model(&FileUploadSession{}).Where("object_key<>'' OR upload_state<>'restored'").Count(&activeRestored)
	if activeRestored != 0 {
		t.Fatal("restored worker could touch original keys")
	}
	var restoredObject StoredObject
	if restoredDB.First(&restoredObject, "object_key=?", localKey).Error != nil || restoredObject.RemoteKey == o.RemoteKey || restoredObject.StoreID == 0 {
		t.Fatal("isolated restore lost remote mapping")
	}
	if len(restoredResult["unregistered_files"].([]any)) != 1 {
		t.Fatal("unregistered recovery inventory missing")
	}
	var invented int64
	restoredDB.Model(&StoredObject{}).Where("object_key='unregistered.pdf'").Count(&invented)
	if invented != 0 {
		t.Fatal("unknown file was automatically associated")
	}
	if a.currentDatabase() == restoredName {
		t.Fatal("restore changed active database")
	}
	if e := migrate(a.DB); e != nil {
		t.Fatal("migration not rerunnable", e)
	}
}

func TestCloudSessionJSONNeverContainsObjectCredentials(t *testing.T) {
	s := FileUploadSession{ID: "id", ObjectKey: "PRIVATE-KEY", MultipartID: "PRIVATE-MULTIPART", LeaseToken: "LEASE", Fingerprint: "HASH", OwnerID: 8}
	b, _ := json.Marshal(s)
	for _, secret := range []string{"PRIVATE-KEY", "PRIVATE-MULTIPART", "LEASE", "HASH", "owner_id"} {
		if bytes.Contains(b, []byte(secret)) {
			t.Fatal("session exposes private internal state")
		}
	}
}
