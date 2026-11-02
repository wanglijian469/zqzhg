# 恢复弹出视频播放：本机及手机测试交付

日期：2026-11-02。本记录取代此前三份手机测试记录中的“页面内展开播放”说明；此前产品、企业管理及输入稳定性改动保留。

## 本轮行为

- 网站展厅、产品详情、视频管理、编辑及链接检查预览共用视频弹窗。封面留在列表内，页面不再展开第二个播放器；标题在弹窗内只显示一次。
- 本地视频打开后只发起一次有声播放请求，不静音。播放被浏览器拒绝时显示“点击开始播放”，不自动循环重试。
- App 本地视频打开专用播放页并初始化、有声播放；抖音先校验官方地址，再直接打开原生播放窗口，没有中间播放页。
- 输入、移动光标、粘贴和检查链接不播放；过期响应不打开窗口。编辑预览关闭后保留输入，网站恢复入口焦点。
- 关闭、离开、后台或切换时停止并释放；返回前台不自行重播。新本地播放器等待旧播放器释放。全屏返回先退出全屏。
- 视频弹窗移到表单之外，避免其关闭按钮触发编辑表单保存。PDF、画册仍沿用原交互。
- 本轮未新增接口、数据库迁移或依赖；未修改地图、页脚、产品审核、配额与原视频数据。

## 抖音支持边界

[官方接口文档](https://partner.open-douyin.com/docs/resource/zh-CN/dop/develop/openapi/video-management/douyin/iframe-player/get-iframe-by-video)未明确承诺自动起播。本轮保留已校验的官方播放器和 `autoplay=0`，没有修改参数、模拟点击或操作跨域播放器。

**抖音自动起播未实现，实播仍未验收通过。** 页面明确引导点击官方播放按钮。此次实际验证了官方检查可用、弹窗承载和关闭卸载，没有将接口成功或 iframe 出现当作实播通过。授权测试视频 ID：7690195495465012480。

## 改动文件（相对本轮开始前的副本）

- Web：`web/src/resources.tsx`、`video-playback.tsx`、`inline-video.css`。
- Web 测试：`resource-dialog.test.tsx`、`video-playback.test.tsx`、`showroom-v2.test.tsx`。
- App：`mobile/lib/app_upgrade.dart`、`resources_experience.dart`、`video_links.dart`。
- Android：`OfficialVideoView.kt`、`VideoPlayerActivity.kt`。
- 新增 App 测试：`mobile/test/popup_video_test.dart`。
- 本记录及 `docs/screenshots/popup-video-20261102/` 截图。

本轮开始前的未提交工作没有重置；上述列表不包含此前产品流程、视觉和服务端改动。

## 实际验收

| 项目 | 结果 |
| --- | --- |
| Web 相关组件测试 | 3 文件、31 项通过，无未处理异常 |
| Web 类型检查与生产构建 | 通过；保留现有大分块体积提示 |
| App 相关测试 | 4 文件、28 项通过，包含双击、直接官方窗口、输入保留、迟到响应、初始化中切页/后台/换账号及等待旧播放器释放 |
| Flutter 静态检查 | 无问题 |
| 服务端视频相关测试 | 使用测试 MySQL，所选视频链接、平台失败、草稿及产品关联测试通过；本轮服务端未改动 |
| Chrome 实际点击 | 47 项断言通过：展厅与产品页分别检查 1440/768/390/360px；未额外调用 `play()` 代替用户点击；本地视频时间推进、未静音、单窗口、关闭及焦点恢复通过 |
| Chrome 管理与编辑预览 | 本地视频自动起播通过；抖音预览单窗口、输入保留及关闭无保存请求通过 |
| Chrome 全屏 | 实际点击进入全屏，第一次 Esc 退出全屏、第二次关闭弹窗通过 |
| 网页真实切换标签后台 | 无头浏览器未产生隐藏状态，不能宣称此项实机通过；已有生命周期自动化覆盖 |
| 安卓真机/模拟器 | 没有可连接设备，未进行实播、锁屏、后台和真实返回交互验收 |
| 正式发布 | 未执行 ECS、正式 APK 替换或 GitHub 推送 |

早先完整 Web 测试结果为 286 通过、43 失败及 7 个未处理异常。对本轮开始前副本的对照能复现其中 42 项失败，另 1 项尚未稳定归类；不能将全部失败称为已证实的历史失败。此次最终的相关 31 项均通过，不代表完整测试已全部通过。原始记录保留在 `.local/popup-video/web-full-suite.log`、`web-baseline-failures.log`。此前 App 全量失败记录见 `APP_EDITING_PHONE_TEST_20261102.md`。

## 预览和手机包

- 电脑预览：<http://localhost:18088/products/23>、<http://localhost:18088/v/v-21?tab=videos>。
- 手机网站：<http://192.168.1.19:18088/products/23>。
- 手机 APK：<http://192.168.1.19:18088/downloads/zqzg-editor-phone-test-popup.apk>。
- 文件：`.local/appflow/artifacts/zqzg-editor-phone-test-popup.apk`。
- 包名：`com.zqzt.dalunongji.editdev`；名称“中企展馆发布测试”。与正式 App 共存，连接完整隔离数据副本。电脑地址变化后重新编译，旧测试会话不会跨地址使用。
- 大小、SHA256、签名及完整下载核对结果见下方最终包记录。

手机和电脑连接同一局域网；电脑及隔离服务保持运行。临时服务本次于 18:36 启动，约 22:36 自动关闭。可提前执行 `.local/appflow/stop-phone.ps1` 关闭。测试账号仅在副本有效，见本机 `.local/appflow/artifacts/手机测试说明.md`，不在本文记录口令。

手机重点检查：展厅/产品视频、本地与抖音编辑预览；播放、暂停、进度、全屏返回、关闭、再次打开、锁屏和切后台；检查时修改链接、重复粘贴和返回编辑不丢输入。抖音官方按钮及自动起播分别记录，不能以页面加载代替播放成功。

## 截图与记录

- [电脑展厅弹窗](screenshots/popup-video-20261102/local-popup-showroom-1440.png)
- [390px 产品视频弹窗](screenshots/popup-video-20261102/local-popup-product-390.png)
- [抖音弹窗](screenshots/popup-video-20261102/douyin-popup.png)

以上均为 Chrome 截图，不是安卓设备截图。原始测试、构建及下载日志在 `.local/popup-video/`。本轮改动前备份在该目录的 `android-before/` 和 `web-20261102-162454/`。只回退本轮文件时应与这些副本逐项比较，不能重置其他未提交工作；本轮无需数据库回退。

## 正式发布门槛

待安卓设备验证、抖音能力核验、全量失败处理及既有发布门槛通过后，再使用已授权正式发布流程。生产地图资源与代理路径的四级真实边界验收仍须按 `MAP_DEPLOYMENT_REQUIREMENTS.md` 完成，本次临时开发预览和 APK 构建不能替代该项生产验收。

## 最终包记录

- 构建时间（UTC）：2026-11-02T10:49:43.8763691Z。
- 字节数：146143442。
- SHA256：`98f88d67b16bedefea4a1044d71aa1d43be8d3bac7e33d5f9a8f17adae500991`。
- LAN 完整 GET：HTTP 200，字节数及 SHA256 与本机包完全一致。
- 包名、签名检查通过；正式下载文件及正式包配置校验未变化。
