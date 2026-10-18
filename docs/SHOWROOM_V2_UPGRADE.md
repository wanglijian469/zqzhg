# 企业独立展厅 V2.0 交付说明

交付版本：2.1.0（企业展厅 V2.0）；日期：2026-10-17。已更新本地 http://localhost:8080，未部署生产环境。

## 实际调整

- 六栏目统一为展厅首页、企业介绍、产品中心、视频展示、电子画册、联系企业；保持旧 tab 标识和展厅别名链接。
- 首页为400px桌面品牌首屏，子栏目使用紧凑品牌栏；单套企业导航在900px以下折叠，支持触控与键盘。
- 首页按主营产品、简介、实拍、视频、画册、联系方式展示，缺少可选数据不填造；资质与生产能力完整保留在企业介绍。
- 首页最多8款本企业公开产品，手工指定3款优先，再按原精选/人工顺序/稳定ID补足；手动清空保持隐藏。产品中心只渲染一份产品列表，24款服务端分页，搜索、分类、排序和返回状态保留URL。
- 产品图片4:3完整展示，桌面4列、平板3列、手机2列；工厂照片按原比例展示，支持放大、键盘左右切换和退出。
- 视频卡片16:9，点击后挂载播放器且preload=none；画册封面不再越界，PDF与图片阅读器支持缩放、翻页、退出，真实且有权访问的PDF提供下载。
- 联系信息来自当前厂商公开审核版本，电话支持拨打及复制失败后的手动复制；移动联系栏预留安全区，阅读弹窗打开时隐藏。
- 首页主图优先加载，其余图片懒加载；取消非产品栏目中无用的产品列表请求，切换查询取消旧请求。

## 故障诊断

原视频封面容器200px，图片实际308px；画册竖图实际731px，造成真实内容越界。通过明确图片定位与16:9边界修复。单资源卡片原有特例会占满整行，现保持正常网格尺寸。产品重复来自代表产品与完整列表同时渲染；子栏目重复首屏与导航来自组件结构。长截图中固定联系栏可被截在画面中部，移动截图现使用实际视口，布局检查另验证页面可滚动及底部留白。

## 数据及API兼容

本轮无数据库字段、迁移或破坏性操作。公开数据基线和验收后均为5家企业、38款产品；企业名称、产品数量、原代表产品顺序与资源数量一致。

GET /api/v1/public/showrooms/:slug 新增 home_products 数组，上限8条；representative_products仍最多3条，原后台配置与Android契约不变。新前端对旧后端保留代表产品字段回退，完整8款效果需要配套后端。docs/openapi.json已记录扩展。

资源读取继续使用企业归属、公开审核状态和关联产品权限校验；不新增邮箱、地图或不可提交的询盘功能。

## 主要修改文件

前台结构与组件：
- web/src/showroom.tsx、web/src/showroom-v2.css
- web/src/main.tsx（可选请求启用、图片优先级）、web/src/vendor-brand.tsx、web/src/types.ts
- web/src/company-portal.tsx、web/src/resources.tsx
- web/src/showroom-messages.ts、web/src/resource-messages.ts、web/src/merchant-messages.ts

接口与测试：
- server/showroom.go、server/showroom_display.go、server/showroom_display_test.go
- web/src/showroom-v2.test.tsx，以及现有showroom/company-portal/app/vendor-list/merchant-workspace测试
- docs/openapi.json
- scripts/verify-showroom-v2.cjs、scripts/verify-showroom-media.cjs

## 验证结果

- npm test：18个文件，96项通过。
- npm run build：TypeScript及Vite生产构建通过；现有大分块提示保留，没有新增大型依赖。
- RUN_MYSQL_TESTS=1 go test -v ./...：65项通过，4项按既有显式开关跳过；测试使用独立数据库。go vet ./...通过。
- 实际Edge：1440、1366、768、390、375px × 中英文 × 100%及等效125%视口 × 六栏目，共120组通过，无横向溢出、资源图片越界或运行异常；同时验证触控菜单、企业产品互链及返回；另验证接口失败重试、空结果清除保留展馆、键盘切栏、破图回退及联系栏不遮挡页脚。
- 独立多媒体测试库：PDF渲染、缩放、下载，图片画册翻页，视频主动播放与关闭，1440/390px阅读弹窗通过；测试资源未写入业务库。
- 原登录、工作台、后台审核、入驻与权限流程由前后端现有测试回归；本轮未在生产环境手工登录或部署。125%使用等效视口检查，未操作系统浏览器缩放设置。

截图：docs/screenshots/showroom-v2；结果：docs/test-output/showroom-v2-*.json及日志文本。

## 待运营核对

河北丹顿液压科技有限公司“宣传视频”（资源ID 2、文件媒体ID 122）归属字段正确，但画面为人物击鼓，疑似误传。依照约定保留上传数据，未擅自删除或下架；需企业或后台运营核对并决定是否下架、替换。该问题不属于跨企业接口泄露。

本机业务库没有公开PDF画册，PDF验证使用独立测试样本；生产Linux运行环境和真实手机设备尚未实测。

## 本地启动

PowerShell，在项目根目录执行：

~~~powershell
cd web
npm ci
npm test
npm run build
cd ..
./scripts/stop.ps1
./scripts/start.ps1
~~~

启动脚本会构建Go后端。若刚停止服务，需等待旧进程完全退出再覆盖文件；确认8080监听进程的路径属于本项目。

健康检查：GET http://localhost:8080/api/v1/health。

## 部署与回滚

交付目录delivery/v2.1.0，包含Windows/Linux amd64后端、完整web/dist、环境配置示例、说明和SHA256校验清单。已有delivery/v2.0.0保持不变。

1. 升级前备份数据库、媒体、当前程序和完整静态目录；保留原.env、媒体、签名及运营目录。
2. 本轮无新增迁移；若服务器仍早于平台V2，先按平台V2说明完成已有展馆兼容迁移。
3. 停止既有服务，确认进程退出；配套替换后端与完整web/dist，Linux程序保留执行权限，启动既有服务。不要把.env.example覆盖到真实.env。
4. 检查/api/v1/health、企业六栏目、搜索分页、登录、视频与画册；必要时清理浏览器静态缓存。
5. 回滚时停止服务，恢复上一版配套程序与静态目录，环境和媒体保留；本轮不需回滚数据库。只回滚到已支持展馆隔离的兼容版本，不能直接退到缺少展馆隔离的旧版本。

本地回滚备份：.local/backups/pre-showroom-v2-20261017。交付包不包含真实.env、数据库、媒体、令牌或测试库凭据。
