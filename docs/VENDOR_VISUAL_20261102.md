# 找厂家 Web / App 视觉统一（本机）

预览：<http://localhost:18087/browse>。使用完整隔离数据库和媒体副本，后台任务与短信关闭；未改线上网站、正式 APK 或 GitHub。

以现有 Web 为准，将 App 的主题蓝从 `#1769D2` 统一为 `#1769E0`；深蓝文字 `#172B45`、浅灰背景 `#F4F7FB`、卡片边框 `#E5EAF1`，新增可复用的浅蓝、弱化文字和卡片操作尺寸常量。导航与收藏沿用主题变量，收藏保持空心/实心星标区分。

Web 只调整公共厂商检索卡片的留白、标题间距和操作对齐，保留蓝色“进入品牌展厅”和图标文字“联系厂家”，不影响首页案例、地图及管理预览。宽屏速览保留，低于1200 CSS像素仍沿用此前隐藏规则。1440px同一检索卡片实测高度从约215.5px降为205px。

App 厂商卡片主按钮和浅蓝次按钮均为44dp，圆角复用主题规范；图片由88dp收至80dp，压缩卡片间隔和操作上方空白。修正原卡片 ButtonStyle.merge 的继承顺序，避免全局48dp尺寸覆盖局部设置。标准字体测试卡片低于190dp；大字体保留原纵向按钮适配，不裁剪公司名称或改变跳转。

## 实际检查

- Web类型检查、构建通过；厂商检索及公共页脚组件22项通过。
- Chrome实际浏览器31项通过：1440、1280、1200、1199、768、390、360px；同数据前后高度、品牌色、透明联系入口、速览断点、无横向溢出、展厅/联系跳转及无页面异常。
- Flutter静态检查无问题；新卡片渲染2项、原有收藏/联系跳转1项及360/390/412宽度、中英文、1/1.4/2倍字体18项回归通过。
- App截图为使用测试资料的真实Flutter组件渲染，不是安卓真机截图；本轮没有安卓设备实测。

## 修改文件

- `web/src/vendor-visual.css`：限定公共厂商列表的视觉样式。
- `web/src/main.tsx`：引入样式。
- `mobile/lib/industrial_ui.dart`：统一现有主题与复用常量。
- `mobile/lib/exhibit_card.dart`：紧凑厂商卡片及主次按钮。
- `mobile/lib/favorites.dart`、`mobile/lib/app_navigation.dart`：复用主题状态色。
- `mobile/test/vendor_visual_test.dart`：44dp、放大字体、收藏及真实渲染验证。

没有新增接口、改变数据结构或调整导航路径。

截图：[Web桌面](screenshots/vendor-visual-20261102/directory-1440.png)、[Web手机](screenshots/vendor-visual-20261102/directory-390.png)、[App卡片](screenshots/vendor-visual-20261102/app-390.png)。同目录保留前后对照和31项浏览器结果；本机完整日志在 `.local/vendor-visual/`。
