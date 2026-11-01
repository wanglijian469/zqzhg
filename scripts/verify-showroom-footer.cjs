// Read-only acceptance against the local website. Never submit content or contact requests.
const fs=require('node:fs'),assert=require('node:assert/strict');
const {chromium}=require('../.tools/browser-qa/node_modules/playwright');
const base='http://localhost:8080',dir='docs/screenshots/showroom-footer';
fs.mkdirSync(dir,{recursive:true});
(async()=>{
 const read=async path=>{const r=await fetch(base+'/api/v1'+path);assert.equal(r.status,200,path);return r.json()};
 const [vendors,config]=await Promise.all([read('/public/vendors?page_size=100'),read('/public/config')]);assert(vendors.items.length>0);assert.equal(vendors.items.length,vendors.total,'all published showrooms are included');
 const browser=await chromium.launch({headless:true,executablePath:'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',args:['--no-proxy-server']});
 const receipt={base,productionDeployed:false,contentWrites:false,publishedVendors:vendors.total,showrooms:[],layouts:[],platformPages:[],errors:[]};
 try{
 const ctx=await browser.newContext({viewport:{width:1440,height:900}});await ctx.grantPermissions(['clipboard-read','clipboard-write'],{origin:base});const p=await ctx.newPage();p.setDefaultTimeout(12000);p.on('pageerror',e=>receipt.errors.push(e.message));
 const path=v=>'/v/'+(v.showroom_slug||'v-'+v.id);
 const checkFooter=async(v,width)=>{
  await p.setViewportSize({width,height:900});await p.goto(base+path(v),{waitUntil:'domcontentloaded'});const footer=p.locator('.showroom-footer-simple');await footer.waitFor();await footer.scrollIntoViewIfNeeded();for(let settled=0;settled<8;settled++){await p.evaluate(()=>scrollTo(0,document.documentElement.scrollHeight));await p.waitForTimeout(100);const b=await footer.boundingBox();if(b.y+b.height<=await p.evaluate(()=>innerHeight)-70)break;}
  const text=await footer.innerText();assert(text.includes(v.data.name));assert(text.includes('由中企展馆提供展示服务'));assert(!/版权所有|技术支持|河北企元数智科技有限公司|入驻与服务说明|入驻使用指南|投诉与服务申请|分享官网/.test(text));
  assert.equal(await footer.getByRole('link',{name:'中企展馆',exact:true}).getAttribute('href'),'/');assert.equal(await footer.getByRole('link',{name:config.site.icp_number||'冀ICP备2026047360号-2',exact:true}).getAttribute('href'),'https://beian.miit.gov.cn/');
  assert.equal(await footer.getByRole('button',{name:'分享展厅',exact:true}).count(),1);assert.equal(await p.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false);
  const bounds=await footer.boundingBox();assert(bounds.height<=160,'compact footer: '+bounds.height);const quick=p.locator('.showroom-mobile-contact');if(width<=600&&await quick.isVisible()){const q=await quick.boundingBox();if(bounds.y+bounds.height>q.y+1)await p.screenshot({path:dir+'/contact-overlap.png'});assert(bounds.y+bounds.height<=q.y+1,'contact bar must not cover footer '+JSON.stringify({bounds,q}))}
  return {vendor:v.id,width,height:bounds.height,noOverflow:true};
 };
 for(const width of [1440,390])for(const v of vendors.items)receipt.showrooms.push(await checkFooter(v,width));
 const longest=[...vendors.items].sort((a,b)=>b.data.name.length-a.data.name.length)[0];
 for(const width of [1440,768,390,360]){receipt.layouts.push(await checkFooter(longest,width));await p.screenshot({path:dir+'/showroom-'+width+'.png',animations:'disabled'});if(width===390)await p.locator('.showroom-footer-simple').screenshot({path:dir+'/footer-390.png'})}
 // The old numeric entry still uses the same independent showroom footer.
 await p.goto(base+'/vendors/'+longest.id);await p.locator('.showroom-footer-simple').waitFor();receipt.legacyNumericRoute=true;
 await p.locator('.showroom-footer-simple').getByRole('button',{name:'分享展厅',exact:true}).click();const modal=p.getByRole('dialog');await modal.waitFor();assert.equal(await modal.getByRole('button',{name:'分享展厅',exact:true}).count(),1);assert.equal(await modal.getByRole('button',{name:'分享官网',exact:true}).count(),0);
 const address=await modal.getByRole('textbox',{name:'展厅地址',exact:true}).inputValue();await modal.getByRole('button',{name:'复制地址',exact:true}).click();assert.equal(await p.evaluate(()=>navigator.clipboard.readText()),address);
 await modal.getByRole('button',{name:'展厅二维码',exact:true}).click();await modal.locator('.share-qr').waitFor();assert((await modal.locator('.share-qr').getAttribute('src')).startsWith('data:image/png;base64,'));await modal.getByRole('button',{name:'生成海报预览',exact:true}).click();await modal.locator('.poster-preview img').waitFor();const downloaded=p.waitForEvent('download');await modal.getByRole('button',{name:'保存宣传海报',exact:true}).click();assert((await downloaded).suggestedFilename().endsWith('-poster.png'));await modal.getByRole('button',{name:'关闭',exact:true}).click();receipt.sharing={label:'分享展厅',copy:true,qr:true,posterPreview:true,posterDownload:true};
 await p.locator('.showroom-footer-source').getByRole('link',{name:'中企展馆',exact:true}).click();await p.locator('footer.footer').waitFor();assert.equal(new URL(p.url()).pathname,'/');
 for(const width of [1440,390]){
  await p.setViewportSize({width,height:900});await p.goto(base+'/');const footer=p.locator('footer.footer');await footer.waitFor();await footer.scrollIntoViewIfNeeded();for(let settled=0;settled<8;settled++){await p.evaluate(()=>scrollTo(0,document.documentElement.scrollHeight));await p.waitForTimeout(100);const b=await footer.boundingBox();if(b.y+b.height<=await p.evaluate(()=>innerHeight)-70)break;}const text=await footer.innerText();assert(text.includes('河北企元数智科技有限公司'));assert(!text.includes('技术支持'));
  for(const name of ['入驻与服务说明','入驻使用指南','投诉与服务申请',config.site.icp_number||'冀ICP备2026047360号-2'])assert.equal(await footer.getByRole('link',{name,exact:true}).count(),1,name);
  assert.equal(await p.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false);await p.screenshot({path:dir+'/platform-'+width+'.png'});await footer.getByRole('link',{name:'投诉与服务申请',exact:true}).click();await p.getByRole('heading',{name:'投诉与服务申请',exact:true}).waitFor();receipt.platformPages.push({width,fullFooter:true,complaintEntry:true});
 }
 for(const url of ['/guide','/services','/join','/merchant/login']){await p.goto(base+url);await p.getByRole('link',{name:'入驻使用指南',exact:true}).waitFor();receipt.platformPages.push({path:url,guideLink:true})}
 assert.deepEqual(receipt.errors,[]);receipt.passed=true;
 }finally{await browser.close();fs.mkdirSync('docs/test-output',{recursive:true});fs.writeFileSync('docs/test-output/showroom-footer.json',JSON.stringify(receipt,null,2))}
 console.log(JSON.stringify(receipt,null,2));
})().catch(e=>{console.error(e.stack);process.exitCode=1});
