# 五栏目展台与多媒体管理 V1.5.0

网站与 Android 展台固定显示企业介绍、展品、短视频、电子图册、联系我们，默认打开展品。企业介绍分为基本信息和生产能力。展品使用现有分类和子分类，分类数量统计全部公开产品，搜索与分页不影响分类总数。

## 厂商操作

在厂商中心的展位管理打开短视频或电子图册管理。产品编辑页也有管理关联视频、图册的入口。已有公司账号仍使用原账号密码登录。

视频填写标题、用途、说明，可关联本企业产品，并可另外上传图片作为封面。产品演示必须关联产品。选择 MP4/MOV 后点击“开始上传”，上传完显示等待处理、正在处理、处理完成或处理失败；处理中也可保存草稿，处理完成后才可提交审核。失败可重试处理或重新上传。

图册选择 PDF 或图片图册。PDF 可设置图片封面，不设置时显示文件图标；图片册可批量上传、上下调整页面顺序、移除页面，默认用第一页作封面。PDF 可在线阅读、缩放和下载原文件；图片册按顺序翻页、缩放。

资料逐条保存、预览、提交、审核、驳回及下架。企业或关联产品尚未公开时，审核通过的资料也不会公开；企业或关联产品下架后，视频、PDF、封面和图册页面同步停止公开访问。修改待审时保留旧公开版。移除或下架不删除被历史审核版本引用的文件。

Android 将选中的文件复制到应用私有目录，按服务器、账号及资料保存本机草稿；退出或重启后可继续上传待重试文件，已有上传成功的媒体 ID 不重复上传。取消上传后从文件开头重试。视频由用户点击播放，退出阅读页面或应用进入后台时暂停。PDF 下载打开系统文件保存选择器。

## 管理员操作

管理后台新增“短视频管理”和“电子图册管理”，支持按名称、审核状态查找，预览资料、通过审核、填写原因驳回、查看审核记录和下架。操作记录进入现有系统日志。

## 视频工具与部署

后端需要 Go **1.26+** 构建。PDF 校验使用 pdfcpu，网页使用 PDF.js，Android 使用 video_player、pdfrx 与文件选择组件。网页构建自动准备 PDF.js 字符映射、标准字体及 WASM 文件；兼容构建和标准接口补充在当前 Edge 118 实际验证。

本机视频处理工具已安装在 `.tools/ffmpeg/bin`。运行 `scripts/setup-video-tools.ps1` 检查，尚未安装时加 `-Download`；脚本从 FFmpeg 官方下载页关联的 Gyan Windows 构建站下载。工具许可证保存在 `.tools/ffmpeg/LICENSE`。本次使用 FFmpeg/ffprobe 9.0.2。FFmpeg 为独立运行工具，不放入 APK；交付源码和部署包附安装说明，不重新分发该工具二进制。

部署机器可安装自有 FFmpeg/ffprobe，并设置 `FFMPEG_PATH`、`FFPROBE_PATH` 为完整路径，或放入系统 PATH；Windows 开发目录还支持上述 `.tools` 路径。Linux 安装带 libx264 和 AAC 支持的 FFmpeg 后使用 PATH。官方源码与许可信息见 https://ffmpeg.org/download.html、https://ffmpeg.org/legal.html；Windows 构建信息见 https://www.gyan.dev/ffmpeg/builds/ 。

上传文件、转换结果和封面均在 `MEDIA_DIR`。处理任务持久化到数据库，本版一个服务进程、一个处理任务；重启将处理中任务恢复为排队。转码先写独立临时文件，完成后再发布最终文件，待完成前无法读取。备份应停止上传并等待处理完成，再执行既有数据库和媒体备份脚本；恢复数据库和整个媒体目录后启动服务，由迁移补齐新表、队列恢复未完成任务。

可配置默认限制：

| 配置 | 默认值 |
|---|---:|
| MAX_VENDOR_VIDEOS | 20 |
| MAX_PRODUCT_VIDEOS | 3 |
| MAX_VENDOR_CATALOGS | 10 |
| MAX_VIDEO_SECONDS / MAX_VIDEO_MIB | 180 / 200 |
| VIDEO_MAX_EDGE | 1280 |
| MAX_PDF_MIB / MAX_PDF_PAGES | 50 / 200 |
| MAX_CATALOG_PAGES | 30 |

数量包含草稿和下架资料。图片沿用每张 10 MiB 的限制。MP4/MOV 经检测后转换为 H.264/AAC MP4；静音视频允许发布，不放大低分辨率，保持画面比例。PDF 拒绝损坏、加密和超限文件。

## 迁移与验证

新增 `005_showroom_resources.sql`，仅创建资源、版本、审核、媒体引用和媒体元数据表，不修改旧企业、产品或图片记录。使用可重复执行的建表语句，支持中断后恢复。上线前本机备份位于 `.local/backups/pre-resources-v150`。

新接口包括 `/merchant/resources`、`/admin/resources`、`/admin/resource-revisions/:id/review`、`/merchant/resource-media` 和公开资源列表/详情。二进制文件统一由 `/api/v1/media/:id` 校验公开权限或请求头身份，支持 Range/HEAD，不将令牌放入链接。完整契约见 `openapi.json`。

测试证据在 `test-output/resources-*`，截图在 `screenshots/resources-*`。业务变更和实际浏览器上传审核仅使用独立测试库；正式冀农资料只读核对。真机安装、文件选择与原生 PDF/视频播放以实际连接 Android 设备验证为准，无设备时记为待验证。iOS 继续显示即将上线。
