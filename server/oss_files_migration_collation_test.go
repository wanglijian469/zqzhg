package main

import (
	"github.com/gin-gonic/gin"
	"testing"
)

func TestCloudMigrationDryRunMixedHistoricalCollations(t *testing.T) {
	a := importTestApp(t)
	if err := a.DB.Exec("ALTER TABLE stored_objects MODIFY object_key VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL").Error; err != nil {
		t.Fatal(err)
	}
	if err := a.DB.Exec("ALTER TABLE media_object_owners MODIFY object_key VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL").Error; err != nil {
		t.Fatal(err)
	}
	owner := Account{Username: "mixed-collation-owner", Role: "merchant"}
	if err := a.DB.Create(&owner).Error; err != nil {
		t.Fatal(err)
	}
	vendor := Content{OwnerID: owner.ID, Kind: "vendor", HallID: 1}
	if err := a.DB.Create(&vendor).Error; err != nil {
		t.Fatal(err)
	}
	object := StoredObject{Key: "historical-file.pdf", StoreID: 0, Size: 123, SHA256: digestText("fixture")}
	if err := a.DB.Create(&object).Error; err != nil {
		t.Fatal(err)
	}
	if err := a.DB.Create(&MediaObjectOwner{ObjectKey: object.Key, OwnerID: owner.ID, Size: object.Size}).Error; err != nil {
		t.Fatal(err)
	}
	admin := resourceTestSession(t, a, "admin")
	tc := testClient{t, a.router()}
	result := tc.call("POST", "/admin/storage/oss/migrations", admin, gin.H{"enterprise_id": vendor.ID, "dry_run": true}, 200)
	if result["objects"] != float64(1) || result["bytes"] != float64(123) || result["source_retained"] != true {
		t.Fatal("incorrect migration preview")
	}
	var jobs int64
	a.DB.Model(&FileStorageJob{}).Count(&jobs)
	if jobs != 0 {
		t.Fatal("dry run created a migration")
	}
	var after StoredObject
	if a.DB.First(&after, "object_key=?", object.Key).Error != nil || after.StoreID != 0 || after.RemoteKey != "" {
		t.Fatal("dry run changed source mapping")
	}
}
