package main

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math/big"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func procurementTestAccount(t *testing.T, a *App, role string) (Account, string) {
	t.Helper()
	now := time.Now().UTC()
	// Windows clocks can repeat modulo 100ms during a fixture sequence.
	// Choose a random unused number instead of deriving identity from the clock.
	phone := ""
	for {
		n, err := rand.Int(rand.Reader, big.NewInt(100000000))
		if err != nil {
			t.Fatal(err)
		}
		phone = "139" + fmt.Sprintf("%08d", n.Int64())
		var existing int64
		if err = a.DB.Model(&Account{}).Where("phone=?", phone).Count(&existing).Error; err != nil {
			t.Fatal(err)
		}
		if existing == 0 {
			break
		}
	}
	account := Account{Username: "proc-" + randomToken()[:15], Role: role, Phone: &phone, PhoneVerifiedAt: &now}
	if e := a.DB.Create(&account).Error; e != nil {
		t.Fatal(e)
	}
	session, e := a.newSession(account)
	if e != nil {
		t.Fatal(e)
	}
	return account, session["token"].(string)
}
func procurementCategory(t *testing.T, a *App) uint64 {
	t.Helper()
	var cats []Category
	if e := a.DB.Where("hall_id=1 AND category_kind='product' AND enabled=1").Order("id").Find(&cats).Error; e != nil {
		t.Fatal(e)
	}
	for _, cat := range cats {
		if selectableCategory(a.DB, cat.ID, "product", 1, 0, false) {
			return cat.ID
		}
	}
	t.Fatal("no product category")
	return 0
}
func procurementSupplier(t *testing.T, a *App, category uint64) (Account, string, Content, Content) {
	t.Helper()
	account, token := procurementTestAccount(t, a, "merchant")
	v := v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, "采购测试厂商"+fmt.Sprint(account.ID))
	if e := a.DB.Model(&v).Updates(map[string]any{"owner_id": account.ID, "draft_revision_id": v.PublishedRevisionID}).Error; e != nil {
		t.Fatal(e)
	}
	v.OwnerID = account.ID
	p := v2Published(t, a, "product", v.ID, 1, category, 1, false, 0, "采购测试产品"+fmt.Sprint(v.ID))
	a.DB.Model(&p).Updates(map[string]any{"owner_id": account.ID, "draft_revision_id": p.PublishedRevisionID})
	return account, token, v, p
}
func procCall(t *testing.T, r *gin.Engine, method, path, token string, payload any, status int, keys ...string) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(payload)
	req := httptest.NewRequest(method, "/api/v1"+path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	key := randomToken()
	if len(keys) > 0 {
		key = keys[0]
	}
	req.Header.Set("Idempotency-Key", key)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != status {
		t.Fatalf("%s %s wanted %d got %d: %s", method, path, status, w.Code, w.Body.String())
	}
	var result map[string]any
	if e := json.Unmarshal(w.Body.Bytes(), &result); e != nil {
		t.Fatal(e, w.Body.String())
	}
	return result
}
func procID(value any) uint64 { return uint64(value.(float64)) }
func procTestDraft(t *testing.T, a *App, r *gin.Engine, token string, source, cat uint64) map[string]any {
	t.Helper()
	procCall(t, r, "PUT", "/procurement/profile", token, gin.H{"version": 0, "company": "采购测试单位", "contact_name": "采购联系人"}, 200)
	return procCall(t, r, "POST", "/procurement/projects", token, gin.H{"hall_id": 1, "source_product_id": source, "title": "采购询比价测试", "delivery_region": "1305", "deadline": time.Now().Add(48 * time.Hour), "expected_delivery": "15天", "notes": "参与厂商可见的技术要求", "items": []gin.H{{"name": "测试农机", "category_id": cat, "specification": "功率80千瓦", "quantity": "2.500", "unit": "台"}}}, 200)
}
func procSubmitTest(t *testing.T, r *gin.Engine, token string, p map[string]any, audience string, vendors ...uint64) map[string]any {
	t.Helper()
	return procCall(t, r, "POST", fmt.Sprintf("/procurement/projects/%d/submit", procID(p["id"])), token, gin.H{"version": p["version"], "audience": audience, "vendor_ids": vendors}, 200)
}
func procurementQuoteTestPayload(project map[string]any, price string) gin.H {
	revision := project["revision"].(map[string]any)
	line := project["items"].([]any)[0].(map[string]any)
	return gin.H{"version": 0, "demand_revision_id": revision["id"], "submit": true, "tax_mode": "included", "freight_mode": "fixed", "freight": "10.00", "valid_until": time.Now().Add(72 * time.Hour), "notes": "仅采购商可见的报价备注", "items": []gin.H{{"item_id": line["id"], "status": "quoted", "unit_price": price, "minimum_quantity": "1.000", "delivery_days": 10, "notes": "独立报价"}}}
}
func TestProcurementClosedLoopAndPrivacy(t *testing.T) {
	a := importTestApp(t)
	cat := procurementCategory(t, a)
	buyer, buyerToken := procurementTestAccount(t, a, "visitor")
	_, s1, v1, product := procurementSupplier(t, a, cat)
	_, s2, v2, _ := procurementSupplier(t, a, cat)
	admin := resourceTestSession(t, a, "admin")
	r := a.router()
	created := procTestDraft(t, a, r, buyerToken, product.ID, cat)
	pid := procID(created["id"])
	base := fmt.Sprintf("/procurement/projects/%d", pid)
	submitted := procSubmitTest(t, r, buyerToken, created, "direct")
	view := procCall(t, r, "GET", base, buyerToken, nil, 200)
	demandID := procID(view["revision"].(map[string]any)["id"])
	supplierView := procCall(t, r, "GET", base, s1, nil, 200)
	if supplierView["contact"] != nil || strings.Contains(fmt.Sprint(supplierView), *buyer.Phone) {
		t.Fatal("buyer contact exposed before intent")
	}
	procCall(t, r, "GET", base, s2, nil, 404)
	quote1 := procCall(t, r, "POST", base+"/quotes", s1, procurementQuoteTestPayload(view, "12.3456"), 200)
	expand := procCall(t, r, "POST", base+"/expand", buyerToken, gin.H{"version": submitted["version"], "audience": "matched"}, 200)
	var review ProcurementReview
	a.DB.Where("project_id=? AND status='pending'", pid).First(&review)
	procCall(t, r, "POST", fmt.Sprintf("/admin/procurement/reviews/%d", review.ID), admin, gin.H{"approved": true, "revision_id": review.RevisionID}, 200)
	view = procCall(t, r, "GET", base, buyerToken, nil, 200)
	if procID(view["revision"].(map[string]any)["id"]) != demandID {
		t.Fatal("expansion copied demand")
	}
	var projectCount int64
	a.DB.Model(&ProcurementProject{}).Count(&projectCount)
	if projectCount != 1 {
		t.Fatal("duplicated business data")
	}
	otherQuotes := procCall(t, r, "GET", base+"/quotes?vendor_id="+fmt.Sprint(v1.ID), s2, nil, 200)
	if len(otherQuotes["items"].([]any)) != 0 {
		t.Fatal("supplier sees competitor quote")
	}
	procCall(t, r, "POST", base+"/quotes", s2, procurementQuoteTestPayload(view, "15.0000"), 200)
	quotes := procCall(t, r, "GET", base+"/quotes", buyerToken, nil, 200)
	if len(quotes["items"].([]any)) != 2 {
		t.Fatal("comparison missing")
	}
	intent := procCall(t, r, "POST", base+fmt.Sprintf("/participants/%d/state", v1.ID), buyerToken, gin.H{"version": view["version"], "intent": true}, 200)
	supplierView = procCall(t, r, "GET", base, s1, nil, 200)
	if supplierView["contact"] == nil {
		t.Fatal("intent did not open contact")
	}
	supplierView = procCall(t, r, "GET", base, s2, nil, 200)
	if supplierView["contact"] != nil {
		t.Fatal("intent opened competitors' contact")
	}
	conversation := base + fmt.Sprintf("/messages/%d", v1.ID)
	procCall(t, r, "POST", conversation, buyerToken, gin.H{"body": "仅厂商一可见的沟通"}, 200)
	procCall(t, r, "POST", conversation, s1, gin.H{"body": "厂商回复"}, 200)
	procCall(t, r, "GET", conversation, s2, nil, 404)
	key := randomToken()
	message := gin.H{"body": "幂等消息"}
	procCall(t, r, "POST", conversation, buyerToken, message, 200, key)
	procCall(t, r, "POST", conversation, buyerToken, message, 200, key)
	procCall(t, r, "POST", conversation, buyerToken, gin.H{"body": "篡改重试"}, 409, key)
	var messageCount int64
	a.DB.Model(&ProcurementMessage{}).Where("body=?", "幂等消息").Count(&messageCount)
	if messageCount != 1 {
		t.Fatal("duplicate message")
	}
	updated := procCall(t, r, "PUT", base+"/draft", buyerToken, gin.H{"version": intent["version"], "title": "新版需求", "delivery_region": "1305", "deadline": time.Now().Add(48 * time.Hour), "items": []gin.H{{"name": "新版规格", "category_id": cat, "quantity": "3", "unit": "台"}}}, 200)
	procSubmitTest(t, r, buyerToken, updated, "matched")
	a.DB.Where("project_id=? AND status='pending'", pid).First(&review)
	// First clears the reused model ID, to query the new moderation request.
	review = ProcurementReview{}
	a.DB.Where("project_id=? AND status='pending'", pid).First(&review)
	procCall(t, r, "POST", fmt.Sprintf("/admin/procurement/reviews/%d", review.ID), admin, gin.H{"approved": true, "revision_id": review.RevisionID}, 200)
	quotes = procCall(t, r, "GET", base+"/quotes", buyerToken, nil, 200)
	for _, q := range quotes["items"].([]any) {
		h := q.(map[string]any)["history"].([]any)[0].(map[string]any)
		if h["stale"] != true || h["comparable_total"] != nil {
			t.Fatal("old quotes treated as current")
		}
	}
	old := procurementQuoteTestPayload(view, "9.00")
	old["version"] = quote1["quote"].(map[string]any)["version"]
	procCall(t, r, "POST", base+"/quotes", s1, old, 409)
	view = procCall(t, r, "GET", base, buyerToken, nil, 200)
	procCall(t, r, "POST", base+"/state", buyerToken, gin.H{"version": view["version"], "state": "completed"}, 200)
	procCall(t, r, "POST", conversation, s1, gin.H{"body": "关闭后发送"}, 409)
	_ = expand
	_ = v2
}
func TestProcurementPublicModerationAndEligibility(t *testing.T) {
	a := importTestApp(t)
	cat := procurementCategory(t, a)
	_, buyer := procurementTestAccount(t, a, "visitor")
	_, supplier, vendor, _ := procurementSupplier(t, a, cat)
	admin := resourceTestSession(t, a, "admin")
	r := a.router()
	p := procTestDraft(t, a, r, buyer, 0, cat)
	pid := procID(p["id"])
	base := fmt.Sprintf("/procurement/projects/%d", pid)
	procSubmitTest(t, r, buyer, p, "public")
	public := procCall(t, r, "GET", "/public/procurements", "", nil, 200)
	if public["total"] != float64(0) {
		t.Fatal("pending request public")
	}
	var review ProcurementReview
	a.DB.Where("project_id=?", pid).First(&review)
	procCall(t, r, "POST", fmt.Sprintf("/admin/procurement/reviews/%d", review.ID), supplier, gin.H{"approved": true, "revision_id": review.RevisionID}, 403)
	procCall(t, r, "POST", fmt.Sprintf("/admin/procurement/reviews/%d", review.ID), admin, gin.H{"approved": true, "revision_id": review.RevisionID}, 200)
	public = procCall(t, r, "GET", "/public/procurements", "", nil, 200)
	encoded, _ := json.Marshal(public)
	if public["total"] != float64(1) || strings.Contains(string(encoded), "参与厂商可见") || strings.Contains(string(encoded), "contact") || strings.Contains(string(encoded), "phone") || strings.Contains(string(encoded), "specification") {
		t.Fatal("unsafe public summary", string(encoded))
	}
	_, newSupplier, newVendor, _ := procurementSupplier(t, a, cat)
	procCall(t, r, "POST", base+"/join", newSupplier, gin.H{}, 200)
	noAccount, noSupplier := procurementTestAccount(t, a, "merchant")
	noVendor := v2Published(t, a, "vendor", 0, 1, 1, 1, false, 0, "无供货类别厂商")
	a.DB.Model(&noVendor).Update("owner_id", noAccount.ID)
	procCall(t, r, "POST", base+"/join", noSupplier, gin.H{}, 403)
	view := procCall(t, r, "GET", base, buyer, nil, 200)
	key := randomToken()
	invite := gin.H{"version": view["version"], "vendor_ids": []uint64{vendor.ID}}
	procCall(t, r, "POST", base+"/invite", buyer, invite, 200, key)
	procCall(t, r, "POST", base+"/invite", buyer, invite, 200, key)
	var n int64
	a.DB.Model(&ProcurementParticipant{}).Where("project_id=? AND vendor_id=?", pid, vendor.ID).Count(&n)
	if n != 1 {
		t.Fatal("duplicate invitation")
	}
	a.DB.Model(&ProcurementRevision{}).Where("id=?", review.RevisionID).Update("deadline", time.Now().Add(-time.Minute))
	view = procCall(t, r, "GET", base, newSupplier, nil, 200)
	payload := procurementQuoteTestPayload(view, "8.00")
	procCall(t, r, "POST", base+"/quotes", newSupplier, payload, 409)
	procCall(t, r, "POST", base+fmt.Sprintf("/messages/%d", newVendor.ID), buyer, gin.H{"body": "截止后继续沟通"}, 200)
	public = procCall(t, r, "GET", "/public/procurements", "", nil, 200)
	if public["total"] != float64(0) {
		t.Fatal("expired request visible")
	}
}
func TestProcurementAttachmentsAndBackup(t *testing.T) {
	a := importTestApp(t)
	cat := procurementCategory(t, a)
	_, buyer := procurementTestAccount(t, a, "visitor")
	_, supplier, _, product := procurementSupplier(t, a, cat)
	_, stranger := procurementTestAccount(t, a, "visitor")
	r := a.router()
	p := procTestDraft(t, a, r, buyer, product.ID, cat)
	pid := procID(p["id"])
	base := fmt.Sprintf("/procurement/projects/%d", pid)
	var data bytes.Buffer
	png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	var upload bytes.Buffer
	form := multipart.NewWriter(&upload)
	part, _ := form.CreateFormFile("file", "技术规格.png")
	part.Write(data.Bytes())
	form.Close()
	request := httptest.NewRequest("POST", "/api/v1"+base+"/attachments", &upload)
	request.Header.Set("Content-Type", form.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+buyer)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, request)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var file ProcurementAttachment
	json.Unmarshal(w.Body.Bytes(), &file)
	read := func(token string, want int) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/procurement/attachments/%d", file.ID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		out := httptest.NewRecorder()
		r.ServeHTTP(out, req)
		if out.Code != want {
			t.Fatal(out.Code, out.Body.String())
		}
	}
	read(buyer, 200)
	read(stranger, 404)
	read(supplier, 404)
	saved := procCall(t, r, "PUT", base+"/draft", buyer, gin.H{"version": p["version"], "title": "有附件的采购需求", "delivery_region": "1305", "deadline": time.Now().Add(24 * time.Hour), "items": []gin.H{{"name": "农机", "category_id": cat, "quantity": "1", "unit": "台"}}, "attachment_ids": []uint64{file.ID}}, 200)
	procSubmitTest(t, r, buyer, saved, "direct")
	read(supplier, 200)
	objects, e := a.inventory()
	if e != nil {
		t.Fatal(e)
	}
	if e = validateMediaInventory(a.DB, objects); e != nil {
		t.Fatal(e)
	}
	if e = validateMediaInventory(a.DB, nil); e == nil {
		t.Fatal("missing attachment accepted by restore")
	}
	a.DB.Model(&ProcurementAttachment{}).Where("id=?", file.ID).Update("scope_id", 99999999)
	if e = validateMediaInventory(a.DB, objects); e == nil {
		t.Fatal("invalid attachment scope accepted by restore")
	}
	a.DB.Model(&ProcurementAttachment{}).Where("id=?", file.ID).Update("scope_id", procID(procCall(t, r, "GET", base, buyer, nil, 200)["revision"].(map[string]any)["id"]))
	a.DB.Exec("DELETE FROM schema_migrations WHERE version=21")
	if e = migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	read(buyer, 200)
	for _, extension := range []string{".docx", ".xlsx"} {
		full := filepath.Join(t.TempDir(), "fake"+extension)
		os.WriteFile(full, []byte("not an Office document"), 0600)
		if _, e = validateProcurementFile(full, extension); e == nil {
			t.Fatal("fake Office accepted")
		}
	}
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	for name, content := range map[string]string{"[Content_Types].xml": "macroEnabled", "word/document.xml": "doc", "word/vbaProject.bin": "macro"} {
		f, _ := zw.Create(name)
		f.Write([]byte(content))
	}
	zw.Close()
	full := filepath.Join(t.TempDir(), "macro.docx")
	os.WriteFile(full, archive.Bytes(), 0600)
	if _, e = validateProcurementFile(full, ".docx"); e == nil {
		t.Fatal("macro accepted")
	}
}
func TestProcurementNotificationLimitsAndDecimalTotals(t *testing.T) {
	a := importTestApp(t)
	account, _ := procurementTestAccount(t, a, "merchant")
	if e := a.DB.Transaction(func(tx *gorm.DB) error {
		for i := 0; i < 25; i++ {
			if e := a.procNotice(tx, account.ID, uint64(i+1), "invitation", "商机", fmt.Sprintf("event-%d", i), true); e != nil {
				return e
			}
		}
		return a.procNotice(tx, account.ID, 1, "invitation", "重复", "event-1", true)
	}); e != nil {
		t.Fatal(e)
	}
	var instant, digest int64
	a.DB.Model(&ProcurementNotification{}).Where("account_id=? AND kind='opportunity'", account.ID).Count(&instant)
	a.DB.Model(&ProcurementNotification{}).Where("account_id=? AND kind='digest'", account.ID).Count(&digest)
	if instant != 20 || digest != 1 {
		t.Fatal(instant, digest)
	}
	a.DB.Model(&ProcurementPreference{}).Where("account_id=?", account.ID).Update("opportunity_mode", "off")
	if e := a.DB.Transaction(func(tx *gorm.DB) error {
		if e := a.procNotice(tx, account.ID, 1, "invitation", "关闭提醒", "off", true); e != nil {
			return e
		}
		return a.procNotice(tx, account.ID, 1, "message", "回复", "direct-message", false)
	}); e != nil {
		t.Fatal(e)
	}
	var n int64
	a.DB.Model(&ProcurementNotification{}).Where("event_key='off'").Count(&n)
	if n != 0 {
		t.Fatal("off ignored")
	}
	a.DB.Model(&ProcurementNotification{}).Where("event_key='direct-message'").Count(&n)
	if n != 1 {
		t.Fatal("critical message suppressed")
	}
	quote := ProcurementQuote{}
	revision := ProcurementQuoteRevision{Status: "submitted", DemandRevisionID: 1, TaxMode: "included", FreightMode: "fixed", Freight: "10.00", ValidUntil: time.Now().Add(time.Hour)}
	lines := []ProcurementQuoteItem{{ItemID: 1, Status: "quoted", UnitPrice: "12.3456", MinimumQuantity: "1"}}
	items := []ProcurementItem{{ID: 1, Quantity: "2.500"}}
	if total := procComparableTotal(quote, revision, lines, items, 1); total != "40.86" {
		t.Fatal("decimal total", total)
	}
	items = append(items, ProcurementItem{ID: 2, Quantity: "1"})
	if procComparableTotal(quote, revision, lines, items, 1) != nil {
		t.Fatal("partial quote counted as complete")
	}
	for _, s := range []string{"-1", "NaN", "1e8", "1.00001", "999999999999999999999"} {
		if procNumber(s, 4, true) {
			t.Fatal("invalid decimal accepted", s)
		}
	}
}
