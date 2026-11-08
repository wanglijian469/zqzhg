# 中企展馆 Web 与 Linux 部署包 v1.10.10

公开厂商展厅提供独立标题、摘要和 canonical。首页、目录和地区入口支持无脚本发现，根 Sitemap 直接提供公开页面 URL。厂商展厅沿用原有界面。

适用于已部署中企展馆的 Linux amd64 ECS。包内包含 Go 程序、构建后的 Web、77 项开发演示边界和升级脚本。沿用 Schema 30、现有媒体存储、加密密钥和正式 Android 下载目录。

## 升级

1. 解包并运行 `sha256sum -c SHA256SUMS` 和 `python3 upgrade.py check`。
2. 在目标 ECS 依次执行 `python3 upgrade.py backup`、`stage`、`candidate`。候选使用隔离恢复库、现有服务账号和独立 Nginx 代理。
3. 确认候选通过后执行 `python3 upgrade.py apply`。脚本切换服务程序、Web 和地图路径，通过正式 HTTPS 校验展厅、Sitemap、四级边界及 APK 清单。升级或验收失败时可执行 `python3 upgrade.py rollback`，切回上一个程序和配置。
4. 从公网复验全国→河北→邢台→宁晋的边界、统计、手机布局和资源失败重试。正式发布后的百度站点验证、Sitemap 提交和抓取诊断由站长处理。

详细规则见[展厅 SEO](../../docs/INDEPENDENT_SEO.md)和[地图验收要求](../../docs/MAP_DEPLOYMENT_REQUIREMENTS.md)。
