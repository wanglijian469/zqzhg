# 企业名称防重：本次收尾记录

日期：2026-11-05。用户要求尽快收尾，余下工作下次继续。

## 已确认范围

- 公开站点确有两条“河北春耕机械制造有限公司”：`/v/v-64`（12张企业图片，产品65“拖拉机旋耕机”）和 `/v/chungeng`（产品62“液压翻转犁”）。
- 用户明确选择暂时保留两份资料，仅禁止今后的重复创建。不合并、不下架、不删除任何现有资料或媒体。
- 原因：原实现约束一个账号一个展厅、登录名唯一，没有约束不同账号使用相同企业名称。

## 本机改动（未提交、未上线）

- `server/vendor_name.go`：规范化公司名称比较键，忽略大小写、全角 ASCII、空白、零宽空格/BOM，保留法律主体后缀和分公司名称。
- 当前草稿和公开版本保留名称；已下线展厅也占用名称；历史版本不永久占用旧名。既有重名资料保留日常编辑能力。
- 新建、改名、后台代录和导入统一在保存处校验，冲突返回409和企业名称字段错误。
- 复用现有主展馆行锁，内容写事务在第一次读取前取锁，防止不同进程并发保存及 MySQL REPEATABLE READ 旧快照漏查。没有新增迁移、表或数据修正。
- 网站和 App 添加中英文友好提示，复用已有字段定位和输入保留逻辑。
- 调整文件：`server/content.go`、`server/admin.go`、`server/import.go`、`server/public_demo_import.go`、`server/procurement_demo.go`；`web/src/form-guide.tsx`、`web/src/company-editor-messages.ts`；`mobile/lib/onboarding.dart`、`mobile/assets/i18n/{zh,en}.json`。
- 新测试：`server/vendor_name_test.go`、`web/src/vendor-name.test.tsx`、`mobile/test/vendor_name_test.dart`。

## 已运行验证与限制

- 测试先复现原缺陷：跨账号重名创建返回201，改名返回200，并发两个请求均201。实现后原三项独立测试通过。
- 六项企业防重独立复跑全部通过（`go test -run '^TestVendorName' -count=1`，28.096秒）。覆盖规范化、旧公开名与草稿名占用、历史旧名释放、离线保留、产品名称不受影响、导入回滚、历史重名可编辑。
- Web：`vendor-name.test.tsx` 和 `form-guide.test.tsx` 共3项通过。TypeScript `tsc -b` 返回0。
- App：新增重名提示测试与五项产品/企业编辑流程测试通过。旧入驻测试失败：期待 `NativeJoinGuide`，实际不存在；原 HEAD 独立源码副本复跑同样失败。
- 扩展后端回归有3项历史失败，均已通过 `go test -overlay` 使用原 HEAD 代码重现：后台测试旧账号查询导致 `login_name_length`；首页展示测试缺协议确认 `agreement_required`；旧短信登录接口已停用 `sms_login_disabled`。没有修改这些旧测试或放宽业务限制。
- Vite生产构建未完成：esbuild删除系统 Temp 下临时文件返回 `Access is denied`。下次将 TEMP/TMP 设置到项目 `.tools/tmp` 后重试；当前不宣称生产构建通过。
- 未作浏览器端到端及安卓真机验收，未构建新 APK。
- 未部署 ECS、未修改生产数据库、未替换下载、未提交或推送 GitHub。

## 下次继续

1. 六项企业防重独立复跑已通过，见 `.local/vendor-name-final.log`；其余日志 `.local/vendor-name-{backend,web,app,types,build}.log`，历史对照 `.local/vendor-name-baseline/`。
2. 复核共享事务锁的范围、锁顺序及并发改名/后台创建，再补足有协议确认的审核流程测试。
3. 解决 Temp 权限导致的构建失败，完成浏览器企业表单保存冲突及现有资料编辑回归。
4. 按后续用户指示再提交、打包和上线；沿用既有地图生产产物验收要求。正式下载目前仍为已发布的1.10.9+50，不得误用历史脚本降级到45。
5. 任何后续整理仍遵守用户本次选择：两份资料保留，未经新指示不迁移产品或变更旧展厅地址。

## 检查范围说明

排查中一次“全部企业及关联统计”的生产数据库查询脚本被自动审批拦截，原因是范围超出核对指定企业的需要。未执行该脚本，随后仅通过公开接口读取用户指出的两条企业及各自产品信息，没有导出生产数据库。
