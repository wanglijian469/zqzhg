package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestVideoProviderFailureContracts(t *testing.T) {
	a := importTestApp(t)
	t.Setenv("DOUYIN_VIDEO_ENABLED", "true")
	cases := []struct {
		raw, reason string
		timeout     bool
	}{
		{`{"err_no":28003004}`, "video_not_public", false},
		{`{"err_no":28001007}`, "invalid_video_link", false},
		{`{"err_no":28001005}`, "video_provider_unavailable", false},
		{`{"data":{}}`, "video_provider_unavailable", false},
		{`{"err_no":0,"data":{"iframe_code":"<iframe src='https://evil.test/player/video'></iframe>"}}`, "invalid_official_player", false},
		{strings.Repeat("x", 256*1024+1), "video_provider_unavailable", false},
		{"", "video_network_timeout", true},
	}
	for i, c := range cases {
		t.Run(c.reason+fmt.Sprint(i), func(t *testing.T) {
			var calls atomic.Int32
			a.videoHTTP = &http.Client{Transport: videoRoundTrip(func(*http.Request) (*http.Response, error) {
				calls.Add(1)
				if c.timeout {
					return nil, context.DeadlineExceeded
				}
				return videoResponse(c.raw), nil
			})}
			id := fmt.Sprintf("76901954954650124%02d", i)
			got := a.checkDouyin(t.Context(), id)
			if got.Status == "READY" || got.Reason != c.reason || got.PlayerURL != "" {
				t.Fatal(got)
			}
			a.checkDouyin(t.Context(), id)
			if calls.Load() != 1 {
				t.Fatal("temporary failure cache not used")
			}
		})
	}
}
