const fs=require('node:fs');
const assert=require('node:assert/strict');
const path=require('node:path');
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
const base=process.env.V2_BASE_URL||'http://localhost:8080';
const evidence=path.resolve('docs/screenshots/v2-hero-adjustment');fs.mkdirSync(evidence,{recursive:true});
(async()=>{
 const browser=await chromium.launch({executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',headless:true,args:['--no-proxy-server']});
 const errors=[],checks=[];
 try{
  for(const width of [1440,1280,768,390,360])for(const scale of [1,1.25]){
   const context=await browser.newContext({viewport:{width:Math.floor(width/scale),height:Math.floor(960/scale)},deviceScaleFactor:scale});
   const page=await context.newPage();page.on('pageerror',e=>errors.push(e.message));
   for(const route of ['/','/browse','/browse/products'])for(const language of ['zh','en']){
    await page.goto(base+route);await page.locator('h1').first().waitFor();const toggle=page.getByRole('button',{name:language==='en'?'English':'中文',exact:true});if(await toggle.count())await toggle.click();
    await page.locator(route==='/'?'.brand-showroom-card':route==='/browse'?'.vendor-brand-card':'.product-results .item-card').first().waitFor();
    const metrics=await page.evaluate(()=>({viewport:innerWidth,scroll:document.documentElement.scrollWidth,cards:document.querySelectorAll('.item-card').length}));
    assert.ok(metrics.scroll<=metrics.viewport+1,'horizontal overflow '+route+' '+width+' '+scale+' '+JSON.stringify(metrics));
    assert.ok(await page.locator('.industry-section').count()===0);
    if(route==='/browse/products')assert.ok(await page.locator('.product-results .item-card').count()<=24);
    if(route==='/'){
     assert.ok(await page.locator('.brand-showroom-card').count()<=8);
     assert.equal(await page.locator('.hero-product-strip').count(),0);
     assert.equal(await page.locator('.exhibition-stage img').count(),1);
     assert.ok(await page.locator('.industrial-products .item-card').count()>0);
     const hero=await page.locator('.v2-hero h1').evaluate(el=>{const range=document.createRange();range.selectNodeContents(el);const lines=new Set(Array.from(range.getClientRects()).filter(r=>r.width>0).map(r=>Math.round(r.top)));const image=document.querySelector('.exhibition-stage .home-platform-image').getBoundingClientRect();return {text:el.textContent,lines:lines.size,fits:el.scrollWidth<=el.clientWidth+1,imageRatio:image.width/image.height}});
     if(language==='zh'){assert.equal(hero.text,'中企展馆 · 农业机械厅');assert.equal(hero.lines,1,'Chinese hero title wrapped at '+width+' / '+scale);assert.ok(hero.fits,'Chinese hero title clipped')}
     assert.ok(Math.abs(hero.imageRatio-16/9)<0.03,'Hero photo ratio');
     metrics.hero=hero;
    }
    checks.push({width,equivalentZoom:scale,language,route,...metrics});
    if(scale===1){await page.locator('img').evaluateAll(images=>images.forEach(i=>i.loading='eager'));await page.evaluate(()=>Promise.all(Array.from(document.images).map(i=>i.complete?Promise.resolve():new Promise(resolve=>{i.onload=resolve;i.onerror=resolve}))));await page.screenshot({path:path.join(evidence,'v2-'+(route==='/'?'home':route==='/browse'?'manufacturers':'products')+'-'+width+(language==='en'?'-en':'')+'.png'),fullPage:true})}
   }
   await context.close();
  }
  const context=await browser.newContext({viewport:{width:1440,height:960}});const page=await context.newPage();page.on('pageerror',e=>errors.push(e.message));
  const home=await (await page.request.get(base+'/api/v1/public/home')).json();const total=await (await page.request.get(base+'/api/v1/public/products?page_size=24')).json();
  assert.ok(home.vendors.length<=8&&home.products.length<=12);assert.equal(total.page_size,24);assert.equal(total.items.length,Math.min(24,total.total));
  const cable=await (await page.request.get(base+'/api/v1/public/home?hall=cable')).json();assert.equal(cable.vendors.length,0);assert.equal(cable.products.length,0);assert.equal(cable.vendor_count,0);
  const list=await (await page.request.get(base+'/api/v1/public/vendors?page_size=20')).json();assert.equal(list.page_size,20);
  await page.goto(base+'/browse/products');await page.locator('.product-results .item-card').first().waitFor();
  await page.getByRole('combobox',{name:'排序方式'}).selectOption('latest');await page.waitForURL(/sort=latest/);
  if(total.total>24){await page.getByRole('button',{name:'下一页',exact:true}).click();await page.waitForURL(/page=2/);await page.locator('.product-results .item-card').first().waitFor();assert.equal(await page.locator('.product-results .item-card').count(),Math.min(24,total.total-24));}
  const original=new URL(page.url()).pathname+new URL(page.url()).search;const productName=await page.locator('.product-results h3').first().textContent();await page.locator('.product-results h3').first().click();await page.getByRole('heading',{name:productName,exact:true}).waitFor();
  await page.locator('main').getByRole('link',{name:'找产品',exact:true}).click();await page.waitForURL(base+original);await page.locator('.product-results .item-card').first().waitFor();
  await page.locator('.product-vendor').first().click();await page.locator('.showroom-hero h1').waitFor();await page.getByRole('tab',{name:'展品',exact:true}).click();await page.locator('.showroom-product-grid .item-card').first().waitFor();
  await page.goto(base+'/');await page.getByRole('button',{name:'农业机械厅',exact:true}).click();await page.getByRole('menuitem',{name:/电线电缆厅/}).click();await page.getByRole('heading',{name:'中企展馆 · 电线电缆厅',exact:true}).waitFor();assert.equal(new URL(page.url()).search,'?hall=cable');assert.equal(await page.locator('.brand-showroom-card,.item-card').count(),0);
  await page.screenshot({path:path.join(evidence,'v2-cable-preparing.png'),fullPage:true});await page.getByRole('button',{name:'English',exact:true}).click();await page.getByRole('heading',{name:'ZQZG Exhibition · Wire & Cable Hall',exact:true}).waitFor();assert.ok((await page.title()).includes('Wire & Cable Hall'));
  await page.goto(base+'/?hall=missing');await page.getByRole('alert').waitFor();
  await page.goto(base+'/merchant/login');await page.getByRole('textbox',{name:'Company login name'}).or(page.getByRole('textbox',{name:'公司登录账号'})).waitFor();
  await page.goto(base+'/join');await page.getByRole('textbox',{name:/Company \/ factory name|企业 \/ 工厂名称/}).waitFor();
  await page.goto(base+'/admin');await page.getByRole('heading',{name:/Administrator sign-in|管理员登录/}).waitFor();
  await context.close();
  assert.deepEqual(errors,[]);
  const report={base,checkedAt:new Date().toISOString(),checks,publicVendors:list.total,publicProducts:total.total,homepageVendors:home.vendors.length,homepageProducts:home.products.length,consoleErrors:errors,interactionChecks:['product pagination','sorting','detail return preserves URL','product to vendor showroom','vendor product catalog','hall switching','preparing hall isolation','English','unknown hall','merchant login','join','admin login'],zoomNote:'1.25 uses equivalent CSS viewport with device scale factor; native browser zoom remains a manual acceptance item.'};fs.writeFileSync('docs/test-output/v2-browser.json',JSON.stringify(report,null,2));console.log(JSON.stringify(report,null,2));
 }finally{await browser.close()}
})().catch(e=>{console.error(e);process.exitCode=1});
