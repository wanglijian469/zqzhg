import {useEffect,useState} from 'react';
import {CategoryPicker} from './category-picker-field';
import {VendorProductCategoryPicker} from './vendor-product-categories';
import {useLanguage,useLoad} from './ui';
import {categoryPath,businessNatures} from './taxonomy-utils';
import type {Data,Taxonomy} from './types';
import './taxonomy.css';
type Props={data:Data;onChange:(patch:Partial<Data>)=>void;kind:'vendor'|'product';hallID?:number;disabled?:boolean;role?:'admin'|'merchant';ownerID?:number;vendorID?:number};
function TokenField({label,value,onChange,disabled}:{label:string;value:string[];onChange:(v:string[])=>void;disabled?:boolean}) {
 const [text,setText]=useState(value.join(', '));const {lang}=useLanguage();
 const parse=(s:string)=>[...new Set(s.split(/[,，;；\n]/).map(x=>x.trim()).filter(Boolean))];
 useEffect(()=>{if(JSON.stringify(value)!==JSON.stringify(parse(text)))setText(value.join(', '))},[JSON.stringify(value)]);
 return <label>{label}<textarea disabled={disabled} rows={2} maxLength={6030} value={text} onChange={e=>{setText(e.target.value);onChange(parse(e.target.value))}} placeholder={lang==='zh'?'多项用逗号分隔，最多30项':'Separate up to 30 entries with commas'}/></label>
}

export function TaxonomyFields(props:Props){
 const {lang}=useLanguage();const zh=lang==='zh';const {data,onChange,disabled=false,role='merchant',vendorID}=props;
 if(props.kind==='product')return <div className="taxonomy-fields"><VendorProductCategoryPicker value={data.vendor_product_category_id||0} onChange={vendor_product_category_id=>onChange({vendor_product_category_id})} disabled={disabled} role={role} vendorID={vendorID}/>
   <div className="taxonomy-attributes"><label>{zh?'产品品牌':'Product brand'}<input disabled={disabled} maxLength={200} value={data.brand||''} onChange={e=>onChange({brand:e.target.value})}/></label><label>{zh?'产品型号':'Product model'}<input disabled={disabled} aria-label={zh?'产品型号':'Product model'} maxLength={200} placeholder={zh?'例如 CL-20':'e.g. CL-20'} value={data.model||''} onChange={e=>onChange({model:e.target.value})}/><small>{zh?'填写与封面对应的型号；无固定型号可留空。多个型号逐行填写在技术参数中。':'Use the model shown in the cover photo, or leave blank. List multiple models separately in specifications.'}</small></label>
   <TokenField disabled={disabled} label={zh?'适用品牌':'Compatible brands'} value={data.applicable_brands||[]} onChange={applicable_brands=>onChange({applicable_brands})}/><TokenField disabled={disabled} label={zh?'适用机型':'Compatible models'} value={data.applicable_models||[]} onChange={applicable_models=>onChange({applicable_models})}/></div>
 </div>;
 return <VendorTaxonomyFields {...props}/>;
}
function VendorTaxonomyFields({data,onChange,hallID=1,disabled=false,role='merchant',ownerID}:Props){
 const {lang}=useLanguage();const zh=lang==='zh';
 const tax=useLoad<Taxonomy>('/'+role+'/taxonomy?category_kind=vendor',role);
 const legacy=useLoad<Taxonomy>('/'+role+'/taxonomy',role);
 if(hallID!==1)return <CategoryPicker value={data.category_id} onChange={category_id=>onChange({category_id})} kind="vendor" hallID={hallID} disabled={disabled} role={role} ownerID={ownerID}/>;
 const secondary=data.secondary_category_ids||[];
 const changeSecondary=(index:number,value:number)=>{const next=[...secondary];next[index]=value;onChange({secondary_category_ids:next.filter(Boolean)})};
 const picker=(value:number,label:string,field:string,change:(id:number)=>void,excludeIDs:number[]=[])=> <div data-field={field}><CategoryPicker allowSuggest={field!=='secondary_category_ids'} compact kind="vendor" hallID={hallID} role={role} ownerID={ownerID} disabled={disabled} value={value} label={label} excludeIDs={excludeIDs} onChange={change}/></div>;
 return <div className="taxonomy-fields">
 {!!data.category_id&&!data.primary_category_id&&<p className="notice">{zh?'历史分类：':'Historical category: '}{categoryPath(legacy.data?.categories||[],data.category_id,lang)||'#'+data.category_id}</p>}
 {picker(data.primary_category_id||0,zh?'主营分类':'Primary business category','primary_category_id',primary_category_id=>onChange({primary_category_id}),secondary)}
 <details><summary>{zh?'兼营及经营信息（选填）':'Additional business information (optional)'}</summary>
 {picker(secondary[0]||0,zh?'兼营分类1（选填）':'Secondary category 1 (optional)','secondary_category_ids',v=>changeSecondary(0,v),[data.primary_category_id||0,secondary[1]||0])}
 {picker(secondary[1]||0,zh?'兼营分类2（选填）':'Secondary category 2 (optional)','secondary_category_ids',v=>changeSecondary(1,v),[data.primary_category_id||0,secondary[0]||0])}
 <label>{zh?'经营性质':'Business nature'}<select disabled={disabled} value={data.business_nature||''} onChange={e=>onChange({business_nature:e.target.value})}><option value="">{zh?'选填':'Optional'}</option>{businessNatures.map(([v,cn,en])=><option key={v} value={v}>{zh?cn:en}</option>)}</select></label>
 {!!data.industrial_belt_ids?.length&&<p>{zh?'所属产业带（后台维护）：':'Industrial belts (managed by administrators): '}{data.industrial_belt_ids.map(id=>{const belt=tax.data?.industrial_belts?.find(b=>b.id===id);return belt?(zh?belt.name:belt.name_en||belt.name):'#'+id}).join('、')}</p>}
 </details></div>;
}
