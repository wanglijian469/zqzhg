// Read-only UI verification against an isolated production candidate, never real brochure data.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
(async()=>{
 const fixture=JSON.parse(fs.readFileSync(0,'utf8'));assert.equal(fixture.isolated,true);
 const browser=await chromium.launch({headless:true,executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',args:['--no-proxy-server']});
 const output=process.argv[2],errors=[],checks=[],screens=path.join(path.dirname(output),'ai-screenshots');fs.mkdirSync(screens,{recursive:true});
 try {for(const width of [390,1440]){
  for(const role of ['admin','merchant']){
   const context=await browser.newContext({viewport:{width,height:1000},locale:'zh-CN'});
   await context.addInitScript(f=>{sessionStorage.setItem('zqzt.'+f.role,f.token);if(f.role==='merchant'){sessionStorage.setItem('zqzt.merchant.id',String(f.account));sessionStorage.setItem('zqzt.merchant.role','merchant')}localStorage.setItem('zqzt.language','zh')},{role,token:fixture[role].token,account:fixture[role].id});
   await context.route(fixture.base+'/**',async route=>{const req=route.request();if(req.isNavigationRequest()){
    const response=await route.fetch();if(response.ok()&&response.headers()['content-type']?.includes('text/html')){
     const html=(await response.text()).replace(/(<script id="zqzg-runtime-config" type="application\/json">)([^<]+)(<\/script>)/,(_,a,raw,b)=>{const c=JSON.parse(raw);c.site.api_url=fixture.base+'/api/v1';c.site.media_url=fixture.base+'/api/v1/media';return a+JSON.stringify(c)+b});
     const headers={...response.headers()};delete headers['content-encoding'];delete headers['content-length'];return route.fulfill({response,headers,body:html});
    }}return route.continue();});
   const page=await context.newPage();page.on('pageerror',e=>errors.push(e.message));
   const url=role==='admin'?'/admin/settings?tab=ai':'/merchant/ai-import';await page.goto(fixture.base+url,{waitUntil:'domcontentloaded'});
   if(role==='admin'){
    const panel=page.locator('.ai-model-settings');await panel.getByRole('heading',{name:'AI模型配置 · 阿里云百炼'}).waitFor();assert.equal(await panel.getByLabel('默认模型',{exact:false}).inputValue(),'qwen3.8-flash');
    await panel.getByRole('combobox',{name:/^测试模型/}).selectOption('max');assert(await panel.getByRole('button',{name:'测试已保存的连接',exact:true}).isDisabled());
    await panel.getByRole('checkbox',{name:'我明确选择max，并确认费用较高',exact:true}).check();assert(!(await panel.getByRole('button',{name:'测试已保存的连接',exact:true}).isDisabled()));
   }else{
    await page.getByRole('heading',{name:'AI智能建馆',exact:true}).waitFor();const model=page.locator('.ai-import select').first();await model.waitFor();assert.equal(await model.inputValue(),'flash');
   }
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));await page.screenshot({path:path.join(screens,role+'-'+width+'.png'),fullPage:true});
   checks.push({role,width,loaded:true,flash:true,overflow:false,paidModelTestExecuted:false});await context.close();
  }
 }assert.deepEqual(errors,[]);fs.writeFileSync(output,JSON.stringify({isolatedProductionCandidate:true,readOnly:true,checks,errors},null,2)+'\n');console.log('AI import candidate UI: '+checks.length+' checks passed');}
 finally{await browser.close()}
})().catch(e=>{console.error(e.message);process.exit(1)});
