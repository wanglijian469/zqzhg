# 地域地标首页与展馆切换交付记录

版本：**1.5.18+26**。日期：2026 年 10 月 16 日（北京时间）。本轮在 1.5.17 首页升级基础上修改 Flutter App，未重建工程、未增加第三方依赖、未修改后端、数据库或网站代码。

## 首页与展馆行为

- 首页已取消农业机械、农机配件、液压油缸、齿轮、耕整地机械的独立图标网格及所有分类标签；真实分类筛选继续保留在厂家、产品列表。
- 使用后台 `/public/home` 的 `hero.media_id` 加载原始地标照片。本轮接口配置为媒体 123，对应“大陆村农机市场管理区”；代码没有写死该 ID、图片链接或厂商列表。
- 横幅保留图片真实色彩，按上部中心构图完整显示主招牌，文字位于下方局部深蓝渐变区域。中文常规字号高度约 220dp，英文和大字体按文字高度增高，避免压缩招牌展示区域。
- 地标图、宣传文案之后直接展示推荐厂家，缩短首页滚动距离。缺图、加载中或图片失败均以品牌渐变背景兜底，不影响搜索、推荐及其他功能。
- 顶部“中企展馆　农业机械馆⌄”为同一行，右侧保留语言及个人入口。英文、大字体可单行省略，菜单显示完整名称。
- 电线电缆馆可通过下拉菜单切换，菜单标注“筹备中”，首页、厂家、产品均显示筹备页，不显示农业图片、分类、厂家或产品，不发起该示例的业务查询。
- 切回农业机械馆后保留原搜索、分类、地区、页码及滚动位置；已在途的农业请求返回也不会出现在筹备馆页面。首次在筹备馆打开目录不会预先创建农业列表或发出列表查询。
- 展馆选择保留在当前会话中，冷启动默认农业机械馆。更换 API 地址时也回到农业机械馆，避免不同服务器间混用选择状态。
- “我的”、收藏、厂商账号与工作台属于全局功能，继续使用原账户、草稿及收藏体系。

## 修改文件

| 文件 | 内容 |
| --- | --- |
| `mobile/lib/exhibition_home.dart` | 移除分类组件，重排地标横幅，品牌及展馆同行下拉，复用首页响应中的宣传图 |
| `mobile/lib/exhibition_halls.dart`（新增） | 显式区分农业馆与电缆筹备示例，提供统一筹备页及返回入口 |
| `mobile/lib/main.dart` | 共享会话内展馆状态，切换服务器时重置，图片组件增加对齐及可配置加载/失败占位 |
| `mobile/lib/app_navigation.dart` | 筹备馆目录延迟创建，拦截业务导航，保留农业列表状态 |
| `mobile/lib/browse_screen.dart` | 筹备馆阻止查询、取消待执行搜索防抖，保留原分页筛选字段与滚动控制器 |
| `mobile/assets/i18n/zh.json`、`en.json` | 展馆菜单、筹备文案、返回操作和地标语义标签的中英文资源 |
| `mobile/pubspec.yaml` | 安装版本递增至 1.5.18+26 |
| `mobile/test/exhibition_halls_test.dart`（新增） | 23 项切馆、状态恢复、迟到响应隔离、地标刷新与适配验证 |
| `mobile/test/home_upgrade_test.dart` | 更新为无分类首页，保留原搜索、图片、卡片和目录回归 |
| `mobile/test/android_sync_test.dart` | 验证首页使用后台地标图，厂家卡片仍使用各自封面 |
| `mobile/test/home_upgrade_capture_test.dart` | 真实地标图片、展馆菜单、筹备页和大字体截图 |
| `scripts/verify-home-upgrade.ps1` | 增加地标原图核对和独立证据文件前缀，保留上一版报告 |
| `mobile/README.md`、本文、测试日志与截图 | 使用说明及交付记录 |

构建过程中同步了 Android 本地生成配置的版本号；本地缓存、原图及临时构建仓库仅用于开发验证，不作为 App 内置业务数据。

## 数据兼容

沿用原首页、厂家目录、产品目录、分类、媒体、展厅、认证、收藏与发布接口。农业首页首次仅请求一次 `/public/home`；分类在进入目录后按需读取并缓存。移除分类区后没有为地标新增一次重复首页请求。

本轮直接读取线上服务，核对 4 家推荐厂家、前 6 个精选产品及各自详情和展厅资料一致，并成功读取地标原图在内的 61 个公开媒体文件，见 [只读核对报告](test-output/landmark-home-live-data.json)。原图来自后台媒体资源，没有从用户上传的整屏截图中裁取。

电线电缆馆示例没有后端 `hall_id`，也不向原目录接口传入虚构分类或展馆参数。当前后端目录尚不支持按展馆查询；正式开放其他展馆时仍需实现按馆数据查询和地标配置。当前全局首页图片配置可用于运营人员按推广地区手动更换照片；App 刷新首页后获取新配置。

## 验证与截图

| 验证 | 结果 |
| --- | --- |
| Flutter 静态分析 | 无问题，[日志](test-output/landmark-home-analyze.txt) |
| 完整 Flutter 回归 | **105 项通过、4 项可选截图任务按开关跳过**，[日志](test-output/landmark-home-flutter-tests.txt) |
| 展馆及首页适配 | 360/390/412dp × 中英文 × 1.0/1.4/2.0 字体，农业首页、展馆菜单和筹备页均通过 |
| 原业务回归 | 厂家及产品分类筛选、分页、详情、展厅、收藏、登录退出、草稿、工作台、资源上传和发布相关测试通过 |
| 地标与切馆 | 验证真实媒体 ID、原图加载、上部对齐、失败降级、后台换图刷新、筹备馆不请求业务数据、迟到响应不泄漏、切回恢复状态 |
| 真实内容截图 | 已生成三种宽度首页、展馆菜单、筹备馆、厂家及产品页面，另含中英文大字体，[日志](test-output/landmark-home-capture.txt) |
| Android 设备 | 当前 ADB 无连接设备，[记录](test-output/landmark-home-devices.txt) |

主要截图：

- [新版地标首页](screenshots/home-upgrade-landmark-390-home.png)
- [展馆下拉菜单](screenshots/home-upgrade-landmark-390-hall-menu.png)
- [电线电缆馆筹备页](screenshots/home-upgrade-landmark-390-cables-home.png)
- [筹备馆厂家入口](screenshots/home-upgrade-landmark-390-cables-manufacturers.png)
- [厂家目录及保留的分类筛选](screenshots/home-upgrade-landmark-390-manufacturers.png)
- [英文大字体首页](screenshots/home-upgrade-landmark-390-english-home-large.png)
- [中文最大测试字号](screenshots/home-upgrade-landmark-390-chinese-home-largest.png)
- [上一版首页对比](screenshots/home-upgrade-live-390-home.png)

截图由 Flutter 使用本轮真实公开资料渲染，不等同于 Android 真机截图。可选截图任务本轮已单独执行成功，未将历史测试当作本轮通过证据。

## 安装包与验证边界

Debug、Release 均构建成功，已核对包名、版本、架构及签名。本轮仅交付本地版本化安装包，保留 1.5.17+25 及网站现有下载文件。

| 安装包 | 大小 | 签名 |
| --- | --- | --- |
| [Release APK](../delivery/android/dalunongji-1.5.18+26.apk) | 80,442,373 字节（约 76.7 MiB） | 原生产签名 |
| [Debug APK](../delivery/android/dalunongji-1.5.18+26-debug.apk) | 197,665,525 字节（约 188.5 MiB） | 开发签名 |

构建、签名、版本及 SHA-256 结果见 [交付清单](../delivery/manifest-landmark-home-1.5.18.json) 与 [SHA-256 校验文件](../delivery/android/SHA256SUMS-1.5.18+26.txt)。验证记录：[Android 构建](test-output/landmark-home-android-build.txt)、[Release 签名](test-output/landmark-home-release-signature.txt)、[Release 包信息](test-output/landmark-home-release-metadata.txt)、[Debug 签名](test-output/landmark-home-debug-signature.txt)、[Debug 包信息](test-output/landmark-home-debug-metadata.txt)。

Release 使用原生产证书，证书 SHA-256 为：

```text
85656c85541b188c832dc48d67cbf320046a4311aa9e0fefcc662dbe2e0f1585
```

包名沿用 `com.zqzt.dalunongji`，版本名 1.5.18、版本码 26。Debug 使用开发签名，不能直接覆盖原 Release 安装；正式覆盖升级使用原签名 Release 包并保留原数据。

尚未完成真机启动、安装与覆盖升级、系统栏外观、拨号器及真实账号收藏同步的端到端实测。电线电缆馆是筹备示例，真实多馆业务开放与按地区自动切图需要后续后端支持。本轮未向生产服务发布测试厂家、产品或账号，也未进行网站与 App 的真实双端联动发布。
