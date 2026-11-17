const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
const [base,output,assetRoot]=process.argv.slice(2),reference=path.resolve(__dirname,'../.local/map-preview');
const evidence={base,embeddedAPKAssets:!!assetRoot,map:[],layouts:[],mobileRetry:false,pageErrors:[],httpErrors:[]};
const bundle=assetRoot&&path.resolve(assetRoot,'workspace');
let next=Date.now()+20000,queue=Promise.resolve();
function pace(url){if(!/^\/(regions|browse|api\/v1\/public\/(industry-map|products|showrooms))/.test(new URL(url).pathname))return Promise.resolve();queue=queue.then(async()=>{const delay=next-Date.now();if(delay>0)await new Promise(r=>setTimeout(r,delay));next=Date.now()+3300});return queue;}
async function main(){
 const browser=await chromium.launch({headless:true,executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',args:['--no-proxy-server']});
 const config=await fetch(base+'/api/v1/public/config').then(r=>{assert(r.ok);return r.json()});
 const screenDir=path.resolve(path.dirname(output),'screenshots');fs.mkdirSync(screenDir,{recursive:true});
 try{for(const width of [1440,1024,768,390,360]){
  const context=await browser.newContext({viewport:{width,height:900},locale:'zh-CN'});
  await context.route(base+'/**',async route=>{
   const req=route.request(),u=new URL(req.url());await pace(u.href);
   if(bundle&&req.method()==='GET'&&!u.pathname.startsWith('/api/')){
    const spa=req.isNavigationRequest()&&!path.extname(u.pathname);const file=path.resolve(bundle,spa?'index.html':u.pathname.slice(1));
    if(file.startsWith(bundle+path.sep)&&fs.existsSync(file)&&fs.statSync(file).isFile()){
     let body=fs.readFileSync(file);if(spa){const runtime={...config,site:{...config.site,web_url:base,api_url:base+'/api/v1',media_url:base+'/api/v1/media'}};
      const bridge=fs.readFileSync(path.join(bundle,'../native-workspace.js'),'utf8').replace('__DLNJ_BOOTSTRAP__',JSON.stringify({token:'',account:{},language:'zh',nonce:'release-acceptance'}));
      body=Buffer.from(body.toString().replace('<head>',`<head><script id="zqzg-runtime-config" type="application/json">${JSON.stringify(runtime)}</script><script>${bridge}</script><style>${fs.readFileSync(path.join(bundle,'../native-map.css'),'utf8')}</style>`));}
     return route.fulfill({status:200,contentType:({'.html':'text/html','.js':'text/javascript','.css':'text/css','.json':'application/json','.svg':'image/svg+xml'})[path.extname(file)]||'application/octet-stream',body});
    }
   }
   if(req.isNavigationRequest()&&base.startsWith('http:')){
    const response=await route.fetch();if(response.ok()&&response.headers()['content-type']?.includes('text/html')){
     const html=(await response.text()).replace(/(<script id="zqzg-runtime-config" type="application\/json">)([^<]+)(<\/script>)/,(_,a,raw,b)=>{const c=JSON.parse(raw);c.site.api_url=base+'/api/v1';c.site.media_url=base+'/api/v1/media';return a+JSON.stringify(c)+b});
     const headers={...response.headers()};delete headers['content-encoding'];delete headers['content-length'];return route.fulfill({response,headers,body:html});
    }
   }
   return route.continue();
  });
  const page=await context.newPage();page.setDefaultNavigationTimeout(90000);page.on('pageerror',e=>evidence.pageErrors.push(e.message));page.on('response',r=>{if(r.url().startsWith(base)&&r.status()>=400){const row={url:r.url(),status:r.status()};evidence.httpErrors.push(row);console.log(JSON.stringify(row));}});
  const ready=async()=>{
   await page.locator('[data-map-mode=outline] .industry-chart svg path').first().waitFor({state:'attached',timeout:45000});
   await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
   await page.waitForFunction(()=>!document.querySelector('.industry-map-page .loading')&&document.querySelector('[data-map-mode=outline] .industry-map-controls'),{},{timeout:45000});
  };
  await page.goto(base+'/regions',{waitUntil:'domcontentloaded'});await ready();
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
  if(width===1440){for(const [code,label] of [['','全国'],['13','河北'],['1305','邢台'],['130528','宁晋']]){
   if(code){await page.locator('.industry-chart svg text').filter({hasText:new RegExp('^'+label+'[省市县]?$')}).first().click();await page.waitForURL(u=>u.searchParams.get('region_code')===code);await ready();}
   const query=code?'?region_code='+code:'';
   const [boundary,stats,vendors]=await page.evaluate(async q=>Promise.all(['map-demo/regions','industry-map/regions','industry-map/vendors'].map(async endpoint=>{const r=await fetch('/api/v1/public/'+endpoint+q);if(!r.ok)throw Error(endpoint+' '+r.status);return r.json()})),query);
   const expected=JSON.parse(fs.readFileSync(path.join(reference,(code||'national')+'.mapping.json'),'utf8'));
   assert.equal(boundary.sha256,expected.sha256);assert.deepEqual(boundary.geometry,expected.geometry);assert.equal(stats.total,vendors.total);
   await page.waitForFunction(n=>document.querySelector('.industry-panel-heading b')?.textContent===String(n),stats.total);
   evidence.map.push({code,count:stats.total,sha256:boundary.sha256});
  }await page.goBack();await page.waitForURL(u=>u.searchParams.get('region_code')==='1305');await ready();}
  if(width===390){const glob='**/api/v1/public/map-demo/regions*',fail=r=>r.fulfill({status:503,body:'{}'});await page.route(glob,fail);await page.reload({waitUntil:'domcontentloaded'});const retry=page.getByRole('button',{name:'重试',exact:true});await retry.waitFor({timeout:45000});assert.equal(await page.locator('[data-map-mode=schematic] svg').count(),0);await page.unroute(glob,fail);await retry.click();await ready();evidence.mobileRetry=true;}
  await page.screenshot({path:path.join(screenDir,(bundle?'apk-':'')+'map-'+width+'.png'),fullPage:true});
  if(width===1440||width===390){
   const data=await page.evaluate(async()=>{const r=await fetch('/api/v1/public/products?page_size=24');if(!r.ok)throw Error('products '+r.status);return r.json()});
   const product=data.items.find(p=>p.data.vendor_product_category_id);assert(product);
   const id=product.data.vendor_product_category_id,vid=product.vendor_id;
   const showroom=product.vendor?.showroom_slug||('v-'+vid),query='?tab=products&vendor_product_category_id='+id;
   console.log('Checking company '+JSON.stringify({vid,showroom,category:id}));await page.goto(base+'/v/'+showroom+query);await page.locator('#panel-products .showroom-product-grid article').first().waitFor({timeout:45000});
   assert.equal(await page.locator('#panel-products select').first().inputValue(),String(id));
   await page.locator('#panel-products .showroom-product-grid a[href*="/products/"]').first().click();await page.locator('.public-detail-breadcrumb').waitFor();assert.equal(await page.locator('.public-detail-breadcrumb li').count(),2);
   assert((await page.locator('.taxonomy-public-attributes').innerText()).includes(product.vendor_product_category.name));
   await page.screenshot({path:path.join(screenDir,(bundle?'apk-':'')+'product-'+width+'.png'),fullPage:true});
   await page.locator('.public-detail-breadcrumb a').click();await page.locator('#panel-products').waitFor();assert.equal(new URL(page.url()).searchParams.get('vendor_product_category_id'),String(id));
  }
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));evidence.layouts.push({width,outline:true,overflow:false});await context.close();console.log('Verified width '+width);
 }
 assert.equal(evidence.map.length,4);assert(evidence.mobileRetry);assert.deepEqual(evidence.pageErrors,[]);
 // Only the deliberately injected mobile boundary failure is expected.
 assert.deepEqual(evidence.httpErrors.filter(r=>!(r.status===503&&new URL(r.url).pathname==='/api/v1/public/map-demo/regions')),[]);
 evidence.passed=true;
 }finally{await browser.close();fs.writeFileSync(output,JSON.stringify(evidence,null,2)+'\n');}
}
main().catch(e=>{console.error(e);process.exitCode=1});
