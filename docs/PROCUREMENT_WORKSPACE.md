# 采购中心与厂商工作台布局升级

2026-10-23：采购中心只承担采购方业务；厂商询价、采购邀请、逐项报价、报价历史、独立洽谈和商机订阅迁回厂商工作台。复用原账号和采购项目，不复制业务记录，不新增数据库迁移。

## 菜单和地址

采购中心保留五项菜单：我的询价与需求 `/buyer`、发布采购需求 `/buyer/new`、采购需求大厅 `/procurement`、消息通知 `/buyer/notifications`、采购商资料 `/buyer/profile`。网站顶部仍只有一个采购中心入口，匿名进入大厅，登录后进入我的需求。

厂商工作台在产品管理之后、展厅推广之前提供“商机中心”，显示厂商侧未读数：

| 栏目 | 地址 |
| --- | --- |
| 询价与报价 | `/merchant/opportunities` |
| 项目详情、报价历史及独立洽谈 | `/merchant/opportunities/projects/{id}` |
| 消息通知 | `/merchant/opportunities/notifications` |
| 商机订阅 | `/merchant/opportunities/settings` |

旧 `/buyer/opportunities/*` 跳转至对应厂商地址，查询参数和锚点保留；删除相反方向跳转。需求大厅参与、通知、项目详情、需求历史及返回链接均使用新地址。详情页仍选中商机中心及询价与报价栏目。

未登录访问商机时，在厂商工作台使用既有安全业务登录组件，支持动态验证码、手机号密码及公司账号，登录后继续原操作；不会自动创建厂商。普通采购账号显示同账号入驻提示，保留原采购资料。演示账号切换在两个工作台使用正常退出和重新登录。

## 视觉与交互

保留深蓝侧栏、白色顶栏、浅灰背景和白色卡片。共享样式限定在工作台内，官网及管理后台不受影响。

- 侧栏展开宽度 236 像素、收起 76 像素，菜单最小高度 48 像素；两套折叠状态独立保存，提供清晰的选中、悬停和键盘焦点。
- 两套侧栏的 Logo 与“中企展馆”整块链接统一在当前标签页返回官网首页 `/`，保留账号会话和语言；顶栏“返回网站”继续保留，业务首页通过各自菜单访问。
- 顶栏桌面 72 像素、手机最小 64 像素；栏目、面包屑、返回网站、语言切换统一，长标题截断。
- 统一两套工作台的标题、卡片、筛选、表单、列表和空状态，厂商首页、展厅管理、产品管理、推广和设置沿用原业务操作。
- 大厅筛选在桌面首行显示关键词、展馆、分类和状态；次行显示省市区及查询、重置。中等宽度两列，手机单列；输入和查询按钮高 44 像素，不随网格拉伸。
- 重置清空关键词、展馆、分类、地区和页码，恢复进行中；筛选继续沿用原 URL 行为。
- 商机标签使用独立样式，消除公共导航伪元素造成的文字横线。桌面比较表、手机按产品切换报价卡片保留。
- 900 像素及以下使用抽屉，支持遮罩、Escape、Tab/Shift+Tab 焦点循环、关闭后焦点恢复和栏目切换自动关闭。

## 消息接口及兼容

`GET /api/v1/procurement/notifications` 新增可选 `scope=buyer|supplier`；`POST /api/v1/procurement/notifications/read` 请求体新增可选 `scope`。不传范围仍按原逻辑处理全部消息，兼容旧客户端及原生 App。

列表总数、未读数和已读操作使用同一范围；项目归当前账号采购的消息属于采购侧，其余厂商消息及无项目商机汇总属于厂商侧。所有查询仍限定当前账号和已到达的通知。不会因采购侧全部已读而读掉厂商消息，也不能借消息 ID 操作他人通知。

## 验收和本机更新

前端全量 215 项测试、TypeScript、Vite 构建、Go 全量回归通过。真实 Edge 在隔离 MySQL 库完成询价、私有附件、定向报价、扩散审核、第二家报价、比价、意向电话、独立洽谈及关闭。360/390/768/1440 像素与中英文下，两套工作台及业务页面共 98 组布局检查通过，无页面脚本错误。

- [前端完整回归](test-output/procurement-return-web-full.txt)
- [Go 完整回归](test-output/procurement-return-go-full.txt)
- [生产构建](test-output/procurement-return-build.txt)
- [隔离浏览器报告](test-output/procurement-return-browser.json)
- [演示只读浏览器报告](test-output/procurement-return-demo-browser.json)

截图：[采购需求大厅](screenshots/procurement-return/public-hall-1440-zh.png)、[厂商工作台](screenshots/procurement-return/merchant-home-1440-zh.png)、[手机厂商报价](screenshots/procurement-return-demo/supplier-quote-390-zh.png)。响应式检查使用桌面浏览器，不代表实体手机测试。

本机主站 `http://localhost:8080` 与隔离演示 `http://localhost:8081` 更新匹配的后端及完整网页构建。没有生产部署、真实短信、原生 App 更新或演示重置。启动和账号说明见 [PROCUREMENT_DEMO.md](PROCUREMENT_DEMO.md)。

部署时同步后端及网页文件；本次无需额外迁移。回退应恢复上一版匹配的后端与网页，不删除采购记录和附件。生产短信继续使用原阿里云配置，不启用开发短信或演示开关。
