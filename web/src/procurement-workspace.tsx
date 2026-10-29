import {useEffect,useRef,useState,type ReactNode} from 'react';
import {Link,useLocation,useNavigate} from 'react-router-dom';
import {nativeWorkspace,useMobileWorkspace} from './mobile-workspace';
import './procurement-mobile.css';
import {Bell,ChevronLeft,ChevronRight,ClipboardList,ContactRound,Globe2,Home,Menu,SquarePen,Store,Tractor,X} from 'lucide-react';
import {useLanguage} from './ui';
import {ProcurementDemoBanner} from './procurement-demo-banner';
import './merchant-workspace.css';
import './workspace-upgrade.css';

const entries=[
 {path:'/buyer',zh:'我的询价与需求',en:'My requests',Icon:ClipboardList},
 {path:'/buyer/new',zh:'发布采购需求',en:'Create request',Icon:SquarePen},
 {path:'/procurement',zh:'采购需求大厅',en:'Request hall',Icon:Store},
 {path:'/buyer/notifications',zh:'消息通知',en:'Notifications',Icon:Bell},
 {path:'/buyer/profile',zh:'采购商资料',en:'Buyer profile',Icon:ContactRound},
];

export function ProcurementWorkspace({children,notifications}:{children:ReactNode;notifications:ReactNode}){
 const {lang,t,toggle}=useLanguage(),en=lang==='en';const location=useLocation();
 const mobile=useMobileWorkspace(),navigate=useNavigate();
 const [collapsed,setCollapsed]=useState(()=>sessionStorage.getItem('zqzt.procurement.collapsed')==='1'),[drawer,setDrawer]=useState(false);
 const aside=useRef<HTMLElement>(null),toggleRef=useRef<HTMLButtonElement>(null);
 const active=entries.find(entry=>entry.path==='/buyer'?location.pathname==='/buyer'||location.pathname.startsWith('/buyer/projects/'):location.pathname===entry.path)||entries[0];
 const center=en?'Procurement center':'采购中心',openLabel=en?'Open procurement navigation':'打开采购中心导航';
 const pageTitle=/^\/procurement\/[1-9][0-9]*$/.test(location.pathname)?(en?'Request details':'采购需求详情'):location.pathname==='/procurement'?center:en?active.en:active.zh;
 useEffect(()=>{document.title=pageTitle;let robots=document.querySelector<HTMLMetaElement>('meta[name=robots]');if(!robots){robots=document.createElement('meta');robots.name='robots';document.head.appendChild(robots)}robots.content='noindex,follow'},[pageTitle]);

 useEffect(()=>setDrawer(false),[location.pathname,location.search]);
 useEffect(()=>{const resize=()=>{if(window.innerWidth>900)setDrawer(false)};window.addEventListener('resize',resize);return()=>window.removeEventListener('resize',resize)},[]);
 useEffect(()=>{
  if(!drawer)return;
  const prior=document.activeElement as HTMLElement|null,overflow=document.body.style.overflow;
  document.body.style.overflow='hidden';aside.current?.querySelector<HTMLElement>('button,a')?.focus();
  const key=(e:KeyboardEvent)=>{
   if(e.key==='Escape'){e.preventDefault();setDrawer(false);return}
   if(e.key!=='Tab')return;
   const nodes=Array.from(aside.current?.querySelectorAll<HTMLElement>('a[href],button:not(:disabled)')||[]).filter(el=>el.getClientRects().length>0);
   if(!nodes.length)return;const first=nodes[0],last=nodes[nodes.length-1];
   if(e.shiftKey&&document.activeElement===first){e.preventDefault();last.focus()}else if(!e.shiftKey&&document.activeElement===last){e.preventDefault();first.focus()}
  };
  document.addEventListener('keydown',key);
  return()=>{document.body.style.overflow=overflow;document.removeEventListener('keydown',key);if(prior?.isConnected)prior.focus();else toggleRef.current?.focus()};
 },[drawer]);

 if(mobile)return <div className="proc-app proc-mobile">
  {!nativeWorkspace()&&<header className="proc-mobile-header"><button aria-label={en?'Back':'返回'} onClick={()=>{if((window.history.state?.idx||0)>0)navigate(-1);else navigate(location.pathname==='/procurement'?'/':'/procurement')}}><ChevronLeft/></button><strong>{location.pathname==='/procurement'?center:en?active.en:active.zh}</strong><button onClick={toggle} aria-label={en?'切换为中文':'Switch to English'}>{en?'中文':'English'}</button></header>}
  <main className="proc-mobile-content"><ProcurementDemoBanner/>{children}</main>
 </div>;
 return <div className={'proc-app merchant-workspace procurement-workspace'+(collapsed?' is-collapsed':'')+(drawer?' drawer-open':'')}>
  {drawer&&<button className="workspace-shade" aria-label={en?'Close procurement navigation overlay':'关闭采购导航遮罩'} onClick={()=>setDrawer(false)}/>}
  <aside id="procurement-navigation-panel" ref={aside} className="workspace-sidebar" aria-label={center} {...(drawer?{role:'dialog','aria-modal':true}:{})}>
   <Link className="workspace-brand" to="/" title={en?'Back to website':'返回网站'}><span><Tractor size={26}/></span><b>{t('brand')}<small>{center}</small></b></Link>
   <button className="workspace-drawer-close" aria-label={en?'Close procurement navigation':'关闭采购中心导航'} onClick={()=>setDrawer(false)}><X/></button>
   <p className="workspace-nav-label">{en?'PROCUREMENT':'采购服务'}</p>
   <nav aria-label={en?'Procurement navigation':'采购中心导航'}>{entries.map(({path,zh,en:english,Icon})=>{
    const selected=active.path===path;
    return <Link key={path} to={path} className={selected?'active':''} aria-current={selected?'page':undefined} title={en?english:zh}><Icon size={20}/><span>{path==='/buyer/notifications'?notifications:en?english:zh}</span></Link>;
   })}</nav>
   <div className="workspace-sidebar-bottom"><button onClick={()=>setCollapsed(value=>{sessionStorage.setItem('zqzt.procurement.collapsed',value?'0':'1');return !value})} aria-label={en?(collapsed?'Expand procurement navigation':'Collapse procurement navigation'):(collapsed?'展开采购导航':'收起采购导航')} aria-expanded={!collapsed}>{collapsed?<ChevronRight size={18}/>:<ChevronLeft size={18}/>}<span>{en?'Collapse navigation':'收起导航'}</span></button></div>
  </aside>
  <div className="workspace-main"><header className="workspace-topbar">
   <button ref={toggleRef} className="workspace-menu" aria-label={openLabel} aria-controls="procurement-navigation-panel" aria-expanded={drawer} onClick={()=>setDrawer(true)}><Menu/></button>
   <div className="workspace-topbar-title"><strong>{center}</strong><small>{center} / {en?active.en:active.zh}</small></div>
   <div className="workspace-topbar-actions"><Link to="/" title={en?'Back to website':'返回网站'}><Home size={17}/><span>{en?'Back to website':'返回网站'}</span></Link><button onClick={toggle} aria-label={en?'切换为中文':'Switch to English'}><Globe2 size={17}/>{en?'中文':'English'}</button></div>
  </header><main className="workspace-content proc-workspace-content"><ProcurementDemoBanner/>{children}</main></div>
 </div>;
}
