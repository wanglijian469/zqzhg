// Layout-only fixtures. Business permissions are tested separately against isolated MySQL.
const fs=require('node:fs'),path=require('node:path'),http=require('node:http'),assert=require('node:assert/strict');
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
const out=path.resolve('docs/screenshots/mobile-ui-20261024');fs.mkdirSync(out,{recursive:true});
const roots={before:path.resolve('.local/android-web-dist'),after:path.resolve('.local/mobile-ui-web-dist')};
const vendor={id:1,kind:'vendor',hall_id:1,showroom_slug:'jinong',data:{name:'河北冀农农机具有限公司',name_en:'Hebei Jinong Agricultural Machinery Manufacturing Company',main_products:'液压翻转犁、旋耕机及农机配件',region_name:'河北省邢台市宁晋县',description:'专注农业机械产品研发与制造，提供液压翻转犁及相关配件。',description_en:'Agricultural machinery manufacturing and product support.',phone:'13900000001',media_ids:[],company_profile:{summary:'农业机械产品与制造服务',summary_en:'Agricultural equipment and manufacturing services'}}};
const cats=[{id:101,hall_id:1,name:'耕整地机械',name_en:'Tillage machinery and equipment',enabled:true,selectable:true,parent_id:0}];
const products=Array.from({length:6},(_,i)=>({id:i+11,kind:'product',hall_id:1,data:{name:'液压翻转犁产品 '+(i+1),name_en:'Hydraulic reversible plough model '+(i+1),category_id:101,product_category_id:101,model:'JN-2026',parameters:'型号：JN-2026',media_ids:[]},vendor:{id:1,name:vendor.data.name,name_en:vendor.data.name_en,region_name:vendor.data.region_name}}));
const requests=Array.from({length:8},(_,i)=>({id:i+1,hall_id:1,title:'采购液压翻转犁及配件 '+(i+1),state:'open',delivery_region:'130528',deadline:'2026-12-20T08:00:00Z',published_at:'2026-10-24T08:00:00Z',items:[{name:'液压翻转犁',category_id:101,category_name:cats[0].name,category_name_en:cats[0].name_en,quantity:'2.5',unit:'台'}]}));
const nodes=[{code:'13',name:'河北省',parent_code:'',full_name:'河北省'},{code:'1305',name:'邢台市',parent_code:'13',full_name:'河北省邢台市'},{code:'130528',name:'宁晋县',parent_code:'1305',full_name:'河北省邢台市宁晋县'}];
let variant='after';
const mime={'.js':'text/javascript','.css':'text/css','.svg':'image/svg+xml','.json':'application/json','.wasm':'application/wasm'};
const server=http.createServer((req,res)=>{
 const root=roots[variant];let name=decodeURIComponent(new URL(req.url,'http://localhost').pathname);let file=path.resolve(root,'.'+name);
 if(!file.startsWith(root+path.sep)){res.writeHead(404);res.end();return}
 if(!fs.existsSync(file)||!fs.statSync(file).isFile())file=path.join(root,'index.html');
 res.setHeader('Content-Type',mime[path.extname(file)]||'text/html');res.end(fs.readFileSync(file));
});
function payload(url,base){
 const p=new URL(url).pathname,q=new URL(url).searchParams;
 if(p.endsWith('/public/config'))return {version:1,site:{name:'大陆农机',name_en:'Dalu Nongji',api_url:base+'/api/v1',web_url:base,media_url:base+'/api/v1/media'},uploads:{image_mib:10,video_mib:200,pdf_mib:50,default_quota_bytes:1000000},capabilities:{procurement:true,procurement_public_summary:true,administrative_regions:true}};
 if(p.endsWith('/public/halls'))return {items:[{id:1,slug:'agriculture',name:'农业机械馆',name_en:'Agricultural Machinery Hall',status:'open'}]};
 if(p.endsWith('/auth/me'))return {id:7,role:'visitor',phone:'13900000001',phone_verified:true,buyer_registered:true};
 if(p.endsWith('/procurement/profile'))return {profile:{contact_name:'采购联系人',company:'验收采购单位',region_code:'13',version:1},account:{id:7,role:'visitor',phone:'13900000001',phone_verified:true,buyer_registered:true}};
 if(p.endsWith('/taxonomy'))return {categories:cats,regions:[{id:13,name:'河北省',name_en:'Hebei'}],halls:[]};
 if(p.endsWith('/administrative-regions'))return {items:nodes.filter(n=>n.parent_code===(q.get('parent_code')||'')),selected:nodes.find(n=>n.code===q.get('code')),trail:[]};
 if(p.endsWith('/public/procurements'))return {items:q.get('q')==='no-result'?[]:requests,total:q.get('q')==='no-result'?0:8,page:1,page_size:20};
 if(/\/public\/procurements\/\d+$/.test(p))return requests.find(r=>r.id===Number(p.split('/').pop()));
 if(p.endsWith('/resources'))return {items:[],total:0,page:1,page_size:20};
 if(p.includes('/public/showrooms/'))return {item:vendor,product_count:6,home_products:products,representative_products:products.slice(0,3),resource_counts:{video:0,catalog:0},categories:cats};
 if(p.endsWith('/public/products'))return {items:products,total:6,page:1,page_size:24};
 return {items:[],total:0,unread:0,page:1,page_size:20};
}
(async()=>{
 await new Promise(r=>server.listen(0,'127.0.0.1',r));const base='http://127.0.0.1:'+server.address().port;
 const browser=await chromium.launch({headless:true,executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'});
 const checks=[],errors=[];
 async function context(width,lang,embedded=false){
  const c=await browser.newContext({viewport:{width,height:844}});
  await c.addInitScript(({lang,embedded})=>{localStorage.setItem('zqzt.language',lang);sessionStorage.setItem('zqzt.merchant','layout-only-session');sessionStorage.setItem('zqzt.merchant.id','7');sessionStorage.setItem('zqzt.merchant.role','visitor');if(embedded){window.__DLNJ_NATIVE__=true;document.addEventListener('DOMContentLoaded',()=>document.documentElement.classList.add('native-workspace'))}},{lang,embedded});
  await c.route('**/api/v1/**',route=>route.fulfill({json:payload(route.request().url(),base)}));
  const page=await c.newPage();page.on('pageerror',e=>errors.push(e.message));return {c,page};
 }
 async function font(page,scale){if(scale===1)return;await page.evaluate(scale=>{const nodes=[...document.querySelectorAll('input,textarea,select,button,label,h1,h2,h3,p,small,a,span,strong')].map(el=>[el,parseFloat(getComputedStyle(el).fontSize)]);nodes.forEach(([el,size])=>{const base=Number(el.dataset.uiBaseFont||size);el.dataset.uiBaseFont=String(base);el.style.fontSize=(base*scale)+'px'})},scale)}
 async function check(page,label,width,lang,scale=1){assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),`horizontal overflow ${label} ${width} ${lang} ${scale}`);checks.push({label,width,lang,scale});if(scale===1&&(width===390||width===1366))await page.screenshot({path:path.join(out,label+'-'+width+'-'+lang+'.png'),fullPage:true})}
 try{
  // Old bundled resources are a truthful before image; they are not rebuilt or edited.
  if(!fs.existsSync(path.join(out,'before-procurement-390-zh.png'))&&fs.existsSync(path.join(roots.before,'index.html'))){variant='before';const {c,page}=await context(390,'zh');await page.goto(base+'/procurement');await page.locator('.proc-hall-filters').waitFor();await page.screenshot({path:path.join(out,'before-procurement-390-zh.png'),fullPage:true});await page.goto(base+'/v/jinong');await page.locator('.sr-home-hero').waitFor();await page.screenshot({path:path.join(out,'before-showroom-home-390-zh.png'),fullPage:true});await page.getByRole('tab',{name:'企业介绍',exact:true}).click();await page.screenshot({path:path.join(out,'before-showroom-about-390-zh.png'),fullPage:true});await c.close()}
  variant='after';
  for(const width of [360,390,430])for(const lang of ['zh','en'])for(const scale of [1,1.3,1.5]){
   const {c,page}=await context(width,lang,true);
   await page.goto(base+'/procurement');await page.locator('.proc-summary-link').first().waitFor();await font(page,scale);
   assert.equal(await page.locator('.workspace-sidebar,.workspace-topbar,.proc-mobile-header').count(),0);
   assert.equal(await page.locator('.proc-shortcuts>a').count(),3);await check(page,'after-procurement',width,lang,scale);
   await page.locator('.proc-filter-row button').first().click();await page.getByRole('dialog').waitFor();await font(page,scale);await check(page,'after-filters',width,lang,scale);await page.keyboard.press('Escape');
   if(width===390&&lang==='zh'&&scale===1){
    await page.evaluate(()=>scrollTo(0,500));await page.waitForFunction(()=>scrollY>=490);
    await page.locator('.proc-summary-link').nth(3).click();await page.locator('.proc-required-hint').waitFor();
    await page.goBack();await page.locator('.proc-summary-link').first().waitFor();await page.waitForFunction(()=>scrollY>=490);
    await page.evaluate(()=>scrollTo(0,0));
    await page.getByRole('textbox',{name:'项目关键词'}).fill('no-result');await page.getByRole('button',{name:'查询',exact:true}).click();await page.locator('.proc-empty').waitFor();await check(page,'after-empty',width,lang,scale);
    await page.locator('.proc-empty button').click();await page.locator('.proc-summary-link').first().waitFor();
    const failing='**/api/v1/public/procurements?q=network-error';await page.route(failing,route=>route.abort());await page.getByRole('textbox',{name:'项目关键词'}).fill('network-error');await page.getByRole('button',{name:'查询',exact:true}).click();await page.locator('[role=alert]').waitFor();await check(page,'after-network-error',width,lang,scale);
    await page.unroute(failing);await page.getByRole('button',{name:'重试',exact:true}).click();await page.locator('.proc-summary-link').first().waitFor();
   }
   await page.locator('.proc-summary-link').first().click();await page.locator('.proc-required-hint').waitFor();await check(page,'after-summary',width,lang,scale);
   await page.goto(base+'/buyer/new');await page.locator('.proc-line-editor').waitFor();await font(page,scale);await check(page,'after-publish',width,lang,scale);
   await page.goto(base+'/v/jinong');await page.locator('.sr-home-hero').waitFor();await font(page,scale);await page.evaluate(()=>window.__stableHero=document.querySelector('.sr-home-hero'));
   for(const tab of ['about','products','videos','catalogs','contact','home']){await page.locator('#tab-'+tab).click();await font(page,scale);assert(await page.evaluate(()=>window.__stableHero===document.querySelector('.sr-home-hero')));await check(page,'after-showroom-'+tab,width,lang,scale)}
   await c.close();
  }
  for(const width of [1366,1920])for(const lang of ['zh','en']){const {c,page}=await context(width,lang);await page.goto(base+'/procurement');await page.locator('.proc-hall-filters').waitFor();assert(await page.locator('.workspace-sidebar').isVisible());await check(page,'after-desktop-procurement',width,lang);await page.goto(base+'/v/jinong');await page.locator('.sr-home-hero').waitFor();await check(page,'after-desktop-showroom',width,lang);await c.close()}
  assert.deepEqual(errors,[]);fs.writeFileSync(path.join(out,'browser-verification.json'),JSON.stringify({checks,errors,fixtures:'synthetic layout data; no business writes'},null,2));console.log(JSON.stringify({layoutChecks:checks.length,errors:errors.length,screenshots:out}));
 }finally{await browser.close();server.close()}
})().catch(e=>{console.error(e);server.close();process.exitCode=1});
