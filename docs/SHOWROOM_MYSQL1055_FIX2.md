# 厂商展厅 MySQL 1055 修复 · Schema21 Fix2

日期：2026-10-24。发布标识：`aliyun-20261024-schema21-fix2`。

## 故障及证据

生产环境厂商展厅 API 返回 HTTP 500，日志为 `error=mysql_1055 source=showroom.go:227`，查询指纹 `941c69b0b47ca2dabe906f3757676684d0d3072aa0bffe1d57896327eeac5028`。健康检查、首页、厂商与产品列表可以访问；故障发生在展厅分类统计，App 将 `internal_error` 显示为“服务暂时异常，请稍后重试”。

在随机命名的隔离 MySQL 5.7.18 测试库、固定连接上开启 `ONLY_FULL_GROUP_BY`，旧 SQL 复现相同的 1055 及查询指纹；修改前展厅接口返回 500。测试恢复原 SESSION 模式，并确认 GLOBAL 模式未改变，没有连接或修改生产数据库。

## 变更

- `server/showroom.go`：内层沿用公开产品授权查询，按已审核发布版本计算有效产品分类；外层对分类列汇总。严格分组校验无需推断关联子查询内部字段的依赖关系。
- 从同一批聚合结果生成产品总数、分类列表及祖先累计数量。统计部分由产品计数、分类去重、分类汇总三次查询合并为一次；这不是整个详情接口的查询总数。
- 保留现代产品分类优先、旧分类字段回退、历史别名映射、父子累计和资源展示；响应字段不变，不改写厂商或产品资料。
- 沿用开放展馆、同馆厂商、厂商和产品可见性、审核通过及已发布版本限制，不读取草稿代替公开版本。
- `server/showroom_strict_sql_test.go`：严格模式复现与回归，覆盖无产品、同类多产品、多个分类、非产品分类关联、旧分类别名、父级累计、草稿/待审/下架、厂商隔离和展馆边界；回归首页、产品详情和 SEO 正文。

Schema 仍为 21，没有新增 SQL 迁移。网站、Android APK、001–021 迁移、基础目录和许可证逐文件与 Fix1 比较一致；只重建 Go 后端。App 地址仍为 `http://39.97.250.244:8089`，保留已安装的 1.8.0+35 即可。

## 验证结果及限制

当前后端执行 Go vet、隔离 MySQL 全量测试和严格模式专项；SEO Playwright 检查手机/桌面、中英文、启用/禁用 JavaScript、展厅与产品互链、规范地址及分页 noindex。确切数量与结果在包内 `manifest.json` 和 `verification/` 中。

本次 Go 全量 **162 项通过、15 项按环境开关跳过、0 失败**（含子测试）；严格模式专项 **6 项通过**（含主测试）；SEO 浏览器 **6 组通过、0 页面脚本错误**；打包换行与校验回归 **4 项通过**；Go vet 和 bash 语法检查退出码均为 0。

严格模式专项中的 `old-query-reproduces-1055` 故意执行旧 SQL，日志中的一条 1055 是复现断言，不能据此判断修复后的接口失败。其余接口必须按断言返回 200 或对应权限的 404。修改前的失败运行只作为复现证据，不计为测试通过。

网站前端 238 项、采购浏览器 89 组、Android 签名和 Flutter 172 项的结果来自前一次同日验证；对应网站和 APK 哈希未改变。它们不会冒充本次 Android 真机或本次完整采购浏览器重测。

安装包逐文件回读 SHA-256，清单及外层校验文件必须 UTF-8 无 BOM、LF 换行；Linux ELF 提供 amd64/arm64，两者关闭 CGO。未在 ECS 运行更新、未重新签名 Android、未向生产创建测试数据、未调整数据库全局模式。

## ECS /opt 更新步骤

手动把以下两项上传到 `/opt`：

- `zqzg-aliyun-20261024-schema21-fix2-linux.tar.gz`
- `zqzg-aliyun-20261024-schema21-fix2-linux.tar.gz.sha256`

在服务器 root 终端中执行：

```bash
cd /opt
sha256sum -c zqzg-aliyun-20261024-schema21-fix2-linux.tar.gz.sha256 && \
tar -xzf zqzg-aliyun-20261024-schema21-fix2-linux.tar.gz && \
cd /opt/zqzg-aliyun-20261024-schema21-fix2 && \
bash upgrade.sh --check
```

出现 `Preflight passed` 后：

```bash
bash /opt/zqzg-aliyun-20261024-schema21-fix2/upgrade.sh --apply
```

复用现有升级脚本：暂停服务，先备份当前有效数据库、主媒体、配置及运维指针，再切换新版本。保留现有恢复库、媒体目录、账户、密钥和 MySQL 严格模式。旧版本目录保留；备份和错误处理规则见主 `README.md`。不手动导入数据库包，不运行演示初始化。

出现 `Healthy. New release:` 后验收：

```bash
systemctl status zqzt-exhibition.service --no-pager
curl --noproxy '*' -fsS http://127.0.0.1:8089/api/v1/health
curl --noproxy '*' -sS -o /dev/null -w '厂商37 HTTP %{http_code}\n' http://127.0.0.1:8089/api/v1/public/showrooms/v-37
curl --noproxy '*' -sS -o /dev/null -w '厂商27 HTTP %{http_code}\n' http://127.0.0.1:8089/api/v1/public/showrooms/v-27
```

两个仍公开的厂商应返回 HTTP 200；App 中返回列表后重新进入展厅或点击重试。检查新启动服务进程的日志，不应继续出现该处理路径的 1055。旧进程的历史错误不会因修复被删除：

```bash
PID=$(systemctl show zqzt-exhibition.service --property=MainPID --value)
journalctl -u zqzt-exhibition.service _PID="$PID" --since '5 minutes ago' --no-pager |
grep -E 'event=db_query kind=error|panic|runtime error'
```

没有匹配日志时 grep 返回 1 属于正常结果。若健康或展厅接口失败，保留输出与升级备份，按包内错误处理说明排查；不要通过关闭 `ONLY_FULL_GROUP_BY` 掩盖查询问题。
