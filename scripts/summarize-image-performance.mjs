import fs from 'node:fs';
import path from 'node:path';
import {gunzipSync} from 'node:zlib';

const read=mode=>JSON.parse(fs.readFileSync('.local/lightweight/'+mode+'-browser.json','utf8'));
const baseline=read('baseline'),optimized=read('optimized');
for(const data of [baseline,optimized])if(data.errors.length||data.rows.length!==40||data.rows.filter(r=>r.weak).length!==25)throw Error('Incomplete performance run: '+data.mode);
const median=values=>[...values].sort((a,b)=>a-b)[Math.floor(values.length/2)];
const reduction=(a,b)=>Math.round((1-b/a)*10000)/100;
const names={home:'首页',vendors:'厂商列表',products:'产品列表',showroom:'企业展厅',product:'产品详情'};
const pages=Object.entries(names).map(([key,name])=>{
 const before=baseline.rows.find(r=>r.page===key&&r.width===390&&!r.weak),after=optimized.rows.find(r=>r.page===key&&r.width===390&&!r.weak);
 const oldLCP=median(baseline.rows.filter(r=>r.page===key&&r.weak).map(r=>r.cold.lcp)),newLCP=median(optimized.rows.filter(r=>r.page===key&&r.weak).map(r=>r.cold.lcp));
 return {page:key,name,mobileImageBytesBefore:before.cold.imageBytes,mobileImageBytesAfter:after.cold.imageBytes,imageReductionPercent:reduction(before.cold.imageBytes,after.cold.imageBytes),weakLCPBeforeMs:oldLCP,weakLCPAfterMs:newLCP,weakLCPImprovementPercent:reduction(oldLCP,newLCP),warmImageStatuses:after.warm.statuses.map(s=>s.status)};
});
const sum=data=>data.rows.filter(r=>!r.weak).reduce((total,row)=>total+row.cold.imageBytes,0);
const aggregateImageReductionPercent=reduction(sum(baseline),sum(optimized)),maxCLS=Math.max(...optimized.rows.map(r=>r.cold.cls));
if(aggregateImageReductionPercent<40||maxCLS>0.1)throw Error('Performance gate failed');
let gzipAssets=0;
function check(dir){for(const entry of fs.readdirSync(dir,{withFileTypes:true})){const file=path.join(dir,entry.name);if(entry.isDirectory())check(file);else if(entry.name.endsWith('.gz')){if(!gunzipSync(fs.readFileSync(file)).equals(fs.readFileSync(file.slice(0,-3))))throw Error('Gzip mismatch: '+file);gzipAssets++}}}
check('web/dist');
const report={measuredAt:'2026-10-26',fixture:'Disposable MySQL databases; deterministic textured 2048x1280 images, transparent logo, 12 vendors and 24 products; no business records used',network:{downloadMbps:1.6,latencyMs:150,runsPerPage:5},viewports:[390,768,1440],casesPerVersion:40,pages,aggregateImageReductionPercent,maxCLS,gzipAssets,firstColdHomeImageResponses:optimized.rows[0].cold.responses,baseline,optimized};
fs.mkdirSync('docs/test-output',{recursive:true});fs.writeFileSync('docs/test-output/image-loading-20261026.json',JSON.stringify(report,null,2)+'\n');
console.log(JSON.stringify({pages,aggregateImageReductionPercent,maxCLS,gzipAssets},null,2));
