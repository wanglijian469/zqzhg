package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"sync"
	"testing"
)

func TestShowroomRepresentativesIsolationVersionsAndVisibility(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	admin := resourceTestSession(t, a, "admin")
	merchant := tc.login("13900008401")
	other := tc.login("13900008402")
	create := func(token, kind, name string) map[string]any {
		p := Payload{Name: name, CategoryID: 1}
		if kind == "vendor" {
			p.MainProducts = "测试设备"
			p.RegionName = "河北"
			p.Phone = "13900008401"
		}
		return tc.fixtureCall("POST", "/merchant/contents", token, gin.H{"kind": kind, "data": p}, 201)
	}
	publish := func(token string, item map[string]any) {
		tc.call("POST", "/merchant/contents/"+number(item["id"])+"/submit", token, gin.H{"revision_id": item["revision_id"]}, 200)
		tc.call("POST", "/admin/revisions/"+number(item["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	}
	tc.call("GET", "/merchant/showroom-display", "", nil, 401)
	tc.call("PUT", "/merchant/showroom-display", admin, gin.H{"mode": "auto", "version": 0}, 403)
	tc.call("GET", "/merchant/showroom-display", merchant, nil, 409)
	vendor := create(merchant, "vendor", "代表展品验收厂商")
	publish(merchant, vendor)
	foreignVendor := create(other, "vendor", "其他厂商")
	publish(other, foreignVendor)
	foreign := create(other, "product", "其他账号产品")
	publish(other, foreign)
	products := []map[string]any{}
	for i := 0; i < 10; i++ {
		p := create(merchant, "product", fmt.Sprintf("公开产品%d", i))
		publish(merchant, p)
		products = append(products, p)
	}
	draft := create(merchant, "product", "草稿不能入选")
	get := tc.call("GET", "/merchant/showroom-display", merchant, nil, 200)
	if get["mode"] != "auto" || get["version"] != float64(0) || len(get["preview_products"].([]any)) != 3 {
		t.Fatal(get)
	}
	path := "/public/showrooms/v-" + number(vendor["id"])
	public := tc.call("GET", path, "", nil, 200)
	home := public["home_products"].([]any)
	if len(home) != 8 {
		t.Fatal("homepage limit", len(home))
	}
	seen := map[any]bool{}
	for _, raw := range home {
		row := raw.(map[string]any)
		if seen[row["id"]] || row["vendor_id"] != vendor["id"] {
			t.Fatal("duplicate or foreign homepage product", row)
		}
		seen[row["id"]] = true
	}
	auto := public["representative_products"].([]any)
	if auto[0].(map[string]any)["id"] != products[9]["id"] {
		t.Fatal("automatic order changed")
	}
	save := func(ids []any, version any, status int) map[string]any {
		return tc.call("PUT", "/merchant/showroom-display", merchant, gin.H{"mode": "manual", "product_ids": ids, "version": version}, status)
	}
	save([]any{products[0]["id"], products[1]["id"], products[2]["id"], products[3]["id"]}, 0, 409)
	save([]any{products[0]["id"], products[0]["id"]}, 0, 400)
	save([]any{foreign["id"]}, 0, 400)
	save([]any{draft["id"]}, 0, 400)
	ids := []any{products[1]["id"], products[0]["id"], products[2]["id"]}
	saved := save(ids, 0, 200)
	save(ids, 0, 409)
	public = tc.call("GET", path, "", nil, 200)
	if public["home_products"].([]any)[0].(map[string]any)["id"] != products[1]["id"] {
		t.Fatal("homepage manual priority ignored")
	}
	if public["representative_products"].([]any)[0].(map[string]any)["id"] != products[1]["id"] {
		t.Fatal("manual order ignored")
	}
	p := products[1]
	changed := tc.fixtureCall("PUT", "/merchant/contents/"+number(p["id"])+"/draft", merchant, gin.H{"revision_id": p["revision_id"], "data": Payload{Name: "未审核私有改名", CategoryID: 1}}, 200)
	tc.call("POST", "/merchant/contents/"+number(p["id"])+"/submit", merchant, gin.H{"revision_id": changed["revision_id"]}, 200)
	public = tc.call("GET", path, "", nil, 200)
	if public["representative_products"].([]any)[0].(map[string]any)["data"].(map[string]any)["name"] == "未审核私有改名" {
		t.Fatal("private revision exposed")
	}
	saved = save(ids, saved["version"], 200)
	tc.call("POST", "/merchant/contents/"+number(products[0]["id"])+"/offline", merchant, gin.H{}, 200)
	public = tc.call("GET", path, "", nil, 200)
	if len(public["representative_products"].([]any)) != 2 {
		t.Fatal("offline product visible")
	}
	state := tc.call("GET", "/merchant/showroom-display", merchant, nil, 200)
	if state["selected"].([]any)[1].(map[string]any)["public_visible"] != false {
		t.Fatal("invalid selection not indicated")
	}
	save(ids, saved["version"], 400)
	// Restoring the same approved public version restores its configured slot.
	a.DB.Model(&Content{}).Where("id=?", products[0]["id"]).Update("visible", true)
	if len(tc.call("GET", path, "", nil, 200)["representative_products"].([]any)) != 3 {
		t.Fatal("republish did not restore selection")
	}
	saved = save([]any{}, saved["version"], 200)
	if len(tc.call("GET", path, "", nil, 200)["home_products"].([]any)) != 0 {
		t.Fatal("manual empty homepage must hide")
	}
	if len(tc.call("GET", path, "", nil, 200)["representative_products"].([]any)) != 0 {
		t.Fatal("manual empty must hide")
	}
	tc.call("PUT", "/merchant/showroom-display", merchant, gin.H{"mode": "auto", "product_ids": []any{}, "version": saved["version"]}, 200)
	tc.call("POST", "/merchant/contents/"+number(vendor["id"])+"/offline", merchant, gin.H{}, 200)
	tc.call("GET", path, "", nil, 404)
	state = tc.call("GET", "/merchant/showroom-display", merchant, nil, 200)
	if len(state["preview_products"].([]any)) != 0 {
		t.Fatal("parent visibility bypass")
	}
	var audit int64
	a.DB.Model(&OperationLog{}).Where("action='showroom_representatives_saved'").Count(&audit)
	if audit != 4 {
		t.Fatal("unexpected audit count", audit)
	}
}

func TestShowroomDisplayConcurrentRollbackAndMigration(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	m := tc.login("13900008403")
	tc.fixtureCall("POST", "/merchant/contents", m, gin.H{"kind": "vendor", "data": Payload{Name: "并发配置厂商", MainProducts: "机械", RegionName: "河北", Phone: "13900008403"}}, 201)
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := homePutRequest(a.router(), "/merchant/showroom-display", m, gin.H{"mode": "manual", "product_ids": []any{}, "version": 0})
			codes <- r.Code
		}()
	}
	wg.Wait()
	close(codes)
	success, conflict := 0, 0
	for c := range codes {
		if c == 200 {
			success++
		} else if c == 409 {
			conflict++
		} else {
			t.Fatal(c)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal(success, conflict)
	}
	a.DB.Callback().Create().Before("gorm:create").Register("display_log_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "operation_logs" {
			tx.AddError(errors.New("isolated log failure"))
		}
	})
	tc.call("PUT", "/merchant/showroom-display", m, gin.H{"mode": "auto", "product_ids": []any{}, "version": 1}, 500)
	a.DB.Callback().Create().Remove("display_log_failure")
	state := tc.call("GET", "/merchant/showroom-display", m, nil, 200)
	if state["version"] != float64(1) || state["mode"] != "manual" {
		t.Fatal("transaction did not roll back")
	}
	a.DB.Exec("DELETE FROM schema_migrations WHERE version=8")
	if e := migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	if e := migrate(a.DB); e != nil {
		t.Fatal(e)
	}
	var row ShowroomDisplaySettings
	a.DB.First(&row)
	b, _ := json.Marshal(row)
	if row.Version != 1 || row.Mode != "manual" {
		t.Fatal(string(b))
	}
}
