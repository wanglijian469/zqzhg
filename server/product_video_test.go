package main

import (
	"github.com/gin-gonic/gin"
	"sync/atomic"
	"testing"
)

func TestProductVideoAssociationAndPublication(t *testing.T) {
	a := importTestApp(t)
	t.Setenv("DOUYIN_VIDEO_ENABLED", "true")
	var calls atomic.Int32
	mockVideoClient(a, &calls)
	tc := testClient{t, a.router()}
	merchant := resourceTestSession(t, a, "merchant")
	admin := resourceTestSession(t, a, "admin")
	other := resourceTestSession(t, a, "merchant")
	vendor := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "产品视频隔离厂商", MainProducts: "机械", RegionName: "河北", Phone: "13900009188"}}, 201)
	tc.call("POST", "/merchant/service-acceptances", merchant, gin.H{"agreements": serviceTestRefs(t, a, "terms", "privacy", "showroom-agreement"), "source": "web"}, 200)
	approve := func(item map[string]any) {
		tc.call("POST", "/merchant/contents/"+number(item["id"])+"/submit", merchant, gin.H{"revision_id": item["revision_id"]}, 200)
		tc.call("POST", "/admin/revisions/"+number(item["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	}
	approve(vendor)
	imageID := appFlowProductImage(t, a, merchant)
	first := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "product", "data": Payload{Name: "产品甲", CategoryID: 1, MediaIDs: []uint64{imageID}}}, 201)
	second := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "product", "data": Payload{Name: "产品乙", CategoryID: 1, MediaIDs: []uint64{imageID}}}, 201)
	p := ResourcePayload{Title: "产品抖音视频", Format: "video", Purpose: "product", ProductID: uint64(first["id"].(float64)), VideoSource: videoDouyin, OriginalShareURL: "https://www.douyin.com/video/7690195495465012480", RightsConfirmed: true}
	resource := tc.call("POST", "/merchant/resources?video_api_version=2", merchant, gin.H{"kind": "video", "data": p}, 201)
	rid := number(resource["id"])
	publishVideo := func(r map[string]any) {
		tc.call("POST", "/merchant/resources/"+rid+"/submit?video_api_version=2", merchant, gin.H{"revision_id": r["revision_id"]}, 200)
		tc.call("POST", "/admin/resource-revisions/"+number(r["revision_id"])+"/review?video_api_version=2", admin, gin.H{"decision": "approve"}, 200)
	}
	publishVideo(resource)
	tc.call("GET", "/public/resources/"+rid+"?video_api_version=2", "", nil, 404)
	approve(first)
	tc.call("GET", "/public/resources/"+rid+"?video_api_version=2", "", nil, 200)
	p.ProductID = uint64(second["id"].(float64))
	draft := tc.call("PUT", "/merchant/resources/"+rid+"/draft?video_api_version=2", merchant, gin.H{"revision_id": resource["revision_id"], "data": p}, 200)
	pub := tc.call("GET", "/public/resources/"+rid+"?video_api_version=2", "", nil, 200)
	if pub["data"].(map[string]any)["product_id"] != first["id"] {
		t.Fatal("pending product association leaked")
	}
	publishVideo(draft)
	tc.call("GET", "/public/resources/"+rid+"?video_api_version=2", "", nil, 404)
	approve(second)
	pub = tc.call("GET", "/public/resources/"+rid+"?video_api_version=2", "", nil, 200)
	if pub["data"].(map[string]any)["product_id"] != second["id"] {
		t.Fatal("wrong public product")
	}
	list := tc.call("GET", "/public/resources?video_api_version=2&kind=video&product_id="+number(first["id"]), "", nil, 200)
	if list["total"] != float64(0) {
		t.Fatal("video remained on old product")
	}
	p.ProductID = uint64(first["id"].(float64))
	tc.call("POST", "/merchant/resources?video_api_version=2", merchant, gin.H{"kind": "video", "data": p}, 409)
	tc.call("PUT", "/merchant/resources/"+rid+"/draft?video_api_version=2", other, gin.H{"revision_id": draft["revision_id"], "data": p}, 403)
}
