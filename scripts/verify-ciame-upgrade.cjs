const fs=require('fs'),assert=require('assert/strict');
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
(async()=>{
 const f=JSON.parse(fs.readFileSync('.local/ciame-browser-fixture.json','utf8'));
 const out='docs/screenshots/ciame-1.9.0';fs.mkdirSync(out,{recursive:true});fs.mkdirSync('docs/test-output',{recursive:true});
 const browser=await chromium.launch({headless:true,executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',args:['--no-proxy-server']});
 const checks=[],layouts=[],errors=[];let page;
 async function context(width=390,lang='zh') {const c=await browser.newContext({viewport:{width,height:900}});await c.addInitScript(l=>localStorage.setItem('zqzt.language',l),lang);await c.route('**/api/v1/public/analytics/events',r=>r.fulfill({status:202,contentType:'application/json',body:'{}'}));return c;}
 async function api(path,method='GET',token='',data){const r=await fetch(f.base+'/api/v1'+path,{method,headers:{'Content-Type':'application/json',...(token?{Authorization:'Bearer '+token}:{})},...(data?{body:JSON.stringify(data)}:{})});if(!r.ok)throw Error(`${method} ${path}: ${r.status} ${await r.text()}`);return r.json();}
 try {
  const c=await context();page=await c.newPage();page.on('pageerror',e=>errors.push(e.message));page.on('dialog',d=>d.accept());
  await page.goto(f.base+'/');await page.getByRole('heading',{name:'中企展馆-农业机械厅',exact:true}).waitFor();assert.equal(await page.locator('.industry-hall-grid').count(),0);assert.equal(await page.getByText('采购中心',{exact:true}).count(),0);await page.screenshot({path:out+'/home-mobile.png',fullPage:true});
  checks.push('Homepage opens agriculture directly with matching website identity');
  await page.goto(f.base+'/join');await page.getByLabel('公司登录账号',{exact:true}).fill('ciame-browser-'+Date.now());await page.getByLabel('设置密码',{exact:true}).fill('CIAME-Browser-2026!');await page.getByLabel('确认密码',{exact:true}).fill('CIAME-Browser-2026!');await page.getByRole('button',{name:'注册并继续',exact:true}).click();await page.waitForURL('**/merchant/booth/edit');
  await page.getByLabel('企业 / 工厂名称',{exact:true}).fill('新注册CIAME验收企业（隔离测试）');await page.getByLabel('主营产品',{exact:true}).fill('耕整地机械');await page.getByLabel('电话',{exact:true}).fill('0319-1234567');
  await page.getByLabel('省／直辖市／自治区',{exact:true}).selectOption('13');await page.getByLabel('市／州／盟',{exact:true}).selectOption('1305');await page.getByLabel('区／县（选填）',{exact:true}).selectOption('130528');
  const primary=page.getByRole('combobox',{name:'主营分类',exact:true});await primary.selectOption({label:'耕整地机械及零部件 / 耕地机械'});
  await page.getByRole('button',{name:'保存云端草稿',exact:true}).click();await page.getByText('企业资料已保存到云端草稿',{exact:true}).waitFor();
  await page.getByRole('button',{name:'预览官网',exact:true}).click();await page.locator('.company-website-preview .sr-home-hero').waitFor();await page.screenshot({path:out+'/website-draft.png',fullPage:true});await page.getByRole('dialog').getByRole('button',{name:'关闭',exact:true}).click();
  const token=await page.evaluate(()=>sessionStorage.getItem('zqzt.merchant'));
  const draft=(await api('/merchant/contents?kind=vendor&page_size=1','GET',token)).items[0];
  await page.getByRole('button',{name:'提交审核',exact:true}).click();await page.waitForURL('**/merchant/booth');
  const submitted=await api('/merchant/contents/'+draft.id,'GET',token);
  await api('/admin/revisions/'+submitted.revision_id+'/review','POST',f.admin_token,{decision:'approve'});
  const publicVendor=await api('/public/showrooms/v-'+draft.id);assert.equal(publicVendor.item.data.name,draft.data.name);checks.push('Company registration → minimal CIAME profile → draft → shared-template preview → review → public website');
  await page.goto(f.base+'/v/v-'+draft.id);await page.getByRole('heading',{name:draft.data.name,exact:true}).waitFor();assert.equal(await page.getByRole('tab',{name:'视频展示',exact:true}).count(),0);assert.equal(await page.getByRole('link',{name:'冀ICP备2026047360号-2',exact:true}).count(),1);await page.screenshot({path:out+'/website-public.png',fullPage:true});
  for(const path of ['/buyer','/procurement/1','/merchant/opportunities','/admin/procurement/reviews']){await page.goto(f.base+path);await page.getByRole('heading',{name:'功能暂时暂停',exact:true}).waitFor();}
  checks.push('Historical procurement routes show pause; no active links; ICP registration is visible');
  for(const width of [360,390,768,1440])for(const lang of ['zh','en']){
   const ctx=await context(width,lang);const p=await ctx.newPage();p.on('pageerror',e=>errors.push(e.message));
   for(const path of ['/','/?hall=agriculture','/browse','/v/v-'+f.vendor_id,'/join']){await p.goto(f.base+path);await p.locator('h1').first().waitFor();await p.waitForTimeout(180);const overflow=await p.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1);assert.equal(overflow,false,`${width} ${lang} ${path}`);layouts.push({width,lang,path,overflow});}
   if(lang==='zh'){await p.goto(f.base+'/?hall=agriculture');await p.getByRole('heading',{name:'中企展馆-农业机械厅',exact:true}).waitFor();await p.screenshot({path:out+`/hall-${width}.png`,fullPage:true});}
   await ctx.close();
  }
  assert.equal(errors.length,0);fs.writeFileSync('docs/test-output/ciame-1.9.0-browser.json',JSON.stringify({checks,layouts,errors,isolatedDatabase:true,productionWrites:false,realDeviceVerified:false},null,2));console.log(JSON.stringify({checks:checks.length,layouts:layouts.length,errors}));
 } catch(e){if(page&&!page.isClosed())await page.screenshot({path:out+'/failure.png',fullPage:true});throw e;}finally{await browser.close();}
})().catch(e=>{console.error(e);process.exit(1)});
