import {useEffect,useRef,useState} from 'react';
import {Link,useLocation,useNavigate,useParams,useSearchParams} from 'react-router-dom';
import {Bell,ChevronDown,ClipboardList,Search,SquarePen,X} from 'lucide-react';
import {useLanguage,useLoad,Loading,ErrorState,Pager} from './ui';
import {useHall} from './halls';
import {useMobileWorkspace,useWorkspaceBack} from './mobile-workspace';
import {runtimeConfig} from './runtime-config';
import type {Taxonomy} from './types';
import type {ProcPage} from './procurement-api';

export interface PublicProcurement {
 id:number;hall_id:number;title:string;delivery_region:string;deadline:string;published_at?:string|null;state:string;
 items:{name:string;category_id:number;category_name?:string;category_name_en?:string;quantity?:string;unit?:string}[];
}
type Filters={hall_id:string;category_id:string;region:string;state:string};
const fromParams=(p:URLSearchParams):Filters=>({hall_id:p.get('hall_id')||'',category_id:p.get('category_id')||'',region:p.get('region')||'',state:p.get('state')||'open'});
function useWords(){const {lang}=useLanguage();return {en:lang==='en',L:(zh:string,en:string)=>lang==='en'?en:zh}}
export function DeliveryRegion({code}:{code:string}){const r=useLoad<{selected?:{full_name:string}}>('/public/administrative-regions?code='+encodeURIComponent(code),undefined,!!code);return <>{r.data?.selected?.full_name||code||'—'}</>}
type RegionNode={code:string;name:string;level?:number;has_children?:boolean};
export function RegionPicker({value,onChange}:{value:string;onChange:(s:string)=>void}){
 const {L}=useWords();
 const selection=useLoad<{selected?:RegionNode;trail:RegionNode[]}>('/public/administrative-regions?code='+encodeURIComponent(value),undefined,value.length>2);
 const trail=selection.data?.trail||[];
 const province=trail.find(n=>n.level===1)?.code||value.slice(0,2);
 const city=trail.find(n=>n.level===2)?.code||(value.length>=4?value.slice(0,4):'');
 const district=trail.length?(trail.some(n=>n.level===3)?value:''):(value.length===6?value:'');
 const roots=useLoad<{items:{code:string;name:string}[]}>('/public/administrative-regions');
 const cities=useLoad<{items:{code:string;name:string}[]}>('/public/administrative-regions?parent_code='+province,undefined,!!province);
 const districts=useLoad<{items:{code:string;name:string}[]}>('/public/administrative-regions?parent_code='+city,undefined,!!city);
 return <div className="proc-region"><div className="proc-grid three">
  <label>{L('省份','Province')}<select value={province} onChange={e=>onChange(e.target.value)}><option value="">{L('全部地区','All regions')}</option>{roots.data?.items.map(r=><option key={r.code} value={r.code}>{r.name}</option>)}</select></label>
  <label>{L('城市','City')}<select disabled={!province} value={city} onChange={e=>onChange(e.target.value||province)}><option value="">{L('全省','All cities')}</option>{cities.data?.items.map(r=><option key={r.code} value={r.code}>{r.name}</option>)}</select></label>
  <label>{L('区县','District')}<select disabled={!city||selection.data?.selected?.level===2&&selection.data.selected.has_children===false} value={district} onChange={e=>onChange(e.target.value||city)}><option value="">{L('全市','All districts')}</option>{districts.data?.items.map(r=><option key={r.code} value={r.code}>{r.name}</option>)}</select></label>
 </div>{(roots.error||cities.error||districts.error||selection.error)&&<ErrorState code={roots.error||cities.error||districts.error||selection.error} retry={()=>{roots.reload();cities.reload();districts.reload();selection.reload()}}/>}</div>;
}
function PublicSummary({p}:{p:PublicProcurement}){
 const {L,en}=useWords();const date=(s:string)=>new Date(s).toLocaleString(en?'en':'zh-CN');
 return <><h2><span>{p.title}</span><span className="badge proc-status open">{L('进行中','Open')}</span></h2>
  {p.items.map((item,i)=><p key={i}>{item.name}{item.quantity?` · ${item.quantity} ${item.unit||''}`:''}{(en?item.category_name_en||item.category_name:item.category_name)&&<small> · {en?item.category_name_en||item.category_name:item.category_name}</small>}</p>)}
  <div className="proc-meta"><span>{L('交货地区：','Delivery: ')}<DeliveryRegion code={p.delivery_region}/></span>
   {p.published_at&&<span>{L('发布：','Published: ')}{date(p.published_at)}</span>}
   <span>{L('截止：','Deadline: ')}{date(p.deadline)}</span>
  </div></>;
}
export function ProcurementHall(){
 const {L,en}=useWords(),{halls}=useHall(),mobile=useMobileWorkspace();const [params,setParams]=useSearchParams();const location=useLocation();
 const [q,setQ]=useState(params.get('q')||''),[sheet,setSheet]=useState(false),[draft,setDraft]=useState(()=>fromParams(params));
 const ref=useRef<HTMLDivElement>(null),trigger=useRef<HTMLButtonElement|null>(null);
 const fallbackHall=String(halls.find(h=>h.status==='open')?.id||1);
 const taxonomy=useLoad<Taxonomy>('/public/taxonomy?category_kind=product&hall_id='+(draft.hall_id||fallbackHall));
 const r=useLoad<ProcPage<PublicProcurement>>('/public/procurements?'+params);
 useEffect(()=>setQ(params.get('q')||''),[params.get('q')]);
 useEffect(()=>{if(!sheet)setDraft(fromParams(params))},[params.toString(),sheet]);
 const apply=(f:Filters,term=q)=>{const n=new URLSearchParams();if(term.trim())n.set('q',term.trim());Object.entries(f).forEach(([k,v])=>{if(v&&!(k==='state'&&v==='open'))n.set(k,v)});setParams(n);setSheet(false)};
 const clear=()=>{setQ('');setDraft({hall_id:'',category_id:'',region:'',state:'open'});setParams({});setSheet(false)};
 useWorkspaceBack(()=>{if(!sheet)return false;setSheet(false);return true},sheet);
 useEffect(()=>{
  if(!sheet)return;const overflow=document.body.style.overflow;document.body.style.overflow='hidden';ref.current?.querySelector<HTMLElement>('button,select')?.focus();
  const key=(e:KeyboardEvent)=>{if(e.key==='Escape'){e.preventDefault();setSheet(false)}if(e.key==='Tab'){const nodes=Array.from(ref.current?.querySelectorAll<HTMLElement>('button:not(:disabled),select:not(:disabled)')||[]);const first=nodes[0],last=nodes.at(-1);if(e.shiftKey&&document.activeElement===first){e.preventDefault();last?.focus()}else if(!e.shiftKey&&document.activeElement===last){e.preventDefault();first?.focus()}}};
  document.addEventListener('keydown',key);return()=>{document.body.style.overflow=overflow;document.removeEventListener('keydown',key);trigger.current?.focus({preventScroll:true})};
 },[sheet]);
 // Restore after the list has rendered, including when a summary is visited and Back is used.
 const memoryKey='dlnj.proc.scroll:'+runtimeConfig().site.api_url+':'+(sessionStorage.getItem('zqzt.merchant.id')||'guest')+':'+location.pathname+'?'+params;
 useEffect(()=>{if(!r.data)return;const frame=requestAnimationFrame(()=>{const y=Number(sessionStorage.getItem(memoryKey)||0);if(y)window.scrollTo(0,y)});return()=>cancelAnimationFrame(frame)},[memoryKey,!!r.data]);
 useEffect(()=>{const save=()=>sessionStorage.setItem(memoryKey,String(window.scrollY));window.addEventListener('scroll',save,{passive:true});return()=>window.removeEventListener('scroll',save)},[memoryKey]);
 const controls=<><label>{L('行业展馆','Hall')}<select value={draft.hall_id} onChange={e=>setDraft({...draft,hall_id:e.target.value,category_id:''})}><option value="">{L('全部','All')}</option>{halls.filter(h=>h.status==='open').map(h=><option key={h.id} value={h.id}>{en?h.name_en||h.name:h.name}</option>)}</select></label>
  <label>{L('产品分类','Product category')}<select value={draft.category_id} disabled={!taxonomy.data} onChange={e=>setDraft({...draft,hall_id:draft.hall_id||fallbackHall,category_id:e.target.value})}><option value="">{L('全部','All')}</option>{taxonomy.data?.categories.filter(c=>c.enabled&&c.hall_id===Number(draft.hall_id||fallbackHall)).map(c=><option key={c.id} value={c.id}>{c.parent_id?'↳ ':''}{en?c.name_en||c.name:c.name}</option>)}</select></label>
  {taxonomy.error&&<ErrorState code={taxonomy.error} retry={taxonomy.reload}/>}
  <RegionPicker value={draft.region} onChange={region=>setDraft({...draft,region})}/>
  <label>{L('状态','Status')}<select value={draft.state} onChange={e=>setDraft({...draft,state:e.target.value})}><option value="open">{L('进行中','Open')}</option><option value="closing_soon">{L('三天内截止','Closes within 3 days')}</option></select></label></>;
 return <section className="proc-hall"><h1>{L('采购需求大厅','Procurement request hall')}</h1>
  <form className={mobile?'proc-search':'proc-card proc-hall-filters'} onSubmit={e=>{e.preventDefault();(document.activeElement as HTMLElement)?.blur();apply(mobile?fromParams(params):draft)}}>
   {mobile?<><input aria-label={L('项目关键词','Search')} placeholder={L('搜索采购产品或关键词','Product name or keyword')} maxLength={200} value={q} onChange={e=>setQ(e.target.value)}/><button className="button" aria-label={L('查询','Search requests')}><Search size={20}/></button></>:<><label>{L('项目关键词','Search')}<input maxLength={200} value={q} onChange={e=>setQ(e.target.value)}/></label>{controls}<div className="proc-filter-actions"><button type="button" className="button" onClick={()=>apply(draft)}>{L('查询','Search')}</button><button type="button" className="button secondary" onClick={clear}>{L('重置','Reset')}</button></div></>}
  </form>
  {mobile&&<><nav className="proc-shortcuts" aria-label={L('采购快捷入口','Procurement shortcuts')}><Link to="/buyer/new"><SquarePen/><span>{L('发布需求','Create request')}</span></Link><Link to="/buyer"><ClipboardList/><span>{L('我的询价','My requests')}</span></Link><Link to="/buyer/notifications"><Bell/><span>{L('消息通知','Notifications')}</span></Link></nav>
   <div className="proc-filter-row">{[['hall_id',L('行业展馆','Hall')],['category_id',L('产品分类','Category')],['region',L('地区','Region')]].map(([key,label])=><button key={key} className={params.get(key)?'active':''} aria-haspopup="dialog" onClick={e=>{trigger.current=e.currentTarget;setDraft(fromParams(params));setSheet(true)}}><span>{label}{params.get(key)?' · 1':''}</span><ChevronDown size={16}/></button>)}</div></>}
  {r.error?<ErrorState code={r.error} retry={r.reload}/>:!r.data?<div className="proc-loading" role="status"><Loading/></div>:<><div className="proc-request-list">{r.data.items.map(p=><article className="proc-card" key={p.id}><Link className="proc-summary-link" to={'/procurement/'+p.id} state={{summary:p,returnTo:location.pathname+location.search}}><PublicSummary p={p}/><span className="proc-summary-view">{L('查看详情','View details')} →</span></Link></article>)}</div>{!r.data.items.length&&<div className="proc-card proc-empty"><h2>{L('暂无符合条件的采购需求','No matching requests')}</h2><p>{L('试试更换关键词，或减少分类和地区限制。','Try another keyword or fewer category and region filters.')}</p><button className="button secondary" onClick={clear}>{L('重置筛选','Reset filters')}</button></div>}<Pager page={r.data.page} size={r.data.page_size||20} total={r.data.total} onPage={page=>{const n=new URLSearchParams(params);n.set('page',String(page));setParams(n);window.scrollTo(0,0)}}/></>}
  {sheet&&<div className="proc-sheet-backdrop" onClick={()=>setSheet(false)}><div ref={ref} className="proc-sheet" role="dialog" aria-modal="true" aria-labelledby="proc-filter-title" onClick={e=>e.stopPropagation()}><div className="proc-sheet-header"><h2 id="proc-filter-title">{L('筛选采购需求','Filter requests')}</h2><button aria-label={L('关闭筛选','Close filters')} onClick={()=>setSheet(false)}><X/></button></div><div className="proc-sheet-body">{controls}</div><div className="proc-sheet-actions"><button className="button secondary" onClick={clear}>{L('重置','Reset')}</button><button className="button" onClick={()=>apply(draft)}>{L('完成','Apply')}</button></div></div></div>}
 </section>;
}
export function PublicProcurementDetail(){
 const {id}=useParams(),{L}=useWords();const location=useLocation(),navigate=useNavigate();
 const r=useLoad<PublicProcurement>('/public/procurements/'+id);
 // Older servers can still show the exact public summary selected in this session.
 const cached=location.state?.summary as PublicProcurement|undefined;
 const legacy=runtimeConfig().capabilities?.procurement_public_summary!==true;
 const p=r.data||(legacy&&r.error==='not_found'&&String(cached?.id)===id?cached:undefined);
 return <section className="proc-card"><h1>{L('采购需求详情','Request details')}</h1>{!p?(r.error?<ErrorState code={r.error} retry={r.reload}/>:<Loading/>):<><PublicSummary p={p}/>{legacy&&r.error&&<p className="notice" role="status">{L('当前服务器尚不支持公开详情，以上为刚才列表的摘要；需求状态以参与时校验为准。','This server needs an update for public details. Showing the selected list summary; eligibility and status are checked when participating.')}</p>}<p className="proc-required-hint">{L('此处为公开摘要。完整要求、附件和联系方式按原有参与资格开放。','This is a public summary. Full requirements, files and contact details retain their existing access rules.')}</p><button className="button" onClick={()=>navigate('/merchant/opportunities/join/'+p.id)}>{L('参与并查看完整需求','Participate and view details')}</button></>}<div className="proc-actions"><Link to={typeof location.state?.returnTo==='string'&&location.state.returnTo.startsWith('/procurement?')?location.state.returnTo:'/procurement'} onClick={e=>{if((window.history.state?.idx||0)>0){e.preventDefault();navigate(-1)}}}>{L('返回采购大厅','Back to request hall')}</Link></div></section>;
}
