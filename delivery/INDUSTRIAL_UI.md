# 高端企业云展厅 UI V1.5.1

本版网站与 Android 使用深蓝主视觉、浅色内容，主色为深蓝 `#071B36`、科技蓝 `#1769E0`、银灰 `#E5EAF1`、浅背景 `#F4F7FB`。保留成功、待审和驳回的语义颜色。企业与设备展示使用已审核公开图片，没有添加模拟企业、交易或三维展厅功能。

## 展示与操作

- 首页保留搜索、入驻及 App 下载，强化逛展馆、看厂家、找产品入口；真实工厂照片与设备卡片组成主视觉，不自动轮播。
- 行业入口仍为农业机械、农机配件，名称从现有分类读取。只有一个精选厂商时使用宽幅品牌入口，避免空白网格；不会修改后台的精选或排序数据。
- 企业展厅展示封面、Logo、企业名称与主营，保留企业介绍、展品、短视频、电子图册、联系我们五栏目，默认展品。代表展品来自现有公开列表排序的前三项，不添加新的产品标记。
- 封面适度裁切；品牌 Logo、设备与参数图片保持完整。图片失败显示占位，缺少资料显示空状态。中英文切换、搜索条件、分类、页码、详情返回及旧展厅地址兼容。
- App 企业封面可随页面滚动收起，五栏目吸顶；首页和浏览页新增产品入口，直接使用已有公开产品接口。正常宽度的产品列表双列，窄屏或大字体使用单列。图片组件支持比例、裁切模式、加载及失败占位，沿用请求头鉴权。
- 网站厂商中心、管理员后台、下载与登录页统一基础视觉，保留操作顺序。二维码与宣传海报使用深蓝配色，分享规则保持兼容。

## 维护与兼容

网页集中样式位于 `web/src/industrial.css`，首页组件独立组织；App 主题和展示组件位于 `mobile/lib/industrial_ui.dart`。基础控件使用现有 Lucide 与 Material 图标。过渡以 180ms 为主，网页尊重减少动态效果设置；未加入自动播放或大幅滚动动画。

本次没有新增接口、数据库字段、迁移或依赖。公开、审核、下架、配额、上传处理、草稿、分享、公司账号登录沿用现有规则。升级只替换网站和 APK；服务器二进制复用相同业务代码，启动迁移仍为 1–5。

## 验收与截图

网站 21 项测试、生产构建通过；真实 Edge 浏览器核对 1440、1920、768、390、360 像素布局、单厂商宽幅入口、前三项代表展品、中文/英文、旧地址深链接、产品筛选及分页后详情返回。另验证真实厂商与管理员账号的只读工作台、长企业名称、缺失封面/Logo、无精选资料的页面。

Flutter analyze 零问题，12 项常规组件测试通过，另有专项原生组件截图校验通过。截图使用正式公开资料的只读副本，不代表连接真机验收。Windows 中文字体仅用于测试渲染，没有打包分发到 App；Material 字体使用 Flutter 自带资源。

| 页面 | 截图 |
|---|---|
| 首页电脑 | [1440px](screenshots/ui-home-1440.png) · [1920px](screenshots/ui-home-1920.png) |
| 首页手机 | [390px](screenshots/ui-home-390.png) · [360px](screenshots/ui-home-360.png) |
| 企业展厅 | [电脑](screenshots/ui-showroom-1440.png) · [手机](screenshots/ui-showroom-390.png) · [英文](screenshots/ui-showroom-english.png) |
| 产品目录 | [电脑](screenshots/ui-products-desktop.png) |
| 厂商中心 | [电脑](screenshots/ui-merchant-desktop.png) · [手机](screenshots/ui-merchant-mobile.png) |
| 管理员后台 | [电脑](screenshots/ui-admin-desktop.png) · [手机](screenshots/ui-admin-mobile.png) |
| App | [首页](screenshots/ui-app-home.png) · [企业封面](screenshots/ui-app-showroom.png) · [吸顶展品](screenshots/ui-app-exhibits.png) · [产品目录](screenshots/ui-app-products.png) · [大字体](screenshots/ui-app-large-text.png) |

证据位于 `test-output/ui-*`。运行 `node scripts/verify-industrial-ui.cjs` 可对当前本机冀农公开资料进行只读浏览器复核；此脚本需要已配置的本机服务与 `.tools/browser-qa` Playwright。

## Android 与交付

Android 版本 **1.5.1+18**，原签名证书 SHA-256 为 `85656c85541b188c832dc48d67cbf320046a4311aa9e0fefcc662dbe2e0f1585`。APK SHA-256 为 `0923cc89ce0a0f97896eb553521f3441be6af5d76ae1c4e460ef0ac11e1cb97e`；交付副本与网站 public/dist 下载文件一致。

电脑本机使用 `http://localhost:8080`；手机同网使用 `http://172.27.35.2:8080`，App 构建地址相同。版本化源码包、部署包和校验清单位于 `delivery`。iOS 继续即将上线。

ADB 当前无连接设备，覆盖安装、手机文件选择、视频播放/拖动、PDF/图片阅读、系统分享和真实系统字体仍需 Android 真机验收；没有将组件截图记为真机通过。本次不涉及公网部署。
