package main

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestKuaishouVideoShareValidation(t *testing.T) {
	check, err := checkKuaishou("分享给你 https://v.kuaishou.com/Abc123/?shareToken=123。")
	if err != nil || check.CanonicalURL != "https://v.kuaishou.com/Abc123" || check.VideoID != "share:Abc123" || check.Status != "EXTERNAL_ONLY" || check.PlayerURL != "" {
		t.Fatalf("unexpected normalized link: %+v %v", check, err)
	}
	for _, raw := range []string{
		"http://v.kuaishou.com/Abc123",
		"https://v.kuaishou.com.evil.example/Abc123",
		"https://evil.example@v.kuaishou.com/Abc123",
		"https://v.kuaishou.com:443/Abc123",
		"https://v.kuaishou.com/Abc123#video",
		"https://v.kuaishou.com/%41bc123",
		"https://www.kuaishou.com/profile/Abc123",
		"https://v.kuaishou.com/Abc123 https://v.kuaishou.com/Def456",
	} {
		if _, err := checkKuaishou(raw); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
}

func TestVideoGalleryV3ReviewAndCompatibility(t *testing.T) {
	a := importTestApp(t)
	t.Setenv("DOUYIN_VIDEO_ENABLED", "false")
	tc := testClient{t, a.router()}
	merchant := resourceTestSession(t, a, "merchant")
	admin := resourceTestSession(t, a, "admin")
	vendor := tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{
		"kind": "vendor", "data": Payload{Name: "视频栏目隔离测试", MainProducts: "机械", RegionName: "河北", Phone: "13900009181"},
	}, 201)
	tc.call("POST", "/merchant/service-acceptances", merchant, gin.H{"agreements": serviceTestRefs(t, a, "terms", "privacy", "showroom-agreement"), "source": "web"}, 200)
	tc.call("POST", "/merchant/contents/"+number(vendor["id"])+"/submit", merchant, gin.H{"revision_id": vendor["revision_id"]}, 200)
	tc.call("POST", "/admin/revisions/"+number(vendor["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)

	const v3 = "?video_api_version=3"
	const v2 = "?video_api_version=2"
	link := gin.H{"source": videoKuaishou, "text": "https://v.kuaishou.com/Abc123/"}
	tc.call("POST", "/merchant/video-links/check"+v2, merchant, link, 409)
	tc.call("POST", "/merchant/video-links/check"+v3, merchant, link, 200)
	p := ResourcePayload{Title: "快手测试", Purpose: "other", Format: "video", VideoSource: videoKuaishou, OriginalShareURL: link["text"].(string)}
	tc.call("POST", "/merchant/resources"+v2, merchant, gin.H{"kind": "video", "data": p}, 409)
	r := tc.call("POST", "/merchant/resources"+v3, merchant, gin.H{"kind": "video", "data": p}, 201)
	p.OriginalShareURL = "https://v.kuaishou.com/Abc123?tracking=abc"
	tc.call("POST", "/merchant/resources"+v3, merchant, gin.H{"kind": "video", "data": p}, 409)
	rid := number(r["id"])
	pub := "/public/resources/" + rid
	merchantPath := "/merchant/resources/" + rid
	publish := func(rev any) {
		tc.call("POST", merchantPath+"/submit"+v3, merchant, gin.H{"revision_id": rev}, 200)
		tc.call("POST", "/admin/resource-revisions/"+number(rev)+"/review"+v3, admin, gin.H{"decision": "approve"}, 200)
	}
	tc.call("GET", pub+v3, "", nil, 404)
	publish(r["revision_id"])
	tc.call("GET", pub, "", nil, 404)
	tc.call("GET", pub+v2, "", nil, 404)
	tc.call("GET", merchantPath+v2, merchant, nil, 409)
	tc.call("GET", pub+"/playback"+v2, "", nil, 409)
	play := tc.call("GET", pub+"/playback"+v3, "", nil, 200)
	if play["status"] != "EXTERNAL_ONLY" || play["canonical_url"] != "https://v.kuaishou.com/Abc123" || play["player_url"] != nil {
		t.Fatal(play)
	}
	listPath := "/public/resources?kind=video&purpose=other&vendor_id=" + number(vendor["id"])
	for _, version := range []string{"2", "3"} {
		list := tc.call("GET", listPath+"&video_api_version="+version, "", nil, 200)
		if (list["total"] == float64(1)) != (version == "3") {
			t.Fatal(list)
		}
	}
	var cover bytes.Buffer
	if err := png.Encode(&cover, image.NewRGBA(image.Rect(0, 0, 36, 64))); err != nil {
		t.Fatal(err)
	}
	img := resourceUpload(t, a, merchant, "/merchant/media", "portrait.png", cover.Bytes(), 201)
	p.CoverMediaID = uint64(img["id"].(float64))
	p.Description = "这段简介来自隔离测试数据。"
	draft := tc.call("PUT", merchantPath+"/draft"+v3, merchant, gin.H{"revision_id": r["revision_id"], "data": p}, 200)
	old := tc.call("GET", pub+v3, "", nil, 200)["data"].(map[string]any)
	if old["cover_media_id"] != float64(0) || old["description"] != "" {
		t.Fatal("unapproved edits leaked", old)
	}
	publish(draft["revision_id"])
	data := tc.call("GET", pub+v3, "", nil, 200)["data"].(map[string]any)
	if data["cover_media_id"] != img["id"] || data["description"] != p.Description {
		t.Fatal("published edits missing", data)
	}
}
