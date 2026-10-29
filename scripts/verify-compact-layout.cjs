const fs=require('fs'),assert=require('assert/strict');
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
(async()=>{
 const b=await chromium.launch({headless:true,executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',args:['--no-proxy-server']});
 const base=JSON.parse(fs.readFileSync('.local/ciame-browser-fixture.json','utf8')).base;
 const out='docs/screenshots/compact-layout-20261028';fs.mkdirSync(out,{recursive:true});const checks=[],errors=[];let activePage,current;
 try {
  for(const width of [360,390,768,1440])for(const lang of ['zh','en']){
   const c=await b.newContext({viewport:{width,height:900}});await c.addInitScript(l=>localStorage.setItem('zqzt.language',l),lang);await c.route('**/api/v1/public/analytics/events',r=>r.fulfill({status:202,body:'{}',contentType:'application/json'}));const p=await c.newPage();p.on('pageerror',e=>errors.push(e.message));
   activePage=p;for(const path of ['/','/browse','/browse/products','/regions']){
    current={width,lang,path};console.log(JSON.stringify(current));
    await p.goto(base+path,{waitUntil:'domcontentloaded'});
    await p.locator(path==='/regions'?'.industry-title h1':path==='/'?'.industrial-hero h1':'.browse-page h1').first().waitFor();
    if(path==='/') {assert.equal(await p.locator('.industry-hall-grid').count(),0);assert.equal(await p.locator('.exhibition-portal').count(),0);}
    if(path==='/regions'){assert.equal(await p.locator('.exhibition-breadcrumb').count(),0);await p.locator('.industry-map-controls').waitFor();}
    if(path.startsWith('/browse')){
     assert.equal(await p.getByText('ZQZG / EXPLORE THE EXHIBITION',{exact:true}).count(),0);
     const metrics=await p.evaluate(()=>{const f=document.querySelector('.browse-page .public-search-form').getBoundingClientRect(),h=document.querySelector('.header').getBoundingClientRect(),crumb=document.querySelector('.exhibition-breadcrumb');return{formTop:f.top,headerBottom:h.bottom,crumbHeight:crumb?.getBoundingClientRect().height||0,crumbPosition:crumb?getComputedStyle(crumb).position:null};});
     assert(metrics.formTop-metrics.headerBottom<230,`${width} ${lang} ${path}: excessive top spacing`);
     assert(metrics.crumbHeight<55,`${width} ${lang} ${path}: tall breadcrumb`);assert.equal(metrics.crumbPosition,'static');checks.push({width,lang,path,...metrics});
    }
    assert.equal(await p.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false,`${width} ${lang} ${path}: horizontal overflow`);
    if(lang==='zh'&&(width===1440||width===390))await p.screenshot({path:out+'/'+(path==='/'?'home':path.slice(1).replaceAll('/','-'))+'-'+width+'.png',fullPage:path!=='/regions'});
   }
   await c.close();
  }
  assert.equal(errors.length,0);fs.writeFileSync('docs/test-output/compact-layout-20261028.json',JSON.stringify({checks,errors,viewports:[360,390,768,1440],languages:['zh','en'],pages:4,productionWrites:false,isolatedDatabase:true},null,2));console.log(JSON.stringify({checks:checks.length,errors}));
 }catch(e){console.error(JSON.stringify({current,errors,url:activePage?.url()}));if(activePage&&!activePage.isClosed()){await activePage.screenshot({path:out+'/failure.png',fullPage:true});console.error((await activePage.locator('body').innerText()).slice(0,1500));}throw e;}finally{await b.close();}
})().catch(e=>{console.error(e);process.exit(1)});

