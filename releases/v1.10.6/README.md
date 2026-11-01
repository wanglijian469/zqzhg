# 中企展馆网站与大陆农机 Android 1.10.6+44

本版同步发布网站使用指南、公共页脚、厂商展厅精简页脚和按语言显示的英文装饰。Android 保留原包名、应用名称及发布签名，固定连接 `https://zqzhg.com`，同步内嵌网页和九张指南操作配图，并将“关于”中的版本文字改为 1.10.6（44）。

- 网站：[中企展馆](https://zqzhg.com/)
- 指南：[厂商入驻与使用指南](https://zqzhg.com/guide)
- 安装包：[正式安卓下载](https://zqzhg.com/downloads/dalunongji.apk)
- GitHub：[v1.10.6 Release](https://github.com/wanglijian469/zqzhg/releases/tag/v1.10.6-r3)

安卓保持包名 `com.zqzt.dalunongji`，versionCode 为 44。原签名证书 SHA-256：`85656c85541b188c832dc48d67cbf320046a4311aa9e0fefcc662dbe2e0f1585`。安装包具体大小、文件哈希及下载地址见本目录 `manifest.json`，GitHub 附件配套提供 SHA256 清单。

## 运行与升级

本包面向现有 `/opt/zqzt`、systemd、Nginx、MySQL 的 Linux amd64 安装。数据库结构保持 Schema 28，没有新增迁移；账号、厂商、产品、媒体、短信配置及证书续期沿用原配置。升级脚本只重启 `zqzt-exhibition.service`，不停止 Nginx、MySQL、GoSync 或其他应用，不重新启用已停用的旧站。

Linux 包含程序、完整网页、原签名 APK、迁移链、运行目录、77 个开发演示地图快照及来源元数据。地图继续使用已部署的公开演示路径，不依赖开发接口。演示用途说明保留，不声明已经获得正式地图授权；130571 的缺失轮廓仍采用原有“轮廓未匹配”地区选择方式。

解压 Linux 附件，核验外部 SHA256 清单后，以 root 执行包内脚本：

```bash
python3 upgrade.py check
python3 upgrade.py backup
python3 upgrade.py stage
python3 upgrade.py candidate
python3 upgrade.py apply
python3 upgrade.py status
```

`backup` 在服务器私有目录备份旧程序、数据库、完整媒体及配置，校验归档和媒体文件。`stage` 明确设置发布根目录的访问权限，并以 `zqzt` 账号核验程序、网页和地图文件可读及程序可执行。`candidate` 使用独立 MySQL 实例恢复备份，以正式 `zqzt` 服务账号和生产配置启动候选程序，经独立 Nginx 回环代理验证指南、配图、接口、站点地图及全国→河北→邢台→宁晋的边界哈希。候选服务关闭短信和后台任务，不访问生产数据库写入。通过后才执行 `apply`；启动或基本检查失败自动恢复旧程序与环境文件。

正常回滚：

```bash
python3 upgrade.py rollback
```

回滚只恢复程序和环境文件，不导入旧业务库。回滚前先保留升级后的数据。Android 已安装 44 版本后不能用更低 versionCode 直接覆盖，应发布更高版本修复包，避免卸载丢失未上传草稿。

服务器备份、数据库、私有媒体、环境文件、SSH 凭据和签名私钥不进入 GitHub 或发布附件。升级回执中的私有目录由运维保留，公开验收摘要见后续 `deployment.json`。

## 验证边界

本机完成 APK 原签名、包名、版本及 237 项内嵌网页资源校验，包含九张指南配图；静态分析通过。未连接 Android 真机，覆盖安装、相机、系统分享及 Android WebView 真机行为不能标记为已验收。iOS 不在本次发布范围。

完整历史测试存在旧登录字段、旧审核流程、旧首页布局和地图夹具失败；本次通过未修改的 HEAD 基线比对单独记录，不修改业务校验迎合旧断言。最终验证及限制见本次发布验收记录。
