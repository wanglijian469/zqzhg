package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"testing"
)

func TestVideoUnresolvedShortLinkDraft(t *testing.T) {
	a := importTestApp(t)
	t.Setenv("DOUYIN_VIDEO_ENABLED", "true")
	a.videoHTTP = &http.Client{Transport: videoRoundTrip(func(*http.Request) (*http.Response, error) { return nil, context.DeadlineExceeded })}
	tc := testClient{t, a.router()}
	merchant := resourceTestSession(t, a, "merchant")
	tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "短链接隔离厂商", RegionName: "河北", MainProducts: "机械", Phone: "13900009187"}}, 201)
	p := ResourcePayload{Title: "合法短链接暂存", Format: "video", Purpose: "company", VideoSource: videoDouyin, OriginalShareURL: "https://v.douyin.com/example/", RightsConfirmed: true}
	check := tc.call("POST", "/merchant/video-links/check?video_api_version=2", merchant, gin.H{"text": p.OriginalShareURL}, 200)["check"].(map[string]any)
	if check["status"] != "TEMPORARY_ERROR" || check["reason"] != "video_network_timeout" || check["video_id"] != "" {
		t.Fatal(check)
	}
	r := tc.call("POST", "/merchant/resources?video_api_version=2", merchant, gin.H{"kind": "video", "data": p}, 201)
	tc.call("POST", "/merchant/resources?video_api_version=2", merchant, gin.H{"kind": "video", "data": p}, 409)
	tc.call("POST", "/merchant/resources/"+number(r["id"])+"/submit?video_api_version=2", merchant, gin.H{"revision_id": r["revision_id"]}, 409)
	tc.call("GET", "/public/resources/"+number(r["id"])+"?video_api_version=2", "", nil, 404)
	a.videoHTTP = &http.Client{Transport: videoRoundTrip(func(*http.Request) (*http.Response, error) { return nil, fmt.Errorf("blocked_video_destination") })}
	p.OriginalShareURL = "https://v.douyin.com/another/"
	tc.call("POST", "/merchant/resources?video_api_version=2", merchant, gin.H{"kind": "video", "data": p}, 400)
}
