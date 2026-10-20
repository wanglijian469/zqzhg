// Read-only acceptance against the running local service. No account/data mutations.
const assert=require('node:assert/strict');
const fs=require('node:fs');
const crypto=require('node:crypto');
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
const base='http://127.0.0.1:8080';
const receipt=JSON.parse(fs.readFileSync('.local/android-preview-verification.json','utf8').replace(/^\uFEFF/,''));
const before=JSON.parse(fs.readFileSync('.local/mobile18-business-before.json','utf8').replace(/^\uFEFF/,''));
async function json(path,origin=base){const r=await fetch(origin+path,{signal:AbortSignal.timeout(15000)});assert.equal(r.status,200,path);return r.json()}
async function hash(path){const response=await fetch(base+path);assert.equal(response.status,200,path);const h=crypto.createHash('sha256');for await(const chunk of response.body)h.update(chunk);return h.digest('hex')}
(async()=>{
 const results={readonly:true,deviceTested:false,productionDeployed:false,checks:[]};
 const config=await json('/api/v1/public/config');
 for(const key of ['administrative_regions','industry_map','product_name_only'])assert.equal(config.capabilities[key],true);
 results.checks.push('three public capability flags');
 assert.equal((await json('/api/v1/health','http://172.27.35.2:8080')).status,'ok');
 assert.equal((await json('/api/v1/dev/map-preview/config')).enabled,true);
 assert.equal((await fetch('http://172.27.35.2:8080/api/v1/dev/map-preview/config')).status,404);
 results.checks.push('LAN health200; preview allowed on loopback and refused on LAN');
 const preview=await json('/downloads/previews/manifest.json');assert.equal(preview.version,receipt.version);assert.equal(preview.sha256,receipt.sha256);
 assert.equal(await hash(preview.url),receipt.sha256);assert.equal(await hash('/downloads/dalunongji.apk'),receipt.stableSHA256);
 assert.deepEqual((await json('/maps/manifest.json')).resources,{});
 results.checks.push('downloaded acceptance/stable APK hashes; formal manifest remains empty');
 const browser=await chromium.launch({executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',headless:true,args:['--no-proxy-server']});
 const errors=[],layouts=[];fs.mkdirSync('docs/screenshots/mobile18-local',{recursive:true});
 try{
  const context=await browser.newContext();
  const readonlyCache=new Map();
  await context.route('**/api/v1/**',async route=>{
   if(!['GET','HEAD'].includes(route.request().method()))return route.fulfill({status:202,contentType:'application/json',body:'{}'});
   const key=route.request().url();
   if(readonlyCache.has(key))return route.fulfill(readonlyCache.get(key));
   const response=await route.fetch();
   const result={status:response.status(),contentType:response.headers()['content-type']||'application/json',body:await response.body()};
   if(result.status===200)readonlyCache.set(key,result);
   return route.fulfill(result);
  });
  const page=await context.newPage();page.on('pageerror',e=>errors.push(e.message));
  for(const [width,height] of [[1440,900],[1366,768],[390,844],[360,640],[844,390]]){
   await page.setViewportSize({width,height});
   for(const path of ['/join','/merchant/login','/regions','/app']){
    await page.goto(base+path);
    if(path==='/regions')await page.getByRole('button',{name:'复位地图',exact:true}).waitFor();
    else if(path==='/app')await page.getByRole('link',{name:'下载独立验收 APK',exact:true}).waitFor();
    else await page.getByLabel('公司登录账号',{exact:true}).waitFor();
    assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),path+' fits width');
    if(path==='/regions'||(width>=960&&path!=='/app'))assert.ok(await page.evaluate(()=>document.documentElement.scrollHeight<=innerHeight+1),path+' fits height');
    if(path==='/app')assert.equal(await page.getByRole('link',{name:'下载独立验收 APK',exact:true}).getAttribute('href'),preview.url);
    layouts.push({width,height,path,passed:true});
    if((width===390&&['/join','/app'].includes(path))||(width===1440&&path==='/app'))await page.screenshot({path:'docs/screenshots/mobile18-local/'+path.slice(1).replaceAll('/','-')+'-'+width+'.png',fullPage:true});
   }
  }
  await page.setViewportSize({width:1440,height:900});await page.goto(base+'/regions');
  await page.getByRole('button',{name:'复位地图',exact:true}).waitFor();
  for(const [name,code] of [['河北','13'],['邢台市','1305'],['宁晋县','130528']]){
   await page.locator('.industry-chart svg text').filter({hasText:new RegExp('^'+name+'$')}).first().click();
   await page.waitForURL(url=>url.searchParams.get('region_code')===code);
   await page.getByRole('button',{name:'复位地图',exact:true}).waitFor();
   const stats=await json('/api/v1/public/industry-map/regions?hall=agriculture&region_code='+code);
   await page.waitForFunction(total=>document.querySelector('.industry-panel-heading b')?.textContent===String(total),stats.total);
   assert.equal((await json('/api/v1/public/industry-map/vendors?hall=agriculture&region_code='+code+'&page_size=8&unassigned=0')).total,stats.total);
  }
  await page.goBack();await page.waitForURL(url=>url.searchParams.get('region_code')==='1305');
  await page.goForward();await page.waitForURL(url=>url.searchParams.get('region_code')==='130528');
  results.checks.push('local learning outline actual clicks Hebei/Xingtai/Ningjin and history; counts match API (not formal map acceptance); viewport repeats reuse successful live GETs to respect rate limits');
  assert.deepEqual(errors,[]);results.layouts=layouts;results.browserErrors=errors;
 }finally{await browser.close()}
 const vendors=await json('/api/v1/public/vendors?page_size=20'),products=await json('/api/v1/public/products?page_size=1');
 assert.equal(vendors.total,before.vendors);assert.equal(products.total,before.products);assert.deepEqual(vendors.items.map(x=>x.id).sort((a,b)=>a-b),before.vendorIDs);
 results.businessAfter={vendors:vendors.total,vendorIDs:before.vendorIDs,products:products.total};results.checks.push('business public counts/IDs unchanged; all browser writes intercepted');
 fs.writeFileSync('docs/test-output/mobile18-local.json',JSON.stringify(results,null,2)+'\n');console.log(JSON.stringify(results,null,2));
})().catch(error=>{console.error(error);process.exitCode=1});
