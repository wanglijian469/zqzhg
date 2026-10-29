package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSMSAdminReadinessDoesNotRequireStaticKeys(t *testing.T) {
	a := importTestApp(t)
	a.Config.SMSProvider = "aliyun"
	a.SMS = AliyunSMS{Sign: "河北企元", Templates: map[string]smsTemplate{"register": {"SMS_512636763", "code"}, "reset_password": {"SMS_338956940", "code"}}}
	for _, key := range []string{"ALIYUN_ACCESS_KEY_ID", "ALIYUN_ACCESS_KEY_SECRET", "ALIBABA_CLOUD_ACCESS_KEY_ID", "ALIBABA_CLOUD_ACCESS_KEY_SECRET"} {
		t.Setenv(key, "")
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	a.adminSystem(c)
	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result["aliyun_configured"] != true {
		t.Fatal("running provider was incorrectly reported as requiring static keys", result, err)
	}
}
