const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
(async()=>{
 const fixture=JSON.parse(fs.readFileSync(0,'utf8'));const out=path.resolve(__dirname,'../docs/screenshots/vendor-product-categories-20261115');fs.mkdirSync(out,{recursive:true});
 const browser=await chromium.launch({headless:true,executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',args:['--no-proxy-server']});const errors=[],checks=[];let lastPage;
 async function overflow(page){const size=await page.evaluate(()=>({width:innerWidth,total:document.documentElement.scrollWidth}));assert(size.total<=size.width+1,JSON.stringify(size));}
 try{
  for(const width of [360,390,768,1024,1440])for(const lang of ['zh','en']){
   const context=await browser.newContext({viewport:{width,height:950}});await context.addInitScript(({token,account,lang})=>{sessionStorage.setItem('zqzt.merchant',token);sessionStorage.setItem('zqzt.merchant.id',String(account));sessionStorage.setItem('zqzt.merchant.role','merchant');localStorage.setItem('zqzt.language',lang)},{token:fixture.owner,account:fixture.account,lang});
   const page=await context.newPage();lastPage=page;page.on('pageerror',e=>errors.push(e.message));
   await page.goto(fixture.base+'/merchant/products/new');await page.locator('.vendor-category-picker select:not(:disabled)').waitFor();
   const skip=page.getByRole('button',{name:lang==='zh'?'跳过':'Skip',exact:true});if(await skip.isVisible())await skip.click();
   assert.equal(await page.getByRole('button',{name:lang==='zh'?'推荐分类':'Recommend categories',exact:true}).count(),0);
   await page.getByRole('textbox',{name:lang==='zh'?'产品名称':'Product name',exact:true}).fill('隔离验收产品');
   await overflow(page);await page.screenshot({path:path.join(out,`editor-${width}-${lang}.png`),fullPage:true});
   await page.getByRole('button',{name:lang==='zh'?'新增产品分类':'Add product category',exact:true}).click();
   const dialog=page.getByRole('dialog');await dialog.waitFor();
   assert.equal(await dialog.locator('select').inputValue(),String(fixture.industry));
   const name=`验收阀组-${width}-${lang}`;await dialog.getByRole('textbox',{name:lang==='zh'?'分类名称':'Category name',exact:true}).fill(name);
   await overflow(page);await page.screenshot({path:path.join(out,`new-category-${width}-${lang}.png`)});
   await dialog.getByRole('button',{name:lang==='zh'?'保存分类':'Save category',exact:true}).click();await dialog.waitFor({state:'hidden'});
   await page.waitForFunction(name=>{const select=document.querySelector('.vendor-category-picker select');return select?.selectedOptions[0]?.textContent===name},name);
   if(lang==='zh'&&[390,1440].includes(width))await page.screenshot({path:path.join(out,`editor-selected-${width}-${lang}.png`),fullPage:true});
   await page.goto(fixture.base+'/merchant/product-categories');await page.locator('.vendor-category-list article').first().waitFor();await overflow(page);await page.screenshot({path:path.join(out,`manager-${width}-${lang}.png`),fullPage:true});
   await page.goto(fixture.base+fixture.company+'?tab=products&vendor_product_category_id='+fixture.category+'&page=2');await page.locator('#panel-products .showroom-product-grid article').first().waitFor();
   const active=await page.locator('#panel-products select').first().inputValue();assert.equal(active,String(fixture.category));
   const request=await page.request.get(fixture.base+'/api/v1/public/products?vendor_id='+fixture.vendor+'&vendor_product_category_id='+fixture.category+'&page=2&page_size=24');const result=await request.json();assert(result.total>24&&result.items.length>0);
   await overflow(page);await page.screenshot({path:path.join(out,`showroom-${width}-${lang}.png`),fullPage:true});
   await page.locator('#panel-products .showroom-product-grid a[href*="/products/"]').first().click();await page.locator('.public-detail-breadcrumb').waitFor();
   assert.equal(await page.locator('.public-detail-breadcrumb li').count(),2);await page.locator('.public-detail-breadcrumb a').click();await page.locator('#panel-products').waitFor();assert(new URL(page.url()).searchParams.get('vendor_product_category_id')===String(fixture.category));assert.equal(new URL(page.url()).searchParams.get('page'),'2');
   const initialHTML=await (await page.request.get(fixture.base+'/products/'+fixture.product)).text();assert(initialHTML.includes('产品分类：液压回转马达'));assert(initialHTML.includes('"category":"液压回转马达"'));
   await page.goto(fixture.base+'/products/'+fixture.product);await page.locator('.public-product-detail h1').waitFor();assert((await page.locator('.taxonomy-public-attributes').innerText()).includes('液压回转马达'));await overflow(page);await page.screenshot({path:path.join(out,`product-${width}-${lang}.png`),fullPage:true});
   checks.push({width,lang,createAndSelect:true,preferredIndustry:true,companyManagement:true,longCategoryName:true,publicCountsAndFilter:true,pagingReturn:true,productCategory:true,initialHTMLAndStructuredCategory:true,overflow:false});await context.close();
  }
  const admin=await browser.newContext({viewport:{width:1440,height:1000}});await admin.addInitScript(token=>sessionStorage.setItem('zqzt.admin',token),fixture.admin);const page=await admin.newPage();lastPage=page;page.on('pageerror',e=>errors.push(e.message));await page.goto(fixture.base+'/admin/contents/'+fixture.vendor);await page.locator('summary').filter({hasText:'产品分类管理'}).click();await page.locator('.vendor-category-manager .vendor-category-list article').first().waitFor();await overflow(page);await page.screenshot({path:path.join(out,'admin-company-1440-zh.png'),fullPage:true});await admin.close();
  assert.deepEqual(errors,[]);const report={isolatedDatabase:true,productionDeployed:false,physicalDeviceVerified:false,checks,errors,screenshots:out};fs.writeFileSync(path.resolve(__dirname,'../docs/test-output/vendor-product-categories-browser-20261115.json'),JSON.stringify(report,null,2));console.log(JSON.stringify({checks:checks.length,errors,screenshots:out}));
 }catch(error){if(lastPage&&!lastPage.isClosed()){await lastPage.screenshot({path:path.join(out,'failure.png'),fullPage:true});console.error(await lastPage.locator('select').evaluateAll(nodes=>nodes.map(n=>({value:n.value,options:[...n.options].map(o=>o.textContent)}))))}throw error;}finally{await browser.close()}
})().catch(e=>{console.error(e);process.exit(1)});
