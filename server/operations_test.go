package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func waitOperation(t *testing.T, a *App, id string) OperationJob {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		r := a.operations()
		r.mu.Lock()
		job := *r.jobs[id]
		r.mu.Unlock()
		if job.State == "succeeded" || job.State == "failed" || job.State == "cancelled" {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job timeout")
	return OperationJob{}
}
func TestPermissionDependencies(t *testing.T) {
	grants, err := normalizePermissions([]string{"product.review", "analytics.export", "resource.write"})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(grants, ",")
	for _, key := range []string{"product.read", "analytics.read", "resource.read", "vendor.read"} {
		if !strings.Contains(joined, key) {
			t.Fatalf("missing dependency %s", key)
		}
	}
	if _, err = normalizePermissions([]string{"admin"}); err == nil {
		t.Fatal("accepted unknown permission")
	}
}
func TestEncryptedBackupFrames(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	original := bytes.Repeat([]byte("snapshot-content\x00"), 400000)
	var encrypted bytes.Buffer
	if err := backupCrypt(bytes.NewReader(original), &encrypted, key, true); err != nil {
		t.Fatal(err)
	}
	var restored bytes.Buffer
	if err := backupCrypt(bytes.NewReader(encrypted.Bytes()), &restored, key, false); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, restored.Bytes()) {
		t.Fatal("backup round trip differs")
	}
	data := append([]byte(nil), encrypted.Bytes()...)
	data[len(data)/2] ^= 1
	if backupCrypt(bytes.NewReader(data), io.Discard, key, false) == nil {
		t.Fatal("tampered archive accepted")
	}
	if backupCrypt(bytes.NewReader(encrypted.Bytes()[:encrypted.Len()-1]), io.Discard, key, false) == nil {
		t.Fatal("truncated archive accepted")
	}
	if backupCrypt(bytes.NewReader(append(encrypted.Bytes(), 1)), io.Discard, key, false) == nil {
		t.Fatal("trailing bytes accepted")
	}
	if backupCrypt(bytes.NewReader(encrypted.Bytes()), io.Discard, bytes.Repeat([]byte{8}, 32), false) == nil {
		t.Fatal("wrong key accepted")
	}
}
func TestLocalStorePreventsTraversalAndOverwrite(t *testing.T) {
	store := localObjectStore{t.TempDir()}
	ctx := context.Background()
	for _, key := range []string{"../outside", "/absolute", "a/../b", "a\\b"} {
		if store.Put(ctx, key, strings.NewReader("x"), 1) == nil {
			t.Fatalf("unsafe key %q", key)
		}
	}
	if err := store.Put(ctx, "safe/object", strings.NewReader("abc"), 3); err != nil {
		t.Fatal(err)
	}
	if store.Put(ctx, "safe/object", strings.NewReader("xyz"), 3) == nil {
		t.Fatal("overwrote immutable object")
	}
	if err := verifyObject(ctx, store, "safe/object", digest("abc"), 3); err != nil {
		t.Fatal(err)
	}
}
func TestPlatformOperatorIsolationAndRevocation(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	admin := resourceTestSession(t, a, "admin")
	created := tc.call("POST", "/admin/operators", admin, gin.H{"username": "operator-" + randomToken()[:10], "display_name": "审核岗位", "enabled": true, "permissions": []string{"vendor.review"}}, 201)
	login := tc.call("POST", "/auth/admin", "", gin.H{"username": created["username"], "password": created["initial_password"]}, 200)
	operator := login["token"].(string)
	tc.call("GET", "/admin/dashboard", operator, nil, 200)
	tc.call("GET", "/admin/contents?kind=vendor", operator, nil, 200)
	tc.call("GET", "/admin/contents?kind=product", operator, nil, 403)
	for _, path := range []string{"/admin/operators", "/admin/accounts", "/admin/storage/profiles", "/admin/backups", "/admin/analytics", "/admin/operation-logs"} {
		tc.call("GET", path, operator, nil, 403)
	}
	tc.call("POST", "/admin/vendors", operator, gin.H{}, 403)
	merchant := resourceTestSession(t, a, "merchant")
	v := tc.call("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "权限审核测试", MainProducts: "机械", RegionName: "河北", Phone: "0319-1234567"}}, 201)
	tc.call("POST", "/merchant/contents/"+number(v["id"])+"/submit", merchant, gin.H{"revision_id": v["revision_id"]}, 200)
	tc.call("POST", "/admin/revisions/"+number(v["revision_id"])+"/review", operator, gin.H{"decision": "approve"}, 200)
	var log OperationLog
	a.DB.Where("action='approve'").Last(&log)
	if log.ActorID == 0 {
		t.Fatal("review actor missing")
	}
	tc.call("PUT", "/admin/operators/"+number(created["id"]), admin, gin.H{"display_name": "停用审核员", "enabled": false, "permissions": []string{"vendor.review"}}, 200)
	tc.call("GET", "/admin/dashboard", operator, nil, 401)
	tc.call("POST", "/auth/admin", "", gin.H{"username": created["username"], "password": created["initial_password"]}, 401)
	tc.call("GET", "/admin/operators", merchant, nil, 403)
}
func TestAnalyticsDedupPrivacyAndUniqueVisitors(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	admin := resourceTestSession(t, a, "admin")
	now := time.Now().UTC()
	first := AnalyticsInput{EventID: "11111111-1111-4111-8111-111111111111", VisitorID: "anonymousvisitor0001", SessionID: "anonymoussession0001", Source: "web", EventName: "page_view", Page: "/", Referrer: "https://example.com/path?token=secret", OccurredAt: now}
	second := first
	second.EventID = "22222222-2222-4222-8222-222222222222"
	second.SessionID = "anonymoussession0002"
	second.OccurredAt = now.Add(-time.Hour)
	tc.call("POST", "/public/analytics/events", "", gin.H{"events": []AnalyticsInput{first, first, second}}, 202)
	var count int64
	a.DB.Model(&AnalyticsEvent{}).Count(&count)
	if count != 2 {
		t.Fatalf("dedup count=%d", count)
	}
	var saved AnalyticsEvent
	a.DB.First(&saved)
	if saved.Referrer != "example.com" || saved.VisitorHash == first.VisitorID {
		t.Fatal("raw visitor or full referrer persisted")
	}
	report := tc.call("GET", "/admin/analytics", ""+admin, nil, 200)
	summary := report["summary"].(map[string]any)
	if summary["pv"] != float64(2) || summary["uv"] != float64(1) || summary["visits"] != float64(2) {
		t.Fatalf("incorrect unique metrics %#v", summary)
	}
	bad := first
	bad.EventID = "33333333-3333-4333-8333-333333333333"
	bad.Page = "/admin?token=secret"
	tc.call("POST", "/public/analytics/events", "", gin.H{"events": []AnalyticsInput{bad}}, 202)
	a.DB.Model(&AnalyticsEvent{}).Count(&count)
	if count != 2 {
		t.Fatal("counted admin request")
	}
	if cleanSearch("电话13900001234") != "[已脱敏]" {
		t.Fatal("phone retained in keyword")
	}
	if csvSafe("=cmd") != "'=cmd" {
		t.Fatal("unsafe csv cell")
	}
}
func TestStorageUploadRangeAndMigration(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	admin := resourceTestSession(t, a, "admin")
	merchant := resourceTestSession(t, a, "merchant")
	var imageBytes bytes.Buffer
	png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	first := resourceUpload(t, a, merchant, "/merchant/media", "first.png", imageBytes.Bytes(), 201)
	root := t.TempDir()
	profile := tc.call("POST", "/admin/storage/profiles", admin, gin.H{"name": "第二存储", "kind": "local", "local_root": root}, 200)["profile"].(map[string]any)
	tc.call("POST", "/admin/storage/profiles/"+number(profile["id"])+"/activate", admin, gin.H{}, 200)
	second := resourceUpload(t, a, merchant, "/merchant/media", "second.png", imageBytes.Bytes(), 201)
	var media Media
	a.DB.First(&media, uint64(second["id"].(float64)))
	if _, err := os.Stat(filepath.Join(root, media.Path)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a.Config.MediaDir, media.Path)); !os.IsNotExist(err) {
		t.Fatal("new file stored in legacy directory")
	}
	request := httptest.NewRequest("GET", "/api/v1/media/"+number(second["id"]), nil)
	request.Header.Set("Authorization", "Bearer "+merchant)
	request.Header.Set("Range", "bytes=0-3")
	out := httptest.NewRecorder()
	a.router().ServeHTTP(out, request)
	if out.Code != 206 || out.Body.Len() != 4 {
		t.Fatalf("range %d %d", out.Code, out.Body.Len())
	}
	job, ticket, err := a.newJob("migration", migrationParams{0, uint64(profile["id"].(float64))}, 1)
	if err != nil || ticket == "" {
		t.Fatal(err)
	}
	done := waitOperation(t, a, job.ID)
	if done.State != "succeeded" {
		t.Fatalf("migration %s", done.Error)
	}
	media = Media{}
	a.DB.First(&media, uint64(first["id"].(float64)))
	var object StoredObject
	a.DB.First(&object, "object_key=?", media.Path)
	if object.StoreID != uint64(profile["id"].(float64)) {
		t.Fatal("migration did not switch mapping")
	}
	if _, err = os.Stat(filepath.Join(a.Config.MediaDir, media.Path)); err != nil {
		t.Fatal("old copy removed before confirmation")
	}
	tc.call("DELETE", "/admin/storage/profiles/"+number(profile["id"]), admin, nil, 409)
}
func TestCompleteBackupRestoreAndCutover(t *testing.T) {
	a := importTestApp(t)
	if _, err := mysqlTool("mysqldump"); err != nil {
		t.Skip("mysql tools unavailable")
	}
	t.Setenv("OPERATIONS_DIR", a.operations().dir)
	originalName := a.currentDatabase()
	names := []string{originalName}
	t.Cleanup(func() {
		db, err := openExistingDB("", env("DB_USER", "root"), env("DB_PASSWORD", "root"))
		if err == nil {
			defer func() { pool, _ := db.DB(); pool.Close() }()
			for _, name := range names {
				if strings.HasPrefix(name, "zqzt_exhibition_test_") || strings.HasPrefix(name, "zqzt_exhibition_restore_") {
					db.Exec("DROP DATABASE IF EXISTS `" + name + "`")
				}
			}
		}
	})
	admin := resourceTestSession(t, a, "admin")
	if err := os.WriteFile(filepath.Join(a.Config.MediaDir, "snapshot.txt"), []byte("snapshot media"), 0600); err != nil {
		t.Fatal(err)
	}
	backup, _, err := a.newJob("backup", backupParams{}, 1)
	if err != nil {
		t.Fatal(err)
	}
	done := waitOperation(t, a, backup.ID)
	if done.State != "succeeded" {
		t.Fatalf("backup %s", done.Error)
	}
	var entry BackupEntry
	json.Unmarshal(done.Result, &entry)
	restore, _, err := a.newJob("restore", restoreParams{entry.ID}, 1)
	if err != nil {
		t.Fatal(err)
	}
	done = waitOperation(t, a, restore.ID)
	if done.State != "succeeded" {
		t.Fatalf("restore %s", done.Error)
	}
	var result RestoreResult
	json.Unmarshal(done.Result, &result)
	names = append(names, result.Database)
	data, err := os.ReadFile(filepath.Join(result.MediaDir, "snapshot.txt"))
	if err != nil || string(data) != "snapshot media" {
		t.Fatal("restored media mismatch")
	}
	activate, _, err := a.newJob("activate", activateParams{restore.ID}, 1)
	if err != nil {
		t.Fatal(err)
	}
	done = waitOperation(t, a, activate.ID)
	if done.State != "succeeded" || a.currentDatabase() != result.Database {
		t.Fatalf("cutover %s", done.Error)
	}
	testClient{t, a.router()}.call("GET", "/admin/dashboard", admin, nil, 401)
	host, err := readHostTargetFrom(a.operations().dir)
	if err != nil || host.Pending || host.Previous.Database != originalName {
		t.Fatal("rollback target not preserved")
	}
	target, err := bootstrapTarget(a.Config.MediaDir, originalName)
	if err != nil || target.Database != result.Database {
		t.Fatal("restart lost committed target", err)
	}
	rollback, _, err := a.newJob("rollback", activateParams{}, 1)
	if err != nil {
		t.Fatal(err)
	}
	rolled := waitOperation(t, a, rollback.ID)
	if rolled.State != "succeeded" || a.currentDatabase() != originalName {
		t.Fatal("rollback failed", rolled.Error)
	}
}
func TestPersistentControlRestartAndPendingCutover(t *testing.T) {
	dir := t.TempDir()
	media := t.TempDir()
	t.Setenv("OPERATIONS_DIR", dir)
	a := &App{Config: Config{MediaDir: media}}
	r := a.operations()
	job := &OperationJob{ID: strings.Repeat("a", 24), Kind: "migration", State: "running", TicketHash: digest("short-ticket"), TicketExpires: time.Now().Add(time.Minute), Params: json.RawMessage(`{"source_id":0,"target_id":1}`)}
	if err := r.persist(job); err != nil {
		t.Fatal(err)
	}
	restartedApp := &App{Config: a.Config}
	restarted := restartedApp.operations()
	if restarted.jobs[job.ID].State != "interrupted" || len(restarted.jobs[job.ID].Params) == 0 {
		t.Fatal("job was not recovered safely")
	}
	restarted.jobs[job.ID].Result = json.RawMessage(`{"database":"hidden-db","manifest":{"objects":["private-list"]}}`)
	req := httptest.NewRequest("GET", "/api/v1/maintenance/"+job.ID, nil)
	req.Header.Set("X-Maintenance-Token", "short-ticket")
	out := httptest.NewRecorder()
	restartedApp.router().ServeHTTP(out, req)
	if out.Code != 200 || strings.Contains(out.Body.String(), "hidden-db") || strings.Contains(out.Body.String(), "private-list") {
		t.Fatal("maintenance token exceeded progress scope")
	}
	previous := ActiveTarget{"zqzt_before", media}
	next := ActiveTarget{"zqzt_after", t.TempDir()}
	if err := atomicJSON(filepath.Join(dir, "active.json"), HostTarget{Current: next, Previous: &previous, Pending: true, JobID: job.ID}); err != nil {
		t.Fatal(err)
	}
	target, err := bootstrapTarget(media, "ignored_default")
	if err != nil || target != previous {
		t.Fatal("interrupted cutover did not recover original pointer", err)
	}
	host, err := readHostTargetFrom(dir)
	if err != nil || host.Pending || host.Current != previous {
		t.Fatal("recovery pointer not persisted", err)
	}
	committedJob := &OperationJob{ID: strings.Repeat("b", 24), Kind: "activate", State: "running"}
	if err = r.persist(committedJob); err != nil {
		t.Fatal(err)
	}
	if err = atomicJSON(filepath.Join(dir, "active.json"), HostTarget{Current: next, Previous: &previous, Outcome: "committed", JobID: committedJob.ID}); err != nil {
		t.Fatal(err)
	}
	afterCommit := (&App{Config: a.Config}).operations().jobs[committedJob.ID]
	if afterCommit == nil || afterCommit.State != "succeeded" || !strings.Contains(string(afterCommit.Result), "relogin_required") {
		t.Fatal("committed cutover reported as interrupted after restart")
	}
	a.Config.Env = "production"
	t.Setenv("CONFIG_ENCRYPTION_KEY", "")
	if _, err = a.masterKey(); err == nil {
		t.Fatal("production backup allowed without independent key")
	}
}
func TestOperatorPrivateDraftMediaAndImmediateReset(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	admin := resourceTestSession(t, a, "admin")
	merchant := resourceTestSession(t, a, "merchant")
	v := tc.call("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "私有媒体厂商", MainProducts: "设备", RegionName: "河北", Phone: "0319-1234567"}}, 201)
	created := tc.call("POST", "/admin/operators", admin, gin.H{"username": "edit-" + randomToken()[:12], "enabled": true, "permissions": []string{"vendor.write"}}, 201)
	login := tc.call("POST", "/auth/admin", "", gin.H{"username": created["username"], "password": created["initial_password"]}, 200)
	operator := login["token"].(string)
	var imageBytes bytes.Buffer
	png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	image := resourceUpload(t, a, operator, "/admin/contents/"+number(v["id"])+"/media", "private.png", imageBytes.Bytes(), 201)
	var media Media
	a.DB.First(&media, uint64(image["id"].(float64)))
	if media.OwnerID == uint64(created["id"].(float64)) {
		t.Fatal("operator stole merchant ownership")
	}
	req := httptest.NewRequest("GET", "/api/v1/media/"+number(image["id"]), nil)
	req.Header.Set("Authorization", "Bearer "+operator)
	out := httptest.NewRecorder()
	a.router().ServeHTTP(out, req)
	if out.Code != 200 {
		t.Fatalf("uploader cannot preview unsaved scoped draft: %d", out.Code)
	}
	req = httptest.NewRequest("GET", "/api/v1/media/"+number(image["id"]), nil)
	out = httptest.NewRecorder()
	a.router().ServeHTTP(out, req)
	if out.Code != 404 {
		t.Fatal("private draft publicly readable")
	}
	created2 := tc.call("POST", "/admin/operators", admin, gin.H{"username": "unrelated-" + randomToken()[:12], "enabled": true, "permissions": []string{"product.read"}}, 201)
	login2 := tc.call("POST", "/auth/admin", "", gin.H{"username": created2["username"], "password": created2["initial_password"]}, 200)
	req = httptest.NewRequest("GET", "/api/v1/media/"+number(image["id"]), nil)
	req.Header.Set("Authorization", "Bearer "+login2["token"].(string))
	out = httptest.NewRecorder()
	a.router().ServeHTTP(out, req)
	if out.Code != 404 {
		t.Fatal("unrelated permission exposed draft media")
	}
	tc.call("POST", "/admin/operators/"+number(created["id"])+"/password-reset", admin, gin.H{}, 200)
	tc.call("GET", "/auth/me", operator, nil, 401)
}
func TestAnalyticsCrossDaySourceAndArchivedUV(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	admin := resourceTestSession(t, a, "admin")
	today := time.Now().In(shanghai)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, shanghai)
	for i, s := range []string{"web", "web", "android"} {
		occurred := today.Add(2 * time.Hour)
		if i == 0 {
			occurred = today.Add(-2 * time.Hour)
		}
		if err := a.DB.Create(&AnalyticsEvent{EventID: randomToken()[:36], VisitorHash: analyticHash([]byte("key"), s, "visitor"), SessionHash: analyticHash([]byte("key"), s, "session"), Source: s, EventName: "page_view", Page: "/", OccurredAt: occurred.UTC()}).Error; err != nil {
			t.Fatal(err)
		}
	}
	query := "?from=" + today.AddDate(0, 0, -1).Format("2006-01-02") + "&to=" + today.Format("2006-01-02")
	report := tc.call("GET", "/admin/analytics"+query, admin, nil, 200)
	summary := report["summary"].(map[string]any)
	if summary["pv"] != float64(3) || summary["uv"] != float64(2) {
		t.Fatalf("cross-day UV incorrectly summed: %#v", summary)
	}
	report = tc.call("GET", "/admin/analytics"+query+"&source=web", admin, nil, 200)
	summary = report["summary"].(map[string]any)
	if summary["pv"] != float64(2) || summary["uv"] != float64(1) {
		t.Fatal("source filter or cross-day dedup failed")
	}
	archived := today.AddDate(0, 0, -110)
	a.DB.Create(&AnalyticsEvent{EventID: randomToken()[:36], VisitorHash: "old", SessionHash: "old", Source: "web", EventName: "page_view", Page: "/", OccurredAt: archived.UTC()})
	a.aggregateAnalytics()
	report = tc.call("GET", "/admin/analytics?from="+archived.Format("2006-01-02")+"&to="+today.Format("2006-01-02"), admin, nil, 200)
	summary = report["summary"].(map[string]any)
	if report["detail_available"] != false || summary["uv"] != nil || summary["pv"] != float64(4) {
		t.Fatalf("archived report falsely claimed range UV or lost daily counts: %#v", report)
	}
	var count int64
	a.DB.Model(&AnalyticsEvent{}).Where("occurred_at<?", today.AddDate(0, 0, -90).UTC()).Count(&count)
	if count != 0 {
		t.Fatal("detail retention not enforced")
	}
}
func TestBackupChunkImportDownloadAndRevokedTicket(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	admin := resourceTestSession(t, a, "admin")
	data := bytes.Repeat([]byte{9}, backupUploadChunk+1024)
	copy(data, []byte(backupMagic))
	session := tc.call("POST", "/admin/backups/imports", admin, gin.H{"size": len(data)}, 201)
	uploadID := session["upload"].(map[string]any)["id"].(string)
	chunk := func(index int, body []byte, want int) {
		t.Helper()
		req := httptest.NewRequest("PUT", "/api/v1/admin/backups/imports/"+uploadID+"/chunks/"+strconv.Itoa(index), bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+admin)
		req.Header.Set("Content-Type", "application/octet-stream")
		out := httptest.NewRecorder()
		a.router().ServeHTTP(out, req)
		if out.Code != want {
			t.Fatalf("chunk %d: %d %s", index, out.Code, out.Body.String())
		}
	}
	chunk(1, data[:backupUploadChunk], 409)
	chunk(0, data[:backupUploadChunk], 200)
	chunk(0, data[:backupUploadChunk], 409)
	chunk(1, data[backupUploadChunk:], 200)
	tc.call("POST", "/admin/backups/imports/"+uploadID+"/complete", admin, gin.H{}, 201)
	req := httptest.NewRequest("POST", "/api/v1/admin/backups/"+uploadID+"/download-ticket", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer "+admin)
	req.Header.Set("Content-Type", "application/json")
	out := httptest.NewRecorder()
	a.router().ServeHTTP(out, req)
	if out.Code != 200 || len(out.Result().Cookies()) != 1 {
		t.Fatal("download ticket failed")
	}
	cookie := out.Result().Cookies()[0]
	if !cookie.HttpOnly {
		t.Fatal("ticket available to scripts")
	}
	get := func(want int) {
		t.Helper()
		req := httptest.NewRequest("GET", "/api/v1/backup-download/"+uploadID, nil)
		req.AddCookie(cookie)
		out := httptest.NewRecorder()
		a.router().ServeHTTP(out, req)
		if out.Code != want {
			t.Fatalf("download: %d", out.Code)
		}
		if want == 200 && !bytes.Equal(out.Body.Bytes(), data) {
			t.Fatal("streamed import/download differed")
		}
	}
	get(200)
	job, _, err := a.newJob("verify", restoreParams{uploadID}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if done := waitOperation(t, a, job.ID); done.State != "failed" {
		t.Fatal("corrupt archive passed integrity check")
	}
	tc.call("POST", "/auth/logout", admin, gin.H{}, 200)
	get(401)
}
func TestBackupCatalogLocationSurvivesDatabaseChanges(t *testing.T) {
	a := importTestApp(t)
	root := t.TempDir()
	profile := StorageProfile{Name: "备份独立位置", Kind: "local", LocalRoot: root}
	if err := a.DB.Create(&profile).Error; err != nil {
		t.Fatal(err)
	}
	store := localObjectStore{root}
	ctx := context.Background()
	if err := store.Put(ctx, "archives/test.zqbak", strings.NewReader("snapshot"), 8); err != nil {
		t.Fatal(err)
	}
	entry := BackupEntry{ID: "persisted", StoreID: profile.ID, Key: "archives/test.zqbak", Size: 8, SHA256: digest("snapshot"), Location: &BackupLocation{Profile: profile, Credentials: "encrypted-only"}}
	if err := a.saveBackupEntries([]BackupEntry{entry}); err != nil {
		t.Fatal(err)
	}
	if err := a.DB.Delete(&profile).Error; err != nil {
		t.Fatal(err)
	}
	entries, err := a.backupEntries()
	if err != nil || len(entries) != 1 {
		t.Fatal(err)
	}
	restoredStore, err := a.backupEntryStore(entries[0])
	if err != nil {
		t.Fatal(err)
	}
	if err = verifyObject(ctx, restoredStore, entry.Key, entry.SHA256, entry.Size); err != nil {
		t.Fatal("backup depended on switched database profiles", err)
	}
	public, _ := json.Marshal(entries)
	if bytes.Contains(public, []byte("encrypted-only")) || bytes.Contains(public, []byte(root)) {
		t.Fatal("private location leaked through backup API")
	}
}
func TestMigrationRejectsLocationAliasesAndChecksumFailure(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	admin := resourceTestSession(t, a, "admin")
	tc.call("POST", "/admin/storage/profiles", admin, gin.H{"name": "同一原目录", "kind": "local", "local_root": a.Config.MediaDir}, 409)
	key := "check-source.bin"
	os.WriteFile(filepath.Join(a.Config.MediaDir, key), []byte("correct"), 0600)
	profile := StorageProfile{Name: "迁移校验目标", Kind: "local", LocalRoot: t.TempDir()}
	a.DB.Create(&profile)
	os.WriteFile(filepath.Join(profile.LocalRoot, key), []byte("wrong"), 0600)
	job, _, err := a.newJob("migration", migrationParams{0, profile.ID}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if done := waitOperation(t, a, job.ID); done.State != "failed" || done.Error != "checksum" {
		t.Fatal("bad destination accepted", done.State, done.Error)
	}
	var object StoredObject
	if a.DB.First(&object, "object_key=?", key).Error != nil || object.StoreID != 0 {
		t.Fatal("failed checksum changed primary location")
	}
	os.Remove(filepath.Join(profile.LocalRoot, key))
	retry, _, err := a.newJob("migration", migrationParams{0, profile.ID}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if done := waitOperation(t, a, retry.ID); done.State != "succeeded" {
		t.Fatal("interrupted migration could not resume", done.Error)
	}
}
func TestBackupUsesScopedApplicationCredentials(t *testing.T) {
	a := importTestApp(t)
	if _, err := mysqlTool("mysqldump"); err != nil {
		t.Skip("mysql tools unavailable")
	}
	user := "zqb_" + randomToken()[:12]
	password := randomToken()
	database := a.currentDatabase()
	if err := a.DB.Exec("CREATE USER '" + user + "'@'%' IDENTIFIED BY '" + password + "'").Error; err != nil {
		t.Fatal(err)
	}
	defer a.DB.Exec("DROP USER IF EXISTS '" + user + "'@'%'")
	if err := a.DB.Exec("GRANT ALL PRIVILEGES ON `" + database + "`.* TO '" + user + "'@'%'").Error; err != nil {
		t.Fatal(err)
	}
	t.Setenv("DB_USER", user)
	t.Setenv("DB_PASSWORD", password)
	output := filepath.Join(t.TempDir(), "scoped.sql")
	if err := a.dumpDatabase(context.Background(), output); err != nil {
		t.Fatal("backup required privileges beyond its own database", err)
	}
	if stat, err := os.Stat(output); err != nil || stat.Size() == 0 {
		t.Fatal("empty SQL dump")
	}
}
