# 产业地图交付验收

最新说明：默认页面已按确认方案改为可点击的 ECharts 区划示意图，真实图形交互验证见 [当前验收](FACTORY_MAP_SCHEMATIC.md)。本文保留首轮历史记录。

本文件为首轮行政区划列表版本的历史验收。当前页面已调整为紧凑“厂商地图”，真实边界图形仍未完成验收，详见 [最新状态](FACTORY_MAP_COMPACT.md)。

日期：2026-10-19。网站与后端保持现有版本号，本次数据库升级为Schema16。未部署生产，未向生产数据库插入测试企业。

## 功能与接口

- 新增 `/regions`，导航显示“产业地图”，支持省／市／区县下钻、面包屑、关键词和行业筛选、分页、URL状态、浏览器历史、企业展厅往返。
- PC双栏、移动端上下布局及区域／列表视图切换；已检查360、390、768、1440、1920像素视口、中英文及无横向溢出。
- 新增 `GET /api/v1/public/industry-map/regions`、`GET /api/v1/public/industry-map/vendors`，共享审核公开、展馆隔离与筛选规则。
- 按厂商已发布修订统计，区划不完整单列未细分／待完善地区；禁止按编码长度猜层级、把城市资料塞入任意区县或虚构企业位置。
- ECharts按行政编码着色与点击下钻、懒加载及缓存已接入；当前资源清单为空，真实行政地图图形保持关闭。来源、许可、审图及最终公开形式核验仍待资源提供和相应人工核验。

## 修改文件

| 分组 | 文件 |
|---|---|
| 迁移与修订快照 | `server/migrations/016_industry_map.sql`、`server/administrative_snapshot.go`、`server/model.go`、`server/content.go`、`server/main.go` |
| 查询、保护与兼容 | `server/industry_map.go`、`server/public.go`、`server/public_protection.go`、`server/public_seo.go`、`server/backups.go` |
| 页面与样式 | `web/src/industry-map.tsx`、`industry-map.css`、`industry-map-messages.ts`、`industry-map-types.ts`（均在web/src） |
| 图形与资源 | `web/src/industry-map-chart.tsx`、`industry-map-renderer.ts`、`industry-map-resources.ts`；`web/public/maps/manifest.json` |
| 前端集成 | `web/src/main.tsx`、`api.ts`、`seo.ts`、`showroom.tsx`；`web/package.json`、`web/package-lock.json` |
| 后端测试 | `server/industry_map_test.go`、`industry_map_performance_test.go`、`industry_map_browser_test.go` |
| 前端测试 | `web/src/industry-map.test.tsx`、`industry-map-resources.test.ts`、`industry-map-renderer.test.ts` |
| 工具 | `scripts/verify-map-resources.mjs`、`scripts/verify-industry-map-browser.cjs` |
| 文档与证据 | `README.md`、本文件、`docs/INDUSTRY_MAP.md`、`docs/MAP_RESOURCE_REVIEW.md`、`docs/test-output/industry-map-*`、`docs/screenshots/industry-map-*` |

## 验证记录

正式日志保存在 `docs/test-output/industry-map-*`；`industry-map-command-results.json` 直接记录Go静态检查与构建进程返回的实际退出码，命令没有输出时保留空日志。全部厂商夹具仅存在于隔离数据库或前端mock；浏览器数据库及临时服务在测试结束后清理。

| 检查 | 结果及证据 |
|---|---|
| Go全量测试（启用MySQL集成） | 通过，见 `industry-map-backend.txt`；包含审核隔离、越权、下架、迁移失败重试和6000厂商统计检查 |
| Go静态检查与构建 | `industry-map-go-vet.txt`、`industry-map-go-build.txt` |
| 前端全量回归 | 28个文件、147项通过；`industry-map-web-tests.txt` |
| TypeScript＋Vite生产构建 | 通过；`industry-map-web-build.txt`，含前置地图资源核验 |
| Flutter静态检查 | 无问题；`industry-map-flutter-analyze.txt` |
| Flutter回归 | 138项通过、6项跳过；`industry-map-flutter-tests.txt`，跳过遵循既有测试条件 |
| 真实Edge浏览器 | 通过；`industry-map-browser.txt`，覆盖五种视口、统计／列表交互、历史、展厅返回、图形不可用与接口失败重试 |
| 资源核验 | 空资源清单，行政列表模式；未将任何真实行政边界宣称为已核验可发布 |

6000条隔离夹具下，选择性省级统计的EXPLAIN实际使用 `idx_revision_administrative_public` 及 `idx_content_map_public`，按省索引命中60条修订，后续关联使用覆盖索引；详见后端日志中的 `6000-vendor EXPLAIN`。开发环境耗时不作为生产性能承诺。

前端默认28个worker的一次复跑中，原有入驻测试触发5秒超时；未修改原测试断言或超时，限制为4个worker重跑后147项全部通过。Windows构建临时目录改用工作区`.tools/tmp`，解决系统临时目录访问错误。Vite仍有大分包提示：ECharts渲染器独立约499KB且仅在合规资源可用时请求，首版列表模式不请求它；现有主包提示不阻断构建。

截图复查发现并修正移动端面包屑被旧全局nav规则绝对定位的问题，并在浏览器脚本加入定位断言。最终截图：

- [PC](screenshots/industry-map-pc.png)
- [移动端360](screenshots/industry-map-mobile-360.png)
- [移动端390](screenshots/industry-map-mobile-390.png)

完整使用方式、参数口径、迁移及回退边界见 [使用说明](INDUSTRY_MAP.md)。
