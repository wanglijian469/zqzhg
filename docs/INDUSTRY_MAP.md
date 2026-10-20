# 产业地图／全国厂商地图

当前默认图形已改为可点击下钻的 ECharts 行政区方块示意图，页面标注非真实地理边界，不再依赖边界资源。当前行为及验收以 [示意图说明](FACTORY_MAP_SCHEMATIC.md) 为准。下文保留接口、迁移及此前阶段记录。

当前页面已更名为“厂商地图”，并改为固定视口工作区、自适应分页及手机地图／厂商切换；不再提供下文首版的长区划卡片布局。真实地图边界授权及公开发布核验材料尚未取得，图形下钻未完成。当前实现和验收状态以 [厂商地图调整说明](FACTORY_MAP_COMPACT.md) 为准；下文保留首轮接口与迁移记录。

## 交付范围

入口：网站导航“产业地图”，路径 `/regions`。跟随当前展馆，按全国→省级→市级→区县→企业展厅浏览；直辖市及省直辖县级节点遵循目录实际父子关系。PC 左侧区划、右侧厂商列表；移动端上下排布，可切换全部、区域和厂商视图。关键词、行业、地区、分页、未细分筛选和移动视图保存在 URL；浏览器前进后退及展厅返回恢复原条件。

本版采用行政区划列表模式，**不发布真实地图图形**。ECharts 渲染、按区域资源加载、浏览器缓存和故障降级已经接入，待最终地图展示形式的合规核验完成后方可启用。未接入高德、百度、OSM、Leaflet、外部地图 API、经纬度或选点；未新增位置授权字段、Redis 或地图服务器。

行政目录复用 `server/catalog/administrative-regions-2023.json`，来源及许可记录见同目录的 `administrative-regions-source.json`。该目录为2023-06-30历史版本，覆盖中国大陆31个省级行政区；页面明确标注范围和年份。**行政区划名称／代码目录的许可不是行政边界图形的授权或审图证明**。

## 数据与统计口径

厂商仍是 `contents.kind='vendor'`，修订资料仍在 `revisions.payload`。仅统计开放展馆内、公开可见、已发布修订为 `approved` 的厂商；草稿、待审及驳回资料不覆盖现有公开版本。无已发布资料、下架及筹建展馆的厂商不展示。

Schema16 增加 `revisions.province_code / city_code / district_code`，均为可空字符串，只作该修订的行政归属快照，不通过公开响应暴露这些内部列。沿目录父链解析编码，禁止把六位省直辖县级市当成三级节点，也不按字符串长度推断层级。

首次升级按修订 ID 每批最多500条回填，仅处理厂商修订：

1. 有效的该修订 `region_code` 优先。
2. 否则仅在既有 `region_id` 对应行政编码的目录全称与该修订地区文字一致时复用映射。
3. 否则使用去空白后的唯一完整地区名称匹配；重名、模糊名称、目录外名称保持未归属。

不改写原始 payload、企业名称、地址、审核结果或发布指针。每批事务提交，全部回填成功后才记录版本016；错误指出修订 ID，修复原数据后重启可以重试。DDL 使用现有迁移器的列／索引存在检查，兼容部分完成的重试。升级前应备份；大库安排维护窗口，避免回填期间并发资料写入。

有城市但没有区县的厂商计入城市及上级，在城市页进入“未细分地区”；没有可核实省级归属的厂商进入全国“待完善地区”。叶节点不再拆分。在非叶节点，`sum(items.vendor_count) + unassigned_count == total`。所有数字是企业去重数，不是区划个数或地图点数。

## 接口

接口前缀 `/api/v1`，无需登录，沿用现有公开读取限流、错误包装及 `Retry-After`。统计和厂商列表均 `Cache-Control: no-store`。

| 接口 | 参数 | 返回 |
|---|---|---|
| `GET /public/industry-map/regions` | `region_code` 可省略代表全国；`hall` 默认 agriculture；`q`、`category_id` 可选 | `region`、`trail`、下级 `items`（包含 `vendor_count`）、`total`、`unassigned_count`、`version`、`historical`、`coverage` |
| `GET /public/industry-map/vendors` | 同上，加 `unassigned=1`、`page`、`page_size` | 现有公开厂商 `items`，及 `total`、`page`、`page_size` |

`unassigned=1` 指当前区域无法细分到下级的厂商，全国时指无省级归属；叶节点返回空列表。分页默认20、最多20，超过20且通过公共参数校验的请求归一到20。无匹配返回零统计／空数组。非法地区编码、分类格式、分页或 `unassigned` 返回400；未知分类 ID 返回零结果。

查询例：`/api/v1/public/industry-map/regions?region_code=13`、`/api/v1/public/industry-map/vendors?region_code=1305&unassigned=1&page=1`。关键词和行业匹配复用找厂家逻辑，包括公开产品关键词／分类；关联产品用 EXISTS，不重复计算厂商。

每次统计仅执行当前下级编码的一次分组聚合，不获取全部厂商 payload，也不按每个行政区循环查库。列表固定数量分页，沿用批量加载公开修订，按精选、排序号和厂商 ID 稳定排序。

新增索引 `idx_revision_administrative_public(province_code,city_code,district_code,status,category_id,content_id)`、`idx_content_map_public(hall_id,kind,visible,published_revision_id,id)`。6000条隔离夹具的选择性省级查询 EXPLAIN 使用上述两个索引；小数据下优化器可能选择全扫描，不能以小库耗时代表生产 SLA。

备份导入支持 Schema10–16，旧备份恢复后执行迁移并回填。代码回退时保留额外区划列和版本记录，不删除企业资料；Schema16备份须使用支持该版本的后端恢复。

## 地图资源接入与核验

默认 `web/public/maps/manifest.json` 的 `resources` 为空。首屏只请求全国统计和首批企业，图形未核验时不加载 ECharts 渲染器或任何边界文件。区划变化按需读取对应资源；图形失败时保留区划列表及厂商结果。

资源清单的键为当前区域编码，全国为 `national`，文件位于同源 `/maps/`。每项必须包含：`url`、`sha256`、`version`、`catalog_version`、`source`、`license`、`review_number`、`review_evidence`、`publication_scope`、`verified_by`、`verified_at`、`presentation='industry-choropleth'`、`approved=true`。版本必须与统计目录2023-06-30对应；资源采用 GeoJSON FeatureCollection，区划要素通过 `properties.adcode` 绑定编码，不能依赖地名唯一性。

`review_evidence` 指向仓库 `docs/` 下的人工核验记录。记录必须识别文件哈希、审图材料、许可、公开范围、核验人和最终着色／交互展示形式，不能仅复制底图下载地址或原始审图号。全国图形须另行核验完整疆域及附图，并在证据中记录“全国完整疆域及附图核验”；不能由31省统计目录生成或裁切全国轮廓。港澳台等超出当前统计目录的图形要素不填造零企业数据，不提供虚构下钻。

核验依据及待补材料见 [地图核验记录](MAP_RESOURCE_REVIEW.md)。资源在正式登记前由相应负责人员核实，软件工具仅检查材料存在性、编码映射及完整性，**不能代替主管机关审核或法律适用判断**。

执行 `node scripts/verify-map-resources.mjs` 检查资源清单，已接入 `npm run build` 前置步骤。前端再次检查发布标记和 SHA-256；只缓存校验通过的边界，键包含文件哈希。内存最多12项，CacheStorage 最多24项；缓存损坏自动删除后重新读取，存储禁用时仍能显示合格资源。不缓存企业统计及列表数据。

图形使用 ECharts 地图、提示框和连续颜色映射组件，渲染器独立分包，只在核验资源可用时加载。保留来源及审图号显示，允许平移缩放；未知统计区划显示未纳入目录，不将其冒充零企业。

## 验证与测试数据

测试数据均在隔离库或测试 mock 中，企业名明确标注“仅测试／不入库”；浏览器测试使用自动创建和清理的 `zqzt_exhibition_test_*` 库及临时 HTTP 服务。生产 seed 不增加厂商。图形测试只使用内存中的合成几何，不登记真实边界或伪造生产审图资料。

项目根目录执行 `scripts/check.ps1` 可运行既有 Go、前端及 Flutter 检查。新增专项命令：

```powershell
cd server
. ../scripts/go-env.ps1
$env:RUN_MYSQL_TESTS='1'
go test -v -run 'Test(IndustryMap|AdministrativeSnapshot)' ./...
go vet ./...
# 先完成 web 构建；需要现有 .tools/browser-qa Playwright 和本机 Edge
$env:RUN_INDUSTRY_MAP_BROWSER='1'
go test -count=1 -v -timeout 5m -run '^TestIndustryMapBrowser$' ./...
```

前端执行 `npm test` 与 `npm run build`；地图资源核验已纳入构建。Windows 如 esbuild 的系统临时目录权限异常，将 TEMP/TMP 指向工作区 `.tools/tmp` 后重试。Flutter 使用既有 `scripts/flutter-env.ps1` 后运行 analyze 与 test。

实际执行结果、文件清单和验证边界见 [交付验收记录](INDUSTRY_MAP_ACCEPTANCE.md)。本次不部署生产，真实行政边界图形验收保留待接入状态。
