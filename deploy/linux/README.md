# 阿里云 Alibaba Cloud Linux 部署 · 1.5.13

目标地址：`http://39.97.250.244:8089`。Android：`1.5.13+21`，包名 `com.zqzt.dalunongji`。网站与后端：`1.5.13`，包含此前 1.5.11 的展示与厂商功能。

## 上传与一键部署

将以下两个文件上传到服务器同一目录（例如 `/root`）：

- `zqzt-aliyun-v1.5.13-linux.tar.gz`：程序、网站、APK、部署脚本，不包含业务数据库和图片。
- `zqzt-data-20261015.tar.gz`：私有迁移包，包含现有企业、产品、账户、密码哈希、审核记录及媒体。不要放到网站目录或提供公开下载。

在服务器该目录运行：

```bash
tar -xzf zqzt-aliyun-v1.5.13-linux.tar.gz && tar -xzf zqzt-data-20261015.tar.gz -C zqzt-aliyun-v1.5.13 && sudo bash zqzt-aliyun-v1.5.13/deploy.sh
```

按提示输入**服务器现有 MySQL 管理员密码**。无需把密码写进命令或发送给他人。默认连接 `127.0.0.1:3306`，管理员 `root`。脚本自动识别 x86_64 / ARM64，创建 `zqzt_exhibition_cloud` 数据库、独立应用账户，并导入数据；绝不覆盖同名已有库，也不会安装或重启 MySQL。

数据库端口或账户不同，在命令末尾补充参数，例如：

```bash
sudo bash zqzt-aliyun-v1.5.13/deploy.sh --db-port 3307 --db-admin root --db-name zqzt_exhibition_cloud2
```

若使用仅 Unix socket 认证的 MySQL 管理员，可加 `--db-host localhost`。若客户端未在 PATH，脚本会检查常见安装位置；否则先将现有 MySQL 的 bin 目录加到 PATH，或用 `sudo env PATH="$PATH" bash ...` 传入路径。要求管理员具备建库、建用户和授予新库权限的能力。

脚本验证包内 SHA-256、媒体与导入前后各表行数，创建专用系统用户、`/opt/zqzt` 目录和 `zqzt-exhibition.service`，监听 `0.0.0.0:8089`。已启用 firewalld 时仅添加 TCP 8089 规则。首次部署时端口被占用会停止。常用基础工具缺失时使用系统 dnf/yum 安装，现有软件仓库须可访问。

## 阿里云安全组与访问

在 ECS 控制台为此实例的安全组增加**入方向 TCP 8089**规则。供所有手机访问可设授权来源 `0.0.0.0/0`；仅测试时可限制为你的公网出口 IP。不要开放数据库端口。安全组操作见[阿里云官方说明](https://help.aliyun.com/zh/ecs/user-guide/security-group-rules)。

- 网站：`http://39.97.250.244:8089`
- 管理后台：`http://39.97.250.244:8089/admin`
- 手机下载：`http://39.97.250.244:8089/app`
- APK：`http://39.97.250.244:8089/downloads/dalunongji.apk`
- 健康检查：`http://39.97.250.244:8089/api/v1/health`

管理员及企业账户沿用本机已有账号密码；迁移不保留旧登录会话。云端数据是打包时快照，部署后请在云端继续录入，避免本机与云端各自修改。

当前使用用户指定的 IP + HTTP。正式对外使用建议配置域名和 HTTPS，避免登录信息通过明文传输；届时可在 App 设置中修改服务地址。

## Android 安装与图标

下载 APK 后允许浏览器安装应用，直接覆盖旧版本，**不要先卸载**。签名证书沿用原版，版本号提升至 `1.5.13+21`，通用 APK 包含 ARMv7、ARM64、x86_64。

新图标使用深蓝满版底色及蓝白展馆标识，提供五种传统图标尺寸和 Android 自适应图标。所有传统 PNG 的像素均不透明，自适应背景也铺满；桌面形状由手机启动器决定。

首次运行云端新版时，原默认 `172.27.35.2:8080` 或模拟器默认地址会自动切换到云端，并退出旧服务器会话；旧本机草稿保留在原地址下。自定义地址保留。之后手动修改地址不会被反复覆盖。中英文与其他本机设置保留。

## 短信与视频

未提供阿里云短信凭证，本包生产配置为 `SMS_PROVIDER=disabled`：可注册企业账号、使用企业账号密码及管理员密码登录；手机号验证码登录不可用，不会返回开发验证码。需要短信时编辑 `/etc/zqzt/exhibition.env`，设置 `SMS_PROVIDER=aliyun` 并补齐 `ALIYUN_ACCESS_KEY_ID`、`ALIYUN_ACCESS_KEY_SECRET`、`ALIYUN_SMS_SIGN_NAME`、`ALIYUN_SMS_TEMPLATE_CODE`，再重启服务。不要启用 development 短信模式。

已有视频及图片随迁移包保留。新视频转码需要 FFmpeg 和 FFprobe（包含 libx264 编码器）。部署脚本先使用已有工具，缺失时尝试从**已配置的系统软件仓库**安装 `ffmpeg`；仓库未提供时会明确提示。请按服务器系统版本从可信软件源安装兼容版本，然后验证：

```bash
ffmpeg -version
ffprobe -version
ffmpeg -hide_banner -encoders | grep libx264
sudo systemctl restart zqzt-exhibition
```

使用自定义安装位置时可在配置中加入 `FFMPEG_PATH=/绝对路径/ffmpeg` 和 `FFPROBE_PATH=/绝对路径/ffprobe`。可用来源参见 [FFmpeg 官方下载页](https://ffmpeg.org/download.html)。没有转码工具时网站、图片、图册和既有可播放视频仍可使用，新上传视频不能完成处理。

## 运维、备份与升级

```bash
sudo systemctl status zqzt-exhibition --no-pager
sudo journalctl -u zqzt-exhibition -n 100 --no-pager
curl --noproxy '*' -f http://127.0.0.1:8089/api/v1/health
sudo systemctl restart zqzt-exhibition
```

程序在 `/opt/zqzt/releases/`，当前版本由 `/opt/zqzt/current` 指向；媒体位于 `/opt/zqzt/shared/media/`，配置位于 `/etc/zqzt/exhibition.env`（包含数据库密码，禁止公开）。systemd 随开机启动。

升级前先保存配置和媒体，并使用现有数据库运维方案对新库执行 `mysqldump --single-transaction` 备份；为使媒体与数据库对应，备份过程中暂停应用写入。不要只备份程序目录。

后续将新程序包解压到新目录并运行其 `deploy.sh`。已有 `/etc/zqzt/exhibition.env` 时进入升级模式，保留配置/数据库/媒体，跳过迁移；启动健康检查失败会自动恢复先前程序链接。本版本没有新增数据库迁移。回退程序不会还原数据库和媒体。

首次导入失败会留下新建库供诊断；脚本再次运行不会覆盖它。确认失败原因后使用另一个新的 `--db-name` 重试，或由数据库管理员检查并清理确定不再需要的失败库。若配置已生成但启动失败，可修复问题后直接重跑。

迁移包在导入和备份完成后应移至非网站的私有备份位置，并限制权限，例如 `chmod 600 zqzt-data-20261015.tar.gz`。不要公开上传数据库、配置文件、签名密钥或私有备份。

## 本次验证与边界

已验证：Flutter 静态分析及 26 项测试（1 项可选截图测试跳过）；网站 72 项测试；Go 完整测试（含隔离 MySQL 流程）和 vet；数据恢复到独立测试库后逐表行数、账号密码哈希及全部媒体副本一致；部署脚本语法、参数及无写入预检；APK 版本、包名、三种架构、签名及副本 SHA-256；图标不透明像素和自适应资源。

本机为 Windows，没有可用 Linux 虚拟机、连接的 Android 设备或已配置模拟器。本次未实际操作 ECS、未在 Alibaba Cloud Linux 实机运行安装脚本、未实测手机全新安装/覆盖升级、相机、系统分享和媒体播放。服务端转码工具是否能由 ECS 软件仓库安装，需要部署时确认。具体证据和校验值随包提供。
