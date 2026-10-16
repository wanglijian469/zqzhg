import fs from 'node:fs/promises';
import assert from 'node:assert/strict';

const base = process.env.API_BASE_URL || 'http://localhost:8080';
const manifest = JSON.parse(await fs.readFile('.local/imports/V0001.json', 'utf8'));
const get = async path => {
  const response = await fetch(base + '/api/v1' + path);
  assert.equal(response.status, 200, path);
  return response.json();
};
const health = await get('/health');
const search = await get('/public/vendors?q=' + encodeURIComponent(manifest.vendor.data.name));
assert.equal(search.total, 1, 'one imported booth, no duplicate');
const vendorId = search.items[0].id;
const detail = await get('/public/vendors/' + vendorId + '?page_size=100');
assert.equal(detail.item.data.phone, manifest.vendor.data.phone);
assert.equal(detail.products.total, manifest.products.length);
const expected = new Map(manifest.products.map(p => [p.data.name, p]));
const media = new Set(detail.item.data.media_ids);
for (const product of detail.products.items) {
  const original = expected.get(product.data.name);
  assert.ok(original, 'unexpected product: ' + product.data.name);
  assert.equal(product.data.parameters, original.data.parameters);
  const result = await get('/public/products/' + product.id);
  assert.equal(result.vendor.id, vendorId);
  assert.equal(result.item.data.media_ids.length, original.media_paths.length);
  result.item.data.media_ids.forEach(id => media.add(id));
}
const expectedImages = [manifest.vendor, ...manifest.products].reduce((n, p) => n + p.media_paths.length, 0);
assert.equal(media.size, expectedImages);
for (const id of media) {
  for (const suffix of ['', '?thumb=1']) {
    const response = await fetch(base + '/api/v1/media/' + id + suffix);
    assert.equal(response.status, 200, 'media ' + id + suffix);
    assert.match(response.headers.get('content-type'), /^image\//);
    assert.ok((await response.arrayBuffer()).byteLength > 0);
  }
}
const taxonomy = await get('/public/taxonomy');
for (const name of ['翻转犁', '驱动耙', '耕整地机械']) {
  const category = taxonomy.categories.find(c => c.name === name);
  assert.ok(category);
  const filtered = await get('/public/vendors?category_id=' + category.id + '&region_id=' + detail.item.data.region_id);
  assert.equal(filtered.total, 1, 'category/region vendor deduplication');
}
assert.equal((await get('/public/vendors?q=1LF-440')).total, 1);
assert.equal((await get('/public/vendors?q=不存在的产品型号V0001')).total, 0);
const home = await get('/public/home');
assert.ok(home.vendors.some(v => v.id === vendorId));
assert.ok(home.products.length > 0);
assert.equal((await fetch(base + '/api/v1/merchant/contents')).status, 401);
const result = {verifiedAt: new Date().toISOString(), base, health, vendorId, products: detail.products.total, images: media.size, mediaChecks: media.size * 2, featuredProducts: home.products.length, searchAndTaxonomy: 'PASS', contactPhone: detail.item.data.phone, privateEntryProtection: 'PASS'};
await fs.writeFile('docs/test-output/V0001-public-verification.json', JSON.stringify(result, null, 2));
console.log(JSON.stringify(result, null, 2));
