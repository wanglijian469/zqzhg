# 中企展馆 V2.0 P0 / P1 补齐交付记录

交付日期：2026-10-18。项目继续使用现有 React / TypeScript / Go / Gin / GORM / MySQL 架构。本轮没有改版 Android，没有部署生产服务器，没有新增业务表或大型依赖。

## P0 实现与验收

| 项目 | 实际实现 | 主要文件 | 验证 |
| --- | --- | --- | --- |
| 展馆设置 | 现有后台“首页展示 → 展馆设置”编辑双语名称、描述、开放/筹建状态及主视觉；固定 ID / slug；元信息与图片事务保存，共用首页配置版本，冲突需刷新 | web/src/hall-settings.tsx、home-display.tsx；server/hall_settings.go、permissions.go | 登录/权限、错误图片、并发保存、旧首页版本冲突、独立展馆日志类型/动作筛选与双语、保存后上下文刷新、隔离后台实际上传/保存通过 |
| 筹建厅隔离 | 筹建页显示本厅真实配置图片；不请求厂家/产品列表；只有配置主视觉可公开，未开放业务详情及媒体仍受原权限约束 | web/src/halls.tsx；server/halls.go、media.go、public.go | 开放/筹建切换、跨厅创建/分类拒绝、详情隐藏、图片公开/移除及旧客户端默认农业机械厅通过 |
| 联合筛选 | 厂家/产品按展馆、关键词、父子分类/别名和企业地区组合查询；厂商可匹配其公开产品名称/型号；名称/参数型号查询复用公开审核版本 | server/public.go；web/src/browse-page.tsx、showroom.tsx | 联合条件准确总数，分页无重复、无效条件空结果、隐藏/非公开数据排除通过 |
| 分页与排序 | 网页厂家固定20家、产品24款；公开API分别上限20/24，兼容旧客户端更小 page_size；白名单排序和稳定 ID 次序；筛选回第一页，URL/浏览器返回保留；从首页进入产品详情也按内容所属展馆返回目录 | server/public.go；web/src/browse-page.tsx | 2000家/24000款及真实5家/38款数据验证通过 |

P0 本轮功能没有未实现项。涉及运行环境及真机的未验证范围见下文，未计为通过。

## P1 实现与验收

| 项目 | 实际实现 | 主要文件 | 验证 |
| --- | --- | --- | --- |
| 厂家推荐 | 最多8家，精选状态与人工排序优先，同优先级分散主营分类和地区，分类/地区分别预留有界候选，防止热门分类的多地区记录挤掉少数分类；保留后台最多12家的既有精选设置 | server/public_recommendations.go；web/src/exhibition-home.tsx | 推荐排序、公开版本、稳定性及真实数据展示通过 |
| 产品推荐 | 最多12款，每厂商严格最多3款；按厂商轮次选择，结合精选、人工排序、分类覆盖及公开时间/ID；有其他候选时分散同厂商，不足不凑数 | server/public_recommendations.go | 单厂商仅3款、分类/厂商多样性、撤销推荐、下架、隐藏厂商、稳定排序通过 |
| 查询与读取性能 | 350ms防抖，Enter/按钮立即提交；输入法组词期间暂停；自动查询 replace，显式提交/筛选/分页保留导航；取消计时器/旧请求；代表产品批量有界查询，公开修订与分类别名批量解析 | web/src/search-debounce.ts、browse-page.tsx、showroom.tsx；server/public.go、public_recommendations.go、category_suggestions.go | 防抖/输入法/快速条件变更/过期响应/历史返回通过；1家与20家列表均9条实际SQL查询 |
| 图片与加载 | 单张后台首屏图优先加载；公开图片直接使用受权限约束的URL，其他图片懒加载、固定尺寸、异常回退；首页与企业产品中心补齐骨架屏和失败重试 | web/src/main.tsx、exhibition-home.tsx、browse-page.tsx、home-display.tsx、showroom.tsx | 破图、网络失败重试、3种宽度骨架屏、视频延迟挂载及画册操作通过 |
| 中英文 | 新增后台、状态、推荐规则、防抖、错误、空状态和无障碍文案进入双语字典；企业资料沿用真实中文回退 | web/src/v2-messages.ts、halls.tsx、hall-settings.tsx | 112项前端测试和中英文浏览器矩阵通过 |
| SEO | Go在初始HTML输出首页/查询/产品独立元信息，继续复用企业展厅输出；客户端同步Title/Description/Canonical/OG，清理旧元信息；型号/厂商/展馆参与产品描述 | server/public_seo.go、showroom.go；web/src/seo.ts、halls.tsx、main.tsx、showroom.tsx | 无脚本HTML、详情独立信息、别名统一、查询排序、分页、筹建/筛选noindex、404/私有内容不泄漏、图片Alt通过 |
| 响应式 | 保留公开页及展厅移动布局；新增设置表单列宽及专属后台页头换行修复，不使用页面隐藏溢出掩盖裁切 | web/src/mobile-web.css、admin.tsx | 352组公开页面 + 32组后台布局通过，包含双语与100%/125%等效视口 |
| 索引与恢复 | 012增加按厅/厂商/公开排序组合索引，保留011既有索引；迁移重试检查已有索引；备份格式接受Schema12，验证配置/图片/版本/索引恢复 | server/migrations/012_public_query_indexes.sql、main.go、backups.go | 中断重试、内容不丢失、自定义展馆与主视觉完整备份/恢复/切换/回滚通过 |

P1 本轮功能没有未实现项。运行环境验证范围没有扩大为生产或真机验收。

## 测试结果与限制

- 前端：npm test，20个文件、112项全部通过；npm run build 包含 TypeScript 检查并通过。项目没有独立 lint 脚本，本轮没有新增工具链。
- 后端：隔离 MySQL **5.7.18**。非视频全套65项业务测试通过，视频处理单测另1项通过，隔离后台浏览器夹具另1项通过，共67项不同顶层测试通过；go vet通过。嵌套用例不重复计数。
- 普通 go test -v ./... 和结构化日志运行在本机曾于既有视频处理测试中提前结束，退出码为0但遗漏后续4个展厅测试，因此未把该输出认定为完整通过。最终使用非视频全集与视频单测分别执行，后续展厅回归均实际通过。长进程提前结束原因尚未定位。
- 默认跳过的手动夹具：TestOperationsBrowserFixture、TestResourceBrowserFixture、TestCleanupResourceBrowserFixture；它们用于生成测试环境，本轮使用新建且实际运行成功的 TestV2BrowserHarness 和浏览器路由隔离PDF样本。
- TestRealOSSPrivateStorage 缺少真实隔离OSS测试凭据而跳过。实际OSS环境未验收。MySQL8没有独立运行实例，本轮SQL在5.7.18实际验证，不使用CTE或窗口函数。
- 规模测试仅写入可删除的独立测试库：2000家厂家、24000款产品。地区+分类+型号组合返回1000家/6000款的准确总数，连续分页无重复；有界返回，执行计划已记录。代表产品索引查询实际扫描约每家12条候选。
- 浏览器：Edge无头模式；宽度1440、1366、1280、768、414、390、375、360px，中英文，100%与125%等效视口。352组公开布局、32组后台布局、23组交互/SEO/后台检查，以及375/768/1440px的PDF验证通过。正常页面控制台无新增错误。
- 125%为缩小CSS视口的等效检查；不是人工操作浏览器缩放按钮。未使用iOS/Android真机，系统拨号、真实软键盘和浏览器安全区仍未验证。Linux后端为交叉编译产物，未在Linux主机实际运行。
- 自动浏览拦截本次浏览器上下文中的统计POST，保持生产统计逻辑原样；失败/破图/PDF样本均局限于隔离浏览器路由；所有后台配置写入使用自动删除的测试库。
- 本机公开数据升级前后均为5家企业、38款产品，厂商ID一致。实际首页12款分布为：厂家1/21/39各3款、27两款、37一款。没有新增业务数据。
- 前端构建保留既有大分包体积提示；PDF阅读器按需加载。本轮没有增加大型依赖。
- 内容核对：河北丹顿液压科技有限公司的资源2（媒体122）有疑似误传的企业宣传视频，需运营核对。代码只按真实资源归属展示，未擅自删除或重新归属上传资料。

## 验证命令

在项目根目录执行（PowerShell）：

~~~powershell
. ./scripts/go-env.ps1
cd web
npm test
npm run build
cd ../server
$env:RUN_MYSQL_TESTS='1'
go test -v ./...
# 本机完整验收采用以下两个命令，分别检查全部业务测试实际结束：
go test -v -count=1 -skip '^TestVideoWorkerProcessingRetryAndRecovery$' ./...
go test -v -count=1 -run '^TestVideoWorkerProcessingRetryAndRecovery$' .
go vet ./...
~~~

浏览器：node scripts/verify-p0p1.cjs、verify-p0p1-interactions.cjs、verify-p0p1-search-seo.cjs、verify-p0p1-pdf.cjs。后台脚本 verify-p0p1-admin.cjs 需先显式运行隔离的 TestV2BrowserHarness；测试令牌只写入.local临时文件，不纳入交付。

## API与数据库兼容

- 新增 GET /api/v1/admin/halls、PUT /api/v1/admin/halls/:id，使用现有 home.manage 权限与操作日志。
- PUT接收 name、name_en、description、description_en、status、hero_media_id、version；slug若传入必须与原标识相同。返回更新后的版本；409 stale_revision须重新读取。
- GET /api/v1/public/halls兼容增加 hero_media_id、version、updated_at。旧客户端可忽略新增字段；旧接口缺少hall时仍为农业机械厅。
- 012仅新增组合索引，没有新增业务表、删除字段或重写企业/产品数据；hall属性沿用既有011迁移。不要手动重复执行原始CREATE INDEX语句；应用迁移器检查索引存在并维护版本记录。
- 包内OpenAPI说明同步更新。Android包名、签名、版本1.7.0+29及下载APK字节均未改动。

## 本地启动与部署

本地已更新并运行：http://localhost:8080；后台：http://localhost:8080/admin/featured?tab=settings。

~~~powershell
cd web
npm run build
cd ..
./scripts/stop.ps1
./scripts/start.ps1
~~~

生产未部署。部署时按以下顺序执行：

1. 备份数据库、媒体、server/.env、运维控制目录、旧程序及完整静态目录。媒体、签名、环境和运营数据不使用交付包覆盖。本机备份在.local/backups/pre-p0p1-db-media和pre-p0p1-completion，含私有数据，不分发。
2. 在隔离环境先验证新程序启动自动执行012，以及既有011/旧备份升级；确认Schema12、按厅数据、健康接口、查询、登录、入驻及后台配置。
3. 配套替换后端与完整dist目录，停止旧进程后启动；不要混用新前端和缺少新API的后端。Windows程序使用bin/exhibition-windows-amd64.exe替换server/exhibition.exe；Linux使用bin/exhibition-linux-amd64并chmod +x，保留已有进程管理及FFmpeg/FFprobe、MySQL工具配置。
4. PUBLIC_BASE_URL使用实际公开域名（生产通常为https://zqzhg.com），不要照搬本地172.27.35.2地址。初始HTML与客户端Canonical都从这一配置生成。
5. 检查/api/v1/health、/public/halls、/public/home、联合查询、两厅切换、公开详情、媒体权限、登录/入驻及首页展示后台。执行包内SHA256清单校验。
6. 回滚同时恢复支持展馆隔离的上一套程序与静态资源；012索引可以保留。其他展馆已有业务数据时，不退回缺少展馆隔离的更早后端。若恢复数据库，暂停写入，使用同一时间点的数据库/媒体/配置备份；Schema12备份使用本轮兼容恢复工具，避免旧备份校验器拒绝。

## 截图与完整文件清单

- 桌面首页：screenshots/p0p1/home-1440.png
- 手机首页：screenshots/p0p1/home-375.png
- 企业产品中心：screenshots/p0p1/showroom-products-375.png
- 平板设置：screenshots/p0p1/isolated-admin-halls-768.png（隔离配置测试，借用现有主视觉验证上传，不代表电缆厅正式配图）
- 全部截图、结构化报告、命令原始输出和校验值随交付包提供。

本轮修改文件（相对于本次开始前的备份，不把此前Android或网站升级文件再次计入本轮改动）：

- docs/V2_P0_P1_ACCEPTANCE.md
- docs/openapi.json
- scripts/package-p0p1.mjs
- scripts/verify-p0p1-admin.cjs
- scripts/verify-p0p1-interactions.cjs
- scripts/verify-p0p1-pdf.cjs
- scripts/verify-p0p1-search-seo.cjs
- scripts/verify-p0p1.cjs
- server/backups.go
- server/category_suggestions.go
- server/hall_settings.go
- server/halls.go
- server/halls_v2_test.go
- server/main.go
- server/media.go
- server/migrations/012_public_query_indexes.sql
- server/operations_test.go
- server/p0p1_browser_test.go
- server/p0p1_test.go
- server/permissions.go
- server/public.go
- server/public_recommendations.go
- server/public_seo.go
- server/showroom.go
- web/src/admin.tsx
- web/src/app.test.tsx
- web/src/browse-page.tsx
- web/src/exhibition-home.tsx
- web/src/hall-settings.tsx
- web/src/halls.tsx
- web/src/home-display.test.tsx
- web/src/home-display.tsx
- web/src/main.tsx
- web/src/mobile-web.css
- web/src/mobile-web.test.tsx
- web/src/p0p1.test.tsx
- web/src/search-debounce.ts
- web/src/seo.ts
- web/src/showroom.tsx
- web/src/types.ts
- web/src/v2-messages.ts
- README.md
