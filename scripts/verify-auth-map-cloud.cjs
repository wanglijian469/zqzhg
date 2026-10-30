// Read-only cloud acceptance: never press a send-code button or create users.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
const label=process.env.MAP_ACCEPTANCE_LABEL||'auth-1.10.2';
const bundle=process.env.MAP_BUNDLED_ROOT?path.resolve(process.env.MAP_BUNDLED_ROOT):null;
const developmentCache=process.env.MAP_DEVELOPMENT_CACHE?path.resolve(process.env.MAP_DEVELOPMENT_CACHE):null;
const base='https://zqzhg.com',dir=`docs/screenshots/${label}/map`;fs.mkdirSync(dir,{recursive:true});
(async()=>{
 const browser=await chromium.launch({executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',headless:true,args:['--no-proxy-server']});
 const receipt={origin:base,bundledAPK:!!bundle,developmentReference:developmentCache?'saved-development-cache':'live-development-api',map:[],viewports:[],errors:[]};
 try {
  const ctx=await browser.newContext({viewport:{width:1440,height:900}}),page=await ctx.newPage();
  ctx.setDefaultTimeout(30000);ctx.setDefaultNavigationTimeout(45000);
  // Keep acceptance traffic below the existing 30 list reads/minute guard.
  // This also avoids confusing a deliberate anti-scraping block with map failure.
  let queue=Promise.resolve(),lastRead=0;
  const paced=()=>{queue=queue.then(async()=>{const wait=2200-(Date.now()-lastRead);if(wait>0)await new Promise(r=>setTimeout(r,wait));lastRead=Date.now()});return queue};
  await ctx.route('**/*',async route=>{
   const u=new URL(route.request().url());
   if(u.origin===base&&(/^\/api\/v1\/public\/(industry-map|vendors|products|home)(\/|$|\?)/.test(u.pathname)||['/regions','/','/browse','/browse/products'].includes(u.pathname)))await paced();
   if(bundle&&u.origin===base&&!u.pathname.startsWith('/api/')&&route.request().method()==='GET'){
    const spa=u.pathname==='/'||['/regions','/browse','/browse/products'].includes(u.pathname)||/^\/(v|products|vendors)\//.test(u.pathname);
    const name=spa?'index.html':decodeURIComponent(u.pathname).replace(/^\//,'');
    const file=path.resolve(bundle,name);assert.ok(file.startsWith(bundle+path.sep));
    if(!fs.existsSync(file))return route.fulfill({status:404,body:''});
    let body=fs.readFileSync(file);
    if(spa){
     const runtime={...config,site:{...config.site,name:'中企展馆',name_en:'China Enterprise Exhibition',web_url:base,api_url:base+'/api/v1',media_url:base+'/api/v1/media'}};
     const bridge=fs.readFileSync(path.join(bundle,'../native-workspace.js'),'utf8').replace('__DLNJ_BOOTSTRAP__',JSON.stringify({token:'',account:{},language:'zh',nonce:'acceptance-only'}));
     const mapCSS=fs.readFileSync(path.join(bundle,'../native-map.css'),'utf8');
     body=Buffer.from(body.toString().replace(/<script[^>]*id="zqzg-runtime-config"[^>]*>[\s\S]*?<\/script>/,'').replace('<head>',`<head><script id="zqzg-runtime-config" type="application/json">${JSON.stringify(runtime).replace(/</g,'\\u003c')}</script><script>${bridge}</script><style id="native-map-layout">${mapCSS}</style>`));
    }
    const ext=path.extname(name),types={'.html':'text/html','.js':'text/javascript','.css':'text/css','.json':'application/json','.svg':'image/svg+xml','.wasm':'application/wasm','.woff2':'font/woff2'};
    return route.fulfill({status:200,contentType:types[ext]||'application/octet-stream',body});
   }
   await route.continue();
  });
  page.on('pageerror',e=>receipt.errors.push(e.message));
  const get=async url=>{if(url.startsWith(base+'/api/v1/public/industry-map/'))await paced();const r=await page.request.get(url);assert.equal(r.status(),200,url);return r.json()};
  const config=await get(base+'/api/v1/public/config');assert.equal(config.capabilities.map_demo,true);assert.equal(config.capabilities.phone_sms_register,true);assert.equal(config.capabilities.phone_sms_auth,false);assert.equal(config.site.api_url,base+'/api/v1');
  assert.equal((await page.request.get(base+'/api/v1/dev/map-preview/config')).status(),404);
  const ready=async()=>{await page.locator('[data-map-mode=outline] .industry-chart svg path').first().waitFor({state:'attached'});await page.getByRole('button',{name:'复位地图',exact:true}).waitFor()};
  const total=async n=>page.waitForFunction(n=>document.querySelector('.industry-panel-heading b')?.textContent===String(n),n,{timeout:30000});
  const labels={'河北省':'河北','邢台市':'邢台','宁晋县':'宁晋'};
  const node=name=>page.locator('.industry-chart svg text').filter({hasText:new RegExp('^('+name+'|'+labels[name]+')$')}).first();
  for (const [i,code,name] of [[0,'','全国'],[1,'13','河北省'],[2,'1305','邢台市'],[3,'130528','宁晋县']]) {
   if(!i) await page.goto(base+'/regions'); else {await node(name).click();await page.waitForURL(url=>url.searchParams.get('region_code')===code)}
   await ready();const suffix=code?'?region_code='+code:'';
   const stats=await get(base+'/api/v1/public/industry-map/regions'+suffix),vendors=await get(base+'/api/v1/public/industry-map/vendors'+suffix),boundary=await get(base+'/api/v1/public/map-demo/regions'+suffix);
   assert.equal(stats.total,vendors.total);await total(stats.total);assert.equal(boundary.usage,'development-demonstration');
   const local=developmentCache?JSON.parse(fs.readFileSync(path.join(developmentCache,(code||'national')+'.mapping.json'),'utf8')):await get('http://127.0.0.1:8080/api/v1/dev/map-preview/regions'+suffix);assert.equal(boundary.sha256,local.sha256,'exact development geometry hash');assert.deepEqual(boundary.geometry,local.geometry);
   await ready();assert.ok(await page.locator('.industry-chart svg path').count()>0);assert.equal(await page.getByRole('button',{name:'切回区划示意图',exact:true}).count(),0);
   receipt.map.push({code,name,total:stats.total,sha256:boundary.sha256,features:boundary.geometry.features.length,missing_codes:boundary.missing_codes});
   await page.screenshot({path:path.join(dir,'map-'+(code||'national')+'.png')});
  }
  await page.goBack();await page.waitForURL(/region_code=1305/);await ready();await page.goForward();await page.waitForURL(/region_code=130528/);await ready();
  const county=page.url();await page.locator('.industry-vendor .text-link').first().click();await page.waitForURL(/\/v\//);
  if(bundle){const handled=await page.evaluate(()=>Boolean(window.__DLNJ_BACK__&&window.__DLNJ_BACK__()));if(!handled)await page.goBack();}
  else await page.getByRole('link',{name:'返回厂商地图',exact:true}).click();
  await page.waitForURL(county);await ready();receipt.showroomReturn=true;
  await page.goto(base+'/regions');await ready();
  await node('河北省').hover();await page.locator('.industry-chart').getByText('10 家厂商',{exact:true}).waitFor();await page.mouse.move(0,0);
  const original=await node('河北省').boundingBox();await page.getByRole('button',{name:'放大地图',exact:true}).click();await page.waitForFunction(original=>{let e=[...document.querySelectorAll('.industry-chart svg text')].find(e=>e.textContent==='河北');let box=e?.getBoundingClientRect();return box&&(Math.abs(box.x-original.x)>1||Math.abs(box.y-original.y)>1)},original,{timeout:10000});
  const zoomed=await node('河北省').boundingBox(),instance=await page.locator('.industry-chart').getAttribute('_echarts_instance_');
  await page.getByRole('textbox',{name:'关键词',exact:true}).fill('河北春耕');await page.getByRole('button',{name:'搜索',exact:true}).click();await total(1);await ready();assert.equal(await page.locator('.industry-chart').getAttribute('_echarts_instance_'),instance);const filtered=await node('河北省').boundingBox();assert.ok(Math.abs(filtered.x-zoomed.x)<2&&Math.abs(filtered.y-zoomed.y)<2);await page.getByRole('button',{name:'清除筛选',exact:true}).first().click();await total(11);await ready();await page.getByRole('button',{name:'复位地图',exact:true}).click();receipt.zoomAndFilter=true;
  for(const [width,height] of [[1440,900],[768,1024],[390,844],[360,640],[844,390]]){
   await page.setViewportSize({width,height});await page.goto(base+'/regions');await ready();
   assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),'map horizontal fit');
   assert.ok(await page.evaluate(()=>document.documentElement.scrollHeight<=innerHeight+1),'map vertical fit');
   await page.screenshot({path:path.join(dir,`map-${width}x${height}.png`)});receipt.viewports.push({width,height});
  }
  await page.setViewportSize({width:1440,height:900});
  await page.route('**/api/v1/public/map-demo/regions*',r=>r.fulfill({status:502,contentType:'application/json',body:'{}'}));await page.reload();await page.getByText(/地图图形加载失败/).waitFor();assert.equal(await page.locator('[data-map-mode=schematic] svg').count(),0);await page.unroute('**/api/v1/public/map-demo/regions*');await page.getByRole('button',{name:'重试',exact:true}).click();await ready();receipt.mapResourceRetry=true;
  if(bundle)await page.evaluate(()=>window.__DLNJ_TOGGLE_LANGUAGE__());else await page.getByRole('button',{name:'English',exact:true}).click();
  await page.getByRole('heading',{name:'Manufacturer map',exact:true}).waitFor();await page.getByText('Demo boundaries · DataV/Amap (learning and exchange)',{exact:true}).waitFor();receipt.english=true;
  if(bundle)await page.evaluate(()=>window.__DLNJ_TOGGLE_LANGUAGE__());else await page.getByRole('button',{name:'中文',exact:true}).click();
  assert.deepEqual(receipt.errors,[]);receipt.passed=true;
 }finally{await browser.close();fs.mkdirSync('docs/test-output',{recursive:true});fs.writeFileSync(`docs/test-output/${label}-map.json`,JSON.stringify(receipt,null,2))}
 console.log(JSON.stringify(receipt,null,2));
})().catch(e=>{console.error(e.stack||e);process.exitCode=1});
