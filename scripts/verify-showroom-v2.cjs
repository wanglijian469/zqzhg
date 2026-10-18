const fs=require('fs'),path=require('path'),assert=require('assert/strict');
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
(async()=>{
 const out=path.resolve(__dirname,'../docs/screenshots/showroom-v2');fs.mkdirSync(out,{recursive:true});const reports=path.resolve(__dirname,'../docs/test-output');fs.mkdirSync(reports,{recursive:true});
 const browser=await chromium.launch({headless:true,executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',args:['--no-proxy-server']});const checks=[],errors=[];
 const base='http://localhost:8080';const vendors=await fetch(base+'/api/v1/public/vendors?page_size=20').then(r=>r.json());
 const before=JSON.parse(fs.readFileSync(path.resolve(__dirname,'../.local/backups/pre-showroom-v2-20261017/public-data.json')));const after=[];
 for(const v of vendors.items){const s=await fetch(base+'/api/v1/public/showrooms/'+v.showroom_slug).then(r=>r.json());after.push({id:v.id,name:v.data.name,product_count:s.product_count,representatives:s.representative_products.map(p=>p.id),resources:s.resource_counts});assert(s.home_products.length<=8);assert.equal(new Set(s.home_products.map(p=>p.id)).size,s.home_products.length);assert(s.home_products.every(p=>p.vendor_id===v.id));}
 assert.deepEqual(after,before,'Public business data changed');
 for(const lang of ['zh','en'])for(const width of [1440,1366,768,390,375])for(const zoom of [1,1.25]){
  const context=await browser.newContext({viewport:{width:Math.floor(width/zoom),height:Math.floor(1000/zoom)},deviceScaleFactor:1});await context.addInitScript(l=>localStorage.setItem('zqzt.language',l),lang);const page=await context.newPage();page.on('pageerror',e=>errors.push(e.message));
  for(const tab of ['home','about','products','videos','catalogs','contact']){
   await page.goto(base+'/v/v-21?tab='+tab);await page.locator('.sr-content').waitFor();
   if(tab==='home'){await page.locator('.portal-home-resource .resource-card').last().waitFor();}
   if(tab==='products')await page.locator('.showroom-product-grid article').first().waitFor();
   if(tab==='videos'||tab==='catalogs')await page.locator('.resource-card').first().waitFor();
   for(const img of await page.locator('.resource-card img').all()){await img.scrollIntoViewIfNeeded();await img.evaluate(i=>i.complete?Promise.resolve():new Promise(r=>{i.onload=r;i.onerror=r}));}
   const result=await page.evaluate(({width,zoom,lang,tab})=>{const bound=e=>{const r=e.getBoundingClientRect();return {x:r.x,y:r.y,w:r.width,h:r.height}};return {width,zoom,lang,tab,viewport:innerWidth,overflow:document.documentElement.scrollWidth>innerWidth,hero:document.querySelector('.sr-home-hero')?bound(document.querySelector('.sr-home-hero')):null,compact:!!document.querySelector('.sr-compact-company'),tabs:document.querySelectorAll('[role=tab]').length,productIds:[...document.querySelectorAll('.showroom-product-grid article')].map(e=>e.querySelector('a').getAttribute('href')),resources:[...document.querySelectorAll('.resource-card-cover')].map(e=>({box:bound(e),images:[...e.querySelectorAll('img')].map(bound)}))}}, {width,zoom,lang,tab});
   assert(!result.overflow,JSON.stringify(result));assert.equal(result.tabs,6);assert.equal(!!result.hero,tab==='home');assert.equal(result.compact,tab!=='home');assert.equal(new Set(result.productIds).size,result.productIds.length);
   if(result.hero&&result.viewport>1000)assert(result.hero.h>=360&&result.hero.h<=440,'Hero height '+result.hero.h);
   for(const r of result.resources){assert(Math.abs(r.box.w/r.box.h-16/9)<.03);for(const i of r.images)assert(i.w<=r.box.w+1&&i.h<=r.box.h+1,'Resource image escaped cover');}
   if(lang==='zh'&&zoom===1&&[1440,390].includes(width)){for(const img of await page.locator('.sr-content img').all()){await img.scrollIntoViewIfNeeded();await img.evaluate(i=>i.complete?Promise.resolve():new Promise(r=>{i.onload=r;i.onerror=r}));}await page.evaluate(()=>scrollTo(0,0));await page.screenshot({path:path.join(out,tab+'-'+width+'.png'),fullPage:width!==390});}
   checks.push(result);
  }
  // Touch navigation, video activation, reader paging and modal quick-bar layering.
  if(width===390&&zoom===1&&lang==='zh'){
   await page.goto(base+'/v/v-21?tab=videos');await page.locator('.resource-card').waitFor();assert.equal(await page.locator('video').count(),0);await page.locator('.resource-card').click();await page.locator('video').waitFor();assert.equal(await page.locator('video').getAttribute('preload'),'none');assert.equal(await page.locator('.showroom-mobile-contact').isVisible(),false);await page.keyboard.press('Escape');
   await page.goto(base+'/v/v-21?tab=catalogs');await page.locator('.resource-card').click();await page.locator('.image-catalog img').waitFor();const first=await page.locator('.image-catalog img').getAttribute('src');await page.getByRole('button',{name:'下一页',exact:true}).click();assert.notEqual(await page.locator('.image-catalog img').getAttribute('src'),first);await page.getByRole('button',{name:'放大',exact:true}).click();assert((await page.locator('.image-catalog').getAttribute('style')).includes('1.25'));await page.keyboard.press('Escape');await page.locator('.sr-menu-toggle').click();assert(await page.getByRole('tab',{name:'产品中心',exact:true}).isVisible());await page.getByRole('tab',{name:'产品中心',exact:true}).click();await page.locator('.showroom-product-grid article').first().waitFor();await page.locator('.showroom-product-grid article a').first().click();await page.locator('.detail-layout').waitFor();await page.getByRole('link',{name:'返回厂商展厅',exact:true}).click();await page.locator('.showroom-product-grid article').first().waitFor();assert(new URL(page.url()).searchParams.get('tab')==='products');
  }
  await context.close();
 }
 await browser.close();assert.equal(errors.length,0,errors.join('\n'));const report={checked_at:new Date().toISOString(),checks:checks.length,layouts:checks,consoleErrors:errors,businessDataUnchanged:true,publicData:after};fs.writeFileSync(path.join(reports,'showroom-v2-browser.json'),JSON.stringify(report,null,2));console.log(JSON.stringify({checks:checks.length,errors,dataUnchanged:true,screenshots:out}));
})().catch(e=>{console.error(e);process.exit(1)});
