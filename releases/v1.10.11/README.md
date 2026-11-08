# 中企展馆 Web 与 Linux 部署包 v1.10.11

企业展厅的视频栏目和首页预览显示视频封面、用途与简介。后台支持上传、替换和清除封面。已审核视频沿用现有公开版本，第三方视频按平台提供站内播放或原平台入口。

适用于现有 Linux amd64 ECS。服务端提供视频 API v3，并兼容旧客户端的本地视频与抖音数据。数据库沿用 Schema 30。Web 部署沿用现有 Android 下载目录。包内包含 77 项开发演示边界及完整性清单。

1. 解包后运行 `sha256sum -c SHA256SUMS` 和 `python3 upgrade.py check`。
2. 依次运行 `python3 upgrade.py backup`、`stage`、`candidate`。使用独立恢复库及 Nginx 代理检查播放接口、展厅 SEO 和四级地图。
3. 验收候选后运行 `python3 upgrade.py apply`。切换程序、Web 和地图路径，复验生产 HTTPS。验收失败时脚本切回上一版本，也可运行 `python3 upgrade.py rollback` 回退。
4. 从生产页面复验视频弹层、真实抖音起播和地图下钻及重试。
