# 中企展馆短信及演示数据升级交付

实施时间：2026-10-28至2026-10-29（北京时间）。正式网站及接口已切换至 https://zqzhg.com 。本轮不替换网站原有APK正式下载，提供独立验收APK。

## 实现及修改清单

| 范围 | 主要文件 | 结果 |
| --- | --- | --- |
| 阿里云官方SDK及角色凭据 | server/sms_provider.go、sms_preflight.go、go.mod、go.sum、.env.example | 官方Go SDK V2接口生成模块，SendSms；默认凭据链获取ECS角色STS，禁止静态AK配置依赖、SDK自动重试及请求调试日志 |
| 验证码、图片验证、限流及幂等 | server/sms_service.go、migrations/024_sms_service.sql | MySQL摘要、5分钟、5次尝试、原子消费；60秒/每小时5次/每天10次；IP及图片验证；发送流水、超时不补发 |
| 账号及兼容 | server/auth.go、phone_accounts.go、phone_sms_login.go、main.go、admin.go、sms_https.go、legacy_origin.go、public_origin.go | 普通用户注册、已验证手机号登录、密码登录保留；旧接口不自动开户；原角色不变；HTTPS及旧网址跳转；后台就绪判断使用运行中RAM提供者，不要求静态AK |
| 网页 | web/src/phone-account.tsx、phone-login-api.ts、sms-challenge.tsx、merchant-phone-login.tsx、merchant-auth.tsx、main.tsx、runtime-config.ts、api.ts | /register、/login、/account；图形验证、60秒倒计时、错误反馈、主动协议确认、登录跳转、中英文 |
| App | mobile/lib/native_auth_entry.dart、main.dart、mobile_upgrade.dart、industrial_ui.dart、assets/i18n/*、pubspec.yaml、scripts/build-apk.ps1 | 同一后端和用户库；默认HTTPS，端点迁移及切换；原包名、原签名、名称大陆农机；1.10.1+39 |
| 演示地图及公开资料 | server/map_demo.go、public_demo_import.go、model.go；web/src/industry-map-{chart,preview}.tsx/ts、types.ts、ui.tsx、showroom.tsx；scripts/export-public-demo.py | 开发边界生产代理可加载；保留来源；公开演示资料增量导入，既有资料、网址不覆盖；新导入内容标识开发演示 |
| 交付及运维 | scripts/package-sms-release.py、prepare-sms-acceptance.py、verify-sms-cloud.cjs、verify-sms-cloud-api.py、enable-aliyun-sms.ps1；deploy/linux/sms-release-operator.py、sms-upgrade-audit.py、zqzhg-https.conf、configure-zqzhg-renewal.py | 备份、校验、候选服务、HTTPS切换、原下载保留、只读诊断、续期任务 |

仓库原有CIAME分类、免费展示、200MB额度及布局改动随当前版本部署。没有覆盖现有工作区改动或将采购业务恢复。

## 当前云端状态

- 当前发布目录：`/opt/zqzt/releases/sms-demo-1.10.1-20261028-r3`，`/opt/zqzt/current` 指向该目录。最终程序SHA256为 `24a6f6833353b8b65d61c33fa1fe81f08c2db9e602a4093cb779c3fbf352c7c8`。
- 主服务：`zqzt-exhibition.service`，8089；Nginx仅为zqzhg.com新增HTTPS站点。其他现有应用及gosyncr.com仍返回200。
- 候选服务：`zqzt-sms-candidate.service`，127.0.0.1:8091，切换后已停止；候选期间关闭重复后台任务。
- 数据库保留原active.json指向，schema21增量到24，没有更换、清空原数据库。部署时账号10→12（两个无登录密码及手机号的演示厂商属主），后续用户真实注册验收后为13；内容54→57，媒体178→180。
- 原有9家厂商、45个产品保留；从本机11家厂商、46个产品的公开快照中，只新增缺失的2家、1个产品、2张图片。导入不复制本机账号、口令、会话、草稿、核验或投诉附件。
- 再导入按名称保守去重，目标不公开或匹配歧义时跳过。云端再次执行结果为新增0厂商、0产品、0图片，保留11厂商、46产品、歧义0。公开图片保留已有样式，避免重复生成水印；失败回滚仅清理本次新增文件。
- 免费模式及采购关闭已验证；默认容量200,000,000字节。公安备案号及App备案号为空，网站使用真实冀ICP备2026047360号-2。
- 网站/API/媒体均为同一HTTPS来源。可信代理仅127.0.0.1、::1；Nginx覆盖客户端地址及协议转发头，外部直连接口不能靠伪造转发头跳过短信HTTPS要求。
- TLS证书有效期至2027-01-26；`zqzhg-cert-renew.timer` 每天两次检查，续期后检验并重载Nginx；Certbot模拟续期成功。

## RAM、模板与真实发送

不需要Redis或长期AccessKey。ECS已验证凭据来源 `default/ecs_ram_role`，STS存在；生产启动强制核对来源。角色策略须允许：

```json
{"Version":"1","Statement":[{"Effect":"Allow","Action":["dysms:SendSms","dysms:GetSmsTemplate"],"Resource":"*"}]}
```

注册SMS_512636763、登录SMS_338956940已分别查询成功：验证码类型0、审核通过1、关联签名河北企元、变量code、正文完全匹配用户提供内容。注册模板不用于登录。手机号绑定与短信设置密码模板未配置，对应新短信入口关闭；已有密码登录保留。

代理仅向用户再次确认的尾号8982执行过**一次真实注册短信发送**，未向另一号码发送；其最初验收记录如下。随后用户操作系统重新获取注册验证码并完成注册，后续记录见下文，代理没有再次调用发送接口。

| 项目 | 结果 |
| --- | --- |
| API返回 | HTTP200，accepted，有效期300秒 |
| 阿里云Code / Message | OK / OK |
| RequestId | 01A18344-C987-503C-AC38-ACF653CD686A |
| BizId | 782910393203685885^0 |
| 实际收信 | 2026-10-29用户确认已收到短信 |
| 最初验证码消费 | 用户当时仅收信未录入，该次验证码已过期，未用于注册 |

本地已落盘一次发送保护，网络超时也不自动再次调用。未记录验证码明文或把云凭据下发至客户端。当前无法代替收信人输入验证码及自设密码；过期后须获得新的明确发送授权，不能自动补发。真实登录短信未发送，登录完整真实流程待收信人配合补验。

2026-10-29后续真实注册验收：用户反馈“已经注册并登录”。只读核验尾号8982对应账号仅1个，角色visitor，手机号已验证、密码已设置、有效登录会话1个；验证码used=1，服务日志注册HTTP201。因此真实注册及注册后自动登录已通过。最新已消费验证码对应阿里云RequestId为 `01A184ED-B9ED-51AC-ACED-D9AC6BB7BD69`，BizId为 `562810393231534687^0`，Code=OK。当前累计两条受理成功流水，最初代理发送一条，后续用户注册操作新增一条。没有验证单独退出后的密码登录或登录短信流程。

## 验收结果

- Go使用 `RUN_MYSQL_TESTS=1` 创建隔离测试数据库：短信注册/登录/共享身份、重复号码、角色保持、旧接口不开户、冷却与持久化限流、并发幂等、拒绝和超时、五次错误/过期/并发消费、图片验证绑定及一次失效通过（相关组合96.952秒）。演示导入开启水印的回归8.911秒通过；200MB迁移、管理员覆盖、历史分类快照及旧来源兼容组合23.704秒通过。最终后台RAM状态、SDK及HTTPS防伪造组合7.342秒通过。
- Web短信及认证相关20项测试通过，TypeScript及Vite生产构建成功。
- Flutter短信/端点相关11项测试通过；`flutter analyze --no-pub` 无问题；Android内置Web资源4项检查通过。
- HTTPS生产配置浏览器验收：全国→河北→邢台→宁晋，几何及哈希与开发一致；实际公开数11→10→8→8，列表与统计一致；手机360×640、390×844、平板768×1024、桌面1440×900、横屏844×390；中英文、历史导航、资源失败重试通过。记录在 `docs/test-output/sms-1.10.1-cloud.json`，截图在 `docs/screenshots/sms-1.10.1/`。
- API只读验收记录在 `docs/test-output/sms-1.10.1-cloud-api.json`：免费模式、采购503、未注册登录拒绝、缺图形验证拦截、开发接口生产404。外部伪造HTTPS头最初得到426；后续Windows直连8089出现网络重置，改在ECS通过公网IP非回环连接再次核验426，配合Go外部地址防伪造测试通过，未把网络重置称作功能成功。
- 反复自动刷新曾触发原有每IP每分钟30次列表保护；验收脚本已按2200ms间隔控制列表读取，未放宽生产限流。代理配置补齐回环可信地址，避免所有HTTPS用户共用127.0.0.1限流键。
- 地图演示来源及用途保持真实，不提供虚构的授权或审图记录。邢台130571目录节点缺少源轮廓，保留明确的未匹配地区选择入口。

## APK

独立验收下载：https://zqzhg.com/acceptance/dalunongji-1.10.1-39.apk

本机：`delivery/android-v1.10.1/大陆农机-1.10.1+39.apk`，85,347,841字节。aapt核对名称大陆农机、包名com.zqzt.dalunongji、versionName1.10.1、versionCode39；apksigner核对原证书一致。

- APK SHA256：`ecd4bf183df344438681f9b959186ad990aac7d22457d27f0f43147e12cbd4a0`
- 原证书 SHA256：`85656c85541b188c832dc48d67cbf320046a4311aa9e0fefcc662dbe2e0f1585`
- 网站原downloads两文件共84,717,602字节，复制前后清单哈希均为 `d7527830eb39409a629cab960c36aa61e8a46bdabc6dfa59f746196d5e03899a`，没有替换正式下载。

本机无连接Android设备，未做真实安装或手机收信实测；iOS仅同步公共代码，未构建、签名或设备验证。

## 备份、重复部署与回退

最近迁移前备份在ECS：`/opt/zqzt/shared/upgrade-backups/sms-20261028T154955Z`，包含原配置、程序、原发布路径及完整SQL。SQL519,797字节，SHA256 `a7ef65edb751cf9f1bf1be1c28a51cf2afc1ae46fe6aa8a21b653c3ef44f222d`。完整备份仅保留服务器，不下载到开发机。

最终后台状态修复前另有24版数据备份 `/opt/zqzt/shared/upgrade-backups/sms-20261028T162415Z`，SQL600,614字节，SHA256 `cef6051b60300ee37691286bafbc9625236ea9f47bd5098ea3c0aa008141a8ca`，包含本轮新增演示数据及短信发送流水。

操作脚本保留在服务器 `/tmp/zqzt-sms-release-operator.py`，源码见deploy/linux；私有配置 `/etc/zqzt/exhibition.env`（600）及回环候选配置不进入源码/交付包。生产随机SMS_HASH_KEY须稳定保留，不能每次重启重生成。

再次部署步骤：先运行prepare-sms-acceptance.py备份；使用新的不冲突发布目录打包并校验SHA；在候选端口启动，保留原active.json数据库和媒体；运行sms-preflight（不发送），必要时执行公开演示增量导入；按候选HTTPS代理验证后切主服务，候选停止。`zqzhg-https.conf` 是最终8089上游，候选验证时单独临时改为8091，校验Nginx后重载。所有命令均不主动发送短信。

优先回退短信功能，保留新账号、验证码流水和24版数据库：

```bash
sudo cp -p /etc/zqzt/exhibition.env /etc/zqzt/exhibition.env.before-sms-disable
sudo sed -i 's/^SMS_ENABLED=.*/SMS_ENABLED=false/;s/^SMS_PROVIDER=.*/SMS_PROVIDER=disabled/' /etc/zqzt/exhibition.env
sudo systemctl restart zqzt-exhibition
curl -fsS https://zqzhg.com/api/v1/health
```

旧21版程序可能恢复已停用采购或旧短信自动开户逻辑，不能直接作为安全回退。需要程序回退时应制作保留这些安全约束的修复版本，继续使用增量后的数据库。数据库恢复仅用于停机灾难恢复：先备份当前数据、核对新增账号及active.json，再由运维执行，不能直接将旧SQL覆盖新注册账号。其他站点及证书配置保持独立。

## 仍需人工完成

实际送达、云端真实注册及注册后自动登录已确认通过。单独退出后的密码登录、登录短信、Android安装、跨端账号实测及iOS设备测试仍待完成；代理不自动追加短信发送。地图资源完整公开使用授权、运营主体与域名、App/公安备案、视频展示范围、人工核验依据、资料保留与退出规则继续由运营方核实，不以免费展示自动推断许可结论。
