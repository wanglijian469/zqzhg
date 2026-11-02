# 本地视频首次点击黑框与0:00修复

## 原因与修改

旧流程在点击封面后只挂载 HTML5 播放器，仍保留 `preload=none`，没有播放器封面，也没有发起播放。因此未读取视频信息时出现黑框、0:00 和默认的小尺寸。之前的浏览器测试在展开后额外调用播放，漏掉了真实首次点击路径。

新增 `web/src/local-video.tsx`，在用户打开后读取视频信息、显示封面和已知横竖比例，并请求播放；页面初始状态仍不创建播放器。浏览器拒绝播放时明确显示“点击开始播放”，等待用户操作，不自动重试。网络缓慢及失败有文字反馈；卸载时暂停视频。

`web/src/resources.tsx` 接入组件及媒体宽高，`web/src/inline-video.css` 约束显示尺寸，更新 `resource-dialog.test.tsx` 和 `showroom-v2.test.tsx`。未修改视频文件、审核、数据库、配额或抖音官方播放逻辑。

## 真实验证

- 两项新回归修复前失败，修复后五个相关测试文件共30项通过。类型检查、网页及独立 APK 构建、签名检查通过。
- Chrome 对展厅与产品页分别检查1440/768/390/360，共八组通过。自动化只点击封面，不再额外调用 `video.play()`：播放时间推进、时长3秒、封面存在、没有重复封面或播放器；收起后卸载。
- 模拟媒体请求失败，实际页面出现加载失败提示；组件测试覆盖浏览器 `NotAllowedError` 后的明确播放按钮。
- 记录：`.local/appflow/video-start-results.json`、`video-start-green.log`；截图：`screenshots/first-click-*`。
- 本次截图中的“本地竖屏回归（仅副本联调）”是隔离环境的3秒测试素材。

## 最新独立测试包

- `.local/appflow/artifacts/zqzg-editor-phone-test.apk`，108,385,443字节。
- [局域网下载](http://172.27.35.2:18088/downloads/zqzg-editor-phone-test-first-click.apk)。完整GET200，长度及SHA256一致。
- SHA256：`cfeb0f104b32d61d2e4771cf7dc2beb2aa71bc2b5840f18cf8beec1a08c8cdb7`。
- 原包名 `.editdev`、账号及测试端口不变。上次包另存 `artifacts/previous/zqzg-editor-single-player.apk`，此前测试下载文件保留。
- 这是本地网页及独立测试包更新，未部署线上或替换正式下载。Android原生播放器没有本轮改动；真机验收、抖音实播及正式发布原有门槛保持。
