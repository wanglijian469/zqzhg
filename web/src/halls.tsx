import {runtimeConfig} from './runtime-config';
import {mediaURL} from './runtime-config';
import {canonicalPublicURL,filteredSearch,showroomSearchVariant} from './seo';
import {HomeVisual} from './home-display';
import {createContext,useContext,useEffect,useRef,useState,type ReactNode} from 'react';
import {Link,useLocation,useNavigate} from 'react-router-dom';
import {Cable,Check,ChevronDown,Factory,Tractor} from 'lucide-react';
import {api} from './api';
import {useLanguage,ErrorState,Loading} from './main';
import type {Hall,Item} from './types';

export function hallHref(path:string,slug='agriculture'){
 const url=new URL(path,'https://zqzhg.com');
 if(slug==='agriculture')url.searchParams.delete('hall');else url.searchParams.set('hall',slug);
 return url.pathname+url.search+url.hash;
}
const HallContext=createContext({halls:[] as Hall[],hall:undefined as Hall|undefined,slug:'agriculture',error:'',loading:false,reload:()=>{},setContentHall:(_id:number)=>{},href:(path:string)=>path});
export function HallProvider({children}:{children:ReactNode}){
 const location=useLocation();const [version,setVersion]=useState(0);const [result,setResult]=useState<{items?:Hall[];error?:string}>();
 const [content,setContent]=useState<{path:string;id:number}>();
 useEffect(()=>{const ctrl=new AbortController();setResult(undefined);api<{items:Hall[]}>('/public/halls',{signal:ctrl.signal}).then(r=>{if(!ctrl.signal.aborted)setResult(Array.isArray(r.items)?r:{error:'internal_error'})}).catch(e=>{if(!ctrl.signal.aborted)setResult({error:e.code||'network_error'})});return()=>ctrl.abort()},[version]);
 const halls=result?.items||[];const query=new URLSearchParams(location.search).get('hall')||'agriculture';
 const detail=/^\/(?:products|vendors|v)\//.test(location.pathname);const own=detail&&content?.path===location.pathname?halls.find(h=>h.id===content.id):undefined;
 const hall=own||halls.find(h=>h.slug===query);const slug=hall?.slug||query;
 return <HallContext.Provider value={{halls,hall,slug,error:result?.error||'',loading:!result,reload:()=>setVersion(v=>v+1),setContentHall:id=>setContent(current=>current?.path===location.pathname&&current.id===id?current:{path:location.pathname,id}),href:path=>hallHref(path,slug)}}>{children}</HallContext.Provider>;
}
export const useHall=()=>useContext(HallContext);
export function useContentHall(item?:Item){const {setContentHall}=useHall();useEffect(()=>{if(item?.hall_id)setContentHall(item.hall_id)},[item?.id,item?.hall_id,setContentHall])}
export function HallSwitcher(){
 const {hall,halls}=useHall();const {lang,t}=useLanguage();const [open,setOpen]=useState(false);const ref=useRef<HTMLDivElement>(null);const trigger=useRef<HTMLButtonElement>(null);const navigate=useNavigate();const location=useLocation();
 useEffect(()=>setOpen(false),[location.pathname,location.search]);
 useEffect(()=>{if(!open)return;const close=(e:PointerEvent)=>{if(!ref.current?.contains(e.target as Node))setOpen(false)};document.addEventListener('pointerdown',close);return()=>document.removeEventListener('pointerdown',close)},[open]);
 return <div className="hall-switcher" ref={ref} onKeyDown={e=>{const buttons=Array.from(ref.current?.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')||[]);if(e.key==='Escape'){setOpen(false);trigger.current?.focus()}if(e.key==='ArrowDown'||e.key==='ArrowUp'){e.preventDefault();if(!open){setOpen(true);requestAnimationFrame(()=>ref.current?.querySelector<HTMLButtonElement>('[role="menuitem"]')?.focus())}else{const index=buttons.indexOf(document.activeElement as HTMLButtonElement);buttons[(index+(e.key==='ArrowDown'?1:buttons.length-1)+buttons.length)%buttons.length]?.focus()}}}}><button className="hall-trigger" ref={trigger} aria-haspopup="menu" aria-expanded={open} onClick={()=>setOpen(!open)}>{hall?(lang==='en'?hall.name_en:hall.name):t('selectHall')}<ChevronDown size={14}/></button>{open&&<div className="hall-menu" role="menu" aria-label={t('selectHall')}>{halls.map(h=>{const Icon=h.icon==='tractor'?Tractor:h.icon==='cable'?Cable:Factory;return <button role="menuitem" key={h.id} onClick={()=>{setOpen(false);navigate(hallHref('/',h.slug));trigger.current?.focus()}}><Icon size={23}/><span><strong>{lang==='en'?h.name_en:h.name}</strong><small>{t(h.status==='open'?'hallOpen':'hallPreparing')}</small></span>{h.id===hall?.id&&<Check size={16}/>}</button>})}</div>}</div>;
}
export function HallGate({children}:{children:ReactNode}){const {loading,error,reload,hall}=useHall();if(loading)return <div className="container section"><Loading/></div>;if(error)return <div className="container section"><ErrorState code={error} retry={reload}/></div>;if(!hall)return <div className="container section"><ErrorState code="not_found"/></div>;return children}
export function PreparingHall(){const {hall}=useHall();const {lang,t}=useLanguage();return <section className="container preparing-hall"><span className="preparing-symbol"><Cable size={48}/></span><span className="eyebrow">ZQZG / INDUSTRIAL EXHIBITION</span><h1>{t('brand')} · {lang==='en'?hall?.name_en:hall?.name}</h1><span className="status-pill">{t('hallPreparing')}</span><p>{(lang==='en'?hall?.description_en:hall?.description)||t('preparingDescription')}</p>{!!hall?.hero_media_id&&<div className="preparing-visual"><HomeVisual id={hall.hero_media_id}/></div>}<Link className="button" to="/">{t('backOpenHall')}</Link></section>}
export function usePageMetadata(title:string,description:string,canonical?:string,unavailable=false,imageID=0){
 const location=useLocation();const {hall}=useHall();
 useEffect(()=>{if(!title)return;const base=runtimeConfig().site.web_url||document.querySelector<HTMLMetaElement>('meta[name="zqzg-public-base"]')?.content||window.location.origin;
 const url=canonicalPublicURL(location.pathname,location.search,base,canonical);
 const robots=unavailable||!hall||hall.status==='preparing'||filteredSearch(location.search)||showroomSearchVariant(location.pathname,location.search)||/^\/(admin|merchant|join|buyer|procurement)(\/|$)/.test(location.pathname)?'noindex,follow':'index,follow';
 document.title=title;const meta=(key:string,value:string,property=false)=>{const selector=property?'property':'name';let node=document.querySelector<HTMLMetaElement>('meta['+selector+'="'+key+'"]');if(!node){node=document.createElement('meta');node.setAttribute(selector,key);document.head.append(node)}node.content=value;document.querySelectorAll('meta['+selector+'="'+key+'"]').forEach(other=>{if(other!==node)other.remove()})};
 meta('description',description);meta('robots',robots);meta('og:title',title,true);meta('og:description',description,true);meta('og:url',url,true);
 if(imageID)meta('og:image',new URL(mediaURL(imageID),base).href,true);else document.querySelectorAll('meta[property="og:image"]').forEach(n=>n.remove());
 let link=document.querySelector<HTMLLinkElement>('link[rel="canonical"]');if(!link){link=document.createElement('link');link.rel='canonical';document.head.append(link)}link.href=url;document.querySelectorAll('link[rel="canonical"]').forEach(n=>{if(n!==link)n.remove()});
 document.querySelectorAll('script[type="application/ld+json"]').forEach(node=>{try{if(JSON.parse(node.textContent||'{}').url!==url)node.remove()}catch{node.remove()}});
 },[title,description,canonical,unavailable,imageID,location.pathname,location.search,hall?.status,runtimeConfig().site.web_url]);
}
