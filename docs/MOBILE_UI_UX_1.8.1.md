# 大陆农机移动端 UI/UX 优化与验收

日期：2026-10-24。Android 验收版本：**1.8.1+36**。本轮基于工作区已有改动增量实施，未修改数据库结构，未部署生产，未替换正式下载。

## 实现方式与主要变化

| 页面 | 实际实现 | 优化前 | 本次结果 |
| --- | --- | --- | --- |
| 产品、厂家列表 | Flutter 原生 | 悬浮标签与边框存在重叠风险；筛选展开占屏 | 外置标签；分类、地区、排序一行按钮；底部选择抽屉 |
| 产品列表卡片 | Flutter 原生 | 双列大图；收藏叠在图片上 | 横向紧凑卡片；图片与名称、型号、厂家、地区分区；收藏在文字区右上角 |
| 厂家卡片 | Flutter 原生 | 图片和空白偏大；长名称省略 | 缩小图片与留白；名称自然换行；保留收藏、进入展厅、联系厂家 |
| 采购中心 | Android WebView 加载 APK 内置 React | 原生标题栏叠加网页工作台顶栏、侧栏；大型筛选表单 | 原生单标题栏；大厅先展示搜索、发布/我的询价/消息入口、筛选按钮和需求卡片 |
| 发布需求 | React，沿用原业务接口 | 所有信息连续展开 | 必填信息保持可见；规格、期望交期、附件折叠到更多信息；保留草稿及校验 |
| 厂商展厅 | Flutter 原生及 React 网页 | 首页大头图与其他栏目的紧凑头部互换；切换引发跳动 | 六栏目持续使用相同企业头部；稳定外层滚动容器；已访问栏目保留状态 |

移动端底部导航仍为首页、厂家、产品、我的。首页新增采购入口，登录与未登录均先进入大厅；采购商资料移到原生“我的”。桌面采购工作台保留原有侧栏与功能。

表单统一采用标签在上、控件在下的布局，保留原控制器、焦点、报错、提示、输入格式和校验。原生源码已移除 `labelText` 悬浮标签调用，未通过隐藏标签掩盖问题。

## 主要代码文件

| 文件/组件 | 本轮修改 |
| --- | --- |
| `mobile/lib/mobile_fields.dart`（新增） | 外置标签组件、可滚动且支持安全区域的筛选抽屉 |
| `mobile/lib/browse_screen.dart`、`exhibit_card.dart` | 紧凑筛选、重置、列表卡片；保留查询、分页与返回状态 |
| `mobile/lib/resources_experience.dart` | 展厅头部持续挂载；产品、视频、画册按需创建并保留；外层滚动 key 不再随栏目变化 |
| `mobile/lib/main.dart`、`onboarding.dart`、`taxonomy.dart`、认证/企业/资源编辑组件 | 外置标签迁移及统一输入样式；不改变字段校验 |
| `mobile/lib/exhibition_home.dart`、`app_navigation.dart`、`procurement_workspace.dart` | 统一基础导航高度；首页采购与个人资料入口；公开摘要路径校验 |
| `mobile/android/app/src/main/kotlin/com/zqzt/dalunongji/WorkspaceActivity.kt` | 单原生标题栏、语言与页面标题同步；键盘/弹层/历史/退出的返回顺序；保留未保存保护 |
| `mobile/android/app/src/main/assets/native-workspace.js` | 明确内嵌运行标记；沿用原会话初始化及私密附件桥接 |
| `web/src/procurement-hall.tsx`（新增） | 移动大厅、筛选抽屉、三级地区、公开摘要详情；空结果、重试及滚动恢复 |
| `web/src/procurement-workspace.tsx`、`procurement-mobile.css`、`mobile-workspace.ts` | 手机与内嵌工作台外壳、弹层返回栈及样式隔离；修复全局 `nav` 样式对快捷入口的遮挡 |
| `web/src/procurement.tsx`、`procurement-links.tsx`、`main.tsx` | 发布页信息折叠、共用地区选择器、手机大厅入口、原生语言切换与页面滚动协调 |
| `web/src/showroom.tsx` | 相同企业头图和 DOM 节点持续保留；取消栏目切换的滚动归零；保留产品及资源栏目状态 |
| `server/procurement.go`、`procurement_demo.go`、`spa_routes.go` | 兼容扩展公开查询、摘要详情、能力标记和 SPA 路由 |
| 原生/Web/API 测试与 `scripts/verify-mobile-ui.cjs` | 新布局、真实点击、权限、异常及尺寸矩阵；旧测试改为按外置标签查找输入框 |

## 接口兼容与数据边界

- `GET /api/v1/public/procurements` 保留原分页和筛选参数；`q` 同时匹配项目标题及当前发布版本的产品名称，不搜索私密备注或规格。
- 列表增加可空的 `published_at` 及分类中英文名称。时间取当前版本的公开审核通过记录；历史记录缺失时返回 `null`，界面不伪造时间。
- 新增 `GET /api/v1/public/procurements/:id`，只读返回与大厅相同的公开摘要。待审核、非公开、暂停、过期和不可公开的项目均不提供摘要。
- 公开摘要不含联系人、电话、私密技术规格、附件或报价。点击卡片只进入摘要，不创建参与记录；“参与并查看完整需求”继续调用原有登录、供货资格和参与流程。
- 新增公开能力标记 `procurement_public_summary`。旧采购服务缺少摘要接口时，可以显示刚选中的公开列表摘要并明确提示服务需升级；完整参与仍由服务端校验。没有采购能力的服务器继续显示原来的配套升级提示。
- 地区按服务器返回的行政区划层级恢复，包含直辖市、省直辖县级市；改变上级清空下级，不按六位代码一律推断为区县。
- 搜索、筛选和页码保留在 URL；采购列表滚动记录按服务器、账号及 URL 隔离。原生列表沿用展馆/服务器的原状态隔离。

## 截图与关键对照

采购“优化前”来自本次首次重打包之前保留的旧内置网页，同一组合成布局数据。优化后采购图是内置页面在浏览器中的验证结果；产品、厂家图来自 Flutter widget 渲染。均不标称 Android 真机截图。

| 页面 | 优化前 | 优化后 |
| --- | --- | --- |
| 采购大厅 | [大型筛选表单](screenshots/mobile-ui-20261024/before-procurement-390-zh.png) | [搜索、快捷入口、筛选及需求列表](screenshots/mobile-ui-20261024/after-procurement-390-zh.png) |
| 展厅首页与介绍 | [旧首页](screenshots/mobile-ui-20261024/before-showroom-home-390-zh.png)、[旧介绍](screenshots/mobile-ui-20261024/before-showroom-about-390-zh.png) | [首页](screenshots/mobile-ui-20261024/after-showroom-home-390-zh.png)、[介绍](screenshots/mobile-ui-20261024/after-showroom-about-390-zh.png)，头部持续保留 |
| 产品与厂家 | 旧代码布局为对照，未伪造旧包真机图 | [产品](screenshots/mobile-ui-20261024/product-390-zh.png)、[厂家英文长名称](screenshots/mobile-ui-20261024/vendor-390-en.png) |
| 复杂筛选与异常 | 原表单默认占据页面主体 | [英文底部筛选](screenshots/mobile-ui-20261024/after-filters-390-en.png)、[空结果](screenshots/mobile-ui-20261024/after-empty-390-zh.png)、[网络失败](screenshots/mobile-ui-20261024/after-network-error-390-zh.png) |

## 验证结果及边界

- Flutter 全量测试：**174 通过，7 项按环境开关跳过**；静态分析通过。最后的分类文案/测试定位调整专项 **33 项通过**。
- Web 全量测试：**242 通过**。随后补充省直辖县级市场景，采购相关 **31 项通过**；TypeScript 检查通过。
- 原生新增列表矩阵：360、390、430px × 中英文 × 1.0/1.3/1.5 字体 × 厂家/产品，共 **36 组**；另有分类 **64 组**尺寸、字体、键盘布局验证。
- 浏览器视觉与交互：**190 组检查、0 脚本错误**。涵盖 360/390/430px、中英文、三档文字大小、1366/1920px 桌面，含抽屉点击、空结果、失败重试、返回列表滚动恢复及六栏目头部节点稳定性。[报告](screenshots/mobile-ui-20261024/browser-verification.json)
- 隔离 MySQL + 浏览器业务闭环：询价、采购商资料、草稿、附件上传/下载/预览、两家厂商报价、比价、意向联系方式、私信、审核扩散及结束均通过；**122 组布局检查，0 脚本错误**。[报告](screenshots/mobile-ui-business-20261024/verification.json)
- 新增公开摘要接口已验证读取不产生参与记录、产品名称搜索、不泄露私密字段、历史时间为空、暂停/非公开/过期返回不可访问。
- 采购 API 完整隔离数据库回归及 `go vet ./...` 通过；附件桥接 **4 项通过**。
- 浏览器大字体检查通过调大实际计算字号模拟，不能替代 Android 系统字体及显示比例真机验收。

本机 `adb devices` 未发现连接设备。**尚未实测**真实 Android 的覆盖安装、物理/手势返回、系统 WebView、键盘与手势区域、文件选择和保存、系统杀进程恢复及厂商系统差异；不能将编译或浏览器检查标记为这些项目已验收。

## 安装包与使用

独立 APK、签名/哈希清单及本轮验证摘要位于 `delivery/android-v1.8.1/`。构建使用 `scripts/build-apk.ps1 -BuildOnly`，沿用原包名和发布签名；构建会重新生成 APK 内置网页，不能只复制旧网页资源或只执行 Flutter 编译。

生产下载及部署保持原状。需要在提供采购接口的配套测试服务上验收采购功能；新增产品名称搜索、公开摘要和发布时间须使用本轮后端。测试没有在真实业务库发布需求、报价或审核，不发送正式短信。
