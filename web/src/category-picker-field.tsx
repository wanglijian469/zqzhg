import {useEffect,useRef,useState} from 'react';
import {createPortal} from 'react-dom';
import {Plus} from 'lucide-react';
import {api,APIError,body,token} from './api';
import {useHall,hallHref} from './halls';
import {issueText} from './form-guide';
import {ErrorState,Modal,useLanguage} from './main';
import {categoryPath} from './taxonomy-utils';
import type {Taxonomy} from './types';

type Category=Taxonomy['categories'][number];
type Props={allowParent?:boolean;allowSuggest?:boolean;label?:string;excludeIDs?:number[];compact?:boolean;hallID?:number;value:number;kind:'vendor'|'product';disabled?:boolean;onChange:(id:number)=>void;role?:'admin'|'merchant';ownerID?:number;onLogin?:()=>Promise<boolean>};
const official=(c:Category)=>c.enabled&&(!c.suggestion_status||c.suggestion_status==='approved');

export function CategoryPicker({allowParent=false,allowSuggest=true,label:customLabel,excludeIDs=[],compact=false,hallID,value,kind,disabled=false,onChange,role='merchant',ownerID,onLogin}:Props){
 const {t,lang}=useLanguage();const {hall,halls}=useHall();const selectedHall=hallID||hall?.id||1;const hallSlug=halls.find(h=>h.id===selectedHall)?.slug||'agriculture';const scoped=(path:string)=>hallHref(path,hallSlug);const zh=lang==='zh';const signed=!!token(role);const scope=role+':'+token(role)+':'+selectedHall+':'+kind;
 const [loaded,setLoaded]=useState<{scope:string;data:Taxonomy}>();const [version,setVersion]=useState(0);
 const [search,setSearch]=useState('');const [searchOpen,setSearchOpen]=useState(false);
 const [loadError,setLoadError]=useState('');const [entryError,setEntryError]=useState('');
 const [recent,setRecent]=useState<{scope:string;items:Category[]}>({scope,items:[]});
 const [open,setOpen]=useState(false);const [starting,setStarting]=useState(false);const [busy,setBusy]=useState(false);
 const [name,setName]=useState('');const [en,setEn]=useState('');const [parent,setParent]=useState(0);
 const [parents,setParents]=useState<Category[]>([]);const [error,setError]=useState('');const submitting=useRef(false);const opening=useRef(false);
 useEffect(()=>{let active=true;setLoadError('');api<Taxonomy>(scoped((signed?'/'+role+'/taxonomy':'/public/taxonomy')+'?category_kind='+kind),{},signed?role:undefined).then(data=>{if(active)setLoaded({scope,data})}).catch(e=>{if(active)setLoadError((e as APIError).code||'internal_error')});return()=>{active=false}},[scope,signed,role,version,kind]);
 const data=loaded?.scope===scope?loaded.data:undefined;
 const aliases=data?.aliases||[];const categories=[...(data?.categories||[]),...(recent.scope===scope?recent.items.filter(c=>!data?.categories.some(v=>v.id===c.id)&&!aliases.some(a=>a.source_id===c.id)):[])];
 const selected=aliases.find(a=>a.source_id===value)?.target_id||value;
 const choices=categories.filter(c=>(!c.hall_id||c.hall_id===selectedHall)&&(role!=='admin'||!c.suggestion_status||c.suggestion_status==='approved'||c.owner_id===ownerID)&&(!c.category_kind||c.category_kind===kind));
 const label=customLabel||(kind==='vendor'?(zh?'企业经营分类':'Business category'):(zh?'产品分类':'Product category'));
 const addLabel=kind==='vendor'?(zh?'新增经营分类':'Add business category'):(zh?'新增产品分类':'Add product category');
 const start=async()=>{
  if(disabled||opening.current)return;opening.current=true;setStarting(true);setEntryError('');
  try{
   if(!token('merchant')&&(!onLogin||!await onLogin()))return;
   const currentScope=role+':'+token(role)+':'+selectedHall+':'+kind;const fresh=await api<Taxonomy>(scoped('/merchant/taxonomy?category_kind='+kind),{},'merchant');setLoaded({scope:currentScope,data:fresh});
   const formal=fresh.categories.filter(c=>official(c)&&(!c.hall_id||c.hall_id===selectedHall));const currentID=fresh.aliases?.find(a=>a.source_id===value)?.target_id||value;
   const current=fresh.categories.find(c=>c.id===currentID);
   setParents(formal);setParent(kind==='vendor'?0:current&&official(current)?(formal.some(c=>c.parent_id===current.id)?current.id:current.parent_id):0);setError('');setOpen(true);
  }catch(e){setEntryError((e as APIError).code||'internal_error')}
  finally{opening.current=false;setStarting(false)}
 };
 const save=async()=>{
  if(disabled||submitting.current)return;
  if(!name.trim()||(parent!==0&&!parents.some(c=>c.id===parent))||new TextEncoder().encode(name).length>300||new TextEncoder().encode(en).length>300){setError(zh?'请填写分类名称（最多300字节），并选择顶级分类或正式上级分类。':'Enter a category name (up to 300 bytes) and choose top-level or an official parent.');return}
  submitting.current=true;setBusy(true);setError('');
  try{
   const category=await api<Category>('/merchant/categories',{method:'POST',body:body({name,name_en:en,parent_id:parent,hall_id:selectedHall,category_kind:kind})},'merchant');
   const currentScope=role+':'+token(role)+':'+selectedHall+':'+kind;setRecent(old=>({scope:currentScope,items:[...(old.scope===currentScope?old.items.filter(c=>c.id!==category.id):[]),category]}));
   onChange(category.id);setVersion(v=>v+1);setOpen(false);setName('');setEn('');
  }catch(e){const problem=e as APIError;setError(problem.fields?.length?issueText(problem.fields[0],lang):t(problem.code||'internal_error'))}
  finally{submitting.current=false;setBusy(false)}
 };
 return <div className="category-field">
  <div className="category-field-controls"><label data-field="category_id">{label}<select aria-label={label} disabled={disabled} value={selected||0} onChange={e=>onChange(Number(e.target.value))}><option value={0}>{t('optional')}</option>{choices.filter(c=>!search||categoryPath(choices,c.id,lang).toLowerCase().includes(search.toLowerCase())||c.search_aliases?.toLowerCase().includes(search.toLowerCase())).map(c=><option key={c.id} value={c.id} disabled={c.id!==selected&&(!c.enabled||c.selectable===false||excludeIDs.includes(c.id)||(kind==='product'&&!allowParent&&choices.some(x=>x.parent_id===c.id)))}>{categoryPath(choices,c.id,lang)}{c.suggestion_status==='pending'?(zh?'（待管理员规范）':' (pending standardization)'):''}</option>)}{selected>0&&!choices.some(c=>c.id===selected)&&<option value={selected}>#{selected}</option>}</select></label><button type="button" className="button secondary" disabled={disabled} onClick={()=>{setSearch('');setSearchOpen(true)}}>{zh?'搜索分类':'Search categories'}</button>
   {role==='merchant'&&allowSuggest&&<details className={compact?'product-category-more':'category-add-expanded'} open={compact?undefined:true}><summary hidden={!compact}>{zh?'没有合适分类？':'Missing a category?'}</summary><button type="button" className="button secondary category-add-button" disabled={disabled||starting} onClick={()=>void start()}><Plus size={18}/>{starting?t('loading'):addLabel}</button></details>}
  </div>
  {role==='merchant'&&!compact&&<p className="category-field-help">{zh?'没有对应分类，可自行补充，管理员后续统一规范。':'Missing a category? Add one; administrators standardize it later.'}{!signed&&<span className="category-login-note">{zh?'需登录；登录后返回这里，已填写内容会保留。':'Sign in to continue here. Your entries are retained.'}</span>}</p>}

  {searchOpen&&<Modal close={()=>{setSearchOpen(false);setSearch('')}}><div className="taxonomy-search-panel"><h2>{label}</h2><input autoFocus type="search" aria-label={zh?'搜索分类名称或别名':'Search category names or aliases'} placeholder={zh?'搜索分类名称或别名':'Search category names or aliases'} value={search} onChange={e=>setSearch(e.target.value)}/><div className="taxonomy-search-results">{choices.filter(c=>!search||categoryPath(choices,c.id,lang).toLowerCase().includes(search.toLowerCase())||c.search_aliases?.toLowerCase().includes(search.toLowerCase())).map(c=><button type="button" key={c.id} disabled={!c.enabled||c.selectable===false||excludeIDs.includes(c.id)||(kind==='product'&&!allowParent&&choices.some(x=>x.parent_id===c.id))} onClick={()=>{onChange(c.id);setSearchOpen(false);setSearch('')}}>{categoryPath(choices,c.id,lang)}</button>)}</div></div></Modal>}
  {(loadError||entryError)&&<ErrorState code={entryError||loadError} retry={()=>{setEntryError('');setVersion(v=>v+1)}}/>}
  {open&&createPortal(<Modal close={()=>{if(!submitting.current)setOpen(false)}} closeDisabled={busy}><div className="category-dialog"><h2>{addLabel}</h2><p className="notice">{zh?'新增后先供本厂商使用，管理员后续统一规范。内容审核通过即可公开。':'Available to your company immediately. Administrators standardize it later; content review proceeds normally.'}</p><form onSubmit={e=>{e.preventDefault();e.stopPropagation();void save()}}><fieldset disabled={busy||disabled}><label>{zh?'中文分类名称':'Chinese category name'}<input aria-label={zh?'中文分类名称':'Chinese category name'} value={name} onChange={e=>{setName(e.target.value);setError('')}} placeholder={kind==='vendor'?(zh?'例如：液压元件制造':'Hydraulic components'):(zh?'例如：液压翻转犁':'Hydraulic reversible plough')}/><small className="field-guide">{zh?'必填，最多300个 UTF-8 字节。':'Required, up to 300 UTF-8 bytes.'}</small></label><label>{t('nameEN')}<input aria-label={t('nameEN')} value={en} onChange={e=>{setEn(e.target.value);setError('')}} placeholder={kind==='vendor'?'Hydraulic components':'Hydraulic reversible plough'}/><small className="field-guide">{zh?'选填，最多300个 UTF-8 字节。':'Optional, up to 300 UTF-8 bytes.'}</small></label><label>{t('parent')}<select aria-label={t('parent')} value={parent} onChange={e=>{setParent(Number(e.target.value));setError('')}}><option value={0}>{zh?'顶级（一级）分类':'Top-level (level 1) category'}</option>{parents.filter(c=>kind!=='vendor'&&categoryPath(parents,c.id,'zh').split(' / ').length<3).map(c=><option key={c.id} value={c.id}>{lang==='en'&&c.name_en?c.name_en:c.name}</option>)}</select><small className="field-guide">{zh?'选择“顶级（一级）分类”可新增一级分类；选择已有正式分类则新增其下级分类。':'Choose Top-level to create a level 1 category, or an enabled official category to add a subcategory.'}</small></label></fieldset>{error&&<p className="notice error" role="alert">{error}</p>}<div className="form-actions"><button type="button" className="button secondary" disabled={busy} onClick={()=>setOpen(false)}>{t('cancel')}</button><button type="submit" className="button" disabled={busy||disabled}>{busy?t('loading'):(zh?'保存并选中':'Save and select')}</button></div></form></div></Modal>,document.body)}
 </div>;
}
