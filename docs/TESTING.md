# 实施与验收记录

## V1.5.1 高端企业云展厅 UI（2026-10-12）

- 网站 21 项测试、TypeScript/Vite 生产构建通过，覆盖新版导航、搜索、分页、详情返回、公司账号和原审核操作。证据：`test-output/ui-web-test.txt`、`ui-web-build.txt`。
- 实际浏览器检查桌面 1440/1920、平板 768、手机 390/360，验证真实图片、单厂商宽幅展示、代表展品、五栏目、英文、旧地址参数和产品详情返回；另验证厂商/管理员工作台、长企业名、失败图片和无精选资料。测试修改只发生在浏览器模拟响应，服务器除登录会话外业务数据只读。证据：`ui-browser.txt`、`ui-management-browser.txt`、`ui-admin-browser.txt`。
- Flutter analyze 零问题；12 项常规组件测试通过，单独的 4 项专项视觉测试含原生截图生成通过。验证完整设备图片、公开产品接口、窄屏/1.4 倍字体、吸顶栏目、搜索恢复及已有草稿、登录、二维码和上传处理。常规执行跳过需本机公开图片副本的截图任务。证据：`ui-flutter-analyze.txt`、`ui-flutter-test.txt`、`ui-flutter-screenshots.txt`。
- APK **1.5.1+18** 原签名 Release 构建成功，签名证书与此前一致。APK SHA-256：`0923cc89ce0a0f97896eb553521f3441be6af5d76ae1c4e460ef0ac11e1cb97e`，三个副本一致。证据：`ui-apk-build.txt`、`ui-apk-signature.txt`、`ui-apk-metadata.txt`。
- ADB 无连接设备；真机覆盖安装、字体、播放、文件选择、阅读与分享仍为待验证项。iOS 保持即将上线。本版不改变后端业务规则，无新增迁移。视觉说明、桌面/手机/App 截图见 [工业云展厅 UI](INDUSTRIAL_UI.md)。

## V1.5.0 五栏目展台与多媒体管理（2026-10-12）

- 完整 Go/MySQL 集成测试和 `go vet ./...` 通过。新增测试在独立库覆盖资源草稿与旧公开版本、独立审核、冻结和驳回重提、账号隔离、数量限制、封面类型、部分迁移恢复与重复执行、父展位与产品下架后的资源/文件权限、Range/HEAD、PDF 损坏与加密拒绝、横竖静音视频转换、失败重试与重启队列恢复。证据：`test-output/resources-backend.txt`、`resources-go-vet.txt`。
- 网站 21 项测试、TypeScript/Vite 生产构建通过。实际 Edge 浏览器在独立数据库验证默认展品、五栏目、播放/拖动/离开暂停、PDF 实际绘制及下载、图片册翻页与排序、上传处理—提交—管理员预览审核—公开展示闭环，以及 390 像素手机宽度无横向溢出。PDF.js 主线程和独立 Worker 均提供旧浏览器所需的标准 API 补充。证据：`test-output/resources-web-test.txt`、`resources-web-build.txt`、`resources-browser.txt`，截图 `screenshots/resources-*`。
- Flutter analyze 零问题，9 项测试通过，包括既有公司账号、草稿和大字体测试，以及五栏目默认状态、分类数量、资源本地文件/上传 ID 恢复、处理中预览和上传进度。证据：`test-output/resources-flutter-analyze.txt`、`resources-flutter-test.txt`。
- Android **1.5.0+17** Release 构建成功，原签名证书 SHA-256 为 `85656c85541b188c832dc48d67cbf320046a4311aa9e0fefcc662dbe2e0f1585`。APK SHA-256 为 `16b28d31b8e4a8f7a9c4868eb09f259af27656dfb34de25e4fbee8bcee221c65`，交付文件与网站下载副本一致。证据：`test-output/resources-apk-build.txt`、`resources-apk-signature.txt`、`resources-apk-metadata.txt`。
- ADB 无连接设备，覆盖安装、Android 真机视频播放/拖动、文件选择/取消、断网后重启重传、原生 PDF/图片阅读与系统保存仍为待真机验证项。组件和构建结果不代表真机通过。证据：`test-output/resources-android-devices.txt`。iOS 保持即将上线。

使用和运维说明见 [多媒体展台](MULTIMEDIA_SHOWROOM.md)，交付校验见 `../delivery/manifest.json`。下方为历史验收记录。

## V1.4.9 下载、厂商工作台与分享改版（2026-10-12）

- 完整 Go/MySQL 集成测试通过，`go vet ./...` 通过；业务测试仅使用独立测试库。新增测试验证 23 款产品跨页统计、搜索与组合筛选、账号隔离、旧公开版保留和父展位下架的公开限制。证据：`test-output/experience-backend.txt`。
- 网站 21 项交互测试通过，TypeScript/Vite 生产构建通过。证据：`test-output/experience-web-test.txt`、`experience-web-build.txt`。
- Flutter analyze 零问题，5 项测试通过；覆盖公司账号登录、390×844 手机尺寸与 1.4 倍字体、全量统计、宣传资料保留、本机草稿和 API 地址隔离，以及原生二维码/海报生成。证据：`test-output/experience-flutter-analyze.txt`、`experience-flutter-test.txt`。
- 实际无头 Edge 验证桌面 1440×1000 和手机 390×844 的首页、下载页、厂商工作台与分享；冀农正式资料只读，统计为 19 款公开产品；搜索后编辑返回保留条件。二维码和网页海报均解码为正确的局域网地址，无横向溢出和页面脚本错误。证据：`test-output/experience-browser.txt`、`screenshots/experience-*`。
- 原生二维码及宣传海报的 PNG 经 jsQR 直接解码通过。测试发现抗锯齿造成单元边缘灰线，已改为整数像素和无抗锯齿黑色模块，并重新构建最终 APK。Windows 系统中文字体仅用于测试预览，没有随 APK 分发。
- Android 版本 **1.4.9+16**，沿用原 Release 签名，签名证书 SHA-256 为 `85656c85541b188c832dc48d67cbf320046a4311aa9e0fefcc662dbe2e0f1585`。最终 APK SHA-256 为 `9d623c4200503eb7a1a6b7f9a7b60f1b3b67e69b4f262bd195dffa43e5df4f85`；三个下载副本一致。证据：`test-output/experience-apk-build.txt`、`experience-apk-signature.txt`、`experience-apk-metadata.txt`。
- 本机服务已重启，`http://172.27.35.2:8080/api/v1/health` 返回 200；该地址仅供同网使用。ADB 没有连接设备，因此安装覆盖、真机接收应用与系统文件保存选择器仍需真机验证，未将构建及组件测试记为真机通过。iOS 为“即将上线”，无 IPA 或上架操作。

操作说明见 [厂商体验说明](MERCHANT_EXPERIENCE.md)。本节之后是此前版本的历史记录。

执行日期：2026-10-11，Windows 本机。交付的是本机可运行系统、服务器部署文件和签名 Android APK。正式短信送达及 Android 真机安装仍需外部条件，未记为通过。

## 环境与数据隔离

| 项目 | 实际结果 |
| --- | --- |
| MySQL | 5.7.18，127.0.0.1:13306，utf8mb4 |
| Go / Node.js | Go 1.25.5 / Node.js 24.12.0 |
| React / Vite / TypeScript | 19.2 / 7.3.7 / 5.9，依赖版本见锁文件 |
| Flutter / Dart | 3.47.7 stable / 3.13.5 |
| Android | compile/target SDK 36，min SDK 24，Build Tools 36.0.0，NDK 28.2.13676358 |
| Gradle / JDK | Gradle 9.3.1，Android Studio 随附 JBR |
| 正式数据库 | zqzt_exhibition：管理员 1、基础分类 2、迁移 1；厂家、产品、媒体均为 0 |

已有 `dlc` 与 `dl_nongji_parts` 结构仅作只读盘点，未导入或修改。表名及正式库计数见 [数据库盘点](test-output/database-inventory.txt)。后端自动验收新建时间戳测试库；网页闭环使用 `zqzt_exhibition_test_browser_20261011`、临时端口 8081 和独立图片目录，完成后停止临时服务并清理该库。测试资料均标明“仅测试”或“不入库”。

## 六步执行结果

| 步骤 | 启动 / 构建证据与结果 |
| --- | --- |
| 1. 环境、迁移与契约 | 连接 MySQL 5.7；创建独立库；迁移成功；[OpenAPI](openapi.json) 包含公开、身份、厂商、审核、分类、地区及媒体接口 |
| 2. 业务后端 | [真实 MySQL 集成测试](test-output/backend.txt) PASS；[go vet](test-output/go-vet.txt) exit 0 |
| 3. 厂商 App | [Flutter 分析](test-output/flutter-analyze.txt) 无问题；[2 项组件测试](test-output/flutter-test.txt) 通过；Release 构建成功 |
| 4. 网站与后台 | [TypeScript/Vite 构建](test-output/web-build.txt) 成功；独立库网页入驻、上传、提交、审核、展示闭环通过 |
| 5. 中英文 | [2 项网页交互测试](test-output/web-test.txt) 通过；关键词、分类、地区与语言切换保持状态；App 语言切换保留本机草稿 |
| 6. 打包与恢复 | [APK 构建](test-output/apk-build.txt) 成功；[签名验证](test-output/apk-signature.txt) 通过；Windows/Linux amd64 服务文件已生成；实际 SQL 和图片恢复测试通过 |

本机 Flutter 启动 Gradle 时曾遇到工程目录检查错误。构建脚本保留自动重试：停止旧 Gradle daemon，使用新的 Gradle 进程完成 Release。日志包含第一次失败及随后 `BUILD SUCCESSFUL`，最终交付包来自成功的构建。依赖中的 Gradle 弃用警告不影响本次产物；后续升级工具链需重新验证。

## 自动化业务场景

以下使用真实 MySQL 和实际图片文件，通过 API 发出请求，未以模拟数据库代替：

- 手机验证码发送间隔、错误尝试上限、会话退出失效；管理员密码登录。
- 新库公开内容为空；草稿和待审详情不公开；其他账号无法读取草稿、私有图片或关联不属于自己的图片。
- 提交冻结内容；过期 revision 保存返回冲突；驳回原因必填；重复审核拒绝。
- 产品先审核但展位未公开时，产品详情和媒体仍不可公开，管理端准确返回 `public_visible=false`；展位公开后同步生效。
- 待审修改期间旧公开文字和旧图片保留，新图片保持私有；驳回后修改重提并通过。
- 英文已审核资料匹配、厂家去重、子分类匹配、地区和关键词组合搜索；超长中文查询正常处理。
- 分类父子环与不存在的父分类拒绝；厂商下架使产品、图片和详情同步退出公开访问。
- 并发审核与下架不会让下架内容再次公开。
- 备份 SQL 和媒体 ZIP 后核验校验和，在独立恢复库导入并解压 3 张原图、3 张缩略图，逐字节比较；内容、版本、媒体关联及审核记录数量保持一致。使用恢复库重新启动 API 路由，验证管理员读取私有图片成功、游客读取下架资料和媒体失败。

网页交互测试在 jsdom 中执行真实 React 页面，API 使用明确标记的内存测试响应；验证组合筛选不会丢失输入关键词，以及下架取消不调用 API、明确确认后只调用一次。Flutter 组件测试验证语言切换保留自动保存的草稿，以及 API 地址切换清除会话、按来源隔离草稿。

统一复测入口：项目根目录执行 `./scripts/check.ps1`。需要当前 MySQL 可用；Windows 受限进程环境应允许测试启动备份工具。单独后端测试需 `RUN_MYSQL_TESTS=1`，否则会显式跳过集成测试。

## 浏览器人工检查

在独立测试库完成：游客填写并预览、手机号绑定、云端草稿保存、PNG 上传、展位与产品提交、管理员查看图片并审核、公开厂家和产品同步出现、英文产品关键词搜索、分类和地区组合筛选、中文/英文切换保持筛选、中文内容回退、微信复制到剪贴板和详情图片放大。

原生浏览器确认框曾阻塞内嵌浏览器自动操作，现已改为页面内确认框；其取消与确认逻辑通过网页交互回归测试。实际下架后的公开权限和并发审核冲突由真实 MySQL 集成测试验证。

初始交付时的正式空库检查：桌面 1440 × 960、手机 390 × 844，首页没有虚构厂家、产品或认证；基础分类及入驻引导正常，PC 导航与手机折叠菜单按尺寸呈现。后台可登录，初始审核列表为空、分类维护显示两项基础分类。截图见 [桌面首页](screenshots/home-desktop.png)、[手机首页](screenshots/home-mobile.png)、[管理后台](screenshots/admin-desktop.png)、[独立测试公开展位](screenshots/test-published-vendor.png)。随后根据用户指定导入实际采集厂商，当前数据见 [V0001 导入记录](V0001_IMPORT.md)。

## 安装包与待验证项

APK：`delivery/android/dalunongji.apk`；包名 `com.zqzt.dalunongji`，当前版本名 1.4.1、版本号 15，包含 arm64-v8a、armeabi-v7a 和 x86_64。沿用专用 RSA 2048 位 Release 密钥。[最新包元数据](test-output/apk-V0001-metadata.txt) 与 [最新签名输出](test-output/apk-V0001-signature.txt) 可核对。当前 SHA-256 为 `4393fadc686c4467bf040dde69b88f238812761d8f26728aec9dfa5977617b63`；初始 1.4.0 版本证据仍保留在原包元数据与签名文件，交付清单见 `delivery/manifest.json`。

默认 API 地址为 `http://192.168.1.19:8080`，App 设置可修改；电脑 IP 改变或正式部署时须更新。签名维护与 HTTPS 部署见 [部署说明](DEPLOYMENT.md)。

| 待验证项 | 原因与执行方式 |
| --- | --- |
| Android 安装、启动、相机/相册权限、拨号、网络断开与照片重试 | [ADB 设备列表](test-output/android-devices.txt) 为空，未进行真机操作。连接设备后 `adb install -r delivery/android/dalunongji.apk`，按 README 完成闭环与断网恢复 |
| 阿里云短信送达、供应商异常和正式号码验收 | 未提供服务商凭据；目前仅开发适配器验证。配置环境变量后使用正式号码验收 |
| iOS 编译与设备运行 | 已交付 iOS 工程和用途说明；当前为 Windows，未进行 Xcode 签名构建。本次不交付 IPA |

上述待验证项没有用构建成功或开发验证码结果替代。

## 产品图片链接空白页修复（2026-10-11）

已在运行中的网站复现：从河北冀农厂商展位的产品图册点击“液压翻转犁1LF-440”，进入 `/products/2` 后页面空白，控制台报告读取未定义对象的 `data`。厂商详情和产品详情复用页面组件，旧请求数据在新页面首次渲染时仍被使用，造成异常。

网页请求结果现绑定请求路径、权限入口及刷新版本，切换页面时立即显示加载状态，只显示当前请求的数据，并继续忽略已离开页面的请求结果。[跳转回归测试](test-output/web-product-navigation.txt) 验证厂商图册进入产品、延迟响应期间不显示旧资料、产品联系方式及返回厂商；全部 4 项网页测试通过，[生产构建](test-output/web-build.txt) 成功。

实际浏览器重新加载新版后，沿厂商图册再次进入产品详情正常；4 张缩略图加载成功，原图放大（1000 × 667）、多图切换及关闭正常，参数与拨号入口显示正确，没有新增控制台异常。[修复截图](screenshots/product-link-fixed.png)。此次仅更新网页，没有修改厂商、产品或数据库数据。

## 阿里云短信补充验证

已补充 [短信适配器与登录测试](test-output/aliyun-sms.txt)：RFC3986 编码、HTTPS SendSms 请求字段、中文签名和验证码模板参数、成功与限流/HTTP/JSON/网络失败响应、缺少配置拒绝，以及真实 MySQL 下的验证码登录、单次使用、发送间隔、失败发送回滚和不泄露开发验证码。外部 HTTP 由模拟供应商响应替代，未发送真实短信。真实配置和送达仍待验收，操作见 [短信说明](ALIYUN_SMS.md)。

## 网站产品浏览补充验证（2026-10-11，平台 1.4.4）

新增 `/browse/products`，导航、手机菜单和首页搜索提供独立产品入口；热门产品的“查看全部”指向产品目录。产品卡片展示图片、名称、公开厂家名称及厂家地区，图片保持完整比例。找厂与找产品切换保留关键词、分类、地区并重置页码；产品或厂家详情返回保留来源列表筛选与页码，直接访问产品详情默认返回产品目录。

- [后端完整测试](test-output/backend.txt) 与 [go vet](test-output/go-vet.txt) 通过。新增 `TestPublicProductBrowse` 在独立 MySQL 库验证型号与中英文关键词、父子分类、厂家地区、组合筛选、分页、空结果、草稿/待审隔离、厂家待审修改不改变公开摘要，以及产品/厂家下架退出公开结果；没有向正式库写入测试资料。
- [7 项网页交互测试](test-output/web-product-browse.txt) 通过，包括首页默认找厂与切换找产品、热门产品目录链接、直接访问详情的返回入口、从产品或厂家详情返回时保留筛选和分页、语言切换回退中文、两种浏览模式切换，以及先前图片跳转空白页、账号登录、筛选和下架确认回归。[TypeScript/Vite 构建](test-output/web-build.txt) 成功。
- [运行服务只读验收](test-output/product-browse-public-verification.json) 核对冀农 19 款产品、19 张卡片缩略图、公开厂家名称和地区、型号/父分类/地区组合搜索、空结果及分页无重复。
- 实际浏览器验证：首页选择“找产品”搜索 `1LF-440`，进入对应产品结果；`440＋农业机械＋宁晋地区` 命中 1 款；点击图片正常打开详情，返回保持筛选；中英文切换保持筛选并回退中文资料；空结果有产品专用提示，清除后恢复 19 款。新版未出现控制台错误。
- 电脑 1440 × 960 和手机 390 × 844 均无横向溢出，手机保持两列产品卡片、筛选表单及双入口菜单。[电脑截图](screenshots/product-browse-desktop.png)、[手机截图](screenshots/product-browse-mobile.png)。检查后恢复原浏览尺寸。

此次没有数据库迁移，未修改正式厂商和产品资料；已更新 Windows/Linux 后端、网页、源码 ZIP 和部署 ZIP。Android/iOS 工程与 APK 保持本次修改前版本。

## 管理员运营工作台验证（2026-10-11，平台 1.4.5）

- [后端完整回归](test-output/backend.txt) 共 11 项测试通过，含独立 MySQL 库的管理员代录与配额测试；[Go vet](test-output/go-vet.txt) 通过。覆盖新账号及厂商事务创建、无展位账号关联、重复展位拒绝、失败创建回滚、产品和图片保留厂商归属、跨厂商图片拒绝、版本冲突、待审锁定、驳回重提、单独审核、旧公开版本保留、所属厂商下架后的公开与精选限制，以及组合筛选、分页、真实统计、日志完整性、历史去重和密码/哈希/会话不泄露。1000 款边界与并发最后名额的既有测试继续通过。
- [网页交互测试](test-output/admin-web.txt) 共 13 项通过，包含 6 项新增后台场景：独立布局、默认工作台与导航折叠记忆、深链接和语言筛选保持、登录失效后回到原页面、初始密码仅创建后显示且关闭后清除、待审编辑禁用与驳回原因必填、扩展说明页无执行按钮。原公开浏览、产品图片空白页修复、厂商账号登录和下架确认回归保留。[生产构建](test-output/web-build.txt) 成功。
- [运行服务只读检查](test-output/admin-runtime-verification.json) 核对真实厂商 1 家、产品 19 款、公开数量一致，产品与图片所属账号保持原样，最新与公开版本对照存在，型号与父分类筛选正确，账号、日志、状态和认证保护正常。43 条既有审核操作按来源 ID 迁入通用日志。冀农 [54 张图片与 108 个原图/缩略图请求](test-output/V0001-public-verification.json)、[19 款产品检索与分页](test-output/product-browse-public-verification.json) 验证通过。
- 实际浏览器在 1440 × 960 检查工作台、导航收起后刷新恢复、产品组合筛选、中文/英文切换后刷新、详情公开版本对照及返回来源筛选。390 × 844 检查手机抽屉、菜单跳转、系统状态与产品表格；表格在容器内横向滚动，页面没有横向溢出，19 张产品缩略图加载成功。后台沿用原账号登录，未修改其密码。见 [桌面工作台](screenshots/admin-console-desktop.png)、[手机抽屉](screenshots/admin-console-mobile-navigation.png)、[手机状态](screenshots/admin-console-mobile-status.png)、[手机产品](screenshots/admin-console-mobile-products.png)。
- 正式库迁移前，数据库和媒体备份到 `.local/backups/pre-admin-v145`。SQL 和媒体 ZIP 的 SHA-256 均通过，并在独立恢复库 `zqzt_exhibition_restore_1791716707125` 恢复 20 条厂商/产品与 54 条图片；恢复图片位于该备份目录的新子目录。迁移 `003_operation_logs.sql` 已应用，未向正式库写入测试厂商、产品或图片。数据库及媒体配套恢复测试同时检查新操作日志表。

已构建并启动新版 Windows 服务，交付 Linux amd64 后端、网站及 1.4.5 源码包/部署包；Flutter 工程与 APK 保持既有功能和签名。阿里云实际短信凭据尚未补充，后台只展示配置状态；本次未发送真实短信。
# V1.4.7 厂商地址与地图定位（历史记录，已于 2026-10-12 回滚）

本次不修改正式库的厂商资料、产品和图片，不增加数据库迁移。冀农的实际位置须在配置有效百度 AK 后由管理员或厂商人工校准，并单独提交审核。

| 验证内容 | 结果与证据 |
| --- | --- |
| 后端完整测试：账号、媒体、审核、下架、后台代编辑、搜索、千款产品及恢复 | PASS；`test-output/map-go.txt` |
| 地图专项：坐标与缺失字段校验、公开配置响应头、跨厂商修改、待审锁定、旧公开坐标、驳回重提、地址变更、旧客户端保留、明确清除、下架 | PASS；独立 MySQL 测试库；`test-output/map-focused-go.txt` |
| 网页交互与回归测试 | PASS；4 个测试文件共 26 项；`test-output/map-web.txt` |
| 模拟百度 SDK 的地址定位、拖图、拖标记、点击、确认、附近地址不覆盖原文 | PASS；模拟 SDK，非真实地图精度验收 |
| 新手动位置不被迟到的地址解析覆盖，延迟加载、无 AK、网络失败与重试 | PASS；地图组件自动化测试 |
| 中英文、展台产品详情往返、筛选保持、地图链接地址与 BD-09 坐标编码 | PASS；地图组件与展台页面回归测试 |
| TypeScript/Vite 生产构建、Go vet、Windows/Linux amd64 构建 | PASS；`test-output/map-build.txt`、`test-output/map-vet.txt` |
| 本机服务、公共配置、冀农地址、19 款产品的 12+7 分页、页面及静态资源 | PASS；`test-output/map-runtime.json`；正式库测试厂商写入数 0 |
| 冀农 54 张图片及缩略图，共 108 次读取；型号搜索、旧地址、分享元数据与原海报二维码 | PASS；`test-output/showroom-runtime-verification.json` |
| 真实底图、AK 白名单、农村地址解析准确度与厂址人工校准 | 待验证；当前 `BAIDU_MAP_BROWSER_AK` 为空，启用的是地址查看链接 |
| 桌面/手机浏览器外观、真实触摸拖动 | 待验证；浏览器工具因 URL 策略拒绝绑定当前标签页。本轮未取得外观截图，不以组件测试替代外观验收 |

以上为回滚前的历史验收记录，地图功能及配置说明已移除。Flutter 工程与 APK 未改变。

## V1.4.8 地图需求回滚（2026-10-12）

按用户要求撤回“厂商展台嵌入百度地图”和“厂商自主地址定位、拖动校准”两个需求。公开展台、厂商编辑及管理员代编辑恢复文字地址填写与展示；地图 SDK、查看链接、坐标字段、地图配置接口、AK 配置和相关操作资源均从当前实现移除。原有企业宣传资料、产品目录、账号、审核与图片功能继续保留。

- 网页回归：3 个测试文件、17 项通过；`test-output/map-rollback-web.txt`。
- 后端完整验收在独立 MySQL 测试库进行；`test-output/map-rollback-go.txt`。Go vet、Windows/Linux amd64 和网站生产构建通过；`test-output/map-rollback-vet.txt`、`test-output/map-rollback-build.txt`。
- 运行服务检查确认地图配置接口返回 404，网站构建不包含地图组件或百度 SDK；冀农文字地址、19 款产品、公开图片和原有宣传资料正常。证据见 `test-output/map-rollback-runtime.json`。
- 正式库及媒体不执行删除或迁移，不写入测试厂商。回滚前测试记录和旧版压缩包仅作为历史记录；当前交付版本为 1.4.8。
- Flutter 工程和 APK 不变。本轮使用组件回归、构建产物及接口验证，未重新进行浏览器截图验收。


V1.5.0 正式服务只读复核：迁移版本 1–5 完整；冀农仍有 19 款公开产品，视频和图册为 0，页面显示明确空状态；本机及同网健康接口正常，APK 下载 HTTP 200。证据：test-output/resources-production-smoke.txt、resources-production-migration.txt、resources-production-browser.txt。



## 1.5.2+19 厂商录入与自行入驻（2026-10-12）

- 网站 5 个测试文件、23 项通过，生产构建通过：`test-output/onboard-web-test-final.txt`、`onboard-web-build-final.txt`。
- 后端 `RUN_MYSQL_TESTS=1 go test -count=1 -v ./...`，22 项顶层测试通过（包含独立数据库案例），Go vet 通过；包含字段字符/字节、分类私有隔离/重复/发布/规范/合并、旧 ID 筛选和数量、12 张图库加品牌图片、私有图片及旧公开版本、部分迁移恢复。`onboard-backend-final.txt`、`onboard-go-vet.txt`。
- Flutter analyze 无问题；14 项组件测试通过，截图专用测试独立通过。`onboard-flutter-analyze-final.txt`、`onboard-flutter-test-final.txt`、`onboard-native-screenshots.txt`。
- 浏览器业务变更仅使用隔离测试库：管理员字段定位、样例与官网格式、真实品牌图片上传、上传失败阻止保存/移除后继续、厂商补充分类与管理合并、未登录选图/账号登录续填/原展位不覆盖、中英文及 360/390/768/1440/1920px 页面无整体横向溢出。`onboard-browser-final.txt`。
- 升级前备份已校验并恢复到独立库，原库不覆盖：`onboard-backup-restore.txt`。正式库自动迁移 1–6 完整，既有 21 条内容保留，冀农 19 款公开产品保留；本机和同网健康接口、下载及网站入口 HTTP 200：`onboard-production.json`。
- APK 1.5.2+19，原签名 SHA-256 `85656c85541b188c832dc48d67cbf320046a4311aa9e0fefcc662dbe2e0f1585`；构建/版本/签名证据 `onboard-apk-build.txt`、`onboard-apk-version.txt`、`onboard-apk-signature.txt`。设备列表为空，真机安装升级、相册上传、应用重启文件恢复、设备大字体待验；见 `onboard-device.txt`。
- 完整使用、截图与边界说明：[ONBOARDING.md](ONBOARDING.md)。本轮不执行 iOS 上架或公网部署。

## 1.5.3 管理员厂商密码重置（2026-10-13）

- 网站 6 个测试文件、25 项通过；生产构建通过。覆盖取消不写入、手动校验、重复点击、关闭清除、复制失败、两个入口及中英文。证据 `test-output/reset-web-test.txt`、`reset-web-build.txt`。
- 后端开启 `RUN_MYSQL_TESTS=1`，独立 MySQL 库完整 25 项顶层测试通过，Go vet 通过。新增案例涵盖多客户端会话撤销、旧／新密码、角色与短信账号限制、UTF-8 边界和空格保留、内容版本和图片归属不变、事务失败回滚、并发登录／重置。证据 `reset-go-final.txt`、`reset-go-vet.txt`。
- 真实浏览器验收在独立库执行，覆盖随机和手动重置、取消、一次性显示、剪贴板失败、360/390/768/1440/1920px、中英文；截图中的密码遮挡。`reset-browser.txt` 和 `screenshots/reset-*.png`。
- 部署前备份 SQL／媒体，并恢复到新独立库校验 SHA-256。`reset-backup.txt`、`reset-backup-restore.txt`。本次无数据库迁移。
- Windows 服务替换时等待旧进程完全退出后重启；确认运行二进制与构建一致。本机和同网健康接口 HTTP 200，新接口匿名访问 401，丹顿液压原账号密码指纹、归属及原数据版本／数量与部署前相同。`reset-production.json`。真实厂商密码未重置。
- 网站／服务及新源码／部署包为 1.5.3。Android 仍为 1.5.2+19，APK 字节及签名均保持原包，下载核对通过，无需重新打包；本次未做新增 Android 真机验收。
- [使用及接口说明](PASSWORD_RESET.md)，OpenAPI 476 个本地引用解析通过。构建存在单个 JavaScript chunk 略超过 500 kB 的提示，不影响构建或验收。

## 网站 1.5.4 分类补充入口（2026-10-13）

- 网站 7 个测试文件、29 项通过；生产构建通过：`category-entry-web-test.txt`、`category-entry-web-build.txt`。新增用例覆盖登录取消及续填、文字和待上传图片保留、回车仅提交分类、失败重试、刷新中立即选中、重复分类和正式上级复用、待审禁止、中英文。
- 独立 MySQL 库原分类集成测试通过，验证私有分类隔离、去重、审核公开、旧 ID 合并筛选及统计：`category-entry-backend.txt`。后端代码不变，不重新执行无关测试或编译服务。
- 浏览器真实验收使用独立数据库，覆盖未登录／登录后入驻、企业资料编辑、产品新增／编辑、公共菜单和其他厂商隔离，以及 360/390/768/1440/1920px、中英文；主表单无意外写入。`category-entry-browser.txt` 和 `screenshots/category-entry-*.png`。
- 正式库只读快照及后端／APK 哈希前后相同，确认分类、内容、账号、审核版本和媒体归属未变；本机及同网服务 HTTP 200，新按钮文本位于实际服务的网站资源中，APK 下载字节相同。`category-entry-production.json`。
- 网站和交付包版本 1.5.4，服务维持 1.5.3，原签名 APK 维持 1.5.2+19。无需迁移、服务重启或 App 重打包。本轮未做新的真机验收。
- [入口和截图说明](CATEGORY_ENTRY.md)。Vite 的单个 JavaScript chunk 超过 500 kB 提示沿用，不影响生产构建。
# 1.5.5 新厂商注册与入驻验收（2026-10-13）

- 网站38项测试通过：注册后自动保存／提交、取消意图清除、分类认证续接、已有展位保护、上传失败重试与最新编号、重复创建保护、密码不落草稿、UTF-8长度和网络中断。`test-output/registration-web.txt`。
- MySQL全套后端测试与Go vet通过。新增独立测试库覆盖注册角色、已有登录兼容、空格密码、100字符／中文／emoji字节边界、重名与并发唯一约束、每IP每分钟5次、账号／会话／日志事务回滚、响应和日志不泄露密码。`test-output/registration-backend-all.txt`、`registration-backend.txt`、`registration-vet.txt`。
- 浏览器在8081隔离数据库真实完成“填写 → 注册 → 保存草稿”和“填写 → 注册 → 提交审核”；验证Logo、封面、图库失败重试，成功图片不重传、最新编号完整保存；提交失败重试只有一个展位，待审不公开。取消、注册重名、网络中断后登录、已有管理员代录资料保护、刷新文字草稿、分类认证仅返回分类操作均通过。`test-output/registration-browser.txt`。
- 原分类入口浏览器回归通过，补充分类不进入公开或其他账号菜单，保留输入、当前图片和选中分类，回车不提交主表单。`test-output/registration-category-regression.txt`。
- 360／390／768／1440／1920px检查无横向溢出，中文／英文注册弹窗与成功工作台截图已复核，未出现页面异常。截图 `screenshots/registration-*.png`；密码区域遮罩。网站最终1.5.5构建通过，保留原PDF代码包体积提示。`test-output/registration-build.txt`。
- 部署前备份 `.local/backups/pre-registration-v155`，SQL／媒体SHA-256核对和独立库还原通过：迁移6项、厂商及产品26条、媒体84条。`test-output/registration-backup-restore.txt`。
- 当前网站及Windows/Linux服务1.5.5，本机和WLAN `172.27.35.2:8080`健康检查200。正式账号、密码、分类、内容及版本、媒体归属校验一致；仅隔离库创建验收账号和业务资料。`test-output/registration-production.txt`。
- Android仍为原签名1.5.2+19，下载APK SHA-256保持 `031fe2a13b7b6a4fb9b772846f0be4e3b762a196b709ee3769ea40aae521abc0`。新注册账号通过App沿用的 `/auth/merchant` 接口登录与读取原资料验证通过；本轮没有新增App界面或重新打包，未进行新的Android真机登录验收。

# 1.5.6 厂商一级分类（2026-10-13）

- 网站39项测试、MySQL全套后端回归、Go vet与生产构建通过。新增用例覆盖经营和产品分类创建一级、即时选中、一级待规范分类作为当前值时默认顶级、同名去重、账号隔离、正常内容审核发布、管理员保留一级／子类提升一级／合并／停用。日志 `test-output/root-category-{web,backend,build}.txt`。
- 隔离数据库真实浏览器验证经营分类及产品分类 `parent_id=0` 创建成功，立即选中并显示待规范；公共菜单不出现；管理员可直接批准为正式一级；同名重复返回同一分类。手机390px、桌面1440px和英文弹窗正常，未发生页面异常。日志 `test-output/root-category-browser.txt`，截图 `screenshots/root-category-*.png`。
- 无数据库迁移。更新前备份 `.local/backups/pre-root-category-v156` 并完成SQL／媒体校验与独立还原。网站与Windows/Linux服务升级1.5.6，服务已重启，本机和当前局域网健康200；APK保持1.5.2+19，SHA-256不变。
- 正式库在工作期间有账号4的正常产品新增及提交记录，因此完整业务快照不同；核对升级前备份时，分类、账号、映射和迁移保持一致。保留所有正常业务记录，不回退用户操作。测试一级分类仅在8081隔离库创建，正式库中没有验收名称。上线只读检查 `test-output/root-category-production.txt`。
- 本次调整网站和共用后端，不修改Android界面或重新打包。

# 网站1.5.7 厂商与产品批量审核（2026-10-13）

- 9个网站测试文件、47项测试及生产构建通过。新增用例覆盖管理／审核列表的待审选择、禁用非待审行、本页全选、筛选与分页清空选择、返回原页不恢复旧勾选、预览、确认、取消、共同驳回原因及Unicode上限、提交版本ID、单条快捷审核、并发冲突、网络中断停止后续项、逐条结果和重复点击保护。`test-output/batch-review-web.txt`、`batch-review-navigation.txt`、`batch-review-build.txt`。
- 8081独立测试库浏览器实际批量通过和驳回厂商与产品；逐条审核历史正确，共同原因保存；模拟其他管理员先处理其中一条，结果显示该条已处理、另一条成功；模拟网络断开结果未确认，未自动重复，刷新后可显式单条审核。厂商修改被批量驳回后旧公开资料仍可访问。普通厂商调用管理员审核接口403。`test-output/batch-review-browser.txt`。
- 桌面1440／1920、平板768、手机360／390px无页面溢出；确认面板可查看完整资料与图片。修正批量驳回按钮文本对比度、图片占位高度，中文／英文截图复核完成。`screenshots/batch-review-*.png`。
- 复用原管理员逐条审核接口，后端业务代码及可执行文件保持1.5.6，无新接口或数据库迁移。权限、事务、审核状态、版本锁、审核历史与操作日志均由原接口处理；本次不新增App功能或重新打包。
- 网站1.5.7已由当前服务提供，刷新后台页面即可看到四个列表的批量菜单。旧静态文件备份 `.local/backups/pre-batch-review-v157/web-dist`。本机／同网健康200，后端与APK哈希不变，正式库无验收审核名称；验收写入只发生在隔离库。`test-output/batch-review-production.txt`。
- 源码包、部署包、截图、操作说明与校验清单同步更新，Android仍为原签名1.5.2+19。


# 网站与后端1.5.8 首页展示（2026-10-13）

- 网站10个测试文件、55项测试通过；最终品牌卡片调整后相关8项再次通过，TypeScript及生产构建通过。上传仅私有预览、失败文件重试、保存失败保留编号、版本冲突刷新、移除、准确数量、12家限制、旧kind参数、公开资料及图片失败占位均覆盖。日志 `test-output/home-display-web.txt`、`home-display-web-final.txt`、`home-display-build.txt`。
- 独立MySQL库全套后端测试及Go vet通过，新增3组首页集成测试验证上传格式／事务回滚、管理员权限、未保存／替换／移除的GET与HEAD权限、原厂商媒体未放宽、并发配置冲突、并发添加最多12家、旧库超限不重写、待审修改使用原公开资料、下架退出首页、007重复执行及中断恢复。`home-display-backend.txt`、`home-display-backend-targeted.txt`、`home-display-vet.txt`。
- Edge浏览器实际配置及厂商创建只在8081独立数据库中进行。上传首次断网后文件保留，显式重试成功；模拟保存500后沿用已上传编号，配置冲突刷新后仍保留候选；真实保存／替换／移除后匿名图片权限立即变化。添加、取消、排序、12／12禁用新增及未审核私有企业名不进入首页均通过。`test-output/home-display-browser.txt`。
- 截图复核0／1／2／3／4／12家：单厂商宽幅、两家双列、三家及以上桌面三列，4家末行居中且卡片高度一致。360／390px手机单列，768px两列，1440／1920px三列；无横向溢出。长名称、无封面／Logo、主视觉加载失败、中文／英文、后台桌面与手机预览已检查。配图不再显示企业名牌或展品浮层，无自动轮播。`screenshots/home-display-*.png`（验收图片及多厂商均为隔离库演示）。
- 升级前备份 `.local/backups/pre-home-display-v158`，SQL和媒体SHA-256核对及独立库还原成功，备份含6项旧迁移、38条厂商／产品、122条媒体；旧网站和Windows服务也已保存。启动新服务后迁移数量为7。`test-output/home-display-backup-restore.txt`。
- 只读上线核对网站与Windows服务1.5.8、WLAN地址 `172.27.35.2:8080`、本机及同网健康200、管理员新接口匿名401；正式资料、精选、分类、账号及密码指纹、审核版本、媒体归属与升级前一致。实际平台图片尚未配置，`hero.media_id=0`，登记表为空；管理员可上线后自行配置，正式库无验收企业。`test-output/home-display-production.txt`。
- Windows/Linux后端、源码／部署包、首页展示说明、截图及校验清单同步1.5.8。OpenAPI477个本地引用解析通过。Android保持原签名1.5.2+19，下载APK SHA-256仍为 `031fe2a13b7b6a4fb9b772846f0be4e3b762a196b709ee3769ea40aae521abc0`；本轮不改App、不重打包、不新增真机验收。Vite沿用单个主JavaScript文件超过500kB的提示，构建成功。

# 网站1.5.9 看厂家封面与横向品牌卡（2026-10-13）

- 11个网站测试文件、61项通过，新增6项验证封面优先／图库兼容、无图片及请求／解码失败时占位、不切换到图库、Logo与产品完整展示、最多3款、规范展厅和联系链接、筛选页码返回、后台定位焦点与草稿预览、待审禁用及中英文。TypeScript和最终生产构建通过：`test-output/vendor-cards-web.txt`、`vendor-cards-targeted.txt`、`vendor-cards-build.txt`。
- 使用既有1.5.8后端和8081独立测试库完成真实上传、草稿保存、提交审核与公开验收。模拟封面上传断网，文件保留、重试后成功，失败文件阻止保存；保存／待审期间原封面继续公开，新媒体匿名404，审批后新封面公开200。管理员快捷入口展开并定位品牌图片区、厂商自行编辑预览正常，待审快捷入口禁用并说明。`vendor-cards-browser.txt`。
- 360／390px上下排列，768px封面与企业信息并排、展品置于下方，1440／1920px封面／企业信息／展品三块并排，封面16:9；0／1／3款产品、长企业名、缺图、中文／英文及英文手机无横向溢出。关键词与分类筛选显示匹配展品，产品详情和联系栏目返回原URL；配置封面加载失败不回退图库。截图 `screenshots/vendor-cards-*.png`，后台图片区单独截图暂隐藏固定顶栏以避免遮挡，完整页面截图保留原界面。
- 网站1.5.9已替换当前静态文件，旧版本备份 `.local/backups/pre-vendor-cards-v159/web-dist`。只读核对正式内容及版本、分类、账号及密码指纹、媒体归属、厂商精选和首页配图设置与升级前一致，无验收厂商／产品进入正式库。后端二进制仍1.5.8，迁移数仍7，无新接口或迁移，无需重启服务。`test-output/vendor-cards-production.txt`。
- 正式页面只读检查各厂商图片字节与其公开封面／图库首图相符，规范联系人跳转正确，截图 `screenshots/vendor-cards-live-browse.png`；无登录或业务写入。`vendor-cards-live-ui.txt`。
- 本机和当前WLAN `172.27.35.2:8080`健康200。Android继续原签名1.5.2+19，下载APK SHA-256仍为 `031fe2a13b7b6a4fb9b772846f0be4e3b762a196b709ee3769ea40aae521abc0`。本轮不修改App或新增真机验收。源码／部署包、说明、截图及校验清单同步网站1.5.9／后端1.5.8／Android1.5.2+19；构建沿用主JavaScript文件超过500kB提示。

# 网站／后端1.5.10 厂商工作台与代表展品（2026-10-14）

- 网站12个测试文件、67项通过，最终未建展位引导与键盘焦点调整后相关6项再通过，TypeScript及生产构建成功。覆盖独立后台／旧链接、保留筛选页码、直接编辑页、公开／待审产品、代表展品3款上限／排序／清空／冲突续接、注册与分类原流程、会话失效原页重新登录及草稿恢复。`test-output/merchant-workspace-web.txt`、`merchant-workspace-final-web.txt`、`merchant-workspace-build.txt`。
- 显式启用 `RUN_MYSQL_TESTS=1`，独立MySQL库全套后端回归通过（46.155秒），Go vet通过。新增代表展品专项测试真实执行，验证默认自动、手动0／1／3款、超限／重复／跨账号／草稿拒绝、公开版本保留、下架失效及恢复、父展位权限、同版本并发仅一方成功、日志失败事务回滚、迁移008中断后重跑和重复执行不覆盖配置。`merchant-workspace-backend.txt`、`merchant-workspace-backend-targeted.txt`、`merchant-workspace-vet.txt`。首次未开启MySQL开关的检查不作为数据库验收依据。
- 8081隔离服务实际使用管理员创建的公司账号登录，新工作台独立布局及导航正常，旧 `/join?tab=productManage&status=approved&page=2` 转到新地址，保留参数；编辑深链接刷新和返回仍是原列表页。代表展品真实勾选3款、上移、版本冲突后刷新保留选择、保存按序公开；目录筛选无匹配时代表展品仍在。产品下架立即退出公开并标失效、不能保存；移除及清空后隐藏。旧App使用的原公开产品接口仍返回原排序。`merchant-workspace-browser.txt`。
- 撤销浏览器实际会话后，未保存产品名称和待上传图片仍保留在原编辑页；重新登录同一账号后显式重试上传成功，本机草稿刷新恢复。短视频、电子图册和推广原组件可用。匿名新厂商真实完成两条“填写→注册→保存草稿／提交审核→进入工作台”流程，成功提示和审核状态正确；测试账号和业务写入仅在隔离库。
- 360／390／768／1440／1920px下工作台、产品管理及代表设置无横向溢出；长企业名、中文／英文、侧栏收起、手机抽屉、Tab／Shift+Tab焦点循环和Esc关闭通过。截图 `screenshots/merchant-workspace-*.png` 已复核，均为独立库演示资料。
- 升级前备份 `.local/backups/pre-merchant-workspace-v1510`，SQL／媒体SHA-256及独立库还原通过：旧迁移7项、厂商／产品38条、媒体125条。旧静态文件和Windows服务同时保存。替换网站与Windows/Linux后端为1.5.10后重启，运行二进制哈希与构建一致，迁移8项，本机／WLAN `172.27.35.2:8080`健康200。`merchant-workspace-backup-restore.txt`、`merchant-workspace-production.txt`。
- 正式内容及审核版本、分类、账号及密码指纹、媒体归属、精选及首页配图配置与升级前一致，配置新表为空，默认代表展品与旧公开列表前3款一致；未自动配置真实厂商，正式库无工作台验收记录。源码／部署包、OpenAPI、说明、截图及校验清单同步1.5.10；482个本地OpenAPI引用解析通过。
- Android仍为原签名1.5.2+19，下载APK SHA-256保持 `031fe2a13b7b6a4fb9b772846f0be4e3b762a196b709ee3769ea40aae521abc0`。本轮不修改App或新增真机验收；旧App仍自动推荐3款，不读取网站手动代表配置。构建沿用主JS超过500kB提示，构建成功。

## 企业宣传门户1.5.11

`RUN_MYSQL_TESTS=1 go test ./... -count=1` 执行独立库回归；`TestPortalFieldLimitsAndLegacyMerge` 与 `TestPortalReviewImagesIsolationAndOldClient` 检查Unicode边界、集合清空／保留、独立资质媒体引用、账号隔离、审核快照及公开撤销。网站运行 `npm test`、`npm run build`；浏览器运行 `node scripts/verify-company-portal-browser.cjs`，仅允许8081隔离fixture。截图及日志为 company-portal-*。生产只读验证使用 `verify-company-portal-production.cjs`，APK保持不变。


## Android功能同步1.5.12+20（2026-10-14）

Flutter analyze无问题，完整回归24项通过、1项原可选截图测试跳过；本轮另外生成6张360／390／768px中英文大字体门户截图并复核。新增10项覆盖平台配图、门户缺资料隐藏、代表展品冲突与清空、注册续接、不持久化密码、顶级分类、Unicode字段限制、上传失败后保留与重试、提交失败不重复创建展位、会话和草稿账号隔离。记录 `test-output/android-sync-analyze.txt`、`android-sync-test.txt`。

注册、资质与图集、分类、代表展品、审核快照及下架权限使用现有后端独立MySQL库回归通过；未向生产数据库写入验收资料。日志 `test-output/android-sync-api-regression.txt`。Widget上传及注册测试使用MockClient，后端独立库回归使用真实MySQL，两者不等同于真机端到端验收。

本机没有已连接Android设备，模拟器工具未配置AVD。APK全新安装、覆盖升级、真机相机／相册、分享和原生媒体播放均未实测；包名、递增版本码、原签名与三架构采用构建产物静态验证。设备结果及安装说明见 [ANDROID_SYNC.md](ANDROID_SYNC.md)。

# Android 1.5.15+23 导航与厂商操作优化（2026-10-15）

- 底部补齐逛展馆、看厂家、找产品、我的展位四项导航；列表搜索、分类、地区、分页与滚动位置保留，首页搜索进入对应列表，详情返回恢复原页面。
- 厂家卡片移除整块展品预览；企业展厅与独立产品列表保留产品展示。
- 厂商内部四项导航改为顶部可滚动标签；账号菜单提供退出登录确认。展位管理按任务分组，编辑入口展开并定位到对应区域，视频与图册独立管理。
- 首次创建展位返回现有 App 主框架，并刷新新展位资料；账号变化重建厂商页面，本机草稿保持账号隔离。
- [静态分析](test-output/app-navigation-static-final.txt) 无问题；[完整 Flutter 回归](test-output/app-navigation-regression-final.txt) 38 项通过，1 项历史截图任务按默认跳过。本次页面截图任务已实际运行并生成 7 张截图；自动布局检查覆盖 360／390／768px、中英文、1.4 倍字体。
- [Release 构建](test-output/app-navigation-build.txt) 成功；[APK 元数据](test-output/app-navigation-apk-metadata.txt) 核对包名、1.5.15／23、最低 API 24、目标 API 36、三种架构；[签名验证](test-output/app-navigation-apk-signature.txt) 与原证书一致。
- [交付校验](test-output/app-navigation-delivery.json) 验证五份本地 APK 完全一致，旧版带版本号 APK 保留。最终包 SHA-256：`cd75a8b1a2b4e3dfed58f303a384332decbe760952bc73d944d15c74bdddca3b`。
- [设备检查](test-output/app-navigation-devices.txt) 无已连接 Android 设备，尚未进行真机安装验收。本次未进行线上发布；网站和后端保持现有 1.5.13 版本。

[页面说明与截图](APP_NAVIGATION.md) · [安装说明](ANDROID_SYNC.md)

# 访客收藏、登录同步与导航 1.5.16（2026-10-15）

- App 底部顺序调整为看厂家、找产品、收藏、我的展位；保留启动首页、顶部返回首页入口，以及主页面返回键先回首页的行为。
- 支持厂商／产品列表和详情的收藏按钮、本机持久化、普通访客注册登录、厂商登录、自动合并和账号隔离。收藏下架内容保留不可用条目；网络失败不误判为下架。
- 同步使用按账号持久化的操作队列和服务器去重账本，覆盖断网、响应丢失、并发重试、账号切换和同账号新会话接续。来迟的旧请求不会借用新账号凭据，也不会把已取消的收藏重新加入。
- [Flutter 静态分析](test-output/favorites-analyze-final.txt) 无问题；[完整回归](test-output/favorites-flutter-final.txt) 57 项通过，2 项历史截图任务默认跳过。本次收藏截图任务已运行，生成 8 张主要页面截图。布局检查覆盖 360／390／768px、中英文及 1.4 倍字体，访客登录注册额外覆盖 360px 中英文大字体。
- [Go 全套独立 MySQL 测试](test-output/favorites-go-regression.txt) 42 项通过，[go vet](test-output/favorites-go-vet.txt) 无问题。迁移 009 的创建和重复启动在测试库验证；原有厂商、产品、审核、资源、注册登录流程回归通过。未在当前业务数据库运行迁移。
- [接口文档](openapi.json) 增加访客注册、登录、收藏解析、列表和同步接口；全部本地 schema 引用已检查。
- 构建 Windows amd64、Linux amd64／arm64 配套后端及原签名 Android 1.5.16+24。安装包元数据、签名和交付清单见 `favorites-apk-metadata.txt`、`favorites-apk-signature.txt`、`favorites-delivery.json`；实际构建记录见 [日志](test-output/favorites-apk-build-final.txt)。
- 本次只生成本地交付，未发布线上。跨设备登录同步须先升级配套后端；未连接 Android 设备，真机安装和覆盖升级未验证。

[收藏与迁移说明](FAVORITES.md) · [主要页面截图](FAVORITES.md#验证与截图) · [Android 安装说明](ANDROID_SYNC.md)
