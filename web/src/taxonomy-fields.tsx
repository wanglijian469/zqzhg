import {useEffect,useState} from 'react';
import {CategoryPicker} from './category-picker-field';
import {api,body,APIError} from './api';
import {useLanguage,useLoad,ErrorState} from './main';
import {categoryPath,businessNatures} from './taxonomy-utils';
import type {Data,Taxonomy} from './types';
import './taxonomy.css';

function TokenField({label,value,onChange,disabled}:{label:string;value:string[];onChange:(v:string[])=>void;disabled?:boolean}) {
 const [text,setText]=useState(value.join(', '));const {lang}=useLanguage();
 const parse=(s:string)=>[...new Set(s.split(/[,，;；\n]/).map(x=>x.trim()).filter(Boolean))];
 useEffect(()=>{if(JSON.stringify(value)!==JSON.stringify(parse(text)))setText(value.join(', '))},[JSON.stringify(value)]);
 return <label>{label}<textarea disabled={disabled} rows={2} maxLength={6030} value={text} onChange={e=>{setText(e.target.value);onChange(parse(e.target.value))}} placeholder={lang==='zh'?'多项用逗号分隔，最多30项':'Separate up to 30 entries with commas'}/></label>
}
export function TaxonomyFields({data,onChange,kind,hallID=1,disabled=false,role='merchant',ownerID}:{data:Data;onChange:(patch:Partial<Data>)=>void;kind:'vendor'|'product';hallID?:number;disabled?:boolean;role?:'admin'|'merchant';ownerID?:number}) {
 const {lang}=useLanguage();const zh=lang==='zh';const [recommendations,setRecommendations]=useState<(Taxonomy['categories'][number]&{path:string;path_en:string})[]>([]);const [error,setError]=useState('');const [busy,setBusy]=useState(false);
 const tax=useLoad<Taxonomy>('/'+role+'/taxonomy?category_kind='+kind,role);
 const legacy=useLoad<Taxonomy>('/'+role+'/taxonomy',role);
 const id=kind==='vendor'?data.primary_category_id:data.product_category_id;
 if(hallID!==1)return <CategoryPicker value={data.category_id} onChange={category_id=>onChange({category_id})} kind={kind} hallID={hallID} disabled={disabled} role={role} ownerID={ownerID}/>;
 const secondary=data.secondary_category_ids||[];
 const changeSecondary=(index:number,value:number)=>{const next=[...secondary];next[index]=value;onChange({secondary_category_ids:next.filter(Boolean)})};
 const picker=(value:number,label:string,field:string,change:(id:number)=>void,excludeIDs:number[]=[])=> <div data-field={field}><CategoryPicker allowSuggest={field!=='secondary_category_ids'} compact kind={kind} hallID={hallID} role={role} ownerID={ownerID} disabled={disabled} value={value} label={label} excludeIDs={excludeIDs} onChange={change}/></div>;
 return <div className="taxonomy-fields">
  {!!data.category_id&&!id&&<p className="notice">{zh?'历史分类：':'Historical category: '}{categoryPath(legacy.data?.categories||[],data.category_id,lang)||'#'+data.category_id} · {zh?'原资料已保留，可重新选择分类。':'Original data retained. You may select a new category.'}</p>}
  {kind==='vendor'?<>
   {picker(data.primary_category_id||0,zh?'主营分类':'Primary business category','primary_category_id',primary_category_id=>onChange({primary_category_id}),secondary)}
   {picker(secondary[0]||0,zh?'兼营分类1（选填）':'Secondary category 1 (optional)','secondary_category_ids',v=>changeSecondary(0,v),[data.primary_category_id||0,secondary[1]||0])}
   {picker(secondary[1]||0,zh?'兼营分类2（选填）':'Secondary category 2 (optional)','secondary_category_ids',v=>changeSecondary(1,v),[data.primary_category_id||0,secondary[0]||0])}
   <label>{zh?'经营性质':'Business nature'}<select disabled={disabled} value={data.business_nature||''} onChange={e=>onChange({business_nature:e.target.value})}><option value="">{zh?'选填':'Optional'}</option>{businessNatures.map(([v,cn,en])=><option key={v} value={v}>{zh?cn:en}</option>)}</select></label>
   {!!data.industrial_belt_ids?.length&&<p>{zh?'所属产业带（后台维护）：':'Industrial belts (managed by administrators): '}{data.industrial_belt_ids.map(id=>{const belt=tax.data?.industrial_belts?.find(b=>b.id===id);return belt?(zh?belt.name:belt.name_en||belt.name):'#'+id}).join('、')}</p>}
  </>:<>
   {picker(data.product_category_id||0,zh?'产品分类':'Product category','product_category_id',product_category_id=>onChange({product_category_id}))}
   <button type="button" className="button secondary" disabled={disabled||busy||!data.name.trim()} onClick={async()=>{setBusy(true);setError('');try{const r=await api<{items:typeof recommendations}>('/public/category-recommendations',{method:'POST',body:body({hall_id:hallID,name:data.name,keywords:data.keywords})});setRecommendations(r.items);if(!r.items.length)setError('no_recommendation')}catch(e){setError((e as APIError).code)}finally{setBusy(false)}}}>{zh?'推荐分类':'Recommend categories'}</button>
   {!!recommendations.length&&<div className="taxonomy-recommendations"><small>{zh?'点击应用推荐，不会自动更改已选分类。':'Select a recommendation to apply it.'}</small>{recommendations.map(c=><button key={c.id} type="button" disabled={disabled} onClick={()=>onChange({product_category_id:c.id})}>{zh?c.path:c.path_en||c.path}</button>)}</div>}
   {error==='no_recommendation'?<p role="status">{zh?'暂无匹配推荐，请搜索选择分类。':'No matching recommendation. Search for a category.'}</p>:error&&<ErrorState code={error}/>}
   <div className="taxonomy-attributes"><label>{zh?'产品品牌':'Product brand'}<input disabled={disabled} maxLength={200} value={data.brand||''} onChange={e=>onChange({brand:e.target.value})}/></label><label>{zh?'产品型号':'Product model'}<input disabled={disabled} maxLength={200} value={data.model||''} onChange={e=>onChange({model:e.target.value})}/></label>
   <TokenField disabled={disabled} label={zh?'适用品牌':'Compatible brands'} value={data.applicable_brands||[]} onChange={applicable_brands=>onChange({applicable_brands})}/><TokenField disabled={disabled} label={zh?'适用机型':'Compatible models'} value={data.applicable_models||[]} onChange={applicable_models=>onChange({applicable_models})}/></div>
  </>}
 </div>
}
