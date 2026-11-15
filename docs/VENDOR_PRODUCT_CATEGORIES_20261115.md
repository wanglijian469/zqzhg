# 企业独立产品分类（2026-11-15）

本轮完成后端、网页和 Flutter App 的本地实现、迁移与验收。未部署生产、未发布正式 App、未替换官网下载包。保留此前两级产品面包屑、企业经营信息与生产能力布局，以及工作区原有其他修改；未修改地图或部署包。

## 使用方式

厂商主营、兼营仍使用平台厂商分类。产品选择本企业维护的产品分类，例如所属行业大类“液压泵、阀、输出装置零部件”，企业产品分类“液压回转马达”，产品“2.5K系列液压回转马达”。企业分类不插入平台分类树。

- 发布产品时可以搜索、新增企业产品分类，创建成功立即选中；产品内容继续执行原有审核与协议规则。该位置不再显示平台末级分类、分类推荐或“待管理员规范”。草稿允许暂不选择分类，提交审核时校验。
- 网页工作台和 App 增加“产品分类管理”，支持名称、选填英文名、排序、所属行业大类、启停用和删除。网页管理员在指定企业资料下代为维护。
- 新增时默认主营大类，主营、兼营大类优先；可选同展馆其他启用的正式大类。其他企业可以创建同名分类；同企业同大类中，去首尾空白及忽略大小写后同名不可重复。相同名称位于不同大类时，选择项补充行业名称以区分。
- 展厅分类数量只统计可公开产品，不受草稿影响。停用分类不能被新选，已有产品继续显示、检索和计数。任何历史产品版本仍引用时均不能删除。
- 展厅关键词通过后端匹配产品名称、型号、企业分类中英文名；分类筛选、分页与产品返回展厅的状态保留。产品详情以企业分类为主、行业大类为辅助，中英文采用现有回退规则。

## 数据与迁移

下一版增量迁移为 `server/migrations/031_vendor_product_categories.sql`。服务启动通过现有迁移执行器创建三张表，再运行 Go 回填，成功后才登记 schema 31；仅手动执行 SQL 不会完成历史数据回填。

| 表 | 用途 |
| --- | --- |
| `vendor_product_categories` | 企业归属、展馆、行业大类、名称及英文名、排序、启用状态；企业＋行业大类＋标准化名称唯一索引 |
| `revision_vendor_product_categories` | 每个产品版本的企业分类绑定，按企业、分类、版本建立索引；公开版、草稿、待审版独立 |
| `vendor_product_category_sources` | 各企业旧平台分类到企业分类的兼容映射 |

`Payload` 新增 `vendor_product_category_id`，响应在条目层增加 `vendor_product_category` 描述；历史版本通过独立绑定表补齐响应，不改写已保存的审核 JSON。迁移保留正文、媒体、名称及旧平台索引。按企业和来源复用分类，绝不跨企业复用。

大类归属沿原分类父级向上定位，并依据稳定 CIAME 编码映射厂商/产品大类，不按名称推测。无法确定的历史分类保留原名及产品绑定，返回 `industry_category_id: 0` 和 `needs_industry: true`，管理页标记“待完善归属”；既有引用保留，整理归属后可供新产品选择。回填按250版本分批提交，可安全重跑；已建立的版本绑定不重复创建。

未来执行升级时，先通过既有备份功能保留完整备份，在隔离环境用本版服务启动迁移，再核对分类数、版本绑定数及“待完善归属”记录。迁移中断后重新启动同版服务继续即可。schema 31 备份包含三张新表；恢复兼容校验的支持上限已更新至31，高于支持版本的备份仍被拒绝。回退到旧服务前应恢复升级前备份，不能只删迁移登记或新表。

## 接口与兼容

路径均位于 `/api/v1` 下，完整契约见 [OpenAPI](openapi.json)。

| 请求 | 路径 |
| --- | --- |
| 企业端查询 / 新增 | `GET / POST /merchant/product-categories` |
| 企业端修改 / 删除 | `PUT / DELETE /merchant/product-categories/:categoryID` |
| 管理员查询 / 新增 | `GET / POST /admin/vendors/:id/product-categories` |
| 管理员修改 / 删除 | `PUT / DELETE /admin/vendors/:id/product-categories/:categoryID` |

查询返回 `items`、`industry_categories`、`preferred_industry_ids` 和 `vendor_id`。保存接收 `name`、`name_en`、`industry_category_id`、`sort_order`、`enabled`。同名冲突和已有引用不可删除返回409；跨企业管理返回404，跨企业产品绑定返回400。管理员读取要求 `product.read`，写入要求 `product.write`，目标企业必须显式指定。企业行锁与唯一索引共同处理并发新增，绑定与删除也按企业串行校验。

展厅响应增加 `vendor_product_categories`，保留原 `categories`。产品列表新增 `vendor_product_category_id`，必须同时指定 `vendor_id`，未指定企业或绑定其他企业分类时返回空结果。公开数量和筛选均关联公开产品版本。

旧 `product_category_id`、`category_id`、响应字段及平台版本索引继续保留。旧 App 保存其他字段时，新分类关联沿用原草稿；明确变更旧分类字段时，通过企业来源映射转换。新的企业分类通过稳定大类编码纳入平台大类筛选，无法映射时不制造分类关系。旧响应缺少企业分类列表时，网页和 App 继续使用旧分类展示作为兼容回退。

## 验收结果

全部数据库测试使用本机自动创建、清理的隔离 MySQL 数据库，没有读取或改写生产数据。

| 验证 | 结果 |
| --- | --- |
| 新功能 Go 测试 | 7项通过，含企业隔离、越权绑定、同名/并发新增、停用、历史引用删除保护、草稿与公开隔离、旧客户端兼容、管理员权限 |
| 历史迁移 | schema30→31及直接重复回填通过；包含公开、草稿、待审版本；原名/英文名保留、审核JSON逐字节不变；无法映射的大类正确标记待完善 |
| 加密备份恢复 | 实际创建备份及隔离恢复，核对schema31、企业分类、版本绑定和旧来源映射三张表内容 |
| 后端既有回归 | 11项通过，涵盖SEO首次HTML/结构化数据、CIAME历史迁移、taxonomy和严格GROUP BY；另有下述2项既有失败 |
| Go静态检查 | `go vet ./...`通过 |
| 网页 | 8个相关测试文件44项通过；TypeScript与Vite构建通过，保留现有大分块提示 |
| 实际浏览器 | 本地构建产物＋真实隔离后端，360/390/768/1024/1440px × 中英文，共10组通过；创建即选中、默认大类、长中英文分类名称、管理、公开数量/筛选、第二页往返、首次HTML/JSON-LD与客户端详情分类及两级面包屑，无页面JS错误或横向溢出 |
| Flutter | 分析无问题；4个功能测试文件23项通过，含64组既有分类布局子场景、6组长分类管理页宽度/语言/大字号组合、创建即选中、修改/停用/排序与筛选搜索状态保留；另2项截图测试通过 |

两项既有失败为 `TestMerchantRootCategoriesScopePublishAndNormalization` 和 `TestCompanyCategorySuggestionsAndAliases`：旧测试发布夹具未完成当前要求的协议确认，返回 `agreement_required`。已在保留本轮修改前源码的独立副本中复现相同失败，未将其计为通过，也未降低现有发布校验。日志位于 `.local/vendor-product-categories-20261115/backend-regression.log` 和 `baseline-regression.log`。

既有完整备份流程测试 `TestCompleteBackupRestoreAndCutover` 的采购夹具也会受到当前版本 `proc_disabled` 限制，本轮使用独立的新分类真实加密备份恢复测试验证分类数据。没有执行生产切换。App采用Flutter组件测试与原生控件渲染截图，尚未做实体手机、真机键盘或完整安装包验收。

最后拟额外补跑的地图资源校验、规则同步和PDF资源准备组合命令被自动审批拒绝，理由是可能生成或修改资源，与本轮不变更地图资源的限制存在冲突；该组合未执行，也未尝试绕过。验收中的构建结果是此前已完成的TypeScript检查与Vite构建，未将额外资源准备步骤计为通过。

结构化结果：[汇总](test-output/vendor-product-categories-20261115.json)、[真实后端浏览器检查](test-output/vendor-product-categories-browser-20261115.json)。

## 验收截图

截图仅包含本地隔离测试数据。网页手机全页截图中固定顶栏和底栏由浏览器按当前视口位置渲染；实际使用可正常滚动。

| 场景 | 桌面 / 手机 |
| --- | --- |
| 发布时新建并选中 | [1440px](screenshots/vendor-product-categories-20261115/editor-selected-1440-zh.png) · [390px](screenshots/vendor-product-categories-20261115/editor-selected-390-zh.png) |
| 新增分类及所属行业大类 | [1440px](screenshots/vendor-product-categories-20261115/new-category-1440-zh.png) · [390px](screenshots/vendor-product-categories-20261115/new-category-390-zh.png) |
| 网页分类管理 | [1440px](screenshots/vendor-product-categories-20261115/manager-1440-zh.png) · [390px](screenshots/vendor-product-categories-20261115/manager-390-zh.png) |
| 展厅分类筛选、产品数量 | [1440px](screenshots/vendor-product-categories-20261115/showroom-1440-zh.png) · [390px](screenshots/vendor-product-categories-20261115/showroom-390-zh.png) |
| 产品详情 | [1440px](screenshots/vendor-product-categories-20261115/product-1440-zh.png) · [390px](screenshots/vendor-product-categories-20261115/product-390-zh.png) |
| 指定企业管理员管理 | [1440px](screenshots/vendor-product-categories-20261115/admin-company-1440-zh.png) |
| App分类管理 / 编辑 | [390px管理](screenshots/vendor-product-categories-20261115/app-manager-390-zh.png) · [390px编辑](screenshots/vendor-product-categories-20261115/app-edit-category-390-zh.png) · [768px管理](screenshots/vendor-product-categories-20261115/app-manager-768-zh.png) |

## 复跑

网页按 `web/package.json` 的 prebuild、TypeScript、Vite 顺序执行。测试命令：

```powershell
# web目录
node node_modules/vitest/vitest.mjs run src/vendor-product-categories.test.tsx src/product-editor.test.tsx src/taxonomy.test.tsx src/showroom.test.tsx src/showroom-v2.test.tsx src/product-quality.test.tsx src/company-info.test.tsx src/seo.test.ts --maxWorkers=1
node node_modules/typescript/bin/tsc -b
node node_modules/vite/bin/vite.js build

# server目录：先构建网页；本地MySQL须可用
. ../scripts/go-env.ps1
$env:RUN_MYSQL_TESTS='1'
$env:RUN_VENDOR_CATEGORY_BROWSER='1'
go test -v -run '^TestVendorProductCategory' .
go vet ./...

# 项目根目录
. ./scripts/flutter-env.ps1
Set-Location mobile
& $taskFlutter analyze
& $taskFlutter test test/vendor_product_categories_test.dart test/taxonomy_test.dart test/merchant_form_flow_test.dart test/product_experience_test.dart
$env:VENDOR_CATEGORY_CAPTURE='1'
$env:APP_V18_CAPTURE='1'
& $taskFlutter test test/vendor_product_categories_capture_test.dart
```

浏览器脚本 `scripts/verify-vendor-product-categories.cjs` 由 Go 测试启动，临时会话通过标准输入传递，不写入验收报告。App截图测试需要本机微软雅黑及 Flutter Material 字体资源。
