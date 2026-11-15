const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
// Isolated fixtures; no production database or authenticated requests.
const base=process.env.LAYOUT_BASE||'http://127.0.0.1:5197',phase=process.argv[2]||'after';
assert(['before','after'].includes(phase));
const out=path.resolve(__dirname,'../docs/screenshots/company-info-20261115',phase);
const categories=[{id:1,parent_id:0,name:'液压泵、阀、输出装置零部件',name_en:'Hydraulic pumps, valves and output components'},{id:2,parent_id:1,name:'液压泵、齿轮泵零部件',name_en:'Hydraulic and gear pump components'},{id:3,parent_id:1,name:'液压阀、液压分配器零部件',name_en:'Hydraulic valves and distributor components'}].map(c=>({...c,hall_id:1,enabled:true,category_kind:'vendor',sort_order:0}));
const blank={name_en:'',description_en:'',main_products:'',keywords:'',region_id:1,region_name:'河北省 邢台市 宁晋县',category_id:1,description:'',phone:'0319-1234567',wechat:'',address:'',parameters:'',media_ids:[]};
const vendor={id:21,kind:'vendor',hall_id:1,showroom_slug:'paiyi',data:{...blank,name:'河北派一液压机械有限公司',name_en:'Hebei Paiyi Hydraulic Machinery Manufacturing Company',main_products:'液压泵、液压阀及液压分配器零部件',description:'专注于液压系统及配套零部件的生产制造，为客户提供稳定可靠的产品与技术支持。',address:'河北省邢台市宁晋县凤凰镇万方科技产业园2号车间',primary_category_id:1,secondary_category_ids:[2,3],business_nature:'manufacturer',industrial_belt_ids:[1],company_profile:{factory_area:'1000',equipment:'高精度加工设备84台，配套数控加工中心、检测设备及自动化生产线',capacity:'年产液压零部件100000件\n可按客户要求定制加工',equipment_en:'84 precision machines, CNC machining centres and automated production lines',capacity_en:'100000 hydraulic components annually\nCustom manufacturing available',established:'2010',staff:'60',contact_name:'业务负责人'}}};
const manufacturer={...vendor,id:22,showroom_slug:'jinong',data:{...blank,name:'河北冀农农机具有限公司',name_en:'Hebei Jinong Agricultural Machinery Company'}};
const product={id:31,vendor_id:22,kind:'product',hall_id:1,data:{...blank,name:'液压翻转犁1LF-440',name_en:'Hydraulic Reversible Plough 1LF-440',description:'适用于旱田耕作的液压翻转犁。',model:'1LF-440',parameters:'型号：1LF-440\n工作幅宽：1600mm'}};
(async()=>{
 fs.mkdirSync(out,{recursive:true});
 const browser=await chromium.launch({headless:true,executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',args:['--no-proxy-server']});
 const results=[],errors=[];
 try{
  for(const width of [1440,1024,768,390,360])for(const lang of phase==='before'?['zh']:['zh','en']){
   const context=await browser.newContext({viewport:{width,height:900}});
   await context.route('**/api/v1/**',async route=>{
    const p=new URL(route.request().url()).pathname;let data={};
    if(p.endsWith('/public/halls'))data={items:[{id:1,slug:'agriculture',name:'农业机械厅',name_en:'Agricultural Machinery Hall',status:'open',icon:'tractor'}]};
    else if(p.endsWith('/taxonomy'))data={categories,regions:[],industrial_belts:[{id:1,name:'宁晋液压产业带',name_en:'Ningjin Hydraulics Industry Belt'}]};
    else if(p.includes('/showrooms/'))data={item:vendor,canonical_url:base+'/v/paiyi',product_count:0,home_products:[],resource_counts:{video:0,catalog:0}};
    else if(p.endsWith('/public/products/31'))data={item:product,vendor:manufacturer};
    else if(p.includes('/resources')||p.endsWith('/public/products'))data={items:[],total:0,page:1,page_size:24};
    await route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
   });
   const page=await context.newPage();page.on('pageerror',e=>errors.push(e.message));
    await page.addInitScript(lang=>localStorage.setItem('zqzt.language',lang),lang);
    for(const [kind,url,ready] of [['product','/products/31','.public-detail-breadcrumb'],['company','/v/paiyi?tab=about','.portal-production']]){
     await page.goto(base+url);await page.locator(ready).waitFor();await page.evaluate(()=>document.fonts.ready);
     if(kind==='company'){await page.locator(phase==='before'?'.taxonomy-public-attributes':'.portal-overview').evaluate(el=>el.scrollIntoView({block:'start'}));await page.evaluate(()=>window.scrollBy(0,-76));}
     const metrics=await page.evaluate(()=>{
      const production=document.querySelector('.portal-production');
      const rows=production?[...production.children].map(el=>{const r=el.getBoundingClientRect();return {x:r.x,y:r.y,width:r.width,height:r.height}}):[];
      return {language:document.documentElement.lang,width:innerWidth,scrollWidth:document.documentElement.scrollWidth,rows,breadcrumb:[...document.querySelectorAll('.public-detail-breadcrumb li')].map(el=>el.textContent),backLinks:document.querySelectorAll('.public-product-detail > .back').length,productionHeight:production?.getBoundingClientRect().height};
     });
     if(phase==='after'){
      assert.equal(metrics.language,lang==='zh'?'zh-CN':'en');
      assert(metrics.scrollWidth<=width+1,`${kind}/${width}/${lang} overflows: ${JSON.stringify(metrics)}`);
      if(kind==='product'){
       assert.equal(metrics.breadcrumb.length,2);assert.equal(metrics.backLinks,0);
       assert.equal(await page.locator('.public-detail-breadcrumb a').getAttribute('href'),'/v/jinong');
      }else{
       assert.equal(await page.locator('.company-business-info h2').count(),1);
       assert.equal(await page.locator('.company-business-info .taxonomy-category-name').count(),3);
       assert.equal(await page.locator('.company-business-info .taxonomy-category-parent').count(),2);
       assert.equal(await page.locator('.portal-production dd').first().textContent(),'1000');
       const positions=metrics.rows.map(r=>width>900?r.y:r.x);assert(positions.every(v=>Math.abs(v-positions[0])<1),JSON.stringify(metrics));
       if(width<=900)assert(metrics.productionHeight<350,`Production panel too tall: ${metrics.productionHeight}`);
       await page.evaluate(()=>window.scrollTo(0,document.documentElement.scrollHeight));
       const bottom=await page.evaluate(()=>{const footer=document.querySelector('.showroom-footer').getBoundingClientRect(),contact=document.querySelector('.showroom-mobile-contact');return {footerBottom:footer.bottom,contactTop:contact&&getComputedStyle(contact).display!=='none'?contact.getBoundingClientRect().top:innerHeight}});
       assert(bottom.footerBottom<=bottom.contactTop+1,`Footer obscured: ${JSON.stringify(bottom)}`);
       await page.locator('.company-business-info').scrollIntoViewIfNeeded();
      }
     }
     await page.screenshot({path:path.join(out,`${kind}-${width}-${lang}.png`),fullPage:true});
     if(kind==='company')await page.screenshot({path:path.join(out,`company-focus-${width}-${lang}.png`)});
     results.push({kind,lang,...metrics});
    }
   if(phase==='after'&&width===360&&lang==='zh'){
    const names=[manufacturer.data.name,product.data.name];
    manufacturer.data.name='河北冀农大型农业机械与高精度液压装备研发制造有限公司';
    product.data.name='适用于大马力拖拉机的重型液压翻转犁及配套耕整地机械1LF-440';
    await page.goto(base+'/products/31');await page.locator('.public-detail-breadcrumb').waitFor();
    assert.equal(await page.locator('.public-detail-breadcrumb li').first().textContent(),manufacturer.data.name);
    assert.equal(await page.locator('.public-detail-breadcrumb li').last().textContent(),product.data.name);
    const long=await page.evaluate(()=>({width:innerWidth,scrollWidth:document.documentElement.scrollWidth,clipped:[...document.querySelectorAll('.public-detail-breadcrumb li')].some(el=>el.scrollHeight>el.clientHeight+1)}));
    assert(long.scrollWidth<=width+1&&!long.clipped,JSON.stringify(long));
    await page.screenshot({path:path.join(out,'product-long-360-zh.png'),fullPage:true});results.push({kind:'long-product',lang,...long});
    [manufacturer.data.name,product.data.name]=names;
    const profile=vendor.data.company_profile;
    for(const count of [1,2]){
     vendor.data.company_profile=count===1?{factory_area:'1000'}:{factory_area:'1000',equipment:profile.equipment};
     await page.goto(base+'/v/paiyi?tab=about');await page.locator('.portal-production').waitFor();
     assert.equal(await page.locator('.portal-production>div').count(),count);
     assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
     results.push({kind:'production-field-count',lang,width,count});
    }
    vendor.data.company_profile=profile;
   }
   await context.close();
  }
  assert.deepEqual(errors,[]);fs.writeFileSync(path.join(out,'results.json'),JSON.stringify({phase,fixture:'isolated public-page data',errors,results},null,2));
  console.log(JSON.stringify({phase,screenshots:out,pages:results.length,errors}));
 }finally{await browser.close()}
})().catch(e=>{console.error(e);process.exit(1)});
