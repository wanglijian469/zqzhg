// Read-only public checks. A submitted URL or a matching keyword is never a search pass.
const fs=require('fs');
const path=require('path');
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');

(async()=>{
 const site=new URL(process.argv[2]||'https://zqzhg.com');
 const out=path.resolve(process.argv[3]||'docs/test-output/search-visibility-20261024');
 fs.mkdirSync(out,{recursive:true});
 const proxyURL=process.env.HTTPS_PROXY||process.env.HTTP_PROXY;
 const proxy=proxyURL?new URL(proxyURL):null;
 const browser=await chromium.launch({headless:true,executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',...(proxy?{proxy:{server:proxy.protocol+'//'+proxy.host,username:decodeURIComponent(proxy.username),password:decodeURIComponent(proxy.password)}}:{})});
 const report={site:site.origin,checkedAt:new Date().toISOString(),timeZone:'Asia/Shanghai',production:[],searches:[],searchAcceptance:'not_passed',deployment:'not_performed',searchConsoleSubmission:'not_performed_no_verified_session'};
 const engines=[
  {name:'baidu',url:'https://www.baidu.com/s?wd=',selector:'#content_left > .result, #content_left > .c-container'},
  {name:'bing',url:'https://www.bing.com/search?q=',selector:'#b_results > li.b_algo'},
  {name:'google',url:'https://www.google.com/search?q=',selector:'#search div.MjjYud:has(h3)'}
 ];
 const context=await browser.newContext({viewport:{width:1366,height:900},locale:'zh-CN'});
 const sameSite=url=>{try{return new URL(url).hostname.replace(/^www\./,'')===site.hostname.replace(/^www\./,'')}catch{return false}};
 try{
  for(const route of ['/','/robots.txt','/sitemap.xml']){
   const check={path:route,status:'unverified'};
   try{const response=await context.request.get(site.origin+route,{timeout:15000});check.httpStatus=response.status();check.status=response.ok()?'reachable':'http_error';if(route!=='/')check.body=(await response.text()).slice(0,12000)}catch(error){check.error=String(error.message).split('\n')[0]}
   report.production.push(check);
  }
  for(const engine of engines)for(const query of ['内卷网','河北冀农农机具有限公司']){
   const page=await context.newPage();
   const check={engine:engine.name,query,url:engine.url+encodeURIComponent(query),status:'unverified',results:[],screenshot:engine.name+'-'+(query==='内卷网'?'keyword':'company')+'.png'};
   try{
    const response=await page.goto(check.url,{waitUntil:'domcontentloaded',timeout:20000});
    check.httpStatus=response?.status();
    await page.locator(engine.selector).first().waitFor({state:'visible',timeout:6000}).catch(()=>{});
    check.pageTitle=await page.title();
    const rows=await page.locator(engine.selector).all();
    for(const row of rows.slice(0,20)){
     if(!await row.isVisible())continue;
     const title=await row.locator('h3').first().innerText().catch(()=> '');
     if(!title)continue;
     const urls=await row.locator('a[href]').evaluateAll(links=>links.map(a=>a.href));
     check.results.push({position:check.results.length+1,title,urls,text:(await row.innerText()).slice(0,600)});
    }
    if(check.results.length)check.status='not_found_in_visible_results';
    for(const row of check.results){
     // Direct destination or displayed result URL; follow and verify the final host.
     const href=row.urls.find(sameSite)||(/(?:www\.)?zqzhg\.com\b/i.test(row.text)?row.urls[0]:undefined);
     if(!href)continue;
     check.status='found_link_unverified';check.position=row.position;
     const destination=await context.newPage();
     try{
      const landing=await destination.goto(href,{waitUntil:'domcontentloaded',timeout:20000});
      check.targetURL=destination.url();
      const companyPage=query==='内卷网'||(/^\/(?:v|vendors)\//.test(new URL(destination.url()).pathname)&&(await destination.locator('body').innerText()).includes(query));
      if(landing?.ok()&&sameSite(destination.url())&&companyPage)check.status='verified';
     }catch(error){check.targetError=String(error.message).split('\n')[0]}finally{await destination.close()}
     break;
    }
   }catch(error){check.error=String(error.message).split('\n')[0]}
   await page.screenshot({path:path.join(out,check.screenshot),fullPage:false,timeout:10000}).catch(error=>{check.screenshotError=String(error.message).split('\n')[0]});
   report.searches.push(check);await page.close();
   fs.writeFileSync(path.join(out,'report.json'),JSON.stringify(report,null,2));
  }
  if(report.searches.some(row=>row.engine==='baidu'&&row.query==='内卷网'&&row.status==='verified'))report.searchAcceptance='passed';
 }finally{
  fs.writeFileSync(path.join(out,'report.json'),JSON.stringify(report,null,2));await browser.close();
 }
 console.log(JSON.stringify({report:path.join(out,'report.json'),production:report.production.map(({path,status})=>({path,status})),searches:report.searches.map(({engine,query,status})=>({engine,query,status})),searchAcceptance:report.searchAcceptance}));
})().catch(error=>{console.error(error.message);process.exitCode=1});
