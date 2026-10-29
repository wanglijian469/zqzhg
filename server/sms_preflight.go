package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	dysms "github.com/alibabacloud-go/dysmsapi-20170525/v5/client"
	"github.com/alibabacloud-go/tea/dara"
)

// Read-only cloud diagnostic. It deliberately cannot send a text or touch MySQL.
func smsPreflight() error {
	s, err := NewAliyunSMS()
	if err != nil {
		return err
	}
	if err = s.CheckECSCredential(); err != nil {
		return err
	}
	out := map[string]any{"credential_source": "default/ecs_ram_role", "sts_present": true, "send_permission": "not_verified_by_read_only_check", "sent_messages": 0}
	templates := []map[string]any{}
	client := s.Client.(*dysms.Client)
	failed := false
	for _, purpose := range []string{"register", "reset_password", "bind", "setup_password"} {
		entry := map[string]any{"purpose": purpose, "configured": s.Ready(purpose)}
		if s.Ready(purpose) {
			template := s.Templates[purpose]
			response, e := client.GetSmsTemplateWithOptions(&dysms.GetSmsTemplateRequest{TemplateCode: dara.String(template.Code)}, &dara.RuntimeOptions{Autoretry: dara.Bool(false), MaxAttempts: dara.Int(1), ConnectTimeout: dara.Int(3000), ReadTimeout: dara.Int(8000)})
			if e != nil || response == nil || response.Body == nil {
				entry["query"] = "failed_or_not_authorized"
				entry["error_kind"] = fmt.Sprintf("%T", e)
				entry["error_code"] = smsSDKResult(e).Code
				failed = true
			} else {
				b := response.Body
				entry["query_code"] = dara.StringValue(b.Code)
				entry["template_code"] = template.Code
				entry["content"] = dara.StringValue(b.TemplateContent)
				entry["type"] = dara.StringValue(b.TemplateType)
				entry["approval_status"] = dara.StringValue(b.TemplateStatus)
				entry["related_sign"] = dara.StringValue(b.RelatedSignName)
				entry["template_name"] = safeSMSMessage(dara.StringValue(b.TemplateName))
				entry["apply_scene"] = safeSMSMessage(dara.StringValue(b.ApplyScene))
				entry["remark"] = safeSMSMessage(dara.StringValue(b.Remark))
				valid := dara.StringValue(b.Code) == "OK" && dara.StringValue(b.TemplateType) == "0" && dara.StringValue(b.TemplateStatus) == "1" && dara.StringValue(b.RelatedSignName) == s.Sign && strings.Contains(dara.StringValue(b.TemplateContent), "${"+template.Variable+"}")
				if purpose == "register" {
					valid = valid && dara.StringValue(b.TemplateContent) == "您的注册码为：${code}，如非本人操作，请忽略本短信！"
				}
				if purpose == "reset_password" {
					valid = valid && dara.StringValue(b.TemplateContent) == "您的验证码为：${code}，请勿泄露于他人！"
				}
				entry["matches_supplied_approved_template"] = valid
				if !valid {
					failed = true
				}
			}
		}
		templates = append(templates, entry)
	}
	out["templates"] = templates
	if err = json.NewEncoder(os.Stdout).Encode(out); err != nil {
		return err
	}
	if failed {
		return fmt.Errorf("sms_template_preflight_incomplete")
	}
	return nil
}
