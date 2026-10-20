# 厂商地图：ECharts 区划示意图

后续更新：本机开发环境可显式启用 [真实轮廓学习预览](FACTORY_MAP_OUTLINE_PREVIEW.md)。本文描述的方块示意图保留为生产默认及手动降级模式。

日期：2026-10-19。本轮按确认方案以行政区方块替代真实边界展示。入口仍为 `/regions`，界面始终标注“行政区划示意图，非真实地理边界”。

## 使用与统计

- 全国显示2023年大陆31省级目录节点；宽屏按抽象的大致方位排列，手机按行政编码排列。下级地区始终按编码网格排列。方块不表示疆域、面积、经纬度或企业实际位置。
- 方块显示地区名称和真实厂商数，浅色至深蓝表示数量；零值仍显示。省级方块使用简称，下级长名称按方块宽度省略，避免手机标签重叠；悬浮可查看完整地区名称和数量，点击进入下一级。
- 点击地区、面包屑、返回全国或上一级均同步厂商结果及URL；关键词、行业、分页及手机视图沿用现有状态。展厅返回恢复地区和筛选。
- 最下级节点显示当前地区总数，不制造下级；省直辖县级市和直辖市按接口真实关系浏览。“未细分地区／待完善地区”保持独立筛选，不创建虚拟行政节点。
- 图形支持拖动、滚轮／触屏缩放、放大缩小按钮及复位。切换地区或实际改变容器尺寸时重新适配，普通尺寸通知不会重置缩放。
- 保留固定视口工作区、PC厂商栏及按可用高度计算的1—8家分页；手机切换地图／厂商。图形分包或渲染失败时可重试，厂商结果保持可用。

## 实现及兼容

默认使用按需加载的 ECharts Graph 与 SVGRenderer，不请求 `/maps/manifest.json` 或任何真实边界文件。原有边界加载、核验及地图渲染适配代码保留，未开放未经核验的真实边界入口。

主要修改文件：

- `web/src/industry-map-schematic.ts`：抽象布局、等尺寸方块、数量颜色、Graph事件与视口控制。
- `web/src/industry-map-chart.tsx`、`industry-map.css`、`industry-map-messages.ts`：默认图形集成、销毁重建、重试、响应式工作区及中英文说明。
- `web/src/industry-map-schematic.test.ts`、`industry-map-chart.test.tsx`、`industry-map.test.tsx`：布局、数量、导航、生命周期及页面回归。
- `scripts/verify-industry-map-browser.cjs`：真实SVG图形点击、视口和异常恢复验收。

接口保持：`GET /api/v1/public/industry-map/regions`、`GET /api/v1/public/industry-map/vendors`。继续使用审核通过的已发布版本、当前展馆及公开状态；不修改统计接口或查询条件，不新增数据库迁移、经纬度、Redis或外部地图服务。

此前的 [紧凑工作区记录](FACTORY_MAP_COMPACT.md) 和 [真实边界核验记录](MAP_RESOURCE_REVIEW.md) 为历史过程及未来真实边界接入依据；其中“默认地图不可见”的结论由本次示意图交付替代。真实地理边界图形仍未接入，也未宣称已完成此类地图的发布核验。

## 验证

原始输出位于 `docs/test-output/schematic-*`，截图位于 `docs/screenshots/factory-map-schematic-*`。

| 命令 | 实际结果 |
|---|---|
| `cd web; npm run build` | 通过；Graph分包按需加载；保留现有主包大于500kB的Vite提示 |
| `cd web; node node_modules/vitest/vitest.mjs run --maxWorkers=4` | 30个文件、159项测试通过 |
| `cd server; go test -count=1 ./...`，设置 `RUN_MYSQL_TESTS=1` | 通过，168.617秒；使用隔离MySQL测试数据库 |
| `cd server; go vet ./...` | 退出码0，无诊断输出 |
| `cd server; go test -count=1 -run '^TestIndustryMapBrowser$' -v .`，设置两个测试开关 | 通过，21.256秒；真实Edge与隔离测试服务 |

Go命令先加载 `scripts/go-env.ps1`；浏览器测试额外设置 `RUN_INDUSTRY_MAP_BROWSER=1`，需要本地Edge及仓库浏览器验收依赖。

浏览器实际点击图形完成全国→河北→邢台→宁晋；图形悬浮值与厂商总数一致，市级未细分记录独立筛选。另验收北京→北京市→海淀区、湖北→仙桃市、零厂商、叶子区域、关键词、分页、历史、展厅返回、缩放／拖动／复位、手机切换和中英文。

1440×900、1366×768、390×844、360×640及844×390横屏均检查整页无横向／纵向溢出、厂商卡片不越界、图形标签不重叠。阻断所有 `/maps/` 请求仍正常显示，断言没有边界请求；统计接口与图形组件／Graph分包加载失败均验证恢复。

当前验收对象是已确认的区划示意图；没有把它记作真实行政边界地图验收。

所有浏览器测试企业均由隔离MySQL测试数据库创建并随测试清理。本地业务库不插入示例企业，不部署生产。

## 本地查看

本地开发服务已重新构建并启动：`http://localhost:8080/regions`。只读浏览器验收确认页面HTTP200、31个省级方块、现有公开厂商9家（河北8家），并在实际业务数据上点击河北→邢台→宁晋成功；边界资源请求0次。服务进程19064，未部署生产。

当前业务数据效果：![厂商地图本地效果](screenshots/factory-map-schematic-local.png)。其他 `factory-map-schematic-*` 验收截图来自隔离测试数据，图中28家不属于业务库。
