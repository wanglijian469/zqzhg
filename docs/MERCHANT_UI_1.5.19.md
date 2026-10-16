# 厂商管理端 UI/UX 升级交付记录

交付日期：2026 年 10 月 16 日。App **1.5.19+27**，网站与后端交付标记 **1.5.19**。

基于原 Flutter、React、Go 工程实施，没有重建项目，没有增加依赖、数据库迁移、删除接口或直接上架接口。普通用户浏览页保留布局，相关共享展示文案统一为“展厅”。本轮未部署生产网站、后端或 APK 下载文件，也未修改生产账号和业务数据。

## 已完成的三个阶段

### 第一阶段：导航、工作台、产品管理

- App 移除厂商工作台顶部横向四标签，改为工作台、展厅管理、产品管理及推广独立页面，复用原编辑器和登录入驻流程。
- “我的”显示当前账号真实企业摘要、Logo、两行名称、审核与展示状态，突出“我的企业展厅”；未入驻显示免费入驻，游客草稿及访客转厂商登录继续保留。
- 退出登录集中在设置，App 同时保留原服务器地址配置、退出确认及收藏同步提示；网站增加 `/merchant/settings`，保留桌面侧栏、手机抽屉、旧入驻链接兼容。
- 工作台显示企业、地区、三个紧凑统计及四个常用操作：编辑展厅、添加产品、管理产品、分享企业展厅。统计来自全账号接口，不能将各项相加当作总数。
- 必须补充项目复用既有表单校验；建议完善 Logo、封面、简介及实拍不增加审核门槛。已有介绍不会因宣传简介为空而被误报缺失。
- 产品管理采用 92dp／px 方形缩略图、两行名称、两行已有参数摘要、小状态标签、编辑和更多菜单。添加入口位于标题区，大字体使用带完整语义提示的图标操作；精选产品设置移到列表下方。
- 名称／型号搜索使用 350ms 防抖，提交立即执行，保留 20 条分页、组合状态筛选、编辑返回条件与浏览位置。迟到响应不能覆盖当前筛选或其他账号数据。
- 产品菜单保留草稿预览、审核记录和可用的公开预览、分享、下架。下架二次确认；重新展示继续从编辑页提交审核；不显示无后端支持的删除操作。

### 第二阶段：展厅管理、术语和分享

- 展厅管理采用“企业资料、展示素材、预览审核”分组。基本信息、介绍、联系方式、Logo／封面、实拍及资质入口定位到原完整表单，视频和画册沿用原资源管理器。
- 保留本地／云端草稿、保存反馈、上传进度及重试、审核原因、版本冲突校验和待审只读。修改审核中，原已展示资料仍由后端原规则决定是否公开。
- 全面统一指定中英文术语；审核通过和实际展示分别显示。主动下架终止审核的记录显示“审核已终止（下架）”，不会冒充一次审核驳回。
- 展厅推广直接显示分享管理页。预览卡、简介、海报只取公开接口数据；Logo 为零值时可正确使用真实封面，缺图继续降级。
- 一个主分享操作、复制链接／二维码／保存海报三个辅助操作，文案和渠道指引折叠。复用原 Android 分享与文件保存通道、网站分享及二维码能力。
- 产品分享先读取公开产品及所属展厅的规范地址，以公开名称和配置域名生成真实产品路由，不使用待审名称或管理页筛选参数。
- 分享取消不报成功，系统分享不可用时提供真实链接及复制／保存替代。二维码、海报生成或保存失败可重试。

### 第三阶段：适配与回归

- App 检查 360／390／430dp、中英文、1.0／1.4／2.0 字体组合，涵盖工作台、产品、管理及推广。
- 网站实际浏览器检查 360／390／430／1440px，共 80 个页面组合；手机字号用逐元素放大模拟，另验证 URL 条件、编辑返回滚动位置及公开分享地址。
- 修复既有公共重试组件将 Future 返回给 `setState` 的问题、大字体分页挤出、原退出按钮宽度遗留造成语言按钮裁切，以及大字号分享文字越界。
- 请求与缓存按服务器、账号和会话隔离；退出／切换账号失效。组件诊断标识不包含原始登录令牌。

## 组件及数据结构

App 的 `AppModel` 继续作为共享状态源。新增 `MerchantDataStore` 复用企业及统计请求，失败会清除对应 Future 以允许重试；刷新和保存返回时失效。工作台、企业摘要、状态、小分组列表独立于产品分页组件。原 `EditorScreen`、资源编辑器、API Client、`Photo`、草稿和认证机制继续使用。

网站保留厂商路由壳，企业及统计在壳内共享。产品筛选存入 URL，滚动位置按账号及筛选保存在会话存储中。通用加载钩子检查请求身份和当前令牌，防止切换后显示旧响应。推广复用可直接嵌入页面的分享组件。

后端唯一业务变化是 `GET /api/v1/merchant/contents?kind=product&q=...` 同时检索当前草稿载荷的 `parameters`。保留原参数、响应、分页、状态和所有者限制；不扫描整段 JSON，不改变公开或管理员搜索。数据库仍只使用原字段，查询参数继续绑定及转义。

## 修改文件清单

| 分组 | 文件 |
| --- | --- |
| App 导航、账号及状态 | `mobile/lib/main.dart`、`app_navigation.dart`、`favorites.dart` |
| App 厂商组件 | 新增 `mobile/lib/merchant_workspace.dart`、`merchant_products.dart`；调整 `merchant_experience.dart`、`app_sync.dart` |
| App 资源及版本 | `mobile/assets/i18n/zh.json`、`en.json`、`mobile/pubspec.yaml`、`mobile/README.md` |
| App 回归 | 新增 `mobile/test/merchant_ui_test.dart`；更新 `android_sync_test.dart`、`app_navigation_test.dart`、`favorites_test.dart`、`merchant_experience_test.dart` |
| 网站管理和分享 | `web/src/merchant-console.tsx`、`merchant-workspace.css`、`company-profile.tsx`、`showroom-share.ts`、`main.tsx`，新增 `merchant-messages.ts` |
| 网站术语 | `i18n.ts`、`ui-messages.ts`、`experience-messages.ts`、`showroom-messages.ts`、`resource-messages.ts`、`admin-messages.ts`、`form-guide.tsx`、`merchant-auth.tsx` |
| 网站回归及版本 | 新增 `web/src/merchant-ui.test.tsx`；更新 `app`、`category-picker`、`company-portal`、`experience`、`home-display`、`merchant-registration`、`merchant-workspace`、`showroom`、`vendor-list` 对应测试；`web/package.json`、`package-lock.json` |
| 后端 | `server/content.go`、`server/merchant_dashboard_test.go` |
| 文档及验证 | `README.md`、`docs/openapi.json`、本文、新增 `scripts/verify-merchant-ui-browser.cjs`、本轮测试日志与截图 |

本地 Android 生成配置同步为版本名 1.5.19、版本码 27。未改变签名配置，密钥不包含在交付包内。

## 验证结果

| 检查 | 结果及本轮证据 |
| --- | --- |
| Flutter 静态分析 | 无问题，[日志](test-output/merchant-ui-analyze.txt) |
| Flutter 全量测试 | **130 项通过，5 项可选截图按开关跳过**，[日志](test-output/merchant-ui-flutter-tests.txt)；本轮截图任务已另行执行成功 |
| App 截图 | 单独执行成功，[日志](test-output/merchant-ui-capture.txt)；截图使用明确标记的测试数据 |
| 网站全量测试 | **14 个文件、79 项通过**，[日志](test-output/merchant-ui-web-tests.txt) |
| 网站类型检查及生产构建 | 成功，[构建日志](test-output/merchant-ui-web-build.txt)；存在原 PDF／主包分块体积提示，不影响构建 |
| 浏览器适配 | **80 个组合通过**，无页面横向溢出，编辑返回条件和实际浏览位置恢复通过，[报告](test-output/merchant-ui-browser.txt) |
| 后端完整测试 | 使用自动创建并清理的隔离 MySQL 测试库，全部通过，[日志](test-output/merchant-ui-go-tests.txt) |
| 型号与权限专项 | 中文规格、特殊字符、分页、跨账号、未展示过滤和管理员搜索兼容通过，[日志](test-output/merchant-ui-stage1-backend.txt) |
| Linux amd64 后端构建 | 成功，[日志](test-output/merchant-ui-backend-build.txt) |
| Android Debug／Release | **两种构建均成功**，[构建日志](test-output/merchant-ui-android-build.txt)；版本及原生产签名已核对 |

测试覆盖原登录退出、入驻、草稿恢复、上传重试、审核、版本锁、分页、收藏、精选设置、资源及公开浏览。生产真实账号收藏同步、生产发布和双端人工联动没有执行；测试接口样例仅供验收，未作为正式业务数据内置到 App 或网站。

## 新旧界面预览

- App：[工作台](screenshots/merchant-ui-app-390-dashboard.png)、[我的](screenshots/merchant-ui-app-390-mine.png)、[产品管理](screenshots/merchant-ui-app-390-products.png)、[展厅管理](screenshots/merchant-ui-app-390-showroom.png)、[推广](screenshots/merchant-ui-app-390-promotion.png)。同目录提供 360、430dp 版本。
- 网站：[工作台](screenshots/merchant-ui-web-390-zh-dashboard.png)、[产品管理](screenshots/merchant-ui-web-390-zh-products.png)、[展厅管理](screenshots/merchant-ui-web-390-zh-showroom.png)、[推广](screenshots/merchant-ui-web-390-zh-promotion.png)、[英文产品](screenshots/merchant-ui-web-390-en-products.png)、[桌面工作台](screenshots/merchant-ui-web-1440-zh-dashboard.png)。
- 旧版参照：[App 展厅管理](screenshots/app-navigation-booth.png)、[网站工作台](screenshots/merchant-workspace-dashboard-390.png)、[网站产品管理](screenshots/merchant-workspace-products.png)。旧图仅用于对比，不作为本轮通过证据。

新版由多块大统计、横向管理标签和独立分享中转，调整为紧凑摘要、明确操作入口、单列产品与直接分享。截图由 Flutter 和真实浏览器渲染，均标记“测试数据 · UI 验收”，不是 Android 真机截图。

## 安装包及运行边界

- [Release APK](../delivery/android/dalunongji-1.5.19+27.apk)：80,541,485 字节（约 76.8 MiB），沿用原生产签名，正式覆盖升级应使用此包。包名 `com.zqzt.dalunongji`，版本 1.5.19（27），最低 API 24、目标 API 36，支持 ARMv7、ARM64 和 x86_64。
- [Debug APK](../delivery/android/dalunongji-1.5.19+27-debug.apk)：235,245,238 字节（约 224.3 MiB），开发签名，不能直接覆盖原 Release，验收时使用独立设备或测试环境。
- [SHA-256](../delivery/android/SHA256SUMS-1.5.19+27.txt)、[交付清单与签名指纹](../delivery/manifest-merchant-ui-1.5.19.json)。
- [网站静态构建包](../delivery/merchant-ui-1.5.19/web-dist.zip)、[Linux amd64 后端](../delivery/merchant-ui-1.5.19/exhibition-linux-amd64)：仅本地交付，不自动发布。网站包不包含旧 APK 下载副本，后续部署应保留既有 `/downloads` 文件。

签名及包信息证据：[Release 签名](test-output/merchant-ui-release-signature.txt)、[Release 元数据](test-output/merchant-ui-release-metadata.txt)、[Debug 签名](test-output/merchant-ui-debug-signature.txt)、[Debug 元数据](test-output/merchant-ui-debug-metadata.txt)。Linux 二进制已交叉编译，未在生产服务器启动。

正式 HTTPS 域名尚未提供。当前 IP／HTTP 地址仅作真实测试链接；正式分享验收仍需配置现有 `PUBLIC_BASE_URL`、有效证书和可访问业务域名，再验证 `/v/:slug`、`/products/:id` 及二维码。产品型号搜索的线上生效需要部署本轮后端，旧服务不保证匹配仅写在规格中的型号。

删除和直接重新上架不在现有后端能力范围内，本轮不提供伪按钮或绕过审核。没有新增第三方社交 SDK；能否直接发送到特定应用由系统和已安装应用决定。

## Android 验证步骤

1. 在测试手机保留旧 Release 和本机草稿，安装本轮原签名 Release，核对版本 1.5.19（27）。若用 Debug，使用独立测试环境，不卸载正式 App 以免清除资料。
2. 检查农业机械首页、展馆切换、厂家／产品、收藏及中文／英文；确认普通用户浏览不受影响。
3. 使用测试厂商账号登录，从“我的 → 我的企业展厅”进入工作台，核对名称、地区及统计与服务端一致。退出入口只在设置，取消退出不得清理会话或草稿。
4. 编辑展厅各分组，保存草稿、退出再进入继续编辑。待审资料只读；审核中的修改不能替换原公开版本。模拟上传失败，确认已上传图片不重复上传、失败文件可重试。
5. 产品名称／型号查询、状态组合筛选及翻页后进入编辑，再返回核对条件、页码及位置。已展示且待审的产品应同时表达两个状态。
6. 分享前核对公开企业／产品版本，复制真实链接并在另一浏览器访问；扫描二维码，保存海报，打开系统分享；取消分享或保存不应误报成功。未展示内容不得生成公开宣传材料。
7. 在 360／390／430dp 设备或模拟器及大字体下检查键盘、状态栏、系统导航栏、安全区域、返回行为和按钮可点击性。
8. 切换厂商账号或服务器，确认不出现上个账号资料；校验后端拒绝修改他人的企业、产品和媒体。

当前本机没有连接 Android 设备，ADB 检查见 [设备记录](test-output/merchant-ui-devices.txt)。真机安装启动、覆盖升级、系统栏外观、拨号／外部应用和原生分享面板仍需按以上步骤实测；本轮不把 Widget、浏览器截图或历史设备记录当作真机通过证据。
