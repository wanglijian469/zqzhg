# AI智能建馆：本地MVP交付说明

日期：2026-11-17。适用于本次源码与 schema34。入口为企业工作台 `/merchant/ai-import`，任务页 `/merchant/ai-import/:id`。

当前模型策略：提取、复核与重试默认全部Flash，Max仅在任务中手动选择并确认。管理员可在系统设置中界面化管理百炼连接及加密密钥，见 [AI模型配置说明](AI_IMPORT_MODEL_SETTINGS.md)。

同日任务3修复：后台刷新保留编辑器、逐页／逐产品真实进度、独立展示区块保留和空响应修复，见 [劳斯耐斯画册修复记录](AI_IMPORT_TASK3_FIX.md)。

本次实现真实上传、异步百炼解析、私有草稿、企业差异选择、产品编辑／图纸裁剪／合并拆分、厂商确认、转交现有平台审核。**厂商确认不等于平台通过，`completed` 不等于公开。** 两家真实样例停留在 `review_pending`，未创建公开业务产品；浏览器公开链路仅在自动清理的隔离测试库验证。

## 1. 代码检查及复用

| 检查项 | 实际实现与适配 |
|---|---|
| 前端 | React19、TypeScript、Vite、React Router；沿用 `web/src/merchant-console.tsx` 工作台、Bearer会话、分类与媒体组件，新增懒加载页面。 |
| 后端 | Go、Gin，接口统一 `/api/v1/merchant/ai-import`；沿用错误结构、登录中间件与企业内容归属校验。 |
| 数据库 | MySQL、GORM，原schema31；032增加六类AI表，033补充产品更新目标和显式选择，034保存任务模型及max确认。业务企业／产品表不另建。 |
| 企业 | `Content(kind=vendor)`＋`Revision.Payload`＋`CompanyProfile`；AI只形成建议，已有企业逐字段覆盖。 |
| 产品 | `Content(kind=product)`关联企业，版本中新增可选 `product_details`，旧型号／参数／介绍字段继续生成；旧客户端遗漏详情时服务端保留已有详情。 |
| 媒体 | 复用私有Media、ObjectStore、本地／OSS、上传配额预占及水印快照。AI原件和派生图均计入现有空间。图纸原像素PNG裁剪，不进入照片水印流程。 |
| 权限 | 每条任务、草稿、资产记录owner；从会话推导企业，禁止跨企业读取、编辑、关联或提交。原件不会公开提供给模型。 |
| 发布 | 从既有处理器抽取 `createOwnedDraft`、`saveOwnedDraft`、`submitOwnedDraft`；旧接口沿用服务。AI只能创建／更新草稿并提交pending，无审批通过能力。 |
| 删除 | 原产品没有物理删除接口；AI删除为识别草稿软删除。已提交业务项不允许通过AI删除或修改。 |
| 限额 | 普通API原1MiB限制保留；仅精确AI上传路径独立30,000,000字节／100页。组图单图沿用原限制，总量／数量共同受限；原1000产品、12照片、默认200,000,000字节配额保留。 |
| 部署与日志 | 复用Linux二进制、Nginx、systemd、环境变量、维护锁及备份。新增脱敏模型调用审计、PDF依赖预检；本次未制作部署包。 |

主要文件：`server/ai_import*.go`、`server/internal/bailian/`、`server/content_services.go`、`web/src/ai-import.tsx`、`web/src/ai-import-company-fields.tsx`、`web/src/product-specifications.tsx`。

## 2. 配置百炼及PDF工具

复制 `server/.env.example` 的AI配置到服务器私有环境文件。真实密钥仅由后端读取；不要填入前端环境变量、提交Git或放入部署包。仅使用按量计费业务密钥，客户端拒绝 `sk-sp-` Coding Plan密钥。业务空间、地域、模型权限必须匹配。

| 配置 | 默认／用途 |
|---|---|
| `AI_IMPORT_ENABLED` | `false`，关闭时AI接口明确503，原手工业务继续可用 |
| `DASHSCOPE_API_KEY` | 必填，服务器私有业务密钥 |
| `DASHSCOPE_BASE_URL` | 必填；复制业务空间的HTTPS地址，结尾 `/compatible-mode/v1`，不含 `/chat/completions` |
| `DASHSCOPE_MODEL` | `qwen3.8-flash` |
| `DASHSCOPE_REVIEW_MODEL` | `qwen3.8-flash`，网站普通任务始终flash |
| `DASHSCOPE_TIMEOUT_SECONDS` | `360`，单次请求超时；暂时性网络、429、5xx最多再试2次 |
| `DASHSCOPE_PRICES_JSON` | 留空时费用为null；可填写核实过的每百万Token／每PDF解析页人民币单价，按模型记录估算依据，不当作账单 |
| `AI_IMPORT_MAX_FILE_BYTES` | `30000000`，所有任务输入累计字节上限 |
| `AI_IMPORT_MAX_PAGES` | `100`，PDF页数／有序组图数量上限 |
| `AI_IMPORT_CONCURRENCY` | `1`，数据库全局领取并发数，多个进程必须配置一致 |
| `AI_IMPORT_OWNER_CONCURRENCY` | `1`，企业活动上传任务数；失败重试仍要求企业当前无其他活动任务 |
| `AI_IMPORT_TASK_TIMEOUT_SECONDS` | `7200`，单轮任务时限；超时保留检查点，可人工重试 |
| `AI_IMPORT_WORKER_ENABLED` | `false`，只在关闭其他后台worker的隔离候选实例上单独启用；正常后台worker开启时无需此项 |
| `PDFTOTEXT_PATH`／`PDFTOPPM_PATH` | 留空从PATH查找；否则配置实际可执行文件绝对路径 |

费用配置示意：`{"MODEL_NAME":{"input":0,"output":0,"cached":0,"pdf_page":0}}` 仅展示字段，**不要把这些零当实际价格**。本次没有设置价格表，费用保留未知。缺失Token／缓存字段不当零；原始返回用量原样保存。模型调用表会区分已完成尝试、无响应用量和崩溃时未返回的调用。

本机Poppler位于 `.tools/poppler/poppler-26.09.0/Library/bin/`，来源和下载校验记录在 `.tools/poppler/download.json`；本次已补齐 `pdftotext`。Windows需保留完整依赖目录。Linux安装系统包 `poppler-utils` 后用服务运行用户执行 `pdftotext -v`、`pdftoppm -v`。不要把Windows工具直接放进Linux包。

模型请求使用Chat Completions（含SSE用量），超时与错误只返回脱敏错误码，不记录Authorization或供应商错误正文。参见[百炼PDF理解官方说明](https://help.aliyun.com/zh/model-studio/pdf-understanding)。

## 3. 独立模型连通性

`server/cmd/bailian-preflight` 不连接数据库，不上传到公开URL。程序读取进程环境变量，**不会自动加载 `.env`**，应由私有启动环境注入上述四项。不要把密钥作为命令行参数。

在项目根目录先运行 `./scripts/go-env.ps1`，再进入server：

```powershell
go run ./cmd/bailian-preflight --pdf '../厂商信息/洋柯画册.pdf' --probe-page 1 --attempts 1 --output '../.local/ai-import-inspection/yangke-flash-probe.json'
go run ./cmd/bailian-preflight --pdf '../厂商信息/洋柯画册.pdf' --probe-page 1 --model qwen3.8-max --attempts 1 --output '../.local/ai-import-inspection/yangke-max-probe.json'
```

派一使用其真实文件路径重复执行。`--probe-page 1` 会把真实源文件第1页渲染为紧凑单页PDF，再通过Base64文件输入；报告记录原件SHA-256、探测页及实际发送字节数。省略该参数发送完整原件；`--text`仅诊断文字请求；`--model qwen3.8-max`为显式高级模型选择，默认不调用max；`--trace`只打印网络阶段。

实际结果：洋柯、派一各自的flash和max单页PDF探测均HTTP200。完整原件直传尝试超时，**没有宣称整份原始PDF直接传输已通过**。因此业务流水线在本地解析全部页，再分批传递页面视觉证据，所有原文件保存在私有存储。未知调用用量不计零，也不把失败探测当成功。

## 4. 异步处理与数据映射

上传验证扩展名、魔数、完整图片解码、大小；PDF由pdfcpu校验结构、页数及加密状态。原件有SHA-256；同企业相同文件返回原任务提示，需要明确选择更新导入才新建任务。

MySQL领取锁＋90秒租约、20秒心跳，默认一个worker；宕机租约过期后重新排队。维护、备份写暂停锁覆盖处理过程。成功模型结果按阶段／输入指纹缓存，成功产品草稿保留；重试不覆盖人工草稿。响应不符合结构时先严格校验，可执行一次有记录的结构修复；技术数字用 `json.Number` 精确保留原字符，不能做浮点猜测。

处理顺序：原件保存 → 本地文字及原始嵌入图片 → 逐页目录区块 → 跨页目录合并 → 有来源的企业资料 → 按产品及页范围详细提取 → 所选模型（默认flash）对照原页复核 → 确定性裁剪 → 目录覆盖与数量校验 → 待审核。

无文字扫描页保留原分辨率嵌入图；混合页用Poppler渲染。目录可用1600边长预览，详细参数和复核使用原分辨率页面的JPEG传输副本；原件及高分辨率裁剪图不被压缩副本替换。来源同时记录PDF物理页序、左右分区和可选印刷页码。裁剪是归一化 `[left,top,right,bottom]`，转为原图整数像素区域，保留父资产关系。裁剪及资产接口只允许本人访问。

企业按字段来源白名单映射Payload／CompanyProfile，未提供来源的AI字段丢弃；已有企业默认不覆盖。产品详情包含系列、特点、应用、型号＋结构变体、逐项原值／单位／页码、分类图片。参数默认待核对，缺失为空；订货编码只记录已有选项，不生成全组合。照片进入已有水印快照；结构图、原理图、尺寸图、参数截图保留原像素，不生成式重绘。整页或原件资产禁止直接作为产品主图。

## 5. 审核、保存及续审

页面支持拖拽上传、有序组图、真实上传进度、历史任务、阶段进度／失败重试；企业资料逐字段差异、Logo原图选择或上传替换；产品编辑、参数表、图片主图选择／解除关联、源页裁剪、合并／按型号拆分、错误项删除、批量确认。

编辑后版本递增，已确认版本失效；待确认问题必须填写核对说明，审核记录关联具体版本。提交前复核分类、企业协议、主图、媒体实际存在和草稿版本，缺项时仍可编辑。相同产品名称＋完整型号／结构身份仅形成更新建议，厂商必须选择更新还是另建；歧义不会自动覆盖。

`publish` 锁定选择清单及版本、幂等键，逐项事务复用原草稿保存与提交服务，最终业务ID与提交同事务提交。失败批次可重试；需要编辑失败项时用 `reopen-failed`，已成功项保持锁定。成功批次后若还有未选草稿，用 `continue-review` 开始下一批。任务完成仅表示选中项转交原平台审核，公开数读取真实业务状态。

首次企业若尚不存在，必须人工补齐分类等必填项、确认协议后，才在最终保存阶段创建业务企业草稿。测试账号没有自动接受协议、没有自动批准企业／产品。

## 6. API清单

所有接口均要求企业Bearer会话，错误遵循原 `{error:{code,...}}`。路径前缀 `/api/v1/merchant/ai-import`。完整结构参见 `docs/openapi.json`。

| 方法／路径 | 输入及结果 |
|---|---|
| GET `/config` | 非敏感配置、模型配置就绪／PDF工具就绪及限额 |
| POST `/tasks` | multipart `files`（或单个`file`）、`request_id`；`mode=update`为显式重新导入。新任务202，重复200 |
| GET `/tasks` | `page`／`page_size`历史列表 |
| GET `/tasks/:id` | `status`／`stage`／`progress`／错误 |
| GET `/tasks/:id/result` | 企业／产品草稿、现有资料差异、来源资产、调用用量、固定提交清单、最终业务状态 |
| POST `/tasks/:id/retry` | 仅失败任务，从已落库阶段恢复 |
| PUT `/company/:id` | `version,data,selected_fields`；嵌套覆盖字段如`company_profile.website` |
| PUT `/products/:id` | `version,data,publish_mode`；更新可带`target_content_id,target_revision_id`，必须为本人产品及当前版本 |
| POST `/tasks/:id/review` | `items:[{kind,id,version,resolution}]`；kind为company／product |
| POST `/tasks/:id/products/merge` | `name,products:[{id,version}]`；仅未提交草稿 |
| POST `/products/:id/split` | `version,products:[Payload,...]`；必须保留原型号与结构身份 |
| DELETE `/products/:id` | 软删除识别项，不删除现有业务产品 |
| GET `/assets/:id` | 本人私有原件／图片二进制 |
| POST `/assets/:id/crops` | `source_page,panel,printed_page,image_type,crop_region,caption`；返回派生资产及父关系／坐标 |
| POST `/tasks/:id/publish` | `idempotency_key,company_version,products:[{id,version}]`；返回提交／失败数、最终ID，仍需平台审核 |
| POST `/tasks/:id/reopen-failed` | 清除失败批次锁，失败未提交项重新确认，已提交项不能编辑 |
| POST `/tasks/:id/continue-review` | 已成功批次后继续审核剩余未提交项 |

常用错误：`ai_disabled`、`bailian_configuration_required`、`ai_pdf_tools_required`、`invalid_pdf`、`invalid_image`、`file_too_large`、`ai_task_busy`、`stale_revision`、`ai_review_required`、`ai_resolution_required`、`ai_update_choice_required`、`ai_publish_conflict`、`ai_asset_wrong_task`、`storage_quota_exceeded`。

## 7. 真实样例与准确性边界

两份原件都在忽略Git的 `厂商信息/`。2026-11-17本地任务1洋柯、任务2派一已运行真实API，均100%进入 `review_pending`。没有硬编码产品名或固定数量。

| 实际输入 | 结构 | 本次模型草稿数 | 已返回的累计Token |
|---|---|---:|---:|
| 洋柯画册.pdf | 21,851,653字节／10物理页／无文字层／2516×1719扫描图 | 37 | 304,758 |
| 派一液压(6).pdf | 8,755,242字节／16物理页／无文字层／3355×2292双页扫描 | 7 | 269,615 |

以上是**原始识别草稿数量，不是人工验收正确产品数量**。Token来自已返回调用，用量未知的中断尝试不包括在该数中；价格未配置，不能据此声称准确账单。

已查看全部26页概览并检查目录、型号、图纸类型、参数页码。发现封面宣传语／系列标题误作产品，洋柯第9页独立共享参数表的型号适用范围、配件图片归属，以及派一共享参数表、订货编码／通用系列占位名称仍需人工核对。复核模型的问题已经进入草稿，原始数据保留供修正。**尚未完成逐参数、逐裁剪坐标的厂商人工签字验收，不能宣称准确性14项全部通过。** 详见本地 `real-samples.json` 和任务待确认事项。

本机候选入口：`http://127.0.0.1:8094/merchant/ai-import`。私有本地测试账号及任务会话存放于 `.local/ai-import-samples/ai-yangke.json`、`ai-paiyi.json`，仅在本机开发库使用，不生成正式对外账号。不要把这些文件或原画册加入Git／交付包。

## 8. 开发与测试

PowerShell从项目根执行 `. ./scripts/go-env.ps1`，复用现有MySQL。构建输出放入隔离目录，不覆盖原站点：

本机可直接运行 `./scripts/start-ai-import-preview.ps1 -Build` 构建并启动8094候选；已有构建时省略`-Build`，停止用 `-Stop`。该辅助脚本只允许loopback的 `zqzt_exhibition` 开发库，关闭其他后台定时作业，使用后端私有`.env`，不输出密钥。以下为分别执行的构建与验证命令：

辅助脚本启用 `LOCAL_PREVIEW_SAME_ORIGIN=true`，只在development且请求Host为loopback时将返回给浏览器的地址指向候选自身，避免本机数据库保存的8080地址把请求导向原开发实例。数据库内网站地址保持原值，production忽略该选项。

```powershell
node web/node_modules/typescript/bin/tsc -b web
Push-Location web
node node_modules/vite/bin/vite.js build --outDir ../.local/ai-import-web
Pop-Location
Push-Location server
go build -o ../.local/ai-import-server-final.exe .
$env:RUN_MYSQL_TESTS='1'
go test . -run '^TestAI' -count=1 -v
go test ./internal/bailian -count=1 -v
go vet ./...
Pop-Location
```

隔离测试自动生成 `zqzt_exhibition_test_*` 并核对实际库名，结束清理；不要把fixture改为正式库。浏览器测试需Playwright和Edge（脚本路径可按环境修改）：构建后在server设置 `AI_IMPORT_BROWSER_TEST=1`、`RUN_MYSQL_TESTS=1`，运行 `go test . -run '^TestAIImportBrowser$' -count=1 -v`。合成夹具明确标记非真实画册，绝不冒充真实解析结果。

`scripts/verify-ai-import-samples.py --require-review`读取本机私有测试会话，仅允许loopback，生成实际任务报告，不修改／提交业务数据。

最新验证结果与基线失败见 [测试记录](AI_IMPORT_TEST_RESULTS.md)。未执行的生产部署、正式账号审核和原生App验收不计通过。

## 9. 部署及回滚

本次未向正式服务器写入测试资料，也没有制作Linux／APK升级包。正式部署应另行审批实际产物，至少执行：

1. 备份数据库和对象存储，保留可恢复的加密配置；先关闭AI开关，发布包含032、033、034的后端与对应前端。既有迁移入口自动执行，不能只复制前端。
2. 安装并用systemd实际服务用户验证Poppler；配置服务环境文件权限600，使用正确业务空间按量密钥。确认临时目录／对象存储可写、现有配额充足。先使用默认单worker，根据实际内存／接口限额评估并发。
3. Nginx对精确上传路径保留足够请求体（默认30MB输入加multipart开销，建议32m）及上传时限。不得为所有API扩大原上限；现有代理上限若已足够则复用。
4. 启用AI，先用授权测试账号检查配置、上传、重启恢复、私有媒体隔离、厂商确认、平台审核前404与通过后网页／手机详情。检查日志不包含密钥或画册Base64。
5. 备份恢复时同时保存数据库AI表及其媒体；已将AI资产纳入对象引用枚举与删除保护，不能只复制六张表却遗漏图片。
6. 如果制作Linux部署／升级包，继续遵守 `docs/MAP_DEPLOYMENT_REQUIREMENTS.md`：真实边界快照、编码映射及生产代理路径，全国→河北→邢台→宁晋、真实统计、手机布局和资源失败重试必须在实际交付产物上验证。本次候选构建不代表完成该部署验收。

常规回滚：关闭 `AI_IMPORT_ENABLED` → 停止worker并确认无处理中任务 → 切回程序／前端版本。保留AI表、原件、裁剪图、已提交Content及Revision，便于重新启用。老后端可能在编辑时丢弃不认识的详情字段，回滚旧程序期间应暂停编辑这类导入产品。

明确需要数据库降级时，先导出AI表并验证可读，先执行 `deploy/linux/rollback-ai-import-034.sql` 回退模型字段，再按需离线执行 `rollback-ai-import-033.sql`（仅更新建议字段）或 `rollback-ai-import-032.sql`（全部AI表及32／33／34迁移标记）。这些脚本**不会删除企业、产品、业务版本或媒体**；不可在worker运行中执行。恢复旧版本不自动执行删表。重新升级可通过现有迁移入口重建AI表，业务数据仍保留。
