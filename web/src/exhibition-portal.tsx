import {Link,useLocation} from 'react-router-dom';
import {Factory,ArrowRight,Tractor,Cable} from 'lucide-react';
import {useHall,hallHref} from './halls';
import {useLanguage,ErrorState,Loading} from './ui';
import './exhibition-portal.css';

export function ExhibitionPortal(){
 const {halls,error,loading,reload}=useHall();const {lang,t}=useLanguage();
 return <div className="container section exhibition-portal"><header><p className="eyebrow">ZQZG</p><h1>{t('brand')}</h1><p>{lang==='zh'?'选择行业展厅，走进厂商展台，了解企业与产品。':'Choose an industry hall to explore manufacturers and products.'}</p></header><h2>{lang==='zh'?'行业展厅':'Industry halls'}</h2>{error?<ErrorState code={error} retry={reload}/>:loading?<Loading/>:<div className="industry-hall-grid">{halls.map(h=>{const Icon=h.slug==='agriculture'?Tractor:h.slug==='cable'?Cable:Factory;return <Link className="industry-hall-card" key={h.id} to={hallHref('/',h.slug)}><Icon size={40}/><h3>{lang==='en'?(h.name_en||h.name):h.name}</h3><span className="status-pill">{lang==='zh'?(h.status==='open'?'开放中':'筹备中'):(h.status==='open'?'Open':'Preparing')}</span><p>{(lang==='en'?h.description_en:h.description)||(lang==='zh'?'浏览行业厂商与产品':'Discover manufacturers and products')}</p><span>{lang==='zh'?'进入展厅':'Enter hall'} <ArrowRight size={16}/></span></Link>})}</div>}<p className="portal-join"><Link className="button" to="/join">{lang==='zh'?'厂商入驻，生成企业官网':'Join and create your company website'}</Link></p></div>;
}
export function ExhibitionBreadcrumb(){
 const {hall}=useHall();const {lang,t}=useLanguage();const location=useLocation();
 if(!hall||!['/browse','/browse/products'].includes(location.pathname))return null;
 return <nav className="container exhibition-breadcrumb" aria-label={lang==='zh'?'当前位置':'Breadcrumb'}><Link to="/">{t('brand')}</Link><span>/</span><Link to={hallHref('/',hall.slug)}>{lang==='en'?(hall.name_en||hall.name):hall.name}</Link>{location.pathname!=='/'&&<><span>/</span><span>{t(location.pathname==='/browse'?'navBrowse':location.pathname==='/regions'?'imTitle':'navProducts')}</span></>}</nav>;
}
