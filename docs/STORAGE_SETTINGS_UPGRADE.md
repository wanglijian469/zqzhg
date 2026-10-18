# 中企展馆：文件存储与系统设置升级

本版本复用 Go/Gin/GORM/MySQL、React/Vite、Local/OSS、原媒体 ID、UUID 相对 key、现有加密备份和迁移任务。无 Android 改版、无生产部署、无业务文件自动删除。新部署使用本地磁盘；升级保留已有存储配置。

## 已实现

- 超级管理员“系统设置”：网站中英文名、网站/API/媒体地址；图片/视频/PDF 限制、厂商默认和独立配额、磁盘预警；路径只读、存储与备份入口。版本号防覆盖，事务保存，关键修改日志不包含密钥。
- 未保存设置时沿用环境和当前站点；保存后数据库配置优先，初始 HTML 注入公开运行时配置。API、XHR、私有图片、视频、PDF、海报和 SEO 共用地址辅助函数。更改域名无需重新编译。
- 新地址保存前检查：DNS 解析并固定连接 IP，生产环境拒绝私网、回环、链路本地及云元数据地址；禁止重定向、8 秒超时、64 KiB 响应上限、TLS 默认校验、相同站点标识；检测凭证 10 分钟有效并绑定配置版本。开发仅允许当前请求或服务器显式配置、并确实绑定本机网卡的测试地址（含回环地址）。检测不携带登录凭据。
- `/media/:id` 和旧 `/api/v1/media/:id` 共用审核/所有权、下载、HEAD、Range 和媒体限流。不对磁盘目录设置公开静态 alias。
- Local 路径只允许服务器注册标识；已有旧配置仍可读取，已有启用的旧目录保持写入兼容；改动或重新启用前必须登记并检测。NAS 使用同一适配器，标记 `require_mount` 的路径必须是实际挂载点。
- 默认图片10 MiB、视频200 MiB、PDF50 MiB、厂商2 GiB；后台限制不能超过环境硬上限。保留签名、解码、像素、视频时长、PDF、路径和归属校验。图片原有压缩和缩略图继续生效。
- 厂商所有权库存包含原图、缩略图、视频源与结果、PDF；同 key 迁移副本只记一次。账号行锁串行化原子预留和结算，失败释放，转码结果再次结算。历史超额内容继续可读，仅阻止新增。平台管理员自有首页图片不计入厂商；代录使用原厂商归属。
- 临时目录与当前本地目标都检查剩余空间；默认80%/90%预警，不足1 GiB或操作需要的空间拒绝写入。OSS 登记对象逻辑用量可查看，不伪造桶总容量。
- 递归库存扫描；迁移保持 key，复制后大小和 SHA256 双校验、保存旧副本登记，然后切换指针；中断重试不覆盖不匹配目标。读取/转码/备份共用校验副本解析；损坏副本拒绝，成功校验缓存10分钟（大小/修改时间变化失效），避免每个 Range 全文件哈希。
- 加密备份保留既有策略；未配置预设为02:00、7份、禁用，检测后管理员启用。目录位置快照保留；恢复重建所有权库存，清除旧机器副本位置和上传预留。旧默认备份目录仍有回退读取。
- 删除表单图片仅解除引用。新增超级管理员明确确认的未引用媒体删除接口，检查所有修订/资源/首页引用及处理状态；已引用文件拒绝删除。旧迁移副本清理前必须验证当前副本，不自动清理历史文件。

## Schema15 与兼容接口

`server/migrations/015_storage_settings.sql` 增加 `storage_profiles.root_key` 及 `stored_object_replicas`、`media_object_owners`、`merchant_storage_quotas`、`media_upload_reservations`。沿用原 `system_settings` 保存版本化配置，不改媒体 key，不改业务厂商/产品字段。启动迁移可重试，不删除旧表或旧索引。备份恢复接受至Schema15。

| 方法/路径 | 权限/用途 |
| --- | --- |
| GET `/api/v1/public/config` | 公开品牌、地址、限制、版本、站点标识；无目录和密钥 |
| GET/PUT `/api/v1/admin/settings` | 超级管理员读写；PUT携带version，地址变化附检测返回的probe_token |
| POST `/api/v1/admin/settings/test-addresses` | 超级管理员；检测完整配置；失败不保存 |
| GET `/api/v1/admin/storage/status` | 超级管理员；路径、磁盘、登记用量、副本降级统计、备份策略 |
| GET `/api/v1/merchant/storage/usage` | 厂商本人；used_bytes/reserved_bytes/quota_bytes |
| GET `/api/v1/admin/storage/quotas?page=1&page_size=20` | 超级管理员分页查看厂商用量 |
| PUT `/api/v1/admin/storage/quotas/:account_id` | 超级管理员；quota_bytes整数；null恢复默认；0停止新增 |
| DELETE `/api/v1/admin/media/:id` | 超级管理员，JSON `{"confirm":true}`；引用或处理中返回409 |
| GET/HEAD `/media/:id` | 与原媒体路径相同权限；可用thumb/download及Range |
| GET `/media/status` | 地址检测站点标识，不含存储状态 |

原网页/API、媒体ID和旧APK接口继续可用。`system.read` 操作员仍访问原系统状态，不获得配置写入、密钥或厂商配额管理权限。没有新建全量导出入口。

## ECS 配置与部署

先完整备份数据库、媒体、环境、旧程序/静态目录、OPERATIONS_DIR（含active.json、任务、备份目录清单）。加密密钥另存其他设备。当前磁盘里的备份不能防单盘损坏；下载备份并异地保存，密钥与备份分开保管。

建议服务器环境：

```dotenv
MEDIA_DIR=/data/zqzhg/media
MEDIA_TEMP_DIR=/data/zqzhg/temp
BACKUP_DIR=/data/zqzhg/backup
OPERATIONS_DIR=/data/zqzhg/operations
HTTP_ADDR=127.0.0.1:8090
IMAGE_UPLOAD_HARD_MAX_MIB=20
VIDEO_UPLOAD_HARD_MAX_MIB=1024
PDF_UPLOAD_HARD_MAX_MIB=200
LOCAL_STORAGE_ROOTS_JSON=[]
MEDIA_REQUIRE_MOUNT=false
```

不要覆盖原DB、签名、短信、管理员及 CONFIG_ENCRYPTION_KEY 配置。`OPERATIONS_DIR`必须稳定独立，不能因MEDIA_DIR变化重新生成一套控制状态。新目录在部署目录/Web根外；生产启动会拒绝部署目录内部的存储路径。

1. 在原后台存储页面确认实际默认目标、各文件位置；检查当前 `OPERATIONS_DIR/active.json` 的有效数据库和媒体根。存在待完成恢复时先处理，不能只看环境MEDIA_DIR。
2. 停止写入及服务，再用复制校验脚本；默认只检查，`--copy`才复制，不覆盖不同内容：

```sh
python3 scripts/storage-copy-verify.py --source /实际旧媒体目录 --target /data/zqzhg/media --operations /实际稳定operations目录 --copy > /安全位置/media-copy-report.json
```

3. 保留源目录。若active.json有效媒体根与环境不同，不直接覆盖控制文件；优先通过现有后台迁移到服务器新登记目录。离线更改指针须备份并保持原数据库、previous、pending状态，确认完整副本后按原恢复机制处理。脚本不自动改指针。
4. 保留原发布目录，安装配套Linux程序、dist、migrations、catalog、licenses到新release；服务仍读取原 `/etc/zqzt/exhibition.env`。先隔离验证自动迁移015，再更新 `current` 链接、启动原服务。
5. `sudo systemctl restart zqzt-exhibition.service`；检查 `curl --fail http://127.0.0.1:8090/api/v1/health`，抽查历史图片、企业、产品、登录、视频Range和PDF。
6. 目录权限和挂载依赖见 `deploy/linux/storage-prepare.example.sh`；按真实服务账号调整，禁止给目录777权限。

本地更新：`cd web; npm run build`，回到根目录运行 `./scripts/stop.ps1` 后 `./scripts/start.ps1`。数据库启动时自动执行兼容迁移。

## 域名切换（不修改企业自身官网）

当前生产地址可在后台填写：网站 `http://39.97.250.244:8089`，API末尾`/api/v1`，媒体末尾`/media`。本地默认仍指当前本机站点，不预置云IP。

运维先完成DNS、HTTPS证书、Nginx代理，再从旧站点后台检测并保存三项新地址。示例 `deploy/linux/nginx-storage-domain.example.conf`：Go使用8090回环，Nginx提供旧IP8089和新HTTPS443，避免端口冲突。可信代理只配置真实Nginx地址。`nginx -t`通过再reload。

新网站/API/媒体建议都为 `https://zqzhg.com` 同源，后两项分别加`/api/v1`和`/media`。如使用独立API/媒体域名，也须通过同站检测、代理正确处理Origin/CORS/Authorization和Range；检测成功后才保存，不向未确认站点发送登录凭据。旧网页GET/HEAD跳新域名；旧`/api/v1/`和`/media/`继续反向代理同一后端，不跳转认证请求。首次访问新域名重新登录，不用URL传递token。

## NAS / OSS

- NAS：挂载实际目录，复制校验；在服务器 `LOCAL_STORAGE_ROOTS_JSON`登记，例如 `[{"key":"nas","name":"NAS","path":"/mnt/zqzg-media","require_mount":true}]`。该目录必须是挂载点；配置systemd RequiresMountsFor及网络依赖。后台选择nas、检测、运行原迁移任务，验证后启用新上传目标；旧副本留存。挂载缺失拒绝写入，不退回写入空挂载目录。
- OSS：创建私有桶，后台输入Region/Endpoint/Bucket/Prefix和凭据；凭据服务端加密，前端不回显。检测私有权限及读写，运行原迁移复制/校验任务，再启用。key相对路径不变，历史内容继续按媒体ID读取。OSS总/剩余容量无法获取时不显示虚假值。真实桶与网络验收需要实际凭据。
- 已验证旧副本回退会增加降级计数；存储页应处理故障，不能把回退当作备份。校验缓存不替代磁盘备份。

## 回滚

保留旧网站包及APK。本次索引/表为非破坏性，可保留；未启用新设置时可恢复先前配套版本。启用新目录标识、已迁移位置或新地址后，应使用支持Schema15和副本解析的兼容构建；不能直接换回忽略root_key/配额/动态地址的旧后端。必要时停止写入并恢复同一时点数据库、媒体、配置、operations控制与静态资源；不得只恢复数据库或删除新表。

## 安全依据与边界

实现继续采用文件白名单、内容解码/校验、服务端唯一key、Web根外存储、权限校验；参考[OWASP文件上传指导](https://cheatsheetseries.owasp.org/cheatsheets/File_Upload_Cheat_Sheet.html)。地址检测采用目标验证、DNS解析固定及禁止重定向，参考[OWASP SSRF指导](https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html)。站点标识用于部署一致性校验，不是第三方域名所有权认证。

本版本数据库预留以单实例服务为运行基线；多实例需要共享操作控制及聚合访问限制，不能直接并行使用各自operations目录。未接入杀毒或CDR服务，不声称能识别所有恶意内容。

## 配套包安装目录示例（运维执行，不是本次已部署）

解压本交付包至`/srv/zqzg-package`后，以下路径示例需改成新的、未使用的release名称；旧release保留。先完成前述备份、隔离迁移验证和持久目录准备。

```sh
pkg=/srv/zqzg-package/storage-settings-20261018
release=/opt/zqzt/releases/storage-settings-20261018
# 确认release不存在；避免覆盖已有发布。
test ! -e "$release" || exit 1
install -d "$release/server" "$release/web"
install -m 0755 "$pkg/bin/exhibition-linux-amd64" "$release/server/exhibition"
cp -a "$pkg/migrations" "$pkg/catalog" "$pkg/licenses" "$release/server/"
cp -a "$pkg/dist" "$release/web/dist"
# 保持原服务账号、环境文件、MySQL客户端、FFmpeg/FFprobe配置。
# 确认当前current确为预期旧版本，再由运维切换：
systemctl stop zqzt-exhibition.service
ln -sfn "$release" /opt/zqzt/current
systemctl start zqzt-exhibition.service
curl --fail http://127.0.0.1:8090/api/v1/health
# 检查Nginx配置后才reload；不可重复绑定Go与Nginx的8089端口。
nginx -t && systemctl reload nginx
```

不复制包内测试数据到业务库。包内不包含生产`.env`、数据库备份、媒体目录或加密密钥。启动会自动运行兼容迁移，迁移失败时保留当前数据并检查服务日志，不反复切换数据库或重置管理员密码。
