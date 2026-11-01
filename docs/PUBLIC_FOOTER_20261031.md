# 公共页脚三栏优化：本机交付

预览：<http://localhost:8080/>。管理后台：<http://localhost:8080/admin>。本次已完成本机代码、构建与验收，未更新线上服务器或制作部署包。

## 页面效果

- 普通公共页共用 `PublicFooter`：左栏为品牌及“连接厂家与采购，让产业被看见。”；中栏为厂商服务；右栏为帮助与支持。
- 六个功能链接按需求分组，分别指向 `/join`、`/guide`、`/services`、`/service-requests`、`/app`、`/merchant/login`，每个入口在页脚中仅出现一次。
- 底部分隔线下保留公司版权、技术支持、ICP备案、可选公安备案、原平台声明及协议入口。“管理后台”位于底部，为低调文字链接，电脑和手机均可进入原 `/admin` 登录页面。
- 保留深蓝背景、主页面容器宽度；标题 16px、常用链接 15px。桌面三栏，手机品牌独占一行、两组链接并排；小于 360px 为单列。
- 手机链接触控区域至少 44px，文字自然换行；悬停有下划线和提亮，键盘聚焦有明显边框。
- 地图、注册和登录继续使用原专用紧凑页脚，厂商独立展厅继续使用已精简页脚。

## 修改文件

| 文件 | 作用 |
| --- | --- |
| `web/src/public-footer.tsx` | 新的共享三栏公共页脚及底部法律、管理入口 |
| `web/src/public-footer.css` | 限定作用范围的深蓝布局、手机适配和交互样式 |
| `web/src/site-common.tsx` | 提取纯备案链接 `FilingLinks`，原 `ICPRegistration` 仍保留原有服务入口和结构 |
| `web/src/main.tsx` | 普通公共页接入共享组件，专用页脚分支保持原样 |
| `web/src/public-footer.test.tsx` | 分组、真实路由、后台入口、备案协议和特殊页面隔离测试 |
| `scripts/verify-public-footer.cjs` | 本机只读浏览器布局、链接、键盘、后台权限及专用页脚验收 |

未新增业务接口、数据库结构、权限或前端依赖，没有修改协议内容。厂商独立展厅、分享组件、注册登录及地图相关的六个受保护文件与修改前 SHA-256 完全一致。

## 验收结果

| 项目 | 结果 |
| --- | --- |
| TypeScript、资源准备和 Vite 构建 | 通过 |
| 七个专项测试文件 | 49 项通过 |
| 响应式布局 | 1440、1280、768、390、360、320px，以及 390/1440 的等效 125% 缩放，共 8 种状态通过 |
| 对齐与可读性 | 与主页面容器左右对齐，无横向滚动、重叠或文字截断；链接纵向排列，手机触控高度满足要求 |
| 链接操作 | 两种屏幕宽度下，六个常用入口均已实际点击；两次后台键盘进入，共 14 项入口检查通过 |
| 后台权限 | 打开原管理员登录页面；未登录访问 `/api/v1/admin/contents` 返回 401 |
| 普通公共页面 | 首页、厂商列表、产品列表、产品详情、指南、服务说明、投诉申请及三类协议页面，共 10 类路径通过 |
| 专用页脚 | 电脑和手机的地图、注册、登录、独立展厅共 8 项通过；六个相关源文件哈希一致 |
| 悬停、键盘及语言 | 悬停反馈、Tab 焦点、Enter 进入后台及英文导航通过 |
| 数据与异常 | 浏览器异常 0；未提交账号、短信、投诉或业务内容；本机网站保持运行 |

完整页脚高度约为桌面 263px、390px 手机 457px；320px 极窄屏采用单列自然滚动，约 655px，以保留规定字号和触控区域。没有通过压小字体、裁切或隐藏入口降低高度。

扩大回归运行了 17 个测试文件、106 项测试：91 项通过，15 项为此前已记录的历史失败。逐项对比 `admission-guide-baseline.json` 的失败名称，名单完全相同，没有新增失败。没有将这组回归描述为“全部通过”，也没有为旧测试改写现有业务。

自动检查首次快速切换多个页面触发了原有访问频率限制；最终验收已降低页面切换频率并遵守重试间隔，通过全部检查。未关闭或修改防采集规则。

## 记录和截图

- `docs/test-output/public-footer.json`：浏览器详细结果。
- `docs/test-output/public-footer-isolation.json`：专用页脚相关文件的哈希对照。
- `docs/test-output/public-footer-regression.json`：扩展回归及历史失败对比。
- `docs/screenshots/public-footer/footer-1440-1.png`：PC 页脚。
- `docs/screenshots/public-footer/footer-390-1.png`：手机页脚。
- `docs/screenshots/public-footer/footer-320-1.png`：极窄屏单列。
- `docs/screenshots/public-footer/admin-focus-390.png`：手机后台入口和键盘焦点。
- `.local/public-footer/`：构建、测试及浏览器原始日志。

复验命令：在 `web` 目录用现有 Node 运行 `node node_modules/vitest/vitest.mjs run src/public-footer.test.tsx src/showroom-footer.test.tsx src/showroom.test.tsx src/showroom-v2.test.tsx src/admission-guide.test.tsx src/free-service.test.tsx src/merchant-entry.test.tsx --maxWorkers=4`；本机网站启动后，在项目根目录运行 `node scripts/verify-public-footer.cjs`。浏览器脚本只接受固定本机地址，不执行写入或登录提交。
