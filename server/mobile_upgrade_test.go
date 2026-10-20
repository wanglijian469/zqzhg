package main

import (
	"image"
	"image/color"
	"image/png"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMobileClientCapabilities(t *testing.T) {
	a := importTestApp(t)
	result := testClient{t, a.router()}.call("GET", "/public/config", "", nil, 200)
	capabilities, ok := result["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatal("missing capabilities")
	}
	for _, key := range []string{"administrative_regions", "industry_map", "product_name_only"} {
		if capabilities[key] != true {
			t.Fatalf("missing %s", key)
		}
	}
}

func TestMobileUpgradeIntegration(t *testing.T) {
	if os.Getenv("RUN_MOBILE18_INTEGRATION") != "1" {
		t.Skip("set RUN_MOBILE18_INTEGRATION=1 with RUN_MYSQL_TESTS=1")
	}
	a := importTestApp(t)
	industryFixture(t, a, Payload{Name: "移动统计隔离企业（仅测试）", RegionCode: "130528", MainProducts: "隔离验收产品"}, 1)
	industryFixture(t, a, Payload{Name: "城市级隔离企业（仅测试）", RegionCode: "1305"}, 1)
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(a.router())
	defer server.Close()
	command := exec.CommandContext(t.Context(), "cmd", "/c", filepath.Join(root, ".tools", "flutter", "bin", "flutter.bat"), "test", "test/mobile_upgrade_integration_test.dart", "--no-pub", "--reporter", "expanded")
	command.Dir = filepath.Join(root, "mobile")
	fixturePath := filepath.Join(t.TempDir(), "native-fixture.png")
	fixtureFile, err := os.Create(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	fixtureImage := image.NewRGBA(image.Rect(0, 0, 8, 8))
	fixtureImage.Set(0, 0, color.RGBA{20, 90, 160, 255})
	if err := png.Encode(fixtureFile, fixtureImage); err != nil {
		t.Fatal(err)
	}
	if err := fixtureFile.Close(); err != nil {
		t.Fatal(err)
	}
	command.Env = append(os.Environ(), "MOBILE18_INTEGRATION_BASE="+server.URL, "MOBILE18_IMAGE_FIXTURE="+fixturePath)
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := a.DB.Model(&Account{}).Where("role=?", "merchant").Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("expected company and phone accounts only: %d %v", count, err)
	}
	if err := a.DB.Model(&Content{}).Where("kind=?", "product").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("single native product creation: %d %v", count, err)
	}
}
