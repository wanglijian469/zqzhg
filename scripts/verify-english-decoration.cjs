// Read-only local acceptance: no account, content or SMS submissions.
const fs=require('node:fs'),assert=require('node:assert/strict');
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
const base='http://localhost:8080',dir='docs/screenshots/english-decoration';fs.mkdirSync(dir,{recursive:true});
const labels=['ZQZG / INDUSTRIAL EXHIBITION','FOR MANUFACTURERS','INDUSTRIAL EXHIBITION','MANUFACTURING / PRODUCTS / CONNECTIONS','ABOUT THE COMPANY','MANUFACTURING','QUALIFICATIONS','HONORS & AWARDS'];
(async()=>{
 const browser=await chromium.launch({headless:true,executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',args:['--no-proxy-server']});
 const receipt={base,productionDeployed:false,layouts:[],languagePersistence:[],regressions:[],errors:[],writes:[]};let p;
 try{
  p=await browser.newPage({viewport:{width:1440,height:900}});p.setDefaultTimeout(15000);
  p.on('pageerror',e=>receipt.errors.push(e.message));p.on('request',r=>{if(r.url().startsWith(base+'/api/')&&!['GET','HEAD'].includes(r.method()))receipt.writes.push({path:new URL(r.url()).pathname,method:r.method()})});
  let lastOpen=0;
  const open=async path=>{const pause=6000-(Date.now()-lastOpen);if(pause>0)await p.waitForTimeout(pause);let r=await p.goto(base+path,{waitUntil:'domcontentloaded'});lastOpen=Date.now();if(r.status()===429){let remaining=(Number(r.headers()['retry-after'])||60)*1000+1000;console.log('Respecting public access retry interval');while(remaining>0){const delay=Math.min(30000,remaining);await p.waitForTimeout(delay);remaining-=delay}r=await p.goto(base+path,{waitUntil:'domcontentloaded'});lastOpen=Date.now()}assert.equal(r.status(),200,path);if(path==='/')await p.locator('.industrial-products .product').first().waitFor()};
  const toggle=async()=>{const button=p.locator('.language-button').first(),menu=p.locator('.header .menu-button');if(!await button.isVisible()){await menu.click()}await button.click();if(await menu.isVisible()&&await menu.getAttribute('aria-expanded')==='true')await menu.click()};
  const snapshot=async options=>{await p.waitForLoadState('networkidle');await p.evaluate(()=>document.fonts.ready);await p.waitForFunction(()=>Array.from(document.images).filter(i=>{const r=i.getBoundingClientRect();return r.bottom>0&&r.top<innerHeight&&r.width>0}).every(i=>i.complete));await p.screenshot(options)};
  const noDecor=async()=>{for(const text of labels)assert.equal(await p.getByText(text,{exact:true}).count(),0,text)};
  const layout=async(state,width)=>{assert.equal(await p.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false,state+' '+width+' overflow');receipt.layouts.push({state,width,noOverflow:true})};
  await open('/');await p.locator('.industrial-products .product').first().waitFor();
  for(const width of [1440,768,390,360]){
   await p.setViewportSize({width,height:900});await noDecor();assert.equal(await p.locator('.stage-baseline,.hero-eyebrow,.stage-english-label').count(),0);assert.equal(await p.locator('.stage-overline').innerText().catch(()=>'' )||await p.locator('.stage-overline').textContent(),'农业机械厅');
   await layout('home-zh',width);await p.evaluate(()=>scrollTo(0,0));await snapshot({path:dir+'/home-zh-'+width+'.png'});
   await toggle();await p.waitForFunction(()=>localStorage.getItem('zqzt.language')==='en');assert.equal(await p.getByText('ZQZG / INDUSTRIAL EXHIBITION',{exact:true}).count(),3);assert.equal(await p.locator('.stage-baseline').count(),1);assert.equal(await p.getByText('FOR MANUFACTURERS',{exact:true}).count(),1);await layout('home-en',width);await snapshot({path:dir+'/home-en-'+width+'.png'});
   if(width===1440){assert(await p.locator('.hero-eyebrow').isVisible());assert(await p.locator('.stage-english-label').isVisible());await open('/');assert.equal(await p.getByText('ZQZG / INDUSTRIAL EXHIBITION',{exact:true}).count(),3);receipt.languagePersistence.push({width,language:'en',refresh:true})}
   await toggle();await noDecor();if(width===390){await open('/');await noDecor();assert.equal(await p.evaluate(()=>localStorage.getItem('zqzt.language')),'zh');receipt.languagePersistence.push({width,language:'zh',refresh:true})}
  }
  await p.setViewportSize({width:1440,height:900});await p.locator('.section-title').first().scrollIntoViewIfNeeded();await snapshot({path:dir+'/home-zh-sections-1440.png'});const footer=p.locator('footer.public-footer');assert.equal(await footer.getByRole('link',{name:'管理后台',exact:true}).getAttribute('href'),'/admin');assert.equal(await footer.getByRole('link',{name:'投诉与服务申请',exact:true}).getAttribute('href'),'/service-requests');assert(!(await footer.innerText()).includes('技术支持'));assert(!(await footer.innerText()).includes('公开展示与联系平台'));receipt.regressions.push('current public footer retained');
  await p.locator('.hero-search input').fill('冀农');await p.locator('.hero-search button').click();await p.waitForURL(u=>u.pathname==='/browse'&&u.searchParams.get('q')==='冀农');await p.getByRole('heading',{name:'河北冀农农机具有限公司',exact:true}).waitFor();receipt.regressions.push('manufacturer search');
  await p.locator('footer.public-footer').getByRole('link',{name:'免费入驻',exact:true}).click();await p.waitForURL(u=>u.pathname==='/join');await p.locator('main h1').waitFor();receipt.regressions.push('free join entry');
  await open('/?hall=cable');await p.locator('.preparing-hall h1').waitFor();await noDecor();assert((await p.locator('.preparing-hall h1').innerText()).includes('电线电缆厅'));await toggle();assert.equal(await p.locator('.preparing-hall .eyebrow').count(),1);await toggle();await noDecor();receipt.regressions.push('preparing hall zh/en/zh');
  await open('/v/jinong?tab=about');await p.locator('#panel-about .portal-production').waitFor();
  for(const width of [1440,768,390,360]){
   await p.setViewportSize({width,height:900});await noDecor();assert((await p.locator('#panel-about').innerText()).includes('拥有各种生产设备180台套'));assert.equal(await p.locator('.showroom-footer-simple').count(),1);assert.equal(await p.locator('footer.public-footer').count(),0);await layout('showroom-zh',width);await p.locator('#panel-about .portal-section-heading').first().scrollIntoViewIfNeeded();await snapshot({path:dir+'/showroom-zh-'+width+'.png'});
   await toggle();assert(await p.locator('#panel-about').getByText('ABOUT THE COMPANY',{exact:true}).isVisible());assert(await p.locator('#panel-about').getByText('MANUFACTURING',{exact:true}).isVisible());await layout('showroom-en',width);await snapshot({path:dir+'/showroom-en-'+width+'.png'});await toggle();await noDecor();
  }
  assert.deepEqual(receipt.errors,[]);assert.deepEqual(receipt.writes,[]);receipt.passed=true;
 }catch(e){receipt.failure=e.stack;if(p)await p.screenshot({path:dir+'/failure.png'});throw e}
 finally{await browser.close();fs.mkdirSync('docs/test-output',{recursive:true});fs.writeFileSync('docs/test-output/english-decoration.json',JSON.stringify(receipt,null,2))}
 console.log(JSON.stringify(receipt,null,2));
})().catch(e=>{console.error(e.stack);process.exitCode=1});
