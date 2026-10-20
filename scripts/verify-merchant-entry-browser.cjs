// Invoked only by TestMerchantEntryBrowser against its disposable MySQL server.
const assert=require('node:assert/strict');
const fs=require('node:fs');
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
const base=process.argv[2];assert.match(base,/^http:\/\/127\.0\.0\.1:\d+$/);
(async()=>{
 const browser=await chromium.launch({executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',headless:true,args:['--no-proxy-server']});
 const errors=[];fs.mkdirSync('docs/screenshots',{recursive:true});
 try{
  const context=await browser.newContext({viewport:{width:1440,height:900}}),page=await context.newPage();
  page.on('pageerror',e=>errors.push(e.message));
  const input=name=>page.getByLabel(name,{exact:true});
  const ready=()=>input('公司登录账号').waitFor();
  async function registration(name,password){await input('公司登录账号').fill(name);await input('设置密码').fill(password);await input('确认密码').fill(password)}
  async function fit(desktop=false){
   assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),'no horizontal overflow');
   if(desktop){assert.ok(await page.evaluate(()=>document.documentElement.scrollHeight<=innerHeight+1),'desktop page fits height');assert.ok(await page.locator('.merchant-auth-submit').evaluate(el=>el.getBoundingClientRect().bottom<=innerHeight),'desktop submit is visible')}
   assert.ok(await page.locator('.merchant-entry-shell').evaluate(el=>![...el.querySelectorAll('main,.merchant-entry,.merchant-entry-card,.merchant-auth')].some(node=>['scroll','auto'].includes(getComputedStyle(node).overflowY)&&node.scrollHeight>node.clientHeight+1)),'natural page scroll only');
  }
  for(const [width,height] of [[1440,900],[1366,768],[390,844],[360,640],[844,390]]){
   await page.setViewportSize({width,height});
   for(const route of ['join','merchant/login']){
    await page.goto(base+'/'+route);await ready();await fit(width>=960);
    const prefix='docs/screenshots/merchant-entry-'+(route==='join'?'join':'login')+'-'+width+'x'+height;
    await page.screenshot({path:prefix+'.png',fullPage:true,animations:'disabled'});
    // Expanded help and validation remain reachable on short screens and mobile keyboards.
    await page.locator('.merchant-auth-submit').click();await input('公司登录账号').evaluate(el=>{if(document.activeElement!==el)throw Error('first invalid field is not focused')});
    await page.locator('.merchant-auth-help summary').click();await page.locator('.merchant-auth-submit').scrollIntoViewIfNeeded();await fit();
   }
  }
  console.log('PASS layout: both pages at 1440x900, 1366x768, 390x844, 360x640 and 844x390; expanded help and errors reachable');
  await page.setViewportSize({width:1440,height:900});await page.goto(base+'/join');await ready();await page.getByRole('button',{name:'English',exact:true}).click();await page.getByRole('heading',{name:'Create a company account',exact:true}).waitFor();await fit(true);await page.screenshot({path:'docs/screenshots/merchant-entry-join-en.png',fullPage:true});
  await page.getByRole('link',{name:'Company sign-in',exact:true}).click();await page.getByRole('heading',{name:'Sign in to your workspace',exact:true}).waitFor();await fit(true);await page.screenshot({path:'docs/screenshots/merchant-entry-login-en.png',fullPage:true});await page.getByRole('button',{name:'中文',exact:true}).click();await ready();
  await input('公司登录账号').focus();await page.keyboard.press('Tab');assert.equal(await page.evaluate(()=>document.activeElement?.getAttribute('name')),'password');await page.keyboard.press('Tab');assert.equal(await page.evaluate(()=>document.activeElement?.getAttribute('aria-label')),'显示密码');await page.keyboard.press('Enter');assert.equal(await input('密码').getAttribute('type'),'text');
  await page.getByRole('button',{name:'短信验证码',exact:true}).click();await page.getByRole('button',{name:'账号密码',exact:true}).click();assert.equal(await input('密码').getAttribute('type'),'password');assert.equal(await input('密码').inputValue(),'');
  console.log('PASS English, keyboard tab order, visibility and authentication mode reset');
  const username='entry-browser-'+Date.now(),password='Entry-Browser-2026!';
  const next='/merchant/products?status=draft';
  await page.goto(base+'/join?hall=agriculture&next='+encodeURIComponent(next));await ready();await registration(username,password);
  await page.getByRole('button',{name:'显示设置密码',exact:true}).click();assert.equal(await input('设置密码').getAttribute('type'),'text');assert.equal(await input('确认密码').getAttribute('type'),'password');await page.getByRole('button',{name:'隐藏设置密码',exact:true}).click();
  await page.getByRole('link',{name:'已有公司账号登录',exact:true}).click();await ready();assert.equal(await input('公司登录账号').inputValue(),username);assert.equal(await input('密码').inputValue(),'');assert.equal(new URL(page.url()).searchParams.get('next'),next);assert.equal(new URL(page.url()).searchParams.get('hall')||'agriculture','agriculture');assert.ok(!page.url().includes(username));
  await page.getByRole('link',{name:'新厂商入驻',exact:true}).click();await ready();assert.equal(await input('公司登录账号').inputValue(),username);
  // An aborted registration is explicitly uncertain, never silently retried.
  let failedCalls=0;await page.route('**/api/v1/auth/merchant/register',route=>{failedCalls++;return route.abort('failed')});await registration(username,password);await page.getByRole('button',{name:'注册并继续',exact:true}).click();await page.getByText(/无法确认账号/).waitFor();assert.equal(failedCalls,1);assert.equal(await input('设置密码').inputValue(),password);await page.getByRole('link',{name:'已有账号？登录继续',exact:true}).waitFor();await page.unroute('**/api/v1/auth/merchant/register');
  await page.evaluate(()=>localStorage.setItem('zqzt.draft.guest.vendor',JSON.stringify({data:{name:'入口浏览器验收企业（仅测试）',region_name:'河北省',main_products:'测试产品',phone:'13900009998',category_id:0},revision_id:0})));
  let registrationCalls=0;page.on('request',r=>{if(new URL(r.url()).pathname==='/api/v1/auth/merchant/register')registrationCalls++});
  await page.locator('.merchant-auth form').evaluate(form=>{form.requestSubmit();form.requestSubmit()});await page.waitForURL('**/merchant/booth/edit');await input('企业 / 工厂名称').waitFor();assert.equal(registrationCalls,1);assert.equal(await input('企业 / 工厂名称').inputValue(),'入口浏览器验收企业（仅测试）');
  const account=await page.evaluate(()=>({token:sessionStorage.getItem('zqzt.merchant'),id:sessionStorage.getItem('zqzt.merchant.id')}));assert.ok(account.token);assert.ok(await page.evaluate(password=>!Object.values(localStorage).some(v=>v.includes(password))&&!Object.values(sessionStorage).some(v=>v.includes(password)),password));
  await page.goto(base+'/merchant/login?next='+encodeURIComponent('/merchant/booth'));await page.waitForURL('**/merchant/booth');assert.equal(await page.locator('.merchant-entry').count(),0);await page.goto(base+'/join');await page.waitForURL('**/merchant/booth/edit');
  await page.evaluate(()=>sessionStorage.clear());await page.goto(base+'/join');await ready();await registration(username,password);await page.getByRole('button',{name:'注册并继续',exact:true}).click();await page.getByText('此登录账号已使用，请更换',{exact:true}).waitFor();assert.equal(await input('公司登录账号').evaluate(el=>el===document.activeElement),true);assert.equal(await input('设置密码').inputValue(),password);
  await page.getByRole('link',{name:'已有公司账号登录',exact:true}).click();await ready();await input('密码').fill('incorrect-password');await input('密码').press('Enter');await page.getByText('账号或密码错误',{exact:true}).waitFor();assert.equal(await input('密码').inputValue(),'incorrect-password');await input('密码').fill(password);await input('密码').press('Enter');await page.waitForURL('**/merchant');assert.equal(await page.evaluate(()=>sessionStorage.getItem('zqzt.merchant.id')),account.id);
  // Valid return address and unsafe return rejection use real account authentication.
  await page.evaluate(()=>sessionStorage.clear());await page.goto(base+'/merchant/login?next='+encodeURIComponent(next));await ready();await input('公司登录账号').fill(username);await input('密码').fill(password);await input('密码').press('Enter');await page.waitForURL(url=>url.pathname==='/merchant/products'&&url.searchParams.get('status')==='draft');
  await page.evaluate(()=>sessionStorage.clear());await page.goto(base+'/merchant/login?next='+encodeURIComponent('https://example.org/'));await ready();await input('公司登录账号').fill(username);await input('密码').fill(password);await input('密码').press('Enter');await page.waitForURL(base+'/merchant');
  console.log('PASS isolated real registration/login, duplicate submission, duplicate name focus, wrong password, uncertain outcome, draft restoration, signed-in routing and safe return');
  await page.evaluate(()=>sessionStorage.clear());await page.goto(base+'/merchant/login');await ready();await page.getByRole('button',{name:'短信验证码',exact:true}).click();await page.getByText(/不会自动接管公司账号展厅/).waitFor();await input('手机号').fill('13900009998');await page.getByRole('button',{name:'获取验证码',exact:true}).click();await page.getByRole('button',{name:/秒后重发/}).waitFor();const code=await page.locator('.merchant-phone-auth .notice b').textContent();assert.match(code,/^\d{6}$/);await input('验证码').fill(code);await input('验证码').press('Enter');await page.waitForURL(base+'/merchant');assert.notEqual(await page.evaluate(()=>sessionStorage.getItem('zqzt.merchant.id')),account.id);await page.getByRole('heading',{name:'完善入驻资料',exact:true}).waitFor();
  console.log('PASS development SMS send/countdown/Enter login with a separate phone account; no showroom takeover');
  assert.deepEqual(errors,[]);console.log('PASS no browser runtime errors; isolated database removed by Go test cleanup');
 }finally{await browser.close()}
})().catch(error=>{console.error(error);process.exit(1)});
