package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestProcurementMatchingUsesApprovedCategoriesAndCoverage(t *testing.T) {
	a := importTestApp(t)
	cat := procurementCategory(t, a)
	_, buyer := procurementTestAccount(t, a, "visitor")
	s1, _, v1, _ := procurementSupplier(t, a, cat)
	_, _, v2, _ := procurementSupplier(t, a, cat)
	seoSetPayload(t, a, v2, Payload{Name: "仅北京供货", CompanyProfile: &CompanyProfile{SupplyRegionCodes: []string{"11"}}})
	s3, _ := procurementTestAccount(t, a, "merchant")
	v3 := v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, "待审供货类别")
	a.DB.Model(&v3).Update("owner_id", s3.ID)
	payload, _ := json.Marshal(Payload{Name: "待审供货类别", CompanyProfile: &CompanyProfile{SupplyProductCategoryIDs: []uint64{cat}}})
	draft := Revision{ContentID: v3.ID, Status: "pending", Payload: string(payload)}
	a.DB.Create(&draft)
	a.DB.Model(&v3).Update("draft_revision_id", draft.ID)
	r := a.router()
	created := procTestDraft(t, a, r, buyer, 0, cat)
	pid := procID(created["id"])
	procSubmitTest(t, r, buyer, created, "public")
	admin := resourceTestSession(t, a, "admin")
	var review ProcurementReview
	a.DB.Where("project_id=?", pid).First(&review)
	procCall(t, r, "POST", fmt.Sprintf("/admin/procurement/reviews/%d", review.ID), admin, gin.H{"approved": true, "revision_id": review.RevisionID}, 200)
	var parts []ProcurementParticipant
	a.DB.Where("project_id=?", pid).Find(&parts)
	if len(parts) != 1 || parts[0].VendorID != v1.ID {
		t.Fatal("irrelevant suppliers notified", parts)
	}
	if e := a.DB.Model(&ProcurementPreference{}).Where("account_id=?", s1.ID).Update("supplier_suspended", true).Error; e != nil {
		t.Fatal(e)
	}
	var p ProcurementProject
	a.DB.First(&p, pid)
	matches, e := a.procFindMatches(a.DB, p)
	if e != nil || len(matches) != 0 {
		t.Fatal("suspended supplier remains eligible", matches, e)
	}
}
func TestProcurementDraftQuotesAndOperatorPermissions(t *testing.T) {
	a := importTestApp(t)
	cat := procurementCategory(t, a)
	buyer, b := procurementTestAccount(t, a, "visitor")
	supplier, s, _, product := procurementSupplier(t, a, cat)
	admin := resourceTestSession(t, a, "admin")
	r := a.router()
	created := procTestDraft(t, a, r, b, product.ID, cat)
	procSubmitTest(t, r, b, created, "direct")
	base := fmt.Sprintf("/procurement/projects/%d", procID(created["id"]))
	view := procCall(t, r, "GET", base, b, nil, 200)
	quoteInput := procurementQuoteTestPayload(view, "9.8765")
	quoteInput["submit"] = false
	draft := procCall(t, r, "POST", base+"/quotes", s, quoteInput, 200)
	buyerQuotes := procCall(t, r, "GET", base+"/quotes", b, nil, 200)
	if len(buyerQuotes["items"].([]any)) != 0 {
		t.Fatal("draft quote leaked to buyer")
	}
	quoteInput["submit"] = true
	quoteInput["version"] = draft["quote"].(map[string]any)["version"]
	procCall(t, r, "POST", base+"/quotes", s, quoteInput, 200)
	op := procCall(t, r, "POST", "/admin/operators", admin, gin.H{"username": "proc-reader", "enabled": true, "permissions": []string{"procurement.read"}}, 201)
	login := procCall(t, r, "POST", "/auth/admin", "", gin.H{"username": op["username"], "password": op["initial_password"]}, 200)
	reader := login["token"].(string)
	procCall(t, r, "GET", fmt.Sprintf("/admin/procurement/projects/%d", procID(created["id"])), reader, nil, 200)
	procCall(t, r, "GET", fmt.Sprintf("/admin/procurement/projects/%d/quotes", procID(created["id"])), reader, nil, 403)
	procCall(t, r, "POST", fmt.Sprintf("/admin/procurement/accounts/%d/restrict", supplier.ID), admin, gin.H{"supplier": true, "suspended": true, "reason": "异常报价核查"}, 200)
	procCall(t, r, "PUT", "/procurement/preferences", s, gin.H{"opportunity_mode": "off", "supplier_suspended": false}, 200)
	var preference ProcurementPreference
	a.DB.First(&preference, "account_id=?", supplier.ID)
	if !preference.SupplierSuspended {
		t.Fatal("preference update bypassed restriction")
	}
	quoteInput["version"] = float64(2)
	procCall(t, r, "POST", base+"/quotes", s, quoteInput, 403)
	procCall(t, r, "POST", fmt.Sprintf("/admin/procurement/accounts/%d/restrict", buyer.ID), admin, gin.H{"suspended": true, "reason": "采购资料核查"}, 200)
	procCall(t, r, "PUT", "/procurement/profile", b, gin.H{"version": 1, "contact_name": "尝试恢复", "suspended": false}, 403)
}
func TestProcurementQuoteAndClosureAreSerialized(t *testing.T) {
	a := importTestApp(t)
	cat := procurementCategory(t, a)
	_, buyer := procurementTestAccount(t, a, "visitor")
	_, supplier, _, product := procurementSupplier(t, a, cat)
	r := a.router()
	created := procTestDraft(t, a, r, buyer, product.ID, cat)
	procSubmitTest(t, r, buyer, created, "direct")
	base := fmt.Sprintf("/procurement/projects/%d", procID(created["id"]))
	view := procCall(t, r, "GET", base, buyer, nil, 200)
	var wait sync.WaitGroup
	statuses := make(chan int, 4)
	start := make(chan struct{})
	for i := 0; i < 4; i++ {
		wait.Add(1)
		go func(i int) {
			defer wait.Done()
			token, path, payload := supplier, base+"/quotes", procurementQuoteTestPayload(view, "15.00")
			if i%2 == 1 {
				token = buyer
				path = base + "/state"
				payload = gin.H{"version": view["version"], "state": "closed"}
			}
			raw, _ := json.Marshal(payload)
			request := httptest.NewRequest("POST", "/api/v1"+path, bytes.NewReader(raw))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+token)
			request.Header.Set("Idempotency-Key", randomToken())
			<-start
			response := httptest.NewRecorder()
			r.ServeHTTP(response, request)
			statuses <- response.Code
		}(i)
	}
	close(start)
	wait.Wait()
	close(statuses)
	for status := range statuses {
		if status != 200 && status != 409 {
			t.Fatal("unexpected concurrent result", status)
		}
	}
	var p ProcurementProject
	a.DB.First(&p, procID(created["id"]))
	if p.State != "closed" {
		t.Fatal(p.State)
	}
	var quotes []ProcurementQuoteRevision
	a.DB.Find(&quotes)
	if len(quotes) > 1 {
		t.Fatal("duplicate competing submissions")
	}
	for _, quote := range quotes {
		if quote.CreatedAt.After(p.UpdatedAt) {
			t.Fatal("quote accepted after closure")
		}
	}
}
func TestProcurementOfficeAndUnusedUploadCleanup(t *testing.T) {
	for _, extension := range []string{".docx", ".xlsx"} {
		var buffer bytes.Buffer
		archive := zip.NewWriter(&buffer)
		main := "word/document.xml"
		root := "document"
		contentType := "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"
		if extension == ".xlsx" {
			main = "xl/workbook.xml"
			root = "workbook"
			contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"
		}
		part, _ := archive.Create("[Content_Types].xml")
		fmt.Fprintf(part, `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Override PartName="/%s" ContentType="%s"/></Types>`, main, contentType)
		part, _ = archive.Create(main)
		fmt.Fprintf(part, "<%s/>", root)
		archive.Close()
		file := filepath.Join(t.TempDir(), "valid"+extension)
		os.WriteFile(file, buffer.Bytes(), 0600)
		if _, e := validateProcurementFile(file, extension); e != nil {
			t.Fatal(extension, e)
		}
	}
	a := importTestApp(t)
	buyer, _ := procurementTestAccount(t, a, "visitor")
	project := ProcurementProject{BuyerID: buyer.ID, HallID: 1, State: "draft", Audience: "direct", Version: 1}
	a.DB.Create(&project)
	full := filepath.Join(t.TempDir(), "orphan.txt")
	os.WriteFile(full, []byte("unused"), 0600)
	key := "procurement/unused.txt"
	if e := a.storeWorkFile(context.Background(), key, full); e != nil {
		t.Fatal(e)
	}
	attachment := ProcurementAttachment{OwnerID: buyer.ID, ProjectID: project.ID, ObjectKey: key, Name: "unused.txt", ContentType: "text/plain", Size: 6, CreatedAt: time.Now().Add(-25 * time.Hour)}
	a.DB.Create(&attachment)
	a.cleanupProcurementUploads(context.Background())
	var n int64
	a.DB.Model(&ProcurementAttachment{}).Where("id=?", attachment.ID).Count(&n)
	if n != 0 {
		t.Fatal("orphan not cleaned")
	}
	a.DB.Model(&StoredObject{}).Where("object_key=?", key).Count(&n)
	if n != 0 {
		t.Fatal("orphan bytes retained")
	}
}

func TestProcurementDraftScopeAndSupersededExpansion(t *testing.T) {
	a := importTestApp(t)
	cat := procurementCategory(t, a)
	buyer, token := procurementTestAccount(t, a, "visitor")
	_, supplierToken, vendor, product := procurementSupplier(t, a, cat)
	admin := resourceTestSession(t, a, "admin")
	r := a.router()
	p := procTestDraft(t, a, r, token, product.ID, cat)
	base := fmt.Sprintf("/procurement/projects/%d", procID(p["id"]))
	pub := procSubmitTest(t, r, token, p, "direct")
	expanded := procCall(t, r, "POST", base+"/expand", token, gin.H{"version": pub["version"], "audience": "public"}, 200)
	var oldReview ProcurementReview
	a.DB.Where("project_id=? AND status='pending'", procID(p["id"])).First(&oldReview)
	draft := gin.H{"version": expanded["version"], "title": "重新指定厂商", "delivery_region": "13", "deadline": time.Now().Add(48 * time.Hour), "audience": "designated", "vendor_ids": []uint64{vendor.ID}, "items": []gin.H{{"name": "农机", "category_id": cat, "quantity": "3.000", "unit": "台"}}}
	saved := procCall(t, r, "PUT", base+"/draft", token, draft, 200)
	view := procCall(t, r, "GET", base+"?draft=1", token, nil, 200)
	if view["revision"].(map[string]any)["requested_audience"] != "designated" || procID(view["requested_vendor_ids"].([]any)[0]) != vendor.ID {
		t.Fatal("draft scope was not retained", view)
	}
	// Draft selections are private and cannot change the active published audience.
	supplierView := procCall(t, r, "GET", base, supplierToken, nil, 200)
	if supplierView["requested_vendor_ids"] != nil || supplierView["audience"] != "direct" {
		t.Fatal("draft configuration leaked or altered publication", supplierView)
	}
	procCall(t, r, "POST", fmt.Sprintf("/admin/procurement/reviews/%d", oldReview.ID), admin, gin.H{"approved": true, "revision_id": oldReview.RevisionID}, 409)
	procSubmitTest(t, r, token, saved, "designated", vendor.ID)
	// Adding supplier capability preserves the additive buyer profile and account.
	upgraded := phoneCall(t, r, "POST", "/auth/me/merchant-upgrade", token, gin.H{"confirmed": true}, 200)
	if procID(upgraded["account"].(map[string]any)["id"]) != buyer.ID {
		t.Fatal("upgrade changed account")
	}
	var profile BuyerProfile
	if e := a.DB.First(&profile, "account_id=?", buyer.ID).Error; e != nil {
		t.Fatal("buyer profile lost", e)
	}
}

func procurementBackupFixture(t *testing.T, a *App) (uint64, ProcurementAttachment, uint64) {
	t.Helper()
	cat := procurementCategory(t, a)
	buyer, token := procurementTestAccount(t, a, "visitor")
	_, supplier, vendor, product := procurementSupplier(t, a, cat)
	r := a.router()
	p := procTestDraft(t, a, r, token, product.ID, cat)
	procSubmitTest(t, r, token, p, "direct")
	projectID := procID(p["id"])
	base := fmt.Sprintf("/procurement/projects/%d", projectID)
	view := procCall(t, r, "GET", base, token, nil, 200)
	quote := procCall(t, r, "POST", base+"/quotes", supplier, procurementQuoteTestPayload(view, "12.5000"), 200)
	procCall(t, r, "POST", base+fmt.Sprintf("/messages/%d", vendor.ID), token, gin.H{"body": "保留采购洽谈历史"}, 200)
	full := filepath.Join(t.TempDir(), "private.pdf")
	if e := os.WriteFile(full, []byte("%PDF-1.7 private procurement backup fixture"), 0600); e != nil {
		t.Fatal(e)
	}
	attachment := ProcurementAttachment{ProjectID: projectID, OwnerID: buyer.ID, Scope: "requirement", ScopeID: procID(view["revision"].(map[string]any)["id"]), ObjectKey: "procurement/" + randomToken() + ".pdf", Name: "private.pdf", ContentType: "application/pdf", Size: 41, CreatedAt: time.Now().UTC()}
	if e := a.storeWorkFile(context.Background(), attachment.ObjectKey, full); e != nil {
		t.Fatal(e)
	}
	if e := a.DB.Create(&attachment).Error; e != nil {
		t.Fatal(e)
	}
	return projectID, attachment, procID(quote["revision"].(map[string]any)["id"])
}

func TestProcurementFirstInquiryWithExpansionDeliversOnlySource(t *testing.T) {
	a := importTestApp(t)
	cat := procurementCategory(t, a)
	_, buyer := procurementTestAccount(t, a, "visitor")
	_, supplier, _, product := procurementSupplier(t, a, cat)
	_, other, _, _ := procurementSupplier(t, a, cat)
	r := a.router()
	p := procTestDraft(t, a, r, buyer, product.ID, cat)
	procSubmitTest(t, r, buyer, p, "public")
	base := fmt.Sprintf("/procurement/projects/%d", procID(p["id"]))
	view := procCall(t, r, "GET", base, supplier, nil, 200)
	if view["audience"] != "direct" || view["revision"].(map[string]any)["status"] != "approved" {
		t.Fatal("source inquiry was delayed by expansion review", view)
	}
	procCall(t, r, "GET", base, other, nil, 404)
	var review ProcurementReview
	if e := a.DB.Where("project_id=? AND action='expand' AND status='pending'", procID(p["id"])).First(&review).Error; e != nil {
		t.Fatal("missing same-project expansion review", e)
	}
	if review.RevisionID != procID(view["revision"].(map[string]any)["id"]) {
		t.Fatal("expansion copied demand version")
	}
}
