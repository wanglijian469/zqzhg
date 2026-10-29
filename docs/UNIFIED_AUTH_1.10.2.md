# 中企展馆统一注册与登录升级 1.10.2

交付日期：2026-10-29（北京时间）。网站已平滑升级到 https://zqzhg.com ，本机开发服务 http://localhost:8080/ 已启动最新代码。

## 已实现

- 沿用原版蓝白左右分栏，手机上下排列。注册默认手机验证，也可切换账号密码注册；页签切换保留输入，密码仅留在内存。旧 /register、/login 兼容跳转，保留展厅参数。
- 手机注册先验证手机号，再在同页设置、确认密码及接受协议，第二步才开户；账号密码注册填写用户名及密码。两种方式和旧公开厂商注册接口均创建普通用户，成功后自动登录展馆首页。
- 登录统一使用手机号或用户名＋密码，保留历史长密码、角色及企业归属。已缓存失效会话不会阻止打开登录表单，退出可清除失效本地会话。
- 新密码为8–20个可见英文字符，允许字母、数字及常用英文符号，不强制组合，不接受空格、中文或其他字符。服务端与Web、Flutter从同一JSON规则生成；密码显示/隐藏、确认校验、通俗错误提示已同步，bcrypt存储保持不变。历史密码登录不按新规则拦截。
- 短信直接登录及旧别名返回410 sms_login_disabled，旧环境配置不能恢复；验证码用于注册和手机号找回。找回后撤销该账号所有旧会话及找回凭证，返回密码登录并预填手机号，不自动签发新会话。
- 首次欢迎引导支持厂家申请、产品搜索、逛逛及跳过；服务器按账号保存完成状态。历史用户默认不弹，App展示前刷新服务器状态，避免另一端关闭后重复显示。
- 企业申请必须主动确认；账号密码注册用户也可直接申请，不强制短信。基本资料、展示资料、核验与提交分步填写，复用本地草稿与云端草稿；未完成必填项可保存本机，基本资料完整后可保存云端继续。人工核验及审核通过前不公开。
- 普通浏览、收藏、找产品与企业管理不互斥；没有恢复采购中心、需求、询报价或洽谈接口和任务。

## 修改清单与接口

| 子系统 | 主要文件 |
| --- | --- |
| 公共密码规则 | server/password_policy.json、auth_unified.go；scripts/sync-auth-policy.mjs；生成的Web与Flutter规则文件 |
| 认证与迁移 | server/migrations/025_unified_auth.sql、merchant_register.go、merchant_login.go、phone_accounts.go、phone_sms_login.go、sms_service.go、sms_provider.go、sms_preflight.go、permissions.go、password_reset.go、main.go |
| 网站与企业步骤 | web/src/merchant-auth.tsx、merchant-entry.tsx/css、auth-routes.tsx、phone-account.tsx、main.tsx、company-editor.tsx、runtime-config.ts、password-reset.tsx |
| App | mobile/lib/unified_auth.dart、native_auth_entry.dart、app_navigation.dart、main.dart、service_center.dart、password_policy.dart；mobile/pubspec.yaml；scripts/build-apk.ps1 |
| 验收及运维 | scripts/verify-unified-auth.cjs、verify-auth-cloud-api.py、verify-auth-map-cloud.cjs、package-auth-release.py；deploy/linux/auth-release-operator.py |

所有接口在 /api/v1 下，原响应中的token/account结构保留：

| 接口 | 输入及结果 |
| --- | --- |
| POST /auth/password | username、password；自动识别普通或厂商账号，返回会话，管理员登录仍走独立入口 |
| POST /auth/phone/code | 复用现有发送接口；purpose为register或reset_password，继续要求生产图形验证与幂等键 |
| POST /auth/phone/verify | phone、purpose、code；成功仅返回action_token、expires_in=300，不创建账号、不登录 |
| POST /auth/phone/register | phone、action_token、password、agreements、source；兼容旧code输入，固定普通账号并自动登录 |
| POST /auth/visitor/register | login_name、password、agreements、source；旧厂商注册入口同样固定普通账号 |
| POST /auth/phone/password/reset | phone、action_token、password；重置密码、撤销该账号旧会话，不返回会话 |
| POST /auth/me/welcome | 登录后提交choice=manufacturer/products/browse/skip；幂等确认，不直接改变角色 |
| GET /auth/me、GET /public/config | 增补welcome_pending、统一认证/密码找回能力、公共密码规则；phone_sms_auth固定false |

25版只增量添加phone_action_proofs和account_welcomes。凭证只存摘要，绑定手机号及用途，五分钟有效，事务内一次性消费；错误次数、过期、用途隔离和并发重放均检查。欢迎状态只对本轮之后新注册账号创建，不覆盖历史用户资料。

## 测试结果

- Go隔离MySQL相关回归通过（64.411秒）：统一注册/登录、历史密码、欢迎状态、无手机号企业申请、短信并发幂等及持久化限流、拒绝/超时、图形验证、CIAME历史快照、200MB迁移和地图接口。最终核心与企业核验发布回归21.461秒通过。测试使用RUN_MYSQL_TESTS=1，未冒称全仓所有历史测试已全部运行。
- Web认证及密码重置12项测试通过，TypeScript检查、Vite生产构建成功。
- Flutter认证、欢迎跨端状态刷新及端点兼容8项测试通过；flutter analyze无问题。原签名Release APK构建成功。
- 隔离Mock浏览器端完整通过：手机两步注册→自动登录首页→欢迎选择产品→刷新不重复→退出后密码登录→验证码找回→新密码登录→企业申请及分步草稿。见 docs/test-output/auth-1.10.2-mock.json。
- 正式HTTPS页面检查通过，桌面1440×900、平板768×1024、手机390×844无横向溢出，原版左右布局、注册页签、登录与找回入口正常。见 auth-1.10.2-cloud.json。
- 正式接口检查通过：8–20规则、无效密码拦截、未知账号拒绝、旧短信登录410、采购503、开发地图接口404。见 auth-1.10.2-api.json。
- 最终候选产物的真实地图通过：全国→河北→邢台→宁晋，开发与部署几何哈希相等，统计与列表分别11/10/8/8；缩放筛选、历史导航、展台返回、360×640及横屏等五种布局、中英文、资源失败重试均正常，失败不回退方块图。见 auth-1.10.2-map.json，截图在 docs/screenshots/auth-1.10.2/。
- Mock服务及临时数据库/限定回环测试用户已清理，本机8080服务使用最新代码，短信模式为development。

上述为相关专项检查，未做连接实体Android设备的安装/操作验收；iOS仅同步公共代码，未构建签名或真机验证。

## 短信及云端部署记录

继续使用官方Go SDK、ECS RAM角色及STS、现有MySQL；没有新增Redis或长期AccessKey。注册SMS_512636763仍用于注册，找回使用SMS_338956940、code变量，配置如下：

```dotenv
SMS_ENABLED=true
SMS_PROVIDER=aliyun
ALIYUN_SMS_SIGN_NAME=河北企元
ALIYUN_SMS_TEMPLATE_CODE=SMS_512636763
ALIYUN_SMS_RESET_TEMPLATE_CODE=SMS_338956940
ALIYUN_SMS_RESET_CODE_VARIABLE=code
ALIYUN_SMS_ENDPOINT=dysmsapi.aliyuncs.com
ALIBABA_CLOUD_IMDSV1_DISABLED=true
```

SMS_HASH_KEY沿用服务器现有私有值，不轮换、不写入交付物。原LOGIN_TEMPLATE配置已移除，绑定及历史初设密码用途未配置则继续关闭。

候选版本在ECS取得default/ecs_ram_role凭据并确认STS存在；GetSmsTemplate只读查询两个模板均为验证码类型0、审核通过1、关联河北企元、正文完全匹配。找回模板名“验证码短信”，返回备注和应用场景为空，没有返回仅限登录的说明。真实找回发送和手机收信仍须用户自行验证，不能以模板查询代替运营商送达证明。

**本轮代理真实短信发送为0。** 切换前后短信发送表仍保留此前两条受理记录，没有主动追加短信或创建线上测试账号。

- 当前目录：/opt/zqzt/releases/auth-1.10.2-20261029-r2，current已切换。
- 程序SHA256：56d926664e18a6928aafaba5952ccb458fbe22cefa31bfd725d4bee2ed558fa7。
- 内部发布快照SHA256：d31922ecc30d4c587e8aab0554b6e51a5e0cd3ec26e6fc28d3358d08e48e28e6，329个文件按清单核验；未对外生成Linux交付包。
- 主服务zqzt-exhibition在8089；候选先经8091、最终8092验证，Nginx保持HTTPS服务后切换主服务，两个候选服务现已停止。
- 数据库24→25，现有账号14、内容57、媒体180保持不变；公开厂商11家、产品46个。没有覆盖企业、审核快照、媒体或历史展厅网址。
- gosyncr.com及服务器另一现有应用仍返回200；原zqzhg证书续期任务保持active。
- 最新检查约792MiB可用内存，未新增常驻Redis或其他重型组件。

## APK与下载

[大陆农机1.10.2+40验收APK](https://zqzhg.com/acceptance/dalunongji-1.10.2-40.apk)

本机文件：delivery/android-v1.10.2/大陆农机-1.10.2+40.apk。大小85,460,938字节，名称大陆农机，包名com.zqzt.dalunongji，versionName1.10.2、versionCode40；默认接口https://zqzhg.com，保留端点切换。

- APK SHA256：16ae75aa0a742093434bf5ea3ff1f5763ab94ca92c88719cace2200a01d4e29f。
- 原签名证书SHA256：85656c85541b188c832dc48d67cbf320046a4311aa9e0fefcc662dbe2e0f1585。
- 网站原正式downloads两文件共84,717,602字节，清单哈希仍为d7527830eb39409a629cab960c36aa61e8a46bdabc6dfa59f746196d5e03899a；没有替换正式下载。

## 备份和回退

迁移前完整备份保留ECS：/opt/zqzt/shared/upgrade-backups/sms-20261029T020037Z，schema24，SQL605,669字节，SHA256 7f7f53065d1f2af26bd828d995354fc51c8b9da5de4d50c9949f1ac2570606b9，含原程序、配置及发布路径。

最终切换前另有schema25备份：/opt/zqzt/shared/upgrade-backups/sms-20261029T022245Z，SQL607,660字节，SHA256 c95af918dfe0c3f1b6b7556441d35293bf8c56bd985bfa04caca5eebc0112106。完整云端SQL及私有配置没有下载到开发机。

本机升级前备份在.local/sms/local-backup-auth102-20261029-101025，SQL1,183,038字节；开发资料保留。

回退优先保留25版数据库和本轮认证安全规则，必要时仅关闭短信发送：在/etc/zqzt/exhibition.env设置SMS_ENABLED=false、SMS_PROVIDER=disabled，再重启主服务；密码登录及已有会话不依赖短信发送。不得直接恢复旧程序重新开放短信登录、采购或公开注册厂商权限。

如果需要程序修复，使用保留25版接口、普通注册及短信登录关闭规则的修复版本，在候选端口验证后再切换。不得将旧SQL覆盖新注册账号、密码或欢迎状态；灾难恢复须先备份当前数据并核对active.json。运维脚本位于deploy/linux/auth-release-operator.py（依赖同目录sms-release-operator.py）及服务器/tmp对应脚本；全程不自动发送短信。

地图继续按用户指定的开发演示用途，来源信息、130571缺失轮廓提示保留，不伪造授权或审图记录。企业核验、资料保留/退出、地图公开使用资源及App/公安备案等运营确认事项沿用既有清单。
