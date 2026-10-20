import type {BoundaryGeometry,BoundaryManifest,BoundaryResource} from './industry-map-types';
const memory=new Map<string,BoundaryGeometry>();
export const boundaryCacheName='zqzt.boundaries.v1';
export function approvedResource(value:BoundaryResource|undefined):value is BoundaryResource {
 if(!value||value.approved!==true||value.presentation!=='industry-choropleth'||value.catalog_version!=='2023-06-30')return false;
 if(value.platforms&&(!value.platforms.includes('web')||value.cache_allowed!==true||!['WGS84','GCJ-02','CGCS2000'].includes(value.coordinate_system||'')))return false;
 if(value.code_field&&!['adcode','code'].includes(value.code_field))return false;
 if(value.code_map&&Object.entries(value.code_map).some(([source,target])=>!/^\d{2,6}$/.test(source)||!/^\d{2}(?:\d{2}){0,2}$/.test(target)))return false;
 return /^\/maps\/[a-zA-Z0-9/_-]+\.json$/.test(value.url)&&!value.url.includes('..')&&/^[a-f0-9]{64}$/.test(value.sha256)&&
 ['version','source','license','review_number','review_evidence','publication_scope','verified_by','verified_at'].every(key=>typeof value[key as keyof BoundaryResource]==='string'&&String(value[key as keyof BoundaryResource]).trim().length>0)&&Number.isFinite(Date.parse(value.verified_at));
}
export async function loadBoundary(code:string,signal:AbortSignal):Promise<{resource:BoundaryResource;geometry:BoundaryGeometry}|null>{
 let response:Response;try{response=await fetch('/maps/manifest.json',{signal,cache:'no-cache'})}catch(error){if(signal.aborted)throw error;return null;}
 if(response.status===404)return null;
 if(!response.ok)return null;
 const manifest=await response.json() as BoundaryManifest;
 if(manifest.version!==1||manifest.catalog_version!=='2023-06-30'||!manifest.resources)throw Error('boundary_manifest_invalid');
 const resource=manifest.resources[code||'national'];
 if(!resource)return null;
 if(!approvedResource(resource))throw Error('boundary_not_verified');
 const key=resource.url+'?sha256='+resource.sha256;
 let geometry=memory.get(key);
 if(!geometry){
  let cache:Cache|undefined;
  try{if(globalThis.caches)cache=await caches.open(boundaryCacheName)}catch{/* Browser storage may be disabled. */}
  let saved=await cache?.match(key).catch(()=>undefined);
  const verify=async(res:Response)=>{
   const bytes=await res.arrayBuffer();
   const digest=Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',bytes))).map(n=>n.toString(16).padStart(2,'0')).join('');
   if(digest!==resource.sha256)throw Error('boundary_integrity_failed');
   const data=JSON.parse(new TextDecoder().decode(bytes)) as BoundaryGeometry;
   if(data.type!=='FeatureCollection'||!Array.isArray(data.features)||!data.features.length||data.features.some(f=>f.type!=='Feature'||!f.geometry||!/^\d{2,6}$/.test(String(f.properties?.[resource.code_field||'adcode']))))throw Error('boundary_geometry_invalid');
   return data;
  };
  if(saved){try{geometry=await verify(saved)}catch{await cache?.delete(key).catch(()=>false);saved=undefined}}
  if(!saved){
   const result=await fetch(resource.url,{signal,cache:'no-cache'});
   if(!result.ok)throw Error('boundary_unavailable');
   geometry=await verify(result.clone());
   if(cache){try{await cache.put(key,result);const keys=await cache.keys();for(const old of keys.slice(0,Math.max(0,keys.length-24)))await cache.delete(old)}catch{/* Cache failure does not prevent display. */}}
  }
  if(signal.aborted)throw new DOMException('Aborted','AbortError');
  memory.set(key,geometry!);if(memory.size>12)memory.delete(memory.keys().next().value!);
 }
 const mapped={...geometry!,features:geometry!.features.map(feature=>{const source=String(feature.properties[resource.code_field||'adcode']);return {...feature,properties:{...feature.properties,adcode:resource.code_map?.[source]||source}}})};
 return {resource,geometry:mapped};
}
