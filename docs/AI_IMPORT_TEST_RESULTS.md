# AI智能建馆测试记录

2026-11-18 操作优化已部署v1.12.2：15项Go AI、4项SDK、Go vet、TypeScript／Vite、8文件19项前端测试及独立系统设置3项复验通过。合成7产品11组交互检查、候选4组只读后台检查和正式HTTPS五种地图布局通过；补充037隐私旧句式迁移回归。真实画册未重跑，原资料未自动确认／提交，本轮不处理暂缓的企业资料提交异常。详见 [完整回执](RELEASE_1.12.2_20261118.md)。下列2026-11-17记录保留为历史基线。

执行日期：2026-11-17；本机Windows、Go1.26、MySQL隔离测试库、Edge／Playwright。未部署生产，未制作Linux交付包或原生App包。

## 已执行并通过

| 验证 | 结果／证据 |
|---|---|
| TypeScript | `node web/node_modules/typescript/bin/tsc -b web`通过 |
| Vite生产构建 | 输出 `.local/ai-import-web`通过；仍有现有大chunk提示；不覆盖原站构建 |
| Go静态检查 | `go vet ./...`通过 |
| AI业务测试 | `RUN_MYSQL_TESTS=1 AI_IMPORT_BROWSER_TEST=1 go test . -run '^TestAI' -count=1 -v`：7个测试通过，含浏览器测试；同源预览仅在development／loopback启用，生产及非loopback忽略的断言通过 |
| 百炼客户端 | 4个单元测试通过：SSE／缺失用量、脱敏、无效／截断JSON和Coding密钥、429逐次审计与重试 |
| 数据库迁移 | 从schema31升级32／33、重复执行、实际执行033及032回滚、确认业务表仍在、重新升级通过；全部在自动清理的临时库 |
| 任务恢复 | 同一任务不能重复领取；过期租约重新领取且token变化；语义失败会清除相关错误检查点、保留其他产品阶段；真实任务曾失败重试后继续完成既有阶段 |
| 文件入口 | 伪PDF、PNG伪装JPG、截断PNG、超限输入拒绝；合法PNG202，相同文件200重复提示 |
| 私有权限 | 未登录401；另一企业读取任务／原图／草稿修改404；跨任务AI资产关联拒绝；媒体删除引用保护409 |
| 参数与来源 | JSON数值原字符保留（含大整数、科学记数法），不浮点取整；复合猜测值拒绝；无来源企业字段丢弃；外页参数拒绝 |
| 图纸完整性 | 确定性PNG裁剪尺寸与原像素比对通过；整页不得直接作产品照片；技术图纸不进入照片水印 |
| 审核发布 | 未确认409；有问题无说明400；缺主图400且不冻结任务；版本过期409；重复提交不重复建产品；平台审核前公开404，通过后200 |
| 显式更新 | 同名同型号仅生成建议；未选择更新方式409；选择更新后不增加Content；旧公开参数保持原值至审批；老客户端编辑保留结构化详情 |
| 合并拆分 | 不保留原型号／结构的拆分400；正确拆分200；同型号不同结构合并后均保留 |
| 分批续审 | 成功批次后继续审核剩余项通过；已提交项仍不可修改 |
| 浏览器 | 360／390／768／1440宽度上传、续读、修改原值、未保存时禁确认、保存确认、审核前不可公开、审核通过后的型号与技术图展示；8组合成数据视口检查通过，无JS异常或横向溢出；两家真实任务另在390／1440宽度只读检查，共4组通过，无跨站请求 |
| 密钥审计 | 151个源码／文档／前端构建文件检查，实际密钥泄露0；私有环境、百炼原始配置、画册、样例会话均被Git忽略且未跟踪 |

测试文件：`server/ai_import_test.go`、`ai_import_update_test.go`、`ai_import_decode_test.go`、`ai_import_browser_test.go`、`internal/bailian/client_test.go`。浏览器合成数据仅供交互和公开链路验证，明确标识“非真实解析”。

## 真实API样例

| 样例 | 状态 | 原始产品草稿 | 型号条目 | 参数条目 | 待确认问题 |
|---|---|---:|---:|---:|---:|
| 河北洋柯机械有限公司 | review_pending，100% | 37 | 38 | 137 | 223 |
| 河北派一液压机械有限公司 | review_pending，100% | 7 | 36 | 120 | 186 |

两份原件26个物理页都进入本地解析。独立的真实源第1页派生PDF探测，两个模型各2份均HTTP200；完整原件Base64直传超时，业务使用本地解析后的原分辨率分页视觉输入完成。不是Mock识别。

累计已返回Token：洋柯304,758，派一269,615；每项各有1条用量未知的中断调用，不含在该累计数内。未配置价格表，费用保持null，实际账单需核对百炼。

以上统计含封面标题误识别、通用系列／订货编码占位等模型错误，不能当作验收应有数量。已逐页看概览并抽查型号与图纸，尚未完成逐参数、逐图纸的人工核验。重点问题：洋柯物理第9页共享表的型号范围及配件小图；派一通用型号、共享表行列归属、端口／订货代码及图纸裁剪边缘。待确认项保留在页面。首次真实输出包含英文描述／复核说明，仍作为原始草稿保留；后续提示已要求中文，并明确单产品、当前页范围，减少把其他已建目录产品或跨页证据误报缺失。本次未重新计费重跑全部原件，不能把提示调整当作新的准确性验收结果。所有产品最终业务ID仍为0，没有替两家企业确认或公开。

本地证据位于 `.local/ai-import-inspection/`：`real-samples.json`、`真实画册待审核清单.md`、四份`*-probe.json`、`go-ai-final.txt`、`bailian-tests.txt`、`go-vet-final.txt`、`web-build-final.txt`、`secret-audit.json`、`browser/report.json`及视口截图。含企业资料的原始证据不加入源码仓库。真实页面证据另见 `browser/real-ui.json` 和 `real-ai-*.png`。

## 既有全量测试失败：未隐瞒为通过

为区分新增问题，使用 `git archive HEAD web server scripts` 导出未修改基线到 `.local/ai-import-baseline`，同一机器、依赖与并发条件复跑，不改写用户工作区。

| 范围 | 当前与HEAD基线对比 |
|---|---|
| 前端全量Vitest，`--maxWorkers 4` | 两者均64文件：15失败／49通过；354测试：42失败／312通过；7个未处理错误。失败测试名集合一致，没有新增失败项 |
| Go非MySQL全量 | 两者均有 `TestDisabledSMSDoesNotIssueOrConsumeCodes` 失败：旧夹具以生产环境普通HTTP调用，实际426 `https_required`，期望503 `sms_disabled` |
| 既有MySQL重点回归 | 两者同样7项失败，名称见下；其余选中的配额／存储等测试通过。新增AI用当前协议及认证夹具单独验证 |

7项既有失败：`TestCompanyOnboardingRegionAndReview`（密码长度夹具）、`TestBusinessAndMediaIsolation`（旧认证接口）、`TestCompanyCategorySuggestionsAndAliases`、`TestProductV22ClassifiedReviewAndCompatibility`、`TestWatermarkUploadSnapshotLegacyAndQuota`、`TestWatermarkPublicNameAndOldSourceURL`、`TestWatermarkSnapshotVariantsAndPublicProtection`（后5项缺现行协议确认，`agreement_required`）。

另一次过宽筛选 `TestComplete` 命中既有 `TestCompleteBackupRestoreAndCutover`，采购默认关闭时旧夹具期望写入而返回503。随后限定 `^TestAI` 的最终验证通过；本次不将这次备份全链路失败计为AI通过或备份回归通过。

因此只能报告新增功能定向测试通过，不能声称整个仓库测试全绿或原有全部业务已全面验收。基线日志分别为 `web-baseline-limited.txt`、`go-baseline.txt`、`go-integration-baseline.txt`。

## 尚需人工／环境验收

- 厂商逐个核对真实画册的独立产品、参数数值、共享表适用范围、图纸尺寸和照片归属，修正错误并确认必填分类／协议，再提交平台审核。
- 大规模并发／长时间运行、100页极限文档、生产OSS及完整备份恢复链路、真实进程故障注入尚未作为本次通过项；已有租约、配额及引用保护有定向测试。
- 未完成正式Linux／Nginx代理部署、真实地图部署包四级验收、正式账号平台审批或原生App验收。

部署、回滚、接口与操作方法见 [AI_IMPORT.md](AI_IMPORT.md)。
