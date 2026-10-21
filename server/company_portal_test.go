package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPortalFieldLimitsAndLegacyMerge(t *testing.T) {
	p := Payload{Name: "企业", MainProducts: "机械", RegionName: "河北", Phone: "123", MediaIDs: []uint64{1}, CompanyProfile: &CompanyProfile{Credentials: []CompanyCredential{{GalleryAnnotation: GalleryAnnotation{MediaID: 2, Title: strings.Repeat("😀", 100)}, Kind: "qualification"}}, GalleryAnnotations: []GalleryAnnotation{{MediaID: 1, Title: "车间"}}}}
	if got := payloadFieldIssues("vendor", p); len(got) != 0 {
		t.Fatal(got)
	}
	p.CompanyProfile.Credentials[0].Title += "😀"
	found := false
	for _, e := range payloadFieldIssues("vendor", p) {
		if e.Field == "company_profile.credentials.0.title" && e.Limit == 100 {
			found = true
		}
	}
	if !found {
		t.Fatal("Unicode boundary missing")
	}
	legacy := Payload{MediaIDs: []uint64{1}, CompanyProfile: &CompanyProfile{Summary: "old app"}}
	mergePortalProfile(&legacy, p)
	if len(legacy.CompanyProfile.Credentials) != 1 || len(legacy.CompanyProfile.GalleryAnnotations) != 1 {
		t.Fatal("legacy save lost metadata")
	}
	clear := Payload{CompanyProfile: &CompanyProfile{Credentials: []CompanyCredential{}, GalleryAnnotations: []GalleryAnnotation{}}}
	mergePortalProfile(&clear, p)
	if len(clear.CompanyProfile.Credentials) != 0 || len(clear.CompanyProfile.GalleryAnnotations) != 0 {
		t.Fatal("explicit clear ignored")
	}
	legacy.MediaIDs = nil
	mergePortalProfile(&legacy, p)
	if len(legacy.CompanyProfile.GalleryAnnotations) != 0 {
		t.Fatal("deleted photo caption survived")
	}
	p.CompanyProfile.Credentials[0].Title = ""
	p.CompanyProfile.Credentials[0].Kind = "unknown"
	if len(payloadFieldIssues("vendor", p)) < 2 {
		t.Fatal("required title/kind not validated")
	}
	p.CompanyProfile.Credentials = make([]CompanyCredential, 21)
	if len(payloadFieldIssues("vendor", p)) == 0 {
		t.Fatal("quota missing")
	}
}

func TestPortalReviewImagesIsolationAndOldClient(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	merchant := tc.login("13900009201")
	other := tc.login("13900009202")
	admin := resourceTestSession(t, a, "admin")
	var owner Account
	a.DB.Where("username=?", "13900009201").First(&owner)
	image := Media{OwnerID: owner.ID, Path: "credential.png", ThumbPath: "credential-thumb.png"}
	a.DB.Create(&image)
	os.WriteFile(filepath.Join(a.Config.MediaDir, image.Path), []byte("test-image"), 0600)
	read := func(token string, status int) {
		t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/media/"+fmt.Sprint(image.ID), nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		tc.router.ServeHTTP(w, req)
		if w.Code != status {
			t.Fatalf("media: got %d want %d", w.Code, status)
		}
	}
	gallery := Media{OwnerID: owner.ID, Path: "gallery.png", ThumbPath: "gallery-thumb.png"}
	a.DB.Create(&gallery)
	d := Payload{Name: "企业门户测试", MainProducts: "机械", Phone: "123", RegionName: "河北", MediaIDs: []uint64{gallery.ID}, CompanyProfile: &CompanyProfile{Credentials: []CompanyCredential{{GalleryAnnotation: GalleryAnnotation{MediaID: image.ID, Title: "原公开证书"}, Kind: "qualification"}}, GalleryAnnotations: []GalleryAnnotation{{MediaID: gallery.ID, Title: "原车间"}}}}
	tc.fixtureCall("POST", "/merchant/contents", other, gin.H{"kind": "vendor", "data": d}, 400)
	v := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": d}, 201)
	path := "/merchant/contents/" + number(v["id"])
	read("", 404)
	read(other, 404)
	read(merchant, 200)
	publish := func(v map[string]any) {
		tc.call("POST", path+"/submit", merchant, gin.H{"revision_id": v["revision_id"]}, 200)
		tc.call("POST", "/admin/revisions/"+number(v["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	}
	publish(v)
	read("", 200)
	legacy := d
	legacy.CompanyProfile = &CompanyProfile{Summary: "旧App修改"}
	v = tc.fixtureCall("PUT", path+"/draft", merchant, gin.H{"revision_id": v["revision_id"], "data": legacy}, 200)
	raw, _ := json.Marshal(v["data"])
	var saved Payload
	json.Unmarshal(raw, &saved)
	if len(saved.CompanyProfile.Credentials) != 1 || len(saved.CompanyProfile.GalleryAnnotations) != 1 {
		t.Fatal("old App erased portal content")
	}
	saved.CompanyProfile.Credentials[0].Title = "待审证书"
	v = tc.fixtureCall("PUT", path+"/draft", merchant, gin.H{"revision_id": v["revision_id"], "data": saved}, 200)
	tc.call("POST", path+"/submit", merchant, gin.H{"revision_id": v["revision_id"]}, 200)
	pub := tc.call("GET", "/public/showrooms/v-"+number(v["id"]), "", nil, 200)
	blob, _ := json.Marshal(pub)
	if strings.Contains(string(blob), "待审证书") || !strings.Contains(string(blob), "原公开证书") {
		t.Fatal("review snapshot leaked")
	}
	tc.fixtureCall("PUT", path+"/draft", merchant, gin.H{"revision_id": v["revision_id"], "data": saved}, 409)
	tc.call("POST", "/admin/revisions/"+number(v["revision_id"])+"/review", admin, gin.H{"decision": "reject", "note": "测试驳回"}, 200)
	saved.CompanyProfile.Credentials = []CompanyCredential{}
	saved.MediaIDs = nil
	v = tc.fixtureCall("PUT", path+"/draft", merchant, gin.H{"revision_id": v["revision_id"], "data": saved}, 200)
	publish(v)
	read("", 404)
	tc.call("POST", path+"/offline", merchant, gin.H{}, 200)
	tc.call("GET", "/public/showrooms/v-"+number(v["id"]), "", nil, 404)
}
