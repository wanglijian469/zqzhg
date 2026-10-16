# 部署脚本修复 fix1：MySQL 密码策略

原部署脚本的应用数据库密码仅由十六进制字符组成，可能被服务器的 MEDIUM / STRONG 密码策略以错误 1819 拒绝。这与交互输入的 MySQL 管理员密码不同。

修复后默认生成包含大小写字母、数字和符号的 60 字符随机密码；增加 `--prompt-app-db-password` 参数，可以隐藏输入并确认应用数据库密码。手动密码支持 12–128 位 ASCII 字母、数字和 `@%+_=!?:.,~-`，并须符合服务器自身的密码策略。不会修改或降低服务器的全局密码策略。创建用户在创建新库之前执行，因此密码不符合策略时不会再留下新数据库。

## 已上传原部署包的服务器

下载 `zqzt-deploy-v1.5.13-fix1.tar.gz` 上传到 `/opt`，在 root 终端运行：

```bash
cd /opt &&
tar -xzf zqzt-deploy-v1.5.13-fix1.tar.gz &&
env -u DB_ADMIN_PASSWORD bash /opt/zqzt-aliyun-v1.5.13/deploy.sh \
  --db-name "zqzt_exhibition_cloud_$(date +%Y%m%d_%H%M%S)" \
  --prompt-app-db-password
```

按顺序输入：现有 MySQL 管理员密码、新应用数据库密码、再次确认新应用数据库密码。输入不会显示。root 与应用账号为两个不同账号；可按自己的管理方案设置密码。补丁不内置任何密码。

补丁包含更新后的 `deploy.sh`、配套 `SHA256SUMS`、修复说明及验证报告，其余原版程序/网站/APK 沿用。脚本仍会检查整个部署目录。原先失败尝试留下的数据库会保留，命令选择新的库名。原迁移资料无需重新上传。

## 从头部署

可以改用完整包 `zqzt-aliyun-v1.5.13-fix1-linux.tar.gz`，解压后仍为 `/opt/zqzt-aliyun-v1.5.13`。媒体/数据库迁移包和 Android APK 不变。

## 验证

在独立启动的 MySQL 实例中启用 MEDIUM 密码策略、最小长度 12：原十六进制密码被 1819 拒绝；新生成密码通过创建用户验证；手动输入格式以合成密码验证；已有 MySQL 实例与全局策略未修改。另检查 Bash 语法、参数与归档校验。此修复尚未在用户的 ECS 上运行。

策略说明：[MySQL 官方密码验证组件文档](https://dev.mysql.com/doc/mysql-security-excerpt/8.0/en/validate-password.html)。
