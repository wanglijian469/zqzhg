# 头条搜索网站验证（2026-11-14）

用户提供的 `ByteDanceVerify.html` 已原样部署，文件为20字节。公网验证地址：`https://www.zqzhg.com/ByteDanceVerify.html`。

- 持久文件位于服务器 `/opt/zqzt/shared/site-verification/ByteDanceVerify.html`，不依赖当前应用发布目录；不要在升级时删除。
- `deploy/linux/zqzhg-https.conf` 为HTTP、裸域名HTTPS及www HTTPS增加精确匹配的验证文件路径。www其他路径继续301跳转主站，路径与查询参数保留。
- `web/public/ByteDanceVerify.html` 保存同一原始文件，供之后前端构建携带。复用Nginx模板前，应确保共享目录内已有文件。
- 已先备份线上Nginx配置，执行 `nginx -t` 并平滑重载。从服务器和本机公网分别验证四个入口均为200、无跳转且逐字节一致；头条爬虫User-Agent检查通过。未验证头条实际爬虫来源IP。
- 首页、健康接口、站点地图均200；其他入口原有跳转检查通过。站点地图当前为urlset，共90个地址。
- 此次只增加验证静态文件及精确路由，未打部署包或升级应用。当前应用发布目录保持 `v1.10.11-67f9f62`。
- 浏览器控制运行时因沙箱启动失败不可用，尚未点击头条后台的“验证”，也未完成平台链接提交。需用户在原页面点击验证后继续。

公开检查记录：`docs/test-output/toutiao-verification-20261114.json`。
