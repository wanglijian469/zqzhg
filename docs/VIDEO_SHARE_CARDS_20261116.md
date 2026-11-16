# 快手与微信视频号免费分享卡片验收

日期：2026-11-16。范围：本地后端、网页、Flutter App 源码与验收；未部署生产、未发布正式 App，也未替换官网下载包。

## 完成行为

- 企业及产品的视频添加入口提供抖音、快手、微信视频号、本地上传四种来源；管理员复用同一编辑表单。视频号需要 API v4，旧服务能力下仍保留原有入口。
- 快手、视频号填写分享链接、名称、简介、用途及关联产品；封面可选，沿用企业图片或默认封面。继续使用现有协议确认、草稿、独立资源审核与公开快照流程。
- 链接检查明确表示只验证格式，审核后可前往原平台观看。提交不要求视频文件，资源的 `file_media_id` 为 0。可选封面仍占用原有图片存储。
- 公开卡片提供来源标签、观看按钮、复制链接。网页的封面、标题及观看按钮直接在新标签页打开原链接；App 使用系统外部打开方式。不会先进入无法播放的站内播放器。
- 外跳前停止站内播放；不切换原页面，因此保留展厅筛选、分页及当前位置。剪贴板不可用时提供手动复制。视频号提示可复制后在微信内打开，不承诺自动唤起微信。
- 抖音继续使用原官方播放器，本地视频继续使用原播放器；保留本地上传入口。

## 服务端与兼容

- 新增来源 `WECHAT_CHANNELS`，复用资源、版本、审核和链接检查字段，无数据库迁移，未修改历史审核快照。
- 示例 `https://weixin.qq.com/sph/Aht6MMIJPm` 原样保留分享码大小写。只接受 HTTPS、准确的 `weixin.qq.com` 域名和 `/sph/分享码` 路径。拒绝伪造域名、账号密码、端口、编码路径、片段、多个链接及非法路径。规范化会移除末尾斜杠及分享跟踪查询参数。
- 分享码限制为 3～60 位字母、数字、下划线或连字符，带 `sph:` 前缀的资源身份不超过现有 64 字符存储限制。
- 快手保持现有分享链接规则及身份；快手和视频号的检查、审核检查、播放信息均走本地格式校验，返回 `EXTERNAL_ONLY` 与规范化原链接，不调用解析服务，不下载或中转原视频。
- 运行配置声明视频 API v4；网页和 App 按能力协商 4→3→2。v4 展示视频号列表、详情及数量；v2/v3 的原来源与用途兼容行为保留。
- 旧客户端隐藏视频号资源及数量，拒绝详情管理和修改；即使旧编辑器漏传来源或发送默认 `LOCAL`，也不能覆盖包含视频号的草稿或公开版本。
- 更新 `docs/openapi.json`、网页类型及 App 中英文来源、检查结果、观看和复制文案。

## 验收结果

| 检查 | 结果 |
| --- | --- |
| 后端相关回归 | 14 项通过，使用自动创建并销毁的隔离 MySQL 数据库 |
| 网页相关测试 | 4 个文件、33 项通过 |
| TypeScript | `tsc -b` 通过 |
| 网页生产构建 | 通过；保留已有的大于 500 kB 分块提示 |
| Go 静态检查 | `go vet ./...` 通过 |
| Flutter 静态分析 | 无问题 |
| Flutter 相关测试 | 5 个文件、31 项通过 |
| Flutter 截图测试 | 2 项通过，390/768px，6 张截图 |
| 真实后端与构建网页联通 | 1440、1024、768、390、360px × 中英文，10 组通过，20 张截图 |

后端覆盖链接格式与大小写、企业隔离、重复链接、审核前后可见性、草稿不修改公开版本、旧客户端读写保护、列表与展厅数量。测试将视频 HTTP 客户端替换为拒绝所有网络调用的传输器，确认快手/视频号相关新流程不调用解析器或拉取视频。抖音 v2 生命周期、产品关联、平台协议、解析错误及短链接草稿回归通过。

网页联通检查覆盖外跳目标、新标签页、返回后用途筛选和 URL 保留、复制或手动复制、四种来源入口、检查视频号链接、无视频文件保存草稿、无播放器/弹窗，以及长名称、无封面和页面无横向溢出。手机端四个来源入口采用两行两列。

App 测试验证系统外部打开调用、无播放信息请求、外跳失败后的卡片与复制兜底、能力协商和检查接口版本；中英文长名称在 360/390/768px、1.6 倍文字尺寸下无布局异常。既有抖音、本地视频及播放会话回归通过。

### 复查命令

PowerShell 中在 `server` 目录加载 `../scripts/go-env.ps1`，设置 `RUN_MYSQL_TESTS=1` 后运行：

```text
go test -v -run '^Test(VideoLink|VideoNotes|VideoRights|PlatformVideoAgreements|ProductVideo|VideoProvider|VideoUnresolved|WechatShare|KuaishouVideo|VideoGallery)' .
go vet ./...
```

先构建网页，再设置 `RUN_VIDEO_SHARE_BROWSER=1`，运行 `go test -v -count=1 -run '^TestVideoShareCardsBrowser$' .`，会启动隔离后端并调用 `scripts/verify-video-share-cards.cjs`。

网页相关文件：`video-share.test.tsx`、`video-playback.test.tsx`、`resource-dialog.test.tsx`、`video-rights.test.tsx`。App 相关文件：`video_share_cards_test.dart`、`video_links_test.dart`、`video_input_lifecycle_test.dart`、`video_rights_optional_test.dart`、`popup_video_test.dart`。截图测试为 `video_share_cards_capture_test.dart`，需设置 `VIDEO_SHARE_CAPTURE=1` 和 `APP_V18_CAPTURE=1`。

### 截图与记录

- [PC 公开卡片](screenshots/video-share-cards-20261116/cards-1440-zh.png)
- [手机公开卡片](screenshots/video-share-cards-20261116/cards-390-zh.png)
- [手机添加表单](screenshots/video-share-cards-20261116/form-390-zh.png)
- [App 手机卡片](screenshots/video-share-cards-20261116/app-cards-390-zh.png)
- [App 手机来源入口](screenshots/video-share-cards-20261116/app-sources-390-zh.png)
- [App 手机添加表单](screenshots/video-share-cards-20261116/app-form-390-zh.png)
- [浏览器自动检查记录](test-output/video-share-cards-browser-20261116.json)

完整本机日志位于 `.local/video-share-cards-20261116/`，不包含于发布产物。

## 验证边界

浏览器测试拦截了原平台页面，只验证外跳目标与本地页面行为；App 外部打开使用 Flutter 平台替身验证，截图来自 Widget 渲染。本轮未进行 Android 真机微信跳转或原视频可播放性验收。原平台可能要求登录、微信内打开，或因作者修改/删除而不可用；格式检查不承诺视频存在或可播放。没有接入收费服务，也没有原视频存储流程。
