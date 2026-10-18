const fs=require('fs'),path=require('path'),assert=require('assert/strict');
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
(async()=>{
 const base='http://localhost:8080',out=path.resolve('docs/screenshots/p0p1');fs.mkdirSync(out,{recursive:true});
 const vendors=await(await fetch(base+'/api/v1/public/vendors?page_size=20')).json();const products=await(await fetch(base+'/api/v1/public/products?page_size=24')).json();
 const before={vendors:vendors.total,products:products.total,vendorIds:vendors.items.map(x=>x.id)};
 const product=products.items[0],slug='v-21';const checks=[],errors=[];const quick=process.env.MW_QUICK==='1';
 const pages=[['home','/'],['preparing','/?hall=cable'],['manufacturers','/browse'],['products','/browse/products'],['detail','/products/'+product.id],...['home','about','products','videos','catalogs','contact'].map(tab=>['showroom-'+tab,'/v/'+slug+'?tab='+tab])];
 const browser=await chromium.launch({headless:true,executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',args:['--no-proxy-server']});
 let page;
 try{
 for(const width of (quick?[375,768,1440]:[360,375,390,414,768,1280,1366,1440]))for(const lang of (quick?['zh']:['zh','en']))for(const zoom of (quick?[1]:[1,1.25])){
 const context=await browser.newContext({viewport:{width:Math.floor(width/zoom),height:Math.floor(812/zoom)},deviceScaleFactor:1});await context.route('**/api/v1/public/analytics/events',r=>r.fulfill({status:202,contentType:'application/json',body:'{}'}));await context.addInitScript(l=>localStorage.setItem('zqzt.language',l),lang);page=await context.newPage();page.on('pageerror',e=>errors.push(e.message));page.on('console',m=>{if(m.type()==='error')errors.push(m.text())});
 for(const [name,url] of pages){
 await page.goto(base+url);await page.locator(name.startsWith('showroom')?'.sr-content':'.public-web main h1').first().waitFor();
 if(name==='home')await page.locator('.brand-showroom-card').first().waitFor();
 if(name==='products'||name==='showroom-products')await page.locator('.product .card-content h3').first().waitFor();
 if(name==='manufacturers')await page.locator('.vendor-brand-card').first().waitFor();
 if(name.endsWith('videos')||name.endsWith('catalogs'))await page.locator('.resource-card').first().waitFor();
 await page.evaluate(()=>document.fonts.ready);await page.waitForTimeout(40);
 const result=await page.evaluate(({name,width,lang,zoom})=>{
 const rect=e=>{if(!e)return null;const r=e.getBoundingClientRect();return{x:r.x,y:r.y,w:r.width,h:r.height,right:r.right,bottom:r.bottom}};
 const image=document.querySelector('.showroom-product-grid .card-image');const title=document.querySelector('.showroom-product-grid h3');const quickbar=document.querySelector('.showroom-mobile-contact');
 return{name,width,lang,zoom,viewport:innerWidth,overflow:document.documentElement.scrollWidth>innerWidth+1,heroTitle:document.querySelector('.v2-hero h1')?{...rect(document.querySelector('.v2-hero h1')),textWidth:document.querySelector('.v2-hero h1').scrollWidth,clientWidth:document.querySelector('.v2-hero h1').clientWidth}:null,pageScroll:scrollY,searchVisible:!!document.querySelector('.persistent-search input')&&!!document.querySelector('.persistent-search input').getClientRects().length,filterOpen:document.querySelector('.responsive-filters')?.open,tabs:document.querySelectorAll('[role=tab]').length,activeTab:rect(document.querySelector('[role=tab][aria-selected=true]')),tabStrip:rect(document.querySelector('.sr-tabs')),firstProductImage:rect(image),firstProductTitle:rect(title),quickbar:rect(quickbar),gridColumns:getComputedStyle(document.querySelector('.showroom-product-grid,.product-results,.industrial-products')||document.body).gridTemplateColumns,productIds:[...document.querySelectorAll('.product')].map(e=>e.querySelector('a').getAttribute('href'))};
 },{name,width,lang,zoom});
 checks.push(result);assert(!result.overflow,JSON.stringify(result));
 if(name==='preparing')assert.equal(await page.locator('.product,.vendor-brand-card').count(),0);
 assert.equal(await page.locator('head title').count(),1);assert.equal(await page.locator('head link[rel=canonical]').count(),1);assert.equal(await page.locator('head meta[name=description]').count(),1);assert.equal(await page.locator('img:not([alt]),img[alt=""]').count(),0,'Missing image alt');
 if(name.startsWith('showroom')&&(name.endsWith('videos')||name.endsWith('catalogs'))){for(const image of await page.locator('.resource-card-cover img').all()){const box=await image.boundingBox(),cover=await image.locator('..').boundingBox();assert(box.height<=cover.height+1,'Resource image overflows cover')}}
 if(result.heroTitle&&lang==='zh')assert(result.heroTitle.right<=result.viewport+1&&result.heroTitle.x>=0&&result.heroTitle.textWidth<=result.heroTitle.clientWidth+1,'Clipped Chinese title '+JSON.stringify(result));
 if(name==='products'||name==='manufacturers'||name==='showroom-products'){assert(result.searchVisible);assert.equal(result.filterOpen,result.viewport>600,'Filter default '+JSON.stringify(result));}
 if(name.startsWith('showroom')){assert.equal(result.tabs,6);assert(result.activeTab.x>=result.tabStrip.x-1&&result.activeTab.right<=result.tabStrip.right+1,'Active tab outside horizontal strip '+JSON.stringify(result));}
 if(name==='showroom-home')assert.equal(result.pageScroll,0,'Company hero unexpectedly scrolled');
 if(name==='showroom-products'&&width===375&&zoom===1&&lang==='zh'){assert(result.firstProductTitle.bottom<=result.quickbar.y,'First product image and title hidden below quick bar: '+JSON.stringify(result));}
 assert.equal(new Set(result.productIds).size,result.productIds.length,'Duplicate product cards');
 if(lang==='zh'&&zoom===1&&[375,390,768,1440].includes(width)){
 await page.locator('img').evaluateAll(images=>Promise.all(images.filter(i=>i.loading!=='lazy'||i.getBoundingClientRect().top<innerHeight).map(i=>i.complete?Promise.resolve():new Promise(resolve=>{i.addEventListener('load',resolve,{once:true});i.addEventListener('error',resolve,{once:true});setTimeout(resolve,2000)}))));
 await page.screenshot({path:path.join(out,name+'-'+width+'.png'),fullPage:false});
 }
 }
 await context.close();console.log('Layouts passed: '+width+' '+lang+' '+zoom);
 }
 const afterV=await(await fetch(base+'/api/v1/public/vendors?page_size=20')).json(),afterP=await(await fetch(base+'/api/v1/public/products?page_size=24')).json();assert.deepEqual({vendors:afterV.total,products:afterP.total,vendorIds:afterV.items.map(x=>x.id)},before);assert.equal(errors.length,0,errors.join('\n'));
 fs.writeFileSync('docs/test-output/p0p1-browser.json',JSON.stringify({checks:checks.length,layouts:checks,errors,publicData:before,businessDataUnchanged:true,deviceValidation:false},null,2));console.log(JSON.stringify({checks:checks.length,errors,screenshots:out}));
 }catch(e){if(page&&!page.isClosed())await page.screenshot({path:path.join(out,'failure.png')});throw e}finally{await browser.close()}
})().catch(e=>{console.error(e);process.exit(1)});
