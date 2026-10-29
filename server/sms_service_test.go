package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type smsRecordingProvider struct {
	mu      sync.Mutex
	calls   int
	codes   map[string]string
	failure error
	delay   time.Duration
}

func (s *smsRecordingProvider) Ready(p string) bool { return validSMSPurpose(p) }
func (s *smsRecordingProvider) Send(phone, code string) error {
	_, err := s.SendPurpose(context.Background(), phone, code, "register", "")
	return err
}
func (s *smsRecordingProvider) SendPurpose(_ context.Context, phone, code, purpose, id string) (SMSResult, error) {
	s.mu.Lock()
	s.calls++
	if s.codes == nil {
		s.codes = map[string]string{}
	}
	s.codes[phone] = code
	s.mu.Unlock()
	time.Sleep(s.delay)
	return SMSResult{Code: "OK", Message: "OK", RequestID: "mock-request", BizID: "mock-biz"}, s.failure
}
func smsFixtureAccount(t *testing.T, a *App, phone, role string) Account {
	t.Helper()
	now := time.Now().UTC()
	account := Account{Username: "sms-fixture:" + randomToken()[:20], Phone: &phone, PhoneVerifiedAt: &now, Role: role}
	if err := a.DB.Create(&account).Error; err != nil {
		t.Fatal(err)
	}
	return account
}
func smsRawCall(a *App, path string, data any) (int, map[string]any) {
	b, _ := json.Marshal(data)
	r := httptest.NewRequest("POST", "/api/v1"+path, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	a.router().ServeHTTP(w, r)
	out := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}
func TestSMSRegistrationAndSharedIdentity(t *testing.T) {
	a := importTestApp(t)
	clearRegistrationLimit()
	tc := testClient{t, a.router()}
	phone := "13900008101"
	sent := tc.call("POST", "/auth/phone/code", "", gin.H{"phone": phone, "purpose": "register", "request_id": randomToken()}, 200)
	code := sent["development_code"].(string)
	var row PhoneVerification
	if err := a.DB.First(&row, "phone=?", phone).Error; err != nil {
		t.Fatal(err)
	}
	if len(code) != 6 || len(row.Digest) != 64 || row.Digest == code || row.SendID == "" {
		t.Fatal("OTP was not stored as a purpose-bound digest")
	}
	request := gin.H{"phone": phone, "code": code, "password": "SMS-Register-2026!", "role": "merchant", "source": "web", "agreements": serviceTestRefs(t, a, "terms", "privacy")}
	registered := tc.call("POST", "/auth/phone/register", "", request, 201)
	if registered["account"].(map[string]any)["role"] != "visitor" {
		t.Fatal("SMS registration elevated account role")
	}
	tc.call("POST", "/auth/phone/password", "", gin.H{"phone": phone, "password": "SMS-Register-2026!"}, 200)
	tc.call("POST", "/auth/phone/code", "", gin.H{"phone": phone, "purpose": "register"}, 409)
	tc.call("POST", "/auth/phone/register", "", request, 400)
	var count int64
	a.DB.Model(&Account{}).Where("phone=?", phone).Count(&count)
	if count != 1 {
		t.Fatal("duplicate account")
	}
	// Simulate time passing without sleeping or sending an external message.
	a.DB.Model(&SMSSendAttempt{}).Where("phone_key=?", smsPhoneKey(phone)).Update("created_at", time.Now().Add(-61*time.Second))
	a.DB.Model(&PhoneVerification{}).Where("phone=?", phone).Update("sent_at", time.Now().Add(-61*time.Second))
	sent = tc.call("POST", "/auth/phone/code", "", gin.H{"phone": phone, "purpose": "login"}, 200)
	login := tc.call("POST", "/auth/phone/login", "", gin.H{"phone": phone, "code": sent["development_code"]}, 200)
	if login["account"].(map[string]any)["id"] != registered["account"].(map[string]any)["id"] {
		t.Fatal("Web/App login did not share identity")
	}
	tc.call("POST", "/auth/phone/login", "", gin.H{"phone": phone, "code": sent["development_code"]}, 401)
}
func TestSMSUnknownPhoneAndLegacyNeverCreate(t *testing.T) {
	a := importTestApp(t)
	clearRegistrationLimit()
	tc := testClient{t, a.router()}
	phone := "13900008102"
	tc.call("POST", "/auth/phone/code", "", gin.H{"phone": phone, "purpose": "login"}, 409)
	tc.call("POST", "/auth/sms", "", gin.H{"phone": phone}, 409)
	tc.call("POST", "/auth/phone", "", gin.H{"phone": phone, "code": "123456"}, 401)
	var count int64
	a.DB.Model(&Account{}).Where("phone=? OR username=?", phone, phone).Count(&count)
	if count != 0 {
		t.Fatal("legacy authentication created account")
	}
	a.DB.Model(&SMSSendAttempt{}).Count(&count)
	if count != 0 {
		t.Fatal("ineligible phone incurred a send")
	}
}
func TestSMSPersistentLimitsAndRestart(t *testing.T) {
	for _, tt := range []struct {
		name  string
		count int
		age   time.Duration
		ip    bool
	}{
		{"minute", 1, 0, false}, {"hour", 5, 10 * time.Minute, false}, {"day", 10, 2 * time.Hour, false}, {"ip_hour", 20, 10 * time.Minute, true}, {"ip_day", 50, 2 * time.Hour, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := importTestApp(t)
			clearRegistrationLimit()
			phone := "13900008103"
			now := time.Now().UTC()
			for i := 0; i < tt.count; i++ {
				pk := smsPhoneKey(phone)
				if tt.ip {
					pk = digest(randomToken())
				}
				row := SMSSendAttempt{ID: randomToken(), RequestKey: randomToken(), PhoneKey: pk, IPKey: smsIPKey("192.0.2.1"), Purpose: "register", State: "unknown", CreatedAt: now.Add(-tt.age), UpdatedAt: now}
				if err := a.DB.Create(&row).Error; err != nil {
					t.Fatal(err)
				}
			}
			// Fresh App and cleared process counters must still enforce MySQL quotas.
			restarted := &App{DB: a.DB, Config: a.Config, SMS: a.SMS}
			clearRegistrationLimit()
			(testClient{t, restarted.router()}).call("POST", "/auth/phone/code", "", gin.H{"phone": phone, "purpose": "register"}, 429)
		})
	}
}
func TestSMSConcurrentReservationAndIdempotency(t *testing.T) {
	a := importTestApp(t)
	clearRegistrationLimit()
	provider := &smsRecordingProvider{delay: 40 * time.Millisecond}
	a.SMS = provider
	a.Config.SMSProvider = "aliyun"
	phone := "13900008104"
	id := randomToken()
	results := make(chan int, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, _ := smsRawCall(a, "/auth/phone/code", gin.H{"phone": phone, "purpose": "register", "request_id": id})
			results <- status
		}()
	}
	wg.Wait()
	close(results)
	for status := range results {
		if status != 200 && status != 409 {
			t.Fatalf("unexpected concurrent status %d", status)
		}
	}
	if provider.calls != 1 {
		t.Fatalf("idempotent request called supplier %d times", provider.calls)
	}
	status, _ := smsRawCall(a, "/auth/phone/code", gin.H{"phone": phone, "purpose": "register", "request_id": id})
	if status != 200 || provider.calls != 1 {
		t.Fatal("completed replay sent again")
	}
	status, _ = smsRawCall(a, "/auth/phone/code", gin.H{"phone": "13900008105", "purpose": "register", "request_id": id})
	if status != 409 || provider.calls != 1 {
		t.Fatal("idempotency key accepted another phone")
	}
	status, _ = smsRawCall(a, "/auth/phone/code", gin.H{"phone": phone, "purpose": "register", "request_id": randomToken()})
	if status != 429 || provider.calls != 1 {
		t.Fatal("different key bypassed cooldown")
	}
}
func TestSMSTimeoutAndRejectionNeverReportSuccess(t *testing.T) {
	for _, unknown := range []bool{true, false} {
		t.Run(map[bool]string{true: "unknown", false: "rejected"}[unknown], func(t *testing.T) {
			a := importTestApp(t)
			clearRegistrationLimit()
			provider := &smsRecordingProvider{}
			if unknown {
				provider.failure = &SMSDeliveryError{Unknown: true}
			} else {
				provider.failure = errors.New("synthetic rejection")
			}
			a.SMS = provider
			a.Config.SMSProvider = "aliyun"
			phone := "13900008106"
			smsFixtureAccount(t, a, phone, "merchant")
			id := randomToken()
			tc := testClient{t, a.router()}
			out := tc.call("POST", "/auth/phone/code", "", gin.H{"phone": phone, "purpose": "reset_password", "request_id": id}, 503)
			if out["sent"] != nil || out["development_code"] != nil {
				t.Fatal("failed send returned success or plaintext")
			}
			tc.call("POST", "/auth/phone/code", "", gin.H{"phone": phone, "purpose": "reset_password", "request_id": id}, 503)
			if provider.calls != 1 {
				t.Fatal("uncertain send retried")
			}
			want := 400
			if unknown {
				want = 200
			}
			out = tc.call("POST", "/auth/phone/verify", "", gin.H{"phone": phone, "purpose": "reset_password", "code": provider.codes[phone]}, want)
			if unknown && len(out["action_token"].(string)) != 64 {
				t.Fatal("existing merchant role changed")
			}
		})
	}
}
func TestSMSAttemptsExpiryAndAtomicConsumption(t *testing.T) {
	a := importTestApp(t)
	clearRegistrationLimit()
	tc := testClient{t, a.router()}
	phone := "13900008107"
	smsFixtureAccount(t, a, phone, "visitor")
	code := phoneFixture(t, a, phone, "login", 0)
	for i := 0; i < 5; i++ {
		tc.call("POST", "/auth/phone/login", "", gin.H{"phone": phone, "code": "000000"}, 401)
	}
	tc.call("POST", "/auth/phone/login", "", gin.H{"phone": phone, "code": code}, 401)
	code = phoneFixture(t, a, phone, "login", 0)
	a.DB.Model(&PhoneVerification{}).Where("phone=?", phone).Update("expires_at", time.Now().Add(-time.Second))
	tc.call("POST", "/auth/phone/login", "", gin.H{"phone": phone, "code": code}, 401)
	code = phoneFixture(t, a, phone, "login", 0)
	clearRegistrationLimit()
	results := make(chan int, 6)
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, _ := smsRawCall(a, "/auth/phone/login", gin.H{"phone": phone, "code": code})
			results <- status
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for status := range results {
		if status == 200 {
			success++
		} else if status != 401 {
			t.Fatalf("unexpected verification status %d", status)
		}
	}
	if success != 1 {
		t.Fatalf("code consumed %d times", success)
	}
}
func TestSMSChallengeBindingAndSingleUse(t *testing.T) {
	a := importTestApp(t)
	clearRegistrationLimit()
	t.Setenv("SMS_REQUIRE_CAPTCHA", "true")
	phone := "13900008108"
	id := randomToken()
	now := time.Now().UTC()
	challenge := SMSChallenge{ID: id, PhoneKey: smsPhoneKey(phone), IPKey: smsIPKey("192.0.2.1"), Purpose: "register", Digest: smsCodeDigest(id, phone, "register", "12345"), CreatedAt: now, ExpiresAt: now.Add(time.Minute)}
	if err := a.DB.Create(&challenge).Error; err != nil {
		t.Fatal(err)
	}
	tc := testClient{t, a.router()}
	tc.call("POST", "/auth/phone/code", "", gin.H{"phone": phone, "purpose": "register", "request_id": randomToken(), "challenge_id": id, "challenge_answer": "00000"}, 400)
	tc.call("POST", "/auth/phone/code", "", gin.H{"phone": phone, "purpose": "register", "request_id": randomToken(), "challenge_id": id, "challenge_answer": "12345"}, 400)
	got := tc.call("POST", "/auth/phone/challenge", "", gin.H{"phone": phone, "purpose": "register"}, 200)
	if !strings.HasPrefix(got["image"].(string), "data:image/png;base64,") || got["answer"] != nil {
		t.Fatal("invalid challenge response")
	}
	challenge.ID = randomToken()
	challenge.Used = false
	challenge.Digest = smsCodeDigest(challenge.ID, phone, "register", "12345")
	a.DB.Create(&challenge)
	tc.call("POST", "/auth/phone/code", "", gin.H{"phone": phone, "purpose": "register", "request_id": randomToken(), "challenge_id": challenge.ID, "challenge_answer": "12345"}, 200)
}
