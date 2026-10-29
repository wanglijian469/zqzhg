package main

import (
	"context"
	"encoding/json"
	"errors"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/utils"
	dysms "github.com/alibabacloud-go/dysmsapi-20170525/v5/client"
	"github.com/alibabacloud-go/tea/dara"
	"github.com/aliyun/credentials-go/credentials"
	"io"
	"net/http"
	"strings"
	"testing"
)

type smsHTTPMock func(*http.Request) (*http.Response, error)

func (f smsHTTPMock) Call(r *http.Request, _ *http.Transport) (*http.Response, error) { return f(r) }
func TestAliyunOfficialSDKRequestAndErrors(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		network    bool
		wantError  bool
	}{
		{"accepted", `{"Code":"OK","RequestId":"test-request","BizId":"test-biz","Message":"OK"}`, false, false},
		{"denied", `{"Code":"isv.BUSINESS_LIMIT_CONTROL","Message":"blocked"}`, false, true},
		{"broken_json", `not-json`, false, true}, {"timeout", "", true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			// Synthetic credentials only, intercepted by the in-memory SDK transport.
			cred, err := credentials.NewCredential(&credentials.Config{Type: dara.String("sts"), AccessKeyId: dara.String("test-only-id"), AccessKeySecret: dara.String("test-only-secret"), SecurityToken: dara.String("test-only-session")})
			if err != nil {
				t.Fatal(err)
			}
			client, err := dysms.NewClient(&openapi.Config{Credential: cred, Endpoint: dara.String("dysmsapi.aliyuncs.com"), HttpClient: smsHTTPMock(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.Host != "dysmsapi.aliyuncs.com" || r.URL.Scheme != "https" || r.Method != "POST" {
					t.Fatal("unexpected SDK target")
				}
				r.Form = r.URL.Query()
				if r.Form.Get("PhoneNumbers") != "13900000008" || r.Form.Get("SignName") != "河北企元" || r.Form.Get("TemplateCode") != "SMS_512636763" {
					t.Fatal("incorrect SDK fields")
				}
				var param map[string]string
				if json.Unmarshal([]byte(r.Form.Get("TemplateParam")), &param) != nil || len(param) != 1 || param["code"] != "012345" {
					t.Fatal("incorrect approved template variables")
				}
				tokenPresent := r.Form.Get("SecurityToken") != ""
				for k, v := range r.Header {
					if strings.EqualFold(k, "x-acs-security-token") && len(v) > 0 && v[0] != "" {
						tokenPresent = true
					}
				}
				if !tokenPresent {
					t.Fatal("STS token missing")
				}
				if tt.network {
					return nil, errors.New("synthetic timeout")
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tt.body))}, nil
			})})
			if err != nil {
				t.Fatal(err)
			}
			s := AliyunSMS{Sign: "河北企元", Templates: map[string]smsTemplate{"register": {"SMS_512636763", "code"}}, Client: client}
			result, err := s.SendPurpose(context.Background(), "13900000008", "012345", "register", "request-only-test")
			if (err != nil) != tt.wantError {
				t.Fatal("unexpected send result")
			}
			if calls != 1 {
				t.Fatalf("SDK made %d calls; uncertain sends must never auto-retry", calls)
			}
			if !tt.wantError && (result.RequestID != "test-request" || result.BizID != "test-biz") {
				t.Fatal("provider receipt not retained")
			}
			if s.Ready("login") {
				t.Fatal("registration template enabled SMS login")
			}
		})
	}
}
func TestAliyunConfigurationNeedsNoStaticAccessKey(t *testing.T) {
	for _, key := range []string{"ALIYUN_ACCESS_KEY_ID", "ALIYUN_ACCESS_KEY_SECRET", "ALIBABA_CLOUD_ACCESS_KEY_ID", "ALIBABA_CLOUD_ACCESS_KEY_SECRET"} {
		t.Setenv(key, "")
	}
	t.Setenv("ALIYUN_SMS_SIGN_NAME", "河北企元")
	t.Setenv("ALIYUN_SMS_TEMPLATE_CODE", "SMS_512636763")
	t.Setenv("ALIYUN_SMS_ENDPOINT", "dysmsapi.aliyuncs.com")
	t.Setenv("ALIYUN_SMS_LOGIN_TEMPLATE_CODE", "")
	s, err := NewAliyunSMS()
	if err != nil {
		t.Fatal(err)
	}
	if s.Credential == nil || !s.Ready("register") || s.Ready("login") {
		t.Fatal("invalid purpose readiness")
	}
	t.Setenv("ALIYUN_SMS_ENDPOINT", "http://untrusted.invalid")
	if _, err = NewAliyunSMS(); err == nil {
		t.Fatal("untrusted endpoint accepted")
	}
}
func TestSMSDigestAndDiagnosticPrivacy(t *testing.T) {
	t.Setenv("SMS_HASH_KEY", strings.Repeat("a", 32))
	a := smsCodeDigest("salt", "13900000008", "register", "012345")
	if a == smsCodeDigest("salt", "13900000008", "login", "012345") {
		t.Fatal("purpose not bound")
	}
	t.Setenv("SMS_HASH_KEY", strings.Repeat("b", 32))
	if a == smsCodeDigest("salt", "13900000008", "register", "012345") {
		t.Fatal("server key ignored")
	}
	if strings.Contains(safeSMSMessage("phone=13900000008 code=012345"), "012345") || strings.Contains(safeSMSMessage("phone=13900000008"), "13900000008") {
		t.Fatal("diagnostic leaked sensitive digits")
	}
}
