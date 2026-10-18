# 确认弹窗可读性修复（2026-10-18）

- 根因：通用 .danger 的暗红文字（!important）覆盖了红色填充按钮的白色文字。
- 修复：填充危险按钮使用白色文字与一致的红色边框；离开编辑采用蓝色操作按钮，保留其他危险操作原有语义。
- 企业与产品离开提示改为明确标题、未同步说明、待上传图片说明，以及“继续编辑 / 离开页面”。中英文同步，取消和 Esc 保留输入。
- 确认框最大宽度480px，手机自适应，按钮至少44px，支持焦点和键盘操作；其他媒体弹窗不改变尺寸。

修改文件：web/src/main.tsx、industrial.css、ui-messages.ts、company-editor.tsx、company-editor-messages.ts、product-editor.tsx、product-messages.ts、company-editor.test.tsx、category-picker.test.tsx；新增 scripts/verify-confirmation.cjs。

验证：前端128项测试（22个文件）通过；npm run build（含TypeScript）通过，保留既有大分包提示。隔离MySQL浏览器夹具通过；12组浏览器验收（1440/390/360px、中英文、100%及等效125%）通过，无控制台错误或横向溢出，正常按钮文字对比度5.47:1。取消、Esc、确认跳转均通过。报告：docs/test-output/confirmation-browser.json；截图：docs/screenshots/confirmation/。

本地服务已更新，http://localhost:8080。无API、数据库、Android或生产变更；此前交付包保留。本次实际真机与系统软键盘未验证。
