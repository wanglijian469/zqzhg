import {useEffect,useId,useRef,useState,type ReactNode} from 'react';
import {ChevronDown,Phone,X} from 'lucide-react';
import {useLanguage,Modal} from './ui';
import type {Lang,Taxonomy} from './types';

export function usePhoneViewport(){
 const [small,setSmall]=useState(()=>window.matchMedia?.('(max-width: 600px)').matches??false);
 useEffect(()=>{const media=window.matchMedia?.('(max-width: 600px)');if(!media)return;const update=()=>setSmall(media.matches);update();media.addEventListener('change',update);return()=>media.removeEventListener('change',update)},[]);
 return small;
}
export function ResponsiveFilters({children,count=0}:{children:ReactNode;count?:number}){
 const {t}=useLanguage();const small=usePhoneViewport();const [open,setOpen]=useState(!small);
 useEffect(()=>setOpen(!small),[small]);
 return <details className="responsive-filters" open={open} onToggle={e=>setOpen(e.currentTarget.open)}><summary>{t('filters')}{count>0&&<span>{count}</span>}<ChevronDown size={18}/></summary><div className="filter-options">{children}</div></details>;
}
export function ActiveFilters({items,onRemove}:{items:{key:string;label:string}[];onRemove:(key:string)=>void}){
 const {t}=useLanguage();if(!items.length)return null;
 return <div className="active-filters" aria-label={t('mwSelectedFilters')}>{items.map(item=><button type="button" key={item.key} onClick={()=>onRemove(item.key)} aria-label={t('mwRemoveFilter')+': '+item.label}><span>{item.label}</span><X size={14}/></button>)}</div>;
}
export function selectedFilters(params:URLSearchParams,tax:Partial<Taxonomy>|undefined,lang:Lang,t:(key:string)=>string){
 return ['q','category_id','region_id','vendor_id','sort'].filter(key=>params.has(key)&&params.get(key)).map(key=>{
 const value=params.get(key)!;
 const category=tax?.categories?.find(c=>String(c.id)===value||String(c.id)===String(tax.aliases?.find(a=>String(a.source_id)===value)?.target_id));
 const region=tax?.regions?.find(r=>String(r.id)===value);
 const name=(item:{name:string;name_en?:string}|undefined)=>item?(lang==='en'&&item.name_en?item.name_en:item.name):'';
 const label=key==='q'?value:key==='category_id'?name(category)||t('category')+' #'+value:key==='region_id'?name(region)||t('region')+' #'+value:key==='vendor_id'?t('vendor')+' #'+value:t(value==='name'?'sortName':'sortProductLatest');
 return {key,label};
 });
}
export function useHorizontalTabs(active:string){
 const ref=useRef<HTMLDivElement>(null);
 useEffect(()=>{const position=()=>{const root=ref.current;const tab=root?.querySelector<HTMLElement>('[aria-selected="true"]');if(!root||!tab)return;const left=tab.offsetLeft-root.offsetLeft;root.scrollLeft=Math.max(0,left-(root.clientWidth-tab.offsetWidth)/2)};const frame=requestAnimationFrame(position);window.addEventListener('resize',position);return()=>{cancelAnimationFrame(frame);window.removeEventListener('resize',position)}},[active]);
 return ref;
}
export function phoneNumbers(value:string){
 const clean=value.replace(/[（(]([^）)]*)[）)]/g,(_,note:string)=>/^\d{2,5}$/.test(note.trim())?note:' ');
 return [...new Set((clean.match(/\+?\d[\d -]{5,}\d/g)||[]).map(n=>n.replace(/[^\d+]/g,'')).filter(n=>{const size=n.replace(/\+/g,'').length;return size>=7&&size<=15}))];
}
export function PublicPhone({value,compact=false,className='',withCopy=false}:{value:string;compact?:boolean;className?:string;withCopy?:boolean}){
 const {t}=useLanguage();const id=useId();const numbers=phoneNumbers(value);const [open,setOpen]=useState(false);const [copy,setCopy]=useState('');
 const copyPhone=async()=>{try{await navigator.clipboard.writeText(value);setCopy('copied')}catch{setCopy('srCopyFailed')}};
 const label=<><Phone size={17}/><span>{compact?t('phone'):value}</span></>;
 return <>{numbers.length===1?<a className={className} href={'tel:'+numbers[0]}>{label}</a>:<button type="button" className={className} onClick={()=>setOpen(true)}>{label}</button>}{withCopy&&<><button type="button" className="button secondary" onClick={()=>void copyPhone()}>{t('srCopyPhone')}</button>{copy&&<p role="status">{t(copy)}</p>}{copy==='srCopyFailed'&&<input readOnly value={value} aria-label={t('phone')} onFocus={e=>e.target.select()}/>}</>}{open&&<Modal close={()=>setOpen(false)}><div className="public-phone-dialog"><h2 id={id}>{t(numbers.length?'mwChoosePhone':'phone')}</h2><p>{value}</p>{numbers.map(n=><a className="button" key={n} href={'tel:'+n}><Phone size={17}/>{n}</a>)}<button className="button secondary" onClick={()=>void copyPhone()}>{t('srCopyPhone')}</button>{copy&&<p role="status">{t(copy)}</p>}{copy==='srCopyFailed'&&<input readOnly value={value} aria-label={t('phone')} onFocus={e=>e.target.select()}/>}</div></Modal>}</>;
}

export function useQuickContact(id:number|undefined,language:string){
 const ref=useRef<HTMLDivElement>(null);
 useEffect(()=>{const bar=ref.current,root=bar?.closest<HTMLElement>('.showroom-v2');if(!bar||!root)return;const measure=()=>{const h=bar.getBoundingClientRect().height;if(h>0)root.style.setProperty('--quick-contact-height',(h+12)+'px')};const keyboard=()=>{const input=document.activeElement?.matches('input,textarea,select');root.classList.toggle('keyboard-open',!!input&&!!window.visualViewport&&window.visualViewport.height<window.innerHeight*.75)};measure();const observer=typeof ResizeObserver==='undefined'?undefined:new ResizeObserver(measure);observer?.observe(bar);window.visualViewport?.addEventListener('resize',keyboard);document.addEventListener('focusin',keyboard);document.addEventListener('focusout',keyboard);return()=>{observer?.disconnect();window.visualViewport?.removeEventListener('resize',keyboard);document.removeEventListener('focusin',keyboard);document.removeEventListener('focusout',keyboard);root.classList.remove('keyboard-open')}},[id,language]);
 return ref;
}
