// Isolated fixture only; every provider request is mocked by the Go test server.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
(async()=>{
 const fixture=JSON.parse(fs.readFileSync(process.argv[2],'utf8')),base=fixture.base;
 assert.match(base,/^http:\/\/127\.0\.0\.1:\d+$/);
 const browser=await chromium.launch({headless:true,executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',args:['--no-proxy-server']});
 const receipt={cases:[],errors:[],actual_engine_submissions:0,provider_transport:'isolated_mock'};
 const dir='docs/screenshots/search-engines-20261031';fs.mkdirSync(dir,{recursive:true});
 try{
  const context=await browser.newContext({viewport:{width:1440,height:1000}});
  await context.addInitScript(({token})=>{sessionStorage.setItem('zqzt.admin',token);if(!localStorage.getItem('zqzt.language'))localStorage.setItem('zqzt.language','zh')},{token:fixture.token});
  await context.route('**/*',route=>{const u=new URL(route.request().url());return u.origin===base?route.continue():route.abort()});
  const page=await context.newPage();page.on('pageerror',e=>receipt.errors.push(e.message));
  await page.goto(base+'/admin/seo');await page.getByRole('heading',{name:'引擎配置与自动提交'}).waitFor();
  const baidu=page.locator('.seo-engine-grid article').first();
  assert.equal(await baidu.getByLabel('API Token',{exact:true}).inputValue(),'');
  await baidu.getByLabel('启用接口',{exact:true}).check();await baidu.getByLabel('API Token',{exact:true}).fill('fixture-only-browser-token');
  await page.getByRole('button',{name:'保存配置',exact:true}).click();await page.getByRole('status').filter({hasText:'设置已保存'}).waitFor();
  assert.equal(await baidu.getByLabel('API Token',{exact:true}).inputValue(),'');
  assert.equal(await page.evaluate(()=>JSON.stringify({...sessionStorage,...localStorage}).includes('fixture-only-browser-token')),false);
  const config=await page.request.get(base+'/api/v1/admin/seo/settings',{headers:{Authorization:'Bearer '+fixture.token}});assert.equal(config.status(),200);assert.equal((await config.text()).includes('fixture-only-browser-token'),false);
  await page.getByLabel('规范URL，每行一个，最多500个',{exact:true}).fill(fixture.company_url);await page.getByRole('button',{name:'核验并加入队列',exact:true}).click();await page.getByRole('status').filter({hasText:'入队不代表已发送'}).waitFor();
  await page.getByRole('button',{name:'处理下一任务（可能真实发送）',exact:true}).click();await page.getByRole('status').filter({hasText:'已处理一个任务'}).waitFor();
  await page.locator('.seo-log-list article h3').filter({hasText:'接口已受理'}).waitFor();await page.screenshot({path:path.join(dir,'desktop-logs.png')});
  await page.getByRole('button',{name:'查看发送记录',exact:true}).first().click();await page.locator('.seo-log-list li').filter({hasText:'HTTP 200'}).waitFor();
  for(const [width,height,lang] of [[1440,1000,'zh'],[768,1024,'zh'],[390,844,'zh'],[390,844,'en']]){
   await page.setViewportSize({width,height});await page.evaluate(lang=>localStorage.setItem('zqzt.language',lang),lang);
   await page.reload();await page.getByRole('heading',{name:lang==='en'?'Engines and automatic submission':'引擎配置与自动提交'}).waitFor();
   assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),'horizontal overflow');
   assert.equal(await page.getByLabel('API Token',{exact:true}).inputValue(),'');
   await page.screenshot({path:path.join(dir,`admin-${width}-${lang}.png`)});receipt.cases.push({width,height,language:lang,credentials_blank:true,horizontal_fit:true});
  }
  const nojs=await browser.newContext({javaScriptEnabled:false,viewport:{width:390,height:844}});const publicPage=await nojs.newPage();await publicPage.goto(base+'/v/v-'+fixture.company_url.split('-').pop());
  await publicPage.getByRole('heading',{name:'SEO自动提交验收厂家（隔离数据）'}).waitFor();await publicPage.screenshot({path:path.join(dir,'company-no-js-390.png')});await nojs.close();receipt.no_javascript_company_readable=true;
  assert.deepEqual(receipt.errors,[]);receipt.mock_request_accepted=true;receipt.passed=true;
 }finally{await browser.close();fs.writeFileSync('docs/test-output/search-admin-browser-20261031.json',JSON.stringify(receipt,null,2))}
 console.log(JSON.stringify(receipt));
})().catch(e=>{console.error(e.stack);process.exitCode=1});
