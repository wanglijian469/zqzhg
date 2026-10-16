import assert from 'node:assert/strict';
import fs from 'node:fs/promises';

const base=process.env.API_BASE_URL||'http://localhost:8080';
const get=async path=>{
  const response=await fetch(base+'/api/v1'+path);
  assert.equal(response.status,200,path);
  return response.json();
};
const vendors=await get('/public/vendors?q='+encodeURIComponent('河北冀农农机具有限公司'));
assert.equal(vendors.total,1);
const vendor=vendors.items[0];
const all=await get('/public/products?vendor_id='+vendor.id+'&page_size=100');
assert.equal(all.total,19,'existing collected products remain intact');
for(const product of all.items){
  assert.equal(product.vendor.id,vendor.id);
  assert.equal(product.vendor.name,vendor.data.name);
  assert.equal(product.vendor.region_id,vendor.data.region_id);
  assert.equal(product.vendor.region_name,vendor.data.region_name);
  assert.ok(product.public_visible);
  assert.ok(product.data.media_ids.length>0);
  const thumbnail=await fetch(base+'/api/v1/media/'+product.data.media_ids[0]+'?thumb=1');
  assert.equal(thumbnail.status,200);
  assert.match(thumbnail.headers.get('content-type'),/^image\//);
  assert.ok((await thumbnail.arrayBuffer()).byteLength>0);
}
const filter='&category_id=1&region_id='+vendor.data.region_id;
assert.equal((await get('/public/products?q=1LF-440'+filter)).total,1);
assert.equal((await get('/public/products?category_id=1&region_id='+vendor.data.region_id)).total,19);
assert.equal((await get('/public/products?q='+encodeURIComponent('不存在的产品型号V0001')+filter)).total,0);
const first=await get('/public/products?page_size=5&region_id='+vendor.data.region_id);
const second=await get('/public/products?page_size=5&page=2&region_id='+vendor.data.region_id);
assert.equal(first.total,19);assert.equal(first.items.length,5);assert.equal(second.items.length,5);
assert.equal(new Set([...first.items,...second.items].map(p=>p.id)).size,10);
const result={verifiedAt:new Date().toISOString(),base,products:all.total,publicVendorSummaries:'PASS',productThumbnailsChecked:all.items.length,modelCategoryRegionSearch:'PASS',emptyResults:'PASS',pagination:'PASS',productionDataChanges:0};
await fs.writeFile('docs/test-output/product-browse-public-verification.json',JSON.stringify(result,null,2)+'\n');
console.log(JSON.stringify(result,null,2));
