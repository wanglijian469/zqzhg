> 后续更新：视频交互已恢复弹出播放，当前 APK、地址及手机清单以 [弹出播放交付记录](POPUP_VIDEO_PLAYBACK_20261102.md) 为准；本文保留此前历史验收结果。

# App 编辑流程与内嵌视频：手机测试交付

本轮交付独立手机测试包，未部署 ECS、未替换正式下载、未提交或推送 GitHub。正式发布仍受安卓设备交互验证及完整回归门槛约束。

## 测试包与环境

本节记录首轮测试包；后续修复包与新哈希见 [视频重复展示修复](SINGLE_VIDEO_DISPLAY_FIX_20261102.md)。本机同名 APK 现已更新，首轮包另存于 `artifacts/previous/zqzg-editor-before-single-player.apk`。

- 包名：`com.zqzt.dalunongji.editdev`；名称：中企展馆发布测试。采用调试签名，可与正式版共存，未改变正式签名或版本号。
- 文件：`.local/appflow/artifacts/zqzg-editor-phone-test.apk`，108,517,765 字节。
- SHA256：`420229b4e7e2cf1d1e0a410bc4fa440c71f348226a874bd11a4c88aa3f8bc259`。
- 局域网下载：`http://172.27.35.2:18088/downloads/zqzg-editor-phone-test.apk`；本机预览：`http://localhost:18088`。
- 测试库：`zqzt_exhibition_test_appflow_20261102`，来自此前完整隔离副本，媒体、地图单独复制。地图文件哈希逐项一致，公开厂商/产品保留 11/46。
- 用户明确批准仅手机测试期间开放局域网 18088。测试服务启动后四小时自动停止；可提前运行 `.local/appflow/stop-phone.ps1`。未修改防火墙设置。手机需与电脑网络互通，跨网络不可直接访问。
- 独立账号仅写入测试库；登录信息存于忽略的 `.local/appflow/artifacts/手机测试说明.md`，不入 Git。
- 短信、定时发布、轮播、备份任务关闭；仅启用原媒体转码任务，使用项目已有 FFmpeg。正式数据库、媒体及 APK 未改动。

## 当前代码

- App 产品列表保留管理操作，编辑采用“添加产品 → 完善详情 → 预览发布”。图片在第一步以三列网格显示，菜单提供封面、排序、替换及解除关联。
- 草稿保留输入及创建请求编号；服务端按账号和编号去重。创建超时后重用原始创建请求，再保存后续编辑，避免生成重复产品。
- 产品复制只带资料与图片引用；视频、画册和审核信息不复制。提交校验名称、分类及图片，视频选填。
- 企业资料采用六项管理入口和独立栏目，保存合并后的版本，仍使用原版本冲突和审核流程；联系方式继续审核后公开。
- 当前有效协议已经确认时不重复显示确认控件，保留历史审计；缺少有效版本时仍需确认。
- Web 与 App 的产品、展厅及管理视频入口改为原位置展开。网页沿用 HTML5/官方 iframe；App 使用页面内 Android PlatformView。PDF、画册仍走原阅读方式。
- 输入抖音链接不自动请求、导航或加载播放器。文字未变的光标/输入法通知不再清除检查结果和授权勾选；检查加锁、过时响应丢弃、失败不自动重试。播放器开关与输入控制器分离。
- 全局播放会话管理切换、页面离开、后台停止；官方播放器不携带企业登录令牌。保留旧通道用于兼容，新视频入口不再调用独立播放 Activity。
- Chrome 实测发现竖屏封面在网格中越界遮挡收起按钮，已将封面约束到自身容器；未改变产品图片画廊。

## 数据库及回退

Schema30 新增 `content_create_requests`，记录账号、请求编号、资料摘要和内容编号。与内容创建在同一事务提交；旧客户端不传编号仍兼容。Schema29 与历史资源、审核及公开版本不重写。

本轮前未提交工作备份在 `.local/appflow/before.zip`。不要整库恢复或降级迁移来回退界面。测试环境可直接停服务；正式环境尚未上线本轮代码。正式发布时需另做生产备份、恢复演练、代理及最终 APK 地图验收。

## 已执行与限制

| 项目 | 实际结果 |
| --- | --- |
| Android 静态检查 | 无问题；原生 Kotlin 和独立 APK 构建、签名及包名检查通过 |
| Android 完整自动化 | 194 通过、8 跳过、42 失败；修改前独立对照为 173 通过、8 跳过、44 失败。最终失败全部也存在于对照中，未将完整测试标为通过 |
| 关键 App 流程 | 产品/企业/分类/图片/协议及播放器相关八文件 44 通过、1 跳过；补充视频输入异常与过时响应在完整测试中通过 |
| Web 相关自动化 | 产品编辑、视频、资源阅读、产品资料和地图共七文件 40 通过；类型检查和生产构建通过 |
| 真实隔离接口 | 测试账号登录、草稿创建去重/冲突、缺图提交拒绝、版本冲突、完整产品审核/公开/下架、旧客户端及恶意地址验证通过 |
| 本地视频 | 实际上传测试 MP4，原转码流程得到 ready、时长 3 秒；Chrome 播放推进、暂停与收起通过 |
| 抖音 | 已授权视频官方检查 READY、官方 iframe 实际 HTTP200 并出现视频元素；但 Chrome 点击后未观察到播放时间推进，实播、暂停及全屏未通过，不把 iframe 加载当作播放成功 |
| 响应式 | Chrome 1440/768/390/360：内嵌区域、收起、链接变更卸载、无横向溢出通过；四级真实地图显示通过 |
| APK 下载 | 局域网完整 GET 200，下载长度及 SHA256 与本机包一致 |
| Android 真机/模拟器 | 尚未实测；全屏、返回、后台停止、长文本/输入法、多图拍照及实际播放仍需手机记录 |
| 正式发布 | 未执行；不能用编译或组件测试替代设备验证，也不能忽略完整回归中的失败 |

输入控件状态误清除的触发点已经通过自动化复现并修正；用户手机所见的“持续刷新”症状仍需真机复测，未宣称已完全解决。

测试原始记录位于 `.local/appflow/`：`mobile-all-final.log`、`mobile-baseline-tests.log`、`mobile-baseline-comparison.json`、`web-final-tests.log`、`backend-final.log`、`api-results.json`、`browser-results.json`、`local-upload-results.json`、`official-browser.json`、`official-play-results.json`。Chrome 截图在 `screenshots/`，不是安卓真机截图。

## 改动文件

- App：`mobile/lib/main.dart`、`merchant_form_flow.dart`、`merchant_products.dart`、`onboarding.dart`、`taxonomy.dart`、`service_center.dart`、`video_links.dart`、`inline_video.dart`、`inline_resource.dart`、`resources_experience.dart`、`app_upgrade.dart`。
- Android：`MainActivity.kt`、新增 `OfficialVideoView.kt`、`build.gradle.kts` 及 `src/editdev/res/values/strings.xml`；构建入口 `scripts/build-editor-test-apk.ps1`。
- Web：`web/src/main.tsx`、`product-editor.tsx`、`resources.tsx`、`video-playback.tsx`、`inline-video.css`。
- 后端：`server/main.go`、`content.go`、`content_create_requests.go`、`form_validation.go`、`showroom.go`、`service_documents.go`、`procurement_demo.go` 及迁移030。
- 新增及更新相应 App、Web 和后端测试。此前品牌视觉文件及页脚保留，本轮没有部署旧网站、地图或页脚改版。

## 手机重点测试

1. 输入授权抖音链接，移动光标、重复粘贴；只在点“检查链接”后请求。检查中改链接、断网、返回编辑，观察有无刷新或输入丢失。
2. 编辑预览、产品视频、企业展厅视频应原位置展开；验证播放、暂停、收起、切换另一个视频、全屏返回、锁屏和后台停止。
3. 新建不完整产品草稿、退出再进入；补分类和图片后提交。复制产品确认不带视频，待审时不覆盖旧公开资料。
4. 企业资料、联系方式分别修改；已确认有效协议不重复确认。拍照、多选、封面与排序、失败重试需在手机记录。

记录手机型号、Android 版本、失败步骤或录屏。设备验证完成且其余发布门槛通过后，再处理 ECS、正式 APK 下载及 GitHub 推送。
