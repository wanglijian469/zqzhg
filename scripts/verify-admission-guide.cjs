// Explicitly invoked by TestAdmissionGuideBrowser against a disposable local database.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
const fixture=JSON.parse(fs.readFileSync('.local/guide/fixture.json','utf8'));
const base=fixture.base;assert.match(base,/^http:\/\/127\.0\.0\.1:\d+$/);
const capture=process.env.GUIDE_CAPTURE==='1',qa=process.env.GUIDE_QA==='1';
const assets='web/public/guide-images',evidence='docs/screenshots/admission-guide';
fs.mkdirSync(assets,{recursive:true});fs.mkdirSync(evidence,{recursive:true});
const receipt={isolatedDatabase:true,productionDeployed:false,realSmsSends:0,checks:[],screenshots:[],layouts:[],errors:[]};
(async()=>{
 const browser=await chromium.launch({headless:true,executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',args:['--no-proxy-server']});
 let page;
 try{
 const ctx=await browser.newContext({viewport:{width:1100,height:1000},deviceScaleFactor:1});page=await ctx.newPage();page.setDefaultTimeout(12000);
 page.on('pageerror',e=>receipt.errors.push(e.message));
 const api=async(p,token='',data,method=data?'POST':'GET')=>{const r=await page.request.fetch(base+'/api/v1'+p,{method,headers:token?{Authorization:'Bearer '+token}:{},data});assert(r.ok(),p+': '+r.status()+' '+await r.text());return r.json()};
 const shot=async(name,locator)=>{if(!capture)return;await page.evaluate(()=>document.fonts.ready);await (locator||page).screenshot({path:assets+'/'+name+'.png',animations:'disabled'});receipt.screenshots.push(name)};
 const goto=async(p)=>{await page.goto(base+p,{waitUntil:'domcontentloaded'});};
 const button=name=>page.getByRole('button',{name,exact:true});const input=name=>page.getByLabel(name,{exact:true});
 console.log('Guide: verifying real registration and showroom application.');
 await goto('/join');await page.getByRole('tab',{name:'手机验证码注册',exact:true}).waitFor();
 await shot('register-phone',page.locator('.merchant-entry-card'));
 await page.getByRole('tab',{name:'账号密码注册',exact:true}).click();await input('用户名').fill('guide_sample_2026');await input('设置密码').fill('Guide!2026');await input('确认密码').fill('Guide!2026');await page.getByRole('checkbox').check();
 await shot('register-account',page.locator('.merchant-entry-card'));
 await button('注册并进入展馆').click();await page.getByRole('heading',{name:'欢迎来到中企展馆',exact:true}).waitFor();
 const visitor=await page.evaluate(()=>sessionStorage.getItem('zqzt.merchant'));assert.equal((await api('/auth/me',visitor)).role,'visitor');
 await button('我是厂家，免费入驻').click();await button('确认申请免费企业展厅').waitFor();await shot('company-apply',page.locator('main>.container.section'));
 await button('确认申请免费企业展厅').click();await input('企业 / 工厂名称').waitFor();
 const token=await page.evaluate(()=>sessionStorage.getItem('zqzt.merchant'));assert.equal((await api('/auth/me',token)).role,'merchant');
 await page.setViewportSize({width:760,height:1000});
 await input('企业 / 工厂名称').fill('示例机械制造有限公司');await input('主营产品').fill('翻转犁、旋耕机及配件');await input('微信').fill('example_factory');
 await input('省／直辖市／自治区').selectOption('13');await input('市／州／盟').selectOption('1305');await input('区／县（选填）').selectOption('130528');
 const vendorCategories=(await api('/merchant/taxonomy?category_kind=vendor',token)).categories;const primary=vendorCategories.find(c=>c.enabled&&c.category_kind==='vendor'&&(!c.hall_id||c.hall_id===1));assert(primary);await input('主营分类').selectOption(String(primary.id));
 await button('保存云端草稿').click();await page.getByText('企业资料已保存到云端草稿',{exact:true}).waitFor();await page.evaluate(()=>scrollTo(0,0));await shot('company-basics');
 let vendor=(await api('/merchant/contents?kind=vendor&page_size=1',token)).items[0];assert.equal(vendor.data.name,'示例机械制造有限公司');
 await goto('/merchant/promotion');await page.getByText('资料公开后可下载二维码和宣传海报。',{exact:true}).waitFor();assert.equal(await button('展厅二维码').count(),0);receipt.checks.push('registration → visitor → free company application → merchant; draft saved; unpublished sharing unavailable');
 console.log('Guide: verifying product draft, agreements, review and public search.');
 await goto('/merchant/products');await page.getByRole('link',{name:'添加产品',exact:true}).first().click();await input('产品名称').fill('液压翻转犁（示例产品）');
 const cats=(await api('/merchant/taxonomy?category_kind=product',token)).categories;const leaf=cats.find(c=>c.enabled&&c.category_kind==='product'&&(!c.hall_id||c.hall_id===1)&&!cats.some(x=>x.parent_id===c.id));assert(leaf,'product leaf category');
 await input('产品分类').selectOption(String(leaf.id));await button('保存云端草稿').click();await page.getByText('已保存到云端草稿',{exact:true}).waitFor();await page.evaluate(()=>scrollTo(0,0));await shot('product-basics');
 await button('下一步').click();await page.getByRole('heading',{name:'产品图片（选填）',exact:true}).waitFor();await button('下一步').click();await button('提交审核').waitFor();await shot('product-submit');
 // Company agreements are required before the first product can be submitted.
 await goto('/merchant/booth/edit?step=2');await page.locator('.publication-agreements').waitFor();await page.locator('.publication-agreements').scrollIntoViewIfNeeded();await shot('company-agreements',page.locator('.publication-agreements'));await page.locator('.publication-agreements input[type=checkbox]').check();await button('确认协议').click();await page.getByText('已确认当前版本协议',{exact:true}).waitFor();await button('提交审核').scrollIntoViewIfNeeded();await shot('company-submit');
 const companySubmission=page.waitForResponse(r=>r.url().endsWith('/submit')&&r.request().method()==='POST');await button('提交审核').click();const companyResponse=await companySubmission;assert(companyResponse.ok(),await companyResponse.text());await page.waitForURL(/\/merchant\/booth(?:\?|$)/);
 vendor=(await api('/merchant/contents?kind=vendor&page_size=1',token)).items[0];assert.equal(vendor.status,'pending');
 const product=(await api('/merchant/contents?kind=product&page_size=1',token)).items[0];await goto('/merchant/products/'+product.id+'/edit');await button('下一步').click();await button('下一步').click();await button('提交审核').click();await page.waitForURL(/\/merchant\/products$/);
 const submitted=await api('/merchant/contents/'+product.id,token);assert.equal(submitted.status,'pending');
 assert.equal((await page.request.get(base+'/v/v-'+vendor.id)).status(),404);
 await api('/admin/revisions/'+vendor.revision_id+'/review',fixture.admin_token,{decision:'approve'});await api('/admin/revisions/'+submitted.revision_id+'/review',fixture.admin_token,{decision:'approve'});
 const published=await api('/public/showrooms/v-'+vendor.id);assert.equal(published.item.data.name,vendor.data.name);
 const found=await api('/public/vendors?q='+encodeURIComponent(vendor.data.name));assert.equal(found.total,1);
 const products=await api('/public/products?q='+encodeURIComponent('液压翻转犁'));assert.equal(products.total,1);
 receipt.checks.push('company and product saved as private drafts, submitted separately, approved and discoverable through real public searches');
 console.log('Guide: verifying real showroom QR code and poster.');
 await goto('/merchant/promotion');await button('展厅二维码').click();await page.locator('.share-qr').waitFor();assert((await page.locator('.share-qr').getAttribute('src')).startsWith('data:image/png;base64,'));await shot('showroom-share',page.locator('.compact-share'));
 await button('生成海报预览').click();await page.locator('.poster-preview img').waitFor();assert.equal(await button('复制地址').count(),1);const downloadPromise=page.waitForEvent('download');await button('保存宣传海报').click();const download=await downloadPromise;assert(download.suggestedFilename().endsWith('-poster.png'));
 receipt.checks.push('published showroom sharing, real QR generation, poster preview and image download');
 // Existing public pages and login remain reachable; no production data is used.
 const publicPaths=['/','/browse?q='+encodeURIComponent(vendor.data.name),'/browse/products?q='+encodeURIComponent('液压翻转犁'),'/v/v-'+vendor.id,'/products/'+product.id,'/merchant/login','/join','/regions','/services','/service-requests'];
 for(const p of publicPaths){await goto(p);const showroom=/^\/(?:v|vendors)\//.test(p);await page.locator(showroom?'.showroom-footer':'footer').waitFor();assert.equal(await page.getByRole('link',{name:'入驻使用指南',exact:true}).count(),showroom?0:1,p+' footer');assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false,p+' overflow')}
 receipt.checks.push('guide footer retained on platform pages; independent showroom has compact attribution without platform service links');
 // Verify the SMS route only with the development provider of the disposable fixture.
 const phoneCtx=await browser.newContext({viewport:{width:390,height:844}}),phonePage=await phoneCtx.newPage();await phonePage.goto(base+'/join');await phonePage.getByLabel('手机号',{exact:true}).fill('13900007789');
 const send=phonePage.waitForResponse(r=>r.url().endsWith('/auth/phone/code')&&r.request().method()==='POST');await phonePage.getByRole('button',{name:'获取验证码',exact:true}).click();const sent=await(await send).json();assert.match(sent.development_code,/^\d{6}$/);await phonePage.getByLabel('短信验证码',{exact:true}).fill(sent.development_code);await phonePage.getByRole('button',{name:'下一步',exact:true}).click();await phonePage.getByLabel('设置密码',{exact:true}).fill('Guide!2026');await phonePage.getByLabel('确认密码',{exact:true}).fill('Guide!2026');await phonePage.getByRole('checkbox').check();await phonePage.getByRole('button',{name:'注册并进入展馆',exact:true}).click();await phonePage.getByRole('heading',{name:'欢迎来到中企展馆',exact:true}).waitFor();await phoneCtx.close();receipt.checks.push('mobile verification → password → agreements → ordinary account registration using development SMS only');
 if(qa){
 console.log('Guide: validating guide assets, navigation and responsive layouts.');
 for(const name of ['register-account','register-phone','company-apply','company-basics','product-basics','company-agreements','company-submit','product-submit','showroom-share']){const r=await page.request.get(base+'/guide-images/'+name+'.png');assert.equal(r.status(),200,name);assert(Number(r.headers()['content-length'])>1000)}
 const guideCtx=await browser.newContext({viewport:{width:1440,height:1000}}),g=await guideCtx.newPage();page=g;g.on('pageerror',e=>receipt.errors.push(e.message));
 for(const [width,scale] of [[1440,1],[768,1],[390,1],[360,1],[390,1.25],[1440,1.25]]){
  await g.setViewportSize({width:Math.floor(width/scale),height:900});await g.goto(base+'/guide?hall=cable',{waitUntil:'networkidle'});await g.getByRole('heading',{name:'厂商入驻与使用指南',exact:true}).waitFor();
  if(scale!==1)await g.evaluate(()=>{document.documentElement.style.fontSize='125%'});
  assert.equal(await g.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false,'guide width '+width+' scale '+scale);
  assert.equal(await g.locator('.guide-chapter').count(),10);assert.equal(await g.locator('.guide-faq details[open]').count(),0);
  const controls=await g.locator('.guide-actions .button,.guide-five a').evaluateAll(es=>es.map(e=>e.getBoundingClientRect().height));assert(controls.every(h=>h>=44));
  for(const id of ['register','company','products','review','find','share','contact','fees','faq','help']){await g.locator('.guide-directory a[href="#'+id+'"]').click();await g.waitForTimeout(80);const top=await g.locator('#'+id).evaluate(e=>e.getBoundingClientRect().top);assert(top>=70&&top<=150,id+' visible at '+top)}
  await g.goto(base+'/guide#share');await g.waitForFunction(()=>{const e=document.getElementById('share');return e&&e.getBoundingClientRect().top>=70&&e.getBoundingClientRect().top<160});await g.reload();await g.waitForFunction(()=>{const e=document.getElementById('share');return e&&e.getBoundingClientRect().top>=70&&e.getBoundingClientRect().top<160});
  await g.goto(base+'/guide');await g.getByRole('heading',{name:'厂商入驻与使用指南',exact:true}).waitFor();await g.screenshot({path:evidence+'/guide-'+width+'-'+scale+'.png',animations:'disabled'});
  receipt.layouts.push({width,scale,noOverflow:true,chapters:10,directoryTargets:10,directHash:true,reloadedHash:true});
 }
 const links=await g.locator('.admission-guide a[href^="/"]').evaluateAll(es=>[...new Set(es.map(e=>e.getAttribute('href')))]);receipt.linkStatuses=[];for(const href of links){const response=await g.request.get(base+href);assert.equal(response.status(),200,href);receipt.linkStatuses.push({href,status:response.status()})}
 const figure=g.getByRole('button',{name:'放大查看：选择账号密码注册，填写后注册'});await figure.click();await g.getByRole('dialog',{name:'选择账号密码注册，填写后注册'}).waitFor();await g.keyboard.press('Escape');assert.equal(await g.getByRole('dialog').count(),0);assert(await figure.evaluate(e=>e===document.activeElement));
 await g.locator('#faq summary').first().click();assert.equal(await g.locator('#faq details[open]').count(),1);
 for(const [name,pathname] of [['立即免费入驻','/join'],['登录','/merchant/login'],['投诉与服务申请','/service-requests']]){await g.goto(base+'/guide');await g.locator('.guide-hero').getByRole('link',{name,exact:true}).click();assert.equal(new URL(g.url()).pathname,pathname)}
 await g.route('**/guide-images/register-account.png',r=>r.abort());await g.goto(base+'/guide#register');await g.getByText('这张操作图暂时无法显示，照着上面的文字也能完成。',{exact:true}).waitFor();assert(await g.getByText('用用户名和密码注册',{exact:true}).isVisible());await g.unroute('**/guide-images/register-account.png');await g.getByRole('button',{name:'重新加载图片',exact:true}).click();await g.getByRole('img',{name:'选择账号密码注册，填写后注册，示例资料'}).waitFor();
 const config=await api('/public/config');await g.route('**/api/v1/public/config',r=>r.fulfill({status:200,contentType:'application/json',body:JSON.stringify({...config,capabilities:{...config.capabilities,phone_sms_register:false}})}));await g.route('**/guide',async r=>{const response=await r.fetch();await r.fulfill({response,body:(await response.text()).replaceAll('"phone_sms_register":true','"phone_sms_register":false')})});await g.goto(base+'/guide');await g.waitForTimeout(300);assert.equal(await g.getByRole('heading',{name:'也可以用手机验证码注册'}).count(),0);await guideCtx.close();
 receipt.checks.push('guide screenshot assets, 6 layout states, 60 directory jumps, direct/reloaded anchors, FAQ, image zoom/focus, fallback/retry and CTA navigation');
 }
 assert.deepEqual(receipt.errors,[]);receipt.passed=true;
 }catch(e){receipt.failure=e.stack;if(page&&!page.isClosed())await page.screenshot({path:evidence+'/failure.png',fullPage:true});throw e}
 finally{await browser.close();fs.mkdirSync('docs/test-output',{recursive:true});fs.writeFileSync('docs/test-output/admission-guide-'+(qa?'acceptance':'capture')+'.json',JSON.stringify(receipt,null,2))}
 console.log(JSON.stringify(receipt,null,2));
})().catch(e=>{console.error(e.stack);process.exitCode=1});
