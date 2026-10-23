export function canonicalPublicURL(path:string,search:string,base:string,explicit?:string){
 const url=new URL(explicit||path,base);url.hash='';const source=new URLSearchParams(search);const q=new URLSearchParams();
 const keys=['hall','q','category_id','region_id','region_code','unassigned','vendor_id','sort','page'];
 if(['/','/browse','/browse/products','/regions'].includes(url.pathname)){for(const key of keys){let v=source.get(key)?.trim();if(!v||(key==='hall'&&v==='agriculture')||(key==='page'&&(!/^\d+$/.test(v)||Number(v)<=1)))continue;if(key==='page')v=String(Number(v));q.set(key,v)}}q.sort();url.search=q.toString();return url.href;
}
export function showroomSearchVariant(path:string,search:string){const q=new URLSearchParams(search);return /^\/(?:v|vendors)\//.test(path)&&Number(q.get('page'))>1}
export function filteredSearch(search:string){const q=new URLSearchParams(search);return ['q','category_id','region_id','vendor_id','sort'].some(k=>!!q.get(k)?.trim())}
