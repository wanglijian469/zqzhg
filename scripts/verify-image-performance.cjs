// Actual production frontend and backend, with disposable fixture data only.
const fs=require('node:fs');const path=require('node:path');const assert=require('node:assert/strict');
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
const f=JSON.parse(fs.readFileSync(process.argv[2],'utf8'));const mode=process.env.IMAGE_BENCH_MODE||'optimized';
assert.match(f.base,/^http:\/\/127\.0\.0\.1:\d+$/);
const output=path.resolve('.local/lightweight',mode+'-browser.json');
(async()=>{
 const browser=await chromium.launch({executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',headless:true,args:['--no-proxy-server']});
 const rows=[],errors=[];const pages=[['home','/'],['vendors','/browse'],['products','/browse/products'],['showroom','/v/v-'+f.vendorID],['product','/products/'+f.productID]];
 async function sample(name,url,width,weak,index){
  const context=await browser.newContext({viewport:{width,height:900},deviceScaleFactor:1});const page=await context.newPage();page.on('pageerror',error=>errors.push(error.message));
  await page.addInitScript(()=>{window.__imageMetrics={lcp:0,cls:0};new PerformanceObserver(list=>{for(const e of list.getEntries())window.__imageMetrics.lcp=e.startTime}).observe({type:'largest-contentful-paint',buffered:true});new PerformanceObserver(list=>{for(const e of list.getEntries())if(!e.hadRecentInput)window.__imageMetrics.cls+=e.value}).observe({type:'layout-shift',buffered:true})});
  const cdp=await context.newCDPSession(page);await cdp.send('Network.enable');
  if(weak)await cdp.send('Network.emulateNetworkConditions',{offline:false,latency:150,downloadThroughput:1600000/8,uploadThroughput:750000/8,connectionType:'cellular3g'});
  let requests=new Map(),imageBytes=0,totalBytes=0,statuses=[];
  cdp.on('Network.requestWillBeSent',e=>requests.set(e.requestId,e.request.url));
  cdp.on('Network.loadingFinished',e=>{totalBytes+=e.encodedDataLength;if(/\/(?:api\/v1\/)?media\/\d+/.test(requests.get(e.requestId)||''))imageBytes+=e.encodedDataLength});
  cdp.on('Network.responseReceived',e=>{if(/\/(?:api\/v1\/)?media\/\d+/.test(e.response.url))statuses.push({status:e.response.status,cache:e.response.fromDiskCache||false,url:e.response.url,ttfbMs:e.response.timing?.receiveHeadersEnd})});
  async function waitImages(){await page.waitForLoadState('networkidle',{timeout:90000});const failed=await page.evaluate(()=>[...document.images].filter(i=>{const r=i.getBoundingClientRect();return r.width&&r.height&&r.top<innerHeight&&r.bottom>0&&r.left<innerWidth&&r.right>0&&(!i.complete||!i.naturalWidth)}).map(i=>({src:i.currentSrc,complete:i.complete,html:i.outerHTML})));if(failed.length)console.log('VISIBLE IMAGE DIAGNOSTIC '+JSON.stringify({name,width,failed}));if(mode==='optimized')assert.equal(failed.length,0,'Visible image failed: '+JSON.stringify(failed))}
  await page.goto(f.base+url,{waitUntil:'domcontentloaded',timeout:90000});await waitImages();
  const cold={...await page.evaluate(()=>window.__imageMetrics),imageBytes,totalBytes,imageRequests:statuses.length,responses:[...statuses]};
  const scripts=[...requests.values()].filter(url=>/\.js(?:$|\?)/.test(url));
  assert(!scripts.some(url=>/\/(admin|merchant-console|procurement|pdf|industry-map-outline|industry-map-schematic)-[^/]+\.js/.test(url)),'Unrelated module downloaded on '+name);
  imageBytes=0;totalBytes=0;statuses=[];requests=new Map();
  if(!weak){await page.reload({waitUntil:'domcontentloaded',timeout:90000});await waitImages()}
  const warm={...await page.evaluate(()=>window.__imageMetrics),imageBytes,totalBytes,imageRequests:statuses.length,statuses:[...statuses]};
  if(mode==='optimized'){
   assert(cold.cls<=0.1,name+' CLS '+cold.cls);
   const sources=await page.locator('img').evaluateAll(images=>images.map(i=>({src:i.currentSrc,priority:i.fetchPriority,loading:i.loading})));
   assert(sources.some(i=>i.src.includes('?w=')),'No responsive variants on '+name);
   const repeated=await page.request.get(f.base+'/api/v1/media/'+f.photos[0]+'?w=480');const etag=repeated.headers().etag;assert(etag);
   const unchanged=await page.request.get(f.base+'/api/v1/media/'+f.photos[0]+'?w=480',{headers:{'If-None-Match':etag}});assert.equal(unchanged.status(),304);assert.equal((await unchanged.body()).length,0);
   if(name==='product'&&!weak){await page.locator(width<=650?'.product-main-photo':'.gallery button').first().click();await page.locator('.lightbox img').waitFor();assert(!await page.locator('.lightbox img').getAttribute('srcset'),'Lightbox must use existing HD image');await page.keyboard.press('Escape');assert.equal(await page.locator('.lightbox').count(),0)}
   if(name==='home'&&width===390&&!weak){await page.screenshot({path:'.local/lightweight/home-390.png',fullPage:false});const before=(await page.locator('[data-image-pending=true]').count());await page.evaluate(()=>scrollTo(0,document.body.scrollHeight));await page.waitForLoadState('networkidle');assert(before>0,'Below-fold images should be deferred')}
  }
  rows.push({page:name,width,weak,index,cold,warm:weak?null:warm});console.log(JSON.stringify(rows.at(-1)));fs.writeFileSync(output,JSON.stringify({mode,network:{downloadMbps:1.6,latencyMs:150},rows,errors},null,2)+'\n');await context.close();
 }
 try{
  for(const width of [390,768,1440])for(const [name,url]of pages)await sample(name,url,width,false,0);
  if(process.env.IMAGE_BENCH_QUICK!=='1')for(let index=0;index<5;index++)for(const [name,url]of pages)await sample(name,url,390,true,index);
  assert.deepEqual(errors,[]);fs.writeFileSync(output,JSON.stringify({mode,network:{downloadMbps:1.6,latencyMs:150},rows,errors},null,2)+'\n');
 }finally{await browser.close()}
})().catch(error=>{console.error(error);process.exitCode=1});
