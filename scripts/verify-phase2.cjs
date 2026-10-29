// Invoked only by TestProcurementBrowser, whose server and data are isolated.
const assert=require('node:assert/strict');
const fs=require('node:fs');
const zlib=require('node:zlib');

module.exports=async({browser,fixture:f,context,layout,checks})=>{
 const network=[];
 for(const width of [1920,1366,390]){
  const guest=await context(null,width);
  const js=new Set();guest.p.on('request',r=>{if(r.resourceType()==='script')js.add(r.url())});
  await guest.p.goto(f.base+'/');await guest.p.locator('.exhibition-stage').waitFor();
  await guest.p.waitForLoadState('networkidle');
  assert(![...js].some(url=>/\/(admin|merchant-console|procurement|auth-routes)-[^/]+\.js/.test(url)),'Unrelated business chunk loaded on homepage');
  let bytes=0,gzip=0;
  for(const url of js){const response=await guest.c.request.get(url);assert.equal(response.status(),200);const raw=await response.body();bytes+=raw.length;gzip+=zlib.gzipSync(raw).length}
  network.push({width,jsRequests:js.size,bytes,gzip});
  const missing=await guest.p.goto(f.base+'/phase2-unknown-page');assert.equal(missing.status(),404);
  await guest.p.locator('meta[name=robots][content="noindex,follow"]').waitFor({state:'attached'});
  assert.equal(await guest.p.locator('link[rel=canonical]').count(),1);
  await layout(guest.p,'phase2-404',width,'zh');
  for(const path of ['/admin/login','/admin/procurement/reviews','/merchant/opportunities','/buyer/new','/buyer/opportunities/settings?x=1']){
   const response=await guest.c.request.get(f.base+path);
   assert.equal(response.status(),200,path);assert.equal(response.headers()['x-robots-tag'],'noindex,follow');assert.equal(response.headers()['cache-control'],'no-store');
  }
  await guest.p.goto(f.base+'/merchant/login?next='+encodeURIComponent('/merchant/opportunities'));
  const calls=[];guest.p.on('request',r=>{if(r.method()==='POST')calls.push(new URL(r.url()).pathname)});
  await guest.p.getByRole('button',{name:'短信验证码',exact:true}).click();
  if(width===1920){
  await guest.p.getByLabel('手机号',{exact:true}).fill(f.buyer.phone);
  await guest.p.getByRole('button',{name:'获取验证码',exact:true}).click();
  const code=await guest.p.getByLabel('本机模拟验证码',{exact:true}).innerText();
  assert(/^\d{6}$/.test(code));await guest.p.getByLabel('验证码',{exact:true}).fill(code);
  // Do not capture screenshots while a verification code is visible.
  await guest.p.getByRole('button',{name:'登录',exact:true}).click();
  await guest.p.getByRole('button',{name:'使用同一账号入驻厂商',exact:true}).waitFor();
  assert.equal(new URL(guest.p.url()).pathname,'/merchant/opportunities');
  assert(calls.includes('/api/v1/auth/phone/login')&&calls.includes('/api/v1/auth/phone/code'));
  assert(!calls.includes('/api/v1/auth/sms')&&!calls.includes('/api/v1/auth/phone'));
  await layout(guest.p,'phase2-same-account-guidance',width,'zh');
  await guest.p.goto(f.base+'/join');await guest.p.getByRole('button',{name:'使用同一账号入驻厂商',exact:true}).waitFor();
  assert.equal(new URL(guest.p.url()).pathname,'/merchant');
  }else await layout(guest.p,'phase2-sms-login',width,'zh');
  await guest.c.close();

  const supplier=await context(f.supplier1,width);
  await supplier.p.goto(f.base+'/merchant/products/'+f.product_id+'/edit');
  const name=supplier.p.getByRole('textbox',{name:'产品名称',exact:true});await name.waitFor();
  await supplier.p.getByLabel('产品资料完整度',{exact:true}).waitFor();
  const original=await name.inputValue();
  assert(original.length>0);assert.equal(await supplier.p.getByRole('textbox',{name:'产品型号',exact:true}).count(),1);
  await layout(supplier.p,'phase2-product-quality',width,'zh');
  await supplier.p.getByRole('button',{name:'Switch to English',exact:true}).click();
  await supplier.p.getByLabel('Product information completeness',{exact:true}).waitFor();
  assert.equal(await supplier.p.getByRole('textbox',{name:'Product name',exact:true}).inputValue(),original);
  await layout(supplier.p,'phase2-product-quality',width,'en');
  await supplier.c.close();
  const reviewer=await context({token:f.admin},width,'zh',true);
  await reviewer.p.goto(f.base+'/admin/contents/'+f.product_id);
  await reviewer.p.getByLabel('产品资料完整度',{exact:true}).waitFor();
  await reviewer.p.getByText('审核请人工核对封面、名称、型号与参数是否对应；完整度仅表示字段填写情况，不代替审核结论。',{exact:true}).waitFor();
  await layout(reviewer.p,'phase2-product-review-quality',width,'zh');
  await reviewer.c.close();
 }
 // Transient route chunk failure is handled inside the application and can be retried.
 const retry=await context(null,1366);let blocked=false;
 await retry.p.route('**/assets/admin-*.js',async route=>{if(!blocked){blocked=true;await route.abort('failed')}else await route.continue()});
 await retry.p.goto(f.base+'/admin/login');
 await retry.p.getByText('页面资源加载失败，请检查网络后重试。',{exact:true}).waitFor();
 await retry.p.getByRole('button',{name:'重试',exact:true}).click();
 await retry.p.getByRole('textbox',{name:/账号|用户名/}).waitFor();
 await retry.c.close();
 checks.push('Phase 2: 1920/1366/390 viewports, unknown HTTP 404 and client noindex, lawful private deep links, unified SMS login without implicit supplier registration, unchanged product data with bilingual quality guidance, route chunk retry');
 fs.writeFileSync('.local/phase2-home-network.json',JSON.stringify(network,null,2));
};
