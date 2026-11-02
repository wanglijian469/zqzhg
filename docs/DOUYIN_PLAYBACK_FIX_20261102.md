# 抖音黑屏和重复勾选修复（47版独立测试包）

## 已核实的原因

1. `https://v.douyin.com/c3Tok9wUn9o` 跳转到图文笔记7551737336111893818。它不是普通视频地址，官方视频接口没有返回播放器。现在明确提示“这是抖音图文链接，当前视频播放器暂不支持”，不再使用笼统失败提示或将它当成播放器地址。
2. 用户补充的 `https://v.douyin.com/BBA3wTPX7WE/` 对应视频7136314708138724640，官方接口正常，尺寸544×960。
3. 原生资源名单漏掉 `lf-developer-cdn.open-douyin.com`。`open-douyin.com` 与 `douyin.com` 是不同域名，原来的后缀规则未覆盖官方播放器脚本。此前将被阻止的资源返回空的成功响应，因此页面可以保持黑屏而不提示失败。

## 实现

- 仅补充两个实际核实的精确主机及命名空间：`lf-developer-cdn.open-douyin.com/obj/open-platform-scm/`、`lf-c-flwb.bytetos.com/obj/rc-client-security/`。
- HTTPS、官方播放器地址校验、受限iframe、用户手势、禁止任意导航、文件访问及SSL错误拒绝等规则保持。未知资源返回403；未放开所有域名，未放开额外统计域名。
- 加入加载状态、加载超时反馈、手动重试及主动“打开抖音原视频”入口；不自动跳到外部，不自动循环重载。关闭／后台释放及全屏返回保持。
- Web、App去掉逐条“我确认该视频由本人发布，或已取得合法展示授权”勾选，不把它偷偷改成默认已同意。
- 后端改为复用真实、有效的企业服务协议。缺失协议或有效版本变化仍拦截；平台账号也没有例外，后台复用同一协议处理器主动确认自己的当前协议。
- 历史逐条授权审计保留；新客户端不冒填该字段，不伪造确认人或时间。注册、协议文本、协议版本、审核、权限、配额和本地视频保持。本轮无数据库迁移，仍为Schema30。

后台增加当前账号自己的协议读写入口 `/admin/service-acceptances`，沿用既有平台身份守卫和协议写入处理器，不允许指定其他账号。新客户端继续使用既有视频接口版本2。

## 实际验证

- 用实际Java网络策略逐一决定Chrome资源请求是否放行，未知请求按403处理。用户提供的7136314708138724640视频出现画面，真实鼠标点击后播放时间推进到约4.97秒，非暂停、无媒体错误。没有调用跨域脚本模拟播放，也没有额外调用play代替用户点击。
- 23项原生Java策略检查通过，覆盖真实静态资源、伪造主机、HTTP、端口、用户信息、重复参数和不受支持资源。
- Web相关4文件33项通过，类型检查通过。
- App相关4文件20项通过，静态检查无问题，涵盖输入稳定性、旧响应、窗口返回、内容保留、无需逐条勾选及不发送伪造确认。
- 后端视频、产品关联、协议、草稿与迁移兼容相关回归通过，包括无协议仍拦截、旧审计时间不改写、其他角色不能调用后台协议入口及本地视频转码。
- 本地转码旧用例缺少现有协议前置条件，本次只补齐测试中真实确认协议的步骤，没有移除生产校验或降低审核要求。
- 47版包名 `com.zqzt.dalunongji.editdev`，名称“中企展馆发布测试”，与正式App共存，连接正式域名。签名与独立身份已检查，237项内嵌网页资源哈希通过，新Java策略确认已进入DEX。

**Chrome对照测试不等于安卓手机实播。** 当前尚无已完成的新包手机验证；需要用户安装47测试包复测黑屏、播放、暂停、全屏、返回及后台停止。抖音仍需要点击官方播放按钮，没有承诺自动播放。

官方能力依据：[通过VideoID获取IFrame代码](https://partner.open-douyin.com/docs/resource/zh-CN/dop/develop/openapi/video-management/douyin/iframe-player/get-iframe-by-video)。本次未下载转存外部视频，未使用第三方解析、去水印或绕过平台限制。

## 文件及交付

核心文件：`OfficialVideoPolicy.java`、`OfficialVideoView.kt`、`VideoPlayerActivity.kt`，`mobile/lib/video_links.dart`，`web/src/video-playback.tsx`、`resources.tsx`、`service-center.tsx`，`server/video_links.go`、`video_resources.go`、`service_documents.go`及相应测试。

可执行原生策略测试：`scripts/test-official-video-policy.ps1`。独立包构建：`scripts/build-video-fix-test-apk.ps1`，会校验实际版本，避免直接Gradle构建沿用旧的local.properties版本号。

测试包：`delivery/video-fix-test/zqzhg-video-fix-1.10.9-47.apk`，108,291,307字节；SHA256：`0445f96972ac4f4537428725fbc95f7687f8c0065755acc83fe55349795e2928`。正式45下载不替换，46候选文件保留。网站／服务端上线及地图验收以 `releases/v1.10.9/` 最终回执为准，不在本记录预先宣称上线完成。

原始对照、截图及日志在忽略的 `.local/douyin-fix/`；短链接跳转跟踪参数不提交GitHub。既有全量测试历史失败见1.10.8上线记录，本次不宣称全量测试通过。
