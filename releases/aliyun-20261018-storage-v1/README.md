# 中企展馆：阿里云部署发行包（2026-10-18）

发行标识：`aliyun-20261018-storage-v1`。此标识用于部署归档，与现有Android版本号独立。发布当前网站/后台/配套Go后端，包含迁移001—015；沿用已交付Android 1.7.0+29 APK及原签名。

下载页：[GitHub Release](https://github.com/wanglijian469/zqzhg/releases/tag/aliyun-20261018-storage-v1)。源码提交、产品SHA256及上传结果见本目录随后更新的发布清单。

## 文件

- `zqzg-aliyun-20261018-storage-v1-linux.tar.gz`：Linux amd64和arm64程序、网站静态文件、原APK下载资源、迁移、目录数据、许可证、现有服务升级脚本、运行手册及验证摘要。
- `zqzg-aliyun-20261018-storage-v1-source.zip`：对应Git提交源码，不含node_modules、编译缓存、生产环境、私钥、数据库、媒体或私有备份。Android签名密钥须由所有者自行保管。
- `SHA256SUMS-aliyun-20261018-storage-v1.txt`：发布附件校验值。
- `manifest.json`：程序架构、源码提交、Schema、APK摘要及测试边界。

较大的程序包放在Release附件，Git仓库提交源码、脚本、说明及小型版本清单。不会把历史delivery快照、业务数据库或媒体推入Git。

## 阿里云已有站点升级

本次只编译/发布，不连接ECS执行部署。支持现有`/opt/zqzt/current`管理式安装、`zqzt-exhibition.service`，沿用`/etc/zqzt/exhibition.env`。环境路径不同时设置`INSTALL_DIR`、`ENV_FILE`、`SERVICE`。

1. 上传Linux归档与SHA256文件，核对下载完整性。不要运行历史`deploy/linux/deploy.sh`进行数据库初始化。
2. 检查现有环境：`MEDIA_DIR`必须是实际持久媒体根的绝对路径；`OPERATIONS_DIR`固定在部署目录之外，保留active.json和原加密密钥。已有恢复指针时，脚本按其真实数据库/媒体位置备份；待完成恢复会阻止升级。
3. 现有`PUBLIC_BASE_URL`须设置为当前站点，例如`http://39.97.250.244:8089`；已有正式域名则保留。保留数据库、短信和签名配置。必须沿用原`CONFIG_ENCRYPTION_KEY`；从未配置且没有加密数据时才新生成32字节密钥，另行异地保管，不写进Git。可在服务器本地使用`openssl rand -hex 32`生成，勿替换已在使用的密钥。
4. 新增持久目录时按真实服务用户赋权限。建议后续目录为`/data/zqzhg/{media,temp,backup,operations}`；本次升级不自动迁移现有媒体根，NAS/OSS切换见手册。

```bash
sha256sum -c SHA256SUMS-aliyun-20261018-storage-v1.txt --ignore-missing
tar -xzf zqzg-aliyun-20261018-storage-v1-linux.tar.gz
cd zqzg-aliyun-20261018-storage-v1
sudo bash upgrade.sh --check
sudo bash upgrade.sh --apply
```

`--check`校验包和现有配置，不改服务/数据库。`--apply`先准备新release，暂停原服务，在`/opt/zqzt/shared/upgrade-backups`创建0700私有目录，备份原数据库、媒体、operations及环境，再原子切换程序。保留旧release、数据库、媒体和密钥，不重建数据库、不重置密码、不覆盖环境、不删除旧文件。需MySQL客户端、mysqldump、Python3、systemd、curl、tar及sha256sum。视频处理继续使用服务器原FFmpeg/FFprobe配置。

备份可能较大，可用`sudo env BACKUP_ROOT=/另一受保护磁盘 bash upgrade.sh --apply`选择空间充足的独立目录；不能放进媒体、operations、release或Web根。该升级快照包含敏感配置，只在服务器本地私有保存；另行异地备份，勿上传GitHub或放入公开下载目录。

健康检查失败且已发生程序切换时，脚本停止新服务并保留快照，不自动启动可能缺少展馆隔离/历史短链支持的旧后端。先检查`journalctl -u zqzt-exhibition.service`，修正配置后重试新构建；需回滚时使用兼容构建，或在维护窗口按同一时点恢复数据库、媒体、环境及operations。不要只切回旧程序或直接把数据库快照导入正在写入的业务库。

## 域名、Nginx及存储

不在升级时顺便切域名。先完成DNS/证书/Nginx，再从后台检测并保存网站/API/媒体新地址。旧IP网页跳转，新旧API及媒体继续代理同一后端。包内`deployment/nginx-storage-domain.example.conf`使用Go回环8090、Nginx旧IP8089/HTTPS443；仅在实际调整部署端口后启用，不可让两个程序绑定相同端口。

具体接口、迁移015、目录、NAS/OSS、加密备份与回滚见[存储升级手册](../../docs/STORAGE_SETTINGS_UPGRADE.md)。源码仓库的[验收记录](../../docs/STORAGE_SETTINGS_VALIDATION.md)和包内verification摘要记录已测试与未测试范围。

## 重建

源码归档不含签名APK二进制；重建下载资源时，从已校验Linux产品包的`web/dist/downloads`复制原APK到源码`web/public/downloads`。厂商原始收集/联系人授权资料、旧delivery快照已从当前Git索引移除，本地保留；既有Git历史未重写。

Go 1.26、Node.js/npm；`cd web && npm ci && npm run build`；Linux构建设置`CGO_ENABLED=0`、`GOOS=linux`，分别`GOARCH=amd64`/`arm64`执行`go build -trimpath`。新包的打包脚本为`scripts/package-aliyun-release.py`。

真实ECS/NAS/OSS、生产DNS/TLS/Nginx及Linux运行、真机安装/软键盘不在本机构建验收范围；交叉编译不等同于ECS运行通过。升级助手的隔离模拟测试也不代表已执行生产备份。
