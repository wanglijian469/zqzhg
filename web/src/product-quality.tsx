import type {Data} from './types';
import {useLanguage} from './ui';
import './product-quality.css';

// Advisory only: never mutates, normalizes or rejects an existing product.
export function productQuality(data:Data){
 const name=(data.name||'').trim();
 const checks=[
  {key:'name',ok:!!name,zh:'产品名称',en:'Product name'},
  {key:'category',ok:!!(data.product_category_id||data.category_id),zh:'产品分类',en:'Product category'},
  {key:'cover',ok:!!data.media_ids?.length,zh:'产品封面',en:'Cover image'},
  {key:'model',ok:!!data.model?.trim()||/^(?:型号|model)[ \t]*[:：][ \t]*\S[^\r\n]*$/im.test(data.parameters||''),zh:'型号信息',en:'Model information'},
  {key:'description',ok:!!data.description?.trim(),zh:'产品介绍',en:'Description'},
  {key:'parameters',ok:(data.parameters||'').split(/\r?\n/).some(line=>!!line.trim()&&!/^[^:：]+[:：]\s*$/.test(line)),zh:'技术参数',en:'Specifications'},
 ];
 const warnings:{zh:string;en:string}[]=[];
 if(/^(?:产品|配件|机械|农机|农机配件|设备|product|parts)$/i.test(name))warnings.push({zh:'名称较笼统，建议填写具体产品名称，例如“旋耕机齿轮”。',en:'Use a specific product name, such as “Rotary tiller gear”.'});
 if(Array.from(name).length>50)warnings.push({zh:'名称较长，建议将型号和参数分别填写，方便手机浏览和检索。',en:'The name is long. Keep model numbers and specifications in their own fields.'});
 if((data.model||'').length>80||/[、,，;；]/.test(data.model||''))warnings.push({zh:'请核对型号是否与封面对应；多个型号建议逐行写入参数，避免串接。',en:'Check that the model matches the cover. List multiple models separately in specifications.'});
 return {checks,warnings,percent:Math.round(checks.filter(c=>c.ok).length/checks.length*100)};
}

export function ProductQualityPanel({data,review=false}:{data:Data;review?:boolean}){
 const {lang}=useLanguage(),zh=lang==='zh',quality=productQuality(data);
 return <aside className="product-quality" aria-label={zh?'产品资料完整度':'Product information completeness'}>
  <div className="product-quality-heading"><strong>{zh?'资料完整度':'Information completeness'}</strong><span>{quality.percent}%</span></div>
  <progress max={100} value={quality.percent} aria-label={zh?'资料完整度':'Information completeness'}/>
  <ul>{quality.checks.map(c=><li key={c.key} className={c.ok?'complete':''}><span aria-hidden="true">{c.ok?'✓':'○'}</span> {zh?c.zh:c.en}<small>{c.ok?(zh?'已填写':'Provided'):(zh?'待补充':'Missing')}</small></li>)}</ul>
  {quality.warnings.map((w,i)=><p key={i}>{zh?w.zh:w.en}</p>)}
  <p>{review?(zh?'审核请人工核对封面、名称、型号与参数是否对应；完整度仅表示字段填写情况，不代替审核结论。':'Review whether the cover, name, model and specifications correspond. Completeness does not replace approval.'):(zh?'首张图片用作封面，建议使用清晰的产品实物图，避免无关图片和画册翻拍。补充建议不增加必填项。':'The first image is the cover. Use a clear photo of this product rather than unrelated or photographed catalogue pages. Suggestions are optional.')}</p>
 </aside>;
}
