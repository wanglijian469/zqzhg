import {useEffect,useRef,useState,type ReactNode} from 'react';
import {ImageIcon} from 'lucide-react';
import {token} from './api';
import {mediaURL} from './runtime-config';
import './image-loading.css';

export type ImagePreset='thumbnail'|'card'|'hero'|'detail';
export interface MediaImageProps {
 id:number;role?:'admin'|'merchant';thumb?:boolean;className?:string;onClick?:()=>void;
 fit?:'contain'|'cover';alt?:string;fallback?:ReactNode;priority?:boolean;preset?:ImagePreset;sizes?:string;
}
const widths:Record<ImagePreset,number[]>={thumbnail:[240,480],card:[480,960],hero:[960,1440],detail:[480,960,1440]};
const imageSizes:Record<ImagePreset,string>={thumbnail:'96px',card:'(max-width: 650px) 46vw, (max-width: 1000px) 30vw, 360px',hero:'(max-width: 650px) calc(100vw - 36px), (max-width: 1000px) 45vw, 640px',detail:'(max-width: 650px) calc(100vw - 36px), 700px'};

export function useViewportMatch(query:string){
 const [matches,setMatches]=useState(()=>typeof matchMedia!=='undefined'&&matchMedia(query).matches);
 useEffect(()=>{if(typeof matchMedia==='undefined')return;const media=matchMedia(query);const update=()=>setMatches(media.matches);update();media.addEventListener('change',update);return()=>media.removeEventListener('change',update)},[query]);
 return matches;
}

export function OptimizedMediaImage({id,role,thumb=true,className='',onClick,fit='contain',alt='',fallback,priority=false,preset,sizes,unavailableLabel}:{unavailableLabel:string}&MediaImageProps){
 const element=useRef<HTMLElement|null>(null);const session=role?token(role):'';
 const selected=preset||(thumb?'card':undefined);const candidates=selected?widths[selected]:[];
 const base=mediaURL(id);const key=JSON.stringify([base,role,session,thumb,selected]);
 const [visibility,setVisibility]=useState<{key:string;width:number}>();
 const [result,setResult]=useState<{key:string;src?:string;failed?:boolean}>();
 const current=result?.key===key?result:undefined;
 const visible=priority||visibility?.key===key||typeof IntersectionObserver==='undefined';
 useEffect(()=>{
  const node=element.current;if(!node)return;
  const reveal=()=>{const box=node.getBoundingClientRect();setVisibility({key,width:Math.max(box.width,box.height,96)*(globalThis.devicePixelRatio||1)})};
  if(priority||typeof IntersectionObserver==='undefined'){reveal();return}
  const observer=new IntersectionObserver(entries=>{if(entries.some(entry=>entry.isIntersecting)){reveal();observer.disconnect()}},{rootMargin:'300px'});
  observer.observe(node);return()=>observer.disconnect();
 },[key,priority]);
 const privateWidth=candidates.find(w=>w>=(visibility?.width||480))||candidates.at(-1);
 const query=selected?'?w='+(role?privateWidth:candidates[0]):'';
 useEffect(()=>{
  if(!role||!visible)return;
  const controller=new AbortController();let url='';
  fetch(base+query,{headers:{Authorization:'Bearer '+session},signal:controller.signal,cache:'no-store'})
   .then(response=>{if(!response.ok)throw Error('image_unavailable');return response.blob()})
   .then(blob=>{if(!controller.signal.aborted){url=URL.createObjectURL(blob);setResult({key,src:url})}})
   .catch(()=>{if(!controller.signal.aborted)setResult({key,failed:true})});
  return()=>{controller.abort();if(url)URL.revokeObjectURL(url)};
 },[key,role,session,visible,base,query]);
 const ref=(node:HTMLElement|null)=>{element.current=node};
 if(current?.failed)return <>{fallback||<span className={'image-failed '+className} role="img" aria-label={alt||unavailableLabel}><ImageIcon/></span>}</>;
 if(!visible||role&&!current?.src)return <span ref={ref} className={'image-loading '+className} aria-label={alt} data-image-pending="true"/>;
 return <img ref={ref} key={key} src={role?current?.src:base+query}
  srcSet={!role&&selected?candidates.map(w=>base+'?w='+w+' '+w+'w').join(', '):undefined}
  sizes={!role&&selected?(sizes||imageSizes[selected]):undefined}
  className={className} loading={priority?'eager':'lazy'} fetchPriority={priority?'high':undefined}
  decoding="async" alt={alt} style={{objectFit:fit}} onError={()=>setResult({key,failed:true})} onClick={onClick}/>;
}
