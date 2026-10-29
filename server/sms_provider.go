package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/utils"
	dysms "github.com/alibabacloud-go/dysmsapi-20170525/v5/client"
	"github.com/alibabacloud-go/tea/dara"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/aliyun/credentials-go/credentials"
)

type SMSResult struct{ Code, Message, RequestID, BizID string }
type SMSDeliveryError struct {
	Unknown bool
	Result  SMSResult
}

func (e *SMSDeliveryError) Error() string {
	if e.Unknown {
		return "sms_send_unknown"
	}
	return "sms_unavailable"
}

type purposeSMSProvider interface {
	Ready(string) bool
	SendPurpose(context.Context, string, string, string, string) (SMSResult, error)
}
type smsSDKClient interface {
	SendSmsWithOptions(*dysms.SendSmsRequest, *dara.RuntimeOptions) (*dysms.SendSmsResponse, error)
}
type smsTemplate struct{ Code, Variable string }
type AliyunSMS struct {
	Sign       string
	Templates  map[string]smsTemplate
	Client     smsSDKClient
	Credential credentials.Credential
}

// SMS credentials are never constructed from a stored AccessKey. The official
// chain obtains and refreshes ECS role STS credentials on the production host.
func NewAliyunSMS() (AliyunSMS, error) {
	debug := strings.ToLower(os.Getenv("DEBUG"))
	if strings.Contains(debug, "sdk") || strings.Contains(debug, "dara") || strings.Contains(debug, "tea") || strings.Contains(debug, "*") {
		return AliyunSMS{}, fmt.Errorf("sms_debug_logging_forbidden")
	}

	s := AliyunSMS{Sign: strings.TrimSpace(os.Getenv("ALIYUN_SMS_SIGN_NAME")), Templates: map[string]smsTemplate{
		"register":       {strings.TrimSpace(os.Getenv("ALIYUN_SMS_TEMPLATE_CODE")), "code"},
		"reset_password": {strings.TrimSpace(os.Getenv("ALIYUN_SMS_RESET_TEMPLATE_CODE")), strings.TrimSpace(os.Getenv("ALIYUN_SMS_RESET_CODE_VARIABLE"))},
		"bind":           {strings.TrimSpace(os.Getenv("ALIYUN_SMS_BIND_TEMPLATE_CODE")), strings.TrimSpace(os.Getenv("ALIYUN_SMS_BIND_CODE_VARIABLE"))},
		"setup_password": {strings.TrimSpace(os.Getenv("ALIYUN_SMS_SETUP_TEMPLATE_CODE")), strings.TrimSpace(os.Getenv("ALIYUN_SMS_SETUP_CODE_VARIABLE"))},
	}}
	if s.Sign == "" || !s.Ready("register") {
		return s, fmt.Errorf("sms_configuration_incomplete")
	}
	endpoint := env("ALIYUN_SMS_ENDPOINT", "dysmsapi.aliyuncs.com")
	if endpoint != "dysmsapi.aliyuncs.com" {
		return s, fmt.Errorf("sms_endpoint_invalid")
	}
	cred, err := credentials.NewCredential(nil)
	if err != nil {
		return s, fmt.Errorf("sms_credentials_unavailable")
	}
	s.Credential = cred
	s.Client, err = dysms.NewClient(&openapi.Config{Credential: cred, Endpoint: dara.String(endpoint), Protocol: dara.String("HTTPS"), RegionId: dara.String("cn-hangzhou")})
	if err != nil {
		return s, fmt.Errorf("sms_client_unavailable")
	}
	return s, nil
}

func (s AliyunSMS) Ready(purpose string) bool {
	t, ok := s.Templates[purpose]
	return ok && s.Sign != "" && t.Code != "" && regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`).MatchString(t.Variable)
}
func (s AliyunSMS) CheckECSCredential() error {
	if s.Credential == nil {
		return fmt.Errorf("sms_credentials_unavailable")
	}
	c, err := s.Credential.GetCredential()
	if err != nil || c == nil {
		return fmt.Errorf("sms_credentials_unavailable")
	}
	if dara.StringValue(c.ProviderName) != "default/ecs_ram_role" || dara.StringValue(c.SecurityToken) == "" {
		return fmt.Errorf("sms_ecs_role_required")
	}
	return nil
}
func (s AliyunSMS) Send(phone, code string) error {
	_, err := s.SendPurpose(context.Background(), phone, code, "register", randomToken())
	return err
}
func (s AliyunSMS) SendPurpose(ctx context.Context, phone, code, purpose, id string) (SMSResult, error) {
	if !s.Ready(purpose) || s.Client == nil {
		return SMSResult{}, fmt.Errorf("sms_template_unavailable")
	}
	if ctx.Err() != nil {
		return SMSResult{}, fmt.Errorf("sms_unavailable")
	}
	t := s.Templates[purpose]
	param, _ := json.Marshal(map[string]string{t.Variable: code})
	// SendSms has no provider-side idempotency. Never retry an uncertain result.
	response, err := s.Client.SendSmsWithOptions(&dysms.SendSmsRequest{PhoneNumbers: dara.String(phone), SignName: dara.String(s.Sign), TemplateCode: dara.String(t.Code), TemplateParam: dara.String(string(param)), OutId: dara.String(id)}, &dara.RuntimeOptions{Autoretry: dara.Bool(false), MaxAttempts: dara.Int(1), ConnectTimeout: dara.Int(3000), ReadTimeout: dara.Int(8000)})
	if err != nil {
		result := smsSDKResult(err)
		definitive := strings.HasPrefix(result.Code, "isv.") || strings.HasPrefix(result.Code, "Forbidden") || strings.HasPrefix(result.Code, "Invalid") || strings.HasPrefix(result.Code, "Missing") || strings.HasPrefix(result.Code, "Signature") || strings.HasPrefix(result.Code, "Throttling")
		return result, &SMSDeliveryError{Unknown: !definitive, Result: result}
	}
	if response == nil || response.Body == nil {
		return SMSResult{}, &SMSDeliveryError{Unknown: true}
	}
	b := response.Body
	result := SMSResult{Code: safeSMSMessage(dara.StringValue(b.Code)), Message: safeSMSMessage(dara.StringValue(b.Message)), RequestID: dara.StringValue(b.RequestId), BizID: dara.StringValue(b.BizId)}
	if result.Code != "OK" {
		return result, &SMSDeliveryError{Result: result}
	}
	return result, nil
}

func smsSDKResult(err error) SMSResult {
	result := SMSResult{Message: "provider_error"}
	var code, data *string
	var old *tea.SDKError
	var modern *dara.SDKError
	if errors.As(err, &old) {
		code, data = old.Code, old.Data
	} else if errors.As(err, &modern) {
		code, data = modern.Code, modern.Data
	}
	result.Code = safeSMSMessage(dara.StringValue(code))
	if data != nil {
		var receipt struct {
			RequestId string
			BizId     string
		}
		if json.Unmarshal([]byte(*data), &receipt) == nil {
			result.RequestID = receipt.RequestId
			result.BizID = receipt.BizId
		}
	}
	return result
}

var smsSensitiveDigits = regexp.MustCompile(`[0-9]{4,}`)

func safeSMSMessage(s string) string {
	s = smsSensitiveDigits.ReplaceAllString(s, "[redacted]")
	s = strings.Map(func(r rune) rune {
		if r < 32 {
			return ' '
		}
		return r
	}, s)
	if len(s) > 200 {
		return s[:200]
	}
	return s
}

func (a *App) smsPurposeReady(purpose string) bool {
	if purpose == "login" {
		return false
	}
	if a.Config.SMSProvider == "disabled" || a.SMS == nil {
		return false
	}
	if a.Config.Env == "production" && len(os.Getenv("SMS_HASH_KEY")) < 32 {
		return false
	}
	if provider, ok := a.SMS.(purposeSMSProvider); ok {
		return provider.Ready(purpose)
	}
	return a.Config.Env != "production"
}
