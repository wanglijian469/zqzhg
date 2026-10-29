package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Counts statements without recording SQL, parameters or response bodies.
type procQueryCounter struct {
	logger.Interface
	count atomic.Int64
}

func (l *procQueryCounter) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	l.count.Add(1)
	l.Interface.Trace(ctx, begin, fc, err)
}

func TestProcurementListPerformance(t *testing.T) {
	a := importTestApp(t)
	var database string
	if err := a.DB.Raw("SELECT DATABASE()").Scan(&database).Error; err != nil || !strings.HasPrefix(database, "zqzt_exhibition_test_") {
		t.Fatal("isolated test database required", err)
	}
	buyer, token := procurementTestAccount(t, a, "visitor")
	counter := &procQueryCounter{Interface: a.DB.Logger}
	a.DB = a.DB.Session(&gorm.Session{Logger: counter})
	router := a.router()
	metrics := []map[string]any{}
	for _, total := range []int{1, 20, 100} {
		var existing int64
		a.DB.Model(&ProcurementProject{}).Count(&existing)
		for i := int(existing); i < total; i++ {
			p := ProcurementProject{BuyerID: buyer.ID, HallID: 1, State: "open", Audience: "direct", Version: 1}
			if err := a.DB.Create(&p).Error; err != nil {
				t.Fatal(err)
			}
			for version := 0; version < 3; version++ {
				r := ProcurementRevision{ProjectID: p.ID, Status: "approved", VendorIDs: "[]", Title: fmt.Sprintf("合成性能项目%d", i), DeliveryRegion: "1305", Deadline: time.Now().Add(24 * time.Hour), Notes: strings.Repeat("隔离合成说明", 100), ContactName: "合成联系人", Phone: "13999999999"}
				if err := a.DB.Create(&r).Error; err != nil {
					t.Fatal(err)
				}
				if err := a.DB.Create(&[]ProcurementItem{{RevisionID: r.ID, Name: "合成产品甲", Quantity: "2.500", Unit: "台"}, {RevisionID: r.ID, Name: "合成产品乙", Quantity: "1.000", Unit: "台"}}).Error; err != nil {
					t.Fatal(err)
				}
				p.DraftRevisionID = r.ID
				p.PublishedRevisionID = r.ID
			}
			if err := a.DB.Save(&p).Error; err != nil {
				t.Fatal(err)
			}
			events := make([]ProcurementEvent, 10)
			for n := range events {
				events[n] = ProcurementEvent{ProjectID: p.ID, ActorID: buyer.ID, Action: "fixture"}
			}
			if err := a.DB.Create(&events).Error; err != nil {
				t.Fatal(err)
			}
		}
		for _, view := range []string{"legacy", "summary"} {
			latencies := []float64{}
			var queries int64
			size := 0
			for n := 0; n < 11; n++ {
				path := "/api/v1/procurement/projects"
				if view == "summary" {
					path += "?view=summary"
				}
				req := httptest.NewRequest("GET", path, nil)
				req.Header.Set("Authorization", "Bearer "+token)
				w := httptest.NewRecorder()
				counter.count.Store(0)
				start := time.Now()
				router.ServeHTTP(w, req)
				if w.Code != 200 {
					t.Fatalf("list status %d", w.Code)
				}
				if n > 0 {
					latencies = append(latencies, float64(time.Since(start).Microseconds())/1000)
				}
				queries = counter.count.Load()
				size = w.Body.Len()
			}
			sort.Float64s(latencies)
			metric := map[string]any{"projects": total, "view": view, "queries": queries, "bytes": size, "p50_ms": latencies[4], "p95_ms": latencies[9]}
			if view == "summary" && queries > 7 {
				t.Fatalf("summary query count grows with page size: %d", queries)
			}
			metrics = append(metrics, metric)
			raw, _ := json.Marshal(metric)
			t.Log(string(raw))
		}
	}
	if path := os.Getenv("PROC_LIST_METRICS"); path != "" {
		raw, _ := json.MarshalIndent(metrics, "", "  ")
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProcurementSummaryAuthorizationAndContract(t *testing.T) {
	a := importTestApp(t)
	r := a.router()
	cat := procurementCategory(t, a)
	_, buyer := procurementTestAccount(t, a, "visitor")
	_, stranger := procurementTestAccount(t, a, "visitor")
	_, supplier, _, product := procurementSupplier(t, a, cat)
	_, other, _, otherProduct := procurementSupplier(t, a, cat)
	admin := resourceTestSession(t, a, "admin")
	p := procTestDraft(t, a, r, buyer, product.ID, cat)
	procSubmitTest(t, r, buyer, p, "direct")
	for _, token := range []string{buyer, admin, supplier} {
		root := "/procurement/projects?view=summary"
		if token == supplier {
			root += "&scope=supplier"
		}
		if token == admin {
			root = "/admin/procurement/projects?view=summary"
		}
		out := procCall(t, r, "GET", root, token, nil, 200)
		if len(out["items"].([]any)) != 1 {
			t.Fatal("missing authorized summary")
		}
		row := out["items"].([]any)[0].(map[string]any)
		for _, key := range []string{"contact", "attachments", "participants", "events", "reviews", "versions", "requested_vendor_ids"} {
			if _, ok := row[key]; ok {
				t.Error("detail leaked in summary", key)
			}
		}
		if _, ok := row["revision"].(map[string]any)["notes"]; ok {
			t.Error("notes leaked")
		}
		if _, ok := row["items"].([]any)[0].(map[string]any)["specification"]; ok {
			t.Error("specification leaked")
		}
	}
	for _, path := range []string{"/procurement/projects?view=summary", "/procurement/projects?view=summary&scope=supplier"} {
		for _, token := range []string{stranger, other} {
			out := procCall(t, r, "GET", path, token, nil, 200)
			if out["total"] != float64(0) {
				t.Error("foreign project in list")
			}
		}
	}
	legacy := procCall(t, r, "GET", "/procurement/projects", buyer, nil, 200)
	if legacy["items"].([]any)[0].(map[string]any)["contact"] == nil {
		t.Error("legacy response contract changed")
	}
	// Even a corrupt participation row pointing at a product cannot grant supplier access.
	if err := a.DB.Create(&ProcurementParticipant{ProjectID: procID(p["id"]), VendorID: otherProduct.ID, State: "invited", Source: "fixture"}).Error; err != nil {
		t.Fatal(err)
	}
	procCall(t, r, "GET", "/procurement/projects?scope=supplier&view=summary", other, nil, 404)
	procCall(t, r, "GET", "/procurement/projects?scope=supplier", other, nil, 404)
}
