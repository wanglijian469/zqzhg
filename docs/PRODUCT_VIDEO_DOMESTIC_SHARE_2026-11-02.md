# 产品视频、厂商检索卡片与国内分享：本机交付记录

实施日期：2026-11-01 至 2026-11-02。仅本机开发和隔离验收，未部署 ECS、未更新正式 APK 下载、未推送 GitHub。

## 交付内容

- 网站和安卓的产品编辑第二步直接展示“产品视频（选填）”；新产品先通过原流程保存草稿，再添加视频，不自动提交产品审核。
- “添加产品视频”先选择粘贴抖音链接或上传本地视频，没有默认选项。复用原检查、授权确认、上传、封面、配额和审核功能；产品上下文固定当前产品，企业视频管理允许调整关联产品。重复视频引导前往企业视频管理；一条视频仍最多关联一个产品。
- 产品详情的视频展示和电子画册分开，图片布局保持原样。审核中的修改继续展示旧公开版本；企业或关联产品不公开时视频不可公开访问。
- 仅公共厂商检索列表在内容视口低于 1200 CSS 像素时隐藏整个展品速览模块，并移除对应网格列。采用 1199.98px 断点，兼容 360 小数视口；首页案例和管理预览不受此规则影响。
- 展厅移除来自厂商检索的“找厂家”独立返回行。保留全站找厂家主菜单、平台首页、产品列表来源返回和地图来源返回链接。
- 网站及安卓的展厅分享改为六个国内渠道：微信好友、朋友圈、QQ、QQ空间、企业微信、钉钉。展厅流程不再调用系统文字或海报分享。保留二维码、海报、宣传文案和复制失败时的手动复制；QQ/QQ空间使用原有官方网页入口，其他渠道提供具体手动发送指引。

本轮没有新增业务接口、数据库迁移或业务表，没有修改默认免费配额。此前抖音接口版本 2、Schema 29、原本地视频功能保持原有实现；快手、B站不在范围内。

## 本机预览与测试包

- 产品详情：<http://localhost:18086/products/23>
- 厂商检索：<http://localhost:18086/browse>
- 指定测试展厅：<http://localhost:18086/v/v-21>
- 测试 APK：`.local/product-experience/artifacts/zqzg-video-dev-debug.apk`
- 包名：`com.zqzt.dalunongji.videodev`；名称：中企展馆视频测试；独立调试签名。
- APK 连接 `http://10.0.2.2:18086`，适用于安卓模拟器访问宿主本机。不是正式升级包，不覆盖正式应用；连接真实手机需要另行配置可达的隔离服务地址。
- SHA256：`9e0ec65524f2ecd340b0dd844910aa468ea7f345d49ee8b4acdf0ad74b246d2f`。
- 构建脚本已校验包名、名称、签名、源文件一致性以及正式 APK 未变化。清单见 [apk-manifest.json](qa/product-experience-2026-11-02/apk-manifest.json)。

预览服务只监听 `127.0.0.1:18086`，使用原数据的完整隔离副本和独立媒体、地图目录。未为本次验收修改原数据库。测试副本中将抖音视频 5、本地视频 6、画册 7 通过分别提交和审核关联到产品 23，其他产品不会显示它们。首页案例配置、原有资源 1–4、原产品和企业的公开资料保持一致。副本中产品 57 的草稿指针和更新时间与原库存在差异，公开版本和原有版本资料未变；该副本不是正式业务库。

## 实际验证结果

| 检查 | 实际结果 |
|---|---|
| 网站类型检查、构建 | 通过 |
| 前端组件回归 | 6 个文件、44 项通过，覆盖产品草稿入口、固定关联、分享中英文、复制回退、播放器、页脚及检索返回 |
| 服务端相关测试 | 通过；包含独立产品/视频审核、私有父级隔离、旧公开版本、重复链接、越权及原链接安全检查；官方响应模拟测试与浏览器实播分别记录 |
| 完整数据副本接口流程 | 9 项通过：原公开版本保留、三项资源分别审核、产品关联、其他产品隔离、重复视频拒绝 |
| 安卓相关组件测试 | 12 项通过，包含产品来源无默认选择、固定关联、企业关联产品选择、国内分享不调用系统窗口及原本地上传相关回归 |
| 安卓静态检查、测试 APK 构建 | 通过 |
| Chrome 浏览器 | 56 项通过；含真实抖音播放、暂停、关闭卸载、本地 MP4 播放、PDF、产品编辑和国内分享 |
| 实际 360 浏览器 | 55 项功能检查通过；另一个“无页面脚本错误”检查未通过，记录到浏览器扩展 `chrome-extension://…/contentScripts/index.global.js` 的 `notice.avoidance` 异常及扩展消息通道警告，未将此项改写为通过 |
| 响应式 | Chrome、360 检查 1440、1280、1200、1199、1152、1024、960、768、720、390、360px；包括等效 125%、150%、200% 缩放后的内容宽度，无速览空列或横向溢出。不是浏览器工具栏缩放逐项实测 |
| 手机宽度实播及图片保存 | 抖音默认不播放，点击后播放、暂停、关闭卸载通过；二维码与宣传海报实际保存成功，六渠道说明及官方链接通过 |
| 原产品页高度修复 | Chrome 44 项、实际 360 44 项通过，包括加载、失败、短内容、照片关闭、历史前进和刷新 |
| 地图下钻、统计、手机布局 | 全国→河北→邢台→宁晋的真实边界和统计通过；仍为 11／10／8／8 家，手机无横向溢出 |

所有浏览器检查在隔离副本进行。Chrome/360 的手机宽度检查是桌面浏览器模拟，不代表真实安卓或 iOS 设备验收。未登录真实微信、QQ等账号发送或发布内容，也未承诺自动发布朋友圈。

## 未通过与未验事项

1. **地图返回后的自动重绘未通过**：正常进入展厅、地图返回链接和地区参数保留，但返回地图后可出现“地图图形加载失败”；点“重试”后边界恢复。调试捕获 ECharts 绘制中的 `Cannot read properties of null (reading '0')`。本轮没有修改地图页面、边界、绘制器或统计实现，不为此扩大改动范围；不能将整个地图返回验收标为通过。诊断记录见 [map-debug-final.log](qa/product-experience-2026-11-02/map-debug-final.log)，[实际截图](qa/product-experience-2026-11-02/map-return-retry-needed.png)。
2. **安卓设备实播未验**：ADB 未发现可连接设备；模拟器报告未安装 Android Emulator hypervisor driver。本次没有改变 Windows 虚拟化设置，APK 构建成功不代替设备上的播放、返回、全屏和后台行为验收。
3. **原安卓测试单列**：额外运行 `merchant_experience_test.dart`，其中资料编辑、二维码与海报测试通过；旧的公司登录测试在第 104 行查找输入控件时报 `Bad state: No element`。未修改登录功能或该历史测试来规避失败，日志完整保留。
4. 360 扩展异常按上表记录，未关闭个人浏览器扩展来掩盖问题；验收使用本次独立临时资料目录。

## 改动文件

网站：

- `web/src/product-editor.tsx`、`resources.tsx`、`video-playback.tsx`：显式产品视频入口及产品固定关联。
- `web/src/main.tsx`：分离产品公开视频、画册区域及引入本轮限定样式。
- `web/src/product-experience.css`：产品视频、国内分享和公共检索列表断点样式。
- `web/src/domestic-share.tsx`、`share-channels.ts`、`company-profile.tsx`：国内分享组件和发送说明，原组件入口复用。
- `web/src/showroom.tsx`：移除指定厂商列表返回行。
- `web/src/product-experience.test.tsx`、`showroom.test.tsx`、`vendor-list.test.tsx`：新增行为测试及因本轮变化而调整的断言。

安卓：`mobile/lib/main.dart`、`resources_experience.dart`、`video_links.dart`、`merchant_experience.dart`；新增 `mobile/test/product_experience_test.dart`。

服务端：仅新增 `server/product_video_test.go`，无本轮业务实现或结构变更。

完整清单见 [changed-files.json](qa/product-experience-2026-11-02/changed-files.json)。公共页脚、地图及此前产品页高度样式未变，文件对照见 [acceptance.json](qa/product-experience-2026-11-02/acceptance.json)。

## 截图和验证记录

- [PC 厂商卡片](qa/product-experience-2026-11-02/chrome/vendors-1440.png)、[低于1200px隐藏速览](qa/product-experience-2026-11-02/chrome/vendors-1199.png)、[手机列表](qa/product-experience-2026-11-02/chrome/vendors-390.png)
- [PC 国内分享](qa/product-experience-2026-11-02/chrome/domestic-share-1440.png)、[手机国内分享](qa/product-experience-2026-11-02/chrome/domestic-share-390.png)
- [PC 产品视频](qa/product-experience-2026-11-02/chrome/product-douyin.png)、[手机产品视频](qa/product-experience-2026-11-02/chrome/product-douyin-mobile.png)
- [PC 产品编辑](qa/product-experience-2026-11-02/chrome/product-editor-1440.png)、[手机产品编辑](qa/product-experience-2026-11-02/chrome/product-editor-390.png)
- [手机地图](qa/product-experience-2026-11-02/chrome/map-mobile.png)

其余实际 360 截图、浏览器逐项结果及前后端、安卓日志保存在 `docs/qa/product-experience-2026-11-02/`。记录不包含测试登录令牌或数据库密码。

## 备份与回退

修改前未提交改动已保存到 `.local/product-experience/before-changes.zip`（74 个文件）、`before.patch` 和 `before-manifest.json`。本轮相对备份的差异为 `.local/product-experience/this-task.diff`。

如需回退，只还原清单中本轮修改的文件到备份版本，并移除本轮新增文件；先保留之后产生的新工作，不执行全仓库 reset。重新构建网站和独立测试 APK 即可。本轮没有数据库迁移需要回退，不能降级或删除先前 Schema 29；原正式数据库和正式 APK 无需恢复。
