# 移动端升级验收包：1.8.0-rc.1+30

日期：2026-10-20。同步10月18—20日的厂商端优化。Android 保持原生 Flutter；手机网页保持 React。仅更新本机服务，未部署生产。

GitHub预发布入口：[v1.8.0-rc.1](https://github.com/wanglijian469/zqzhg/releases/tag/v1.8.0-rc.1)。APK、源码和服务器更新包作为Release附件，按附件中的 `SHA256SUMS.txt` 核对；该清单使用扁平附件文件名，与本机交付目录的总清单分开。发布说明见 `releases/v1.8.0-rc.1/README.md`。预发布不替换原稳定版。

## 下载与安装

- 本地下载页：`http://localhost:8080/app`，选择“移动端升级验收包”，不要误选原稳定包。
- APK：`android/dalunongji-1.8.0-rc.1+30.apk`。版本名 `1.8.0-rc.1`，构建号 `30`，包名 `com.zqzt.dalunongji`。
- 原签名证书 SHA-256：`85656c85541b188c832dc48d67cbf320046a4311aa9e0fefcc662dbe2e0f1585`。与原 `1.7.0+29` APK 一致；不是另一个可并存的包。
- minSDK 24、targetSDK 36；ABI 为 `armeabi-v7a`、`arm64-v8a`、`x86_64`。最终 APK 哈希、字节数以同目录 `manifest.json`、`SHA256SUMS.txt` 为准。
- 设计为直接覆盖安装；不要卸载或清除 App 数据。账号、收藏、语言、自定义服务器、本机草稿及上传文件继续沿用原存储。覆盖安装及原生权限尚待真实设备确认。
- 原稳定下载 `/downloads/dalunongji.apk` 及四份稳定文件均保持 SHA-256 `5f0a35407fcb6dc2e9001fa4f1578ca7f63463a7ea6feece2ab18866dfe1441d`。
- 默认服务仍为 `http://39.97.250.244:8089`。验收时在 App 设置手动切换至 `http://172.27.35.2:8080`，该地址已从本机实际请求健康接口返回200；手机须在可互通网络中。手机上的 `localhost` 指手机自身，不能代替电脑。电脑 IP 改变后重新核对。没有将这个临时局域网地址写成新的默认服务。
- 验收版安装后不保证能够以较低构建号覆盖退回稳定版；后续正式发布使用高于30的构建号。

## 同步记录

| 范围 | 本次实现与兼容 |
|---|---|
| 入驻、公司登录 | 公开入口先注册公司账号，再到工作台完善企业并提交审核；注册本身不建企业。公司密码登录默认显示，短信为次级方式，访客收藏及编辑中重新认证仍可用。 |
| 认证体验 | 独立密码显隐、自动填充、首错聚焦、键盘提交、加载及请求锁；切换方式清空密码并恢复隐藏。账号最多100字符，密码12—72 UTF-8字节。网络失败保留输入，注册结果不明确提供登录继续，不自动重发注册。管理员重置密码说明保留。 |
| 草稿归属 | 按服务器、账号、企业／产品隔离，保留旧格式；保存步骤、云端ID、修订和图片顺序。新注册可恢复访客资料，已有账号须明确同意接收访客草稿；不覆盖已有云端企业。账号切换和会话失效继续隔离。密码不进入URL或存储。 |
| 企业编辑 | 基本资料集中、可选资料折叠，介绍范文及事实型建议，真实省市县级联；城市级及历史文字继续可用，不补造区县。保存草稿留在编辑页，提交成功返回展厅管理。修复展开可选项后输入失焦问题。 |
| 产品编辑 | 基础信息→图片与展示资料→预览提交。新版服务仅名称必填；分类、关键词、图片、英文资料可选。建议和空白参数模板不生成性能、资质或数值。分类查询跟随企业展馆。 |
| 图片与提交 | 本机预览、上传进度、失败重试、封面排序；成功上传ID复用，提交失败保留云端ID。首次创建结果不明确时阻止自动再次创建。 |
| 工作台与推广 | 保留原入口和四个底部导航；必填／建议项直达、真实状态统计和产品筛选。推广沿用公开版本，校验展厅URL、二维码／海报／复制／系统分享调用路径；不承诺外部应用最终分享成功。 |
| 厂商地图 | 首页与找厂家增加入口；统计、搜索、行业、地区选择、面包屑、未细分筛选、厂商分页及展厅返回接入原接口。新增原生GeoJSON投影、孔洞、多面、命中、平移缩放和缓存适配层。正式资源清单为空时明确提示轮廓尚未开放。 |
| 手机网页 | 保留已完成的蓝白入口、资料及三步产品页；实际浏览器重新验证注册登录、草稿恢复、触屏尺寸、键盘及返回流程。正式资源优先，现有本机学习预览限制保留。下载页新增独立验收包卡片。 |

旧服务器只有在明确未提供能力时显示兼容说明／升级要求。网络失败显示重试，不能伪装成“不支持”。不会偷偷填入分类、关键词或地区通过旧校验。能力缓存按服务器隔离，切换服务器清除旧请求状态。

## 接口与数据

新增的是现有 `GET /api/v1/public/config` 的向后兼容字段：

```json
{"capabilities":{"administrative_regions":true,"industry_map":true,"product_name_only":true}}
```

继续复用认证、企业／产品、媒体、行政目录及 `/api/v1/public/industry-map/regions`、`/api/v1/public/industry-map/vendors`。未细分筛选用接口已有的 `unassigned=0|1`。审核、权限、已发布修订口径不变。本次没有新增业务表或迁移；服务器包包含既有001—016号迁移及行政目录。

实际写入操作只在自动创建并删除的隔离 MySQL 数据库执行。业务库的公开厂商9家（ID：1、21、27、37、39、44、48、51、53）、产品45件，在升级前后核对一致；没有插入测试企业或修改业务账号密码。

## 地图交付边界

正式真实轮廓**未完成外部资源验收**。本轮没有取得允许网页和App着色下钻、同源分发、缓存及最终公开展示的完整材料。

- DataV 官方明确仅供学习交流，不能据此认定可进入正式 APK。已核查 DataV 说明及高德接口、条款和授权入口，记录见 `docs/MAP_RESOURCE_REVIEW.md`。
- 正式 `/maps/manifest.json` 仍为空；DataV 预览文件未打进 APK、源码包或服务器包。APK 不调用开发预览接口。
- 原生真实轮廓适配层已用隔离几何夹具验证，**夹具测试不等于真实省市县验收**。无资源时提供统计、地区选择和厂商查询，不用方块或测试几何宣称正式地图完成。
- 网页已有 DataV 轮廓继续限开发环境、显式开关及直接回环请求。生产默认示意图行为保留。
- 全国→河北→邢台→宁晋等真实轮廓的正式发布验收、北京／重庆／仙桃最终边界编码核对，等待适用资源材料；本轮名称保持“移动端升级验收包”。

## 验证结果

| 命令／范围 | 实际结果 |
|---|---|
| `flutter analyze --no-pub` | 无问题。 |
| `flutter test --no-pub --reporter expanded` | 154通过，7跳过（6个旧截图开关测试、1个需服务地址的真实接口测试）。跳过项不计为通过；本轮原生真实接口另行运行并通过。 |
| `APP_V18_CAPTURE=1 flutter test test/mobile_upgrade_capture_test.dart --no-pub` | 168个布局组合通过：360、375、390、414、768宽及844×390横屏，中英文、1.0／1.4字号、注册／登录／企业／产品／地图及产品步骤。15张原生组件截图，非真机截图。 |
| `RUN_MYSQL_TESTS=1 go test -count=1 ./...` | 通过（185.754s），所有数据库写入使用临时测试库。 |
| `go vet ./...` | 通过，无输出。 |
| `RUN_MYSQL_TESTS=1 RUN_MOBILE18_INTEGRATION=1 go test -run TestMobileUpgradeIntegration -count=1 -v` | 通过（11.643s），Flutter模型实际连接隔离HTTP服务，注册／重名／错密／登录、企业及产品保存／审核、真实PNG上传、冲突、开发短信独立账号、地区统计及未细分两种查询。 |
| 隔离浏览器 `TestMerchantEntryBrowser`、`TestIndustryMapBrowser`、`TestMapPreviewBrowser` | 通过（41.518s），实际页面交互、历史、失败降级及本机预览隔离。 |
| 隔离浏览器 `TestCompanyOnboardingBrowserHarness`、`TestProductV22BrowserHarness` | 通过（46.760s），企业／产品真实接口、上传重试、草稿、步骤、返回和尺寸。产品报告7项功能及60布局组合。 |
| 前端 `npm test -- --run` | 34文件、185测试通过。 |
| 前端 `npm run build` | 成功；保留现有大分包提示。 |
| `python scripts/test-aliyun-upgrade.py` | 7项通过，含schema16允许、17拒绝、旧默认15保留及备份恢复。 |
| `scripts/build-apk.ps1 -Preview` | 发布构建、包名／版本／ABI及原签名核对通过。源码构建前后哈希核对，不发布构建中被修改的源文件。 |
| `node scripts/verify-mobile18-local.cjs` | 只读本地验收通过：4页面×5视口、验收／稳定APK实际下载哈希、本机轮廓点击、浏览器历史、局域网健康及预览拒绝、业务数量和ID保持一致。重复视口复用成功GET以遵守原限流，所有浏览器写请求被拦截。 |

原始输出在 `verification/mobile18-*`，布局记录 `mobile18-native-layout.json`，本地浏览器记录 `mobile18-local.json`。网页尺寸覆盖1440×900、1366×768、390×844、360×640、844×390；错误展开及手机键盘缩小视口允许自然滚动，不裁切表单。交付截图含15张原生组件截图和本地网页截图；网页本机学习轮廓点击通过不等于正式地图发布验收。

### 尚未验收

- `adb devices -l` 没有连接设备：覆盖安装、旧数据在真实安装后的保留、系统相机／照片权限、拨号、系统分享与外部应用最终行为、原生媒体播放需真机验收。
- 系统分享、二维码和海报已验证数据及调用路径，不能将组件或模型测试当成第三方应用完成分享。
- Linux amd64／arm64已交叉编译并核对ELF架构；未在Linux主机实际安装，也未验证MySQL8与真实OSS。
- 正式地图资源及最终真实轮廓验收待上述材料，不包含在“已通过”中。

## 源码、服务器更新包及本地复现

交付目录含APK、源码ZIP、服务器更新ZIP、哈希清单、截图和验证记录。源码包按目录白名单生成，排除 `.local`、密钥、`.env`、私有媒体、业务数据库、生成构建目录和下载包。服务器更新包包含Linux双架构程序、网页静态文件（含原稳定下载，排除验收下载）、001—016迁移、目录、许可、升级工具和校验清单。

本地运行：`npm run build`（web目录），然后根目录 `scripts/start.ps1 -MapPreview`。已有本地服务已更新；学习预览仍受直接回环限制。验证本机服务使用 `http://localhost:8080`，验收下载页 `/app`。

重建验收包：

```powershell
./scripts/build-apk.ps1 -Preview
```

预发布版本禁止省略 `-Preview` 覆盖稳定文件。默认地址保持原配置；不需要更换密钥。请妥善保管原签名，交付包不含密钥。

服务器包是供后续升级的材料，本轮**没有执行部署**。已有托管Linux安装可在核验环境及备份条件后解压，先使用 `sudo bash upgrade.sh --check` 只读预检；获部署授权后才使用 `--apply`。可通过 `INSTALL_DIR`、`ENV_FILE`、`SERVICE` 指定既有安装。应用步骤停止服务、私有备份数据库／媒体／控制文件／环境、切换程序并健康检查，保留原数据库、媒体根和加密密钥；拒绝高于包schema16的库。此包不负责首次安装。不应仅替换旧程序强行回退，失败须按配套备份成对恢复。

## 主要修改文件

- Flutter：`mobile/lib/main.dart`、`native_auth_entry.dart`、`mobile_upgrade.dart`、`native_industry_map.dart`、`onboarding.dart`、`exhibition_home.dart`、`browse_screen.dart`、`merchant_workspace.dart`、`merchant_experience.dart`，版本、依赖与中英文资源，以及对应测试。
- 网页：`web/src/app-download.tsx`、`industry-map-chart.tsx`、`industry-map-outline.ts`、`industry-map-resources.ts`、`industry-map-types.ts`及资源／渲染测试。
- 后端：`server/system_settings.go`、`mobile_upgrade_test.go`及企业／产品隔离浏览器测试入口。
- 构建升级：`scripts/build-apk.ps1`、`package-mobile-acceptance.py`、`test-aliyun-upgrade.py`、`verify-product-v22.cjs`、`verify-map-resources.mjs`、`deploy/linux/upgrade-storage.sh`、`aliyun-upgrade-support.py`。
- 说明：本记录、`docs/MAP_RESOURCE_REVIEW.md`、截图及 `docs/test-output/mobile18-*`。此前地图和入口已完成的源文件保留于源码包，没有回退已有功能。
