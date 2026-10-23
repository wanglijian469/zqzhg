package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestProcurementNotificationIdentityScopes(t *testing.T) {
	a := importTestApp(t)
	dual, session := procurementTestAccount(t, a, "merchant")
	other, otherSession := procurementTestAccount(t, a, "visitor")
	now := time.Now().UTC()
	owned := ProcurementProject{BuyerID: dual.ID, HallID: 1, State: "open", Audience: "public", Version: 1, CreatedAt: now, UpdatedAt: now}
	foreign := owned
	foreign.BuyerID = other.ID
	for _, p := range []*ProcurementProject{&owned, &foreign} {
		if err := a.DB.Create(p).Error; err != nil {
			t.Fatal(err)
		}
	}
	rows := []ProcurementNotification{
		{AccountID: dual.ID, ProjectID: owned.ID, Kind: "quote", EventKey: "buyer-new", AvailableAt: now, CreatedAt: now},
		{AccountID: dual.ID, ProjectID: owned.ID, Kind: "message", EventKey: "buyer-read", AvailableAt: now, CreatedAt: now, ReadAt: &now},
		{AccountID: dual.ID, ProjectID: foreign.ID, Kind: "invitation", EventKey: "supplier-new", AvailableAt: now, CreatedAt: now},
		{AccountID: dual.ID, ProjectID: 0, Kind: "digest", EventKey: "supplier-digest", AvailableAt: now, CreatedAt: now},
		{AccountID: dual.ID, ProjectID: foreign.ID, Kind: "digest", EventKey: "future", AvailableAt: now.Add(time.Hour), CreatedAt: now},
		{AccountID: other.ID, ProjectID: foreign.ID, Kind: "quote", EventKey: "other", AvailableAt: now, CreatedAt: now},
	}
	if err := a.DB.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	r := a.router()
	check := func(scope string, total, unread int) {
		t.Helper()
		value := procCall(t, r, "GET", "/procurement/notifications"+scope, session, nil, 200)
		if int(value["total"].(float64)) != total || int(value["unread"].(float64)) != unread {
			t.Fatalf("scope %q: %v", scope, value)
		}
		for _, raw := range value["items"].([]any) {
			item := raw.(map[string]any)
			if scope == "?scope=buyer" && item["target_scope"] != "buyer" || scope == "?scope=supplier" && item["target_scope"] != "supplier" {
				t.Fatal("incorrect target scope", item)
			}
		}
	}
	check("?scope=buyer", 2, 1)
	check("?scope=supplier", 2, 2)
	check("", 4, 3)
	page := procCall(t, r, "GET", "/procurement/notifications?scope=supplier&page=2", session, nil, 200)
	if len(page["items"].([]any)) != 0 || page["total"] != float64(2) || page["unread"] != float64(2) {
		t.Fatal(page)
	}
	procCall(t, r, "GET", "/procurement/notifications?scope=admin", session, nil, 400)
	procCall(t, r, "POST", "/procurement/notifications/read", session, gin.H{"scope": "admin", "ids": []uint64{}}, 400)
	procCall(t, r, "POST", "/procurement/notifications/read", session, gin.H{"scope": "buyer", "ids": []uint64{rows[2].ID, rows[5].ID}}, 200)
	check("?scope=supplier", 2, 2)
	procCall(t, r, "POST", "/procurement/notifications/read", session, gin.H{"scope": "buyer", "ids": []uint64{}}, 200)
	check("?scope=buyer", 2, 0)
	check("?scope=supplier", 2, 2)
	procCall(t, r, "POST", "/procurement/notifications/read", session, gin.H{"scope": "supplier", "ids": []uint64{rows[2].ID}}, 200)
	check("?scope=supplier", 2, 1)
	procCall(t, r, "POST", "/procurement/notifications/read", session, gin.H{"ids": []uint64{}}, 200)
	check("", 4, 0)
	otherList := procCall(t, r, "GET", "/procurement/notifications?scope=buyer", otherSession, nil, 200)
	if otherList["unread"] != float64(1) {
		t.Fatal("another account's message marked read", otherList)
	}
	var future ProcurementNotification
	if err := a.DB.First(&future, rows[4].ID).Error; err != nil || future.ReadAt != nil {
		t.Fatal(fmt.Sprint("future notification changed: ", err))
	}
}
