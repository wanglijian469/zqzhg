package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestProcurementPublicMobileSummary(t *testing.T) {
	a := importTestApp(t)
	cat := procurementCategory(t, a)
	_, buyer := procurementTestAccount(t, a, "visitor")
	admin := resourceTestSession(t, a, "admin")
	r := a.router()
	draft := procTestDraft(t, a, r, buyer, 0, cat)
	id := procID(draft["id"])
	detail := fmt.Sprintf("/public/procurements/%d", id)
	procCall(t, r, "GET", detail, "", nil, 404)
	procSubmitTest(t, r, buyer, draft, "public")
	procCall(t, r, "GET", detail, "", nil, 404)
	var review ProcurementReview
	a.DB.Where("project_id=?", id).First(&review)
	procCall(t, r, "POST", fmt.Sprintf("/admin/procurement/reviews/%d", review.ID), admin, gin.H{"approved": true, "revision_id": review.RevisionID}, 200)
	var before int64
	a.DB.Model(&ProcurementParticipant{}).Where("project_id=?", id).Count(&before)
	summary := procCall(t, r, "GET", detail, "", nil, 200)
	if summary["published_at"] == nil {
		t.Fatal("review time missing")
	}
	raw, _ := json.Marshal(summary)
	for _, forbidden := range []string{"contact", "phone", "specification", "attachments", "quotes", "notes", "buyer_id"} {
		if strings.Contains(string(raw), `"`+forbidden+`"`) {
			t.Fatal("private field leaked", forbidden)
		}
	}
	var after int64
	a.DB.Model(&ProcurementParticipant{}).Where("project_id=?", id).Count(&after)
	if before != after {
		t.Fatal("reading a summary enrolled a supplier")
	}
	list := procCall(t, r, "GET", "/public/procurements?q="+url.QueryEscape("测试农机"), "", nil, 200)
	if list["total"] != float64(1) {
		t.Fatal("item name was not searchable", list)
	}
	list = procCall(t, r, "GET", "/public/procurements?q="+url.QueryEscape("参与厂商可见"), "", nil, 200)
	if list["total"] != float64(0) {
		t.Fatal("private notes affected public search")
	}
	a.DB.Model(&ProcurementReview{}).Where("id=?", review.ID).Update("reviewed_at", nil)
	summary = procCall(t, r, "GET", detail, "", nil, 200)
	if summary["published_at"] != nil {
		t.Fatal("missing historical time was fabricated")
	}
	a.DB.Model(&ProcurementProject{}).Where("id=?", id).Update("suspended", true)
	procCall(t, r, "GET", detail, "", nil, 404)
	a.DB.Model(&ProcurementProject{}).Where("id=?", id).Updates(map[string]any{"suspended": false, "audience": "direct"})
	procCall(t, r, "GET", detail, "", nil, 404)
	a.DB.Model(&ProcurementProject{}).Where("id=?", id).Update("audience", "public")
	a.DB.Model(&ProcurementRevision{}).Where("id=?", review.RevisionID).Update("deadline", time.Now().Add(-time.Minute))
	procCall(t, r, "GET", detail, "", nil, 404)
}
