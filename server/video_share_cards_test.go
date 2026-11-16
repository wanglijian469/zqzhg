package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestVideoShareCardsBrowser(t *testing.T) {
	if os.Getenv("RUN_VIDEO_SHARE_BROWSER") != "1" {
		t.Skip("set RUN_VIDEO_SHARE_BROWSER=1 after building web")
	}
	a := importTestApp(t)
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	a.Config.WebDir = filepath.Join(root, "web", "dist")
	t.Setenv("DOUYIN_VIDEO_ENABLED", "true")
	tc := testClient{t, a.router()}
	owner, admin := resourceTestSession(t, a, "merchant"), resourceTestSession(t, a, "admin")
	v := tc.fixtureCall("POST", "/merchant/contents", owner, gin.H{"kind": "vendor", "data": Payload{Name: "免费分享卡片验收企业", MainProducts: "机械产品", RegionName: "河北", Phone: "0319-1234567"}}, 201)
	tc.call("POST", "/merchant/service-acceptances", owner, gin.H{"agreements": serviceTestRefs(t, a, "terms", "privacy", "showroom-agreement")}, 200)
	tc.call("POST", "/merchant/contents/"+number(v["id"])+"/submit", owner, gin.H{"revision_id": v["revision_id"]}, 200)
	tc.call("POST", "/admin/revisions/"+number(v["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	for _, source := range []string{videoKuaishou, videoWechatChannels} {
		link := "https://v.kuaishou.com/Abc123"
		if source == videoWechatChannels {
			link = "https://weixin.qq.com/sph/Aht6MMIJPm"
		}
		p := ResourcePayload{VideoSource: source, OriginalShareURL: link, Title: strings.Repeat("企业产品与生产现场介绍", 5), TitleEN: strings.Repeat("Company products and production demonstration ", 4), Description: "分享企业产品与生产现场，点击观看按钮前往原平台。", Purpose: "company", Format: "video"}
		r := tc.call("POST", "/merchant/resources?video_api_version=4", owner, gin.H{"kind": "video", "data": p}, 201)
		tc.call("POST", "/merchant/resources/"+number(r["id"])+"/submit?video_api_version=4", owner, gin.H{"revision_id": r["revision_id"]}, 200)
		tc.call("POST", "/admin/resource-revisions/"+number(r["revision_id"])+"/review?video_api_version=4", admin, gin.H{"decision": "approve"}, 200)
	}
	var vendor Content
	a.DB.First(&vendor, v["id"])
	srv := httptest.NewServer(a.router())
	defer srv.Close()
	a.Config.PublicBaseURL = srv.URL
	fixture, _ := json.Marshal(gin.H{"base": srv.URL, "owner": owner, "account": vendor.OwnerID, "company": "/v/" + showroomSlug(vendor)})
	cmd := exec.CommandContext(t.Context(), "node", filepath.Join(root, "scripts", "verify-video-share-cards.cjs"))
	cmd.Dir = root
	cmd.Stdin = bytes.NewReader(fixture)
	output, err := cmd.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
}

func TestWechatShareCardValidation(t *testing.T) {
	for _, raw := range []string{"https://weixin.qq.com/sph/Aht6MMIJPm", "分享 https://WEIXIN.QQ.COM/sph/Aht6MMIJPm/?tracking=1。"} {
		v, err := checkExternalShare(videoWechatChannels, raw)
		if err != nil || v.CanonicalURL != "https://weixin.qq.com/sph/Aht6MMIJPm" || v.VideoID != "sph:Aht6MMIJPm" || v.Status != "EXTERNAL_ONLY" || v.PlayerURL != "" {
			t.Fatalf("invalid normalization: %+v %v", v, err)
		}
	}
	for _, raw := range []string{
		"http://weixin.qq.com/sph/Aht6MMIJPm", "https://weixin.qq.com.evil.example/sph/Aht6MMIJPm",
		"https://evil.example@weixin.qq.com/sph/Aht6MMIJPm", "https://weixin.qq.com:443/sph/Aht6MMIJPm",
		"https://weixin.qq.com/sph/Aht6MMIJPm#x", "https://weixin.qq.com/sph/%41ht6MMIJPm",
		"https://weixin.qq.com/sph/abc/def", "https://weixin.qq.com/sph/ab", "https://weixin.qq.com/sph/" + strings.Repeat("A", 61),
		"https://channels.weixin.qq.com/finder-preview/pages/sph", "https://weixin.qq.com/sph/Aht6MMIJPm https://weixin.qq.com/sph/OtherCode",
		"<iframe src='https://weixin.qq.com/sph/Aht6MMIJPm'>", "javascript:alert(1)",
	} {
		if _, err := checkExternalShare(videoWechatChannels, raw); err == nil {
			t.Errorf("accepted unsafe link %q", raw)
		}
	}
	v, err := checkExternalShare(videoWechatChannels, "https://weixin.qq.com/sph/"+strings.Repeat("A", 60))
	if err != nil || len(v.VideoID) != 64 {
		t.Fatal("identity must fit existing storage", v, err)
	}
}

type shareCardNoNetwork struct{ calls int }

func (s *shareCardNoNetwork) RoundTrip(*http.Request) (*http.Response, error) {
	s.calls++
	return nil, fmt.Errorf("share cards must not fetch external providers or videos")
}

func TestWechatShareCardsV4IsolationAndCompatibility(t *testing.T) {
	a := importTestApp(t)
	t.Setenv("DOUYIN_VIDEO_ENABLED", "false")
	transport := &shareCardNoNetwork{}
	a.videoHTTP = &http.Client{Transport: transport}
	tc := testClient{t, a.router()}
	owner, other, admin := resourceTestSession(t, a, "merchant"), resourceTestSession(t, a, "merchant"), resourceTestSession(t, a, "admin")
	vendor := tc.fixtureCall("POST", "/merchant/contents", owner, gin.H{"kind": "vendor", "data": Payload{Name: "免费分享卡片验收企业", MainProducts: "机械", RegionName: "河北", Phone: "13900009182"}}, 201)
	tc.call("POST", "/merchant/service-acceptances", owner, gin.H{"agreements": serviceTestRefs(t, a, "terms", "privacy", "showroom-agreement")}, 200)
	tc.call("POST", "/merchant/service-acceptances", other, gin.H{"agreements": serviceTestRefs(t, a, "terms", "privacy", "showroom-agreement")}, 200)
	tc.call("POST", "/merchant/contents/"+number(vendor["id"])+"/submit", owner, gin.H{"revision_id": vendor["revision_id"]}, 200)
	tc.call("POST", "/admin/revisions/"+number(vendor["revision_id"])+"/review", admin, gin.H{"decision": "approve"}, 200)
	const v4 = "?video_api_version=4"
	p := ResourcePayload{VideoSource: videoWechatChannels, OriginalShareURL: "https://weixin.qq.com/sph/Aht6MMIJPm", Title: "视频号展示", Purpose: "company", Format: "video"}
	for _, version := range []string{"", "?video_api_version=2", "?video_api_version=3"} {
		tc.call("POST", "/merchant/video-links/check"+version, owner, gin.H{"source": videoWechatChannels, "text": p.OriginalShareURL}, 409)
		tc.call("POST", "/merchant/resources"+version, owner, gin.H{"kind": "video", "data": p}, 409)
	}
	checked := tc.call("POST", "/merchant/video-links/check"+v4, owner, gin.H{"source": videoWechatChannels, "text": p.OriginalShareURL}, 200)
	if checked["check"].(map[string]any)["status"] != "EXTERNAL_ONLY" {
		t.Fatal(checked)
	}
	r := tc.call("POST", "/merchant/resources"+v4, owner, gin.H{"kind": "video", "data": p}, 201)
	rid := number(r["id"])
	pub, own := "/public/resources/"+rid, "/merchant/resources/"+rid
	tc.call("GET", pub+v4, "", nil, 404)
	tc.call("GET", own+v4, other, nil, 403)
	tc.call("PUT", own+"/draft"+v4, other, gin.H{"revision_id": r["revision_id"], "data": p}, 403)
	tc.call("POST", own+"/submit"+v4, owner, gin.H{"revision_id": r["revision_id"]}, 200)
	tc.call("POST", "/admin/resource-revisions/"+number(r["revision_id"])+"/review"+v4, admin, gin.H{"decision": "approve"}, 200)
	for _, version := range []string{"", "?video_api_version=2", "?video_api_version=3"} {
		tc.call("GET", pub+version, "", nil, 404)
		tc.call("GET", own+version, owner, nil, 409)
		tc.call("PUT", own+"/draft"+version, owner, gin.H{"revision_id": r["revision_id"], "data": p}, 409)
		// Older editors omit new source fields or send their default LOCAL source.
		// Neither may erase the draft/public WeChat association.
		for _, data := range []gin.H{{"title": "旧客户端修改"}, {"title": "旧客户端修改", "video_source": videoLocal}} {
			blocked := tc.call("PUT", own+"/draft"+version, owner, gin.H{"revision_id": r["revision_id"], "data": data}, 409)
			if blocked["error"].(map[string]any)["code"] != "video_client_upgrade_required" {
				t.Fatal("old editor was not protected", blocked)
			}
		}
		tc.call("POST", "/admin/resources/"+rid+"/offline"+version, admin, gin.H{}, 409)
	}
	play := tc.call("GET", pub+"/playback"+v4, "", nil, 200)
	if play["status"] != "EXTERNAL_ONLY" || play["canonical_url"] != p.OriginalShareURL || play["player_url"] != nil {
		t.Fatal(play)
	}
	for _, version := range []string{"2", "3", "4"} {
		list := tc.call("GET", "/public/resources?kind=video&vendor_id="+number(vendor["id"])+"&video_api_version="+version, "", nil, 200)
		if (list["total"] == float64(1)) != (version == "4") {
			t.Fatal("client list mismatch", version, list)
		}
		showroom := tc.call("GET", "/public/showrooms/v-"+number(vendor["id"])+"?video_api_version="+version, "", nil, 200)
		if (showroom["resource_counts"].(map[string]any)["video"] == float64(1)) != (version == "4") {
			t.Fatal("client count mismatch", version, showroom)
		}
	}
	p.OriginalShareURL += "?tracking=abc"
	tc.call("POST", "/merchant/resources"+v4, owner, gin.H{"kind": "video", "data": p}, 409)
	p.Title, p.Description = "待审核新标题", "新简介"
	tc.call("PUT", own+"/draft"+v4, owner, gin.H{"revision_id": r["revision_id"], "data": p}, 200)
	if data := tc.call("GET", pub+v4, "", nil, 200)["data"].(map[string]any); data["title"] != "视频号展示" || data["description"] != "" || data["file_media_id"] != float64(0) {
		t.Fatal("draft changed public snapshot or retained original video", data)
	}
	if transport.calls != 0 {
		t.Fatal("share cards performed external network calls", transport.calls)
	}
}
