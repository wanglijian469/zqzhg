package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The same harness also runs in the preserved, unmodified backend source snapshot.
func TestImagePerformanceBrowser(t *testing.T) {
	if os.Getenv("RUN_IMAGE_BROWSER") != "1" {
		t.Skip("explicit isolated performance run")
	}
	a := importTestApp(t)
	root := os.Getenv("IMAGE_PROJECT_ROOT")
	if root == "" {
		root, _ = filepath.Abs("..")
	}
	a.Config.WebDir = os.Getenv("BROWSER_WEB_DIR")
	a.Config.Env = "production"
	a.Config.SMSProvider = "disabled"
	t.Setenv("IMAGE_CACHE_DIR", t.TempDir())
	photos := []uint64{}
	for i := 0; i < 6; i++ {
		img := image.NewRGBA(image.Rect(0, 0, 2048, 1280))
		state := uint32(91 + i)
		for y := 0; y < 1280; y++ {
			for x := 0; x < 2048; x++ {
				state = state*1664525 + 1013904223
				noise := int(state >> 26)
				img.SetRGBA(x, y, color.RGBA{uint8((x/10 + noise) % 240), uint8((y/8 + noise) % 240), uint8((x/24 + y/24 + noise) % 240), 255})
			}
		}
		var full, thumb bytes.Buffer
		jpeg.Encode(&full, img, &jpeg.Options{Quality: 88})
		jpeg.Encode(&thumb, scaled(img, 480), &jpeg.Options{Quality: 88})
		media := Media{Path: fmt.Sprintf("photo-%d.jpg", i), ThumbPath: fmt.Sprintf("photo-%d-thumb.jpg", i)}
		if err := a.DB.Create(&media).Error; err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(a.Config.MediaDir, media.Path), full.Bytes(), 0600)
		os.WriteFile(filepath.Join(a.Config.MediaDir, media.ThumbPath), thumb.Bytes(), 0600)
		photos = append(photos, media.ID)
	}
	logoImg := image.NewNRGBA(image.Rect(0, 0, 200, 200))
	for y := 20; y < 180; y++ {
		for x := 20; x < 180; x++ {
			logoImg.SetNRGBA(x, y, color.NRGBA{30, 100, 180, 255})
		}
	}
	var logoData bytes.Buffer
	png.Encode(&logoData, logoImg)
	logo := Media{Path: "transparent-logo.png", ThumbPath: "transparent-logo.png"}
	a.DB.Create(&logo)
	os.WriteFile(filepath.Join(a.Config.MediaDir, logo.Path), logoData.Bytes(), 0600)
	var vendorID, productID uint64
	for i := 0; i < 12; i++ {
		vendor := industryFixture(t, a, Payload{Name: fmt.Sprintf("轻量化验收厂商%02d", i), MainProducts: "农业机械、设备配件", RegionCode: "130528", Phone: "0319-1234567", MediaIDs: photos, CompanyProfile: &CompanyProfile{CoverMediaID: photos[i%len(photos)], LogoMediaID: logo.ID, Summary: "隔离数据库图片性能验收"}}, 1)
		a.DB.Model(&Content{}).Where("id=?", vendor.ID).Update("featured", true)
		for j := 0; j < 2; j++ {
			product := Content{Kind: "product", VendorID: vendor.ID, HallID: 1, Visible: true, Featured: true}
			a.DB.Create(&product)
			makeDraft(a.DB, &product, Payload{Name: fmt.Sprintf("验收机械%02d-%d", i, j), CategoryID: 1, MediaIDs: photos, Description: "图片轻量化验收产品"})
			a.DB.Model(&Revision{}).Where("id=?", product.DraftRevisionID).Update("status", "approved")
			product.PublishedRevisionID = product.DraftRevisionID
			a.DB.Save(&product)
			if productID == 0 {
				productID = product.ID
			}
		}
		if vendorID == 0 {
			vendorID = vendor.ID
		}
	}
	a.DB.Create(&HomeDisplayMedia{MediaID: photos[0]})
	a.DB.Model(&HomeDisplaySettings{}).Where("id=1").Update("hero_media_id", photos[0])
	srv := httptest.NewServer(a.router())
	defer srv.Close()
	a.Config.PublicBaseURL = srv.URL
	fixture := map[string]any{"base": srv.URL, "vendorID": vendorID, "productID": productID, "photos": photos, "logo": logo.ID}
	file := filepath.Join(t.TempDir(), "fixture.json")
	data, _ := json.Marshal(fixture)
	os.WriteFile(file, data, 0600)
	cmd := exec.CommandContext(t.Context(), "node", filepath.Join(root, "scripts/verify-image-performance.cjs"), file)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
}
