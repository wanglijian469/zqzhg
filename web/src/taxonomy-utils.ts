import type {Taxonomy} from './types';
export function categoryPath(categories:Taxonomy['categories'],id:number,lang:string):string {
 const parts:string[]=[],seen=new Set<number>();
 while(id&&!seen.has(id)){seen.add(id);const c=categories.find(v=>v.id===id);if(!c)break;parts.unshift(lang==='en'&&c.name_en?c.name_en:c.name);id=c.parent_id}
 return parts.join(' / ');
}
export const businessNatures=[['manufacturer','生产制造','Manufacturer'],['dealer','经销代理','Dealer / agent'],['manufacturing_trading','工贸一体','Manufacturing and trading'],['processing','加工服务','Processing services'],['other','其他','Other']] as const;
