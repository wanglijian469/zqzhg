# 我要入驻／厂商登录入口优化

交付日期：2026-10-19。本轮仅调整 `/join`、`/merchant/login` 和共享认证组件；已更新本地服务，未部署生产。

## 使用方式

- [我要入驻](http://localhost:8080/join)：创建公司账号，注册成功后进入工作台完善企业资料。注册本身不创建空企业、不保存企业资料或自动提交审核。
- [厂商登录](http://localhost:8080/merchant/login)：默认公司账号和密码，次级切换为短信验证码。原有公司账号继续使用；忘记密码联系平台管理员，通过现有后台重置。
- 企业资料仍按“完善资料 → 保存草稿／提交审核 → 审核通过公开”操作，资料页、产品页和工作台沿用原布局。
- 短信模式仅登录该手机号对应的账号，不接管公司账号展厅。当前浏览器验收使用隔离库中的开发短信提供器，未配置或验收真实短信服务。

## 界面和认证行为

电脑采用不超过 1120px 的双栏入口，左侧为步骤或管理功能说明，右侧为约 420px 表单卡片。960px 以下为单列，缩短介绍、优先显示表单。仅这两个页面启用紧凑页脚。常见电脑尺寸完整显示表单；手机、横屏、错误提示和展开帮助允许自然整页滚动，不使用内部滚动容器或裁切内容。

注册页专注注册，通过底部链接进入公司登录。登录页使用账号密码／短信二选一，底部链接进入新厂商注册。没有增加展示数量、承诺或认证宣传。

- 注册密码、确认密码、登录密码分别提供显隐按钮；保留 `username`、`new-password`、`current-password`、`tel`、`one-time-code` 自动填充语义。切换认证方式清空密码并恢复隐藏。
- 公司账号仍最多 100 个 Unicode 字符；注册密码仍为 12–72 个 UTF-8 字节、不能全为空白，保留空格。中文、emoji 的计量规则收进可展开帮助。
- 前端和后端字段错误均聚焦首个错误项。异步错误在请求结束、字段恢复可用后聚焦；更正输入不反复抢焦点。表单支持 Enter、加载反馈、禁用提交和请求锁。
- 网络失败保留当前输入。注册结果不明确时只提示登录继续，不自动重试注册。
- 两页底部链接保留展馆和合法 `next`；默认农业机械厅沿用现有规范省略 `hall=agriculture`。账号仅经路由状态传递，密码只保留在当前组件中，不进入 URL、草稿、localStorage 或 sessionStorage。
- `next` 仅接受规范化后仍属于 `/merchant` 的站内路径；拒绝外站、协议相对地址、目录越界、反斜杠／控制字符和登录页循环。
- 已登录访问、旧 `?tab=` 入口、访客草稿恢复和会话失效重登沿用原行为。共享 `MerchantAuth` 默认仍为嵌入模式，保留注册／公司登录／短信三种切换，入口页显式选择展示模式。

## 修改文件

| 文件 | 内容 |
|---|---|
| `web/src/merchant-entry.tsx` | 统一入口、介绍、交叉导航、紧凑页脚、安全返回地址解析 |
| `web/src/merchant-entry.css` | 双栏／单列布局、表单、触屏按钮与页脚样式 |
| `web/src/merchant-auth.tsx` | 入口／嵌入模式、显隐、字段错误聚焦、账号路由状态 |
| `web/src/merchant-phone-login.tsx` | 提取现有短信认证，补充语义表单、错误聚焦及防重入 |
| `web/src/main.tsx` | 接入两页，保留旧入口、注册后跳转和工作台分支 |
| `web/src/merchant-entry.test.tsx` | 新增 13 项入口、认证、路由与失败测试 |
| `web/src/merchant-registration.test.tsx` | 更新公司登录入口的链接语义断言，保留草稿／审核断言 |
| `server/merchant_entry_browser_test.go` | 隔离 MySQL + HTTP 浏览器验收，校验未创建企业，清理测试库 |
| `scripts/verify-merchant-entry-browser.cjs` | Edge 实际注册登录、失败恢复、布局、键盘和截图 |
| `docs/MERCHANT_ENTRY_UPGRADE.md`、`README.md`、`docs/REGISTRATION.md` | 当前流程、验证和历史说明链接 |

继续使用 `POST /api/v1/auth/merchant/register`、`POST /api/v1/auth/merchant`、`POST /api/v1/auth/sms`、`POST /api/v1/auth/phone`。会话仍沿用现有 sessionStorage。未改认证接口、业务账号、权限、审核规则或数据库结构，未增加迁移。

## 实际验证

在项目根目录执行；前端设置 TEMP／TMP 为项目 `.tools/tmp`，避免系统临时目录权限问题。

| 检查 | 命令／实际结果 | 原始输出 |
|---|---|---|
| 前端全量 | `cd web; node node_modules/vitest/vitest.mjs run --maxWorkers=4`，34 文件、184 项通过 | [前端测试](test-output/merchant-entry-web-tests.txt) |
| 新入口专项 | 同上附加 `src/merchant-entry.test.tsx`，13 项通过，包含非默认展馆、异步短信错误聚焦 | [专项测试](test-output/merchant-entry-focused-tests.txt) |
| 前端构建 | `cd web; npm run build`，TypeScript、Vite 通过；现有大于 500kB 分包提醒仍存在 | [构建](test-output/merchant-entry-build.txt) |
| 后端全量／MySQL | `cd server; . ../scripts/go-env.ps1; $env:RUN_MYSQL_TESTS='1'; go test -count=1 ./...`，通过，171.634 秒 | [后端测试](test-output/merchant-entry-backend-tests.txt) |
| Go 静态检查 | `cd server; . ../scripts/go-env.ps1; go vet ./...`，退出码 0、无诊断 | [go vet](test-output/merchant-entry-go-vet.txt) |
| 浏览器／真实认证 | 先构建前端，然后 `cd server; . ../scripts/go-env.ps1; $env:RUN_MYSQL_TESTS='1'; $env:RUN_MERCHANT_ENTRY_BROWSER='1'; go test -count=1 -run '^TestMerchantEntryBrowser$' -v`，通过，12.998 秒 | [浏览器](test-output/merchant-entry-browser.txt) |
| 更新本地服务 | 验证项目进程路径及 8080 监听归属后重启 `scripts/start.ps1 -MapPreview`。两页可见；既有轮廓预览可用，真实公开厂商仍为 9 家 | [本地服务](test-output/merchant-entry-local.txt) |

浏览器使用独立 MySQL 库 `zqzt_exhibition_test_时间戳`、临时媒体目录及临时 HTTP 端口，实际创建一个公司账号及一个独立手机号账号。测试断言企业数量为 0、厂商账号数量为 2，结束后删除整个测试库。未向业务库注册账号、修改密码或插入测试企业。测试用企业文字只用于恢复草稿断言，未保存为企业记录。

覆盖真实注册成功、重复提交、账号重名及聚焦、密码错误、密码显隐、模式重置、网络失败、注册结果不明确、Enter、倒计时、手机账号隔离、合法／外站返回、已登录跳转、草稿恢复。现有前端全量测试同时覆盖旧链接、编辑认证弹窗、工作台会话失效后保留编辑内容。

两页在 1440×900、1366×768、390×844、360×640、844×390 下均检查：无横向溢出；电脑无整页纵向溢出；短屏和帮助／错误展开后按钮、提示可滚动到达，无嵌套滚动。中英文、Tab 顺序、Enter 和 44px 显隐／主按钮均检查通过，浏览器无运行时错误。手机键盘以短视口验证可达性，未进行实体手机键盘或真实短信验收。

## 截图

- [电脑入驻 1440×900](screenshots/merchant-entry-join-1440x900.png)、[1366×768](screenshots/merchant-entry-join-1366x768.png)
- [电脑登录 1440×900](screenshots/merchant-entry-login-1440x900.png)、[1366×768](screenshots/merchant-entry-login-1366x768.png)
- [手机入驻 390×844](screenshots/merchant-entry-join-390x844.png)、[360×640](screenshots/merchant-entry-join-360x640.png)
- [手机登录 390×844](screenshots/merchant-entry-login-390x844.png)、[360×640](screenshots/merchant-entry-login-360x640.png)
- [横屏入驻](screenshots/merchant-entry-join-844x390.png)、[横屏登录](screenshots/merchant-entry-login-844x390.png)
- [英文入驻](screenshots/merchant-entry-join-en.png)、[英文登录](screenshots/merchant-entry-login-en.png)
- [更新后本地入驻页](screenshots/merchant-entry-local-join.png)

日后无需重新构建后端即可查看前端构建结果；常规本地启动使用 `scripts/start.ps1`，需保留既有本机轮廓学习预览时使用 `scripts/start.ps1 -MapPreview`。不要将开发短信或本机轮廓预览的通过结果当作生产验收。
