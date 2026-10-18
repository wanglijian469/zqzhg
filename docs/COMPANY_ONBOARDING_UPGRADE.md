# 厂商注册与企业资料填写体验升级交付记录

交付日期：2026-10-18。在现有React／TypeScript／Go／MySQL项目中实施，保留V2.2三步骤产品发布、审核和Android兼容。未部署生产服务器。

## 已实现的流程与界面

- 新注册成功后直接进入 `/merchant/booth/edit`，使用厂商工作台导航。注册只建账号，不自动创建空企业或提交审核。
- 无展厅账号可在工作台创建资料；完善、编辑和空状态入口不再跳回公开入驻表单。`/join`成为简短说明与注册／登录入口；已登录无展厅转编辑，有展厅转工作台；原tab链接保留重定向。
- 企业名称、所在地区、主营、电话／微信及企业介绍集中显示。名称、地区、主营与至少一种联系方式沿用必填要求；企业介绍建议填写，完全可空。
- 手机单列，电脑合理双列。删除旧入驻宣传条、重复标题与自动插入的大段规则，计数仅聚焦／接近限制／错误时显示。示例为浅灰斜体，真实输入为正常字体；占位符不写入数据。
- 企业介绍提供参考范文及基于已填事实的草稿建议；范文不自动应用，建议确认后追加，不覆盖原文，不补造认证、产能、客户、专利等事实。宣传摘要另作短卡片文案，不要求重复介绍。
- 品牌、工厂实拍、生产能力、资质、区域联系人、英文、视频与画册分组默认折叠。所有英文资料可空，包括宣传字段及区域联系人英文；不自动翻译。
- 工作台区分必须完成与建议补充；完整建议清单可直达介绍、Logo、封面、联系人、地址、实拍、真实生产能力、实际资质、视频和画册。英文、资质等不存在的资料不构成门槛。
- 保存云端草稿后留在当前编辑位置，刷新工作台企业信息而不卸载表单。提交成功后返回工作台展厅管理。单请求锁、账号一个主展厅约束和版本检查继续生效；失败保留输入、已上传文件及云端ID。
- 新注册可承接当前设备的未完成文字草稿；已有账号须明确选择使用本机访客草稿，已有企业不被访客内容覆盖。密码不进入草稿。未上传文件无法跨刷新恢复，界面明确提示。
- 复用原品牌图片、资质、实拍标注与排序、资源管理；待上传文件可重试／移除，失败阻止静默保存。图片说明在编辑器统一规范化，避免兼容响应补齐说明时丢失保存提示。
- 手机关键操作至少44px；底栏按实际高度与安全区留空，VisualViewport检测键盘缩小视口后回到文档流。站内离开与刷新关闭提醒未同步文字或待上传内容。

## 地区目录、API与迁移

- 离线目录位于server/catalog，固定内容SHA256、许可和来源元信息随源码与交付包提供。来源为 [modood/Administrative-divisions-of-China](https://github.com/modood/Administrative-divisions-of-China)，WTFPL-2.0；基线2023-06-30，31个省级节点，上游已停止更新，不宣传为最新完整区划。
- 省市必选、区县选填，修改上级清空下级。直辖市适配城市显示，省直辖县级单位可直接选择；无下级不强制区县。未覆盖、更名、特殊及无法匹配的历史地区允许人工填写／保留原文，不猜测修正。
- 新增 `GET /api/v1/public/administrative-regions`，不要求登录，按parent_code返回下级；code或精确归一化name可解析selected与trail；返回version与historical。不依赖外部地图、运行时网络或密钥。
- 企业Payload兼容新增可选 `region_code`，后端验证有效市／县节点并生成名称、解析为现有region_id。原region_id／region_name及旧Android、后台调用仍受支持，后台手动改地区时清除旧编码。
- 迁移013为regions增加可空administrative_code及唯一索引。只按实际使用关联／创建地区，不向业务表批量灌入全部目录。精确同名历史记录沿用原ID；并发关联序列化保护，防止重复编码。
- 手动地区也关联现有平面地区记录，审核后可进入原地区筛选。未审核地区不进入公开业务筛选。无删除、重编号或历史审核快照批量改写。
- 迁移器兼容已有011／012紧凑索引语法及013重试；备份恢复接受Schema13。新列和索引不需要删除即可回滚程序。

## 本轮测试与证据

| 范围 | 结果 |
| --- | --- |
| 前端 | npm test：22个文件，127项通过；新增7项企业编辑测试并将9项注册及相关旧入口断言更新为新流程等价覆盖 |
| 构建 | npm run build包含TypeScript检查并通过；没有独立lint脚本；保留既有大分包提示，无新增大型依赖 |
| 后端 | 隔离MySQL5.7.18：非视频全套69项通过，独立视频1项通过，业务合计70项；Go vet通过 |
| 浏览器夹具 | TestCompanyOnboardingBrowserHarness单独通过；自动建库、造隔离资料并删除；不计入70项业务测试 |
| 数据兼容 | 旧ID沿用、非法编码、省市／区县、直辖市／省直管单位、4并发解析、013中断重试、Schema13地区编码和唯一索引完整备份恢复通过 |
| 注册与审核 | 真实注册→工作台→保存→提交→管理员审核→公开地区筛选通过；新待审版本保留原公开版本；原V2.2名称最小产品提交回归通过 |
| 浏览器交互 | 7组通过：工作台落点、无空企业、保存不卸载、范文与斜体／正常输入、真实鉴权上传重试、审核／筛选、旧文字和英文保留 |
| 响应式 | 1440、1280、768、390、360px ×中英文×100%／等效125%×折叠／展开，共40状态通过；无横向溢出、底部操作越界或页面脚本异常 |

原Go整套长进程曾在视频测试提前结束，本轮仍按非视频全集＋独立视频执行，未将未执行的单条完整命令标为通过。真实OSS缺少隔离凭据跳过；旧手动夹具默认跳过，使用本轮专用夹具。

本机公开数据更新前后均为6家企业、41款产品，厂商ID一致；只读摘要与APK校验随清单验证。本次所有注册、修改、上传和审核验收均在独立测试库，浏览器上下文拦截统计POST，生产统计逻辑未变。APK仍为1.7.0+29且字节不变，V2.2与其他旧交付包保留。

## 尚未验证和明确限制

- 没有iOS／Android真机；系统相册权限、真实软键盘、拍照与安全区需真机验证。125%为等效CSS视口，不是人工点击浏览器缩放。
- MySQL8、真实OSS与Linux运行环境未验收；Linux程序仅交叉编译。Windows本地程序及MySQL5.7实际验证。
- 地区是2023年历史基线，缺失或更名使用人工入口；不提供未核验的自动区划更新。
- 浏览器后退未新增全站路由拦截，依靠自动本机文字草稿；文件选择不能跨刷新保留。首次创建响应断网不确定时不会自动覆盖已有展厅，应进入原资料核对。
- 截图中的企业、产品、联系人及几何图片均为明确标识的隔离测试资料，不代表真实经营信息。

## 本地命令与部署回滚

本地已运行：http://localhost:8080；直接编辑入口：http://localhost:8080/merchant/booth/edit；未登录先认证。

~~~powershell
. ./scripts/go-env.ps1
cd web
npm test
npm run build
cd ..
./scripts/stop.ps1
./scripts/start.ps1
~~~

后端隔离测试：设置RUN_MYSQL_TESTS=1后在server执行：

~~~powershell
go test -v -count=1 -skip '^TestVideoWorkerProcessingRetryAndRecovery$' ./...
go test -v -count=1 -run '^TestVideoWorkerProcessingRetryAndRecovery$' .
go vet ./...
~~~

浏览器：设置COMPANY_BROWSER_HARNESS=1、RUN_MYSQL_TESTS=1，在server运行 `go test -v -count=1 -timeout 14m -run '^TestCompanyOnboardingBrowserHarness$' .`，等.local/company-browser-fixture.json生成，再在根目录运行 `node scripts/verify-company-onboarding.cjs`。令牌和样本随夹具结束删除，不分发。

生产未部署。升级顺序：

1. 备份实际正在使用的数据库、媒体、环境配置、运维控制目录、旧程序及完整dist。本机迁移前数据库、媒体、配置和旧程序位于.local/backups/pre-company-onboarding，含私有数据，不随包分发。
2. 在隔离环境验证启动自动执行013、目录接口、旧数据回填、注册登录、草稿、审核和产品管理。
3. 配套替换后端与完整dist，保留已有环境、PUBLIC_BASE_URL、媒体、FFmpeg和进程管理配置。不要只换前端。目录已嵌入后端二进制，源码构建需保留server/catalog。
4. Windows用bin/exhibition-windows-amd64.exe替换原程序；Linux用bin/exhibition-linux-amd64并chmod +x，Linux实际运行先在测试环境验证。
5. 检查/api/v1/health、/api/v1/public/administrative-regions、注册后落点、保存／提交、公开筛选、登录与产品发布；核对SHA256SUMS。
6. 回滚恢复升级前配套程序与dist。013列和索引可保留，旧客户端忽略新增字段，region_id仍有效。含Schema13的备份用本轮兼容恢复工具；如恢复数据库，暂停写入并恢复同时间点数据库／媒体／配置，勿覆盖升级后的业务数据。

## 主要修改文件

- 企业界面：web/src/company-editor.tsx、company-editor.css、company-editor-messages.ts、company-region-field.tsx、company-gallery-upload.tsx。
- 流程及复用：web/src/main.tsx、merchant-console.tsx、merchant-auth.tsx、company-portal.tsx、types.ts、admin.tsx。
- 区划与校验：server/administrative_regions.go、catalog三个来源文件、migrations/013_administrative_regions.sql、model.go、content.go、main.go、backups.go。
- 测试：company-editor.test.tsx、merchant-registration.test.tsx、form-guide.test.tsx、category-picker.test.tsx、merchant-workspace.test.tsx、merchant-ui.test.tsx、hall-test-fixture.ts；server/company_onboarding_test.go、company_onboarding_browser_test.go、operations_test.go。
- 交付：scripts/verify-company-onboarding.cjs、package-company-onboarding.mjs、docs/openapi.json、本记录及README.md。

截图：screenshots/isolated-1440-basic.png、isolated-768-basic.png、isolated-new-company-390.png、isolated-390-introduction.png；完整展开状态及360px图片随包提供。手机图片使用真实视口截图，避免长截图中固定工具栏位置造成误解。
