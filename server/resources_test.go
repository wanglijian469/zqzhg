package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
	pdfmodel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"image"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func resourceTestSession(t *testing.T, a *App, role string) string {
	t.Helper()
	account := Account{Username: "resources-" + randomToken()[:12], Role: role}
	if e := a.DB.Create(&account).Error; e != nil {
		t.Fatal(e)
	}
	s, e := a.newSession(account)
	if e != nil {
		t.Fatal(e)
	}
	return s["token"].(string)
}
func resourceUpload(t *testing.T, a *App, token, path, name string, data []byte, want int) map[string]any {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	f, _ := w.CreateFormFile("file", name)
	f.Write(data)
	w.Close()
	req := httptest.NewRequest("POST", "/api/v1"+path, &b)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", w.FormDataContentType())
	out := httptest.NewRecorder()
	a.router().ServeHTTP(out, req)
	if out.Code != want {
		t.Fatalf("upload %s: %d %s", name, out.Code, out.Body.String())
	}
	var result map[string]any
	json.Unmarshal(out.Body.Bytes(), &result)
	return result
}
func tinyPDF() []byte {
	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R] /Count 1 >>", "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 400] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>", "<< /Length 37 >>\nstream\nBT /F1 16 Tf 20 350 Td (Catalog) Tj ET\nendstream", "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i, o := range objects {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, o := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return b.Bytes()
}

func TestResourcesIndependentReviewAndVisibility(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	merchant := resourceTestSession(t, a, "merchant")
	other := resourceTestSession(t, a, "merchant")
	admin := resourceTestSession(t, a, "admin")
	vendor := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "资源测试企业", MainProducts: "测试机械", RegionName: "河北", Phone: "13900009901"}}, 201)
	product := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "product", "data": Payload{Name: "资源关联产品", CategoryID: 1}}, 201)
	approveContent := func(item map[string]any) {
		tc.call("POST", "/merchant/contents/"+number(item["id"])+"/submit", merchant, gin.H{"revision_id": item["revision_id"]}, 200)
		tc.call("POST", "/admin/revisions/"+number(item["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	}
	approveContent(vendor)
	approveContent(product)
	var pic bytes.Buffer
	png.Encode(&pic, image.NewRGBA(image.Rect(0, 0, 16, 16)))
	img := resourceUpload(t, a, merchant, "/merchant/media", "page.png", pic.Bytes(), 201)
	pdf := resourceUpload(t, a, merchant, "/merchant/resource-media?kind=pdf", "catalog.pdf", tinyPDF(), 201)
	resourceUpload(t, a, merchant, "/merchant/resource-media?kind=pdf", "broken.pdf", []byte("%PDF-1.4 broken"), 400)
	conf := pdfmodel.NewDefaultConfiguration()
	conf.UserPW = "reader"
	conf.OwnerPW = "owner"
	var encrypted bytes.Buffer
	if e := pdfapi.Encrypt(context.Background(), bytes.NewReader(tinyPDF()), &encrypted, conf); e != nil {
		t.Fatal(e)
	}
	resourceUpload(t, a, merchant, "/merchant/resource-media?kind=pdf", "encrypted.pdf", encrypted.Bytes(), 400)
	payload := ResourcePayload{Title: "测试PDF图册", Format: "pdf", FileMediaID: uint64(pdf["media_id"].(float64)), ProductID: uint64(product["id"].(float64))}
	invalidCover := payload
	invalidCover.CoverMediaID = payload.FileMediaID
	tc.call("POST", "/merchant/resources", merchant, gin.H{"kind": "catalog", "data": invalidCover}, 400)
	r := tc.call("POST", "/merchant/resources", merchant, gin.H{"kind": "catalog", "data": payload}, 201)
	rid := number(r["id"])
	tc.call("GET", "/public/resources/"+rid, "", nil, 404)
	tc.call("GET", "/merchant/resources/"+rid, other, nil, 403)
	approve := func(item map[string]any) {
		tc.call("POST", "/merchant/resources/"+number(item["id"])+"/submit", merchant, gin.H{"revision_id": item["revision_id"]}, 200)
		tc.call("POST", "/admin/resource-revisions/"+number(item["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	}
	approve(r)
	pub := tc.call("GET", "/public/resources/"+rid, "", nil, 200)
	if pub["data"].(map[string]any)["title"] != "测试PDF图册" {
		t.Fatal(pub)
	}
	payload.Title = "待审新标题"
	draft := tc.fixtureCall("PUT", "/merchant/resources/"+rid+"/draft", merchant, gin.H{"revision_id": r["revision_id"], "data": payload}, 200)
	tc.call("POST", "/merchant/resources/"+rid+"/submit", merchant, gin.H{"revision_id": draft["revision_id"]}, 200)
	tc.fixtureCall("PUT", "/merchant/resources/"+rid+"/draft", merchant, gin.H{"revision_id": draft["revision_id"], "data": payload}, 409)
	if p := tc.call("GET", "/public/resources/"+rid, "", nil, 200); p["data"].(map[string]any)["title"] != "测试PDF图册" {
		t.Fatal("pending data leaked", p)
	}
	tc.call("POST", "/admin/resource-revisions/"+number(draft["revision_id"])+"/review", admin, gin.H{"decision": "reject"}, 400)
	tc.call("POST", "/admin/resource-revisions/"+number(draft["revision_id"])+"/review", admin, gin.H{"decision": "reject", "note": "请完善图册"}, 200)
	var foreignAccount Account
	a.DB.Where("role='merchant' AND id<>?", r["owner_id"]).First(&foreignAccount)
	var foreign Media
	a.DB.Create(&Media{OwnerID: foreignAccount.ID, Path: "foreign.jpg"})
	a.DB.Last(&foreign)
	bad := payload
	bad.CoverMediaID = foreign.ID
	tc.fixtureCall("PUT", "/merchant/resources/"+rid+"/draft", merchant, gin.H{"revision_id": draft["revision_id"], "data": bad}, 400)
	pages := ResourcePayload{Title: "图片册", Format: "images", PageMediaIDs: []uint64{uint64(img["id"].(float64))}}
	imageCatalog := tc.call("POST", "/merchant/resources", merchant, gin.H{"kind": "catalog", "data": pages}, 201)
	approve(imageCatalog)
	t.Setenv("MAX_VENDOR_CATALOGS", "2")
	tc.call("POST", "/merchant/resources", merchant, gin.H{"kind": "catalog", "data": pages}, 409)
	tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "product", "data": Payload{Name: "非法文件当图片", CategoryID: 1, MediaIDs: []uint64{payload.FileMediaID}}}, 400)
	for _, method := range []string{"GET", "HEAD"} {
		req := httptest.NewRequest(method, "/api/v1/media/"+number(pdf["media_id"]), nil)
		req.Header.Set("Range", "bytes=0-15")
		out := httptest.NewRecorder()
		a.router().ServeHTTP(out, req)
		if out.Code != 206 {
			t.Fatal(method, out.Code, out.Body.String())
		}
	}
	tc.call("POST", "/merchant/contents/"+number(product["id"])+"/offline", merchant, gin.H{}, 200)
	tc.call("GET", "/public/resources/"+rid, "", nil, 404)
	tc.call("GET", "/media/"+number(pdf["media_id"]), "", nil, 404)
	tc.call("GET", "/public/resources/"+number(imageCatalog["id"]), "", nil, 200)
	tc.call("POST", "/merchant/contents/"+number(vendor["id"])+"/offline", merchant, gin.H{}, 200)
	tc.call("GET", "/public/resources/"+number(imageCatalog["id"]), "", nil, 404)
	tc.call("GET", "/media/"+number(img["id"]), "", nil, 404)
	if list := tc.call("GET", "/merchant/resources?kind=catalog", other, nil, 200); list["total"] != float64(0) {
		t.Fatal("foreign resources leaked")
	}
	tc.call("POST", "/merchant/resources/"+rid+"/offline", merchant, gin.H{}, 200)
	tc.call("POST", "/admin/resource-revisions/"+number(draft["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 409)
	tc.call("GET", "/merchant/resources/"+rid+"/history", merchant, nil, 200)
}

func TestResourceMigrationRecoversPartialTables(t *testing.T) {
	a := importTestApp(t)
	if e := a.DB.Exec("DELETE FROM schema_migrations WHERE version=5").Error; e != nil {
		t.Fatal(e)
	}
	// Simulate stopping between CREATE TABLE statements: retain earlier tables.
	for _, table := range []string{"resource_media_references", "resource_review_logs", "media_metadata"} {
		if e := a.DB.Exec("DROP TABLE " + table).Error; e != nil {
			t.Fatal(e)
		}
	}
	for run := 0; run < 2; run++ {
		if e := migrate(a.DB); e != nil {
			t.Fatal(e)
		}
	}
	for _, table := range []string{"resources", "resource_revisions", "resource_media_references", "resource_review_logs", "media_metadata"} {
		if !a.DB.Migrator().HasTable(table) {
			t.Fatal("missing table after recovery", table)
		}
	}
	var count int64
	a.DB.Table("schema_migrations").Where("version=5").Count(&count)
	if count != 1 {
		t.Fatal("migration marker must occur once", count)
	}
}

func TestVideoWorkerProcessingRetryAndRecovery(t *testing.T) {
	a := importTestApp(t)
	ff := videoTool("ffmpeg")
	if _, e := os.Stat(ff); e != nil {
		t.Skip("FFmpeg must be installed for video integration tests")
	}
	token := resourceTestSession(t, a, "merchant")
	tc := testClient{t, a.router()}
	vendor := tc.fixtureCall("POST", "/merchant/contents", token, gin.H{"kind": "vendor", "data": Payload{Name: "视频企业", MainProducts: "机械", RegionName: "河北", Phone: "13900009902"}}, 201)
	_ = vendor
	for _, size := range []string{"320x180", "180x320"} {
		file := filepath.Join(t.TempDir(), "silent.mov")
		out, e := exec.Command(ff, "-nostdin", "-y", "-v", "error", "-f", "lavfi", "-i", "color=c=green:s="+size+":d=1", "-c:v", "libx264", "-pix_fmt", "yuv420p", file).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(out))
		}
		data, _ := os.ReadFile(file)
		m := resourceUpload(t, a, token, "/merchant/resource-media?kind=video", "silent.mov", data, 202)
		id := number(m["media_id"])
		r := tc.call("POST", "/merchant/resources", token, gin.H{"kind": "video", "data": ResourcePayload{Title: "视频", Purpose: "production", FileMediaID: uint64(m["media_id"].(float64))}}, 201)
		tc.call("POST", "/merchant/resources/"+number(r["id"])+"/submit", token, gin.H{"revision_id": r["revision_id"]}, 409)
		if !a.processNextVideo(context.Background()) {
			t.Fatal("job not processed")
		}
		meta := tc.call("GET", "/merchant/resource-media/"+id, token, nil, 200)
		if meta["state"] != "ready" || meta["duration"].(float64) <= 0 {
			t.Fatal(meta)
		}
		tc.call("POST", "/merchant/resources/"+number(r["id"])+"/submit", token, gin.H{"revision_id": r["revision_id"]}, 200)
		var stored Media
		a.DB.First(&stored, m["media_id"])
		if _, e := os.Stat(filepath.Join(a.Config.MediaDir, stored.ThumbPath)); e != nil {
			t.Fatal(e)
		}
		req := httptest.NewRequest("GET", "/api/v1/media/"+id, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Range", "bytes=0-127")
		response := httptest.NewRecorder()
		a.router().ServeHTTP(response, req)
		if response.Code != 206 {
			t.Fatal(response.Code)
		}
	}
	bad := resourceUpload(t, a, token, "/merchant/resource-media?kind=video", "bad.mp4", []byte("not a video"), 202)
	a.processNextVideo(context.Background())
	id := number(bad["media_id"])
	if s := tc.call("GET", "/merchant/resource-media/"+id, token, nil, 200); s["state"] != "failed" {
		t.Fatal(s)
	}
	tc.call("POST", "/merchant/resource-media/"+id+"/retry", token, gin.H{}, 200)
	a.DB.Model(&MediaMetadata{}).Where("media_id=?", bad["media_id"]).Update("state", "processing")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a.startMediaWorker(ctx)
	if s := tc.call("GET", "/merchant/resource-media/"+id, token, nil, 200); s["state"] != "queued" {
		t.Fatal("job not recovered", s)
	}
	t.Setenv("MAX_VIDEO_MIB", "1")
	resourceUpload(t, a, token, "/merchant/resource-media?kind=video", "large.mp4", []byte(strings.Repeat("x", (1<<20)+1)), 400)
}
