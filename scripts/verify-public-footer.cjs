// Read-only local acceptance. No account, complaint, content or SMS submissions.
const fs=require('node:fs'),assert=require('node:assert/strict');
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
const base='http://localhost:8080',dir='docs/screenshots/public-footer-compact';fs.mkdirSync(dir,{recursive:true});
const entries=[['免费入驻','/join'],['入驻使用指南','/guide'],['入驻与服务说明','/services'],['投诉与服务申请','/service-requests'],['下载 App','/app'],['企业登录','/merchant/login']];
(async()=>{
 const json=async path=>{const r=await fetch(base+'/api/v1'+path);assert.equal(r.status,200,path);return r.json()};
 const [vendors,products,config]=await Promise.all([json('/public/vendors?page_size=1'),json('/public/products?page_size=1'),json('/public/config')]);
 const browser=await chromium.launch({headless:true,executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',args:['--no-proxy-server']});
 const receipt={base,productionDeployed:false,contentWrites:false,layouts:[],routes:[],entries:[],specialPages:[],errors:[],unexpectedWrites:[]};let p;
 try{
  const ctx=await browser.newContext({viewport:{width:1440,height:900}});p=await ctx.newPage();p.setDefaultTimeout(12000);p.on('pageerror',e=>receipt.errors.push(e.message));p.on('request',r=>{if(r.url().startsWith(base+'/api/')&&!['GET','HEAD'].includes(r.method()))receipt.unexpectedWrites.push({path:new URL(r.url()).pathname,method:r.method()})});
  let lastListPage=0;
  const open=async path=>{
   if(['/','/browse','/browse/products','/regions'].includes(path)){const pause=6000-(Date.now()-lastListPage);if(pause>0)await p.waitForTimeout(pause);lastListPage=Date.now()}
   let response=await p.goto(base+path,{waitUntil:'domcontentloaded'});
   if(response?.status()===429){let remaining=(Number(response.headers()['retry-after'])||60)*1000+1000;console.log('Respecting the existing public access retry interval.');while(remaining>0){const pause=Math.min(30000,remaining);await p.waitForTimeout(pause);remaining-=pause}response=await p.goto(base+path,{waitUntil:'domcontentloaded'});lastListPage=Date.now()}
   assert(response&&response.status()===200,path+' HTTP '+response?.status());
  };
  const bottom=async()=>{const footer=p.locator('footer.public-footer');await footer.waitFor();for(let i=0;i<6;i++){await p.evaluate(()=>scrollTo(0,document.documentElement.scrollHeight));await p.waitForTimeout(100);const b=await footer.boundingBox();if(b.y+b.height<=900.5)break}return footer};
  for(const [width,zoom] of [[1440,1],[1280,1],[1024,1],[768,1],[390,1],[360,1],[320,1],[390,1.25],[1440,1.25]]){
   await p.setViewportSize({width:Math.floor(width/zoom),height:900});if(receipt.layouts.length===0)await open('/');const footer=await bottom();const columns=footer.locator('.public-footer-columns'),groups=footer.locator('.public-footer-group');
   assert.equal(await p.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false,'overflow '+width+'/'+zoom);
   const grid=await columns.evaluate(e=>getComputedStyle(e).gridTemplateColumns.split(' ').length),expected=Math.floor(width/zoom)>=768?3:Math.floor(width/zoom)>=360?2:1;assert.equal(grid,expected,'columns '+width+'/'+zoom);
   const main=await p.locator('.header-inner.container').boundingBox(),body=await columns.boundingBox();assert(Math.abs(main.x-body.x)<1&&Math.abs(main.width-body.width)<1,'aligned shared container');
   assert.equal(await footer.evaluate(e=>getComputedStyle(e).backgroundColor),'rgb(8, 26, 49)');
   for(const [label,href] of entries){const link=footer.getByRole('link',{name:label,exact:true});assert.equal(await link.count(),1);assert.equal(await link.getAttribute('href'),href);assert(await link.isVisible());if(Math.floor(width/zoom)<768)assert((await link.boundingBox()).height>=44,'touch area '+label)}
   assert.equal(await groups.count(),2);for(const group of await groups.all()){const rows=await group.locator('li>a').evaluateAll(es=>es.map(e=>{const r=e.getBoundingClientRect();return {x:r.x,y:r.y,font:getComputedStyle(e).fontSize}}));assert.equal(rows.length,3);assert(rows.every(r=>r.font==='15px'));assert.equal(await group.locator('ul').evaluate(e=>getComputedStyle(e).display),'flex');assert.equal(await group.locator('ul').evaluate(e=>getComputedStyle(e).flexWrap),'wrap');if(Math.floor(width/zoom)>=1024){assert(rows.every(r=>Math.abs(r.y-rows[0].y)<1),'desktop horizontal links');assert(rows[0].x<rows[1].x&&rows[1].x<rows[2].x)}}
   const admin=footer.getByRole('link',{name:'管理后台',exact:true});assert(await admin.isVisible());assert.equal(await admin.getAttribute('href'),'/admin');assert.equal(await groups.locator('a[href="/admin"]').count(),0);if(Math.floor(width/zoom)<768)assert((await admin.boundingBox()).height>=44);
   assert.equal(await footer.getByRole('link',{name:config.site.icp_number||'冀ICP备2026047360号-2',exact:true}).getAttribute('href'),'https://beian.miit.gov.cn/');
   for(const [label,href] of [['用户协议','/services/terms'],['隐私政策','/services/privacy'],['企业展厅服务协议','/services/showroom-agreement']])assert.equal(await footer.getByRole('link',{name:label,exact:true}).getAttribute('href'),href);
   const text=await footer.innerText();assert(text.includes('河北企元数智科技有限公司'));assert(text.includes('版权所有'));assert(!text.includes('技术支持'));assert(!text.includes('公开展示与联系平台'));assert(!text.includes('内容审核不代表企业资质认证'));
   const size=await footer.boundingBox();if(Math.floor(width/zoom)>=768)assert(size.height>0&&size.height<=200,'desktop height '+size.height);const statement=footer.locator('.public-footer-statement');assert.equal(await statement.count(),0);assert.equal(await footer.locator('.public-footer-bottom .public-footer-registration').count(),1);await footer.screenshot({path:dir+'/footer-'+width+'-'+zoom+'.png',animations:'disabled'});receipt.layouts.push({width,equivalentZoom:zoom,cssWidth:Math.floor(width/zoom),columns:grid,height:size.height,aligned:true,noOverflow:true,mobileTouchAreas:true});
  }
  console.log('Public footer: 9 layout states passed.');
  for(const width of [1440,390]){
   await p.setViewportSize({width,height:900});await open('/');let footer=await bottom();
   for(const name of ['免费入驻','入驻使用指南']){const link=footer.getByRole('link',{name,exact:true});await link.hover();assert((await link.evaluate(e=>getComputedStyle(e).textDecorationLine)).includes('underline'),'hover feedback '+name)}
   const admin=footer.getByRole('link',{name:'管理后台',exact:true});await admin.focus();await p.keyboard.press('Shift+Tab');await p.keyboard.press('Tab');assert(await admin.evaluate(e=>e===document.activeElement&&e.matches(':focus-visible')&&getComputedStyle(e).outlineStyle==='solid'));await p.screenshot({path:dir+'/admin-focus-'+width+'.png'});await p.keyboard.press('Enter');await p.locator('.admin-login input[type="password"]').waitFor();assert.equal(new URL(p.url()).pathname,'/admin');receipt.entries.push({width,path:'/admin',keyboardEntry:true,loginRequired:true});
   for(const [label,href] of entries){await open('/');footer=await bottom();await footer.getByRole('link',{name:label,exact:true}).click();await p.waitForURL(u=>u.pathname===href);await p.locator('main h1').first().waitFor();receipt.entries.push({width,label,path:href,clicked:true})}
  }
  const denied=await p.request.get(base+'/api/v1/admin/contents');assert([401,403].includes(denied.status()));receipt.adminUnauthenticatedStatus=denied.status();
  const ordinary=['/','/browse','/browse/products','/guide','/services','/service-requests','/services/terms','/services/privacy','/services/showroom-agreement',...(products.items[0]?['/products/'+products.items[0].id]:[])];
  for(const path of ordinary){await open(path);await p.locator('footer.public-footer').waitFor();assert.equal(await p.locator('footer.public-footer').count(),1);assert.equal(await p.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false,path);receipt.routes.push({path,sharedFooter:true,noOverflow:true})}
  for(const width of [1440,390])for(const [path,selector] of [['/regions','.factory-map-footer'],['/join','.merchant-entry-footer'],['/merchant/login','.merchant-entry-footer']]){await p.setViewportSize({width,height:900});await open(path);const footer=p.locator(selector);await footer.waitFor();assert.equal(await p.locator('footer.public-footer').count(),0);for(const name of ['入驻使用指南','入驻与服务说明','投诉与服务申请'])assert.equal(await footer.getByRole('link',{name,exact:true}).count(),1);assert.equal(await p.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false);receipt.specialPages.push({width,path,compactFooter:true})}
  for(const width of [1440,390]){await p.setViewportSize({width,height:900});const v=vendors.items[0];assert(v);await open('/v/'+(v.showroom_slug||'v-'+v.id));const footer=p.locator('.showroom-footer-simple');await footer.waitFor();assert.equal(await p.locator('footer.public-footer').count(),0);assert((await footer.innerText()).includes('由中企展馆提供展示服务'));assert.equal(await footer.getByRole('link',{name:'入驻使用指南',exact:true}).count(),0);assert.equal(await footer.getByRole('button',{name:'分享展厅',exact:true}).count(),1);receipt.specialPages.push({width,path:new URL(p.url()).pathname,showroomFooter:true})}
  await p.setViewportSize({width:390,height:900});await p.evaluate(()=>localStorage.setItem('zqzt.language','en'));await open('/');const english=await bottom();await english.getByRole('navigation',{name:'Manufacturer services'}).waitFor();assert.equal(await english.getByRole('link',{name:'Administration',exact:true}).getAttribute('href'),'/admin');assert.equal(await p.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false);receipt.englishNavigation=true;
  assert.deepEqual(receipt.errors,[]);assert.deepEqual(receipt.unexpectedWrites,[]);receipt.passed=true;
 }catch(e){receipt.failure=e.stack;if(p&&!p.isClosed())await p.screenshot({path:dir+'/failure.png'});throw e}
 finally{await browser.close();fs.mkdirSync('docs/test-output',{recursive:true});fs.writeFileSync('docs/test-output/public-footer-compact.json',JSON.stringify(receipt,null,2))}
 console.log(JSON.stringify(receipt,null,2));
})().catch(e=>{console.error(e.stack);process.exitCode=1});
