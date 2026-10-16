# 中企展馆首页及展示体验升级交付记录

需求名称：V1.3 首页及展示体验升级。Android 版本：**1.5.17+25**。验证日期：2026 年 10 月 16 日（北京时间）。

本轮基于原 Flutter 工程实施，沿用包名 `com.zqzt.dalunongji`、原认证及后端接口。网站源代码、生产数据库和网站下载副本均未修改。未增加购物车、支付、订单、扫码或采购需求发布。

## 1. 修改文件

| 文件 | 修改内容 |
| --- | --- |
| `mobile/lib/exhibition_home.dart`（新增） | 首页、品牌栏、搜索、横幅、动态分类、推荐模块、入驻卡、局部错误及刷新 |
| `mobile/lib/browse_screen.dart`（新增） | 拆出原厂家及产品目录，懒构建、防抖、分页、刷新及请求去重 |
| `mobile/lib/exhibit_card.dart`（新增） | 横向厂家卡、方形产品卡、双列 Sliver、图片优先级、规格和地区展示 |
| `mobile/lib/main.dart` | 注册拆分组件、公共分类缓存、图片降级、精简原卡片及目录实现、公共导航栏 |
| `mobile/lib/industrial_ui.dart` | 移出首页，统一配色、模块标题和产品网格列数 |
| `mobile/lib/app_sync.dart` | 移出旧纵向厂家卡，继续复用展厅、发布及资料管理逻辑 |
| `mobile/lib/app_navigation.dart` | 四栏导航、“我的”、厂商登录续接、首次带筛选导航、设置入口 |
| `mobile/lib/favorites.dart` | 增加纯图标收藏模式，保留原本地及云同步机制 |
| `mobile/assets/i18n/zh.json`、`en.json` | 新增及调整首页、导航、搜索、入驻、设置的中英文文案 |
| `mobile/pubspec.yaml` | 版本递增为 1.5.17+25，无新增依赖 |
| `mobile/test/home_upgrade_test.dart`（新增） | 26 项首页、目录、图片、请求和适配测试 |
| `mobile/test/home_upgrade_capture_test.dart`（新增） | 同样例前后截图及本轮真实接口资料截图 |
| `mobile/test/app_navigation_test.dart` | 按新导航路径回归状态保持、登录退出、草稿及工作台 |
| `mobile/test/favorites_test.dart` | 验证收藏迁入“我的”后仍联动、持久化和同步 |
| `mobile/test/android_sync_test.dart` | 更新首页不再展示市场大图的断言，保留原上传及发布验证 |
| `mobile/test/industrial_ui_test.dart` | 更新产品导航文案断言 |
| `scripts/verify-home-upgrade.ps1`（新增） | 只读核对真实首页、详情、展厅及图片，生成证据 |
| `mobile/README.md` | 更新版本、导航、构建与验收说明 |
| 本文及 `docs/test-output/home-upgrade-*`、`docs/screenshots/home-upgrade-*` | 交付说明、日志、数据核对及截图 |

Android 构建生成的本地配置、缓存、临时 Maven 仓库和接口快照不属于业务实现，也未作为 App 的内置展示数据。

## 2. 组件与主要变化

`AppShell` 使用延迟初始化和 `IndexedStack` 保存首页、厂家、产品、我的四个页面。厂家和产品目录保留各自关键词、分类、地区、页码及滚动位置；详情返回后继续原位置。

首页由 `HomeBrandBar`、`HomeSearchField`、`HomeBrandBanner`、`HomeCategoryGrid`、推荐厂家 Sliver、`ExhibitSliver`、`HomeJoinCard` 和平台说明组成。搜索、横幅不依赖业务请求成功；分类或推荐加载失败可单独重试。下拉刷新重新获取当前已发布资料。

| 优化前 | 优化后 |
| --- | --- |
| 搜索位于长横幅中，另有大幅市场照片 | 搜索紧接品牌栏；常规字体横幅约 164dp，移除市场大图 |
| 首页有多个相同业务方向入口 | 保留一个搜索入口、真实分类及模块“查看全部” |
| 分类按固定 ID 展示为纵向大卡 | 从公共分类读取真实 ID，四列排列，最多 8 项；展馆 slug 可作为组件参数复用 |
| 厂家大图纵排 | 104dp 图片、两行企业名称和主营、紧凑地区、展厅及联系按钮、右上角收藏 |
| 产品图 4:3，文字高度不一致 | 1:1 `contain` 图片、统一文字区域、两列对齐；大字体时回退单列 |
| 底栏无首页，收藏和展位直接占两栏 | 首页 / 厂家 / 产品 / 我的；收藏及展位从“我的”进入 |
| 分类多处请求、搜索仅提交触发 | 公共分类会话内共享；目录 350ms 防抖，相同条件不重复请求 |

厂家名称、图片和展厅按钮进入企业独立展厅；联系按钮打开现有展厅联系页。收藏点击不触发展厅或产品详情导航。英文或大字体时厂家操作按钮改为上下排列，保持单行文案。底栏包含 Android 底部安全区域。

## 3. API 与真实数据兼容

默认服务为 `http://39.97.250.244:8089`，继续保留用户已有自定义 API 地址。认证继续使用原 Bearer token，收藏、账号、草稿与发布流程不变。

| 用途 | 现有接口 |
| --- | --- |
| 首页推荐 | `GET /api/v1/public/home` |
| 展馆、分类和地区 | `GET /api/v1/public/taxonomy` |
| 厂家目录及筛选 | `GET /api/v1/public/vendors` |
| 产品目录及筛选 | `GET /api/v1/public/products` |
| 企业品牌展厅 | `GET /api/v1/public/showrooms/{slug}` |
| 产品详情 | `GET /api/v1/public/products/{id}` |
| 缩略图及媒体 | `GET /api/v1/media/{id}?thumb=1` |

没有新增或修改接口、数据库字段、认证协议或后端业务代码。首页沿用后台排序，仅展示前 6 家厂家和前 6 个产品；当前服务实际返回 4 家推荐厂家及 8 个精选产品，因此 App 展示 4 家及 6 个。

本轮直接读取线上服务后确认：

- 4 家推荐厂家分别为宁晋县宇浩农机厂、邢台市威力达齿轮制造有限公司、河北丹顿液压科技有限公司、河北冀农农机具有限公司；首页、厂家详情和独立展厅资料一致。
- 前 6 个精选产品的名称、参数和图片字段与各自详情一致，所属厂家联系方式可用；包含需求中列举的 1LF-440、3 米驱动耙、1LF-170、1LF-230。
- 后台公开分类为农业机械、农机配件、耕整地机械、翻转犁、驱动耙、液压油缸、齿轮；首页按根分类及直接子分类展示真实入口，没有虚构“传动部件”“液压系统”等分类 ID。
- 成功读取 60 个公开媒体文件，供本轮界面验证使用。测试快照与图片仅在 `.local`，不会打包成为正式业务数据。

完整只读证据见 [线上字段核对](test-output/home-upgrade-live-data.json) 和 [服务连通性](test-output/home-upgrade-live-endpoints.json)。先前经默认访问路径得到的 502 已通过直接连接排除，最终线上读取成功。

图片继续复用 `Photo` 的异步加载和 Flutter `ImageCache`。厂家依次尝试封面、原企业媒体封面回退、当前已返回的关联产品图、Logo、统一占位。不会为每个卡片追加展厅或产品详情请求。

## 4. 已完成验证

| 检查 | 结果与证据 |
| --- | --- |
| 修改前静态基线 | 通过，[日志](test-output/home-upgrade-before-analyze.txt) |
| 最终 Flutter 静态分析 | 无问题，[日志](test-output/home-upgrade-analyze.txt) |
| 完整 Flutter 回归 | **82 项通过、4 项按开关跳过**；跳过项为可选截图任务，[日志](test-output/home-upgrade-flutter-tests.txt) |
| 最终首页专项回归 | **26 项通过**，[日志](test-output/home-upgrade-feature-tests.txt) |
| 后端隔离回归 | `RUN_MYSQL_TESTS=1 go test ./... -count=1 -timeout=180s` 通过，[日志](test-output/home-upgrade-backend-tests.txt) |
| 屏幕与字体 | 360、390、412dp × 中英文 × 1.0、1.4、2.0 字体，共 18 组 Widget 适配检查通过 |
| 请求与导航 | 首页两次公共请求、首次筛选导航单次请求、防抖去重、切栏及切语言不重复加载、分页与位置保持通过 |
| 异常与图片 | 网络失败、局部重试、空数据、下拉刷新、缺图占位、封面/产品/Logo 降级、方形比例及收藏独立点击通过 |
| 原业务 | 收藏离线队列、账号隔离、登录合并、退出、草稿、工作台、资源上传与发布相关 Flutter/后端回归通过 |
| 线上一致性 | 4 家厂家及前 6 个精选产品字段核对通过，真实公开媒体读取成功 |
| 真实内容截图 | 三种屏宽首页、厂家、产品及推荐区，中英文大字体截图已生成，[截图日志](test-output/home-upgrade-live-capture.txt) |

发布与审核验证使用本地隔离测试库，测试库由既有测试创建和清理；没有向生产服务提交测试厂家、产品或账号。网站和 App 均沿用同一发布、审核及公共读取接口；本轮没有执行生产环境双端联动发布。

## 5. 界面证据

以下前后截图使用相同测试样例，仅用于比较布局，不是正式业务数据：

| 页面 | 修改前 | 修改后 |
| --- | --- | --- |
| 首页 390dp | [原首页](screenshots/home-upgrade-before-390-home.png) | [新首页](screenshots/home-upgrade-after-390-home.png) |
| 厂家目录 390dp | [原厂家卡片](screenshots/home-upgrade-before-390-manufacturers.png) | [新厂家卡片](screenshots/home-upgrade-after-390-manufacturers.png) |

本轮线上真实资料渲染：[首页](screenshots/home-upgrade-live-390-home.png) · [厂家目录](screenshots/home-upgrade-live-390-manufacturers.png) · [产品目录](screenshots/home-upgrade-live-390-products.png) · [英文大字体](screenshots/home-upgrade-live-390-english-large.png)。正常字号厂家目录首屏可见两家完整厂家及第三家部分卡片。截图为 Flutter 渲染证据，不能替代 Android 真机验收。

## 6. Android 打包与安装

本地安装包、校验值及最终打包结果记录在 `delivery/manifest-home-upgrade-1.5.17.json` 和 `delivery/android/SHA256SUMS-1.5.17+25.txt`。本轮仅增加带版本号的新文件，不替换现有网站下载包。

| 安装包 | 构建结果 | 大小 |
| --- | --- | --- |
| [原签名 Release APK](../delivery/android/dalunongji-1.5.17+25.apk) | 成功，最终源码构建，原证书校验通过 | 80,442,101 字节 |
| [Debug APK](../delivery/android/dalunongji-1.5.17+25-debug.apk) | 成功，开发签名 | 197,663,325 字节 |

[SHA-256 校验文件](../delivery/android/SHA256SUMS-1.5.17+25.txt) · [完整打包清单](../delivery/manifest-home-upgrade-1.5.17.json)。

Release 保持原包名、版本号 1.5.17、版本码 25，最低 Android 7.0（API 24），目标 API 36，包含 `armeabi-v7a`、`arm64-v8a`、`x86_64`。原 Release 签名证书 SHA-256：

```text
85656c85541b188c832dc48d67cbf320046a4311aa9e0fefcc662dbe2e0f1585
```

构建记录：[Release](test-output/home-upgrade-release-build.txt) · [Debug 最终构建](test-output/home-upgrade-debug-build-final.txt) · [Release 签名](test-output/home-upgrade-release-signature.txt) · [Release 元数据](test-output/home-upgrade-release-metadata.txt)。Debug 使用开发签名，不能直接覆盖原 Release 安装。正式覆盖升级使用原签名 Release 包，并保留原数据。

环境排障：本地 Dart/ADB 需在获准的宿主环境运行；旧 Gradle 进程需重启。Debug ARM64 引擎下载停滞，使用 Flutter 官方存储下载同版本文件并验证官方内容哈希，在 `.local` 临时 Maven 仓库完成构建；没有修改项目依赖或工程仓库配置。后端视频测试使用项目临时目录后通过。项目已有 Gradle/Kotlin 弃用提示保留，本轮未升级构建依赖。

## 7. 验证边界与后续支持

- 当前没有连接 Android 设备或已配置模拟器，未完成真机启动、系统栏外观、安装与覆盖升级、拨号器和微信剪贴板的端到端实测；[ADB 记录](test-output/home-upgrade-devices.txt)为空设备列表。
- 网站及 App 双向发布的真实客户端联动仍需在验收环境操作；本轮已完成原发布流程回归、隔离后端验证和线上公开字段核对。
- 未使用真实账号实测生产登录和收藏云同步。若生产服务尚未部署配套收藏接口，App 延续原有本机收藏与升级提示行为，不把本地测试通过等同于生产云同步通过。
- 后端没有独立型号字段，卡片只读取参数中明确的“型号 / 规格 / Model / Specification”行，缺失时不补造。搜索继续使用现有名称、英文名称、关键词和主营产品匹配；只写在参数里的型号不保证能被现有搜索接口检索。这需要后续后端支持，未擅自扩大查询规则。
- 主营产品、部分地区和产品参数没有独立英文字段；新增界面文案均可切换语言，业务内容按原规则使用英文或回退中文。完整业务资料翻译需要厂家完善数据或后端支持。
- 当前首页接口固定返回最多 12 家厂家、8 个产品。App 将展示限制为各 6 条；进一步缩小服务端响应需后台提供可选数量参数，本轮未修改接口。
