import fs from 'node:fs';
import assert from 'node:assert/strict';
const base=process.env.ZQZT_VERIFY_BASE||'http://localhost:8080';
const config=Object.fromEntries(fs.readFileSync('server/.env','utf8').split(/\r?\n/).filter(line=>line.trim()&&!line.trim().startsWith('#')).map(line=>{const p=line.indexOf('=');return [line.slice(0,p).trim(),line.slice(p+1).trim().replace(/^"|"$/g,'')]}));
let session;
async function request(path,method='GET',data){const response=await fetch(base+'/api/v1'+path,{method,headers:{'Content-Type':'application/json',...(session?{Authorization:'Bearer '+session}:{})},...(data?{body:JSON.stringify(data)}:{})});const result=await response.json();assert.ok(response.ok,path+' '+response.status+' '+JSON.stringify(result));return result}
try{
 const publicBefore=await request('/public/vendors/1');assert.equal(publicBefore.item.data.name,'河北冀农农机具有限公司');assert.equal(publicBefore.products.total,19);
 session=(await request('/auth/admin','POST',{username:config.ADMIN_USERNAME,password:config.ADMIN_PASSWORD})).token;
 let current=await request('/admin/contents/1');assert.notEqual(current.status,'pending','Do not overwrite an existing pending submission');
 if(current.showroom_slug!=='jinong')await request('/admin/vendors/1/showroom-slug','PUT',{slug:'jinong'});
 const original=publicBefore.item.data;const marker='\n\n企业信息';assert.ok(original.description.includes(marker),'The source must contain the captured company facts');
 const data={...original,description:original.description.split(marker)[0].trim(),company_profile:{short_name:'冀农',short_name_en:'',summary:'以质量铸品质，以信誉创辉煌',summary_en:'',logo_media_id:0,cover_media_id:original.media_ids[0],established:'1985',staff:'200余人',factory_area:'30000平方米',equipment:'拥有各种生产设备180台套',equipment_en:'',capacity:'完全满足农机生产',capacity_en:'',website:'https://www.hbjinong.com/',contact_name:'耿建虎',contact_name_en:'',contacts:[
 {area:'河北、北京、天津、内蒙古、山东、青海',name:'耿建虎经理',phone:'13482960032'},
 {area:'黑龙江、河南、宁夏、湖北、江西、邯郸、湖南、浙江、福建',name:'郭京献经理',phone:'13482960035'},
 {area:'四川、重庆、云南、广西、广东、贵阳、海南',name:'郭京献经理',phone:'13482960035'},
 {area:'四川、重庆、云南、广西、广东、贵阳、海南',name:'刘子旋经理',phone:'18732981172'},
 {area:'安徽、江苏、南疆、上海',name:'谷军杰经理',phone:'13932950638'},
 {area:'陕西、宁夏',name:'郭玉刚经理',phone:'18333987205'},
 {area:'山西、北疆、甘肃、西藏海外出口',name:'冯磊经理',phone:'18330988833'},
 {area:'辽宁、吉林',name:'刘子旋经理',phone:'18732981172'}
 ]}};
 current=await request('/admin/contents/1/draft','PUT',{revision_id:current.revision_id,data});
 const old=await request('/public/showrooms/jinong');assert.equal(old.item.revision_id,publicBefore.item.revision_id,'Saving must preserve the public version');
 await request('/admin/contents/1/submit','POST',{revision_id:current.revision_id});
 const pending=await request('/public/showrooms/jinong');assert.equal(pending.item.revision_id,publicBefore.item.revision_id,'Submission must preserve the public version');
 await request('/admin/revisions/'+current.revision_id+'/review','POST',{decision:'approve',note:'按已采集的冀农真实资料整理企业宣传、实力及区域联系方式；不新增产品或图片。'});
 const published=await request('/public/showrooms/jinong');assert.equal(published.item.data.company_profile.contacts.length,8);assert.equal(published.product_count,19);
 fs.writeFileSync('docs/test-output/jinong-showroom-publication.json',JSON.stringify({at:new Date().toISOString(),vendor_id:1,old_revision:publicBefore.item.revision_id,new_revision:published.item.revision_id,slug:published.item.showroom_slug,contacts:8,product_count:19,new_products:0,new_media:0,steps:['save_draft','verify_old_public','submit','verify_old_public','approve','verify_new_public']},null,2)+'\n');
 console.log('冀农宣传资料已通过独立保存、提交、审核发布；19款产品和原图片保持原状。');
}finally{if(session)await request('/auth/logout','POST',{})}
