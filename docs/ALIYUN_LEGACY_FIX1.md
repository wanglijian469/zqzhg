# 旧版升级兼容补丁 fix1 · 2026-10-19

适用于已上传并解压的 `zqzg-aliyun-20261019-with-local-data-v1`，不替换程序、数据库、媒体或原包SHA256清单。补丁只新增独立的初始化及修正导入器，原私有完整包保留。

## 修正内容

- 旧版未配置密钥时，确认数据库Schema为1—9、无加密配置表、运维目录无旧文件后才初始化32字节密钥。只读查询失败、新版Schema、已存在运维资料或无效旧密钥时停止，不覆盖密钥。
- 有效的已有密钥保持不变，支持重试。配置变更前备份原环境文件，另存新密钥；文件保持私有权限，环境文件保持原用户、组和模式。密钥不输出到终端。
- 临时MySQL导入账号使用大小写、数字、特殊字符的随机密码，兼容常见MEDIUM校验。
- root创建的数据切换指针继承服务用户所有权，保持0600，避免非root服务无法读取而回退。

## 执行

补丁为不含业务数据和凭据的公开小包。解压到现有私有完整包根目录，校验`legacy-fix-SHA256SUMS`。目录初始化已完成后运行：

```bash
cd /opt/zqzg-aliyun-20261019-with-local-data-v1
python3 initialize-legacy-key.py && \
bash upgrade.sh --check && bash upgrade.sh --apply && \
python3 import-local-data-fix1.py --check && \
python3 import-local-data-fix1.py --apply --confirm-use-local-data
```

MySQL管理员密码仅在终端提示时输入，不写入命令。导入后使用本机账号重新登录。原库和媒体保留，不进行ID合并。

初始化脚本输出的`/etc/zqzt/key-init-backup-.../`包含原环境及新密钥，保留并另行安全离线备份，不放公开GitHub或网站目录。已有加密数据但缺失原密钥时不能通过新建密钥恢复。

## 验证范围

初始化保护6项、导入17项单元测试；当前数据恢复至隔离MySQL，逐表数量、账号密码哈希、357个媒体SHA256、9家企业/45款产品查询及公开页面通过。真实Linux systemd/ECS运行和生产MySQL密码策略尚待服务器验收，不以Windows测试代替。补丁不含任何用户数据库、密钥或上传素材。
