import copy from '../../server/seo-copy.json';
import type {Data,Hall,Lang} from './types';

export const seoCopy=copy;
export const seoText=(lang:Lang)=>copy[lang];
export const limitSEODescription=(value:string)=>Array.from(value.trim()).slice(0,160).join('');
export function seoKeywords(...values:(string|undefined)[]){
 const seen=new Set<string>(),terms:string[]=[];
 for(const value of values)for(const raw of (value||'').split(/[,，、;；|\n\r]/)){
  const term=raw.trim().replace(/\s+/gu,' '),key=term.toLowerCase();
  if(!term||seen.has(key)||Array.from(term).length>80)continue;
  seen.add(key);terms.push(term);if(terms.length===24)return terms.join(',');
 }
 return terms.join(',');
}
const name=(data:Data,lang:Lang)=>lang==='en'&&data.name_en?data.name_en:data.name;
export function publicPageSEO(path:string,brand:string,hall:Hall|undefined,lang:Lang){
 const globalTitles:Record<string,[string,string]>={'/services':['企业入驻与服务说明','Admission and services'],'/services/service-info':['企业入驻与服务说明','Admission and services'],'/services/showroom-agreement':['企业展厅服务协议','Showroom service agreement'],'/services/terms':['用户协议','User agreement'],'/services/privacy':['隐私政策','Privacy policy'],'/service-requests':['投诉与服务申请','Complaints and service requests']};if(globalTitles[path])return {title:globalTitles[path][lang==='zh'?0:1]+' | '+brand,description:lang==='zh'?'基础入驻永久免费，标准企业展厅、专属网址及二维码免费，默认免费存储200 MB。':'Permanently free basic admission, standard showrooms, platform URLs and QR codes. Default free storage: 200 MB.',keywords:brand};
 if(!hall||hall.status!=='open'||!['/','/browse','/browse/products'].includes(path))return undefined;
 const text=seoText(lang),hallName=lang==='en'?hall.name_en:hall.name;
 const prefix=path==='/'?text.homeTitle:path==='/browse'?text.directoryTitle:text.productsTitle;
 const description=path==='/'?text.homeIntro:path==='/browse'?text.directoryDescription:text.productsDescription;
 const terms=path==='/browse/products'?text.productTerms:[...text.directoryTerms,...copy.targetTerms];
 return {title:path==='/'?brand+'-'+hallName:prefix+' | '+brand+'-'+hallName,description:limitSEODescription(hallName+(lang==='zh'?'：':': ')+description),keywords:seoKeywords(brand,hallName,...terms)};
}
export function companySEO(data:Data,lang:Lang,tab:string,label:string,brand:string){
 const text=seoText(lang),company=name(data,lang),profile=data.company_profile;
 const summary=(lang==='en'?(profile?.summary_en||profile?.summary):profile?.summary)||(lang==='en'&&data.description_en?data.description_en:data.description)||data.main_products||'';
 return {
  title:company+' · '+text.showroom+(tab==='home'?'':' · '+label)+' | '+brand,
  description:limitSEODescription(company+(lang==='zh'?'企业展厅。':' company showroom. ')+summary),
  keywords:seoKeywords(company,lang==='en'?(profile?.short_name_en||profile?.short_name):profile?.short_name,data.main_products,data.region_name,...text.directoryTerms,data.keywords)
 };
}
export function productSEO(data:Data,vendor:Data|undefined,lang:Lang,brand:string,hallName:string){
 const product=name(data,lang),model=data.model||data.parameters?.match(/(?:^|\n)\s*(?:型号|model)\s*[:：]\s*([^\n]+)/i)?.[1]?.trim();
 return {
  title:product+' | '+brand+' · '+hallName,
  description:limitSEODescription([product,model&&((lang==='zh'?'型号 ':'Model ')+model),vendor&&name(vendor,lang),hallName,lang==='en'&&data.description_en?data.description_en:data.description].filter(Boolean).join(' · ')),
  keywords:seoKeywords(product,model,vendor&&name(vendor,lang),data.brand,data.region_name,...seoText(lang).productTerms,data.keywords)
 };
}

export function canonicalPublicURL(path:string,search:string,base:string,explicit?:string){
 const url=new URL(explicit||path,base);url.hash='';const source=new URLSearchParams(search);const q=new URLSearchParams();
 const keys=['hall','q','category_id','region_id','region_code','unassigned','vendor_id','sort','page'];
 if(['/','/browse','/browse/products','/regions'].includes(url.pathname)){for(const key of keys){let v=source.get(key)?.trim();if(!v||(key==='hall'&&v==='agriculture')||(key==='page'&&(!/^\d+$/.test(v)||Number(v)<=1)))continue;if(key==='page')v=String(Number(v));q.set(key,v)}}q.sort();url.search=q.toString();return url.href;
}
export function showroomSearchVariant(path:string,search:string){const q=new URLSearchParams(search);return /^\/(?:v|vendors)\//.test(path)&&Number(q.get('page'))>1}
export function filteredSearch(search:string){const q=new URLSearchParams(search);return ['q','category_id','region_id','region_code','unassigned','vendor_id','sort'].some(k=>!!q.get(k)?.trim())}

// Unknown client routes must not overwrite the server 404 with indexable metadata.
export function indexablePublicPath(path:string){return ["/","/browse","/browse/products","/regions","/app","/services","/services/service-info","/services/showroom-agreement","/services/terms","/services/privacy","/service-requests"].includes(path)||/^\/(?:products|vendors)\/[1-9][0-9]*\/?$/.test(path)||/^\/v\/[a-z0-9-]+\/?$/.test(path)}
