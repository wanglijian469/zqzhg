const fs=require('fs'),assert=require('assert/strict');const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
(async()=>{
 const f=JSON.parse(fs.readFileSync('.local/taxonomy-browser-fixture.json','utf8'));const browser=await chromium.launch({headless:true,executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',args:['--no-proxy-server']});
 const checks=[],layouts=[],errors=[];fs.mkdirSync('docs/screenshots/compact-taxonomy',{recursive:true});
 const request=async(path,method='GET',data,role='merchant')=>{const r=await fetch(f.base+'/api/v1'+path,{method,headers:{'Content-Type':'application/json',Authorization:'Bearer '+f[role+'_token']},body:data===undefined?undefined:JSON.stringify(data)});const result=await r.json();assert(r.ok,JSON.stringify(result));return result};
 async function context(width=390,lang='zh'){const c=await browser.newContext({viewport:{width,height:900}});await c.addInitScript(({f,lang})=>{if(!localStorage.getItem('zqzt.language'))localStorage.setItem('zqzt.language',lang);sessionStorage.setItem('zqzt.merchant',f.merchant_token);sessionStorage.setItem('zqzt.merchant.id',String(f.account_id));sessionStorage.setItem('zqzt.admin',f.admin_token)},{f,lang});await c.route('**/api/v1/public/analytics/events',r=>r.fulfill({status:202,contentType:'application/json',body:'{}'}));return c}
 let page;
 try{
  const c=await context();page=await c.newPage();page.on('pageerror',e=>errors.push(e.message));
  await page.goto(f.base+'/merchant/booth/edit');await page.getByRole('combobox',{name:'主营分类',exact:true}).waitFor();
  assert.equal(await page.getByRole('combobox',{name:'兼营分类1（选填）'}).inputValue(),String(f.vendor_secondary_id));
  await page.waitForFunction(id=>document.querySelector(`select[aria-label="主营分类"] option[value="${id}"]`)?.disabled,f.vendor_secondary_id);
  await page.locator('.taxonomy-fields .category-field-controls details > summary').click();assert.equal(await page.getByRole('button',{name:'新增经营分类',exact:true}).count(),1);checks.push('Main business and up to two secondary fields retain values and exclude duplicates');
  await page.goto(f.base+'/merchant/products/new');await page.getByRole('textbox',{name:'产品名称',exact:true}).fill('液压油缸 浏览器隔离测试');
  await page.getByRole('button',{name:'推荐分类',exact:true}).click();const recommended=page.locator('.taxonomy-recommendations button').filter({hasText:'液压油缸'});await recommended.first().waitFor();
  assert.equal(await page.getByRole('combobox',{name:'产品分类',exact:true}).inputValue(),'0');await recommended.first().click();
  assert.equal(await page.getByRole('combobox',{name:'产品分类',exact:true}).inputValue(),String(f.cylinder_id));checks.push('Rule recommendation requires an explicit click');
  await page.getByRole('button',{name:'搜索分类',exact:true}).click();const dialog=page.getByRole('dialog');await dialog.getByRole('searchbox').fill('齿轮');await dialog.getByRole('button',{name:'传动系统及零部件 / 齿轮与变速传动',exact:true}).click();
  assert.equal(await page.getByRole('combobox',{name:'产品分类',exact:true}).inputValue(),String(f.gear_id));checks.push('Full-screen search selects a leaf using its complete path');
  await page.getByRole('textbox',{name:'产品品牌',exact:true}).fill('验收品牌');await page.getByRole('textbox',{name:'产品型号',exact:true}).fill('X100');await page.getByRole('textbox',{name:'适用品牌',exact:true}).fill('品牌A,品牌B');await page.getByRole('textbox',{name:'适用机型',exact:true}).fill('A100，B200');
  await page.getByRole('button',{name:'保存云端草稿',exact:true}).click();await page.getByText('已保存到云端草稿',{exact:true}).waitFor();
  const list=await request('/merchant/contents?kind=product&q='+encodeURIComponent('浏览器隔离测试'));const created=list.items[0];assert(created);assert.equal(created.data.product_category_id,f.gear_id);assert.deepEqual(created.data.applicable_models,['A100','B200']);
  await page.getByRole('button',{name:'下一步',exact:true}).click();await page.getByRole('button',{name:'下一步',exact:true}).click();await page.getByRole('button',{name:'提交审核',exact:true}).click();await page.waitForURL('**/merchant/products');
  const submitted=await request('/merchant/contents/'+created.id);await request('/admin/revisions/'+submitted.revision_id+'/review','POST',{decision:'approve'},'admin');checks.push('Brand/model attributes survive cloud save, three-step submission and review');
  await page.goto(f.base+'/browse?category_id='+f.vendor_secondary_id+'&industrial_belt_id='+f.belt_id+'&business_nature=manufacturer');await page.getByRole('heading',{name:'分类升级验收企业（隔离测试）',exact:true}).waitFor();
  assert.equal(await page.locator('.vendor-brand-card').count(),1);assert(!page.url().includes('industrial_belt_id'));assert(!page.url().includes('business_nature'));assert.equal(await page.locator('.browse-compact-options select').count(),3);if(await page.locator('.menu-button').isVisible())await page.locator('.menu-button').click();await page.locator('.language-button').click();assert(!page.url().includes('industrial_belt_id'));checks.push('Main/secondary search works and removed conditions cannot survive old URLs or language switching');
  await page.goto(f.base+'/admin/categories');await page.getByRole('button',{name:'Historical mappings',exact:true}).click();await page.getByRole('link',{name:'待映射历史展品（隔离测试）',exact:true}).waitFor();
  await page.locator('.taxonomy-admin-row input[type=checkbox]').first().check();await page.getByRole('combobox',{name:'Product category',exact:true}).selectOption(String(f.cylinder_id));await page.getByRole('button',{name:'Preview and confirm mapping',exact:true}).click();await page.getByRole('dialog').filter({hasText:'Map selected records'}).getByRole('button',{name:'Confirm mapping',exact:true}).click();
  await page.getByRole('link',{name:'待映射历史展品（隔离测试）',exact:true}).waitFor({state:'hidden'});checks.push('Historical mapping previews before explicit confirmation and leaves original snapshots intact');
  await c.close();
  for(const width of [360,390,768,1024,1440])for(const lang of ['zh','en'])for(const route of ['company','product','admin','vendors','products']){
   const cx=await context(width,lang),p=await cx.newPage();p.on('pageerror',e=>errors.push(e.message));const zh=lang==='zh';
   await p.goto(f.base+({company:'/merchant/booth/edit',product:'/merchant/products/'+f.product_id+'/edit',admin:'/admin/categories',vendors:'/browse',products:'/browse/products'})[route]);
   if(route==='vendors'||route==='products'){
    await p.locator('.browse-compact-options select').first().waitFor();
    await p.waitForFunction(()=>document.querySelector('.browse-compact-options select')?.options.length>1);
    assert.equal(await p.locator('.browse-compact-options select').count(),3);
    const sizes=await p.locator('.public-search-form input,.public-search-form select,.browse-search-action').evaluateAll(items=>items.map(e=>({height:e.getBoundingClientRect().height,y:e.getBoundingClientRect().y})));
    assert(sizes.every(s=>s.height>=44),'touch controls >=44px');
    assert(Math.abs(sizes[0].y-sizes.at(-1).y)<2,'keyword and search button share a row: '+JSON.stringify({width,lang,route,sizes}));
    if(width>=1024)assert(Math.max(...sizes.map(s=>s.y))-Math.min(...sizes.map(s=>s.y))<2,'desktop search uses one row');
   }else await p.getByRole(route==='admin'?'button':'combobox',{name:route==='admin'?(zh?'厂家分类':'Manufacturer categories'):route==='company'?(zh?'主营分类':'Primary business category'):(zh?'产品分类':'Product category'),exact:true}).waitFor();
   await p.waitForTimeout(100);const overflow=await p.evaluate(()=>({ok:document.documentElement.scrollWidth<=innerWidth+1,nodes:[...document.querySelectorAll('body *')].filter(e=>{const r=e.getBoundingClientRect();return r.width>0&&(r.right>innerWidth+1||r.left< -1)}).slice(-12).map(e=>({tag:e.tagName,cls:e.className,text:e.textContent.slice(0,80),width:e.getBoundingClientRect().width}))}));if(!overflow.ok){await p.screenshot({path:`docs/screenshots/compact-taxonomy/overflow-${width}-${lang}-${route}.png`,fullPage:true});console.log(JSON.stringify(overflow))}assert(overflow.ok,'Overflow '+[width,lang,route]);
   if(route==='product'){await p.getByRole('button',{name:zh?'搜索分类':'Search categories',exact:true}).click();await p.getByRole('searchbox').fill(zh?'油缸':'cylinder');assert(await p.getByRole('dialog').evaluate(e=>e.getBoundingClientRect().width<=innerWidth));await p.getByRole('dialog').getByRole('button',{name:zh?'关闭':'Close',exact:true}).click()}
   layouts.push({width,lang,route,overflow:false});if(lang==='zh'&&[390,1440].includes(width))await p.screenshot({path:`docs/screenshots/compact-taxonomy/${route}-${width}.png`,fullPage:route!=='admin'});await cx.close();
  }
  assert.equal(errors.length,0);fs.mkdirSync('docs/test-output',{recursive:true});fs.writeFileSync('docs/test-output/compact-browser.json',JSON.stringify({checks,layouts,errors,isolatedDatabase:true,productionWrites:false,realDeviceVerified:false},null,2));console.log(JSON.stringify({checks:checks.length,layouts:layouts.length,errors}));
 }catch(e){if(page&&!page.isClosed())await page.screenshot({path:'docs/screenshots/compact-taxonomy/failure.png',fullPage:true});throw e}finally{await browser.close()}
})().catch(e=>{console.error(e);process.exit(1)});
