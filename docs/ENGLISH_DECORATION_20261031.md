# 中文页面隐藏英文装饰文字

本次仅修改本机项目，未部署线上。预览：<http://localhost:8080/>。

## 实际改动

| 文件 | 内容 |
| --- | --- |
| `web/src/exhibition-home.tsx` | 首页三处英文眉题、厂商入驻英文眉题及宣传图旁的英文装饰仅在 English 模式渲染；中文馆名保留；整块英文基线在中文模式不生成。 |
| `web/src/exhibition-portal.tsx` | 展馆选择组件的装饰缩写仅在 English 模式渲染。 |
| `web/src/halls.tsx` | 筹备展馆的英文眉题按语言渲染，保留馆名及状态。 |
| `web/src/company-portal.tsx` | 企业介绍、生产能力、资质与荣誉的英文装饰按语言渲染，共用组件的展厅预览同步生效。 |
| `web/src/v2.css` | 移动端针对英文装饰使用明确的类选择器，避免移除英文后误把中文馆名当成最后一项隐藏。 |
| `web/src/showroom-v2.css` | 允许英文模式生成的装饰眉题显示，限定于相关展厅标题。 |
| `web/src/english-decoration.test.tsx` | 新增 9 项语言、刷新持久化、展馆组件、预览和业务资料保留测试。 |
| `scripts/verify-english-decoration.cjs` | 本机只读浏览器验收与截图脚本。 |

没有改动接口、数据库、厂商资料、图片中的文字及语言持久化机制。厂商填写的英文名称、型号、品牌、证书名称及 App 等功能名称保留。公共页脚、独立展厅页脚、注册表单及分享功能文件与修改前的 SHA256 一致，见 `test-output/english-decoration-protected.json`。

## 测试结果

- 相关组件测试：6 个文件，45 项全部通过（含本次新增 9 项）。覆盖首页、筹备馆、展馆选择组件、企业介绍/生产能力/资质荣誉、展厅预览、独立展厅及公共页脚。
- TypeScript 类型检查通过；Vite 生产构建通过。构建保留大于 500 kB 的分块提示，本次未调整打包策略。
- 真实浏览器：本机 Edge 无头模式；首页及 `/v/jinong?tab=about` 在 1440、768、390、360px 下，分别检查中文与 English，共 16 个布局状态通过，无文档横向溢出。保留原有移动端响应式规则，原本在小屏收起的装饰仍按原布局处理。
- 中文下装饰节点不生成，没有透明文字或隐藏高度占位；中文馆名、搜索、企业和产品内容保持显示。English 下首页三处眉题和其他英文装饰生成；展厅企业介绍与生产能力英文装饰可见。切回中文立即移除。
- 语言刷新保留：1440px 验证 English 刷新，390px 验证中文刷新，均通过。
- 只读回归通过：搜索“冀农”、进入厂商结果、免费入驻入口、筹备馆中文→English→中文、公共页脚管理后台与投诉入口。独立展厅仍使用精简页脚。
- 展馆选择组件及展厅预览通过组件测试核对，不虚构额外公开访问地址。
- 浏览器无页面脚本异常，无业务写请求；未注册账号、发短信、修改资料或提交申请。

可复查记录：

- `test-output/english-decoration.json`：16 个布局状态、语言与回归结果。
- `test-output/english-decoration-components.log`：45 项组件测试结果。
- `test-output/english-decoration-build.log`：生产构建输出。
- `test-output/english-decoration-protected.json`：相关页脚与业务文件未改动核对。

## 截图

- [中文首页 PC](screenshots/english-decoration/home-zh-1440.png)
- [中文首页手机](screenshots/english-decoration/home-zh-390.png)
- [English 首页 PC](screenshots/english-decoration/home-en-1440.png)
- [中文首页内容区](screenshots/english-decoration/home-zh-sections-1440.png)
- [中文展厅手机](screenshots/english-decoration/showroom-zh-390.png)
- [English 展厅手机](screenshots/english-decoration/showroom-en-390.png)

其余 768、360px 及中英文展厅截图保存在 `docs/screenshots/english-decoration/`。
