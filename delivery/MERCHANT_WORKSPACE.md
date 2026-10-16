# 厂商工作台与代表展品（网站／后端1.5.10）

## 厂商如何进入后台

公开网站未登录时显示“我要入驻”和“厂商登录”。已有公司账号从“厂商登录”使用原账号密码登录，进入 `/merchant`；登录后顶部入口合并为“厂商工作台”。网站后台使用独立深蓝侧栏与浅色工作区，不再套用公开展馆的导航及页脚。

新厂商仍在 `/join` 填写。注册／登录后保存草稿或提交审核成功，自动进入工作台并显示结果。仅为图片上传或分类新增而登录时，保留原表单并返回原操作，不提前跳转。已有账号已有展位时不会自动覆盖原资料；直接访问 `/join` 会进入原工作台，没有展位的账号则继续完善入驻。

四项主导航：工作台 `/merchant`、展位管理 `/merchant/booth`、产品管理 `/merchant/products`、展台推广 `/merchant/promotion`。桌面可收起侧栏，手机抽屉支持键盘焦点和Esc关闭；中英文切换保留当前路由。旧 `/join?tab=dashboard|boothManage|productManage|promotion` 自动转换，保留查询条件和页码。

工作台显示完整产品统计、公开／审核状态、驳回提醒及快捷操作。展位管理保留企业资料、短视频和电子图册；产品管理保留20条分页、搜索、审核及公开状态筛选。产品及展位编辑页可以直接打开，返回原列表保留筛选和页码，本机草稿仍按账号隔离。会话失效在原页重新登录，文字与待上传文件留在当前页面；登录后显式重试原操作。切换其他账号会进入新账号工作台，不沿用前一账号编辑状态。

## 代表展品与本厂产品

- **代表展品**：企业主打展示，最多3款。在“产品管理 → 代表展品设置”操作，与管理员的首页精选相互独立。
- **本厂产品**：全部公开产品目录，包含代表展品，继续按分类、关键词和页码浏览。更改目录筛选不会更改代表展品。

默认“自动推荐”沿用原公开排序：平台精选优先、排序值较小优先、同值产品编号从大到小，最多3款。切换“手动选择”后，可搜索和分页选择本厂公开产品，移除、上移或下移，点击“保存代表展品设置”生效；不足3款不补齐，手动清空后隐藏整个代表展品区域。恢复自动模式会清空手动ID列表。

只使用当前公开产品版本。修改待审期间仍可选择原公开版本，不会显示未审核资料。产品下架后立即退出公开代表展品，配置保留，重新公开后可恢复；设置页标明失效并要求移除失效项再保存。展位不公开时，其代表展品与原产品／媒体一并不可公开访问。无展位账号需先完善入驻。

配置使用版本校验，冲突时当前选择保留，厂商可以刷新配置版本、核对后再次保存；不会自动覆盖较新设置。该功能只调整公开产品的展示顺序，不修改产品内容、配额或平台精选，无需重复内容审核。

## 接口、迁移与兼容

`GET /api/v1/merchant/showroom-display` 返回当前账号展位的 `vendor_id/mode/product_ids/version/limit/selected/preview_products`；未配置默认为auto、version 0。selected标明所选ID、公开状态及有效产品的公开快照。

`PUT /api/v1/merchant/showroom-display` 接收 `mode: auto|manual`、`product_ids`、`version`。自动模式ID列表为空，手动允许0–3个不重复且本厂公开的产品ID。账号权限、上限与公开资格由服务端验证；配置和 `showroom_representatives_saved` 操作日志同事务保存，版本冲突返回 `stale_revision`。两接口 `Cache-Control: no-store`，失败保留当前选择。

公开展厅响应新增 `representative_products`，按配置顺序返回有效公开产品，网站不再另取产品列表前三款作为代表展品。原 `/public/products` 和其他旧字段保持不变。

迁移008新增 `showroom_display_settings`，按展位唯一保存模式、顺序ID JSON、配置版本、操作人和时间。使用 `CREATE TABLE IF NOT EXISTS`，中断后重跑不会覆盖配置，不批量创建真实厂商设置或修改原内容／精选。备份和恢复需包含新表、原业务表和媒体目录。

**Android继续原签名1.5.2+19，不重打包。旧App仍按原公开产品排序自动显示3款，不使用网站手动设置。** 本轮手动设置仅作用于网站。

## 验收与交付

独立MySQL测试覆盖默认自动、手动0／1／3款、重复和超限、跨账号／草稿产品、旧公开版本、下架恢复、父展位、并发版本冲突、事务回滚及迁移重复执行。浏览器验收只向8081隔离库写入，检查真实登录、注册保存／提交、选择排序、失效状态、旧链接、深链接刷新、编辑返回、会话失效续填及中英文五种宽度。详见 [TESTING.md](TESTING.md)。

- [工作台桌面截图](screenshots/merchant-workspace-dashboard.png)
- [手机工作台](screenshots/merchant-workspace-dashboard-390.png)
- [产品管理](screenshots/merchant-workspace-products.png)
- [代表展品设置](screenshots/merchant-workspace-representatives.png)
- [手机导航抽屉](screenshots/merchant-workspace-mobile-drawer.png)

截图均为隔离库资料。实际上线保持原资料和默认自动推荐，由厂商自行配置。网站与Windows/Linux后端交付1.5.10，备份位置 `.local/backups/pre-merchant-workspace-v1510`；恢复方法见 [DEPLOYMENT.md](DEPLOYMENT.md)，校验清单 `delivery/manifest.json`。
