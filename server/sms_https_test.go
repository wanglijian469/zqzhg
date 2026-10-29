package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSMSHTTPSRejectsExternalSpoofing(t *testing.T) {
	a := &App{Config: Config{Env: "production", SMSProvider: "disabled"}}
	for _, tt := range []struct {
		name, peer, proto string
		want              int
	}{{"plain", "198.51.100.10:4000", "", 426}, {"spoofed", "198.51.100.10:4000", "https", 426}, {"local_without_tls", "127.0.0.1:4000", "", 426}, {"local_proxy", "127.0.0.1:4000", "https", 503}} {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "http://zqzhg.com/api/v1/auth/phone/code", strings.NewReader(`{}`))
			r.RemoteAddr = tt.peer
			r.Header.Set("X-Forwarded-Proto", tt.proto)
			w := httptest.NewRecorder()
			a.router().ServeHTTP(w, r)
			if w.Code != tt.want {
				t.Fatalf("wanted %d got %d", tt.want, w.Code)
			}
		})
	}
}
