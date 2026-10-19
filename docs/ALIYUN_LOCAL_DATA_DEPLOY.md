# 阿里云升级与本地数据导入 · 2026-10-19

本包为私有交付包，复用已发布 `aliyun-20261018-storage-v1` 配套前后端，增加当前本地数据库和媒体快照。不部署生产，不重新编译已有程序。实际记录数及文件校验见 `local-data/manifest.json`，验证见 `verification/local-data-validation.json`。

**不要上传公开 GitHub、网站目录或提供公开下载。** 包含企业联系方式、草稿、历史审核、账号密码哈希及上传资料。不含服务器环境文件、登录会话、短信验证码、OSS凭据或加密密钥。原登录密码保持不变，导入后需重新登录。

## 准备与上传

适用于已按现有规范部署的 Linux ECS，默认 `/opt/zqzt`、`/etc/zqzt/exhibition.env`、`zqzt-exhibition.service`。不适用于全新安装。需要 Python3、MySQL客户端及 mysqldump、systemd、curl、tar、sha256sum。先确保服务器剩余磁盘足够存放原数据库/媒体备份和新增快照。程序预检要求保留有效 `CONFIG_ENCRYPTION_KEY`、实际 `MEDIA_DIR`、稳定 `OPERATIONS_DIR` 及当前 `PUBLIC_BASE_URL`；不得用本机 `.env` 替换服务器配置。

在本机 PowerShell 执行（将服务器地址替换为实际地址；首次连接核对服务器指纹）：

```powershell
scp "D:\workspace-main\codex-zqzt\delivery\private\zqzg-aliyun-20261019-with-local-data-v1.tar.gz" "D:\workspace-main\codex-zqzt\delivery\private\zqzg-aliyun-20261019-with-local-data-v1.tar.gz.sha256" root@39.97.250.244:/root/
```

## ECS 上先升级程序

以下命令必须在同一个 SSH 终端中执行。校验或预检失败时停止，不跳过检查。

```bash
sudo -i
cd /root
sha256sum -c zqzg-aliyun-20261019-with-local-data-v1.tar.gz.sha256 && \
tar -xzf zqzg-aliyun-20261019-with-local-data-v1.tar.gz && \
cd zqzg-aliyun-20261019-with-local-data-v1 && \
bash upgrade.sh --check && bash upgrade.sh --apply
```

升级程序会暂停服务、备份原数据库/媒体/环境/控制状态，保留原配置和数据。它不自动导入本地数据。

## 再启用包内本地数据

此操作会将网站切换为本机快照，**不是与服务器记录合并**。服务器原库、原媒体、原账号及控制状态均保留，停止在旧库录入。云端快照账号、密码与本机相同；服务器原有账号不合并到新库。

```bash
cd /root/zqzg-aliyun-20261019-with-local-data-v1
python3 import-local-data.py --check && \
python3 import-local-data.py --apply --confirm-use-local-data
```

未配置 `DB_MAINTENANCE_USER/DB_MAINTENANCE_PASSWORD` 时，会安全提示输入现有 MySQL 管理员密码，不回显、不保存；用户名不同可增加 `--db-admin 实际管理员用户名`。管理员需要创建数据库、临时用户及授权权限。不要把密码写进命令。导入只向新建随机库执行，逐表核对数量、校验全部文件，再通过现有 `active.json` 指针切换；保留服务器域名/上传设置和身份标识，不导入本机地址或磁盘配置。新媒体存于 `/opt/zqzt/shared/local-data/<批次>/media`，仍在程序与Web根目录之外。

若任何导入/健康检查失败，原数据指针恢复，重新启动原配套程序；失败的新库和媒体保留诊断。实际结果以日志为准，不自动删除数据库。启用快照后请只在服务器继续录入。

## 检查

```bash
systemctl status zqzt-exhibition.service --no-pager
curl --noproxy '*' -fsS http://127.0.0.1:8089/api/v1/health
journalctl -u zqzt-exhibition.service -n 80 --no-pager
```

访问当前网站，核对企业、产品、图片、视频、画册和登录；端口如已调整使用服务器实际配置。不修改安全组、DNS、Nginx或域名。保留本包在私有目录并限制权限：`chmod 600 /root/zqzg-aliyun-20261019-with-local-data-v1.tar.gz`。

## 回滚与边界

程序升级备份位于 `/opt/zqzt/shared/upgrade-backups/<版本>-<时间>/`；数据切换备份位于同目录 `before-local-data-<时间>/`。数据回退使用支持 Schema15 的当前配套程序，并在暂停服务后恢复切换前的 `active.json`；若原先无指针，则恢复原环境默认数据目标。切换后如已有新业务写入，先备份并人工处理差异，不丢弃新记录。不要盲目运行缺少展馆/短地址隔离能力的旧后端。

本机可验证隔离MySQL恢复、密码哈希、文件SHA256和生产配置下Windows运行；Linux ECS真实升级、MySQL权限环境、NAS/OSS、系统服务启动仍需服务器验收。要求挂载存储的环境不使用此本地磁盘快捷导入器。
