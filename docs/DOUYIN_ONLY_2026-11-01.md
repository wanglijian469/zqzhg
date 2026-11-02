# 抖音视频最小范围升级与本机验收

本次以 `ac68f37` 为基线，保留已确认的 1.10.6 页脚及后续部署修复。仅增加厂商抖音分享链接和展厅内播放，网站、后端、安卓同步修改。没有部署 ECS、更新正式 APK 下载、提交或推送 GitHub。

本机首页：<http://localhost:18086/>；联调展厅：<http://localhost:18086/v/v-21>；视频管理：<http://localhost:18086/merchant/booth?section=video>。管理页需要登录，可使用快照中原有账号。8080 原服务继续运行；18086 只监听 `127.0.0.1`。

**实际交付的功能**

- 视频管理显示“添加企业宣传视频”，先选择“粘贴视频链接”或“上传本地视频”，不自动选中。
- 链接方式仅支持抖音。支持普通地址、含文字分享内容、官方短链接及本次 `/user/self?modal_id=…` 地址；后者仅提取视频 ID。
- 新表单提供视频名称、链接检查、预览和展示授权确认。修改链接立即使检查及确认失效。合法地址暂时检查失败可保存草稿，不能提交审核；恶意地址不能保存。
- 复用原资源版本和审核：修改后继续展示旧公开版本；已有本地资源不能转换成抖音资源。
- 服务器调用官方接口，安全解析唯一受信任播放器地址，不注入外部 HTML、不下载抖音视频。网站使用受限 iframe；安卓增加无登录令牌的独立 WebView 窗口。
- 默认不自动播放；关闭网站弹窗会卸载播放器。安卓包含全屏、返回和退后台释放处理，但尚未完成设备实播验收。
- 原本地上传表单、MP4/MOV 接口、转码、封面、审核和免费额度不变。免费额度仍为 **200,000,000 字节**。外链不占视频文件空间，自行上传的封面使用原存储计量。
- 快手、B站均未实现，界面不将它们展示为可用来源。没有新增删除、排序、文件清理或来源互转业务接口。

官方接入依据：[通过 VideoID 获取 IFrame 代码](https://developer.open-douyin.com/docs/resource/zh-CN/dop/develop/openapi/video-management/douyin/iframe-player/get-iframe-by-video)。官方接口成功只表示取得可用预览信息，页面仍提示厂商实际预览确认。

**原版与新版的对照**

先保存了本轮前的源码补丁和 ZIP，再从原本机库制作完整隔离快照，复制媒体及地图资源。使用 `zqzt_exhibition_test_douyin_full_20261101192749`，没有继续使用先前“1家厂商、1个产品”的示例库。仅在副本原有厂商 21 中联调视频及 PDF，没有新增厂商或首页案例。

- 首页：厂商 **11** 家、产品 **46** 个，原版和副本一致。
- 全国 → 河北 → 邢台 → 宁晋：厂商统计 **11 / 10 / 8 / 8**，四级边界哈希及统计响应一致。
- 1440、768、390、360px：首页案例链接及页脚内容、导航链接一致，无横向溢出；地图下钻及手机布局通过。
- `contents`、`revisions`、首页配置逐表一致；原有 4 条资源和对应版本一致。原库没有抖音记录。
- 首页、公共页脚、地图、原上传处理等受保护源码与基线一致，比较时仅统一 Windows 换行符。播放器策略限定在新播放承载页，没有更改其他页面的安全策略。

完整证据见 [验收汇总](qa/douyin-only-2026-11-01/acceptance.json)、[数据对照](qa/douyin-only-2026-11-01/data-parity.json)、[地图统计](qa/douyin-only-2026-11-01/map-counts.json)。

**真实测试结果**

| 检查 | 结果 |
|---|---|
| Chrome 浏览器检查 | **75项通过，0项失败**；包含对照、4种宽度、管理双入口、原视频与图册、产品页和注册登录入口 |
| 已授权抖音视频 `7690195495465012480` | 官方检查、预览、保存、提交、审核、公开解析通过；桌面 Chrome 和 Pixel 5 手机模拟中，播放/暂停/全屏/退出/关闭卸载通过 |
| 原本地功能 | 历史视频播放/暂停/关闭、图片图册通过；生成竖屏 MP4 经原上传、转码、审核后实际播放通过 |
| PDF | 生成的测试 PDF 经原上传和审核，在 1440、390px 实际渲染非空白页面通过 |
| 网站类型检查、构建 | 通过；存在原构建体积提示，不作为功能失败 |
| 网站相关组件 | **41/41通过**，含新播放器、资源弹窗、两种页脚及地图 |
| 服务端抖音相关测试 | 通过：链接/短跳转/内网防护/伪造播放器、权限、去重、草稿与旧公开版本、官方错误/超时缓存、并发检查合并、旧客户端、开关回退及迁移重入 |
| 安卓 | 静态检查通过；**7/7** 新视频及原资源相关测试通过；独立 APK 构建、包名/应用名称和签名检查通过 |

[浏览器逐项结果](qa/douyin-only-2026-11-01/browser-results.json)、[抖音流程](qa/douyin-only-2026-11-01/flow-results.json)、[本地上传流程](qa/douyin-only-2026-11-01/local-flow-results.json)和相关测试日志保存在同一证据目录。

全套旧测试没有全部通过，不能视为全量验收通过：当前前端为 274通过、36失败及7个未处理错误；独立基线为267通过、37失败及7个未处理错误。35项失败相同，当前额外一项企业表单超时在单线程复跑中通过；单线程仍有一项旧企业表单失败，在基线也复现。旧服务端 `TestResourcesIndependentReviewAndVisibility` 与 `TestVideoWorkerProcessingRetryAndRecovery` 都因 `agreement_required` 失败，基线有同样结果。本次没有修改这些无关用例或绕过协议确认；实际副本联调按照现有协议流程执行。

**尚未完成的验证**

- 没有在线安卓真机或可运行模拟器。`adb devices` 无设备，API35 模拟器报告宿主缺少加速驱动；没有改动 Windows 虚拟化设置。APK 构建不等于安卓播放、全屏、返回和生命周期实测通过。
- 手机网页结果来自真实 Chrome 的设备模拟，不是真实手机浏览器实测。
- 只有用户提供的横屏抖音视频完成实际播放。抖音竖屏和真实分享短链接缺少可用素材，未完成实播；短链及非公开/失效/超时等情况以契约测试验证，不冒称真实公开视频已覆盖全部情况。
- 快手、B站本期未实现。

**改动文件**

- 网站：`web/src/resources.tsx`、`video-playback.tsx`、`douyin-video.css`、`video-playback.test.tsx`、`resource-messages.ts`、`runtime-config.ts`、`showroom.tsx`。
- 服务端：`server/resources.go`、`video_links.go`、`video_resources.go`、`content.go`、`main.go`、`permissions.go`、`procurement_demo.go`、`showroom.go`、`.env.example`、`migrations/029_external_video_resources.sql`；相关测试 `video_links_test.go`、`video_provider_test.go`、`video_short_draft_test.go`。
- 安卓：`mobile/lib/video_links.dart`、`resources_experience.dart`、`app_sync.dart`、`app_upgrade.dart`、`main.dart`、`mobile/test/video_links_test.dart`；原生 `MainActivity.kt`、`VideoPlayerActivity.kt`、`AndroidManifest.xml`；独立调试包配置 `build.gradle.kts`、`src/debug/res/values/strings.xml`。
- 工具与文档：`scripts/start-video-dev.ps1`、`scripts/build-video-test-apk.ps1`、`docs/openapi.json`、本报告及验收证据。

带完整路径的机器可读清单在验收汇总的 `changedFiles` 中。

**数据库、启动与回退**

Schema 29 保留版本来源、外部 ID 索引、授权确认信息和检查缓存。历史本地资料、文件、公开版本不重写。前轮已执行的 Schema 29 不降级、不删除记录、不重写历史；其中删除标记、排序字段和清理任务表保留为空闲结构，本次没有接入对应业务。

本机完整数据副本、SQL备份和前轮改动备份在 `.local/douyin-only/`；含数据库资料和本机测试凭证，不放入 Git。原站业务数据核对结果见上述数据对照记录。此前已发生的本机默认库 Schema 29 迁移继续保留，本轮没有对原库企业、产品、首页或媒体资料做修改。

正常启动：在工作区运行 `./scripts/start-video-dev.ps1 -Background`。脚本校验隔离库名称和媒体路径，使用开发短信模拟，不发送真实短信；不要同时启动多个18086进程。

功能回退：先核对18086监听进程确为 `.local/douyin-only/server.exe` 后停止该副本，运行 `./scripts/start-video-dev.ps1 -DisableDouyin -Background`。这会禁用抖音入口及外链公开展示，保留数据和原本地功能；不要还原原库、降级 Schema 或直接用旧后端读取外链记录。服务端开关 `DOUYIN_VIDEO_ENABLED` 的默认值为 `false`。

独立测试包：`.local/douyin-only/artifacts/zqzg-video-dev-debug.apk`，包名 `com.zqzt.dalunongji.videodev`，名称“中企展馆视频测试”，连接模拟器宿主地址 `http://10.0.2.2:18086`。正式包、正式下载、版本号和签名未改动；校验信息见 [APK记录](qa/douyin-only-2026-11-01/apk-manifest.json)。

**截图**

- [电脑表单](qa/douyin-only-2026-11-01/editor-1440.png) / [手机表单](qa/douyin-only-2026-11-01/editor-390.png)
- [电脑抖音播放](qa/douyin-only-2026-11-01/douyin-desktop.png) / [手机模拟播放](qa/douyin-only-2026-11-01/douyin-mobile-emulation.png)
- 原首页 [电脑](qa/douyin-only-2026-11-01/home-8080-1440.png) / [手机](qa/douyin-only-2026-11-01/home-8080-390.png)；新预览 [电脑](qa/douyin-only-2026-11-01/home-18086-1440.png) / [手机](qa/douyin-only-2026-11-01/home-18086-390.png)
- 原地图 [电脑](qa/douyin-only-2026-11-01/map-original-1440.png) / [手机](qa/douyin-only-2026-11-01/map-original-390.png)；新预览 [电脑](qa/douyin-only-2026-11-01/map-new-1440.png) / [手机](qa/douyin-only-2026-11-01/map-new-390.png)
- [本地竖屏](qa/douyin-only-2026-11-01/local-portrait.png) / [PDF手机阅读](qa/douyin-only-2026-11-01/pdf-390.png)
