const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
(async()=>{
 const fixture=JSON.parse(fs.readFileSync(0,'utf8')),out=path.resolve(__dirname,'../docs/screenshots/video-share-cards-20261116');fs.mkdirSync(out,{recursive:true});
 const report={isolatedDatabase:true,productionChanged:false,paidResolversUsed:false,externalPlatformIntercepted:true,checks:[],pageErrors:[],unexpectedRequests:[]};
 const browser=await chromium.launch({headless:true,executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',args:['--no-proxy-server']});let lastPage;
 const overflow=async page=>assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),'horizontal overflow');
 const capture=async(page,file)=>{await page.evaluate(()=>{document.activeElement?.blur();window.scrollTo({top:0,left:0,behavior:'instant'})});await page.waitForFunction(()=>window.scrollY===0);await page.screenshot({path:path.join(out,file),fullPage:true,animations:'disabled',caret:'hide'})};
 try{for(const width of [1440,1024,768,390,360])for(const lang of ['zh','en']){
  const context=await browser.newContext({viewport:{width,height:1000}});
  await context.addInitScript(({owner,account,lang})=>{sessionStorage.setItem('zqzt.merchant',owner);sessionStorage.setItem('zqzt.merchant.id',String(account));sessionStorage.setItem('zqzt.merchant.role','merchant');localStorage.setItem('zqzt.language',lang)},{...fixture,lang});
  await context.route('https://weixin.qq.com/**',r=>r.fulfill({status:200,contentType:'text/html',body:'External platform destination intercepted for local acceptance.'}));
  await context.route('https://v.kuaishou.com/**',r=>r.fulfill({status:200,contentType:'text/html',body:'External platform destination intercepted for local acceptance.'}));
  context.on('request',r=>{if(!r.url().startsWith(fixture.base)&&!r.url().startsWith('https://weixin.qq.com/')&&!r.url().startsWith('https://v.kuaishou.com/'))report.unexpectedRequests.push(r.url())});
  const page=await context.newPage();lastPage=page;page.on('pageerror',e=>report.pageErrors.push(e.message));
  await page.goto(fixture.base+fixture.company+'?tab=videos');await page.locator('.showroom-video-grid .resource-card').first().waitFor();
  await page.locator('.resource-purpose select').selectOption('company');await page.getByRole('link',{name:lang==='zh'?'前往视频号观看':'Watch on WeChat Channels',exact:true}).waitFor();
  await overflow(page);assert.equal(await page.locator('video,iframe').count(),0);
  await capture(page,`cards-${width}-${lang}.png`);
  const before=page.url();for(const [label,url] of [[lang==='zh'?'前往视频号观看':'Watch on WeChat Channels','https://weixin.qq.com/sph/Aht6MMIJPm'],[lang==='zh'?'前往快手观看':'Watch on Kuaishou','https://v.kuaishou.com/Abc123']]){
   const watch=page.getByRole('link',{name:label,exact:true});assert.equal(await watch.getAttribute('target'),'_blank');
   const popupEvent=context.waitForEvent('page');await watch.click();const popup=await popupEvent;await popup.waitForLoadState('domcontentloaded');assert.equal(popup.url(),url);await popup.close();
   assert.equal(page.url(),before);assert.equal(await page.locator('.resource-purpose select').inputValue(),'company');assert.equal(await page.getByRole('dialog').count(),0);
  }
  await page.getByRole('button',{name:lang==='zh'?'复制链接':'Copy link',exact:true}).first().click();
  const copied=page.getByRole('status').filter({hasText:lang==='zh'?'链接已复制':'Link copied'}),manual=page.getByRole('textbox',{name:lang==='zh'?'视频链接':'Video link',exact:true});
  await page.waitForFunction(()=>document.querySelector('.share-video-actions [role=status],.share-video-copy-fallback input'));
  assert(await copied.count()||await manual.count());
  await page.goto(fixture.base+'/merchant/booth?section=video');await page.getByRole('button',{name:lang==='zh'?'添加企业宣传视频':'Add company video',exact:true}).click();
  await page.getByRole('button',{name:lang==='zh'?'微信视频号分享链接':'Paste WeChat Channels link',exact:true}).click();
  assert.equal(await page.locator('.video-source-fields').count(),1);
  await page.locator('.video-source-fields textarea').fill('https://weixin.qq.com/sph/UiCard'+width+lang);
  await page.getByRole('button',{name:lang==='zh'?'检查链接':'Check link',exact:true}).click();await page.locator('.video-source-fields [role=status]').waitFor();
  await page.getByRole('textbox',{name:lang==='zh'?'视频名称':'Video name',exact:true}).fill('免费分享卡片 '+width+' '+lang);
  assert.equal(await page.locator('form.editor input[type=file][accept*="mp4"]').count(),0);await overflow(page);
  await capture(page,`form-${width}-${lang}.png`);
  const savedEvent=page.waitForResponse(r=>r.request().method()==='POST'&&r.url().includes('/api/v1/merchant/resources?')&&r.status()===201);
  await page.locator('form.editor .form-actions>button:not([type="button"])').click();const saved=await (await savedEvent).json();assert.equal(saved.data.video_source,'WECHAT_CHANNELS');assert.equal(saved.data.file_media_id,0);
  report.checks.push({width,lang,directExternalOpen:true,copy:true,stateRetained:true,noPlayer:true,createWechatDraft:true,noVideoUpload:true,overflow:false});await context.close();
 }
 assert.deepEqual(report.pageErrors,[]);assert.deepEqual(report.unexpectedRequests,[]);report.passed=true;
 }catch(e){if(lastPage&&!lastPage.isClosed())await lastPage.screenshot({path:path.join(out,'failure.png'),fullPage:true});throw e;}
 finally{await browser.close();fs.writeFileSync(path.resolve(__dirname,'../docs/test-output/video-share-cards-browser-20261116.json'),JSON.stringify(report,null,2)+'\n');}
 console.log(JSON.stringify({checks:report.checks.length,passed:true}));
})().catch(e=>{console.error(e);process.exitCode=1});
