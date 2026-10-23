// Read-only business checks against the existing demo. Authentication uses real
// UI-generated codes; this script never resets data, submits quotes or marks read.
const fs=require('fs'),path=require('path'),assert=require('assert/strict');
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
(async()=>{
 const base='http://localhost:8081',out=path.resolve('docs/screenshots/procurement-return-demo');fs.mkdirSync(out,{recursive:true});
 const config=await(await fetch(base+'/api/v1/public/config')).json();assert(config.capabilities.procurement_demo&&config.capabilities.development_sms);
 const browser=await chromium.launch({headless:true,executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',args:['--no-proxy-server']});
 const errors=[],layouts=[];
 async function login(phone,route){
  const c=await browser.newContext({viewport:{width:1440,height:950}});await c.addInitScript(()=>localStorage.setItem('zqzt.language','zh'));
  const p=await c.newPage();p.on('pageerror',e=>errors.push(e.message));await p.goto(base+route);
  await p.getByRole('button',{name:new RegExp(phone)}).click();await p.getByRole('button',{name:'获取验证码',exact:true}).click();
  const output=p.getByLabel('本机模拟验证码',{exact:true});await output.waitFor();const code=(await output.innerText()).trim();assert(/^\d{6}$/.test(code));
  await p.getByRole('textbox',{name:'短信验证码',exact:true}).fill(code);await p.getByRole('button',{name:'登录',exact:true}).click();
  await p.getByRole('link',{name:'演示：农机与配套件采购（两家报价比较）',exact:true}).waitFor();return p;
 }
 async function shot(p,name,width){await p.setViewportSize({width,height:950});assert(await p.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),name+' overflow');await p.screenshot({path:path.join(out,name+'-'+width+'-zh.png'),fullPage:true});layouts.push({name,width})}
 try{
  const buyer=await login('13900009001','/buyer'),a=await login('13900009002','/merchant/opportunities'),b=await login('13900009003','/merchant/opportunities');
  assert.equal(await buyer.locator('.workspace-sidebar nav>a').count(),5);assert.equal(await a.locator('.procurement-workspace').count(),0);
  await buyer.getByRole('link',{name:'演示：农机与配套件采购（两家报价比较）',exact:true}).click();await buyer.locator('.proc-compare').waitFor({state:'attached'});
  await b.getByRole('link',{name:'演示：农机与配套件采购（两家报价比较）',exact:true}).click();await b.locator('.proc-quote-history').waitFor();
  const project=new URL(b.url()).pathname.split('/').pop();await b.goto(base+'/buyer/opportunities/projects/'+project+'?view=history#quote');await b.locator('.proc-quote-form').waitFor();assert.equal(new URL(b.url()).pathname,'/merchant/opportunities/projects/'+project);assert.equal(new URL(b.url()).search,'?view=history');assert.equal(new URL(b.url()).hash,'#quote');
  for(const width of [360,390,768,1440]){await shot(buyer,'buyer-comparison',width);await shot(b,'supplier-quote',width);await a.goto(base+'/procurement');await a.locator('.proc-hall-filters').waitFor();await shot(a,'request-hall',width);assert.equal(Math.round(await a.locator('.proc-filter-actions>.button').first().evaluate(n=>n.getBoundingClientRect().height)),44)}
  await b.getByRole('button',{name:'切换体验账号',exact:true}).click();await b.getByRole('heading',{name:'厂商登录',exact:true}).waitFor();assert.equal(await b.evaluate(()=>sessionStorage.getItem('zqzt.merchant')),null);await shot(b,'supplier-login',390);
  assert.deepEqual(errors,[]);const result={checks:['Three dynamic-code UI logins','Buyer/supplier workspaces and old links','Read-only snapshots; no reset, quotes, intent or message mutations','Normal supplier logout and login continuation'],layouts,errors,businessRecordsWritten:false,datasetReset:false,realSMS:false};
  fs.writeFileSync('docs/test-output/procurement-return-demo-browser.json',JSON.stringify(result,null,2));console.log(JSON.stringify({layouts:layouts.length,errors,datasetReset:false}));
 }finally{await browser.close()}
})().catch(e=>{console.error(e);process.exit(1)});
