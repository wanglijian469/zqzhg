import {lazy,Suspense,useCallback,useEffect,useLayoutEffect,useRef,useState} from 'react';
import {Link,useLocation,useSearchParams} from 'react-router-dom';
import {ArrowUpRight,ChevronLeft,ChevronRight,Home,Info,MapPin,Search,SlidersHorizontal,X} from 'lucide-react';
import {display,ErrorState,Loading,Pager,useLanguage,useLoad} from './main';
import {useHall} from './halls';
import {VendorLogo} from './vendor-brand';
import {vendorPath} from './showroom-share';
import {useDebouncedSearch} from './search-debounce';
import type {Page,Taxonomy} from './types';
import type {RegionStatistics} from './industry-map-types';
import './industry-map.css';

function UnavailableChart(){const {t}=useLanguage();return <div className="industry-map-state" role="status"><MapPin size={32}/><p>{t('imMapFailed')}</p><button className="button secondary" onClick={()=>window.location.reload()}>{t('retry')}</button></div>}
const IndustryMapChart=lazy(()=>import('./industry-map-chart').catch(()=>({default:UnavailableChart})));

// Cards are 96px high, with 8px gaps. The pager has a separate grid row.
export function fittedVendorPageSize(height:number){return Math.max(1,Math.min(8,Math.floor((height+8)/104)))}
function useCompactMap(){
 const [compact,setCompact]=useState(()=>window.matchMedia?.('(max-width:1023px) and (min-height:501px)').matches??false);
 useEffect(()=>{const query=window.matchMedia?.('(max-width:1023px) and (min-height:501px)');if(!query)return;const update=()=>setCompact(query.matches);update();query.addEventListener('change',update);return()=>query.removeEventListener('change',update)},[]);
 return compact;
}

export function IndustryMapPage(){
 const {lang,t}=useLanguage();const {href,slug}=useHall();const location=useLocation();const [params,setParams]=useSearchParams();const compact=useCompactMap();
 const [query,setQuery]=useState(params.get('q')||'');const [regionSearch,setRegionSearch]=useState('');
 const [pickerOpen,setPickerOpen]=useState(false);const [filtersOpen,setFiltersOpen]=useState(false);const [infoOpen,setInfoOpen]=useState(false);
 const picker=useRef<HTMLDivElement>(null);const pickerTrigger=useRef<HTMLButtonElement>(null);const listViewport=useRef<HTMLDivElement>(null);
 const [pageSize,setPageSize]=useState<number>();const lastCapacity=useRef<number|undefined>(undefined);
 const region=params.get('region_code')||'';const unassigned=params.get('unassigned')==='1';const view=params.get('view')==='vendors'?'vendors':'map';
 const filter=new URLSearchParams();for(const key of ['region_code','q','category_id'])if(params.get(key))filter.set(key,params.get(key)!);
 const stats=useLoad<RegionStatistics>(href('/public/industry-map/regions?'+filter));
 const previousStatistics=useRef<RegionStatistics|undefined>(undefined);if(stats.data)previousStatistics.current=stats.data;const chartStatistics=stats.data||previousStatistics.current;
 const listQuery=new URLSearchParams(filter);for(const key of ['page','unassigned'])if(params.get(key))listQuery.set(key,params.get(key)!);listQuery.set('page_size',String(pageSize||1));
 const results=useLoad<Page>(href('/public/industry-map/vendors?'+listQuery),undefined,!!stats.data&&pageSize!==undefined);
 const taxonomy=useLoad<Taxonomy>(href('/public/taxonomy?category_kind=vendor'));
 const navigation=useRef({params,setParams});navigation.current={params,setParams};
 const update=(key:string,value:string,replace=false)=>{const next=new URLSearchParams(params);value?next.set(key,value):next.delete(key);if(!['page','view'].includes(key))next.delete('page');setParams(next,{replace})};
 const selectRegion=useCallback((code:string)=>{const current=navigation.current;const next=new URLSearchParams(current.params);code?next.set('region_code',code):next.delete('region_code');next.delete('page');next.delete('unassigned');current.setParams(next);setPickerOpen(false);setRegionSearch('')},[]);
 const search=useDebouncedSearch(query,params.get('q')||'',(value,replace)=>update('q',value,replace),'/regions:'+slug);
 useEffect(()=>setQuery(params.get('q')||''),[params.get('q')]);
 useEffect(()=>{setRegionSearch('');setPickerOpen(false);setFiltersOpen(false)},[region,slug]);
 useEffect(()=>{if(!pickerOpen)return;const outside=(event:PointerEvent)=>{if(!picker.current?.contains(event.target as Node))setPickerOpen(false)};document.addEventListener('pointerdown',outside);return()=>document.removeEventListener('pointerdown',outside)},[pickerOpen]);
 useLayoutEffect(()=>{
  const element=listViewport.current;if(!element)return;
  const measure=()=>{const height=element.getBoundingClientRect().height;const capacity=height>0?fittedVendorPageSize(height):lastCapacity.current??4;
   if(lastCapacity.current!==undefined&&lastCapacity.current!==capacity){const current=navigation.current;if(current.params.has('page')){const next=new URLSearchParams(current.params);next.delete('page');current.setParams(next,{replace:true})}}
   lastCapacity.current=capacity;setPageSize(capacity);
  };
  measure();if(typeof ResizeObserver==='undefined')return;const observer=new ResizeObserver(measure);observer.observe(element);return()=>observer.disconnect();
 },[]);
 const clear=()=>{const next=new URLSearchParams(params);for(const key of ['q','category_id','unassigned','page'])next.delete(key);setQuery('');setParams(next)};
 const hasFilters=!!(params.get('q')||params.get('category_id')||unassigned);const currentName=stats.data?.region?.name||t('imNational');const label=unassigned?t(region?'imUnassigned':'imUnknown'):currentName;
 const children=stats.data?.items.filter(item=>item.name.includes(regionSearch.trim())||item.code.includes(regionSearch.trim()))||[];
 const parent=stats.data?.trail.at(-2)?.code||'';const returnTo=location.pathname+location.search;
 return <div className="industry-map-page" data-view={view} onKeyDown={event=>{if(event.key==='Escape'){setPickerOpen(false);setFiltersOpen(false);setInfoOpen(false);pickerTrigger.current?.focus()}}}>
  <div className="industry-toolbar">
   <div className="industry-title"><h1>{t('imTitle')}</h1><span aria-live="polite">{stats.data?.total.toLocaleString()??'—'} {t('imCompanies')}</span></div>
   <button type="button" className="industry-filter-toggle" aria-expanded={filtersOpen} aria-controls="industry-filters" onClick={()=>setFiltersOpen(open=>!open)}><SlidersHorizontal size={16}/>{t('imFilters')}{hasFilters&&<i/>}</button>
   <form id="industry-filters" className={'industry-filters'+(filtersOpen?' open':'')} onSubmit={event=>{event.preventDefault();search.submit();setFiltersOpen(false)}}>
    <label className="industry-keyword"><Search size={16}/><input aria-label={t('keyword')} maxLength={200} value={query} onChange={event=>setQuery(event.target.value)} onCompositionStart={search.onCompositionStart} onCompositionEnd={search.onCompositionEnd} placeholder={t('searchPlaceholder')}/></label>
    <select aria-label={t('imCategory')} value={params.get('category_id')||''} onChange={event=>update('category_id',event.target.value)}><option value="">{t('allCategories')}</option>{taxonomy.data?.categories.map(category=><option key={category.id} value={category.id}>{category.parent_id?'↳ ':''}{lang==='en'&&category.name_en?category.name_en:category.name}</option>)}</select>
    <button className="button" type="submit">{t('imSearch')}</button>{hasFilters&&<button className="industry-clear" type="button" aria-label={t('imClear')} onClick={clear}><X size={16}/></button>}
   </form>
   <div className="industry-info"><button className="icon-button" aria-label={t('imDataInfo')} aria-expanded={infoOpen} onClick={()=>setInfoOpen(open=>!open)}><Info size={19}/></button>{infoOpen&&<div className="industry-info-popover" role="note"><strong>{t('imDataInfo')}</strong><p>{t('imCoverage')}</p><p>{t('imCountHelp')}</p></div>}</div>
  </div>
  <div className="industry-navigation">
   <div className="industry-level-controls"><button aria-label={t('imBackLevel')} disabled={!region} onClick={()=>selectRegion(parent)}><ChevronLeft size={17}/></button><button aria-label={t('imHome')} disabled={!region&&!unassigned} onClick={()=>selectRegion('')}><Home size={16}/></button></div>
   <nav className="industry-breadcrumb" aria-label={t('imBreadcrumb')}><button onClick={()=>selectRegion('')}>{t('imNational')}</button>{stats.data?.trail.map(node=><span key={node.code}><ChevronRight size={12}/><button onClick={()=>selectRegion(node.code)} aria-current={node.code===region&&!unassigned?'page':undefined}>{node.name}</button></span>)}{unassigned&&<span><ChevronRight size={12}/><b>{label}</b></span>}</nav>
   <div className="industry-picker" ref={picker}><button ref={pickerTrigger} aria-expanded={pickerOpen} aria-controls="industry-region-options" onClick={()=>setPickerOpen(open=>!open)} disabled={!stats.data?.items.length}>{t('imChooseRegion')}<ChevronRight size={13}/></button>{pickerOpen&&<div id="industry-region-options" className="industry-picker-popover" role="dialog" aria-label={t('imChooseRegion')}><input autoFocus aria-label={t('imRegionSearch')} value={regionSearch} onChange={event=>setRegionSearch(event.target.value)} placeholder={t('imRegionSearch')}/><div className="industry-region-options">{children.map(item=><button key={item.code} onClick={()=>selectRegion(item.code)}><span>{item.name}</span><b>{item.vendor_count.toLocaleString()}</b></button>)}{!children.length&&<p>{t('imNoRegions')}</p>}</div></div>}</div>
   {!!stats.data?.unassigned_count&&<button className="industry-unassigned" aria-pressed={unassigned} title={t('imUnknownHelp')} onClick={()=>update('unassigned',unassigned?'':'1')}>{t(region?'imUnassigned':'imUnknown')} {stats.data.unassigned_count}</button>}
  </div>
  <div className="industry-view-switch" aria-label={t('imView')}><button aria-pressed={view==='map'} onClick={()=>update('view','')}>{t('imRegionView')}</button><button aria-pressed={view==='vendors'} onClick={()=>update('view','vendors')}>{t('imVendorView')} {stats.data?.total??''}</button></div>
  <div className="industry-layout">
   <section className="industry-map-panel" aria-label={t('imNavigation')} aria-hidden={compact&&view!=='map'}>{chartStatistics&&<Suspense fallback={<Loading/>}><IndustryMapChart statistics={chartStatistics} onSelect={selectRegion} onMissing={()=>setPickerOpen(true)}/></Suspense>}{stats.error?<ErrorState code={stats.error} retry={stats.reload}/>:!stats.data?<Loading/>:null}{search.waiting&&<span className="industry-search-status" role="status">{t('searchPending')}</span>}</section>
   <section className="industry-vendors" aria-label={t('imVendorView')} aria-hidden={compact&&view!=='vendors'}>
    <div className="industry-panel-heading"><h2 title={label}>{label}</h2><span>{t('imResults')} <b>{results.data?.total.toLocaleString()??'—'}</b></span></div>
    <div className="industry-list-viewport" ref={listViewport}>
     {stats.error?<ErrorState code={stats.error} retry={stats.reload}/>:results.error?<ErrorState code={results.error} retry={results.reload}/>:!results.data?<Loading/>:!results.data.items.length?<div className="industry-empty"><Search size={28}/><p>{t('imEmpty')}</p>{hasFilters&&<button className="text-link" onClick={clear}>{t('imClear')}</button>}</div>:<div className="industry-vendor-list">{results.data.items.map(item=><article className="industry-vendor" key={item.id}><div className="industry-vendor-logo"><VendorLogo data={item.data} alt={display(item.data,lang,'name')+' Logo'}/></div><div className="industry-vendor-copy"><Link className="industry-vendor-name" to={href(vendorPath(item))} state={{returnTo}} title={display(item.data,lang,'name')}><h3>{display(item.data,lang,'name')}</h3></Link><p title={item.data.main_products}>{item.data.main_products||t('noMainProducts')}</p><small title={item.data.region_name}><MapPin size={12}/>{item.data.region_name||t('imUnknown')}</small><Link className="text-link" to={href(vendorPath(item))} state={{returnTo}}>{t('imViewHall')}<ArrowUpRight size={12}/></Link></div></article>)}</div>}
    </div>
    <div className="industry-list-footer">{unassigned&&<button className="text-link" onClick={()=>update('unassigned','')}>{t('imAllCompanies')}</button>}{results.data&&<Pager page={results.data.page} size={results.data.page_size} total={results.data.total} onPage={page=>update('page',String(page))}/>}</div>
   </section>
  </div>
  {taxonomy.error&&<div className="industry-taxonomy-error"><ErrorState code={taxonomy.error} retry={taxonomy.reload}/></div>}
 </div>;
}
