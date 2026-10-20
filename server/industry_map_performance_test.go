package main

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestIndustryMapAggregateAtScale(t *testing.T) {
	a := importTestApp(t)
	const size = 6000
	contents := make([]Content, size)
	for i := range contents {
		contents[i] = Content{Kind: "vendor", HallID: 1, Visible: true}
	}
	if e := a.DB.CreateInBatches(&contents, 250).Error; e != nil {
		t.Fatal(e)
	}
	revisions := make([]Revision, size)
	for i, c := range contents {
		code := "110108"
		if i%100 == 0 {
			code = "130528"
		}
		p := Payload{Name: fmt.Sprintf("规模统计夹具%04d（仅测试）", i), RegionCode: code}
		blob, _ := json.Marshal(p)
		revisions[i] = Revision{ContentID: c.ID, Status: "approved", Name: p.Name, Payload: string(blob), AdministrativeSnapshot: administrativeSnapshot(p, Region{})}
	}
	if e := a.DB.CreateInBatches(&revisions, 250).Error; e != nil {
		t.Fatal(e)
	}
	if e := a.DB.Exec("UPDATE contents c JOIN revisions r ON r.content_id=c.id SET c.published_revision_id=r.id").Error; e != nil {
		t.Fatal(e)
	}
	if e := a.DB.Exec("ANALYZE TABLE contents,revisions").Error; e != nil {
		t.Fatal(e)
	}
	started := time.Now()
	tc := testClient{t, a.router()}
	result := tc.call("GET", "/public/industry-map/regions?region_code=13", "", nil, 200)
	if mapCount(t, result, "1305") != 60 {
		t.Fatal(result)
	}
	t.Logf("6000-vendor filtered statistics HTTP request: %s (development fixture; not a production SLA)", time.Since(started))
	var explain []map[string]any
	if e := a.DB.Raw("EXPLAIN SELECT r.city_code,COUNT(DISTINCT c.id) FROM contents c JOIN revisions r ON r.id=c.published_revision_id AND r.status='approved' WHERE c.kind='vendor' AND c.visible=1 AND c.hall_id=1 AND r.province_code='13' AND EXISTS(SELECT 1 FROM halls h WHERE h.id=c.hall_id AND h.status='open') GROUP BY r.city_code").Scan(&explain).Error; e != nil {
		t.Fatal(e)
	}
	data, _ := json.Marshal(explain)
	t.Log("6000-vendor EXPLAIN:", string(data))
}
