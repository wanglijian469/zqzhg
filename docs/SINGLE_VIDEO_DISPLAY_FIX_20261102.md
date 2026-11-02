# 视频封面与播放区域重复展示修复

用户反馈：企业展厅同一视频看起来出现多处播放画面。

## 原因

实际 DOM 中只有一个 HTML5 播放器。旧封面卡片在点击后仍然存在，播放器追加到其下方，两处还重复显示视频名称。展开后卡片横跨网格，展厅旧封面宽高比样式把封面一起放大，进一步放大了重复画面的观感。

## 修改

- `web/src/resources.tsx`：正在播放的视频以播放器替换整块封面及简介；收起后恢复原封面并还原键盘焦点。切换用途及分页时收起旧区域。
- `web/src/inline-video.css`：播放中的卡片使用明确状态类，去掉重复边框和间距；本地视频保持自身横竖比例，限制最大高度及宽度。
- `web/src/resource-dialog.test.tsx`：增加本地视频及抖音回归，确认封面被替换、名称只出现一次、只有一个播放区域，收起卸载播放器并恢复入口。

视频记录、文件、审核和接口没有变动。PDF 与画册继续使用原来的阅读方式。本轮未部署线上。

## 验证

- 修复前新回归实际失败于“封面仍存在”；修复后相关五个测试文件 28 项通过。
- 类型检查、网页生产构建、独立测试 APK 构建及签名检查通过。
- 实际 Chrome 对企业展厅和产品详情分别验证 1440、768、390、360 CSS 像素，共八组通过：播放时卡片没有原封面和重复标题、只有一个视频元素、无弹窗、竖屏比例正常、实际本地视频播放时间推进、收起后恢复封面与焦点、无横向溢出。
- 截图与结果：`.local/appflow/screenshots/single-player-*`、`.local/appflow/single-player-results.json`。
- 这些结果不代表安卓真机验证通过，也不改变上一交付记录中抖音实播和正式发布的未完成状态。

## 更新测试包

本节是该次修复包的历史记录；后续本地视频首次点击问题的修复及最新包见 [首次点击修复](LOCAL_VIDEO_FIRST_CLICK_FIX_20261102.md)。本机同名 APK 已更新，本节包另存 `artifacts/previous/zqzg-editor-single-player.apk`。

- 本机：`.local/appflow/artifacts/zqzg-editor-phone-test.apk`。
- 局域网：[本轮修复测试包](http://172.27.35.2:18088/downloads/zqzg-editor-phone-test-single-player.apk)，旧测试下载文件保留。
- 包名：`com.zqzt.dalunongji.editdev`，名称“中企展馆发布测试”；108,517,938 字节。
- SHA256：`c89781869ed7abcd404c77c0ed06c191a3b949cdc965ab3b337cb0a0b904fc07`。
- 局域网完整下载 HTTP200，长度及哈希一致。正式 APK 文件及签名未修改，ECS/GitHub 尚未更新。
- 上一测试包已保存在 `.local/appflow/artifacts/previous/zqzg-editor-before-single-player.apk`。
