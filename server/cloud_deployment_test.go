package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProductionSMSModes(t *testing.T) {
	for _, provider := range []string{"disabled", "aliyun"} {
		if err := validateSMSMode("production", provider); err != nil {
			t.Fatal(err)
		}
	}
	for _, provider := range []string{"development", "", "other"} {
		if validateSMSMode("production", provider) == nil {
			t.Fatalf("production accepted %q", provider)
		}
	}
	if err := validateSMSMode("development", "development"); err != nil {
		t.Fatal(err)
	}
}

func TestDisabledSMSDoesNotIssueOrConsumeCodes(t *testing.T) {
	// A nil DB makes accidental code creation/consumption fail this test.
	a := &App{Config: Config{Env: "production", SMSProvider: "disabled"}, SMS: DisabledSMS{}}
	for _, route := range []string{"/api/v1/auth/sms", "/api/v1/auth/phone"} {
		r := httptest.NewRequest("POST", route, strings.NewReader(`{"phone":"13900000001","code":"123456"}`))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		a.router().ServeHTTP(w, r)
		if w.Code != 503 || !strings.Contains(w.Body.String(), "sms_disabled") || strings.Contains(w.Body.String(), "development_code") {
			t.Fatalf("disabled SMS response: %d %s", w.Code, w.Body.String())
		}
	}
}
