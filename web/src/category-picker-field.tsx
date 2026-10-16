import {useEffect,useRef,useState} from 'react';
import {createPortal} from 'react-dom';
import {Plus} from 'lucide-react';
import {api,APIError,body,token} from './api';
import {issueText} from './form-guide';
import {ErrorState,Modal,useLanguage} from './main';
import type {Taxonomy} from './types';

type Category=Taxonomy['categories'][number];
type Props={value:number;kind:'vendor'|'product';disabled?:boolean;onChange:(id:number)=>void;role?:'admin'|'merchant';ownerID?:number;onLogin?:()=>Promise<boolean>};
const official=(c:Category)=>c.enabled&&(!c.suggestion_status||c.suggestion_status==='approved');

export function CategoryPicker({value,kind,disabled=false,onChange,role='merchant',ownerID,onLogin}:Props){
 const {t,lang}=useLanguage();const zh=lang==='zh';const signed=!!token(role);const scope=role+':'+token(role);
 const [loaded,setLoaded]=useState<{scope:string;data:Taxonomy}>();const [version,setVersion]=useState(0);
 const [loadError,setLoadError]=useState('');const [entryError,setEntryError]=useState('');
 const [recent,setRecent]=useState<{scope:string;items:Category[]}>({scope,items:[]});
 const [open,setOpen]=useState(false);const [starting,setStarting]=useState(false);const [busy,setBusy]=useState(false);
 const [name,setName]=useState('');const [en,setEn]=useState('');const [parent,setParent]=useState(0);
 const [parents,setParents]=useState<Category[]>([]);const [error,setError]=useState('');const submitting=useRef(false);const opening=useRef(false);
 useEffect(()=>{let active=true;setLoadError('');api<Taxonomy>(signed?'/'+role+'/taxonomy':'/public/taxonomy',{},signed?role:undefined).then(data=>{if(active)setLoaded({scope,data})}).catch(e=>{if(active)setLoadError((e as APIError).code||'internal_error')});return()=>{active=false}},[scope,signed,role,version]);
 const data=loaded?.scope===scope?loaded.data:undefined;
 const aliases=data?.aliases||[];const categories=[...(data?.categories||[]),...(recent.scope===scope?recent.items.filter(c=>!data?.categories.some(v=>v.id===c.id)&&!aliases.some(a=>a.source_id===c.id)):[])];
 const selected=aliases.find(a=>a.source_id===value)?.target_id||value;
 const choices=categories.filter(c=>role!=='admin'||!c.suggestion_status||c.suggestion_status==='approved'||c.owner_id===ownerID);
 const label=kind==='vendor'?(zh?'企业经营分类':'Business category'):(zh?'产品分类':'Product category');
 const addLabel=kind==='vendor'?(zh?'新增经营分类':'Add business category'):(zh?'新增产品分类':'Add product category');
 const start=async()=>{
  if(disabled||opening.current)return;opening.current=true;setStarting(true);setEntryError('');
  try{
   if(!token('merchant')&&(!onLogin||!await onLogin()))return;
   const currentScope=role+':'+token(role);const fresh=await api<Taxonomy>('/merchant/taxonomy',{},'merchant');setLoaded({scope:currentScope,data:fresh});
   const formal=fresh.categories.filter(official);const currentID=fresh.aliases?.find(a=>a.source_id===value)?.target_id||value;
   const current=fresh.categories.find(c=>c.id===currentID);
   setParents(formal);setParent(current&&official(current)?current.id:formal.some(c=>c.id===current?.parent_id)?current!.parent_id:0);setError('');setOpen(true);
  }catch(e){setEntryError((e as APIError).code||'internal_error')}
  finally{opening.current=false;setStarting(false)}
 };
 const save=async()=>{
  if(disabled||submitting.current)return;
  if(!name.trim()||(parent!==0&&!parents.some(c=>c.id===parent))||new TextEncoder().encode(name).length>300||new TextEncoder().encode(en).length>300){setError(zh?'请填写分类名称（最多300字节），并选择顶级分类或正式上级分类。':'Enter a category name (up to 300 bytes) and choose top-level or an official parent.');return}
  submitting.current=true;setBusy(true);setError('');
  try{
   const category=await api<Category>('/merchant/categories',{method:'POST',body:body({name,name_en:en,parent_id:parent})},'merchant');
   const currentScope=role+':'+token(role);setRecent(old=>({scope:currentScope,items:[...(old.scope===currentScope?old.items.filter(c=>c.id!==category.id):[]),category]}));
   onChange(category.id);setVersion(v=>v+1);setOpen(false);setName('');setEn('');
  }catch(e){const problem=e as APIError;setError(problem.fields?.length?issueText(problem.fields[0],lang):t(problem.code||'internal_error'))}
  finally{submitting.current=false;setBusy(false)}
 };
 return <div className="category-field">
  <div className="category-field-controls"><label data-field="category_id">{label}<select aria-label={label} disabled={disabled} value={selected||0} onChange={e=>onChange(Number(e.target.value))}><option value={0}>{t('optional')}</option>{choices.map(c=><option key={c.id} value={c.id} disabled={!c.enabled&&c.id!==selected}>{c.parent_id?'↳ ':''}{lang==='en'&&c.name_en?c.name_en:c.name}{c.suggestion_status==='pending'?(zh?'（待管理员规范）':' (pending standardization)'):''}</option>)}{selected>0&&!choices.some(c=>c.id===selected)&&<option value={selected}>#{selected}</option>}</select></label>
   {role==='merchant'&&<button type="button" className="button secondary category-add-button" disabled={disabled||starting} onClick={()=>void start()}><Plus size={18}/>{starting?t('loading'):addLabel}</button>}
  </div>
  {role==='merchant'&&<p className="category-field-help">{zh?'没有对应分类，可自行补充，管理员后续统一规范。':'Missing a category? Add one; administrators standardize it later.'}{!signed&&<span className="category-login-note">{zh?'需登录；登录后返回这里，已填写内容会保留。':'Sign in to continue here. Your entries are retained.'}</span>}</p>}
  {kind==='product'&&<small className="muted">{zh?'选择分类或填写产品关键词，沿用现有填写规则。':'Choose a category or enter product keywords.'}</small>}
  {(loadError||entryError)&&<ErrorState code={entryError||loadError} retry={()=>{setEntryError('');setVersion(v=>v+1)}}/>}
  {open&&createPortal(<Modal close={()=>{if(!submitting.current)setOpen(false)}} closeDisabled={busy}><div className="category-dialog"><h2>{addLabel}</h2><p className="notice">{zh?'新增后先供本厂商使用，管理员后续统一规范。内容审核通过即可公开。':'Available to your company immediately. Administrators standardize it later; content review proceeds normally.'}</p><form onSubmit={e=>{e.preventDefault();e.stopPropagation();void save()}}><fieldset disabled={busy||disabled}><label>{zh?'中文分类名称':'Chinese category name'}<input aria-label={zh?'中文分类名称':'Chinese category name'} value={name} onChange={e=>{setName(e.target.value);setError('')}} placeholder={kind==='vendor'?(zh?'例如：液压元件制造':'Hydraulic components'):(zh?'例如：液压翻转犁':'Hydraulic reversible plough')}/><small className="field-guide">{zh?'必填，最多300个 UTF-8 字节。':'Required, up to 300 UTF-8 bytes.'}</small></label><label>{t('nameEN')}<input aria-label={t('nameEN')} value={en} onChange={e=>{setEn(e.target.value);setError('')}} placeholder={kind==='vendor'?'Hydraulic components':'Hydraulic reversible plough'}/><small className="field-guide">{zh?'选填，最多300个 UTF-8 字节。':'Optional, up to 300 UTF-8 bytes.'}</small></label><label>{t('parent')}<select aria-label={t('parent')} value={parent} onChange={e=>{setParent(Number(e.target.value));setError('')}}><option value={0}>{zh?'顶级（一级）分类':'Top-level (level 1) category'}</option>{parents.map(c=><option key={c.id} value={c.id}>{lang==='en'&&c.name_en?c.name_en:c.name}</option>)}</select><small className="field-guide">{zh?'选择“顶级（一级）分类”可新增一级分类；选择已有正式分类则新增其下级分类。':'Choose Top-level to create a level 1 category, or an enabled official category to add a subcategory.'}</small></label></fieldset>{error&&<p className="notice error" role="alert">{error}</p>}<div className="form-actions"><button type="button" className="button secondary" disabled={busy} onClick={()=>setOpen(false)}>{t('cancel')}</button><button type="submit" className="button" disabled={busy||disabled}>{busy?t('loading'):(zh?'保存并选中':'Save and select')}</button></div></form></div></Modal>,document.body)}
 </div>;
}
