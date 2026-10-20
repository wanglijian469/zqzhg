# 移动端升级验收包 1.8.0-rc.1+30

这是供验收的预发布，不替换稳定版，不部署生产。AI辅助填写尚未实施，不在本次源码或安装包中。

## 安装与源码

[GitHub Release](https://github.com/wanglijian469/zqzhg/releases/tag/v1.8.0-rc.1)提供原签名APK、对应源码ZIP、服务器更新ZIP、发布说明、`manifest.json`及`SHA256SUMS.txt`。源码标签 `v1.8.0-rc.1` 与发布清单的 `sourceCommit` 指向同一提交。

- APK为 `dalunongji-1.8.0-rc.1+30.apk`，包名 `com.zqzt.dalunongji`，构建号30，minSDK24、targetSDK36，包含arm64-v8a、armeabi-v7a、x86_64。
- APK SHA-256：`a86c6ffa47265647aff591247a316dfec4ac3a9d73e7ec2f049b9ac389c2ef7a`。
- 原签名证书 SHA-256：`85656c85541b188c832dc48d67cbf320046a4311aa9e0fefcc662dbe2e0f1585`。
- 使用覆盖安装，不卸载、不清除数据；覆盖安装及原数据保留效果仍需真机确认。正式后续版本使用更高构建号。
- 默认服务地址及已有自定义地址保持不变。局域网验收在App设置切换至实际核验过的电脑地址，不能使用手机自己的localhost。

本轮包含厂商地图统计与原生轮廓适配层、入驻登录、企业资料、三步产品编辑、工作台与推广同步；审核、公开版本及账号归属规则保留。详见[完整同步和验收记录](../../docs/MOBILE18_ACCEPTANCE.md)。

## 测试与限制

Flutter154项通过、7项跳过；前端185项通过。后端隔离MySQL回归、go vet、发布构建及168项原生布局检查通过。真实接口、网页及本地只读验收记录位于 `docs/test-output/mobile18-*`。

正式真实地图资源尚缺适用材料；DataV本机学习边界及轮廓截图不分发。Android真机覆盖安装、相机权限、系统分享和原生媒体尚待验收。Linux程序已交叉编译，未在Linux主机实际安装，也未验证MySQL8及真实OSS。

## 服务器更新与校验

服务器更新ZIP包含Linux amd64／arm64程序、网页、001—016迁移、目录、许可、升级脚本及内部校验清单。仅供后续授权升级，先运行 `sudo bash upgrade.sh --check`；实际应用前须按照[升级说明](../../docs/MOBILE18_ACCEPTANCE.md)核对备份及既有托管安装。包内没有业务数据库、私有媒体、环境密钥或签名密钥。

将全部Release附件下载到同一目录，使用发布专用 `SHA256SUMS.txt` 校验。该清单不引用本机交付目录中的截图和测试记录。GitHub发布工具先核对远端附件摘要，再公开预发布，并保持原最新稳定Release不变。

维护者发布步骤：审查并提交源码，执行 `python scripts/package-mobile-acceptance.py` 更新交付材料（如该步骤产生源码修改，须重新提交），工作区干净后执行 `node scripts/publish-mobile-acceptance.mjs prepare`；创建指向该提交的标签并正常推送；再执行 `publish` 和 `verify`。凭据从Git凭据管理器或私有环境变量取得，不写入发布文件。
