# OSS 改造测试记录（2026-11-18）

本文保留 11 月 18 日的本机基线。后续真实云端结果、13 项更新后 Go 测试及未完成项见 [2026-11-19 验证记录](OSS_ACCEPTANCE_20261119.md)，以新记录和候选 manifest 为准。

当前交付为代码与候选产物，未上线、未迁移真实企业文件。测试中的 OSS 服务由本机 HTTP 合约模拟器提供；MySQL 使用自动创建的隔离库。模拟成功不能替代真实阿里云验收。

## 已通过

| 检查 | 结果与边界 |
|---|---|
| Go 定向回归 | 10 项通过；包括 AI 草稿隔离及裁剪、上传重复、原加密 ZIP 备份恢复、官方 OSS SDK Range 合约、新上传安全／视频封装规则、会话与配额／历史映射／公开复制／流式备份及隔离恢复、响应敏感信息、管理员额度及水印回归 |
| 039 升降级 | 额外 1 项 MySQL 测试通过；038 升级、无导出确认拒绝降级、备份保护拒绝降级、成功降级及重新升级 |
| Go vet | 通过 |
| 网页 | TypeScript 通过；5 个文件共 24 项 Vitest 通过（主回归 20 项＋私有图片访问 4 项）；Vite 生产构建通过 |
| Android | Flutter analyze 无问题、4 项测试通过；原生工作台桥接 4 项 Node 测试通过；同签名 Release APK 构建成功 |
| APK 保护 | 包名 `com.zqzt.dalunongji`，版本 `1.12.0+53`，与上一版证书一致；仅生成独立候选，未替换正式下载 |
| Linux 构建 | linux/amd64、CGO_DISABLED 的 ELF 可执行文件已生成；尚未在真实 Linux/ECS 启动验收 |

上传测试覆盖：非法类型与格式伪装、厂商间越权、精确分片边界、响应丢失恢复、取消后保留预占至实际清理、重复 worker 不重复建媒体、技术原图无损保留、产品照片及缩略图 WebP。公开复制测试覆盖哈希校验、重试不重复计量、复制失败保留旧公开指针及下架后回收。备份恢复测试包含加密 SQL、文件校验、未登记文件私有隔离清单、逻辑媒体 ID 与远端映射保留、恢复后冻结原生产上传键及不切换正式库。

## 地图与实际环境

本机隔离克隆库、`APP_ENV=production`、独立 Node 转发代理路径 `http://127.0.0.1:18945`，从最终 Windows 候选程序及生产前端验证全国→河北→邢台→宁晋。数据库统计分别为 11／10／8／8，几何哈希与 77 个开发快照一致；覆盖 1440、1024、768、390、360 宽度和注入 503 后重试。另以最终 APK 的内嵌资源执行同一路径检查。实际完成结果写入候选包 evidence；缺少任一报告时 manifest 不会标为通过。开发演示资源保留 DataV／高德来源及原用途声明，不冒称正式授权地图。

这不能代替 Linux 二进制在 ECS 正式 HTTPS 代理下的验收。正式上线前仍须从待上线包完整复验，不得沿用本机通过结论。

ECS 只读预检发现 `ZQZHG-SMS-Role` 对两个业务 Bucket 的 `ListMultipartUploads` 均返回 `oss_permission_denied`。此次预检未创建对象、未修改数据库，临时测试程序已移除。需要云管理员合并最小 OSS 权限并保留原短信策略，再执行管理员明确触发的合成对象测试。

## 尚未验收及已知限制

- 真实 OSS 的签名 PUT／分片、CORS、匿名私有拒绝、Range、公开目录策略、复制与删除；独立操作 STS 的真实备份及隔离恢复。
- 洋柯 67、派一 79 的真实迁移、真实配额及业务公开访问；生产文件没有删除或迁移。
- ECS 候选启动、正式代理地图、正式发布切换与演练回滚；目前磁盘余量紧张，禁止复用旧全量本地归档部署脚本。
- Android 真机断网续传、大画册流式保存、原生 PDF 签名过期刷新、视频及升级安装；网页模拟器和自动测试不能替代真机。
- schema 038 的独立新式云备份兼容分支已实现，真实生产 038 备份仍需以独立操作凭据验证。
- 未声明全仓库测试通过。历史 `TestBusinessAndMediaIsolation` 仍使用已退役 `/auth/sms` 接口，会得到 410；本轮不恢复该旧登录接口。原备份测试的已关闭采购功能改用历史数据库夹具，未重新开放采购业务。

## 复现

PowerShell 先运行 `. ./scripts/go-env.ps1`。后端设置 `RUN_MYSQL_TESTS=1` 后运行 `go test -run 'TestCloud|TestOSSOfficial|TestSettingsQuota|TestWatermarkSnapshot|TestAIIsolationCropIntegrityReviewAndSubmission|TestAIUploadContentValidationAndDuplicate|TestCompleteBackupRestoreAndCutover' -count=1 ./`，再执行 `go vet ./...`。务必使用本机隔离测试配置，不将测试指向正式库。

网页在 `web` 执行类型检查和 `vitest run src/oss-upload.test.ts src/product-editor.test.tsx src/system-settings.test.tsx src/free-service.test.tsx src/media-image.test.tsx --maxWorkers=1`，再生产构建。Android 在 `mobile` 执行 analyze／test，再运行 `scripts/build-apk.ps1 -BuildOnly`。从根目录执行 `node --test scripts/android-workspace.test.cjs`。

候选归档运行 `python scripts/package-oss-candidate.py`，仅打包允许的源码变更、Linux 产物、77 个边界文件、迁移、配置示例、云策略和脱敏报告；不自动部署。
