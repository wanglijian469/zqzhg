package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

func favoriteTestVisitor(t *testing.T, a *App, name string) (Account, string) {
	t.Helper()
	login := name
	account := Account{Username: "favorite-test:" + name, LoginName: &login, Role: "visitor"}
	if e := a.DB.Create(&account).Error; e != nil {
		t.Fatal(e)
	}
	session, e := a.newSession(account)
	if e != nil {
		t.Fatal(e)
	}
	return account, session["token"].(string)
}
func favoriteMutation(op, kind string, id any, selected bool) gin.H {
	return gin.H{"operation_id": op, "kind": kind, "content_id": id, "favorited": selected}
}
func TestVisitorRegistrationAndFavoritePermissions(t *testing.T) {
	a := importTestApp(t)
	clearRegistrationLimit()
	tc := testClient{t, a.router()}
	password := "Visitor-Password-2026!"
	out := tc.call("POST", "/auth/visitor/register", "", gin.H{"login_name": "普通访客", "password": password, "role": "merchant"}, 201)
	token := out["token"].(string)
	if out["account"].(map[string]any)["role"] != "visitor" {
		t.Fatal("visitor acquired a different role")
	}
	tc.call("POST", "/auth/visitor", "", gin.H{"username": "普通访客", "password": password}, 200)
	tc.call("POST", "/auth/merchant", "", gin.H{"username": "普通访客", "password": password}, 401)
	tc.call("POST", "/auth/merchant/register", "", gin.H{"login_name": "普通访客", "password": password}, 409)
	tc.call("POST", "/auth/visitor/register", "", gin.H{"login_name": "短密码", "password": "short"}, 400)
	tc.call("GET", "/merchant/contents", token, nil, 403)
	tc.fixtureCall("POST", "/merchant/contents", token, gin.H{"kind": "vendor"}, 403)
	tc.call("POST", "/merchant/media", token, nil, 403)
	tc.call("GET", "/admin/accounts", token, nil, 403)
	tc.call("GET", "/favorites", "", nil, 401)
	tc.call("GET", "/favorites", token, nil, 200)
	if e := migrate(a.DB); e != nil {
		t.Fatal("migration must be safely repeatable", e)
	}
}
func TestFavoriteDeduplicationAccountIsolationAndBatchRollback(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	account, token := favoriteTestVisitor(t, a, "one")
	_, other := favoriteTestVisitor(t, a, "two")
	add := favoriteMutation("deviceA_1", "vendor", 42, true)
	send := func(ops ...gin.H) map[string]any {
		return tc.call("POST", "/favorites/sync", token, gin.H{"operations": ops}, 200)
	}
	send(add)
	send(add)
	send(favoriteMutation("deviceB_1", "vendor", 42, false))
	send(add) // A late retry must not resurrect the deleted favorite.
	if tc.call("GET", "/favorites", token, nil, 200)["total"].(float64) != 0 {
		t.Fatal("retry resurrected a removed favorite")
	}
	send(favoriteMutation("deviceA_2", "vendor", 42, true), favoriteMutation("deviceA_3", "product", 43, true))
	if tc.call("GET", "/favorites", other, nil, 200)["total"].(float64) != 0 {
		t.Fatal("cross-account favorite leak")
	}
	session, e := a.newSession(account)
	if e != nil {
		t.Fatal(e)
	}
	if tc.call("GET", "/favorites?kind=vendor&page_size=1", session["token"].(string), nil, 200)["total"].(float64) != 1 {
		t.Fatal("second device did not see saved favorite")
	}
	tc.call("POST", "/favorites/sync", token, gin.H{"operations": []gin.H{
		favoriteMutation("new_in_failed_batch", "vendor", 99, true), favoriteMutation("deviceA_2", "vendor", 42, false),
	}}, 409)
	if tc.call("GET", "/favorites", token, nil, 200)["total"].(float64) != 2 {
		t.Fatal("failed batch was not rolled back")
	}
	tc.call("POST", "/favorites/sync", token, gin.H{"operations": []gin.H{{"operation_id": "missing_state", "kind": "vendor", "content_id": 42}}}, 400)
	tooMany := make([]gin.H, 101)
	for i := range tooMany {
		tooMany[i] = favoriteMutation(fmt.Sprintf("many_%d", i), "vendor", 42, true)
	}
	tc.call("POST", "/favorites/sync", token, gin.H{"operations": tooMany}, 400)
	tc.call("POST", "/favorites/sync", token, gin.H{"operations": []gin.H{favoriteMutation("invalid", "resource", 42, true)}}, 400)
	tc.call("GET", "/favorites?kind=resource", token, nil, 400)
	var count int64
	a.DB.Model(&FavoriteOperation{}).Where("account_id=?", account.ID).Count(&count)
	if count != 4 {
		t.Fatal("dedup or rollback ledger mismatch", count)
	}
}
func TestFavoriteResolutionNeverExposesDraftsOrWithdrawnParents(t *testing.T) {
	a := importTestApp(t)
	clearRegistrationLimit()
	tc := testClient{t, a.router()}
	merchant := tc.login("13900009401")
	admin := tc.call("POST", "/auth/admin", "", gin.H{"username": "admin", "password": "Import-Test-Admin-2026!"}, 200)["token"].(string)
	vendor := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "公开厂商", RegionName: "河北", MainProducts: "机械", Phone: "0319-1234567"}}, 201)
	product := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "product", "data": Payload{Name: "公开产品", CategoryID: 1}}, 201)
	draft := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "product", "data": Payload{Name: "PRIVATE_SECRET_DRAFT", CategoryID: 1}}, 201)
	for _, item := range []map[string]any{vendor, product} {
		tc.call("POST", "/merchant/contents/"+number(item["id"])+"/submit", merchant, gin.H{"revision_id": item["revision_id"]}, 200)
		tc.call("POST", "/admin/revisions/"+number(item["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	}
	targets := []gin.H{{"kind": "vendor", "content_id": vendor["id"]}, {"kind": "product", "content_id": product["id"]}, {"kind": "product", "content_id": draft["id"]}, {"kind": "product", "content_id": 999999}}
	resolve := func() []any {
		return tc.call("POST", "/public/favorites/resolve", "", gin.H{"items": targets}, 200)["items"].([]any)
	}
	items := resolve()
	for i, item := range items {
		row := item.(map[string]any)
		if row["available"] != (i < 2) {
			t.Fatal("wrong public availability", row)
		}
		if i >= 2 && row["item"] != nil {
			t.Fatal("private payload in unavailable item")
		}
	}
	raw, _ := json.Marshal(items)
	if strings.Contains(string(raw), "PRIVATE_SECRET_DRAFT") {
		t.Fatal("draft leaked")
	}
	// Old public snapshots remain selected while a new private revision is edited.
	tc.fixtureCall("PUT", "/merchant/contents/"+number(vendor["id"])+"/draft", merchant, gin.H{"revision_id": vendor["revision_id"], "data": Payload{Name: "PRIVATE_NEW_VENDOR", RegionName: "河北", MainProducts: "机械", Phone: "0319-1234567"}}, 200)
	raw, _ = json.Marshal(resolve())
	if strings.Contains(string(raw), "PRIVATE_NEW_VENDOR") {
		t.Fatal("unpublished edit leaked")
	}
	tc.call("POST", "/merchant/contents/"+number(vendor["id"])+"/offline", merchant, gin.H{}, 200)
	for _, item := range resolve() {
		if item.(map[string]any)["available"] != false {
			t.Fatal("withdrawn parent leaked")
		}
	}
	tc.call("POST", "/favorites/sync", merchant, gin.H{"operations": []gin.H{favoriteMutation("merchant_favorite", "vendor", vendor["id"], true)}}, 200)
	saved := tc.call("GET", "/favorites", merchant, nil, 200)
	if saved["total"].(float64) != 1 || saved["items"].([]any)[0].(map[string]any)["available"] != false {
		t.Fatal("unavailable favorites should remain removable")
	}
}
func TestConcurrentFavoriteRetriesExecuteOnce(t *testing.T) {
	a := importTestApp(t)
	account, token := favoriteTestVisitor(t, a, "concurrent")
	router := a.router()
	body, _ := json.Marshal(gin.H{"operations": []gin.H{favoriteMutation("same_operation", "product", 123, true)}})
	var wg sync.WaitGroup
	codes := make(chan int, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("POST", "/api/v1/favorites/sync", bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			codes <- w.Code
		}()
	}
	wg.Wait()
	close(codes)
	for code := range codes {
		if code != 200 {
			t.Fatal("concurrent retry failed", code)
		}
	}
	var count int64
	a.DB.Model(&FavoriteOperation{}).Where("account_id=?", account.ID).Count(&count)
	if count != 1 {
		t.Fatal("operation executed more than once", count)
	}
}
