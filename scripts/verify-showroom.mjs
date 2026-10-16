import fs from 'node:fs';
import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
const require=createRequire(new URL('../web/package.json',import.meta.url));const {PNG}=require('pngjs');const jsQR=require('jsqr');
const base=process.env.ZQZT_VERIFY_BASE||'http://localhost:8080';
async function get(path){const response=await fetch(base+'/api/v1'+path);assert.equal(response.status,200,path);return response.json()}
const showroom=await get('/public/showrooms/jinong');assert.equal(showroom.item.data.name,'河北冀农农机具有限公司');assert.equal(showroom.product_count,19);assert.equal(showroom.item.data.company_profile.contacts.length,8);
const first=await get('/public/products?vendor_id=1&page_size=12');const second=await get('/public/products?vendor_id=1&page_size=12&page=2');assert.equal(first.items.length,12);assert.equal(second.items.length,7);assert.equal(new Set([...first.items,...second.items].map(p=>p.id)).size,19);
const model=await get('/public/products?vendor_id=1&q=440');assert.equal(model.total,1);assert.equal(model.items[0].vendor.showroom_slug,'jinong');
assert.equal((await get('/public/products?vendor_id=1&q=MODEL-NOT-FOUND')).total,0);
const parent=await get('/public/products?vendor_id=1&category_id=1');assert.equal(parent.total,19);
const old=await get('/public/vendors/1');assert.equal(old.item.showroom_slug,'jinong');assert.equal((await get('/public/showrooms/v-1')).item.id,1);
for(const path of ['/v/jinong','/vendors/1','/v/v-1']){const response=await fetch(base+path);assert.equal(response.status,200);const html=await response.text();assert.ok(html.includes('rel="canonical" href="'+base+'/v/jinong"'));assert.ok(html.includes('og:title'));assert.ok(html.includes('og:image'));assert.ok(!html.includes('root/root'))}
const images=new Set();for(const product of [...first.items,...second.items])for(const id of product.data.media_ids||[])images.add(id);for(const id of showroom.item.data.media_ids||[])images.add(id);
for(const id of images){for(const suffix of ['', '?thumb=1']){const r=await fetch(base+'/api/v1/media/'+id+suffix);assert.equal(r.status,200,'image '+id);assert.ok(r.headers.get('content-type').startsWith('image/'));assert.ok((await r.arrayBuffer()).byteLength>0)}}
const artifacts=[];for(const file of ['jinong-qr.png','jinong-poster.png']){const img=PNG.sync.read(fs.readFileSync('docs/screenshots/'+file));const decoded=jsQR(new Uint8ClampedArray(img.data),img.width,img.height);assert.equal(decoded?.data,base+'/v/jinong');artifacts.push({file,width:img.width,height:img.height,qr:decoded.data})}
const result={at:new Date().toISOString(),vendor_id:1,public_products:19,unique_images:images.size,original_and_thumbnail_checks:images.size*2,search_model:'PASS',parent_category:'PASS',pagination:'PASS',old_urls:'PASS',metadata:'PASS',artifacts,production_test_vendors_created:0};
fs.writeFileSync('docs/test-output/showroom-runtime-verification.json',JSON.stringify(result,null,2)+'\n');console.log(JSON.stringify(result,null,2));
