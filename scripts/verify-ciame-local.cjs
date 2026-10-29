const fs=require('fs'),assert=require('assert/strict');
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
(async()=>{
 const base='http://localhost:8080';const browser=await chromium.launch({headless:true,executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',args:['--no-proxy-server']});
 const errors=[],checks=[];
 try {
  const c=await browser.newContext({viewport:{width:390,height:844}});await c.route('**/api/v1/public/analytics/events',r=>r.fulfill({status:202,contentType:'application/json',body:'{}'}));const p=await c.newPage();p.on('pageerror',e=>errors.push(e.message));
  await p.goto(base+'/',{waitUntil:'domcontentloaded'});await p.getByRole('heading',{name:'中企展馆-农业机械厅',exact:true}).waitFor();assert.equal(await p.locator('.industry-hall-grid').count(),0);checks.push('Local homepage opens agriculture directly');
  assert.equal(await p.getByText('采购中心',{exact:true}).count(),0);checks.push('Local industry hall identity and hidden procurement');
  await p.goto(base+'/browse',{waitUntil:'domcontentloaded'});await p.getByRole('combobox',{name:'厂家分类',exact:true}).waitFor();await p.waitForFunction(()=>document.querySelectorAll('select')[0]?.options.length===189);checks.push('188 selectable CIAME entries');
  await p.goto(base+'/merchant/login',{waitUntil:'domcontentloaded'});await p.getByLabel('公司登录账号',{exact:true}).waitFor();assert.equal(await p.getByRole('link',{name:'冀ICP备2026047360号-2',exact:true}).count(),1);checks.push('Company account entry and ICP registration');
  await p.goto(base+'/procurement',{waitUntil:'domcontentloaded'});await p.getByRole('heading',{name:'功能暂时暂停',exact:true}).waitFor();const response=await p.request.get(base+'/api/v1/public/procurements');assert.equal(response.status(),503);checks.push('Paused historical page and API');
  assert.equal(errors.length,0);fs.mkdirSync('docs/test-output',{recursive:true});fs.writeFileSync('docs/test-output/ciame-1.9.0-local.json',JSON.stringify({base,checks,errors,readOnly:true,productionWrites:false},null,2));console.log(JSON.stringify({checks:checks.length,errors}));
 } finally {await browser.close();}
})().catch(e=>{console.error(e);process.exit(1)});
