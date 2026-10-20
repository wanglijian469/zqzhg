# 中企展馆 ZQZG V2.0

Android与手机网页升级验收包 **1.8.0-rc.1+30**：同步注册登录、企业资料、三步产品发布、工作台与推广体验，原包名／签名及稳定APK保持不变。本地 `/app` 独立下载。正式地图资源和真机验收仍待完成，未部署生产。[同步清单、测试与升级说明](docs/MOBILE18_ACCEPTANCE.md)。

GitHub安装包与对应源码：[移动端升级预发布 v1.8.0-rc.1](https://github.com/wanglijian469/zqzhg/releases/tag/v1.8.0-rc.1)。APK和完整更新ZIP放在Release附件中，源码修改提交至本仓库；本机学习地图资源及其轮廓截图不随仓库或附件分发。

厂商地图本机轮廓预览：开发环境以 `scripts/start.ps1 -MapPreview` 启用相邻拼接的省市县真实轮廓，保留真实统计及下钻；仅本机学习预览，默认和生产仍使用方块示意图。[当前实现、资源用途与验收](docs/FACTORY_MAP_OUTLINE_PREVIEW.md)。

厂商地图：`/regions` 默认使用 ECharts 可点击行政区方块示意图，显示真实厂商数量并按目录逐级下钻，支持固定视口、自适应分页及手机地图／厂商切换。页面明确标注“非真实地理边界”，不依赖边界文件。[当前使用与验证说明](docs/FACTORY_MAP_SCHEMATIC.md)。

新增产业地图（Schema16）：网站 `/regions` 提供全国／省／市／区县导航、真实公开厂商统计与筛选分页，首版为行政区划列表模式；ECharts 图形等待合规资源核验后启用。[功能与接口说明](docs/INDUSTRY_MAP.md)、[地图资源核验](docs/MAP_RESOURCE_REVIEW.md)、[本次验收](docs/INDUSTRY_MAP_ACCEPTANCE.md)。本次未部署生产，未向生产数据插入测试企业。

当前网站与后端延续 **2.0.0** 版本标识，已合入V2.2产品提交、企业工作台、展厅与存储设置升级（Schema15）。最新阿里云发行包见 [部署发行说明](releases/aliyun-20261018-storage-v1/README.md)，配套源码与产品附件发布到GitHub Release。展馆切换、首页及查询升级、迁移、验收和部署回滚见 [V2.0 升级说明](docs/V2.0_UPGRADE.md)。Android沿用已交付的1.7.0+29安装包及原签名；本次仅编译发布阿里云部署包，不直接部署生产服务器。

React 响应式展馆网站、网页 CMS、Go/Gin/GORM/MySQL 后端与 Flutter 厂商 App。网站、App 和后台使用同一组 API 和数据库，首版提供公开展示、找厂、联系、入驻、审核及中英文切换。

历史 V1.5.19 交付：网站／后端 **1.5.19**、Android **1.5.19+27**，沿用原签名。厂商端改版、安装包与验证边界见 [厂商管理端升级记录](docs/MERCHANT_UI_1.5.19.md)。本轮未部署生产服务。既有企业门户及 Android 同步功能见 [Android同步与安装说明](docs/ANDROID_SYNC.md)。企业展厅默认进入宣传门户首页，自动隐藏无资料模块，支持资质荣誉和逐张图集标注。[企业门户说明](docs/COMPANY_PORTAL.md)。厂商后台独立为 `/merchant`，支持最多3款代表展品的自动／手动选择及排序，与本厂全部产品和首页精选独立。[厂商工作台说明](docs/MERCHANT_WORKSPACE.md)。Android与网站共用手动／自动代表展品配置。“看厂家”统一使用展台封面、企业Logo与横向品牌卡；后台厂商管理提供“设置封面”快捷入口。[封面与列表说明](docs/VENDOR_CARDS.md)。后台“首页展示”可设置独立平台配图、选择和排序最多12家品牌展厅。[首页展示说明](docs/HOME_DISPLAY.md)。厂商／产品管理及审核列表支持勾选、批量通过和共同原因批量驳回。[批量审核说明](docs/BATCH_REVIEW.md)。分类新增支持“顶级（一级）分类”，管理员可保留一级或调整为下级。[分类说明](docs/CATEGORY_ENTRY.md)。新厂商可自行设置公司账号和密码；保存／提交自动接回原操作，已有公司账号管理原展位。[注册入驻说明](docs/REGISTRATION.md)。入驻／企业编辑提供醒目的“新增经营分类”，产品编辑提供“新增产品分类”；登录后续填、立即选中及失败重试沿用现有权限规则。[分类入口说明与截图](docs/CATEGORY_ENTRY.md)。后台厂商账号列表及厂商详情页增加随机／手动密码重置，撤销旧会话并保留资料归属。[密码重置说明](docs/PASSWORD_RESET.md)。管理员录入、厂商自行入驻、企业和产品编辑提供规范、样例及字段级报错；企业 Logo／封面独立上传预览；厂商可补充分类，由后台统一调整；管理导航采用深蓝色。[填写、分类与交付说明](docs/ONBOARDING.md)。工业云展厅视觉保留，企业门户栏目按公开资料显示，[界面说明](docs/INDUSTRIAL_UI.md)、[多媒体使用与部署](docs/MULTIMEDIA_SHOWROOM.md)。管理员密码沿用现有配置。

## 本机运行

本机数据库为 `127.0.0.1:13306`，默认开发连接账号为 `root/root`。应用数据库为 `zqzt_exhibition`，已有其他数据库不参与迁移。首次启动只初始化展馆、两个基础分类和管理员，不生成展示厂家。

1. 复制 `server/.env.example` 为 `server/.env`，填写 `ADMIN_PASSWORD`（首次初始化至少 12 字符）。当前本机配置已写入该文件，可查看其中的管理员账号密码。
2. 在 `web` 目录执行 `npm ci`、`npm run build`。
3. 在项目根目录执行 `./scripts/start.ps1`，浏览 [网站](http://localhost:8080)、[厂商入口](http://localhost:8080/join)、[管理后台](http://localhost:8080/admin)。后台账号使用 `ADMIN_USERNAME`，密码使用初始化时的 `ADMIN_PASSWORD`。
4. 停止本项目后台进程使用 `./scripts/stop.ps1`。日志位于 `.local/server.*.log`。

开发时可分别执行 `cd server; go run .` 和 `cd web; npm run dev`，网站开发端口为 5173，并代理 API 到 8080。后端运行目录必须为 `server`，以读取 `.env`。数据库迁移由启动程序读取嵌入的 SQL，编号记录在 `schema_migrations`。

管理员初始化密码只在首次创建管理员时生效；修改环境变量不会覆盖数据库里的现有密码。

本机已按用户指定导入“河北冀农农机具有限公司”示例：1 个展位、19 款产品和 54 张企业/产品图片，可查看 [厂商展位](http://localhost:8080/vendors/1)。整理依据、图片错名处理、导入命令和测试见 [导入记录](docs/V0001_IMPORT.md)。首次安装的初始化逻辑仍不生成展示企业。

网站提供 [逛展找厂](http://localhost:8080/browse) 和 [逛展找产品](http://localhost:8080/browse/products) 两个入口，首页搜索可切换搜索方式。产品列表展示图片、名称、所属厂家及地区，支持关键词、分类（含子分类）和厂家地区筛选；详情返回保留来源列表的筛选与页码，中英文切换保留状态。公开产品列表接口新增 `region_id` 筛选及 `vendor` 摘要，原有分页结构兼容；Flutter App同步产品浏览、所属厂家与地区信息。

公司专用账号可从 [厂商后台登录](http://localhost:8080/merchant/login)，登录后编辑现有展位与产品。该账号已关联冀农原有资料，操作说明和管理员配置命令见 [厂商账号说明](docs/MERCHANT_LOGIN.md)。

阿里云短信验证码登录已接入；真实发送需填写签名、模板与访问密钥。配置后执行 `./scripts/enable-aliyun-sms.ps1` 切换并重启，详见 [短信接入说明](docs/ALIYUN_SMS.md)。

新版首页提供 [下载 App](http://localhost:8080/app)，Android APK 可直接下载，iOS 标记即将上线。App 底部提供看厂家、找产品、收藏、我的展位四项主导航，启动仍显示展馆首页；支持访客本机收藏、普通访客账号和登录后跨设备同步，需先升级配套后端。[收藏与升级说明](docs/FAVORITES.md)。厂商中心的工作台、展位管理、产品管理、展台推广改为顶部标签，展位管理按任务分组，退出登录移入账号菜单。[本次优化说明](docs/APP_NAVIGATION.md)。网站与 Android 厂商中心支持公司账号密码登录及国内优先的分享面板。[新版操作说明](docs/MERCHANT_EXPERIENCE.md)。当前手机同网下载地址为 [局域网下载页](http://172.27.35.2:8080/app)，电脑 IP 改变后需更新配置与 App 设置。

## Android App

源码为 `mobile`，包含 Android 与 iOS 工程。构建工具版本及验证结果见 [测试记录](docs/TESTING.md)，接口契约见 [OpenAPI](docs/openapi.json)。

```powershell
. ./scripts/flutter-env.ps1
cd mobile
& $taskFlutter pub get
& $taskFlutter analyze
& $taskFlutter test --concurrency=1
```

本轮在项目根目录执行 `./scripts/build-apk.ps1 -Preview` 生成原签名 Release 验收 APK，输出到 `delivery/android-preview/`，并提供独立 `/downloads/previews/` 下载。预发布版本禁止覆盖原稳定 `dalunongji.apk`；默认服务器地址保留，局域网验收在 App 设置切换。

手机与电脑需在同一网络。App 右上角设置可调整 API URL；电脑 IP 改变后在此更新。Android 模拟器一般使用 `http://10.0.2.2:8080`。更改地址会清除登录会话，草稿按来源地址和账号隔离。网络连接失败时先检查地址、电脑防火墙及 8080 端口可达性。

厂商可先填写并预览本机草稿；云端保存与审核前要求手机号绑定。开发模式验证码显示在登录页面，仅用于本机测试。相机和相册选出的照片复制到 App 文件目录，失败后保留，重试时跳过已成功上传的照片。每份内容最多 12 张图片。

签名密钥与密码保存在 `.local/signing`，`mobile/android/key.properties` 引用该密钥；这些文件都已忽略，不进入源码包。后续升级必须使用同一密钥，需安全备份该目录。构建脚本发现已存在的密钥时复用，不自动更换。

## 关键规则

- 草稿和待审提交只有本人和管理员可读取，媒体文件不直接挂在静态目录。
- 每个厂商最多 1000 款产品（包括草稿及下架记录），已创建产品可继续修改，超限返回清晰提示。
- 提交内容被冻结，审核通过原子切换公开版本；复审期间旧版文字和图片继续展示。
- 产品可先审核，但其厂商展位未公开时产品和图片仍不可公开访问。
- 下架立即退出公开查询。重新公开需要另存草稿并提交审核，下架后的旧提交不能再次通过。
- 英文资料选填；固定界面来自本地资源文件，内容缺英文时回退中文原文。
- 推荐项由后台指定；没有推荐项显示空状态，不编造公司、照片、产业带或认证标识。

## 验证与部署

交付文件位于 `delivery`：`android/dalunongji.apk`、Windows/Linux amd64 后端、`web/dist`、完整源码 ZIP 和服务器部署 ZIP。`manifest.json` 记录产物大小及 SHA-256。源码包不含工具缓存、数据库媒体、开发环境配置或签名密钥，签名目录需单独安全保管。

```powershell
cd server
$env:RUN_MYSQL_TESTS='1'
go test -v ./...
go vet ./...
```

集成测试使用 `zqzt_exhibition_test_时间戳`，测试后只清理该次创建的独立库。实际部署、短信配置、备份和恢复参见 [部署说明](docs/DEPLOYMENT.md)。正式环境短信、Android 真机和 iOS 构建结果均按测试记录标注，不以开发验证码替代正式短信验收。

## V1.6.0 后台运营与数据管理

新增访问分析、平台操作员授权、本地/OSS 存储配置与历史迁移、页面加密备份及隔离恢复切换。Android 为 `1.6.0+28`，沿用原签名。操作入口、配置、统计口径、迁移与恢复流程以及测试限制见 [V1.6.0 说明](docs/OPERATIONS_V1.6.0.md)。生产启用和切换仍由管理员在页面发起。

企业独立展厅 V2.0（2.1.0）交付、验证及部署说明见 [docs/SHOWROOM_V2_UPGRADE.md](docs/SHOWROOM_V2_UPGRADE.md)。

## V2.0 P0 / P1 补齐交付（2026-10-18）

见 [功能、测试、迁移与部署回滚记录](docs/V2_P0_P1_ACCEPTANCE.md)。本地服务已更新；生产未部署，Android APK保持原版本。

产品提交页V2.2：三步骤、最少必填、草稿及图片优化，见[交付与验证说明](docs/PRODUCT_V22_UPGRADE.md)。

厂商注册与企业资料填写升级：工作台内完善、省市县、范文与移动端，见[交付与迁移说明](docs/COMPANY_ONBOARDING_UPGRADE.md)。

我要入驻／厂商登录入口优化（2026-10-19）：统一蓝白布局、手机适配、密码显隐及异常恢复，见[使用、截图与验证说明](docs/MERCHANT_ENTRY_UPGRADE.md)。本地可查看，未部署生产。
