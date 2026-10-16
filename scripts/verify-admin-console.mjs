import fs from 'node:fs';
import assert from 'node:assert/strict';
const base=process.env.ZQZT_VERIFY_BASE||'http://127.0.0.1:8080';
const config=Object.fromEntries(fs.readFileSync('server/.env','utf8').split(/\r?\n/).filter(line=>line.trim()&&!line.trim().startsWith('#')).map(line=>{const p=line.indexOf('=');return [line.slice(0,p).trim(),line.slice(p+1).trim().replace(/^"|"$/g,'')]}));
let session;
async function request(path,options={},auth=true){const headers={'Content-Type':'application/json',...(auth&&session?{Authorization:'Bearer '+session}:{})};const response=await fetch(base+'/api/v1'+path,{...options,headers});assert.equal(response.status,200,path);return response.json()}
try{
 const login=await request('/auth/admin',{method:'POST',body:JSON.stringify({username:config.ADMIN_USERNAME||'admin',password:process.env.ZQZT_VERIFY_ADMIN_PASSWORD||config.ADMIN_PASSWORD})},false);session=login.token;
 const dashboard=await request('/admin/dashboard');assert.equal(dashboard.counts.vendor_total,1);assert.equal(dashboard.counts.product_total,19);assert.equal(dashboard.counts.vendor_public,1);assert.equal(dashboard.counts.product_public,19);
 const vendor=await request('/admin/contents/1');assert.equal(vendor.data.name,'河北冀农农机具有限公司');assert.equal(vendor.product_count,19);assert.ok(vendor.published);
 const products=await request('/admin/contents?kind=product&vendor_id=1');assert.equal(products.total,19);assert.ok(products.items.every(item=>item.owner_id===vendor.owner_id&&item.vendor.name===vendor.data.name&&item.published));
 const filtered=await request('/admin/contents?kind=product&q=1LF-440&category_id=1&vendor_id=1&public_state=public');assert.equal(filtered.total,1);assert.equal(filtered.items[0].id,2);
 const accounts=await request('/admin/accounts');assert.equal(accounts.total,1);assert.equal(accounts.items[0].vendor_id,1);assert.equal(accounts.items[0].product_count,19);
 const logs=await request('/admin/operation-logs');assert.ok(logs.total>=40);assert.ok(logs.items.every(item=>item.actor_name));
 const status=await request('/admin/system');assert.equal(status.database_ok,true);assert.equal(status.media_ok,true);assert.equal(status.product_limit,1000);
 for(const path of ['/admin/vendor-reviews','/admin/dashboard','/admin/products','/admin/contents/2','/admin/extensions/analytics']){const result=await fetch(base+path);assert.equal(result.status,200);assert.ok((await result.text()).includes('id="root"'))}
 for(const path of ['/admin/dashboard','/admin/accounts','/admin/operation-logs','/admin/system']){const result=await fetch(base+'/api/v1'+path);assert.equal(result.status,401)}
 const result={verifiedAt:new Date().toISOString(),base,counts:dashboard.counts,vendor:'河北冀农农机具有限公司',productOwnership:'PASS',publishedComparisons:'PASS',modelAndParentCategoryFilters:'PASS',accounts:'PASS',auditLogCount:logs.total,systemStatus:'PASS',privateAPIs:'PASS',deepRoutes:'PASS',productionBusinessWrites:0};
 fs.writeFileSync('docs/test-output/admin-runtime-verification.json',JSON.stringify(result,null,2)+'\n');console.log(JSON.stringify(result,null,2));
}finally{if(session)await fetch(base+'/api/v1/auth/logout',{method:'POST',headers:{Authorization:'Bearer '+session,'Content-Type':'application/json'},body:'{}'})}
