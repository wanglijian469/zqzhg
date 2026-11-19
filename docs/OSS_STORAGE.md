# 企业 OSS 文件存储

本次只改文件存储，继续使用现有 MySQL、Media ID、企业／产品版本及审核流程。默认 `OSS_ENABLED=false`。域名 `upload.zqzhg.com` 已由管理员配置；域名可访问不等于上传、分片、跨域及公开策略均已验证。

当前进度见 [2026-11-19 真实云端验证](OSS_ACCEPTANCE_20261119.md)，本机基线见 [测试结果](OSS_TEST_RESULTS.md)；上线前按 [候选部署与回滚](OSS_CANDIDATE_DEPLOYMENT.md) 执行。分片 RAM 权限、平台与企业公开前缀、非公开路径访问限制及完整合成云端流程均已通过；正式站点尚未切换，不能标为完整上线验收通过。

## 部署与权限

1. 先安装 schema 039 和支持 OSS 映射读取的新程序，保留全部媒体、旧版本及备份。开启前先运行 `exhibition oss-preflight-read`；该命令不连接数据库、不创建对象、不修改权限。2026-11-19 管理员更新策略后，两个业务 Bucket 的分片列表预检均已通过；本轮未安装到正式库。
2. 管理员合并 [网站角色策略](../deploy/oss/website-role-policy.json) 到 `ZQZHG-SMS-Role`，保留原短信权限。网站只访问北京区两个业务 Bucket；不得给它备份 Bucket 权限。GetObject 覆盖 HeadObject；CopyObject 使用源 GetObject 和目标 PutObject 权限；不要添加不存在的 `oss:HeadObject` 或 `oss:CopyObject` Action。
3. 私有 Bucket ACL 保持 private。公开 Bucket ACL 也保持 private，仅审核后目录允许匿名 GetObject，见 [公开目录策略](../deploy/oss/public-prefix-policy.json)。管理员应先检查账户和 Bucket 的阻止公共访问设置：账户级阻止会覆盖 Bucket 策略。不得为本功能擅自关闭整个账户的保护。
4. 根据 [跨域字段](../deploy/oss/cors-console-values.json) 配置精确生产来源。不要允许 `*`、`null` 或任意测试站点。浏览器分片由后端完成初始化、列举和合并，浏览器只 PUT 指定分片；不需要浏览器 POST Bucket。
5. 后端填入 [环境变量示例](../server/.env.example) 的 OSS 项。使用 SDK V2、V4 签名、ECS 实例角色自动续期；不配置长期 OSS AccessKey。默认私有域名 `https://upload.zqzhg.com`、公开域名 `https://img.zqzhg.com`，后台服务使用北京内网 Endpoint。
6. 新程序启动后，在管理员“存储配置→企业文件 OSS”明确执行检查。检查会创建随机合成文件，验证签名、私有匿名拒绝、GET Range、CORS 暴露头、分片及删除；勾选公开探测后同时验证平台与企业公开目录可读、公开 Bucket 临时目录不可匿名读。没有企业记录时使用保留的 0 号合成路径探测，不创建企业或业务记录。仅在这些检查成功后启用对应能力。检查不更改云策略，也不上传真实企业资料。
7. 独立开关：`OSS_UPLOADS_ENABLED=false` 禁止新上传会话和新 AI 导入，允许在途校验及既有读取；`OSS_DIRECT_UPLOAD_ENABLED=false` 关闭新浏览器直传。`OSS_ENABLED=false` 停止 OSS worker、保持已有映射读取；曾启用 OSS 后不会静默恢复本地上传。

配置首次生效将默认额度调整为 **300 MiB / 314,572,800 字节**，逐企业覆盖值保持不变，并发布存储说明的新协议版本，历史正文和接受记录保留。

## 上传、访问和配额

- 企业后台／Android 先创建上传会话，服务端决定真实归属和唯一对象键。新企业尚无 ID 时使用账户私有暂存目录，企业真实建立后更新归属元数据。厂商提供的企业 ID 不能越权。
- 单文件 PUT 授权默认 300 秒，绑定精确键、方法、Content-Type、Content-Length、禁止覆盖。SDK 按 CNAME 签名，不替换签名后的域名。浏览器让 UA 根据 Blob 设置长度，不自行写禁止设置的 Content-Length 头；OSS 请求不携带网站 Bearer。
- 32 MiB 起分片，8 MiB 每片，最多两片并行；服务器校验实际分片长度和顺序后合并。客户端仅保存会话 ID、幂等标识和文件指纹，重新选择同一文件可恢复；签名 URL 不持久保存。已成功上传但响应丢失时由服务端 HEAD 恢复，避免覆盖同一对象。
- 原件验证扩展名、MIME、真实解码、字节数、图片像素、PDF 加密及页数。AI 默认 30,000,000 字节／100 页，普通资源继续沿用网站配置。HTML、SVG 不进入图片库。
- 照片规范方向后生成展示图及缩略图；明确的产品／企业照片及资源封面优先 WebP。水印继续使用已有流程。技术图纸、参数截图和 AI 原始文件保留原像素，不套照片水印、不重绘。原件仅在无业务需要、无引用、授权已失效并真实删除确认后回收。
- 新视频接受 H.264、yuv420p、AAC 或无音轨，沿用时长和像素限制；兼容 MP4 直接使用，必要时仅 `-c copy +faststart` 封装并生成封面。不做重编码。Android 新文件保存使用原生小缓冲流，重新校验归属并在选择目标后获取短时地址，避免把大画册转为 Base64。MOV 或非 faststart 文件的临时处理副本也占实际容量，额度不足会明确失败；不会降低清晰度以绕过配额。
- 企业额度包含私有展示图、必要原件、缩略图、视频、PDF 和独立公开副本；同物理对象只计一次，外链视频不计文件容量。达到 80% 提醒；用满后停止新增，仍能删除未使用文件。签名尚有效的原始上传、失败副本和未确认删除对象保留预占，不提前释放。
- 私有访问先验证登录和权限，再返回短时签名地址；范围读取不经过 ECS 转发。公开访问保留原网站 Media URL，重定向到永久公开地址。已有本地媒体及外链视频仍可读取。

需要 `ffmpeg`（含 libwebp）、`ffprobe`，AI PDF 还需 Poppler；使用既有路径配置。临时空间预算默认 512 MiB，文件校验和视频处理串行，沿用磁盘最低保留空间（生产建议至少 1 GiB）。队列、租约和检查点持久化在 MySQL，采用数据库命名锁限制跨进程处理，并服从维护／备份写锁。

## 平台审核和公开副本

厂商提交仍进入现有审核。平台批准后创建文件发布任务，校验、复制并验证所有公开文件的 SHA-256，最后在事务中按版本／epoch 切换公开指针。失败保留旧公开版本，厂商和管理员均可查看并重试；不重新创建产品。更新、撤销或下架导致版本失效时任务停止。共享图片只在全部公开引用消失后回收公开副本；数据库检查失败或存在备份保护时不删除。

## API

厂商前缀 `/api/v1/merchant/files`；管理员同能力前缀 `/api/v1/admin/storage/files`，管理员沿用最高权限限制。

| 方法和后缀 | 用途 |
|---|---|
| POST `/uploads` | 幂等创建会话，检查归属并预占容量 |
| GET `/uploads/:id` | 状态、校验结果和最终 Media ID |
| POST `/uploads/:id/authorization` | 刷新授权及查询已完成分片 |
| POST `/uploads/:id/parts` | 一次最多两片精确授权 |
| POST `/uploads/:id/complete` | HEAD／分片校验并交给后台验证 |
| POST `/uploads/:id/cancel` | 取消，失效授权及真实清理后释放容量 |
| POST `/uploads/:id/retry` | 重试仍保有原件的失败校验 |
| GET `/`、`/:id` | 企业文件列表和元数据 |
| GET `/:id/access` | 私有签名读取，`thumb=1` 选择缩略图 |
| DELETE `/:id` | `{confirm:true}` 删除无引用文件，保护配额和备份 |

其他接口：

- `GET /api/v1/admin/storage/oss`、`POST .../oss/check`：配置摘要和明确触发的连通性检查。
- `POST .../oss/migrations`：`{enterprise_id,dry_run:true}` 只读预检；`{enterprise_id,confirm:true}` 明确创建该企业迁移。
- `GET /api/v1/merchant/file-jobs`、`/:id`、`POST /:id/retry`；管理员位于 `/api/v1/admin/storage/file-jobs`。
- `POST /api/v1/merchant/ai-import/tasks` 增加 JSON `{file_ids,request_id,mode,agreements}`，只复用当前账户经验证的 AI 原件；旧 multipart 接口保留。
- 运行配置增加 `oss_direct_upload`、`oss_file_access` 能力，旧客户端不必更改原文件上传及业务接口。

## 历史迁移与清理

先用 `exhibition oss-media scan-local` 输出 JSON Lines 清单；它使用实际 active target，不迁移、不注册未知文件、不删除文件。先检查本次盘点的 892 个文件与 890 个登记对象之间的差异，未知文件不能自动关联或公开。

首批为洋柯企业 67 和派一企业 79。先按企业预检、核对产品和容量，再明确执行；不能按名称猜测企业 ID。每个对象记录源、目标、大小、哈希和检查点。迁移后 Media.Path 和业务 URL 不变，由 StoredObject.RemoteKey 映射真实云键；ECS 原件作为已验证恢复副本保留。迁移不会重新提交或批准企业资料。

本页面和本轮代码不清理历史本地目录。盘点显示 ECS 主要占用来自历史发布版本、升级备份和软件包，而非约 394 MiB 媒体；不能宣称迁移媒体可释放数十 GB。此前批准的 17 个临时目录清单不构成删除其他目录的授权。

## 独立备份和隔离恢复

网站角色不能访问 `zqzhg-backup`。使用独立、短期操作 STS：[备份策略](../deploy/oss/operator-backup-policy.json)，恢复时额外申请 [恢复写入策略](../deploy/oss/operator-restore-additional-policy.json)。策略不授予删除备份对象权限，不自动设置保留期或定时清理。

运行网站二进制的独立命令，凭据和独立 32 字节加密密钥只从 stdin 输入，不使用命令行参数、浏览器或网站环境变量。输入 JSON 字段为 `access_key_id`、`access_key_secret`、`security_token`、`expires_at`、`encryption_key_hex`；备份另加 `confirm_offline:true`，验证／恢复另加 `snapshot_id`。请通过安全管道输入，禁止 shell 回显、录屏或提交真实输入文件。

| 命令 | 行为 |
|---|---|
| `exhibition oss-media backup` | 网站停止且上传授权失效、校验已排空后执行；检查本地服务端口，创建删除保护；对象按 SHA-256 去重并以内网服务器复制，mysqldump 经分帧认证加密流直传；最后写加密 manifest 并复读验证 |
| `exhibition oss-media verify-backup` | 验证 manifest、每个对象和完整 SQL 加密流，不改变业务资料 |
| `exhibition oss-media restore-isolated` | 解密至最多 64 MiB SQL 临时空间，导入全新随机隔离库；对象服务端复制到新的恢复键并验哈希，保留逻辑 Media ID 和远端映射；不切换 active target |

schema 038 安装前也可执行独立备份；网站必须完整停止。未登记的本地文件单独保全，恢复时进入私有隔离清单，不伪造企业归属或公开关联。恢复 CLI 需在该进程显式设置 `OSS_ENABLED=true`，仅对新隔离库安装迁移及存储映射；旧本地备份恢复后可先通过原媒体 API 读取，再受控生成公开副本。

恢复清除暂态租约、预占和旧实例本地副本映射，冻结旧上传键，避免隔离 worker 回收原生产对象。待处理的公开任务需要人工明确重试，重新使用新的目标键。恢复命令不会替换正式库或删除原对象。切换隔离库前必须核对完整 manifest、媒体、权限和网页，再单独批准。

备份密钥需独立离线保管；现有主密钥及站点／短信／百炼配置也须在受控渠道保管，不包含在普通代码包中。SQL 备份成功不等于这些站外配置已经备份。备份 Bucket 应保持 private 并启用阻止公共访问，由云管理员核对；网站程序不更改这些设置。旧完整加密 ZIP 仍可通过原恢复接口验证；OSS 开启后禁止旧全量本地压缩备份和自动调度，避免磁盘撑满或把不完整对象包当成完整备份。

## 回滚

常规回滚先关闭新上传并等待在途校验，停止 worker，回到**支持 schema 039 和 RemoteKey 的 OSS 基础版本**，保留表、草稿、批次、文件及公开映射。出现 OSS-only 文件后，不能直接回退到 v1.12.3，否则旧程序读不到这些文件。

[039 降级脚本](../deploy/linux/rollback-oss-039.sql) 仅用于导出检查后的破坏性撤销；存在 OSS 对象、会话、预占、活动任务或备份保护时主动拒绝。不要使用 mysql `--force` 绕过失败。普通版本切换不执行此脚本。

## 构建与验收

运行 Go 定向测试／vet、网页类型检查／Vitest／生产构建、Flutter analyze／测试和同签名 APK 构建。常规先输出独立候选，再按用户已授权的发布范围验收和上线。本次用户明确授权升级 APP；1.12.0+53 已发布，真机安装未验收，具体结果见 [上线记录](RELEASE_1.12.4_20261119.md)。

Linux／升级候选必须携带开发使用的全部 77 个真实边界快照，并遵守 [地图交付要求](MAP_DEPLOYMENT_REQUIREMENTS.md)：从实际待交付产物运行 production，代理路径验证全国→河北→邢台→宁晋、真实数据库统计、手机布局及失败重试。没有通过不能标为正式可部署包。

官方参考：[分片权限](https://www.alibabacloud.com/help/en/oss/user-guide/multipart-upload)、[阻止公共访问](https://www.alibabacloud.com/help/en/oss/user-guide/block-public-access)、[Bucket 策略](https://www.alibabacloud.com/help/en/oss/user-guide/oss-bucket-policy/)。
