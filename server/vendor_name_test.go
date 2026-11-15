package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestVendorNameCreateAcrossAccounts(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	first := tc.login("13900171001")
	second := tc.login("13900171002")
	name := "河北春耕机械制造有限公司"
	tc.call("POST", "/merchant/contents", first, gin.H{"kind": "vendor", "data": Payload{Name: name}}, 201)
	for _, candidate := range []string{name, " 河北春耕机械制造有限公司 ", "河北春耕 机械制造有限公司", "河北春耕\u200b机械制造有限公司"} {
		got := tc.call("POST", "/merchant/contents", second, gin.H{"kind": "vendor", "data": Payload{Name: candidate}}, 409)
		problem := got["error"].(map[string]any)
		if problem["code"] != "vendor_name_exists" || problem["fields"].([]any)[0].(map[string]any)["field"] != "name" {
			t.Fatal(got)
		}
	}
	// The guard is about company names, rather than login names or a single account.
	var admin Account
	a.DB.Where("role='admin'").First(&admin)
	session, err := a.newSession(admin)
	if err != nil {
		t.Fatal(err)
	}
	var before, after int64
	a.DB.Model(&Account{}).Count(&before)
	tc.fixtureCall("POST", "/admin/vendors", session["token"].(string), gin.H{"login_name": "不同登录账号", "data": Payload{Name: name, RegionName: "河北", MainProducts: "农机", Phone: "0319-1234567"}}, 409)
	a.DB.Model(&Account{}).Count(&after)
	if before != after {
		t.Fatal("duplicate creation leaked an account")
	}
	tc.call("POST", "/merchant/contents", second, gin.H{"kind": "vendor", "data": Payload{Name: name + "分公司"}}, 201)
}

func TestVendorNameRenameAndExistingDuplicates(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	first, second := tc.login("13900171003"), tc.login("13900171004")
	one := tc.call("POST", "/merchant/contents", first, gin.H{"kind": "vendor", "data": Payload{Name: "名称保留企业"}}, 201)
	two := tc.call("POST", "/merchant/contents", second, gin.H{"kind": "vendor", "data": Payload{Name: "另一家企业"}}, 201)
	path := "/merchant/contents/" + number(two["id"]) + "/draft"
	tc.call("PUT", path, second, gin.H{"revision_id": two["revision_id"], "data": Payload{Name: "名称保留企业"}}, 409)
	var content Content
	a.DB.First(&content, uint64(two["id"].(float64)))
	if content.DraftRevisionID != uint64(two["revision_id"].(float64)) {
		t.Fatal("failed rename changed draft")
	}
	// Model pre-fix duplicate records explicitly. Existing records must remain editable.
	var rev Revision
	a.DB.First(&rev, content.DraftRevisionID)
	var payload Payload
	json.Unmarshal([]byte(rev.Payload), &payload)
	payload.Name = "名称保留企业"
	blob, _ := json.Marshal(payload)
	if err := a.DB.Model(&rev).Updates(map[string]any{"name": payload.Name, "payload": string(blob)}).Error; err != nil {
		t.Fatal(err)
	}
	payload.Description = "只修改介绍，不新增重复企业"
	tc.call("PUT", path, second, gin.H{"revision_id": rev.ID, "data": payload}, 200)
	// Changing a name away from a legacy duplicate must also be possible.
	tc.call("PUT", "/merchant/contents/"+number(one["id"])+"/draft", first, gin.H{"revision_id": one["revision_id"], "data": Payload{Name: "独立的新企业名称"}}, 200)
}

func TestVendorNameConcurrentCreate(t *testing.T) {
	a := importTestApp(t)
	router := a.router()
	tc := testClient{t, router}
	tokens := []string{tc.login("13900171005"), tc.login("13900171006")}
	start := make(chan struct{})
	results := make(chan int, 2)
	for _, token := range tokens {
		go func(token string) {
			<-start
			body, _ := json.Marshal(gin.H{"kind": "vendor", "data": Payload{Name: "并发重复名称企业"}})
			req := httptest.NewRequest("POST", "/api/v1/merchant/contents", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			results <- w.Code
		}(token)
	}
	close(start)
	counts := map[int]int{}
	for i := 0; i < 2; i++ {
		counts[<-results]++
	}
	if counts[201] != 1 || counts[409] != 1 {
		t.Fatal(fmt.Sprint(counts))
	}
}

func TestVendorNameNormalization(t *testing.T) {
	for _, pair := range [][2]string{
		{"河北春耕机械制造有限公司", " 河北春耕\u3000机械\u200b制造有限公司\ufeff"},
		{"ABC机械(河北)有限公司", "ａｂｃ机械（河北）有限公司"},
		{"Café机械", "Cafe\u0301机械"},
	} {
		if vendorNameKey(pair[0]) != vendorNameKey(pair[1]) {
			t.Fatal(pair)
		}
	}
	if vendorNameKey("河北春耕机械制造有限公司") == vendorNameKey("河北春耕机械制造有限公司分公司") {
		t.Fatal("branch suffix must remain significant")
	}
}

func TestVendorNameReservesPublicAndDraftButNotHistory(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	first, second := tc.login("13900171007"), tc.login("13900171008")
	one := tc.call("POST", "/merchant/contents", first, gin.H{"kind": "vendor", "data": Payload{Name: "原公开企业名称"}}, 201)
	contentID := uint64(one["id"].(float64))
	oldRevision := uint64(one["revision_id"].(float64))
	// A historical public record remains reserved even when it is taken offline.
	a.DB.Model(&Content{}).Where("id=?", contentID).Updates(map[string]any{"published_revision_id": oldRevision, "visible": false})
	changed := tc.call("PUT", "/merchant/contents/"+number(one["id"])+"/draft", first, gin.H{"revision_id": oldRevision, "data": Payload{Name: "拟用新企业名称"}}, 200)
	for _, name := range []string{"原公开企业名称", "拟用新企业名称"} {
		tc.call("POST", "/merchant/contents", second, gin.H{"kind": "vendor", "data": Payload{Name: name}}, 409)
	}
	// Product names have no company-name uniqueness rule.
	tc.call("POST", "/merchant/contents", first, gin.H{"kind": "product", "data": Payload{Name: "拟用新企业名称"}}, 201)
	// Once a name is no longer on either current pointer, revision history alone
	// must not reserve it forever.
	a.DB.Model(&Content{}).Where("id=?", contentID).Update("published_revision_id", uint64(changed["revision_id"].(float64)))
	tc.call("POST", "/merchant/contents", second, gin.H{"kind": "vendor", "data": Payload{Name: "原公开企业名称"}}, 201)
}

func TestVendorNameImportCannotBypassGuard(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	token := tc.login("13900171009")
	name := "已经整理过的企业"
	tc.call("POST", "/merchant/contents", token, gin.H{"kind": "vendor", "data": Payload{Name: name}}, 201)
	root := t.TempDir()
	manifest := importManifest{SourceKey: "V171009", SourceHash: strings.Repeat("a", 64), SourceRoot: root, Vendor: importItem{Data: Payload{Name: name, MainProducts: "机械", RegionName: "河北", Phone: "0319-1234567"}}}
	blob, _ := json.Marshal(manifest)
	path := filepath.Join(root, "import.json")
	if err := os.WriteFile(path, blob, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.importVendor(path); err == nil || err.Error() != "vendor_name_exists" {
		t.Fatal(err)
	}
	var count int64
	a.DB.Model(&Account{}).Where("username=?", "import:"+manifest.SourceKey).Count(&count)
	if count != 0 {
		t.Fatal("failed duplicate import leaked an account")
	}
}
