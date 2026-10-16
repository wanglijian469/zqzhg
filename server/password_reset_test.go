package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"image"
	"image/png"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func resetFixture(t *testing.T) (*App, *gin.Engine, string, Account, map[string]any, string) {
	t.Helper()
	a := importTestApp(t)
	authLimits.Lock()
	authLimits.entries = map[string][]time.Time{}
	authLimits.Unlock()
	admin := resourceTestSession(t, a, "admin")
	r := a.router()
	tc := testClient{t, r}
	created := tc.call("POST", "/admin/vendors", admin, gin.H{"login_name": "重置密码测试公司", "data": Payload{Name: "重置密码测试公司", RegionName: "河北", MainProducts: "液压机械", Phone: "0319-1234567"}}, 201)
	var owner Account
	a.DB.First(&owner, uint64(created["account_id"].(float64)))
	return a, r, admin, owner, created["item"].(map[string]any), created["initial_password"].(string)
}
func resetRequest(r *gin.Engine, path, token string, data any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(data)
	req := httptest.NewRequest("POST", "/api/v1"+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
func TestPasswordResetPermissionsSessionsAndOwnership(t *testing.T) {
	a, r, admin, owner, vendor, oldPassword := resetFixture(t)
	tc := testClient{t, r}
	path := "/admin/accounts/" + strconv.FormatUint(owner.ID, 10) + "/password-reset"
	session, _ := a.newSession(owner)
	old := session["token"].(string)
	second, _ := a.newSession(owner)
	product := tc.call("POST", "/merchant/contents", old, gin.H{"kind": "product", "data": Payload{Name: "既有液压产品", CategoryID: 1}}, 201)
	var imageBytes bytes.Buffer
	png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	image := resourceUpload(t, a, old, "/merchant/media", "original.png", imageBytes.Bytes(), 201)
	tc.call("PUT", "/merchant/contents/"+number(vendor["id"])+"/draft", old, gin.H{"revision_id": vendor["revision_id"], "data": Payload{Name: "重置密码测试公司", RegionName: "河北", MainProducts: "液压机械", Phone: "0319-1234567", MediaIDs: []uint64{uint64(image["id"].(float64))}}}, 200)
	var before []Revision
	a.DB.Order("id").Find(&before)
	tc.call("POST", path, "", gin.H{"mode": "generate"}, 401)
	tc.call("POST", path, old, gin.H{"mode": "generate"}, 403)
	var administrator Account
	a.DB.Where("role='admin'").First(&administrator)
	tc.call("POST", "/admin/accounts/"+strconv.FormatUint(administrator.ID, 10)+"/password-reset", admin, gin.H{"mode": "generate"}, 404)
	tc.call("POST", "/admin/accounts/999999/password-reset", admin, gin.H{"mode": "generate"}, 404)
	sms := Account{Username: "13900007530", Role: "merchant"}
	a.DB.Create(&sms)
	tc.call("POST", "/admin/accounts/"+strconv.FormatUint(sms.ID, 10)+"/password-reset", admin, gin.H{"mode": "generate"}, 400)
	tc.call("GET", "/admin/accounts?account_id="+strconv.FormatUint(owner.ID, 10), admin, nil, 200)
	tc.call("GET", "/admin/accounts?account_id=not-a-number", admin, nil, 400)
	out := resetRequest(r, path, admin, gin.H{"mode": "generate"})
	if out.Code != 200 || out.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(out.Code, out.Header())
	}
	var result map[string]any
	json.Unmarshal(out.Body.Bytes(), &result)
	newPassword := result["password"].(string)
	if newPassword == oldPassword || result["revoked_sessions"] != float64(2) || result["login_name"] != *owner.LoginName {
		t.Fatal("bad reset result")
	}
	tc.call("GET", "/merchant/contents/"+number(vendor["id"]), old, nil, 401)
	tc.call("GET", "/merchant/contents/"+number(product["id"]), second["token"].(string), nil, 401)
	tc.call("POST", "/auth/merchant", "", gin.H{"username": *owner.LoginName, "password": oldPassword}, 401)
	fresh := tc.call("POST", "/auth/merchant", "", gin.H{"username": *owner.LoginName, "password": newPassword}, 200)["token"].(string)
	tc.call("GET", "/merchant/contents/"+number(product["id"]), fresh, nil, 200)
	var after []Revision
	a.DB.Order("id").Find(&after)
	b, _ := json.Marshal(before)
	c, _ := json.Marshal(after)
	if !bytes.Equal(b, c) {
		t.Fatal("content/review revisions changed")
	}
	var media Media
	a.DB.First(&media, uint64(image["id"].(float64)))
	if media.OwnerID != owner.ID {
		t.Fatal("image ownership changed")
	}
	var stored Account
	a.DB.First(&stored, owner.ID)
	if stored.PasswordHash == newPassword || bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte(newPassword)) != nil || stored.LoginName == nil || *stored.LoginName != *owner.LoginName {
		t.Fatal("password or login name changed incorrectly")
	}
	for _, p := range []string{"/admin/accounts", "/admin/operation-logs"} {
		v := tc.call("GET", p, admin, nil, 200)
		raw, _ := json.Marshal(v)
		if strings.Contains(string(raw), newPassword) || strings.Contains(string(raw), stored.PasswordHash) {
			t.Fatal("secret leaked")
		}
	}
	var log OperationLog
	if a.DB.Where("action='account_password_reset' AND target_id=?", owner.ID).First(&log).Error != nil {
		t.Fatal("missing reset audit")
	}
	if log.TargetKind != "account" || log.Note != *owner.LoginName {
		t.Fatal("incorrect audit")
	}
}

func TestManualPasswordResetBytesAndRollback(t *testing.T) {
	a, r, admin, owner, _, oldPassword := resetFixture(t)
	tc := testClient{t, r}
	path := "/admin/accounts/" + strconv.FormatUint(owner.ID, 10) + "/password-reset"
	for _, password := range []string{"short", strings.Repeat(" ", 12), strings.Repeat("中", 25), strings.Repeat("😀", 19)} {
		tc.call("POST", path, admin, gin.H{"mode": "manual", "password": password}, 400)
	}
	tc.call("POST", path, admin, gin.H{"mode": "unknown"}, 400)
	tc.call("POST", path, admin, gin.H{"mode": "generate", "password": "unexpected"}, 400)
	for _, password := range []string{strings.Repeat("中", 4), strings.Repeat("中", 24), strings.Repeat("😀", 18), "  Keep-Spaces-123  "} {
		tc.call("POST", path, admin, gin.H{"mode": "manual", "password": password}, 200)
		var stored Account
		a.DB.First(&stored, owner.ID)
		if bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte(password)) != nil {
			t.Fatal("manual password not preserved")
		}
	}
	// A failed audit must undo both the hash change and session deletion.
	session, _ := a.newSession(owner)
	var before Account
	a.DB.First(&before, owner.ID)
	a.DB.Callback().Create().Before("gorm:create").Register("reset_audit_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "operation_logs" {
			tx.AddError(errors.New("injected audit failure"))
		}
	})
	defer a.DB.Callback().Create().Remove("reset_audit_failure")
	tc.call("POST", path, admin, gin.H{"mode": "manual", "password": oldPassword}, 500)
	var after Account
	a.DB.First(&after, owner.ID)
	if before.PasswordHash != after.PasswordHash {
		t.Fatal("hash not rolled back")
	}
	tc.call("GET", "/merchant/contents", session["token"].(string), nil, 200)
}

func TestPasswordResetSerializesConcurrentLogin(t *testing.T) {
	a, r, admin, owner, _, password := resetFixture(t)
	paused := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	a.DB.Callback().Create().Before("gorm:create").Register("pause_company_session", func(tx *gorm.DB) {
		if tx.Statement.Table == "sessions" {
			once.Do(func() { close(paused); <-release })
		}
	})
	defer a.DB.Callback().Create().Remove("pause_company_session")
	loginDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		loginDone <- resetRequest(r, "/auth/merchant", "", gin.H{"username": *owner.LoginName, "password": password})
	}()
	select {
	case <-paused:
	case <-time.After(5 * time.Second):
		t.Fatal("login did not reach session creation")
	}
	resetDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		resetDone <- resetRequest(r, "/admin/accounts/"+strconv.FormatUint(owner.ID, 10)+"/password-reset", admin, gin.H{"mode": "generate"})
	}()
	select {
	case <-resetDone:
		t.Fatal("reset bypassed account lock")
	case <-time.After(150 * time.Millisecond):
	}
	releaseOnce.Do(func() { close(release) })
	var login, reset *httptest.ResponseRecorder
	select {
	case login = <-loginDone:
	case <-time.After(5 * time.Second):
		t.Fatal("login timeout")
	}
	select {
	case reset = <-resetDone:
	case <-time.After(5 * time.Second):
		t.Fatal("reset timeout")
	}
	if login.Code != 200 || reset.Code != 200 {
		t.Fatal(login.Code, reset.Code)
	}
	var sessions int64
	a.DB.Model(&Session{}).Where("account_id=?", owner.ID).Count(&sessions)
	if sessions != 0 {
		t.Fatal("pre-reset login session survived")
	}
	var loginData map[string]any
	json.Unmarshal(login.Body.Bytes(), &loginData)
	testClient{t, r}.call("GET", "/merchant/contents", loginData["token"].(string), nil, 401)
}
