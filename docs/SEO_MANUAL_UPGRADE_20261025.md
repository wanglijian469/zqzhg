# SEO 手动升级说明（2026-10-25）

本包由你手动上传、执行，不自动连接或修改生产服务器。当前入口为 `http://39.97.250.244:8089/`。

## 本包内容和边界

- 程序包：`zqzg-seo-20261025-linux.tar.gz`，支持 Linux amd64、arm64，包含程序、网站和现有 Schema 1–21 迁移材料。
- 校验文件：同名 `.sha256`；完整文件清单在包内 `SHA256SUMS`。
- 修复首页、企业名单、产品及厂商展厅的 SEO 标题、描述和关键词；增加可抓取的首页说明及 `/sitemaps/pages.xml`。
- 首页／名单覆盖“河北内卷网、内卷网、不卷网、企业名单、企业展厅”；展厅使用企业自己的公开名称、简称、主营产品和地区。不改变现有品牌，不伪造别名。
- 保留产品、厂家、采购功能和展厅稳定切换。包含此前工作区已有修改，是当前整体程序和网站更新，不是仅替换某个文件的补丁。
- **不包含新 APK，也不覆盖正式下载。** 预检读取现网下载目录；升级时复制该目录并逐文件比较 SHA-256，发现变化则在切换前中止。
- 不携带业务数据库快照、生产配置或密钥，不导入演示数据。本轮 SEO 没有新增迁移；现网低于 Schema21 时，原有启动流程仍会执行此前已存在的缺失迁移。

## 1. 上传并校验

将 `.tar.gz` 和 `.tar.gz.sha256` 两个文件上传至服务器 `/root/`。可通过你现有的阿里云远程连接、SFTP 或其他上传工具操作。以下命令均在**服务器 Linux 终端**执行：

```bash
sudo -i
cd /root
sha256sum -c zqzg-seo-20261025-linux.tar.gz.sha256 && \
tar -xzf zqzg-seo-20261025-linux.tar.gz && \
cd zqzg-seo-20261025 && \
bash upgrade.sh --check
```

预检只读检查包内校验值、CPU、生产配置、持久存储路径和原下载文件，不停止服务，不改数据库。看到 `Preflight passed` 后再进行下一步；任何错误都应停止，不跳过校验。

适用既有托管安装：`/opt/zqzt/current` 为版本符号链接，配置为 `/etc/zqzt/exhibition.env`，服务为 `zqzt-exhibition.service`。不同路径需通过 `INSTALL_DIR`、`ENV_FILE`、`SERVICE` 指定实际值，预检和升级使用相同参数。

不要用示例配置覆盖原 `.env`。保留正在使用的数据库、媒体、运维目录和原加密密钥。已有待完成的数据恢复／切换、下载目录含符号链接或配置不符合预期时，预检会拒绝继续，先核对实际安装。

## 2. 正式升级

```bash
cd /root/zqzg-seo-20261025
bash upgrade.sh --apply
```

流程为：取得升级锁 → 准备独立版本目录 → 保留原正式下载并核对哈希 → 暂停服务 → 备份当前数据库、主媒体、运维控制状态及环境配置 → 切换程序 → 启动并做健康检查。

默认备份保存在 `/opt/zqzt/shared/upgrade-backups/`，需要足够磁盘空间；OSS 或额外注册的存储根仍需使用原来的存储侧备份方案。备份含私密配置，不放到公开网页目录。

自定义安装示例（两步均传入同一组实际路径）：

```bash
env INSTALL_DIR=/opt/zqzt ENV_FILE=/etc/zqzt/exhibition.env \
    SERVICE=zqzt-exhibition.service BACKUP_ROOT=/data/private-upgrade-backups \
    HEALTH_TIMEOUT=300 bash upgrade.sh --check

env INSTALL_DIR=/opt/zqzt ENV_FILE=/etc/zqzt/exhibition.env \
    SERVICE=zqzt-exhibition.service BACKUP_ROOT=/data/private-upgrade-backups \
    HEALTH_TIMEOUT=300 bash upgrade.sh --apply
```

不手工导入 SQL，不运行旧版首次部署脚本或本机业务数据导入脚本。脚本不会更换数据库、密钥、媒体目录或网站地址。

## 3. 升级后验证

```bash
systemctl status zqzt-exhibition.service --no-pager
curl --noproxy '*' -fsS http://127.0.0.1:8089/api/v1/health
curl --noproxy '*' -fsS http://127.0.0.1:8089/ | grep -o '<title>[^<]*</title>\|<meta name=.keywords.[^>]*>'
curl --noproxy '*' -fsS http://127.0.0.1:8089/sitemap.xml
curl --noproxy '*' -fsS http://127.0.0.1:8089/sitemaps/pages.xml
curl --noproxy '*' -fI http://127.0.0.1:8089/downloads/dalunongji.apk
```

端口以原配置为准。首页标题应出现“企业展厅与产品展示”，关键词包含“内卷网”，站点地图应包含 `/sitemaps/pages.xml`；原 APK 应仍可下载。浏览器再检查厂家／产品搜索、收藏、联系厂家、采购入口和展厅六个栏目切换。

本包已通过 246 项 Web 测试、类型检查、相关 Go 测试、Go vet，以及 16 组浏览器配置；首页和名单覆盖 360／390／430／1366／1920px 与中英文。新增 6 项下载保留测试、10 项升级工具测试、4 项包校验测试及 Bash 语法检查。双架构 ELF 和归档逐文件哈希已核验。未在真实 Linux/systemd 环境执行升级，不把本地测试当作生产验收。

## 4. 域名与搜索收录

先保持现有 IP 地址可用，不要在域名尚未解析、HTTPS 尚未生效时提前修改网站地址。域名准备好后，将后台网站地址设置为 `https://zqzhg.com`，核验 canonical 和 sitemap 使用同一正式地址，再在已验证所有权的搜索平台提交首页及站点地图。

搜索验收仍为**未通过**。程序升级、关键词输出或站点地图提交成功均不等于已经收录。需直接搜索“内卷网”，看到指向本站的结果，点击能正常访问，并保存日期、结果位置、链接及截图后才能通过。

## 5. 失败处理与回退

程序切换前失败，原服务保持运行或尝试重新启动；程序切换后若新服务未通过健康检查，脚本会停止新服务并保留旧程序和备份，不盲目启动可能不兼容的旧程序。

```bash
systemctl status zqzt-exhibition.service --no-pager
journalctl -u zqzt-exhibition.service -n 100 --no-pager
ls -ld /opt/zqzt/current /opt/zqzt/shared/upgrade-backups/*
```

保留错误日志、输出的旧／新版本路径和备份位置。回退前核对该批备份的 `SHA256SUMS`，由数据库管理员恢复成套数据库、媒体、运维指针、配置和原密钥，再切换对应旧程序。已有新业务写入时先另做备份并处理差异；只改 `current` 链接不等于数据库回滚，不直接清空现有数据库。
