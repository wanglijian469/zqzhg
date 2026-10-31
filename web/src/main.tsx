import {seoMessages} from './search-engine-messages';
import {smsMessages} from './sms-challenge';
import {ServicesPage,ServiceRequestPage,serviceMessages} from './service-center';
import {publicPageSEO,productSEO} from './seo';
import {ExhibitionBreadcrumb} from './exhibition-portal';
import {ICPRegistration,ProcurementPaused} from './site-common';
import {nativeWorkspace} from './mobile-workspace';
import './procurement-mobile.css';
import {ProcurementNav,InquiryButton} from './procurement-links';
import {ResourceList} from './resources';
// Register POP protection before BrowserRouter mounts, including when editors are lazy.
import './use-unsaved-changes';
import {lazyRoute} from './lazy-route';
import {Language,ConfirmationProvider,useLanguage,useLoad,ErrorState,Loading,SectionTitle,ContentInfo,Contact,ItemCard,Pager,display} from './ui';
export * from './ui';
import {useRuntimeConfig,loadRuntime,brandName,runtimeConfig} from './runtime-config';
import {settingsMessages} from './settings-messages';
import {companyEditorMessages} from './company-editor-messages';
import {productMessages} from './product-messages';
import {HallProvider,HallSwitcher,HallGate,PreparingHall,useHall,useContentHall,usePageMetadata} from './halls';
import {v2Messages} from './v2-messages';
import {useAnalyticsNavigation} from './analytics';
import {merchantMessages} from './merchant-messages';
import {uiMessages} from './ui-messages';
import {ExhibitionHome} from './exhibition-home';
import React, {useEffect, useRef, useState} from 'react';
import {createRoot} from 'react-dom/client';
import {BrowserRouter, Link, NavLink, Navigate, Route, Routes, useLocation} from 'react-router-dom';
import {ArrowRight, ArrowUpRight, Building2, ChevronLeft, Globe2, Menu, Tractor, X} from 'lucide-react';
import {token} from './api';
import {messages, type MessageKey} from './i18n';
import {type Item, type Lang, type Page} from './types';
import './style.css';
import './experience.css';
import {resourceMessages} from './resource-messages';
import {experienceMessages} from './experience-messages';
import {AppDownload} from './app-download';
import {adminMessages} from './admin-messages';
import {operationsMessages} from './operations-messages';
import {showroomMessages} from './showroom-messages';
import {Showroom} from './showroom';
import {vendorPath} from './showroom-share';

import './industrial.css';
import './vendor-list.css';
import './v2.css';
import './showroom-v2.css';
import './mobile-web.css';
import {BrowsePage} from './browse-page';
import {IndustryMapPage} from './industry-map';
import {industryMapMessages} from './industry-map-messages';


const LegacyAuthRedirect=lazyRoute(()=>import('./auth-routes').then(m=>({default:m.LegacyAuthRedirect})));
const WelcomeGuide=lazyRoute(()=>import('./phone-account').then(m=>({default:m.WelcomeGuide})));
const PhoneAccountHome=lazyRoute(()=>import('./phone-account').then(m=>({default:m.PhoneAccountHome})));
const ProcurementApp=lazyRoute(()=>import('./procurement').then(m=>({default:m.ProcurementApp})));
const AdminConsole=lazyRoute(()=>import('./admin').then(m=>({default:m.AdminConsole})));
const MerchantWorkspace=lazyRoute(()=>import('./merchant-console').then(m=>({default:m.MerchantWorkspace})));
const Merchant=lazyRoute(()=>import('./auth-routes').then(m=>({default:m.Merchant})));
const MerchantLoginPage=lazyRoute(()=>import('./auth-routes').then(m=>({default:m.MerchantLoginPage})));
const MerchantEntryFooter=lazyRoute(()=>import('./auth-routes').then(m=>({default:m.MerchantEntryFooter})));

export function App(){return <HallProvider><Application/></HallProvider>}
function Application(){
 const runtime=useRuntimeConfig();useEffect(()=>{void loadRuntime()},[]);
 const {hall,href}=useHall();
 useAnalyticsNavigation();
 const [lang, setLang] = useState<Lang>(() => localStorage.getItem('zqzt.language') === 'en' ? 'en' : 'zh');
 const menuButton=useRef<HTMLButtonElement>(null);const platformHeader=useRef<HTMLElement>(null);
 const [signed,setSigned]=useState(!!token('merchant'));const [menu, setMenu] = useState(false); const location = useLocation();const [merchantSigned,setMerchantSigned]=useState(!!token('merchant')&&sessionStorage.getItem('zqzt.merchant.role')!=='visitor');useEffect(()=>{const sync=()=>{setMerchantSigned(!!token('merchant')&&sessionStorage.getItem('zqzt.merchant.role')!=='visitor');setSigned(!!token('merchant'))};window.addEventListener('zqzt.merchant.changed',sync);return()=>window.removeEventListener('zqzt.merchant.changed',sync)},[]);
 useEffect(()=>{if(!nativeWorkspace())return;window.__DLNJ_TOGGLE_LANGUAGE__=()=>setLang(l=>l==='zh'?'en':'zh');return()=>{delete window.__DLNJ_TOGGLE_LANGUAGE__}},[]);
 const tRaw = (key: string) => key==='procurementWorkspace'?(lang==='zh'?'采购中心':'Procurement center'):key==='procurementCenter'?(lang==='zh'?'采购询比价':'Procurement'):key==='procurementOpportunities'?(lang==='zh'?'商机中心':'Opportunities'):key==='brand'&&brandName(lang)?brandName(lang):(key==='hall'||key==='industrialHall')?(hall?(lang==='en'?hall.name_en:hall.name):''):seoMessages[lang][key] || (smsMessages[lang] as Record<string,string>)[key] || (serviceMessages[lang] as Record<string,string>)[key] || industryMapMessages[lang][key] || settingsMessages[lang][key] || companyEditorMessages[lang][key] || productMessages[lang][key] || v2Messages[lang][key] || operationsMessages[lang][key] || merchantMessages[lang][key] || uiMessages[lang][key] || resourceMessages[lang][key] || experienceMessages[lang][key] || showroomMessages[lang][key] || adminMessages[lang][key] || messages[lang][key as MessageKey] || messages[lang].internal_error;
 const t=(key:string)=>{let text=tRaw(key);const up=runtimeConfig().uploads;if(key==='resourceLimit')text=text.replace('200 MiB',up.video_mib+' MiB').replace('50 MiB',up.pdf_mib+' MiB');return text.replace(/10\s*(?:MiB|MB)/g,up.image_mib+' MiB')};
 useEffect(() => {localStorage.setItem('zqzt.language', lang); document.documentElement.lang = lang === 'zh' ? 'zh-CN' : 'en'; }, [lang,location.pathname]);
 useEffect(() => {setMenu(false);if(/^\/(?:v|vendors)\//.test(location.pathname))return;window.scrollTo(0,0);}, [location.pathname,location.search]);
 useEffect(()=>{if(!menu)return;const close=(e:KeyboardEvent)=>{if(e.key==='Escape'){setMenu(false);menuButton.current?.focus()}};const outside=(e:PointerEvent)=>{if(!platformHeader.current?.contains(e.target as Node))setMenu(false)};document.addEventListener('keydown',close);document.addEventListener('pointerdown',outside);return()=>{document.removeEventListener('keydown',close);document.removeEventListener('pointerdown',outside)}},[menu]);
 const hallName=lang==='en'?hall?.name_en:hall?.name;
 const browseHref=(path:string)=>{if(!location.pathname.startsWith('/browse'))return href(path);const p=new URLSearchParams(location.search);p.delete('page');return href(path+'?'+p)};
 const isDetail=/^\/(?:v|vendors|products)\//.test(location.pathname);
 const browseTitle=location.pathname==='/browse'?t('navBrowse'):location.pathname==='/browse/products'?t('navProducts'):location.pathname==='/regions'?t('imTitle'):'';
 const publicSEO=publicPageSEO(location.pathname,t('brand'),hall,lang);
 usePageMetadata(publicSEO?.title||(isDetail||/^\/(?:buyer|procurement)(?:\/|$)/.test(location.pathname)?'':(browseTitle?browseTitle+' | ':'')+t('brand')+' · '+(hallName||'')),publicSEO?.description||(hall?.status==='preparing'?(lang==='en'?hall.description_en:hall.description)||t('preparingDescription'):location.pathname.startsWith('/browse')?t(location.pathname.includes('/products')?'productDescription':'manufacturerDescription'):(lang==='en'?hall?.description_en:hall?.description)||t('v2Intro')),href(location.pathname),false,0,publicSEO?.keywords);
 if(!runtime.capabilities?.procurement&&/^\/(?:buyer|procurement|merchant\/opportunities|admin\/procurement)(?:\/|$)/.test(location.pathname))return <Language.Provider value={{lang,t,toggle:()=>setLang(l=>l==='zh'?'en':'zh')}}><ProcurementPaused/></Language.Provider>;
 if(location.pathname==='/buyer/opportunities'||location.pathname.startsWith('/buyer/opportunities/'))return <Navigate to={location.pathname.replace('/buyer/opportunities','/merchant/opportunities')+location.search+location.hash} replace/>;
 if(/^\/(?:buyer|procurement)(?:\/|$)/.test(location.pathname))return <Language.Provider value={{lang,t,toggle:()=>setLang(l=>l==='zh'?'en':'zh')}}><ConfirmationProvider><ProcurementApp/></ConfirmationProvider></Language.Provider>;
 if(location.pathname==='/admin'||location.pathname.startsWith('/admin/'))return <Language.Provider value={{lang,t,toggle:()=>setLang(l=>l==='zh'?'en':'zh')}}><ConfirmationProvider><AdminConsole/></ConfirmationProvider></Language.Provider>;
 if((location.pathname==='/merchant'||location.pathname.startsWith('/merchant/'))&&location.pathname!=='/merchant/login')return <Language.Provider value={{lang,t,toggle:()=>setLang(l=>l==='zh'?'en':'zh')}}><ConfirmationProvider><MerchantWorkspace/></ConfirmationProvider></Language.Provider>;
 if(/^\/(?:v|vendors)\//.test(location.pathname))return <Language.Provider value={{lang,t,toggle:()=>setLang(l=>l==='zh'?'en':'zh')}}><ConfirmationProvider><Routes><Route path="/v/:slug" element={<Showroom/>}/><Route path="/vendors/:id" element={<Showroom/>}/></Routes></ConfirmationProvider></Language.Provider>;
 return <Language.Provider value={{lang,t,toggle: () => setLang(l => l === 'zh' ? 'en' : 'zh')}}><ConfirmationProvider><div className={'public-web'+(location.pathname==='/regions'?' factory-map-shell':['/join','/merchant/login'].includes(location.pathname)?' merchant-entry-shell':'')}><header className="header" ref={platformHeader}><div className="container header-inner"><Link to="/" className="brand"><span className="brand-mark"><Tractor size={25}/></span><span><b>{t('brand')}</b></span></Link><HallSwitcher/><button ref={menuButton} aria-controls="public-navigation" className="menu-button icon-button" onClick={() => setMenu(!menu)} aria-label={t(menu?'close':'openNavigation')} aria-expanded={menu}>{menu?<X/>:<Menu/>}</button><nav id="public-navigation" className={menu ? 'open' : ''}><NavLink to="/" end>{t('navHome')}</NavLink><NavLink to={browseHref("/browse")} end>{t('navBrowse')}</NavLink><NavLink to={browseHref("/browse/products")}>{t('navProducts')}</NavLink><NavLink to={href('/regions')}>{t('imTitle')}</NavLink><ProcurementNav/>{signed?<>{merchantSigned&&<NavLink to="/merchant">{t('merchantWorkspace')}</NavLink>}<NavLink to={href('/account')}>{lang==='zh'?'我的账号':'My account'}</NavLink></>:<><NavLink to={href('/join')}>{lang==='zh'?'免费注册':'Register free'}</NavLink><NavLink to={href('/merchant/login')}>{lang==='zh'?'登录':'Sign in'}</NavLink></>}<NavLink to={href("/app")}>{t('downloadApp')}</NavLink><button className="language-button" onClick={() => setLang(l => l === 'zh' ? 'en' : 'zh')}><Globe2 size={16}/>{lang === 'zh' ? 'English' : '中文'}</button></nav></div></header><WelcomeGuide/><main>{location.pathname.startsWith('/services')||location.pathname==='/service-requests'?<Routes><Route path="/services" element={<ServicesPage/>}/><Route path="/services/:key" element={<ServicesPage/>}/><Route path="/service-requests" element={<ServiceRequestPage/>}/></Routes>:<HallGate><ExhibitionBreadcrumb/>{hall?.status==='preparing'&&!isDetail?<PreparingHall/>:<Routes><Route path="/" element={<ExhibitionHome/>}/><Route path="/browse" element={<BrowsePage/>}/><Route path="/regions" element={<IndustryMapPage/>}/><Route path="/browse/products" element={<BrowsePage kind="product"/>}/><Route path="/vendors/:id" element={<Detail kind="vendor"/>}/><Route path="/products/:id" element={<Detail kind="product"/>}/><Route path="/app" element={<AppDownload/>}/><Route path="/login" element={<LegacyAuthRedirect/>}/><Route path="/register" element={<LegacyAuthRedirect register/>}/><Route path="/account" element={<PhoneAccountHome/>}/><Route path="/join" element={<Merchant/>}/><Route path="/merchant/login" element={<MerchantLoginPage/>}/><Route path="*" element={<div className="container section"><ErrorState code="not_found"/></div>}/></Routes>}</HallGate>}</main>{['/join','/merchant/login'].includes(location.pathname)?<MerchantEntryFooter/>:location.pathname==='/regions'?<footer className="factory-map-footer"><span>{t('brand')} · {t('imTitle')}</span><span>{t('copyright')}</span><ICPRegistration/></footer>:<footer className="footer"><div className="container footer-top"><div><strong>{t('brand')} <span> / {hallName}</span></strong><p>{t('footer')}</p></div><div className="footer-download"><Link to="/app">{t('downloadApp')}</Link><Link to="/admin">{t('admin')} <ArrowUpRight size={15}/></Link></div></div><div className="container footer-bottom"><span>{t('footerNote')}</span><span>{t('copyright')}</span><span>{t('technicalSupport')}</span><ICPRegistration/></div></footer>}</div></ConfirmationProvider></Language.Provider>;
}
function Detail({kind}:{kind:'vendor'|'product'}){const {t,lang}=useLanguage();const {href,hall}=useHall();const location=useLocation();const origin=location.state?.returnTo;const returnTo=typeof origin==='string'&&/^\/(?:browse(?:\/products)?|vendors\/\d+|v\/[a-z0-9-]+)(?:\?|#|$)/.test(origin)?origin:kind==='product'?'/browse/products':'/browse';const returnLabel=returnTo.startsWith('/browse/products')?'navProducts':(returnTo.startsWith('/vendors/')||returnTo.startsWith('/v/'))?'backVendor':'navBrowse';const [pageNum,setPage]=useState(1);const {data,error,reload}=useLoad<{item:Item;vendor?:Item;products?:Page}>('/public'+location.pathname+'?page='+pageNum);useContentHall(data?.item);const detailSEO=data?productSEO(data.item.data,data.vendor?.data,lang,t('brand'),(lang==='en'?hall?.name_en:hall?.name)||''):undefined;usePageMetadata(detailSEO?.title||t(error||'loading')+' | '+t('brand'),detailSEO?.description||'',location.pathname,!!error||!data,data?.item.data.media_ids?.[0]||0,detailSEO?.keywords);useEffect(()=>setPage(1),[location.pathname]);return <div className="container section public-product-detail"><Link className="text-link back" to={href(returnTo)}><ChevronLeft size={16}/>{t(returnLabel)}</Link>{error?<ErrorState code={error} retry={reload}/>:!data?<Loading/>:<><div className="detail-layout"><div><ContentInfo item={data.item}/>{data.vendor&&<Link className="vendor-reference" to={href(vendorPath(data.vendor))} state={{returnTo}}><Building2/><div><small>{t('vendor')}</small><h3>{display(data.vendor.data,lang,'name')}</h3></div><ArrowRight/></Link>}</div><aside><Contact data={kind==='vendor'?data.item.data:data.vendor!.data}/>{kind==='product'&&<InquiryButton productID={data.item.id}/>}</aside></div>{kind==='product'&&<section className="section"><h2>{t('shortVideos')} / {t('electronicCatalogs')}</h2><ResourceList productID={data.item.id}/></section>}{data.products&&<section className="section"><SectionTitle title="products"/>{data.products.items.length?<div className="cards">{data.products.items.map(p=><ItemCard key={p.id} item={p} returnTo={location.pathname+location.search}/>)}</div>:<p className="muted">{t('emptyProducts')}</p>}<Pager page={data.products.page} size={data.products.page_size} total={data.products.total} onPage={setPage}/></section>}</>}</div>}

const rootElement=document.getElementById('root');
if(rootElement)createRoot(rootElement).render(<React.StrictMode><BrowserRouter><App/></BrowserRouter></React.StrictMode>);
