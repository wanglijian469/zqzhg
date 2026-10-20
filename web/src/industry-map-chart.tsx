import {useEffect,useRef,useState} from 'react';
import type {OutlineBoundary,RegionStatistics} from './industry-map-types';
import {loadBoundary} from './industry-map-resources';
import {loadMapPreview,mapPreviewAvailable} from './industry-map-preview';
import {useLanguage} from './main';
import {MapPin,Minus,Plus,RotateCcw} from 'lucide-react';

type MapControls={dispose:()=>void;resize:()=>void;zoom:(factor:number)=>void;reset:()=>void;update?:(statistics:RegionStatistics)=>void};
export default function IndustryMapChart({statistics,onSelect,onMissing}:{statistics:RegionStatistics;onSelect:(code:string)=>void;onMissing?:()=>void}){
 const {t,lang}=useLanguage();const container=useRef<HTMLDivElement>(null);
 const [state,setState]=useState('loading'),[attempt,setAttempt]=useState(0),[schematic,setSchematic]=useState(false);
 const [verified,setVerified]=useState<{source:string;review_number:string}|null>(null);
 const [available,setAvailable]=useState(false),[mode,setMode]=useState('schematic'),[resource,setResource]=useState<OutlineBoundary>();
 const controls=useRef<MapControls|null>(null),latest=useRef(statistics),previous=useRef(statistics);latest.current=statistics;
 const code=statistics.region?.code||'';
 useEffect(()=>{
  const controller=new AbortController();let chart:MapControls|undefined,observer:ResizeObserver|undefined,moduleFailed=false;
  setState('loading');setResource(undefined);setVerified(null);
  (async()=>{
   const approved=schematic?null:await loadBoundary(code,controller.signal);
   const enabled=approved?false:schematic?false:await mapPreviewAvailable(controller.signal).catch(error=>{if(controller.signal.aborted)throw error;return false});
   if(controller.signal.aborted)return;
   if(!schematic)setAvailable(enabled||!!approved);
   setMode(enabled||approved?'outline':'schematic');
   let instance:MapControls;
   if(enabled||approved){
    const codes=new Set(statistics.items.length?statistics.items.map(item=>item.code):[code]);
    const boundary:OutlineBoundary=approved?{region_code:code,sha256:approved.resource.sha256,missing_codes:[...codes].filter(code=>!approved.geometry.features.some(f=>f.properties.adcode===code)),geometry:{type:'FeatureCollection',features:approved.geometry.features.map(f=>({...f,properties:{...f.properties,mapped:codes.has(f.properties.adcode)}}))}}:await loadMapPreview(code,controller.signal,attempt>0);
    if(approved)setVerified({source:approved.resource.source,review_number:approved.resource.review_number});
    if(controller.signal.aborted)return;
    let renderOutlineMap:typeof import('./industry-map-outline').renderOutlineMap;
    try{({renderOutlineMap}=await import('./industry-map-outline'))}catch(error){moduleFailed=true;throw error}
    if(controller.signal.aborted||!container.current)return;
    instance=renderOutlineMap(container.current,boundary,latest.current,onSelect,t);setResource(boundary);
   }else{
    let renderSchematicMap:typeof import('./industry-map-schematic').renderSchematicMap;
    try{({renderSchematicMap}=await import('./industry-map-schematic'))}catch(error){moduleFailed=true;throw error}
    if(controller.signal.aborted||!container.current)return;
    instance=renderSchematicMap(container.current,latest.current,onSelect,t);
   }
   chart=instance;controls.current=instance;
   if(typeof ResizeObserver!=='undefined'){observer=new ResizeObserver(()=>{if(controller.signal.aborted)return;try{instance.resize()}catch{observer?.disconnect();chart?.dispose();chart=undefined;controls.current=null;setState('failed')}});observer.observe(container.current!)}
   setState('ready');
  })().catch(()=>{if(!controller.signal.aborted)setState(moduleFailed?'module-failed':'failed')});
  return()=>{controller.abort();observer?.disconnect();chart?.dispose();controls.current=null};
 },[code,onSelect,lang,attempt,schematic]);
 useEffect(()=>{
  if(previous.current===statistics)return;previous.current=statistics;
  if(controls.current?.update){try{controls.current.update(statistics)}catch{setState('failed')}}
  else if(controls.current)setAttempt(n=>n+1);
 },[statistics]);
 const failed=state==='failed'||state==='module-failed';
 return <div className="industry-chart-wrapper industry-schematic-wrapper" data-map-mode={mode}>
  <div className="industry-diagram-heading"><strong>{statistics.region?.name||t('imNational')}</strong><span>{statistics.items.length?t('imDrillHint'):t('imLeaf')}</span></div>
  <div className="industry-chart" ref={container} role="img" aria-label={t(mode==='outline'?'imOutlineGraphic':'imGraphic')}/>
  {state==='loading'?<div className="loading" role="status">{t('loading')}</div>:failed?<div className="industry-map-state" role="status"><MapPin size={32}/><p>{t('imMapFailed')}</p><button className="button secondary" onClick={()=>state==='module-failed'?window.location.reload():setAttempt(n=>n+1)}>{t('retry')}</button>{available&&!schematic&&<button className="text-link" onClick={()=>setSchematic(true)}>{t('imUseSchematic')}</button>}</div>:null}
  {state==='ready'&&<div className="industry-map-controls"><button aria-label={t('imZoomIn')} onClick={()=>controls.current?.zoom(1.3)}><Plus size={17}/></button><button aria-label={t('imZoomOut')} onClick={()=>controls.current?.zoom(1/1.3)}><Minus size={17}/></button><button aria-label={t('imResetMap')} onClick={()=>controls.current?.reset()}><RotateCcw size={16}/></button></div>}
  {!!resource?.missing_codes.length&&<button className="industry-missing-outlines" onClick={onMissing}>{t('imMissingOutlines')} {resource.missing_codes.length} · {t('imChooseRegion')}</button>}
  {available&&schematic&&<button className="industry-missing-outlines" onClick={()=>setSchematic(false)}>{t('imReturnOutline')}</button>}
  <div className="industry-schematic-footer"><span className="industry-schematic-legend"><i/>{t('imLess')} — {t('imMore')}</span>{mode==='outline'&&verified?<small>{verified.source} · {verified.review_number}</small>:mode==='outline'?<a href="https://help.aliyun.com/zh/datav/datav-7-0/user-guide/datav-geoatlas-widgets/" target="_blank" rel="noreferrer">{t('imPreviewNote')}</a>:<small>{t('imSchematicNote')}</small>}</div>
 </div>;
}
