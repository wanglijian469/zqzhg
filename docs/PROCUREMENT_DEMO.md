# 采购中心隔离演示与短信登录

## 体验入口与账号

演示地址为 <http://localhost:8081/buyer>，只监听本机回环地址。原主站 <http://localhost:8080> 继续使用原数据库，演示资料不会写入主站。

| 身份 | 手机号 |
| --- | --- |
| 采购商 | 13900009001 |
| 厂商甲 | 13900009002 |
| 厂商乙 | 13900009003 |

在登录页选择号码，点击“获取验证码”，把页面显示的本次动态验证码填入后登录。没有固定验证码、万能验证码或预填会话，不发送真实短信。验证码五分钟有效，一次使用，错误五次失效；获取验证码保留既有间隔和限流。

两个工作台的“切换体验账号”使用正常退出流程，再选择其他手机号重新获取验证码。验证码登录只接受已有且已验证手机号的业务账号，不自动创建账号或厂商；未注册号码使用注册入口，已有公司账号可先公司账号登录再绑定手机号。管理员使用独立后台账号密码。

## 预置流程

演示预置两家已审核厂商及产品、一个待报价定向询价、一个已审核多产品公开需求。公开项目已准备厂商甲完整报价和厂商乙部分报价，以及消息和通知。

1. 采购商进入“我的询价与需求”，查看公开项目的逐项比较；部分报价不会作为零元计算整包总价。
2. 切换厂商甲，进入厂商工作台“商机中心”（`/merchant/opportunities`），处理定向询价、逐项报价、修订及洽谈。
3. 切回采购商查看新报价、选择意向厂商；页面明确确认后才向该厂商开放本项目联系人电话。
4. 切换厂商乙查看自身报价和独立会话，不能查看厂商甲报价、附件、会话或参与名单。
5. 可新建采购需求提交审核，或在原定向询价上申请扩大邀请范围，不创建重复项目。

这些均为虚构本机资料，重置会删除演示中的修改。本轮商机迁回与样式升级不重置演示数据。完整业务闭环在另建隔离测试库验收，现有演示只做登录和只读页面检查。

## 谁审核、在哪里审核

管理员或获得 `procurement.review`（审核采购需求）权限的操作员审核，普通采购商和厂商不能自行审核。后台路径为“采购中心 → 需求审核”：

- 主站：<http://localhost:8080/admin/procurement/reviews>，使用原主站后台账号。
- 演示：<http://localhost:8081/admin/procurement/reviews>，使用 `demo-admin`；随机生成的本机演示密码单独保存在 `.local/procurement-demo/admin-login.txt`。该文件不进入版本控制，不包含配置加密密钥。

在待审核记录上点击“查看资料并审核”，检查需求内容后通过或填写原因驳回。新发布的采购需求、扩大邀请范围须审核；首次仅向产品所属厂商询价直接送达。演示公开需求已预审通过，因此刚初始化的待审核列表为空，新提交后才出现。

## 启动、停止与显式重置

前提：项目本机 MySQL 在 `127.0.0.1:13306` 可用，Go、Node 可用，已构建 `web/dist`。在仓库根目录运行：

```powershell
./scripts/start-procurement-demo.ps1
./scripts/stop-procurement-demo.ps1
# 只在明确需要丢弃演示修改时运行：
./scripts/start-procurement-demo.ps1 -Reset
```

启动脚本使用独立可执行程序、数据库 `zqzt_procurement_demo`，媒体、临时文件、日志、运维和备份均在 `.local/procurement-demo`。使用同一项目代码及网页构建，保持 `8080` 服务和资料不变。数据库连接账号沿用本机配置，不复制任何现有展厅资料。

重复初始化按持久化事件标记去重，不覆盖体验中的修改；服务已经启动时返回已有入口。重置先停止匹配 PID 和可执行路径的演示进程，后端再次验证开发环境、开发短信、回环 HTTP/DB 地址、明确 `PROCUREMENT_DEMO=true` 和精确数据库名，才删除演示数据库；仅清理经过绝对路径范围检查的媒体、临时、运维及备份子目录，保留随机凭据。

`seed-procurement-demo` 和 `reset-procurement-demo` 不是普通生产运维命令；不得在正式环境启用演示开关。生产模式、非回环绑定或其他数据库名均拒绝执行。

## 阿里云真实验证码短信切换

沿用现有阿里云适配器及模板，不新增迁移。正式环境设置：

```dotenv
APP_ENV=production
PROCUREMENT_DEMO=false
SMS_PROVIDER=aliyun
ALIYUN_ACCESS_KEY_ID=<服务端访问密钥 ID>
ALIYUN_ACCESS_KEY_SECRET=<服务端访问密钥>
ALIYUN_SMS_SIGN_NAME=<审核通过的短信签名>
ALIYUN_SMS_TEMPLATE_CODE=<现有验证码模板编号>
```

按既有生产要求保留配置加密密钥等部署配置。密钥只放服务端，不提交代码或放公开配置。重启后，前端继续用 `/api/v1/auth/phone/code`（`purpose=login`）和 `/api/v1/auth/phone/login`；正式模式响应不含模拟验证码。生产环境禁止 `SMS_PROVIDER=development`。本轮没有配置云密钥、调用真实短信或部署生产。

公开配置只增加 `capabilities.phone_sms_auth`、`development_sms`、`procurement_demo` 以及演示开启时的号码提示，不包含短信密钥。原生 App 的旧短信接口保留。商机业务短信仅预留独立接口，当前通知使用站内消息，不复用验证码模板发送商机通知。

## 验收记录

- [Go 全量回归](test-output/procurement-return-go-full.txt)、[验证码生命周期与演示隔离专项](test-output/procurement-demo-go-targeted.txt)、[静态检查](test-output/procurement-return-go-vet.txt)。
- [215 项前端测试](test-output/procurement-return-web-full.txt)、[类型检查与构建](test-output/procurement-return-build.txt)。
- [动态验证码实际浏览器验收](test-output/procurement-return-demo-browser.json)：三账号真实页面登录、两个工作台及旧地址兼容、正常退出，13 组只读页面检查无脚本错误，不重置数据。
- [完整业务闭环与工作台回归](test-output/procurement-return-browser.json)：独立测试数据库、98 组布局，含附件、审核扩散、两家报价、意向、洽谈、关闭及键盘导航。

浏览器验收复现：启动演示后运行 `node scripts/verify-procurement-demo.cjs`。该脚本会修改演示项目，复现前后如需最初状态，应显式运行重置命令。仅在本机隔离环境执行，不能用于生产。

最新升级验收：前端 215 项测试、Go 全量回归及两套工作台 98 组布局通过。`node scripts/verify-workspace-demo.cjs` 在现有演示执行动态验证码登录和只读页面检查，不重置项目、不提交报价、不修改消息已读状态；本轮使用此脚本。原 `verify-procurement-demo.cjs` 是会修改演示记录的完整流程脚本，不用于本轮保留体验资料的验收。
