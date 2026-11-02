// Read-only browser regression for product-page footer sizing. Local servers only.
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const base=process.argv[2]||'http://localhost:18086';
const cdp=process.argv[3],label=cdp?'360':'chrome';
assert.match(base,/^http:\/\/(?:localhost|127\.0\.0\.1):\d+$/);
if(cdp)assert.match(cdp,/^http:\/\/127\.0\.0\.1:\d+$/);
const output=path.resolve('.local/footer-gap',label),results=[],cache=new Map();fs.mkdirSync(output,{recursive:true});
async function record(page,name){
 const value=await page.evaluate(()=>{const footer=document.querySelector('.public-web>.public-footer'),root=document.querySelector('.public-web');const bottom=footer.getBoundingClientRect().bottom+scrollY;return {viewport:innerHeight,docHeight:document.documentElement.scrollHeight,footerBottom:bottom,rootBottom:root.getBoundingClientRect().bottom+scrollY,width:innerWidth,scrollWidth:document.documentElement.scrollWidth,ua:navigator.userAgent}});
 const passed=value.footerBottom>=value.viewport-2&&Math.abs(value.docHeight-value.footerBottom)<=2&&value.scrollWidth<=value.width;
 results.push({name,passed,...value});console.log(name,passed);fs.writeFileSync(path.join(output,'results.json'),JSON.stringify(results,null,2));assert(passed,JSON.stringify(results.at(-1)));
}
(async()=>{
 const browser=cdp?await chromium.connectOverCDP(cdp):await chromium.launch({executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true,args:['--no-proxy-server']});
 // CDP must refer to the separately launched temporary profile, never a personal window.
 const context=cdp?browser.contexts()[0]:await browser.newContext();let scenario='normal',legacy=false;
 await context.route(base+'/api/v1/**',async route=>{
  const request=route.request(),url=new URL(request.url());
  if(request.method()!=='GET'){await route.abort();return;}
  try{
   if(url.pathname==='/api/v1/public/products/23'){
    if(scenario==='pending')await new Promise(resolve=>setTimeout(resolve,1600));
    if(scenario==='error'){await route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:{code:'network_error'}})});return;}
   }
   const key=request.url();if(!cache.has(key)){const response=await route.fetch();cache.set(key,{status:response.status(),headers:response.headers(),body:await response.body()});}
   let response=cache.get(key);
   if(scenario==='short'&&url.pathname==='/api/v1/public/products/23'){
    const data=JSON.parse(response.body);data.item.data={...data.item.data,media_ids:[],description:'',description_en:'',parameters:'',address:''};response={...response,body:Buffer.from(JSON.stringify(data))};
   }
   await route.fulfill(response);
  }catch(error){if(!/closed|canceled|disposed|interrupted/i.test(String(error)))console.log('request interrupted',url.pathname);}
 });
 await context.route(base+'/**/*.css',async route=>{const response=await route.fetch();let css=await response.text();if(legacy)css=css.replace(/\b(\d+)(?:svh|dvh)\b/g,'$1unsupportedvh');await route.fulfill({response,body:css});});
 try{
  for(const [width,height] of [[1440,900],[1880,940],[1152,720],[768,1024],[390,844],[360,800]]){
   for(const state of ['pending','short','error','normal']){
    scenario=state;const page=await context.newPage();await page.setViewportSize({width,height});await page.goto(base+'/products/23',{waitUntil:'domcontentloaded'});await page.locator('.public-footer').waitFor();
    if(state==='pending'){await record(page,`${width}-${height}-loading`);await page.getByRole('heading',{level:1}).waitFor();}
    else if(state==='error'){await page.getByRole('alert').waitFor();}
    else await page.getByRole('heading',{level:1}).waitFor();
    await page.evaluate(()=>window.scrollTo(0,document.documentElement.scrollHeight));await page.waitForTimeout(80);await record(page,`${width}-${height}-${state}`);
    if(state==='normal'){
     const photos=page.locator('.public-product-detail .gallery button,.public-product-detail .product-main-photo');if(await photos.count()){await photos.first().click();await page.getByRole('dialog').waitFor();await page.keyboard.press('Escape');await record(page,`${width}-photo-close`);}
     await page.evaluate(()=>window.scrollTo(0,document.documentElement.scrollHeight));await page.screenshot({path:path.join(output,`product-${width}.png`)});
    }
    await page.close();
   }
  }
  // Emulate rejection of newer viewport units while preserving the real vh fallback.
  legacy=true;scenario='pending';let page=await context.newPage();await page.setViewportSize({width:1440,height:1000});await page.goto(base+'/products/23',{waitUntil:'domcontentloaded'});await page.locator('.public-footer').waitFor();await record(page,'legacy-viewport-units-loading');await page.getByRole('heading',{level:1}).waitFor();await record(page,'legacy-viewport-units-loaded');await page.close();legacy=false;
  scenario='normal';page=await context.newPage();await page.setViewportSize({width:1440,height:900});await page.goto(base+'/products/23');await page.getByRole('heading',{level:1}).waitFor();
  for(const [width,height] of [[1880,940],[390,844],[1440,900]]){await page.evaluate(()=>window.scrollTo(0,document.documentElement.scrollHeight));await page.setViewportSize({width,height});await page.waitForTimeout(100);await record(page,'resize-at-bottom-'+width);}
  await page.locator('.header').getByRole('link',{name:'找产品',exact:true}).click();await page.waitForURL(/\/browse\/products/);await page.locator('a[href*="/products/"]').first().click();await page.waitForURL(/\/products\/\d+/);await page.getByRole('heading',{level:1}).waitFor();await record(page,'list-to-product');await page.goBack();await page.waitForURL(/\/browse\/products/);await page.goForward();await page.getByRole('heading',{level:1}).waitFor();await record(page,'history-forward');await page.reload();await page.getByRole('heading',{level:1}).waitFor();await record(page,'refresh');await page.close();
  console.log(label,results.length,'checks passed');
 }finally{if(cdp){await context.unrouteAll({behavior:'ignoreErrors'});await browser.close();}else await browser.close();}
})().catch(error=>{fs.writeFileSync(path.join(output,'failure.txt'),String(error.stack||error));console.error(error);process.exitCode=1});
