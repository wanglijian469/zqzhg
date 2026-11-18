# AI模型配置与默认Flash策略

2026-11-18更新：厂商模型选择已取消；管理员分别配置新任务的提取和复核模型，既有任务及重试使用原快照。当前实现以 [AI_IMPORT_BATCHES.md](AI_IMPORT_BATCHES.md) 为准，下文为schema34历史记录。

2026-11-17，schema34，本机开发候选。入口：管理员后台 → 系统设置 → **AI模型配置**，直接地址 `/admin/settings?tab=ai`。

## 费用策略

- 普通任务的目录、企业资料、产品提取、参数复核、结构修复和重试均使用 `qwen3.8-flash`。
- 不根据文档长度、失败次数或复杂度自动升级模型。
- 复杂任务需要厂商在创建任务时选择 `qwen3.8-max`，并勾选确认较高费用。该任务全程采用所选模型。管理员可以关闭手动max选项；服务端也会拒绝未经确认／未获允许的请求。
- 新任务保存模型选择、提取／复核模型及max确认时间；任务重试沿用该选择。旧任务后续处理默认Flash，已有模型调用记录和历史费用数据保留原样，不重写为Flash。
- 网站普通任务固定Flash，即使旧环境变量曾配置 `DASHSCOPE_REVIEW_MODEL=qwen3.8-max`，也不会因此自动调用Max。

## 管理员页面

可配置百炼官方HTTPS业务空间API地址、15—600秒的单次超时、密钥来源，以及是否允许厂商手动使用max。页面显示默认模型、复核模型、手动高级模型及配置状态。

密钥操作支持：保留当前密钥、填写／更换后台密钥、清除后台密钥并使用服务器环境变量、停用业务密钥。配置保存后即时供后续调用读取，不需要重新构建网站。运行中任务的模型选择保持不变；已发出的模型请求不属于新的配置测试。

“测试已保存的连接”只发送短文本，不上传画册，**会产生API费用**。测试默认Flash；Max测试同样需要显式选择和费用确认。保存配置不会自动调用任何模型。

仅超级管理员可以读取或修改；厂商和受限操作员不能访问。API Key输入用密码框，提交后清空，不写localStorage／sessionStorage。后端不回传明文、掩码片段或加密字符串，只返回是否已配置及来源。后台密钥复用现有AES-GCM加密存储于 `system_settings` 的独立配置项，日志记录固定的操作说明，不记录密钥。

生产环境保存后台密钥前必须提供有效的 `CONFIG_ENCRYPTION_KEY`，备份恢复需保留同一主密钥。开发环境沿用现有私有 `development.key`。不配置后台密钥时仍可使用后端 `DASHSCOPE_API_KEY`／`DASHSCOPE_BASE_URL`；真实密钥和环境文件不提交Git或打包。

`AI_IMPORT_ENABLED`仍是服务器功能开关，本页面不擅自修改站点的部署级开关。独立连通性程序默认Flash，手动高级测试可明确使用 `--model qwen3.8-max`。

## 接口

| 接口 | 行为 |
|---|---|
| GET `/api/v1/admin/settings/ai-models` | 返回脱敏配置、版本及就绪状态 |
| PUT `/api/v1/admin/settings/ai-models` | `version,base_url,timeout_seconds,allow_manual_max,key_action,api_key?`；乐观版本校验，密钥只在replace操作提交 |
| POST `/api/v1/admin/settings/ai-models/test` | `model_mode:flash|max,confirm_max?`；手动付费短文本测试，记录模型、尝试、Token、费用估算依据 |
| GET `/api/v1/merchant/ai-import/config` | 仅返回安全的模型策略与上传限额，不含地址、密钥或密文 |
| POST `/api/v1/merchant/ai-import/tasks` | multipart新增 `model_mode`（默认flash）；max必须同时 `confirm_max=true`，并通过管理员允许检查 |

新增迁移 `034_ai_import_model_policy.sql`；回滚用 `deploy/linux/rollback-ai-import-034.sql`。先关闭AI／停止worker并导出任务元数据再降级，保留后台加密配置和所有草稿、媒体、业务数据。完整AI删表回滚032已同步处理34迁移标记；分步降级顺序为034→033→032。

## 验证

后端覆盖默认Flash、旧环境Max不升级、重试仍Flash、显式Max选择、管理员禁用、密钥加密及跨应用实例读取、不回显、权限隔离、版本冲突及034升级／回滚重建。模型请求使用模拟传输验证这些分支，不当作真实模型解析。

前端覆盖密钥提交后清空且不写浏览器存储、默认Flash、Max未确认不可提交、连通测试显式选择，以及原系统设置和AI刷新回归。浏览器在自动清理的隔离库实际操作390／1440宽度配置页，验证保存、不回显、Max确认及手机布局；没有点击真实付费连通测试按钮。截图位于 `.local/ai-import-inspection/browser/ai-model-settings-*.png`。

最新结果：11项Go AI业务／浏览器测试、4项Bailian客户端测试通过，连通性命令编译通过；4个前端测试文件共9项测试通过；TypeScript、Vite构建和Go vet通过。浏览器共10组检查（含既有8组及新增2组管理员手机／桌面检查）通过，无脚本异常或横向溢出。日志为 `.local/ai-import-inspection/model-settings-{go-final,sdk-final,vet-final,web-tests,preview-start}.txt`。

本机服务已实际验证 `/merchant/ai-import/config` 返回默认和复核均为Flash；两家初始样例的37／7项草稿及Token记录保持原样。未重复运行已知失败的全仓库套件，既有基线问题见 `AI_IMPORT_TEST_RESULTS.md`。

本次未部署生产、未重新解析已完成的三份画册，未重新调用真实Max；历史草稿及审核状态保持原样。
