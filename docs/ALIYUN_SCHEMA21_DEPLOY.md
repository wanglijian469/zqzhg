# 阿里云 Linux 升级包 · Schema21 · 2026-10-24

本次按确认选择执行：**保留线上数据库与媒体，只升级程序、网站及数据库结构**。不导入本机数据库，不创建演示采购数据，不替换线上账户或密码。本轮只生成文件及部署命令，没有登录 ECS 或部署生产。


## 已上传原包：只修复校验清单换行

首次 `schema21` 包整体校验正确，但内置 `SHA256SUMS` 使用 CRLF，导致 Linux 把 `\r` 当成文件名。`fix1` 已修复打包生成与归档验收，并新增字节级 LF 回归测试。原包未替换，避免已公布的 SHA-256 失效。

如果已经上传原包并且外层 SHA-256 已通过（`ac9915be6967ce7b17d9b6f88308123b1e33f2042b7e62eb5a6a19bda5ce3de0`），无需重新上传，在服务器执行：

```bash
cd /opt/zqzg-aliyun-20261024-schema21 && \
sed -i 's/\r$//' SHA256SUMS && \
sha256sum --check --quiet SHA256SUMS && \
bash upgrade.sh --check
```

这只转换校验清单换行，不修改程序、其他文件或线上数据，也不跳过任何内容校验。显示 `Preflight passed` 后才执行 `bash /opt/zqzg-aliyun-20261024-schema21/upgrade.sh --apply`。若其他步骤报错，停止处理该错误。

## 本次 Fix2 后端修复

Fix2 修复厂商展厅分类统计在 MySQL ONLY_FULL_GROUP_BY 下触发的 1055 错误，Schema 保持 21，网站及 Android 文件沿用原版本。严格模式测试、/opt 更新与验收步骤见 [专项说明](SHOWROOM_MYSQL1055_FIX2.md)。

## 文件和数据范围

| 文件 | 用途 |
| --- | --- |
| `zqzg-aliyun-20261024-schema21-fix2-linux.tar.gz` | Linux amd64/arm64 程序、最新网站、Android 1.8.0+35、升级脚本、迁移文件、验证摘要 |
| 同名 `.sha256` | 上传后校验整个程序包 |
| `zqzg-aliyun-20261024-schema21-fix2-database.zip` | 001–021 数据库迁移、地区目录和许可证，供核对/归档；**不是业务 SQL 快照，不需要手动导入** |
| `zqzg-aliyun-20261024-schema21-fix2-source.zip` | 本次当前工作区的服务端、Web 与部署源码快照，含文件哈希；不是旧 Git HEAD 的替代冒充 |

程序包不包含业务数据库、上传媒体、登录会话、验证码、生产配置、阿里云密钥或 Android 签名私钥。实际线上数据在部署时原地保留，并先生成私有备份。

本包包含采购询比价、采购中心/厂商工作台调整、独立展厅/产品 SEO、第一/二阶段质量整改及 Android 1.8.0 配套接口。Android 下载会在部署后的 `/downloads/dalunongji.apk` 更新为最新包；此次打包没有改动本机原稳定下载副本。

## 适用环境和升级前确认

适用于现有 systemd 管理的 Linux ECS 安装，默认：

```text
安装根目录       /opt/zqzt
当前版本链接     /opt/zqzt/current
配置文件         /etc/zqzt/exhibition.env
服务名           zqzt-exhibition.service
网站             http://39.97.250.244:8089
```

CPU 支持 x86_64/amd64、aarch64/arm64，无需在 ECS 安装 Go、Node 或 Flutter。程序为 CGO_DISABLED 静态 Linux ELF。需要现有 MySQL、`mysql`/`mysqldump` 客户端、Python3（兼容 Python3.6 语法）、bash、systemd、curl、tar、sha256sum、util-linux/flock。保留现有视频服务的 FFmpeg/FFprobe；本包不重新安装数据库或修改防火墙、Nginx和安全组。

检查 `/etc/zqzt/exhibition.env` 的**原配置**，不要使用本机 `.env` 或示例整体覆盖它。不要把文件内容贴到聊天或日志中：

```text
APP_ENV=production
SMS_PROVIDER=disabled                 # 未开通短信时保持 disabled；已开通保留 aliyun
PROCUREMENT_DEMO=false
PROCUREMENT_ENABLED=true             # 缺省为 true；原来明确 false 时脚本会保留
WEB_DIR=/opt/zqzt/current/web/dist
PUBLIC_BASE_URL=http://39.97.250.244:8089
```

上述行是要核对的值，**不是直接追加命令**。实际站点已使用域名、HTTPS、其他安装路径或停用了采购功能时，应保留实际配置。`DB_*`、`MEDIA_DIR`、`OPERATIONS_DIR`、`CONFIG_ENCRYPTION_KEY`、`LOCAL_STORAGE_ROOTS_JSON`、存储及短信凭据保留原值。密钥必须是原来的有效 32 字节密钥，不可盲目重新生成；已存在恢复指针时，会备份并使用其实际数据库和媒体目录。

预检若提示缺少稳定运维目录或有效密钥，先按原部署记录恢复并核实；脚本不会猜测新目录或替换旧密钥。仅能明确证明为 Schema1–9、尚未存在加密存储/运维状态的旧系统，可在核验后使用包内 `deployment/initialize-legacy-key.py`；它拒绝替换已有密钥，不能用于 Schema10 以上丢失密钥的系统。

确保备份磁盘容量足够同时保存数据库、主媒体、运维目录和新版本。默认备份路径是 `/opt/zqzt/shared/upgrade-backups/`，可用 `BACKUP_ROOT` 指定另一私有磁盘。脚本覆盖有效数据库、主媒体、运维控制状态及环境文件；**OSS 或额外注册的 NAS/本地存储根需另做存储侧快照/版本备份**。本包不迁移或删除这些存储对象。

## 1. 本机上传

在 Windows PowerShell 执行，首次 SSH 连接请核对服务器主机指纹：

```powershell
scp "D:\workspace-main\codex-zqzt\delivery\zqzg-aliyun-20261024-schema21-fix2-linux.tar.gz" "D:\workspace-main\codex-zqzt\delivery\zqzg-aliyun-20261024-schema21-fix2-linux.tar.gz.sha256" root@39.97.250.244:/root/
ssh root@39.97.250.244
```

只需上传程序 tar.gz 和其校验文件。数据库 zip、源码 zip 不必上传，不要上传任何本机 `.env`、数据库快照或演示库。

## 2. ECS 校验与只读预检

以下命令在 ECS 的同一个 SSH 终端中执行，任何一步失败都应停止排查，不跳过校验：

```bash
sudo -i
cd /root
sha256sum -c zqzg-aliyun-20261024-schema21-fix2-linux.tar.gz.sha256 && \
tar -xzf zqzg-aliyun-20261024-schema21-fix2-linux.tar.gz && \
cd zqzg-aliyun-20261024-schema21-fix2 && \
bash upgrade.sh --check
```

`--check` 只读核对程序校验值、CPU、配置、存储路径和运行模式，不停止服务、不写数据库。数据库连接、备份权限和 Schema 版本会在正式升级的备份阶段核验，失败则不会切换程序。

## 3. 正式升级

```bash
cd /root/zqzg-aliyun-20261024-schema21-fix2
bash upgrade.sh --apply
```

脚本按顺序：取得升级锁；准备独立版本目录；暂停服务；备份现有数据库、主媒体、运维状态、环境配置及校验清单；切换 `/opt/zqzt/current`；启动同一服务。新后端自动执行尚未应用的迁移至 Schema21，健康检查还核验新接口是否具有采购能力字段。启动检查默认最多重试 180 次，可按实际服务器性能设置 `HEALTH_TIMEOUT=300`（每次网络请求还有独立超时）。

安装路径或备份盘不同，显式传入实际值，例如：

```bash
env INSTALL_DIR=/opt/zqzt \
    ENV_FILE=/etc/zqzt/exhibition.env \
    SERVICE=zqzt-exhibition.service \
    BACKUP_ROOT=/data/private-upgrade-backups \
    HEALTH_TIMEOUT=300 \
    bash upgrade.sh --apply
```

`BACKUP_ROOT` 必须是私有目录，且位于媒体、运维及版本目录之外；不要设为网站目录。备份包含数据库密码和旧密钥，禁止公开下载。默认 umask 077。

不需要运行 `mysql < 021_procurement.sql`，也不要再次执行旧版 `deploy.sh` 或 `import-local-data.py`。迁移由当前后端统一处理，SQL 文件是同版本审计材料。

## 数据结构变化和现有资料

- Schema21 新增 14 张采购业务表：采购商资料、项目、需求版本、明细、审核、参与、报价及版本/明细、会话、附件、事件、通知和订阅。
- 若线上已是 Schema20，仅执行 021；若更旧，按缺少的版本顺序执行，不重复运行已记录的迁移。拒绝升级程序已知版本上限之外的数据库。
- Schema16–20 包含地区快照、农业分类兼容映射、手机号账号和图片水印等既有功能迁移；从更旧版本升级时需要重点核对分类映射及账号兼容。它们是项目已有迁移，不是本机业务数据覆盖线上。
- 不初始化演示账号、询价或报价，不自动改写厂商产品的封面、名称、型号或已有采购记录。
- 不更换正在使用的数据库名称、媒体目录、密钥或活跃数据指针，不清理历史数据。

## 4. 上线后检查

```bash
systemctl status zqzt-exhibition.service --no-pager
curl --noproxy '*' -fsS http://127.0.0.1:8089/api/v1/health
curl --noproxy '*' -fsS http://127.0.0.1:8089/api/v1/public/config | \
  python3 -c 'import json,sys; c=json.load(sys.stdin); print("procurement:",c.get("capabilities",{}).get("procurement"))'
curl --noproxy '*' -fI http://127.0.0.1:8089/downloads/dalunongji.apk
journalctl -u zqzt-exhibition.service -n 80 --no-pager
```

HTTP 端口以原配置为准。采购开关启用时 `procurement: True`；若保留 `PROCUREMENT_ENABLED=false`，会返回 False，这不是启用状态。原配置仍有域名或后端网站配置时，沿用现有站点实际地址。

浏览器依次检查首页、厂商和产品、已有媒体、公司账号登录、采购需求大厅 `/procurement`、采购中心 `/buyer`、厂商商机 `/merchant/opportunities`、后台审核 `/admin/procurement/reviews`、SEO `/sitemap.xml`。超级管理员可审核；操作员需具有采购审核权限。仅检查现有数据，不在真实业务库发布测试采购或报价。

更新 Android 从 `/app` 或 `/downloads/dalunongji.apk` 下载，名称“大陆农机”，版本 `1.8.0+35`、原包名原签名。默认仍连接同一服务器；不先卸载旧应用。未连接真机，本轮未验证设备安装、相机与系统文件选择器。

短信尚未开通时保持 `SMS_PROVIDER=disabled`，密码及公司账号登录仍可用。正式验证码登录须由部署负责人配置 `SMS_PROVIDER=aliyun`、`ALIYUN_ACCESS_KEY_ID`、`ALIYUN_ACCESS_KEY_SECRET`、`ALIYUN_SMS_SIGN_NAME`、`ALIYUN_SMS_TEMPLATE_CODE` 后重启服务。生产环境不显示模拟验证码、不接受演示万能码；不要把密钥写入命令或发布包。

## 失败和回滚

升级前备份位置及旧/新程序路径会显示在终端。程序切换前失败，脚本会尝试启动原服务；切换后启动或健康检查失败，脚本停止新服务并保留现场，**不会盲目启动缺少新数据隔离能力的旧程序**。

先保留错误和备份，并只读查看：

```bash
systemctl status zqzt-exhibition.service --no-pager
journalctl -u zqzt-exhibition.service -n 100 --no-pager
ls -ld /opt/zqzt/current /opt/zqzt/shared/upgrade-backups/*
```

确需回滚时暂停写入，核验该批备份 `SHA256SUMS`，由数据库管理员恢复**成套数据库、媒体、运维指针、环境及原密钥**，再切回对应旧程序；恢复目标优先使用新隔离数据库/目录，核验后切换，不直接清空正在使用的库。已有新写入时先备份并处理差异。只改 `current` 符号链接不等于数据库回滚，MySQL DDL 也不是整个事务自动回滚。

## 本次验证边界

本机完成两个架构的静态 ELF 构建、Web 生产构建/类型检查/自动化测试、Go vet 和隔离 MySQL 全量测试、升级辅助工具单测、bash 语法检查、归档逐文件 SHA-256 校验。确切结果见包内 `manifest.json` 与 `verification/`。内置 APK 沿用上一轮已完成签名与元数据核验的文件，不重新签名。

本机是 Windows，未在 ECS/Linux 实机执行服务升级、systemd、Nginx、OSS、真实短信或 Android 真机测试。打包中的 `vcs.modified=true` 如实标识当前工作区包含尚未提交的本次整改；当前源码快照和逐文件哈希随包保存，不把旧 Git HEAD 宣称为所有修改的完整版本。本轮不推送仓库、不自动操作生产。
