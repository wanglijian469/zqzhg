# 厂商专属轻水印（Schema20 / Android 1.8.0-rc.5+34）

## 使用

进入管理后台 → 系统设置 → 图片水印。默认开启，文字为“中企展馆 · 厂商简称”，右下角、15%不透明度、宽度占图片22%，边距约短边2%。可修改平台文字、不透明度（5–40%）、宽度（10–30%）和四角位置。选择厂商可查看其实际简称；未填简称时使用企业全称。预览不保存，点击“保存”后才对后续上传生效。

复用企业资料的“企业简称”，不增加填写项。简称为空时表单提示完善，不阻止上传。中文水印固定在图片像素内，不随界面语言切换。文字细体、无底色、无描边；按落点亮度选深／浅色。先按整体宽度缩小字号；超过18个名称字符时省略尾部，厂商原资料不变。小图不放大水印。PNG保留原透明度。

## 生效范围

- 仅新上传的企业相册、企业封面、产品展示图处理。旧图片及历史审核快照保持原样。
- Logo、证照、平台横幅、分类图、视频、PDF及资源封面不处理。
- 关闭开关、改变样式或厂商名称都不会重做既有图片。上传时保存完整设置快照；归属名称确定时保存名称快照。
- 已有公开企业优先采用当前公开版本的简称／名称，待审名称不会提前生效。新企业在首次保存资料时确定名称。管理员代传按目标资料所属企业取名。
- 未保存归属前，上传底图只向有权账号开放。保存草稿时同步合成独立展示图和缩略图，保存失败不发布未加印图片，可重试。正常审核后才公开。
- 同图用于Logo与封面时分别保留无水印底图引用和水印展示引用，互不覆盖。旧客户端未传用途也按最终资料槽位处理，伪造用途不能绕过。
- 新客户端以保存接口返回的规范化图片ID更新草稿和顺序。旧客户端缓存的上传ID，在没有独立公开用途时只解析到已公开的水印副本；`original=1`不能获取受保护底图。明确公开的Logo等排除用途仍按其用途提供图片。

## 数据和接口

新增 `020_image_watermarks.sql`。`media_watermarks` 仅记录升级后的新上传和衍生图：

| 字段 | 说明 |
|---|---|
| media_id / source_media_id | 当前媒体与受保护底图；底图的source_media_id为0 |
| variant_key | 来源ID与企业ID形成的唯一键，保证重复保存不叠印 |
| purpose / state | 上传用途提示／实际展示用途；底图pending、disabled及衍生ready |
| config_json / config_version | 上传时完整水印配置与系统配置版本 |
| vendor_id / vendor_text | 服务端确定的所属企业与文字快照 |

迁移为增量建表、带IF NOT EXISTS，支持新安装、中断重试及重复运行。不批量新增历史记录，不改写原媒体路径、内容及审核快照。默认配置由现有系统设置加载器补齐；旧请求缺少watermark字段时保留已有设置，并继续执行原版本校验。

| 接口 | 补充 |
|---|---|
| POST /merchant/media、/admin/contents/:id/media | 可选multipart `purpose`：unknown、company_gallery、company_cover、product_image、company_logo、credential、home_display、category_image、resource_cover。未知值返回字段错误。未传为unknown，实际保存槽位决定处理。 |
| GET /admin/settings、PUT /admin/settings | `watermark`对象：enabled、text、opacity、width_percent、position。更新仍使用version和system_settings_saved操作日志。 |
| GET /admin/settings/watermark/vendors?q= | 最多20个企业的id/name/short_name，优先公开版本。仅管理员。 |
| POST /admin/settings/watermark/preview | 请求 `{watermark, vendor_id:0, dark:false}`，返回960×600 PNG。仅管理员、限流、no-store；不写配置或媒体。 |
| GET /public/config | `watermark`及 `capabilities.image_watermark=true`，便于客户端识别服务能力。 |

错误包括 invalid_watermark（400）、watermark_vendor_required（409，先保存企业资料）、watermark_not_ready（409，重新保存后重试）、storage_quota_exceeded（409）和storage_unavailable（503）。底图和全部衍生文件计入厂商配额。生成期间锁定所属账号，复用原配额台账；SQL失败清理本次随机生成的文件，保留底图供重试。存在衍生引用的底图禁止清理。

字体随服务端嵌入未修改的Noto Sans SC Regular，SIL OFL 1.1，见server/fonts/README.md和server/licenses/NotoSansSC-OFL.txt。Windows/Linux不依赖系统字体或联网下载。

## 验证与证据

结果汇总见 `test-output/watermark-validation.json`，原始测试日志、浏览器8个布局及迁移前后摘要在 `test-output/watermark-*`。截图在 `screenshots/watermark/`，其中effect-light.png、effect-dark.png是1600×1000实际像素效果图；sample-*覆盖短名、长名、全称、竖图、小图、透明图和缩略图，使用合成测试素材。

专项覆盖：首次入驻、简称回退、公开名称优先、配置冻结、关闭后再开启、原图哈希不变、Logo／相册混用、管理员归属、错误用途提示、私有访问、旧图片ID解析、查询参数绕过、重复保存、配额不足、失败回滚、重复迁移及Schema20完整备份恢复。历史业务回归继续验证未审核修改不影响公开版本。

网页后台检查360、390、768、1440px及中英文；Flutter覆盖现有大字体和模拟键盘布局，并增加水印衍生ID在保存、封面排序和草稿接续中的检查。Android包名和证书沿用旧版本。Flutter构建包装器的Gradle可写目录检查在本机失败，使用相同SDK、离线Gradle assembleRelease构建成功；结果以最终构建和签名检查为准。

未验证：真机安装／覆盖升级、真机键盘／大字体／下载交互、真实短信供应商发送、iOS及Linux运行。Linux amd64/arm64仅交叉构建验证。浏览器和原生布局测试不能替代这些真机检查。

## 本机更新、备份与回退

本次仅更新本地配套服务，不部署生产，不改正式下载入口。Android测试包为 `android/dalunongji-1.8.0-rc.5+34.apk`。手机App服务地址可在设置中指向电脑的局域网8080端口；默认远端服务尚未部署本次升级。

切换前停止本项目服务，完整备份当前实际数据库、媒体、操作控制目录、server/.env、原可执行文件和web/dist。备份保存在本机 `.local/backups/watermark-before-*`，含SQL和media.zip。备份含私密资料，不放入交付包。迁移前后核对账号、厂家、产品、修订、媒体、收藏数量及归属／审核JSON／媒体路径哈希；迁移不产生任何业务图片或衍生图。站点重启后确认Schema20、健康接口和水印默认配置，保留原网站轮廓地图配置。

部署包包含Windows/Linux后端、网页构建、迁移、目录和许可证；不包含真实媒体、密钥、签名文件、本机地图预览资源或测试APK的正式下载替换。APK单独交付，正式APK哈希保持不变。

回退优先在当前兼容Schema20后端关闭水印开关，再恢复此前兼容前端。新字段、底图、衍生文件和记录必须保留；不要降到不理解Schema20的后端继续写入，也不要删除新表或恢复旧SQL覆盖当前新增资料。若仅需核对备份，先使用后台加密备份恢复到隔离库验证，确认后才切换。原程序备份用于应急核查；有新数据产生后不能直接回滚数据库。未来生产部署须另行安排完整备份和迁移验证，本次未执行。
