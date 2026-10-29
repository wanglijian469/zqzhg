// Disposable local MySQL fixture only. Never sends SMS or touches cloud accounts.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
const fixture=JSON.parse(fs.readFileSync('.local/sms/service104-browser.json','utf8'));
assert.equal(fixture.base,'http://127.0.0.1:18094');
const dir='docs/screenshots/service-1.10.4';fs.mkdirSync(dir,{recursive:true});
const receipt={origin:fixture.base,viewports:[],errors:[],realSmsSends:0};
(async()=>{
 const browser=await chromium.launch({executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',headless:true,args:['--no-proxy-server']});
 try{
  const ctx=await browser.newContext({viewport:{width:1440,height:1000}});ctx.setDefaultTimeout(25000);
  await ctx.addInitScript(f=>{sessionStorage.setItem('zqzt.admin',f.admin_token);sessionStorage.setItem('zqzt.merchant',f.merchant_token);sessionStorage.setItem('zqzt.merchant.id',String(f.owner));sessionStorage.setItem('zqzt.merchant.role','merchant');},fixture);
  const page=await ctx.newPage();page.on('pageerror',e=>receipt.errors.push(e.message));
  await page.goto(fixture.base+'/admin/dashboard');await page.locator('.admin-stats').waitFor();
  assert.equal(await page.locator('.admin-navigation a[href="/admin/verifications"]').count(),0);
  await page.getByRole('cell',{name:'密码登录',exact:true}).waitFor();
  const contrast=await page.locator('.admin-navigation a').evaluateAll(links=>{
   const lum=c=>{const a=c.match(/[\d.]+/g).slice(0,3).map(Number).map(v=>{v/=255;return v<=.04045?v/12.92:((v+.055)/1.055)**2.4});return .2126*a[0]+.7152*a[1]+.0722*a[2]};
   return links.map(e=>{const s=getComputedStyle(e);let p=e,bg=s.backgroundColor;while(bg==='rgba(0, 0, 0, 0)'&&p.parentElement){p=p.parentElement;bg=getComputedStyle(p).backgroundColor}const [a,b]=[lum(s.color),lum(bg)].sort((a,b)=>b-a);return {label:e.textContent,ratio:(a+.05)/(b+.05)}});
  });assert.ok(contrast.every(v=>v.ratio>=4.5),JSON.stringify(contrast));receipt.sidebarContrast=contrast;
  await page.screenshot({path:path.join(dir,'admin-dashboard.png'),fullPage:true});
  await page.getByRole('button',{name:'收起导航',exact:true}).click();assert.ok(await page.locator('.admin-app.collapsed').count());
  await page.getByRole('button',{name:'展开导航',exact:true}).click();
  for(const route of ['service-requests','service-documents']){await page.goto(fixture.base+'/admin/'+route);await page.locator('.admin-page-heading h1').waitFor();await page.screenshot({path:path.join(dir,'admin-'+route+'.png'),fullPage:true});}
  for(const [width,height]of [[1440,1000],[768,1024],[390,844]]){
   await page.setViewportSize({width,height});
   for(const route of ['dashboard','service-requests','service-documents']){
    await page.goto(fixture.base+'/admin/'+route);await page.locator('.admin-page-heading h1').waitFor();
    assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),route+' overflow');
   }
   if(width<1024){await page.getByRole('button',{name:'打开导航',exact:true}).click();await page.locator('.drawer-open').waitFor();await page.getByRole('button',{name:'关闭',exact:true}).last().click();}
   receipt.viewports.push({width,height});await page.screenshot({path:path.join(dir,`admin-${width}.png`),fullPage:true});
  }
  await page.setViewportSize({width:1440,height:1000});await page.goto(fixture.base+'/merchant/verification');await page.waitForURL(/\/merchant\/booth\/edit\?step=2/);
  await page.locator('.publication-agreements').waitFor();
  for(const label of ['法定代表人','经办人','营业执照','经办授权证明'])assert.equal(await page.getByLabel(label,{exact:true}).count(),0);
  const accepted=page.locator('.publication-agreements');await accepted.getByRole('checkbox').check();await accepted.getByRole('button',{name:'确认协议',exact:true}).click();await accepted.getByText('已确认当前版本协议',{exact:true}).waitFor();
  await page.screenshot({path:path.join(dir,'company-agreements.png'),fullPage:true});
  const submitted=page.waitForResponse(r=>r.url().endsWith('/submit')&&r.request().method()==='POST');await page.getByRole('button',{name:'提交审核',exact:true}).click();assert.equal((await submitted).status(),200);receipt.companySubmittedWithoutIdentityMaterials=true;
  await page.goto(fixture.base+'/admin/vendor-reviews');await page.locator('.admin-page-heading h1').waitFor();await page.screenshot({path:path.join(dir,'company-review.png'),fullPage:true});
  await page.getByRole('button',{name:'English',exact:true}).click();await page.screenshot({path:path.join(dir,'admin-english.png'),fullPage:true});receipt.english=true;
  assert.deepEqual(receipt.errors,[]);receipt.passed=true;
 }finally{await browser.close();fs.writeFileSync('docs/test-output/service-1.10.4-browser.json',JSON.stringify(receipt,null,2));}
 console.log(JSON.stringify(receipt,null,2));
})().catch(e=>{console.error(e.stack||e);process.exitCode=1});
