package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminOperationsOwnershipAuditAndFilters(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	var adminAccount Account
	a.DB.Where("role='admin'").First(&adminAccount)
	session, e := a.newSession(adminAccount)
	if e != nil {
		t.Fatal(e)
	}
	admin := session["token"].(string)
	accountToken := tc.login("13900003001")
	var unassigned Account
	a.DB.Where("username=?", "13900003001").First(&unassigned)
	payload := Payload{Name: "后台代录测试厂商（仅测试）", RegionName: "后台测试地区", MainProducts: "犁具", Phone: "13900003001"}
	created := tc.fixtureCall("POST", "/admin/vendors", admin, gin.H{"login_name": payload.Name, "data": payload}, 201)
	password := created["initial_password"].(string)
	vendor := created["item"].(map[string]any)
	vendorID := number(vendor["id"])
	owner := uint64(created["account_id"].(float64))
	var account Account
	a.DB.First(&account, owner)
	if account.PasswordHash == password || bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(password)) != nil {
		t.Fatal("must store valid hash only")
	}
	merchant := tc.call("POST", "/auth/merchant", "", gin.H{"username": payload.Name, "password": password}, 200)["token"].(string)
	tc.call("GET", "/merchant/contents/"+vendorID, merchant, nil, 200)
	tc.fixtureCall("POST", "/admin/vendors", admin, gin.H{"login_name": payload.Name, "data": payload}, 409)
	var before int64
	a.DB.Model(&Account{}).Count(&before)
	tc.fixtureCall("POST", "/admin/vendors", admin, gin.H{"login_name": "失败事务测试", "data": Payload{Name: "资料不完整"}}, 400)
	var after int64
	a.DB.Model(&Account{}).Count(&after)
	if before != after {
		t.Fatal("failed vendor creation leaked account")
	}
	linked := tc.fixtureCall("POST", "/admin/vendors", admin, gin.H{"account_id": unassigned.ID, "data": payload}, 201)
	if _, ok := linked["initial_password"]; ok {
		t.Fatal("existing account must not get a password")
	}
	tc.fixtureCall("POST", "/admin/vendors", admin, gin.H{"account_id": unassigned.ID, "data": payload}, 409)
	tc.call("GET", "/admin/accounts?unassigned=1", admin, nil, 200)
	tc.call("GET", "/merchant/contents/"+number(linked["item"].(map[string]any)["id"]), accountToken, nil, 200)
	product := tc.fixtureCall("POST", "/admin/vendors/"+vendorID+"/products", admin, gin.H{"data": Payload{Name: "ADMIN-440 测试产品", CategoryID: 1}}, 201)
	productID := number(product["id"])
	if uint64(product["owner_id"].(float64)) != owner {
		t.Fatal("administrator became product owner")
	}
	// Upload through the target route; a caller-supplied owner is ignored.
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)
	writer.WriteField("owner_id", fmt.Sprint(adminAccount.ID))
	part, _ := writer.CreateFormFile("file", "test.png")
	png.Encode(part, image.NewRGBA(image.Rect(0, 0, 20, 20)))
	writer.Close()
	req := httptest.NewRequest("POST", "/api/v1/admin/contents/"+productID+"/media", &b)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+admin)
	w := httptest.NewRecorder()
	tc.router.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var uploaded map[string]any
	json.Unmarshal(w.Body.Bytes(), &uploaded)
	mediaID := uint64(uploaded["id"].(float64))
	var media Media
	a.DB.First(&media, mediaID)
	if media.OwnerID != owner {
		t.Fatal("wrong media owner")
	}
	pp := Payload{Name: "ADMIN-440 测试产品", CategoryID: 1, MediaIDs: []uint64{mediaID}}
	saved := tc.fixtureCall("PUT", "/admin/contents/"+productID+"/draft", admin, gin.H{"revision_id": product["revision_id"], "data": pp}, 200)
	tc.fixtureCall("PUT", "/admin/contents/"+productID+"/draft", admin, gin.H{"revision_id": product["revision_id"], "data": pp}, 409)
	foreign := Media{OwnerID: unassigned.ID, Path: "foreign", ThumbPath: "foreign"}
	a.DB.Create(&foreign)
	pp.MediaIDs = []uint64{foreign.ID}
	tc.fixtureCall("PUT", "/admin/contents/"+productID+"/draft", admin, gin.H{"revision_id": saved["revision_id"], "data": pp}, 400)
	pp.MediaIDs = []uint64{mediaID}
	tc.call("POST", "/admin/contents/"+productID+"/submit", admin, gin.H{"revision_id": saved["revision_id"]}, 200)
	tc.fixtureCall("PUT", "/admin/contents/"+productID+"/draft", admin, gin.H{"revision_id": saved["revision_id"], "data": pp}, 409)
	tc.call("POST", "/admin/revisions/"+number(saved["revision_id"])+"/review", admin, gin.H{"decision": "reject", "note": ""}, 400)
	tc.call("POST", "/admin/revisions/"+number(saved["revision_id"])+"/review", admin, gin.H{"decision": "reject", "note": "请补充资料"}, 200)
	saved = tc.fixtureCall("PUT", "/merchant/contents/"+productID+"/draft", merchant, gin.H{"revision_id": saved["revision_id"], "data": pp}, 200)
	publish := func(contentID string, revision any) {
		tc.call("POST", "/admin/contents/"+contentID+"/submit", admin, gin.H{"revision_id": revision}, 200)
		tc.call("POST", "/admin/revisions/"+number(revision)+"/review", admin, gin.H{"decision": "approve"}, 200)
	}
	publish(productID, saved["revision_id"])
	tc.call("GET", "/public/products/"+productID, "", nil, 404)
	tc.call("PUT", "/admin/contents/"+productID+"/featured", admin, gin.H{"featured": true}, 400)
	publish(vendorID, vendor["revision_id"])
	tc.call("GET", "/public/products/"+productID, "", nil, 200)
	tc.call("PUT", "/admin/contents/"+productID+"/featured", admin, gin.H{"featured": true, "sort_order": 7}, 200)
	detail := tc.call("GET", "/admin/contents/"+productID, admin, nil, 200)
	if detail["published"] == nil || detail["vendor"] == nil {
		t.Fatal("missing comparison / manufacturer")
	}
	pp.Name = "ADMIN-550 新草稿"
	edited := tc.fixtureCall("PUT", "/admin/contents/"+productID+"/draft", admin, gin.H{"revision_id": saved["revision_id"], "data": pp}, 200)
	tc.call("POST", "/admin/contents/"+productID+"/submit", admin, gin.H{"revision_id": edited["revision_id"]}, 200)
	public := tc.call("GET", "/public/products/"+productID, "", nil, 200)
	if public["item"].(map[string]any)["data"].(map[string]any)["name"] != "ADMIN-440 测试产品" {
		t.Fatal("pending replaced public snapshot")
	}
	region := tc.call("GET", "/admin/contents/"+vendorID, admin, nil, 200)["data"].(map[string]any)["region_id"]
	path := "/admin/contents?kind=product&q=ADMIN-550&category_id=1&vendor_id=" + vendorID + "&region_id=" + number(region) + "&status=pending&public_state=public&featured=1&page_size=1"
	if tc.call("GET", path, admin, nil, 200)["total"].(float64) != 1 {
		t.Fatal("combined filter failed")
	}
	if tc.call("GET", path+"&page=2", admin, nil, 200)["items"].([]any) == nil {
		t.Fatal("empty pagination must be array")
	}
	tc.call("POST", "/admin/contents/"+vendorID+"/offline", admin, gin.H{}, 200)
	if tc.call("GET", path, admin, nil, 200)["total"].(float64) != 0 {
		t.Fatal("offline booth still public")
	}
	tc.call("PUT", "/admin/contents/"+productID+"/featured", admin, gin.H{"featured": true}, 400)
	counts := tc.call("GET", "/admin/dashboard", admin, nil, 200)["counts"].(map[string]any)
	if counts["product_public"].(float64) != 0 || counts["vendor_total"].(float64) != 2 {
		t.Fatal("statistics inconsistent")
	}
	tc.call("POST", "/admin/categories", admin, gin.H{"name": "子分类（测试）", "parent_id": 1, "enabled": true}, 200)
	tc.call("POST", "/admin/regions", admin, gin.H{"name": "新地区（测试）", "name_en": "Test", "enabled": true}, 200)
	for _, path := range []string{"/admin/accounts", "/admin/operation-logs", "/admin/system", "/admin/dashboard", "/admin/contents/" + vendorID} {
		out := tc.call("GET", path, admin, nil, 200)
		raw, _ := json.Marshal(out)
		for _, secret := range []string{password, account.PasswordHash, admin, merchant, "access_key_secret", "password_hash", "initial_password"} {
			if strings.Contains(string(raw), secret) {
				t.Fatalf("secret leaked by %s", path)
			}
		}
		tc.call("GET", path, "", nil, 401)
		tc.call("GET", path, merchant, nil, 403)
	}
	var logs []OperationLog
	a.DB.Find(&logs)
	seen := map[string]bool{}
	for _, log := range logs {
		seen[log.Action] = true
	}
	for _, action := range []string{"account_created", "account_linked", "content_created", "media_uploaded", "draft_saved", "submit", "approve", "reject", "admin_offline", "featured_changed", "category_saved", "region_saved"} {
		if !seen[action] {
			t.Fatal("missing audit", action)
		}
	}
	// A migration retry must not duplicate existing review history.
	var count int64
	a.DB.Model(&OperationLog{}).Count(&count)
	a.DB.Exec("DELETE FROM schema_migrations WHERE version=3")
	if e = migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	var newCount int64
	a.DB.Model(&OperationLog{}).Count(&newCount)
	if count != newCount {
		t.Fatal("review import duplicated")
	}
}

func TestAdminQuotaAndCategoryCycle(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	var account Account
	a.DB.Where("role='admin'").First(&account)
	session, _ := a.newSession(account)
	admin := session["token"].(string)
	v := tc.fixtureCall("POST", "/admin/vendors", admin, gin.H{"login_name": "配额测试公司", "data": Payload{Name: "配额测试公司", RegionName: "测试地区", MainProducts: "机械", Wechat: "test"}}, 201)["item"].(map[string]any)
	owner := uint64(v["owner_id"].(float64))
	vendorID := uint64(v["id"].(float64))
	fixtures := make([]Content, maxVendorProducts)
	for i := range fixtures {
		fixtures[i] = Content{Kind: "product", OwnerID: owner, VendorID: vendorID}
	}
	if e := a.DB.CreateInBatches(fixtures, 100).Error; e != nil {
		t.Fatal(e)
	}
	tc.fixtureCall("POST", "/admin/vendors/"+number(v["id"])+"/products", admin, gin.H{"data": Payload{Name: "第1001款", CategoryID: 1}}, 409)
	child := tc.call("POST", "/admin/categories", admin, gin.H{"name": "下级（测试）", "parent_id": 1, "enabled": true}, 200)
	tc.call("PUT", "/admin/categories/1", admin, gin.H{"name": "父类（测试）", "parent_id": child["id"], "enabled": true}, 400)
	tc.fixtureCall("POST", "/admin/vendors", "", gin.H{}, 401)
	merchant := tc.login("13900003002")
	tc.fixtureCall("POST", "/admin/vendors", merchant, gin.H{}, 403)
}
