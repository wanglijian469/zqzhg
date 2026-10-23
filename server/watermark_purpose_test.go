package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"testing"
)

func watermarkUploadPurpose(t *testing.T, a *App, token, path, purpose string) uint64 {
	t.Helper()
	var body, imageData bytes.Buffer
	png.Encode(&imageData, watermarkSample(false, 640, 480))
	w := multipart.NewWriter(&body)
	w.WriteField("purpose", purpose)
	part, _ := w.CreateFormFile("file", "purpose.png")
	part.Write(imageData.Bytes())
	w.Close()
	req := httptest.NewRequest("POST", "/api/v1"+path, &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	out := httptest.NewRecorder()
	a.router().ServeHTTP(out, req)
	if out.Code != 201 {
		t.Fatal(out.Code, out.Body.String())
	}
	var result map[string]any
	json.Unmarshal(out.Body.Bytes(), &result)
	return uint64(result["id"].(float64))
}
func TestWatermarkActualSlotAndAdministratorOwner(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	merchant := tc.login("13900007775")
	admin := resourceTestSession(t, a, "admin")
	saved := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "未填简称的企业全称", RegionName: "河北", MainProducts: "机械", Phone: "0319-1234567"}}, 201)
	source := watermarkUploadPurpose(t, a, admin, "/admin/contents/"+number(saved["id"])+"/media", "company_logo")
	var m Media
	a.DB.First(&m, source)
	if float64(m.OwnerID) != saved["owner_id"] {
		t.Fatal("admin image did not belong to manufacturer")
	}
	p := Payload{Name: "未填简称的企业全称", RegionName: "河北", MainProducts: "机械", Phone: "0319-1234567", CompanyProfile: &CompanyProfile{LogoMediaID: source}}
	logo := tc.fixtureCall("PUT", "/merchant/contents/"+number(saved["id"])+"/draft", merchant, gin.H{"revision_id": saved["revision_id"], "data": p}, 200)
	var variants int64
	a.DB.Model(&MediaWatermark{}).Where("source_media_id=?", source).Count(&variants)
	if variants != 0 {
		t.Fatal("logo was watermarked")
	}
	// Client hints cannot bypass watermarking when an image becomes a gallery photo.
	p.MediaIDs = []uint64{source}
	gallery := tc.fixtureCall("PUT", "/merchant/contents/"+number(saved["id"])+"/draft", merchant, gin.H{"revision_id": logo["revision_id"], "data": p}, 200)
	marked := uint64(gallery["data"].(map[string]any)["media_ids"].([]any)[0].(float64))
	if marked == source {
		t.Fatal("purpose hint bypassed actual slot")
	}
	var wm MediaWatermark
	a.DB.First(&wm, "media_id=?", marked)
	if wm.VendorText != "未填简称的企业全称" || wm.VendorID != uint64(saved["id"].(float64)) {
		t.Fatal("fallback owner/name mismatch", wm)
	}
	home := watermarkUploadPurpose(t, a, admin, "/admin/home-display/media", "product_image")
	var homeWM MediaWatermark
	a.DB.First(&homeWM, "media_id=?", home)
	if homeWM.Purpose != "home_display" {
		t.Fatal("server role did not override hint")
	}
	tc.call("PUT", "/admin/home-display", admin, gin.H{"version": 1, "hero_media_id": home}, 200)
	request := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/media/%d", home), nil)
	out := httptest.NewRecorder()
	a.router().ServeHTTP(out, request)
	if out.Code != 200 {
		t.Fatal("excluded home display blocked", out.Code)
	}
}
