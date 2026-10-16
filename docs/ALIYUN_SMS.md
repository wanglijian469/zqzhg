# 阿里云短信验证码登录

后端已接入中国站 `SendSms`，网站和 Flutter App 共用 `/api/v1/auth/sms` 与 `/api/v1/auth/phone`。真实短信需要填写本机 `server/.env` 中四项配置；密钥留在后端，不进入网站、APK或源码包。

```dotenv
ALIYUN_ACCESS_KEY_ID=填写访问密钥ID
ALIYUN_ACCESS_KEY_SECRET=填写访问密钥Secret
ALIYUN_SMS_SIGN_NAME=填写已审核通过的签名名称
ALIYUN_SMS_TEMPLATE_CODE=填写已审核通过的SMS模板CODE
```

验证码模板使用 `${code}` 变量。例如：`您的登录验证码为${code}，5分钟内有效，请勿泄露。` 签名和模板需使用阿里云控制台审核通过的实际内容，配置以 [SendSms 官方文档](https://help.aliyun.com/zh/sms/developer-reference/api-dysmsapi-2017-05-25-sendsms) 为准。API 账号需具备调用短信发送接口的权限。

填好后在项目根目录执行 `./scripts/enable-aliyun-sms.ps1`。脚本缺少配置时直接退出，保持当前服务；配置完整时设置 `SMS_PROVIDER=aliyun` 并重新启动后端。检查 `/api/v1/health` 的 `sms_provider` 为 `aliyun`。随后在网站或 App 输入本人手机号，获取短信验证码并登录。脚本不会自动向任何号码发短信。

验证码有效期5分钟，同手机号发送间隔60秒，最多5次错误尝试，成功使用后立即失效。阿里云模式在开发与生产环境都不会向客户端返回验证码；供应商拒绝、响应异常或网络失败时不保存可用验证码。真实送达验收需配置和验收手机号，适配器模拟测试不能代替运营商送达结果。

正式部署同时设置 `APP_ENV=production`。未配置真实短信的生产实例拒绝启动，部署与 HTTPS 配置见 [部署说明](DEPLOYMENT.md)。
