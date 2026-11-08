const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {chromium} = require('../.tools/browser-qa/node_modules/playwright');

const [base, reference, reportPath] = process.argv.slice(2);
assert.equal(new URL(base).hostname, '127.0.0.1', 'Use the isolated candidate proxy');
assert(reference && reportPath, 'Supply the development boundary snapshot and receipt paths');
const company = '河北丹顿液压科技有限公司';
const showroom = '/v/v-21';
const canonical = 'https://zqzhg.com' + showroom;
const regions = [
  {code: '', name: '全国', count: 12},
  {code: '13', name: '河北', count: 11},
  {code: '1305', name: '邢台', count: 9},
  {code: '130528', name: '宁晋', count: 9}
];
const regionQuery = code => code ? '?region_code=' + code : '';

async function main() {
  const browser = await chromium.launch({
    executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe',
    headless: true,
    args: ['--no-proxy-server']
  });
  const evidence = {base, canonical, initialHTML: false, browsers: [], boundaries: [], retry: false};
  const errors = [];
  try {
    for (const width of [390, 1440]) {
      const context = await browser.newContext({javaScriptEnabled: false, ignoreHTTPSErrors: true, viewport: {width, height: 844}});
      const page = await context.newPage();
      await page.goto(base);
      await page.locator('[data-seo-content] a[href="/regions"]').click();
      for (const region of regions.slice(1)) {
        const link = '/regions' + regionQuery(region.code);
        await page.locator(`[data-seo-content] a[href="${link}"]`).click();
      }
      await page.locator(`[data-seo-content] a[href="${showroom}"]`).click();
      assert.equal(new URL(page.url()).pathname, showroom);
      await page.getByRole('heading', {name: company, exact: true}).waitFor();
      assert.equal(await page.title(), company + ' - 企业展厅 - 中企展馆');
      assert.equal(await page.locator('link[rel=canonical]').getAttribute('href'), canonical);
      assert.match(await page.locator('meta[name=description]').getAttribute('content'), /液压/);
      assert.match(await page.locator('[data-seo-content]').innerText(), /液压油缸/);
      assert.equal(await page.getByRole('navigation', {name: '面包屑导航'}).count(), 0);
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      evidence.initialHTML = true;
      evidence.browsers.push({width, javaScript: false, regionalLinks: true, showroom: true});
      await context.close();
    }

    for (const width of [390, 1440]) {
      const context = await browser.newContext({ignoreHTTPSErrors: true, viewport: {width, height: 844}});
      await context.route('**/*', route => new URL(route.request().url()).origin === base ? route.continue() : route.abort());
      const page = await context.newPage();
      page.on('pageerror', error => errors.push(error.message));
      assert.equal((await page.goto(base + showroom)).status(), 200);
      await page.locator('.showroom-v2 h1').waitFor();
      assert.equal(await page.locator('.showroom-v2 h1').innerText(), company);
      assert.equal((await page.reload()).status(), 200);
      await page.locator('.showroom-v2 h1').waitFor();
      assert.equal(await page.locator('.public-detail-breadcrumb').count(), 0);
      await page.locator('.sr-hall-back').click();
      await page.locator('.hero-actions a[href="/browse"]').click();
      await page.waitForURL(base + '/browse');
      await page.locator('.vendor-brand-list a[href="/v/v-21"]').first().click();
      await page.waitForURL(base + showroom);
      await page.locator('.showroom-v2 h1').waitFor();
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));

      const ready = () => page.locator('[data-map-mode=outline] .industry-chart svg path').first().waitFor({state: 'attached'});
      const mapURL = base + '/regions';
      await page.goto(mapURL);
      await ready();
      if (width === 1440) {
        for (const region of regions) {
          if (region.code) {
            await page.locator('.industry-chart svg text').filter({hasText: new RegExp('^' + region.name + '[省市县]?$')}).first().click();
            await page.waitForURL(url => url.searchParams.get('region_code') === region.code);
            await ready();
          }
          const query = regionQuery(region.code);
          const get = async url => {
            const response = await context.request.get(base + '/api/v1/public/' + url + query);
            assert.equal(response.status(), 200, url + query);
            return response.json();
          };
          const [boundary, stats, vendors] = await Promise.all([
            get('map-demo/regions'), get('industry-map/regions'), get('industry-map/vendors')
          ]);
          const original = JSON.parse(fs.readFileSync(path.join(reference, (region.code || 'national') + '.mapping.json'), 'utf8'));
          assert.equal(boundary.sha256, original.sha256);
          assert.deepEqual(boundary.geometry, original.geometry);
          assert.equal(stats.total, region.count);
          assert.equal(vendors.total, region.count);
          await page.waitForFunction(count => document.querySelector('.industry-panel-heading b')?.textContent === String(count), region.count);
          evidence.boundaries.push({code: region.code, sha256: boundary.sha256, count: stats.total});
        }
      } else {
        const glob = '**/api/v1/public/map-demo/regions*';
        const fail = route => route.fulfill({status: 503, body: '{}'});
        await page.route(glob, fail);
        await page.reload();
        const retry = page.getByRole('button', {name: '重试', exact: true});
        await retry.waitFor();
        await page.unroute(glob, fail);
        await retry.click();
        await ready();
        evidence.retry = true;
      }
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      evidence.browsers.push({width, javaScript: true, refresh: true, showroomNavigation: true, mapLayout: true});
      await context.close();
    }
    assert.deepEqual(errors, []);
    fs.writeFileSync(reportPath, JSON.stringify(evidence, null, 2));
    process.stdout.write(JSON.stringify(evidence) + '\n');
  } finally {
    await browser.close();
  }
}

main().catch(error => {
  console.error(error);
  process.exitCode = 1;
});
