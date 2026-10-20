import {readFile,stat} from 'node:fs/promises';
import {resolve,dirname,relative,isAbsolute} from 'node:path';
import {fileURLToPath,pathToFileURL} from 'node:url';
import {createHash} from 'node:crypto';

const root=resolve(dirname(fileURLToPath(import.meta.url)),'..');
const required=['version','source','license','review_number','review_evidence','publication_scope','verified_by','verified_at'];
function inside(base,target){const path=relative(base,target);return !path.startsWith('..')&&!isAbsolute(path)}
export async function verifyMapResources(manifestPath=resolve(root,'web/public/maps/manifest.json')){
 const manifest=JSON.parse(await readFile(manifestPath,'utf8'));
 if(manifest.version!==1||manifest.catalog_version!=='2023-06-30'||!manifest.resources||typeof manifest.resources!=='object'||Array.isArray(manifest.resources))throw Error('Invalid map manifest');
 const raw=JSON.parse(await readFile(resolve(root,'server/catalog/administrative-regions-2023.json'),'utf8'));
 const nodes=new Map();const add=(node,parent)=>{nodes.set(node.code,{...node,parent});for(const child of node.children||[])add(child,node.code)};
 for(const province of raw){const children=[];for(const city of province.children||[]){if(city.name.includes('直辖县级行政区划'))children.push(...city.children);else children.push(city)}add({...province,children},'')}
 for(const [code,resource] of Object.entries(manifest.resources)){
  if(code!=='national'&&!nodes.has(code))throw Error('Unknown region: '+code);
  if(resource.approved!==true||resource.presentation!=='industry-choropleth'||resource.catalog_version!==manifest.catalog_version||required.some(key=>typeof resource[key]!=='string'||!resource[key].trim())||!Number.isFinite(Date.parse(resource.verified_at)))throw Error('Missing final-presentation verification: '+code);
  if(resource.platforms&&(!Array.isArray(resource.platforms)||!resource.platforms.includes('web')||resource.platforms.some(platform=>!['web','android'].includes(platform))||resource.cache_allowed!==true||!['WGS84','GCJ-02','CGCS2000'].includes(resource.coordinate_system)))throw Error('Invalid platform, caching or coordinate scope: '+code);
  if(resource.code_field&&!['adcode','code'].includes(resource.code_field))throw Error('Unknown boundary code field');
  if(resource.code_map&&Object.entries(resource.code_map).some(([source,target])=>!/^\d{2,6}$/.test(source)||!/^\d{2}(?:\d{2}){0,2}$/.test(target)))throw Error('Invalid explicit code crosswalk');
  if(!/^\/maps\/[a-zA-Z0-9/_-]+\.json$/.test(resource.url)||resource.url.includes('..')||!/^[a-f0-9]{64}$/.test(resource.sha256))throw Error('Invalid resource path/hash: '+code);
  const publicRoot=resolve(root,'web/public');const asset=resolve(publicRoot,'.'+resource.url);
  if(!inside(publicRoot,asset))throw Error('Resource outside public directory');
  const evidence=resolve(root,resource.review_evidence);
  if(!inside(resolve(root,'docs'),evidence)||!(await stat(evidence)).isFile())throw Error('Review evidence must be a saved document in docs/: '+code);
  const evidenceText=await readFile(evidence,'utf8');
  for(const value of [resource.sha256,resource.review_number,resource.publication_scope,resource.presentation])if(!evidenceText.includes(value))throw Error('Evidence must identify hash, review number, scope and final choropleth presentation: '+code);
  const bytes=await readFile(asset);if(createHash('sha256').update(bytes).digest('hex')!==resource.sha256)throw Error('Boundary hash mismatch: '+code);
  const geometry=JSON.parse(bytes.toString('utf8'));
  if(geometry.type!=='FeatureCollection'||!Array.isArray(geometry.features)||!geometry.features.length)throw Error('Invalid GeoJSON: '+code);
  const allowed=new Set([...nodes.values()].filter(node=>node.parent===(code==='national'?'':code)).map(node=>node.code));
  if(code==='national')for(const context of ['71','81','82'])allowed.add(context);
  if(code!=='national'&&allowed.size===0)allowed.add(code);
  const seen=new Set();
  for(const feature of geometry.features){const source=String(feature.properties?.[resource.code_field||'adcode']);const child=resource.code_map?.[source]||source;if(!allowed.has(child)||seen.has(child)||feature.type!=='Feature'||!['Polygon','MultiPolygon'].includes(feature.geometry?.type)||!Array.isArray(feature.geometry.coordinates))throw Error('Invalid or unmapped boundary feature: '+child);seen.add(child)}
  // A national graphic needs a separately verified complete national presentation;
  // the 31-province statistical catalogue alone must never define its outline.
  if(code==='national'&&!evidenceText.includes('全国完整疆域及附图核验'))throw Error('National presentation requires complete-territory review evidence');
 }
 return {resources:Object.keys(manifest.resources).length,mode:Object.keys(manifest.resources).length?'verified-resources':'map-resources-pending'};
}
if(process.argv[1]&&import.meta.url===pathToFileURL(resolve(process.argv[1])).href){
 try{const result=await verifyMapResources(process.argv[2]?resolve(process.argv[2]):undefined);console.log(JSON.stringify(result));console.log('PASS: resource integrity and evidence presence checked. Legal approval must be verified by the recorded reviewer.')}catch(error){console.error(error.message);process.exitCode=1}
}
