# 阿里云短信与统一认证

当前版本为1.10.2：日常使用手机号或用户名加密码登录，短信只用于手机号注册与找回密码。旧短信直接登录接口返回410 sms_login_disabled，不自动开户。

使用ECS绑定的RAM角色及官方Credentials默认凭据链取得STS，不需要Redis或长期AccessKey。验证码及短期用途凭证保存安全摘要于MySQL，复用冷却、限流、图形验证和发送幂等机制。

```dotenv
SMS_ENABLED=true
SMS_PROVIDER=aliyun
ALIYUN_SMS_SIGN_NAME=河北企元
ALIYUN_SMS_TEMPLATE_CODE=SMS_512636763
ALIYUN_SMS_RESET_TEMPLATE_CODE=SMS_338956940
ALIYUN_SMS_RESET_CODE_VARIABLE=code
ALIYUN_SMS_ENDPOINT=dysmsapi.aliyuncs.com
ALIBABA_CLOUD_IMDSV1_DISABLED=true
SMS_HASH_KEY=沿用服务器私有随机密钥，不在部署或重启时重新生成
```

注册正文“您的注册码为：${code}，如非本人操作，请忽略本短信！”，找回正文“您的验证码为：${code}，请勿泄露于他人！”。变量code为六位数字，不更改已审核文案。绑定及历史初设密码未提供适用模板，保持关闭。

RAM角色最小权限为dysms:SendSms与dysms:GetSmsTemplate，Resource为*。`exhibition sms-preflight`只读检查角色、STS、模板正文、签名和审核状态，不访问业务库、不发送短信。Code=OK仅表示发送受理，不能代替实际收信确认。

注册采用同页两步验证，最终提交后自动登录；找回成功撤销旧会话并返回密码登录。两类凭证隔离、五分钟有效、一次消费。密码新规则8–20位，历史密码保持可登录。

完整接口、修改清单、测试、备份、回退及待验收项见 [1.10.2交付记录](UNIFIED_AUTH_1.10.2.md)。此前真实注册发送记录保留在 [1.10.1历史记录](SMS_UPGRADE_1.10.1.md)。
