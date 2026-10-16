# 部署 备份与恢复

## Android 1.5.12+20

Android公开展示与厂商功能同步到网站／后端1.5.11，沿用原签名。安装APK、局域网连接、验证记录与设备测试限制见 [ANDROID_SYNC.md](ANDROID_SYNC.md)。网站与后端版本保持1.5.11，无新增迁移。


## 网站1.5.7 批量审核

更新完整网站静态文件；后端保持1.5.6、Android保持1.5.2+19，复用原管理员逐条审核接口，不需要迁移、重启后端或重打包App。旧网站静态文件备份 `.local/backups/pre-batch-review-v157/web-dist`。本机／局域网服务直接提供更新后的 `web/dist`，浏览器刷新即可加载新菜单。操作与异常处理见 [BATCH_REVIEW.md](BATCH_REVIEW.md)。


## 1.5.6 分类层级补充

更新网站与Windows/Linux后端为1.5.6；厂商分类新增和管理员规范支持 `parent_id=0` 一级分类。无数据库迁移，原权限、去重、审核及分类归属保留。更新前备份 `.local/backups/pre-root-category-v156`，替换程序后重启。Android仍为1.5.2+19。完整说明见 [CATEGORY_ENTRY.md](CATEGORY_ENTRY.md)。


## 1.5.5 新厂商注册

网站和 Windows/Linux 服务升级为 **1.5.5**，Android 沿用原签名 **1.5.2+19**，无需重打包。新增 `/api/v1/auth/merchant/register`，不新增数据库迁移，继续使用迁移1–6。无需配置真实短信；公司账号注册与登录可直接使用。

备份目录 `.local/backups/pre-registration-v155`；部署前备份数据库和媒体，停止旧服务并确认退出后替换程序及完整 `web/dist`，保留 `.env` 和媒体目录。当前同网地址 `http://172.27.35.2:8080`，健康检查 `/api/v1/health`。步骤和验收见 [REGISTRATION.md](REGISTRATION.md)，校验清单位于 `delivery/manifest.json`。


## 1.5.1 工业云展厅视觉升级

更新完整网站 `web/dist` 和原签名 Android **1.5.1+18**；本版没有新增数据库迁移或接口，后端业务代码与 1.5.0 相同。服务器继续执行迁移 1–5。保持 FFmpeg、数据库、媒体目录、账号及原签名密钥配置。

App 默认同网 API 为 `http://172.27.35.2:8080`，电脑 IP 改变后更新配置及 App 设置。交付清单位于 `delivery/manifest.json`，新版截图和验收说明见 [工业云展厅 UI](INDUSTRIAL_UI.md)。以下章节为多媒体部署说明与历史。

## 1.5.0 五栏目与多媒体升级

网站、Windows/Linux 后端和 Android 同步升级为 1.5.0，APK 为 1.5.0+17。构建后端使用 Go **1.26+**。网站 `npm run build` 自动复制 PDF.js 字体、字符映射、WASM 及许可证到生产目录，部署时复制完整 `web/dist`。

升级前备份 SQL 和完整媒体目录，当前本机备份为 `.local/backups/pre-resources-v150`。启动时执行可重复的 `005_showroom_resources.sql`，原企业、产品和图片兼容；部分建表中断后再次启动会继续完成。备份媒体目录须包括原始待处理视频、转换文件、封面、PDF 和图册图片；恢复后使用新版服务启动，处理中任务重新进入队列。请勿仅备份已发布文件。

视频服务需要 FFmpeg 和 FFprobe；运行 `./scripts/setup-video-tools.ps1 -Download` 可准备 Windows 本机工具，或设置 `FFMPEG_PATH`、`FFPROBE_PATH`。Linux 安装系统 FFmpeg 后配置路径。工具不包含在交付 ZIP，安装来源、许可证和完整限制配置见 [多媒体部署说明](MULTIMEDIA_SHOWROOM.md)。本机单任务处理，处理失败保留原因并允许厂商重试。

本次仍使用同网地址 `http://172.27.35.2:8080`，iOS 即将上线。下方旧版本章节保留作为升级历史。

## 1.4.5 后台升级

后台入口 `/admin`，默认工作台 `/admin/dashboard`。操作说明见 [运营工作台](ADMIN_CONSOLE.md)。本次新增迁移 `003_operation_logs.sql`，创建操作日志并按审核来源 ID 去重导入旧记录；启动程序自动执行，原管理员密码不改。

升级前备份数据库和媒体。当前本机备份位于 `.local/backups/pre-admin-v145`，已在独立恢复库检验 SQL/媒体 SHA-256 并恢复 20 条厂商和产品、54 条图片记录。日志迁移可以重复执行而不重复导入历史。若需回退新版应用，旧程序不会访问新增表；完整恢复仍应遵循下方新库与媒体配套恢复步骤。

新版 Windows/Linux 后端、网站和部署包版本为 1.4.5；APK 继续为 1.4.1+15。新增厂商账户初始密码仅显示一次，请由管理员交给厂商保管；本次不提供已建账号的密码重置。

## 单服务部署

1. 使用 Node.js 20.19+/22.12+ 或较新兼容版本构建 `web`：`npm ci && npm run build`。本次锁定的依赖已记录在 `package-lock.json`。
2. 使用 Go 1.26+ 构建 `server`。Windows：`go build -o exhibition.exe .`；Linux：在对应平台构建，或设置 `GOOS=linux GOARCH=amd64 CGO_ENABLED=0` 后编译。
3. 服务器放置后端可执行文件、`web/dist`、可写媒体目录和后端 `.env`；工作目录为后端目录。程序从 `WEB_DIR` 提供网站，`/admin` 提供 CMS，所有 `/api/v1` 请求走同一后端。
4. `.env` 配置正式 MySQL、`MEDIA_DIR`、`WEB_DIR`、`HTTP_ADDR=127.0.0.1:8080`、`APP_ENV=production` 和 `SMS_PROVIDER=aliyun`。`ADMIN_PASSWORD` 仅用于首次初始化，至少 12 字符。首次创建数据库和迁移由程序执行，后续可收紧数据库账号的 DDL 权限。
5. 反向代理开放 HTTPS。同源网站不需要 CORS；分离开发网站时 `CORS_ORIGIN` 只能填写指定来源。后端不信任客户端提供的代理 IP；当前短信限流按直接连接 IP 计算，正式反向代理环境需根据实际可信代理地址配置 Gin，而不能信任任意代理头。
6. 用 systemd、Windows 服务包装器或其他现有进程管理器守护后端；监控 `/api/v1/health`、数据库连接、磁盘空间与请求日志。

Nginx 示例：

```nginx
server {
    listen 443 ssl;
    server_name exhibition.example.com;
    ssl_certificate /etc/nginx/tls/fullchain.pem;
    ssl_certificate_key /etc/nginx/tls/privkey.pem;
    client_max_body_size 11m;
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_read_timeout 65s;
    }
}
```

## 正式短信

配置 `ALIYUN_ACCESS_KEY_ID`、`ALIYUN_ACCESS_KEY_SECRET`、`ALIYUN_SMS_SIGN_NAME`、`ALIYUN_SMS_TEMPLATE_CODE`；模板包含 `code` 变量。本项目按照 [阿里云 SendSms API](https://help.aliyun.com/zh/sms/developer-reference/api-dysmsapi-2017-05-25-sendsms) 实现 RPC 签名调用。

验证码有效期 5 分钟，同手机号间隔至少 60 秒；验证码单次消费，最多 5 次尝试。短信请求按 IP 限制为每小时 20 次，手机号登录每分钟 30 次，管理员登录每分钟 10 次。开发验证码只在开发环境和 development 适配器同时启用时返回。生产环境缺少真实短信配置会拒绝启动。

上线时用正式号码验证短信送达、错误验证码、超时、重放与频控；不把开发模式测试结果当作送达证据。

## APK 与 iOS

本机交付 APK 使用专用 Release 密钥。源代码中 Android 允许 HTTP，以支持局域网验收；正式发布应使用 HTTPS API，将主清单 `android:usesCleartextTraffic` 改为 `false` 后重新构建。同一应用升级必须保留 `.local/signing` 密钥和密码，不使用调试密钥替代。iOS 源码已配置相机、相册用途说明和本地网络例外；在 macOS/Xcode 环境配置签名后构建，不交付未经验证的 IPA。

## 备份

当前提供 Windows PowerShell 工具，可通过 `DB_HOST`、`DB_PORT`、`DB_USER`、`DB_PASSWORD` 环境变量修改连接；默认连接本机 13306。`MysqlBin` 参数可指定 MySQL 工具目录。

```powershell
./scripts/backup.ps1 -OutputDirectory '.local/backups/manual'
node scripts/restore-test.mjs .local/backups/manual
```

备份包含 `database.sql`、`media.zip` 和记录 SHA-256 的 `manifest.json`。SQL 使用事务一致性导出；要获取数据库和图片的整体一致快照，备份时暂停写入流量，保留只读访问，避免在两个备份阶段之间提交新图片。备份账号和文件可能包含联系方式及会话记录，应放在受限目录并保存异地副本。

## 恢复

`restore-test.mjs` 先核验 SQL 和图片 ZIP 的 SHA-256，再新建 `zqzt_exhibition_restore_时间戳` 并导入，不覆盖业务库。媒体自动解压到备份目录下新建的 `restored-media-恢复库名`，目标路径写入 `restore-test.txt`。检查恢复库的数据与记录对应图片后，使用该新媒体目录恢复服务。

正式恢复时暂停应用写入，备份当前状态；使用测试恢复得到的新数据库及新媒体目录，修改 `DB_NAME` 和 `MEDIA_DIR` 后重启服务。先检查 `/api/v1/health`，再检查公开详情、私有媒体权限、管理员登录和审核。保留原数据库和媒体目录以便回退；回退时恢复原配置并重启，不混用不同备份时间的数据与图片。

恢复测试库不会自动删除，便于人工检查。清理前核对库名确为本次恢复工具创建的 `zqzt_exhibition_restore_` 前缀数据库。

## V1.5.2 厂商录入与自行入驻

网站与 Android 同步为 1.5.2+19。上线前备份数据库与媒体；本机备份 `.local/backups/pre-onboarding-v152`。新服务自动执行迁移 006，新增厂商补充分类与分类 ID 映射两张表，旧分类保留为正式分类。DDL 具备重复执行和部分中断恢复能力，集成测试验证恢复；不重写历史内容版本。回退时优先保留两张新增表，若恢复备份则使用配套新库与媒体目录，按上方恢复流程切换。

网站构建和 APK 构建会运行 `scripts/sync-form-rules.mjs`，将 `server/form_rules.json` 同步为两端规范。字段接口错误新增兼容的 `error.fields`，旧 `error.code` 与 HTTP 状态保留。部署完整网站和后端后重启，不只替换前端。使用与旧 APK 相同的 Release 密钥，版本 1.5.2、versionCode 19。

局域网服务地址核对为 `http://172.27.35.2:8080`，下载页 `/app`。管理员与已有关联厂商账号沿用，原业务资料保留。规范、图片上传、分类及验收说明见 [ONBOARDING.md](ONBOARDING.md)。

## 1.5.3 密码重置升级

新增管理员厂商密码重置接口、账号列表精确筛选及公司密码登录事务保护。没有字段或表结构迁移。上线前备份数据库和媒体；本机已保存 `.local/backups/pre-password-reset-v153` 并恢复校验。替换网站与 Windows/Linux 服务，等待旧进程完全退出、文件释放后再替换二进制并重启，核对 `/api/v1/health` 和新接口鉴权。网站和服务版本 1.5.3，Android 沿用 1.5.2+19，不替换签名或重新打包。

回退时先备份当前状态，再回到原版本源码／部署包；密码哈希仍为相同 bcrypt 格式，新密码仍可在旧版登录，已撤销会话不会恢复。不要为回退代码恢复旧数据库密码快照。详见 [PASSWORD_RESET.md](PASSWORD_RESET.md)。

## 网站 1.5.4 分类入口更新

仅替换 `web/dist`，后端继续 1.5.3，Android 继续原签名 1.5.2+19；无需数据库迁移或后端重启。新的源码／部署包同时标记三端版本。浏览器已打开的页面刷新后加载新入口。

回退时从 1.5.3 部署包恢复网站 `web/dist`，不恢复数据库，不覆盖现有厂商密码或分类记录。操作与验收见 [CATEGORY_ENTRY.md](CATEGORY_ENTRY.md)。

## 1.5.8 首页展示配置

网站与后端一起更新为1.5.8，Android继续原签名1.5.2+19。上线前备份SQL和完整媒体目录，本机备份位置 `.local/backups/pre-home-display-v158`。新服务启动自动执行迁移007：单例 `home_display_settings` 及平台图片登记表 `home_display_media`。DDL和初始化使用IF NOT EXISTS／INSERT IGNORE，重复或部分中断恢复不会重置已保存配图和版本；不修改厂商资料或原精选记录。

备份和恢复必须同时包含两张新表以及原媒体目录中的完整图片／缩略图，否则平台图片可能缺失或失去公开状态。隔离恢复完成后核对配置版本、当前配图匿名GET／HEAD、未保存及已替换配图仅管理员可读。未配置配图时显示工业风占位。管理员上线后自行上传保存及选择品牌展厅，升级不自动设置真实资料。操作说明见 [HOME_DISPLAY.md](HOME_DISPLAY.md)。

回退时先备份当前数据库和媒体，再恢复配套旧网站与服务；保留007新表不会影响旧版本。旧公开首页接口不输出hero字段，厂商显示仍受旧版本数量限制。不要为了回退代码恢复旧库并丢失升级后的正常业务写入。

## 网站1.5.9 厂家品牌卡片与封面

仅更新网站 `web/dist`，后端仍为1.5.8，Android继续原签名1.5.2+19，没有接口或迁移更新。旧静态文件备份 `.local/backups/pre-vendor-cards-v159/web-dist`，替换构建后刷新页面即可。管理员通过厂商管理“设置封面”配置企业封面，保存／审核仍走原流程；本轮不自动修改真实图片或资料。回退仅恢复旧静态文件，不恢复数据库或厂商封面。见 [VENDOR_CARDS.md](VENDOR_CARDS.md)。

## 网站／后端1.5.10 厂商工作台与代表展品

同时更新网站与Windows/Linux服务到1.5.10，Android保持原签名1.5.2+19。上线前备份SQL及完整媒体目录，本机备份 `.local/backups/pre-merchant-workspace-v1510` 并独立恢复核验；旧网站与服务也保存在该目录。新服务启动执行迁移008，仅新增 `showroom_display_settings`，可重复执行和部分中断恢复，不重写原资料、精选或默认排序。

新后台入口 `/merchant`，原 `/join?tab=...`兼容跳转；代理应继续将非API深链接交给网站入口。新公开展厅响应添加代表展品，旧产品接口与App保持兼容，旧App不使用手动配置。配置表是备份恢复的一部分，需与业务表和媒体成套恢复；恢复后检查设置版本、公开资格、登录和旧链接。

回退前备份当前状态，再恢复配套旧网站与服务，可保留新增配置表；旧版网站恢复原自动推荐，不读取手动配置。不要为回退代码恢复旧业务库而丢失正常产品和账号写入。使用及验收见 [MERCHANT_WORKSPACE.md](MERCHANT_WORKSPACE.md)。

## 网站／后端1.5.11 企业宣传门户

企业资料新增资质荣誉及图集标注，存放原版本JSON，无新表或迁移，原8项迁移不变。更新网站与Windows/Linux服务，Android保持1.5.2+19原APK。备份SQL、完整媒体目录、当前网站与服务后部署。新图片仍按原版本审核及媒体权限读取，生产资料不自动修改。恢复必须保持SQL版本资料与媒体文件配套；旧服务不能安全编辑新增字段，回退前先停止写入并保留升级后的备份。详见 COMPANY_PORTAL.md。
