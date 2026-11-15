import type {ReactNode} from 'react';
import {useLanguage} from './ui';
import {categoryPath,businessNatures} from './taxonomy-utils';
import type {Item,Taxonomy,IndustrialBelt} from './types';
import './taxonomy-summary.css';

export function TaxonomySummary({item,categories,belts,variant='compact'}:{item:Item;categories:Taxonomy['categories'];belts:IndustrialBelt[];variant?:'compact'|'panel'}) {
 const {lang}=useLanguage();const zh=lang==='zh',d=item.data,panel=variant==='panel';
 const rows:{label:string;value:ReactNode}[]=[];
 const path=(id:number)=>categoryPath(categories,id,lang)||'#'+id;
 const categoryList=(ids:number[])=>panel?<div className="taxonomy-category-list">{ids.map(id=>{
  const category=categories.find(c=>c.id===id);
  const name=category?(lang==='en'&&category.name_en?category.name_en:category.name):'#'+id;
  const full=path(id),parent=full.endsWith(' / '+name)?full.slice(0,-name.length-3):'';
  return <div className="taxonomy-category-item" key={id}><span className="taxonomy-category-name">{name}</span>{parent&&<span className="taxonomy-category-parent">{parent}</span>}</div>;
 })}</div>:ids.map(path).join('、');
 const add=(label:string,value:ReactNode)=>rows.push({label,value});
 if(item.kind==='vendor'){
  if(d.primary_category_id)add(zh?'主营分类':'Primary category',categoryList([d.primary_category_id]));
  if(d.secondary_category_ids?.length)add(zh?'兼营分类':'Secondary categories',categoryList(d.secondary_category_ids));
  const nature=businessNatures.find(v=>v[0]===d.business_nature);
  if(nature)add(zh?'经营性质':'Business nature',panel?<span className="taxonomy-business-nature">{nature[zh?1:2]}</span>:nature[zh?1:2]);
 }else{
  const own=item.vendor_product_category;
  if(own){add(zh?'产品分类':'Product category',lang==='en'&&own.name_en?own.name_en:own.name);if(own.industry_name)add(zh?'所属行业大类':'Industry category',lang==='en'&&own.industry_name_en?own.industry_name_en:own.industry_name)}else if(d.product_category_id)add(zh?'历史产品分类':'Historical product category',path(d.product_category_id));
  if(d.brand)add(zh?'产品品牌':'Product brand',d.brand);if(d.model)add(zh?'产品型号':'Product model',d.model);
  if(d.applicable_brands?.length)add(zh?'适用品牌':'Compatible brands',d.applicable_brands.join('、'));
  if(d.applicable_models?.length)add(zh?'适用机型':'Compatible models',d.applicable_models.join('、'));
 }
 if(d.industrial_belt_ids?.length)add(zh?'产业带':'Industrial belts',d.industrial_belt_ids.map(id=>{const b=belts.find(v=>v.id===id);return b?(zh?b.name:b.name_en||b.name):'#'+id}).join('、'));
 if(!rows.length)return null;
 const summary=<dl className="taxonomy-public-attributes">{rows.map(({label,value})=><div className="taxonomy-attribute-row" key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl>;
 return panel?<section className="portal-section company-business-info"><div className="portal-section-heading"><h2>{zh?'经营信息':'Business information'}</h2></div>{summary}</section>:summary;
}
