# Android 1.7.0+29 升级交付

日期：2026-10-17。沿用 Flutter 和原有 Android 包名、发布签名、接口及本地存储键。本轮仅调整 App，没有数据库迁移或后端新增接口，没有部署生产服务；网站 V2.0/V2.1 交付目录保持原样。

## 实际调整

- 保留首页、找厂家、找产品、我的四个底部入口及登录、收藏、厂商入驻、草稿和上传管理。品牌统一为中企展馆/ZQZG。
- 展馆配置读取 public/halls，仅明确 404 才启用旧服务器配置。网络异常显示重试；筹建展馆不请求业务列表。服务器、展馆和内容类型分别保存查询状态，旧请求不能覆盖切馆后的内容。
- 首页保留后台单张主视觉，最多显示 4 家企业、8 款真实产品。搜索提交后查询。厂家每页 20 家、产品每页 24 款，以服务器 total 为准，筛选折叠并显示数量。旧服务器不显示无法兑现的最新排序。
- 企业展厅六栏目统一命名。品牌区域只用于首页，子栏目显示紧凑企业信息；横向菜单自动定位仅滚动菜单。首页优先读取 home_products，兼容原代表产品字段及手动清空配置，去重并限制 8 款。产品中心仅保留一份列表，完整显示 4:3 产品图片、名称、型号和真实分类。
- 工厂照片保持原比例并可放大切换。视频卡片固定 16:9，点击播放后才初始化原生播放器；退到后台暂停，关闭释放。画册复用 PDF/图片阅读器，补充翻页、缩放、失败重试及受权限约束的下载。
- 联系信息来自当前企业公开资料；电话备注保留，拨号仅提取有效号码，多个号码可选择，无法识别则复制原文；复制失败提供手动复制。底部联系栏由 Scaffold 预留高度并适配安全区。
- 普通手机产品两列，大字号改为一列。新增中英文文案、48dp 主要按钮、图片回退与加载状态。

## 主要修改文件

| 文件 | 用途 |
| --- | --- |
| mobile/lib/exhibition_halls.dart | 展馆配置、404 兼容、筹建及错误状态 |
| mobile/lib/exhibition_home.dart | 单图首页、有限推荐、切馆缓存 |
| mobile/lib/browse_screen.dart | 搜索筛选、分页、查询与滚动状态 |
| mobile/lib/exhibit_card.dart | 完整产品图片、型号分类及企业卡 |
| mobile/lib/app_upgrade.dart | 紧凑企业信息、首页模块、媒体卡片、电话处理 |
| mobile/lib/resources_experience.dart | 六栏目展厅、产品中心、视频及画册阅读器 |
| mobile/lib/main.dart | 请求状态、接口能力、分类缓存、错误重试及联系组件 |
| mobile/lib/app_sync.dart | 当前企业的有限媒体推荐 |
| mobile/lib/industrial_ui.dart | 共用网格与触控尺寸 |
| mobile/assets/i18n/zh.json、en.json | 双语品牌、菜单和交互文字 |
| mobile/pubspec.yaml | 版本 1.7.0+29 |
| mobile/android/app/src/main/AndroidManifest.xml、res/values/strings.xml、res/values-en/strings.xml | 桌面与系统任务窗口品牌名称 |
| scripts/package-android-core.mjs | 校验通过后生成独立 App 交付包和文件清单 |
| mobile/test/app_v17_test.dart、app_v17_capture_test.dart | 兼容与新版回归、原生布局截图矩阵 |
| mobile/test/legacy_hall_fixture.dart 及现有相关测试 | 旧服务器与既有流程回归 |

## 截图

截图目录：docs/screenshots/app-v1.7。home 为首页，manufacturers 为找厂家，products 为找产品，showroom/about/company-products/videos/catalogs/contact 为企业六栏目。文件名后缀为逻辑像素宽度。全部为原生 Flutter 组件渲染。

## 验证结果与限制

- flutter analyze：无问题。
- flutter test：138 项通过，6 项需显式启用的截图/环境测试跳过；另行启用新版截图测试通过。
- 原生 Flutter 布局矩阵：360/375/390/414/768 逻辑像素 × 中文/英文 × 1.0/1.3 文字缩放 × 9 页面，共 180 组通过。
- 375×812 中文默认字号产品中心首张完整图片进入可视区域；新增断言防止展厅首页被菜单定位自动卷走。
- 提供 375/390/768 三种宽度共 27 张截图，数据及图片来自当前本机公开的 5 家企业、38 款产品。截图由 Flutter 原生组件测试渲染，不是网页截图，也不代表 Android 真机验收。
- 本机没有连接的 Android 真机或配置好的模拟器。覆盖安装、系统电话、相机、分享、真实网络视频、原生 PDF 和系统权限尚未验证。图片画册控制逻辑可回归，真实设备阅读体验待验收。
- 云端当前 public/halls 返回 404，因此默认连接云端时启用旧接口兼容；启用后续展馆、最新排序及 8 款首页补足依赖配套 V2 后端。旧接口仅按已有代表产品返回，不伪造补足数据。
- 运营核对：河北丹顿液压科技有限公司（vendor 21）的宣传视频 resource 2 / media 122 画面为击鼓人物，疑似误传。系统关联确为该企业，本轮保留原数据，由运营确认。

## 构建、更新与回滚

在项目根目录 PowerShell 执行：

```powershell
. ./scripts/flutter-env.ps1
cd mobile
& ../.tools/flutter/bin/flutter.bat analyze
& ../.tools/flutter/bin/flutter.bat test
cd ..
./scripts/build-apk.ps1
```

原生布局截图需要本地真实公开数据及图片夹具，保存在忽略提交的 .local/app-v17-*。运行时设置 APP_V17_CAPTURE=1，再执行对应测试。普通测试不依赖业务数据库。

APK 默认 API 地址保持 http://39.97.250.244:8089，保留已保存自定义服务器和已有本地数据。构建脚本核对原签名后，备份旧 APK 并更新 delivery/android、web/public/downloads、web/dist/downloads、delivery/web/dist/downloads。无需修改网站页面或覆盖既有版本网站包。

连接 Android 设备后可使用 adb install -r delivery/android/dalunongji-1.7.0+29.apk 验收覆盖安装。当前没有执行该安装命令。生产下载文件由运营另行更新；本轮未上传。

旧版 1.6.0+28 APK 和升级前源码保留在 .local/backups/pre-android-1.7，公开旧 APK 也保留在 delivery/android。Android 通常不允许直接降版本覆盖安装；保留用户数据时优先发布使用旧逻辑但版本号递增的兼容修复包。不要默认卸载，卸载可能清除本地草稿和登录数据。发布签名及 signing.json 仅留本机私有目录，不纳入交付包。

## 发布校验

- 包名：com.zqzt.dalunongji
- 版本：1.7.0+29
- APK：80,755,361 字节
- SHA-256：5f0a35407fcb6dc2e9001fa4f1578ca7f63463a7ea6feece2ab18866dfe1441d
- 原发布证书 SHA-256：85656c85541b188c832dc48d67cbf320046a4311aa9e0fefcc662dbe2e0f1585
- 四处本地下载文件哈希一致，localhost:8080 实际下载内容核验通过。
- 本地下载二维码仍使用现有后台配置地址 172.27.35.2:8080；跨设备访问需确认该地址在当前网络可达。未修改部署地址配置。
