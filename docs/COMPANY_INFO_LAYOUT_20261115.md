# 产品导航与企业信息布局优化（2026-11-15）

已完成本地实现和验收，未部署生产。数据库、业务接口和录入规则未改动。此次未打部署包，地图资源不属于本轮变更。

产品内容上方删除独立返回导航；面包屑统一为“公司名称 / 产品名称”，服务端首次HTML和BreadcrumbList同步为两级。公司名称可以进入企业展厅，并保留展馆上下文；从该企业展厅产品列表进入时，还会保留原来的筛选、分页和定位。

企业介绍中，经营信息位于基本资料之后、生产能力之前。主营和兼营突出末级名称，上级路径使用辅助文字，兼营逐项排列。经营性质和产业带保留。分类摘要自行加载样式，公开页首次访问不再依赖管理页的样式；面板外观限定在企业展厅。

生产能力使用统一面板。大于900px按有值项等宽横排，900px及以下分行；手机标签列96px，内边距16px。空字段和无数据分区隐藏，不补写面积单位，中英文保持现有回退。

## 对比截图

截图使用隔离公开页面夹具，修改后为本地构建产物；没有读取或写入生产数据库。真实后端的有脚本、无脚本检查另使用本机隔离测试数据库。

| 页面 | 调整前后对比 |
| --- | --- |
| 手机经营信息与生产能力（390px） | [对比图](screenshots/company-info-20261115/mobile-comparison.png) |
| PC经营信息与生产能力（1440px） | [对比图](screenshots/company-info-20261115/desktop-comparison.png) |
| 手机产品导航（390px） | [调整前](screenshots/company-info-20261115/before/product-390-zh.png) · [调整后](screenshots/company-info-20261115/after/product-390-zh.png) |
| PC产品导航（1440px） | [调整前](screenshots/company-info-20261115/before/product-1440-zh.png) · [调整后](screenshots/company-info-20261115/after/product-1440-zh.png) |
| 长中文公司名与产品名（360px） | [完整换行检查](screenshots/company-info-20261115/after/product-long-360-zh.png) |

相同夹具下，390px的生产能力区域高度由约577px减少至204px；360px由577px减少至226px。内容保持完整。

## 验收结果

- 相关前端6个测试文件，共42项通过。其中一项既有管理页懒加载超时，单文件复跑后通过。
- 追加检查4个测试文件，14项通过、4项失败。失败全部来自既有的分享弹窗旧按钮断言；已在独立副本恢复本轮修改前组件后，复现相同4项失败。未修改分享功能或将这些失败计为通过。比较日志：`.local/company-info-baseline-footer.log`。
- TypeScript检查、资源预检查、规则同步、PDF资源准备和Vite构建通过。环境没有npm，按package.json顺序通过Node执行对应命令；构建临时目录指向工作区，避开系统临时目录权限问题。仍有Vite大分块提示。
- 后端3项相关SEO测试和1项真实后端浏览器验收通过，全部使用自动创建和清理的隔离数据库。覆盖首次HTML、结构化数据、无JavaScript访问及正常客户端导航。
- 构建产物浏览器检查覆盖1440、1024、768、390、360px及中英文，共23个页面/场景。检查无横向溢出、分类逐项显示、长中文名称换行、1/2/3个生产字段，以及滚动到末尾时页尾不被联系栏遮挡。
- 已人工查看PC、手机中文和手机英文截图。使用桌面浏览器模拟手机宽度，未做实体手机验收。

结构化记录：[布局验收](test-output/company-info-layout-20261115.json)、[真实后端SEO浏览器验收](test-output/company-info-seo-browser-20261115.json)。

## 复跑方式

1. 在本机启动Vite开发服务5197端口，或构建后启动preview 5198端口。
2. 使用`node scripts/verify-company-info-layout.cjs after`。测试构建产物时设置环境变量`LAYOUT_BASE=http://127.0.0.1:5198`。脚本只拦截本地浏览器中的公开API，输出到本轮截图目录。
3. 后端沿用`TestIndependentSEOBrowser`：启用`RUN_MYSQL_TESTS=1`和`RUN_SEO_BROWSER=1`，并设置`SEO_SCREENSHOTS`与`SEO_REPORT`为独立输出位置。
