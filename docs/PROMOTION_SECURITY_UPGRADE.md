# 企业推广海报、自定义链接与防采集升级

版本：promotion-security-20261018。配套React/Go更新，Schema14；不改版Android，不部署生产。

## 使用与实际行为

- 厂商工作台 → 展厅推广：查看与企业名称相关的拼音/英文建议，自定义短地址，检查可用性，确认后锁定。仅拥有企业资料的账号可以设置自己的链接，不会自动更改原有链接。
- 管理员在原企业管理中的展厅工具更正地址，需要填写原因。历史地址永久登记，HTML 301到当前地址并保留查询参数；公开API兼容返回当前企业，canonical始终指向当前地址。数字旧链接仍可访问。
- 海报：生成预览后才能主动下载。只使用已公开企业简介/摘要、主营产品、Logo/主图、地区和公开联系方式，最多3款本企业代表产品，尊重手动清空设置。保留真实型号，排除重复和其他企业产品。缺项收紧布局、缺图标明并可重试，中英文回退与原网站一致。
- 采用规则整理，不调用外部AI，不添加虚构认证、产能或营销结论。
- 系统状态页显示公开访问防护开关及本次服务运行累计拦截数。统计不包含IP/搜索内容；重启后内存统计和额度清零。

## API与数据库

新增：
- GET /api/v1/merchant/showroom-link：当前地址、锁定状态、建议、公开域名；可选slug检查，只返回valid/available，不返回占用企业。
- PUT /api/v1/merchant/showroom-link：{slug}。账号只设置自己的企业，首次成功后锁定，大小写统一为小写。重试同一已设置地址幂等。
- GET /api/v1/admin/vendors/:id/showroom-link：复用vendor.write权限。
- 原PUT /api/v1/admin/vendors/:id/showroom-slug兼容扩展{slug,reason}：首次设置维持原契约，更正现有地址时要求原因。保留操作日志。
- GET /robots.txt：禁止API遍历及列明的AI自动采集代理，保留普通页面搜索收录。
- 原GET /api/v1/admin/system兼容增加public_protection（开关、活动IP数和分类拦截计数）。

014_showroom_addresses.sql：新增地址登记表，slug ASCII二进制主键、vendor_id索引；回填原自定义地址，不改企业ID、不删资料。迁移可重试。旧current showroom_slug唯一索引继续保留；历史地址不允许其他企业再次占用。备份恢复支持Schema10—14，并实际回归了当前/历史地址与地区编码恢复。

校验：2—64位小写ASCII字母/数字/单连字符，以字母开头、字母/数字结尾；禁止连续连字符、v-数字和系统保留词。建议不是自动证明商标权，厂商仍需确认企业或品牌关联。

## 防采集配置

单实例有界内存计数，按IP共享网页与API额度，收藏公开资料读取也纳入；不信任UA、Referer或CORS来豁免请求。不同内容额度按返回的真实企业/产品ID去重，包括列表内和代表产品；旧别名不能绕过。媒体单独计数，不缓冲视频/PDF正文，保留Range与HEAD及原媒体权限。

| 环境变量 | 默认 | 单位/说明 |
|---|---:|---|
| PUBLIC_GUARD_ENABLED | true | false可停用防护用于诊断；生产建议保持true |
| PUBLIC_LIST_PER_MINUTE | 30 | 每IP列表请求/分钟 |
| PUBLIC_DETAIL_PER_MINUTE | 90 | 每IP详情请求/分钟 |
| PUBLIC_READS_PER_MINUTE | 120 | 每IP业务读取总数/分钟 |
| PUBLIC_MEDIA_PER_MINUTE | 600 | 每IP媒体请求/分钟，不占业务额度 |
| PUBLIC_VENDORS_PER_HOUR | 300 | 每IP每小时不同企业 |
| PUBLIC_PRODUCTS_PER_HOUR | 1200 | 每IP每小时不同产品 |
| PUBLIC_GUARD_MAX_IPS | 20000 | IP记录上限；定期惰性清理过期记录，满额时新来源短暂429 |
| TRUSTED_PROXIES | 空 | 仅明确配置可信代理IP/CIDR，逗号分隔 |

短时/不同内容超限返回429、Retry-After；10分钟内5次违规后临时限制10分钟，期间请求不延长封禁。前端保留搜索，等待期间禁用重试并阻止重复公开API请求。API错误仍为原{error:{code}}结构。q超过200字、非法页码/页长、异常长查询被400明确拒绝，批量收藏解析仍最多100条。

robots和声明UA封禁仅对合作爬虫有效，不能识别所有伪装。所有代理同受行为额度约束，公开内容仍可被低速或分布式采集；不承诺绝对阻止复制。共享出口可能累计多人请求，应根据访问日志调节阈值。多实例需由统一入口执行聚合限流，不能将各实例内存额度视作全局额度。附带Nginx示例聚合请求频率，不聚合跨副本每小时不同内容数量；应合并到现有TLS站点配置，不能原样替换证书配置。

## 构建、部署与回滚

本地：在web运行npm test、npm run build；根目录运行./scripts/stop.ps1，再./scripts/start.ps1。后端go vet ./...；隔离MySQL测试启用RUN_MYSQL_TESTS=1后go test -v ./...。Windows原生视频用例单独运行以保留完整完成记录，其余用例独立成组运行。

部署：
1. 备份数据库、媒体、.env、旧后端及完整静态目录。实际本地备份位于.local/backups/pre-promotion，私密配置不进交付包；previous-dist来自上一版独立交付包。
2. 在隔离环境验证迁移与配套构建。停止旧服务，替换配套后端、静态目录，保留媒体、环境配置、签名和运营目录。启动时自动执行014，确认schema_migrations最高版本14。
3. 配置PUBLIC_BASE_URL为正式HTTPS域名。反向代理按附带示例覆盖转发头，设置TRUSTED_PROXIES为实际代理IP；限制源站端口只允许代理访问，不信任任意X-Forwarded-For。
4. 检查健康接口、登录、搜索、展厅、旧链接、robots、视频Range、海报二维码和429恢复。监控共享出口误拦情况。

回滚：优先恢复支持Schema14地址登记的已验证兼容构建。地址启用或管理员更正后，直接回滚到只认识showroom_slug的旧后端会丢失历史跳转能力。必要时停止写入并恢复同一时间点数据库、程序和静态资源；不要删除业务记录或释放历史地址。不需要卸载014表，不覆盖已有网站包和APK。

验证结果见同目录PROMOTION_VALIDATION.md及test-output/promotion-browser.json。真机软键盘/系统分享、生产代理/CDN、多实例以及MySQL8/Linux运行环境尚未实际验收，不能标记通过。

## 已有Linux服务的升级命令框架

沿用原部署的/opt/zqzt/releases、/opt/zqzt/current及zqzt-exhibition.service。不要重新运行旧deploy.sh创建新业务库。先把新包在隔离环境整理为与现有release相同结构（server/exhibition、web/dist；共享环境与媒体仍指向原shared目录），并保留current旧目标。

```bash
# 在配套新release目录中，确认环境和媒体路径正确后：
chmod 755 server/exhibition
# 完成备份、停服务、切换current到已验证的新release后：
sudo systemctl restart zqzt-exhibition.service
curl --fail http://127.0.0.1:8089/api/v1/health
```

端口沿用实际HTTP_ADDR；8089来自仓库原部署默认值，不能覆盖实际配置。切换前备份current链接目标，回滚按前文Schema14兼容规则处理；不得使用不含地址登记的旧后端继续接收新地址写入。

海报样张包含隔离验收资料及本机真实公开企业资料。poster-real-company-local.png用于本机排版审阅，其二维码使用当前本地配置地址；对外分享前需在正式环境设置PUBLIC_BASE_URL并重新生成。原上传图片方向、清晰度和企业资料文案由厂商维护，本次不会猜测纠正或删除上传资料。
