# 访客收藏与登录同步 · 1.5.16

## App 使用

底部菜单依次为“看厂家、找产品、收藏、我的展位”。启动仍显示原展馆首页，此时不选中底部菜单。主页面左上角的首页图标可以返回首页；系统返回键先从主页面回首页，再退出。

厂商和产品的列表卡片、详情页均可点击“收藏／已收藏”，按钮不会触发卡片跳转。收藏页分厂商、产品两个标签，显示数量，按最近收藏时间倒序排列。点击条目打开原详情，返回时保留浏览位置；取消收藏后各入口状态同步变化。

未登录时收藏仅保存在本机，按服务器地址隔离，重启后恢复。卸载或清除应用数据会丢失尚未同步的本机收藏。收藏页提供“登录／注册并同步”：普通用户可注册访客账号，无需企业资料或展位；已有厂商可选择厂商账号登录。

登录后自动合并当前本机访客收藏并去重，服务器确认后清理对应访客记录。待合并记录绑定首次登录账号，不会因切换账号而上传到另一账号。账号收藏、待同步队列和访客记录分别保存；退出账号后显示剩余访客收藏。未完成同步的账号操作保留在本机，重新登录同一账号后重试。

收藏操作先保存本机，登录后、进入收藏页、App 返回前台和下拉刷新时同步。离线或请求失败显示重试入口，不清空收藏。服务器尚未升级时，提示升级同步服务，本机收藏仍可使用。

普通访客不能编辑或管理展位。“我的展位”提供切换厂商账号的入口；不自动把访客变成厂商。切换账号登录失败时保留原账号会话。

## 数据与接口

收藏以服务器地址、类型 `vendor|product` 和内容 ID 标识，厂商更换展厅地址不会新增重复条目。新接口均位于 `/api/v1`：

| 接口 | 行为 |
|---|---|
| `POST /auth/visitor/register` | 访客注册，接受 `login_name/password`；固定 visitor 角色，返回既有 Session 格式 |
| `POST /auth/visitor` | 访客密码登录，接受 `username/password`；厂商继续使用原厂商登录接口 |
| `POST /public/favorites/resolve` | 只读解析最多 100 个 `{kind,content_id}` 引用，返回公开摘要或不可用状态 |
| `GET /favorites` | 当前账号收藏列表，支持 kind、page、page_size，返回分页与两类收藏计数 |
| `POST /favorites/sync` | 接收最多 100 个有唯一 operation_id 的增删操作，返回已确认编号 |

注册名沿用全局唯一规则，密码为 12–72 个 UTF-8 字节且不能全为空白。沿用现有 bcrypt、登录限流及会话机制。管理员的厂商账号列表仍只列厂商。

同步操作包含 `operation_id/kind/content_id/favorited`，按账号串行执行并持久化去重记录。相同操作编号重试不再次执行；换成不同内容复用编号会返回 `operation_conflict`，整批回滚。不同设备的新操作以服务器最后接受的操作为准，不使用设备时钟决定冲突。

收藏只保存内容引用，不证明目标存在或公开。未知、草稿、下架内容以及所属厂商下架的产品，都返回 `available:false`，不附加私有内容。失效收藏继续保留并可取消；App 网络失败只显示加载失败，不把内容判为下架。接口定义见 [OpenAPI](openapi.json)。

## 后端升级与迁移

使用配套 `zqzt-favorites-v1.5.16-backend.zip`。包内包含 Windows amd64、Linux amd64、Linux arm64 后端程序、迁移参考、接口文档和验证记录。网站界面保持 1.5.13，本次不增加网页收藏页。

1. 先备份现有数据库和后端程序，停止现有后端服务。保留原 `.env`、数据库名称、媒体目录和网站目录。
2. 从包内 `bin` 选择对应系统架构的后端替换原程序。Linux 保留可执行权限或执行 `chmod +x exhibition`；使用原工作目录、服务配置和环境变量启动。
3. 程序自动执行嵌入的 `009_favorites.sql`，新增 favorites、favorite_operations 两张表；不修改已有厂商、产品、审核或图片资料。迁移可重复启动恢复，无需手工执行 SQL。保留的 SQL 文件用于审阅。
4. 确认健康检查正常，再安装 App 1.5.16+24，并验证访客登录和收藏同步。不要用示例配置覆盖现有生产配置。

发布顺序为后端在前、App 在后。本次仅生成本地交付文件，未发布线上，也未对当前业务数据库执行迁移。回退程序时保留新增表及其中数据，旧后端暂不能提供收藏同步。

## 验证与截图

- [Flutter 静态分析](test-output/favorites-analyze-final.txt) 无问题；[完整回归](test-output/favorites-flutter-final.txt) 57 项通过，另有 2 项历史截图任务默认跳过；本次收藏截图任务已实际运行。[补充 UI 与同步测试](test-output/favorites-final-ui.txt) 覆盖本机持久化、自动合并、双设备模拟、丢失响应重试、账号切换、新会话接续、旧服务器、首页返回和访客权限入口。
- [Go 全套测试](test-output/favorites-go-regression.txt) 42 项通过，[vet](test-output/favorites-go-vet.txt) 无问题。数据库测试只使用新建的隔离测试库；覆盖迁移重复执行、访客权限、全局重名、批次回滚、并发去重和公开资料隔离。
- 布局覆盖 360／390／768px、中英文与 1.4 倍字体；注册页额外检查 360px 中英文大字体。截图使用测试资料和本地字体，不代表真实设备验收，字体未加入安装包。

| 页面 | 截图 |
|---|---|
| 首页与导航 | [首页](screenshots/favorites-home.png) |
| 收藏操作 | [厂家卡片](screenshots/favorites-manufacturer.png) · [访客收藏](screenshots/favorites-guest.png) · [产品收藏](screenshots/favorites-products.png) |
| 登录同步 | [访客注册](screenshots/favorites-visitor-register.png) · [账号收藏](screenshots/favorites-account.png) |
| 状态与适配 | [不可用内容](screenshots/favorites-unavailable.png) · [360px 英文大字体](screenshots/favorites-360-en-large.png) |

安装包、后端包和 SHA-256 清单位于 `delivery`，交付元数据见 `manifest-favorites-1.5.16.json`。
