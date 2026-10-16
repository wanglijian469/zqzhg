package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type smsTransport func(*http.Request) (*http.Response, error)

func (send smsTransport) RoundTrip(r *http.Request) (*http.Response, error) { return send(r) }

func TestAliyunRPCPercentEncoding(t *testing.T) {
	if got := rpcEscape("a b*~+中文"); got != "a%20b%2A~%2B%E4%B8%AD%E6%96%87" {
		t.Fatalf("RPC signature encoding does not match RFC3986: %s", got)
	}
}

func TestAliyunSMSRequestAndErrors(t *testing.T) {
	for _, test := range []struct {
		name           string
		status         int
		body           string
		networkFailure bool
		wantError      bool
	}{
		{"accepted", 200, `{"Code":"OK","RequestId":"test-request","BizId":"test-business"}`, false, false},
		{"provider_limit", 200, `{"Code":"isv.BUSINESS_LIMIT_CONTROL","Message":"test-only"}`, false, true},
		{"http_failure", 503, `{"Code":"OK"}`, false, true},
		{"invalid_response", 200, `not-json`, false, true},
		{"network_failure", 0, "", true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			provider := AliyunSMS{KeyID: "test-only-id", Secret: "test-only-secret", Sign: "测试验证码签名", Template: "SMS_TEST_ONLY"}
			provider.Client = &http.Client{Transport: smsTransport(func(r *http.Request) (*http.Response, error) {
				if r.Method != "POST" || r.URL.String() != "https://dysmsapi.aliyuncs.com/" {
					t.Fatal("unexpected SMS endpoint or method")
				}
				if len(r.URL.RawQuery) != 0 {
					t.Fatal("SMS parameters must be in the POST body")
				}
				if e := r.ParseForm(); e != nil {
					t.Fatal(e)
				}
				for key, want := range map[string]string{"Action": "SendSms", "Version": "2017-05-25", "PhoneNumbers": "13900000008", "SignName": provider.Sign, "TemplateCode": provider.Template, "SignatureMethod": "HMAC-SHA1", "SignatureVersion": "1.0"} {
					if r.Form.Get(key) != want {
						t.Fatalf("incorrect %s", key)
					}
				}
				var params map[string]string
				if json.Unmarshal([]byte(r.Form.Get("TemplateParam")), &params) != nil || params["code"] != "012345" {
					t.Fatal("invalid verification template parameter")
				}
				if r.Form.Get("Signature") == "" || r.Form.Get("SignatureNonce") == "" {
					t.Fatal("missing authentication fields")
				}
				if _, e := time.Parse(time.RFC3339, r.Form.Get("Timestamp")); e != nil {
					t.Fatal("invalid UTC timestamp")
				}
				if test.networkFailure {
					return nil, errors.New("test-only network failure")
				}
				return &http.Response{StatusCode: test.status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(test.body))}, nil
			})}
			e := provider.Send("13900000008", "012345")
			if (e != nil) != test.wantError {
				t.Fatalf("unexpected delivery result: %v", e)
			}
		})
	}
}

func TestAliyunSMSRequiresCompleteConfiguration(t *testing.T) {
	keys := []string{"ALIYUN_ACCESS_KEY_ID", "ALIYUN_ACCESS_KEY_SECRET", "ALIYUN_SMS_SIGN_NAME", "ALIYUN_SMS_TEMPLATE_CODE"}
	for _, missing := range keys {
		t.Run(missing, func(t *testing.T) {
			for _, key := range keys {
				t.Setenv(key, "test-only")
			}
			t.Setenv(missing, "")
			if _, e := NewAliyunSMS(); e == nil {
				t.Fatal("incomplete SMS configuration was accepted")
			}
		})
	}
}

func TestAliyunVerificationLoginFlow(t *testing.T) {
	if os.Getenv("RUN_MYSQL_TESTS") != "1" {
		t.Skip("set RUN_MYSQL_TESTS=1 to run on an isolated MySQL database")
	}
	name := fmt.Sprintf("zqzt_exhibition_test_%d", time.Now().UnixNano())
	db, e := openDB(name)
	if e != nil {
		t.Fatal(e)
	}
	pool, _ := db.DB()
	defer pool.Close()
	defer func() {
		if e := db.Exec("DROP DATABASE `" + name + "`").Error; e != nil {
			t.Error("could not clean up this test database:", e)
		}
	}()
	if e := migrate(db); e != nil {
		t.Fatal(e)
	}
	code := ""
	accept := true
	provider := AliyunSMS{KeyID: "test-only-id", Secret: "test-only-secret", Sign: "测试签名", Template: "SMS_TEST_ONLY", Client: &http.Client{Transport: smsTransport(func(r *http.Request) (*http.Response, error) {
		if e := r.ParseForm(); e != nil {
			t.Fatal(e)
		}
		var params map[string]string
		if json.Unmarshal([]byte(r.Form.Get("TemplateParam")), &params) != nil {
			t.Fatal("invalid SMS request")
		}
		code = params["code"]
		body := `{"Code":"OK"}`
		if !accept {
			body = `{"Code":"isv.BUSINESS_LIMIT_CONTROL"}`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}}
	a := &App{DB: db, Config: Config{Env: "development", SMSProvider: "aliyun", MediaDir: t.TempDir(), WebDir: t.TempDir()}, SMS: provider}
	tc := testClient{t, a.router()}
	response := tc.call("POST", "/auth/sms", "", gin.H{"phone": "13900000008"}, 200)
	if _, leaked := response["development_code"]; leaked {
		t.Fatal("Aliyun mode must never expose a verification code, including in development")
	}
	if len(code) != 6 || response["sent"] != true {
		t.Fatal("SMS was not passed to the adapter")
	}
	tc.call("POST", "/auth/phone", "", gin.H{"phone": "13900000008", "code": "wrong"}, 401)
	session := tc.call("POST", "/auth/phone", "", gin.H{"phone": "13900000008", "code": code}, 200)
	tc.call("GET", "/merchant/contents", session["token"].(string), nil, 200)
	tc.call("POST", "/auth/phone", "", gin.H{"phone": "13900000008", "code": code}, 401)
	tc.call("POST", "/auth/sms", "", gin.H{"phone": "13900000008"}, 429)
	accept = false
	tc.call("POST", "/auth/sms", "", gin.H{"phone": "13900000007"}, 503)
	tc.call("POST", "/auth/phone", "", gin.H{"phone": "13900000007", "code": code}, 401)
	t.Log("Aliyun request/login integration verified without external SMS: no code leakage, single-use login, cooldown and failed-delivery rollback")
}
