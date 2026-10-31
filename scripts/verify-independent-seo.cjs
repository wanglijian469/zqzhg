const fs = require('fs');
const path = require('path');
const assert = require('assert/strict');
const {chromium} = require('../.tools/browser-qa/node_modules/playwright');

(async () => {
  const [base, productPath] = process.argv.slice(2);
  assert(base && productPath, 'Usage: node verify-independent-seo.cjs BASE PRODUCT_PATH');
  const out = process.env.SEO_SCREENSHOTS ? path.resolve(process.env.SEO_SCREENSHOTS) : path.resolve(__dirname, '../docs/screenshots/independent-seo');
  fs.mkdirSync(out, {recursive: true});
  const browser = await chromium.launch({headless: true, executablePath: 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe', args: ['--no-proxy-server']});
  const checks = [], errors = [];
  const companyPath = '/v/seo-browser';
  async function noOverflow(page) {
    const layout=await page.evaluate(()=>({width:innerWidth,total:document.documentElement.scrollWidth,path:location.pathname,lang:document.documentElement.lang,overflow:[...document.querySelectorAll('body *')].filter(el=>{const r=el.getBoundingClientRect();return r.width>0&&(r.right>innerWidth+1||r.left< -1)}).slice(0,8).map(el=>({tag:el.tagName,className:el.className,text:el.textContent.slice(0,100)}))}));
    if(layout.total>layout.width+1)await page.screenshot({path:path.join(out,`overflow-${layout.width}-${layout.lang}.png`),fullPage:false});
    assert(layout.total<=layout.width+1,JSON.stringify(layout));
  }
  async function metadata(page, canonical, robots) {
    await page.waitForFunction(({canonical, robots}) => document.querySelector('link[rel=canonical]')?.href === canonical && document.querySelector('meta[name=robots]')?.content === robots, {canonical, robots});
    assert.equal(await page.locator('link[rel=canonical]').count(), 1);
    assert.equal(await page.locator('meta[name=robots]').count(), 1);
    assert.equal(await page.locator('meta[name=description]').count(), 1);
    assert.equal(await page.locator('meta[name=keywords]').count(), 1);
    assert.equal(await page.locator('title').count(), 1);
  }
  try {
    for(const width of [360,390,430,1366,1920])for(const lang of ['zh','en']){
      const context=await browser.newContext({viewport:{width,height:950}});
      await context.addInitScript(lang=>localStorage.setItem('zqzt.language',lang),lang);
      const page=await context.newPage();
      page.on('pageerror',error=>errors.push(error.message));
      for(const route of ['/','/browse']){
        const response=await page.goto(base+route);
        assert.equal(response.status(),200);
        const initial=await response.text();
        assert(initial.includes('企业展厅')&&initial.includes('data-seo-content'),'Missing initial public search content');
        await page.locator(route==='/'?'.hero-search':'.browse-page').waitFor();
        await metadata(page,base+route,'index,follow');
        assert((await page.locator('meta[name=keywords]').getAttribute('content')).includes('企业展厅'));
        if(lang==='zh'){
          const server=await page.evaluate(raw=>{const doc=new DOMParser().parseFromString(raw,'text/html');return {title:doc.title,description:doc.querySelector('meta[name=description]')?.content,keywords:doc.querySelector('meta[name=keywords]')?.content}},initial);
          assert.equal(await page.title(),server.title);
          assert.equal(await page.locator('meta[name=description]').getAttribute('content'),server.description);
          assert.equal(await page.locator('meta[name=keywords]').getAttribute('content'),server.keywords);
        }
        await noOverflow(page);
        await page.screenshot({path:path.join(out,`${route==='/'?'home':'directory'}-${width}-${lang}.png`),fullPage:true});
      }
      checks.push({width,lang,landingMetadata:true,initialHTML:true,overflow:false});
      await context.close();
    }
    for (const width of [390, 1440]) {
      const context = await browser.newContext({javaScriptEnabled: false, viewport: {width, height: 950}});
      const page = await context.newPage();
      const response = await page.goto(base + companyPath);
      assert.equal(response.status(), 200);
      await page.locator('[data-seo-content] h1').waitFor();
      assert((await page.locator('body').innerText()).includes('无需执行脚本也能读取的企业介绍'));
      await metadata(page, base + companyPath, 'index,follow');
      await noOverflow(page);
      await page.screenshot({path: path.join(out, `company-no-js-${width}.png`), fullPage: true});
      await page.locator(`a[href="${productPath}"]`).click();
      assert((await page.locator('body').innerText()).includes('功率：80千瓦'));
      await metadata(page, base + productPath, 'index,follow');
      await noOverflow(page);
      await page.screenshot({path: path.join(out, `product-no-js-${width}.png`), fullPage: true});
      await page.locator(`a[href="${companyPath}"]`).click();
      assert.equal(new URL(page.url()).pathname, companyPath);
      checks.push({width, javaScript: false, companyAndProductBody: true, crossLinks: true, overflow: false});
      await context.close();
    }
    for (const width of [390, 1440]) for (const lang of ['zh', 'en']) {
      const context = await browser.newContext({viewport: {width, height: 950}});
      await context.addInitScript(lang => localStorage.setItem('zqzt.language', lang), lang);
      const page = await context.newPage();
      page.on('pageerror', error => errors.push(error.message));
      await page.goto(base + companyPath);
      await page.locator('.showroom-v2').waitFor();
      assert.equal(await page.locator('[data-seo-content]').count(), 0);
      assert.equal(await page.locator('h1').count(), 1);
      await metadata(page, base + companyPath, 'index,follow');
      await noOverflow(page);
      await page.screenshot({path: path.join(out, `company-js-${width}-${lang}.png`), fullPage: true});
      await page.getByRole('tab').nth(2).click();
      await page.locator(`#panel-products .showroom-product-grid a[href*="${productPath}"]`).first().click();
      await page.locator('.public-product-detail h1').waitFor();
      await metadata(page, base + productPath, 'index,follow');
      await noOverflow(page);
      await page.screenshot({path: path.join(out, `product-js-${width}-${lang}.png`), fullPage: true});
      await page.locator('.public-product-detail .back').click();
      await page.locator('.showroom-v2').waitFor();
      await metadata(page, base + companyPath, 'index,follow');
      await page.goto(base + companyPath + '?tab=products&page=2');
      await page.locator('.showroom-v2').waitFor();
      await metadata(page, base + companyPath, 'noindex,follow');
      checks.push({width, lang, javaScript: true, rootReplaced: true, navigation: true, canonical: true, paginatedNoindex: true, overflow: false});
      await context.close();
    }
    assert.equal(errors.length, 0, JSON.stringify(errors));
    const report = {checks, errors, isolatedDatabase: true, physicalDeviceVerified: false};
    fs.writeFileSync(process.env.SEO_REPORT||path.resolve(__dirname, '../docs/test-output/seo-browser.json'), JSON.stringify(report, null, 2));
    console.log(JSON.stringify(report));
  } finally {
    await browser.close();
  }
})().catch(error => {console.error(error); process.exit(1);});
