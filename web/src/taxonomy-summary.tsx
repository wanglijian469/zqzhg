import {useLanguage} from './main';
import {categoryPath,businessNatures} from './taxonomy-utils';
import type {Item,Taxonomy,IndustrialBelt} from './types';
export function TaxonomySummary({item,categories,belts}:{item:Item;categories:Taxonomy['categories'];belts:IndustrialBelt[]}) {
 const {lang}=useLanguage();const zh=lang==='zh',d=item.data;const rows:[string,string][]=[];
 const path=(id:number)=>categoryPath(categories,id,lang)||'#'+id;
 if(item.kind==='vendor'){
  if(d.primary_category_id)rows.push([zh?'主营分类':'Primary category',path(d.primary_category_id)]);
  if(d.secondary_category_ids?.length)rows.push([zh?'兼营分类':'Secondary categories',d.secondary_category_ids.map(path).join('、')]);
  const nature=businessNatures.find(v=>v[0]===d.business_nature);if(nature)rows.push([zh?'经营性质':'Business nature',nature[zh?1:2]]);
 }else{
  if(d.product_category_id)rows.push([zh?'产品分类':'Product category',path(d.product_category_id)]);
  if(d.brand)rows.push([zh?'产品品牌':'Product brand',d.brand]);if(d.model)rows.push([zh?'产品型号':'Product model',d.model]);
  if(d.applicable_brands?.length)rows.push([zh?'适用品牌':'Compatible brands',d.applicable_brands.join('、')]);
  if(d.applicable_models?.length)rows.push([zh?'适用机型':'Compatible models',d.applicable_models.join('、')]);
 }
 if(d.industrial_belt_ids?.length)rows.push([zh?'产业带':'Industrial belts',d.industrial_belt_ids.map(id=>{const b=belts.find(v=>v.id===id);return b?(zh?b.name:b.name_en||b.name):'#'+id}).join('、')]);
 return rows.length?<dl className="taxonomy-public-attributes">{rows.map(([key,value])=><div key={key} style={{display:'contents'}}><dt>{key}</dt><dd>{value}</dd></div>)}</dl>:null;
}
