// Mutations are permitted only against the explicitly isolated restored candidate.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
(async()=>{
 const f=JSON.parse(fs.readFileSync(0,'utf8'));assert.equal(f.isolated,true);assert.equal(new URL(f.base).hostname,'127.0.0.1');
 const output=process.argv[2],checks=[],errors=[];
 const browser=await chromium.launch({headless:true,executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',args:['--no-proxy-server']});
 try{for(const width of [1440,390]){
  const context=await browser.newContext({viewport:{width,height:1000},locale:'zh-CN'});
  await context.addInitScript(token=>{sessionStorage.setItem('zqzt.admin',token);localStorage.setItem('zqzt.language','zh');localStorage.setItem('zqzt.privacy.analytics','no')},f.admin.token);
  const localConfig=c=>({...c,site:{...c.site,api_url:f.base+'/api/v1',media_url:f.base+'/api/v1/media'}});
  await context.route('**/*',async route=>{
   const req=route.request(),u=new URL(req.url());if(u.origin!==f.base)return route.abort();
   if(req.isNavigationRequest()){
    const r=await route.fetch(),headers={...r.headers()};delete headers['content-encoding'];delete headers['content-length'];
    const html=(await r.text()).replace(/(<script id="zqzg-runtime-config" type="application\/json">)([^<]+)(<\/script>)/,(_,a,raw,b)=>a+JSON.stringify(localConfig(JSON.parse(raw)))+b);
    return route.fulfill({response:r,headers,body:html});
   }
   if(u.pathname==='/api/v1/public/config'){const r=await route.fetch();return route.fulfill({response:r,json:localConfig(await r.json())})}
   return route.continue();
  });
  const page=await context.newPage();page.on('pageerror',e=>errors.push(e.message));
  await page.goto(f.base+'/admin/settings',{waitUntil:'domcontentloaded'});
  await page.getByRole('tab',{name:'上传与容量',exact:true}).click();
  const panel=page.getByRole('tabpanel'),input=panel.getByRole('spinbutton',{name:/厂商容量额度/});
  await input.waitFor();const original=await input.inputValue();
  for(const value of ['300',original]){
   await input.fill(value);
   const done=page.waitForResponse(r=>r.url()===f.base+'/api/v1/admin/settings'&&r.request().method()==='PUT');
   await panel.getByRole('button',{name:'保存',exact:true}).click();const response=await done;
   assert.equal(response.status(),200);assert.equal((await response.json()).config.uploads.default_quota_bytes,Number(value)*1e6);
   await page.getByRole('status').filter({hasText:'设置已保存'}).waitFor();
   await page.reload({waitUntil:'domcontentloaded'});await page.getByRole('tab',{name:'上传与容量',exact:true}).click();
   await input.waitFor();assert.equal(await input.inputValue(),value);
  }
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
  const screen=path.join(path.dirname(output),'quota-'+width+'.png');await page.screenshot({path:screen,fullPage:true});
  checks.push({width,saved300MB:true,refreshPersisted:true,originalQuotaRestored:true,overflow:false});await context.close();
 }assert.deepEqual(errors,[]);fs.writeFileSync(output,JSON.stringify({isolatedProductionCandidate:true,productionQuotaChanged:false,checks,errors},null,2)+'\n');console.log('Quota settings UI: '+checks.length+' checks passed')}
 finally{await browser.close()}
})().catch(e=>{console.error(e.message);process.exit(1)});
