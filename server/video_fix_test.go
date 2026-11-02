package main

import (
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestVideoNotesReturnSpecificErrorWithoutFetchingPage(t *testing.T) {
	const note = "https://www.iesdouyin.com/share/note/7551737336111893818/"
	var calls atomic.Int32
	a := &App{videoHTTP: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, Transport: videoRoundTrip(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": {note}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}}
	for _, link := range []string{note, "https://www.douyin.com/note/7551737336111893818", "https://v.douyin.com/c3Tok9wUn9o"} {
		_, _, e := a.resolveVideoShare(t.Context(), link)
		if e == nil || e.Error() != "unsupported_video_content" {
			t.Fatalf("want specific note error, got %v", e)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("must not fetch note pages: %d", calls.Load())
	}
}
func TestVideoRightsCheckboxOptionalWithoutFabricatingConsent(t *testing.T) {
	a := importTestApp(t)
	t.Setenv("DOUYIN_VIDEO_ENABLED", "true")
	var calls atomic.Int32
	mockVideoClient(a, &calls)
	tc := testClient{t, a.router()}
	merchant := resourceTestSession(t, a, "merchant")
	admin := resourceTestSession(t, a, "admin")
	tc.fixtureCall("POST", "/merchant/contents", merchant, gin.H{"kind": "vendor", "data": Payload{Name: "视频测试", RegionName: "河北", Phone: "13900009181", MainProducts: "机械"}}, 201)
	denied := ResourcePayload{Title: "先确认企业协议", Format: "video", Purpose: "company", VideoSource: videoDouyin, OriginalShareURL: "https://www.douyin.com/video/7136314708138724640"}
	failure := tc.call("POST", "/merchant/resources?video_api_version=2", merchant, gin.H{"kind": "video", "data": denied}, 409)
	if failure["error"].(map[string]any)["code"] != "agreement_required" {
		t.Fatal(failure)
	}
	tc.call("POST", "/merchant/service-acceptances", merchant, gin.H{"agreements": serviceTestRefs(t, a, "terms", "privacy", "showroom-agreement")}, 200)
	p := ResourcePayload{Title: "不逐条勾选", Format: "video", Purpose: "company", VideoSource: videoDouyin, OriginalShareURL: "https://www.douyin.com/video/7690195495465012480"}
	r := tc.call("POST", "/merchant/resources?video_api_version=2", merchant, gin.H{"kind": "video", "data": p}, 201)
	var rev ResourceRevision
	a.DB.First(&rev, uint64(r["revision_id"].(float64)))
	if rev.RightsActorID != 0 || rev.RightsConfirmedAt != nil || rev.RightsVersion != "" {
		t.Fatal("fabricated consent", rev)
	}
	tc.call("POST", "/merchant/resources/"+number(r["id"])+"/submit?video_api_version=2", merchant, gin.H{"revision_id": r["revision_id"]}, 200)
	tc.call("POST", "/admin/resource-revisions/"+number(r["revision_id"])+"/review?video_api_version=2", admin, gin.H{"decision": "approve"}, 200)
}

func TestPlatformVideoAgreementsRemainExplicitAndAccountScoped(t *testing.T) {
	a := importTestApp(t)
	tc := testClient{t, a.router()}
	admin := resourceTestSession(t, a, "admin")
	merchant := resourceTestSession(t, a, "merchant")
	tc.call("GET", "/admin/service-acceptances", merchant, nil, 403)
	tc.call("POST", "/admin/service-acceptances", merchant, gin.H{"agreements": serviceTestRefs(t, a, "terms", "privacy", "showroom-agreement")}, 403)
	tc.call("GET", "/admin/service-acceptances", admin, nil, 200)
	tc.call("POST", "/admin/service-acceptances", admin, gin.H{"agreements": []AgreementRef{}}, 409)
	tc.call("POST", "/admin/service-acceptances", admin, gin.H{"agreements": serviceTestRefs(t, a, "terms", "privacy", "showroom-agreement"), "source": "web"}, 200)
	owner := serviceTestOwner(t, a, admin)
	var accepted []ServiceAcceptance
	a.DB.Where("account_id=?", owner).Find(&accepted)
	if len(accepted) != 3 {
		t.Fatal(accepted)
	}
	tc.call("POST", "/admin/service-acceptances", admin, gin.H{"agreements": []AgreementRef{}}, 200)
	var after []ServiceAcceptance
	a.DB.Where("account_id=?", owner).Find(&after)
	for i := range accepted {
		if !accepted[i].AcceptedAt.Equal(after[i].AcceptedAt) {
			t.Fatal("consent timestamp rewritten")
		}
	}
}
