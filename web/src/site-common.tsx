import {Link} from 'react-router-dom';
import {useLanguage} from './ui';
import {useRuntimeConfig} from './runtime-config';
import './police-registration.css';

export function PoliceRegistration(){
 const {site}=useRuntimeConfig();
 return site.police_number?<a className="police-registration" href={site.police_url||'https://beian.mps.gov.cn/'} target="_blank" rel="noopener noreferrer"><img src="/police-beian.png" width={36} height={40} alt="" aria-hidden="true"/>{site.police_number}</a>:null;
}

export function FilingLinks(){
 const config=useRuntimeConfig();
 return <><a className="icp-registration" href="https://beian.miit.gov.cn/" target="_blank" rel="noopener noreferrer">{config.site.icp_number||'冀ICP备2026047360号-2'}</a><PoliceRegistration/></>;
}
export function ICPRegistration(){
 const {lang}=useLanguage();
 return <span className="site-registrations"><FilingLinks/><Link to="/services">{lang==='zh'?'入驻与服务说明':'Admission and services'}</Link><Link to="/guide">{lang==='zh'?'入驻使用指南':'Manufacturer guide (Chinese)'}</Link><Link to="/service-requests">{lang==='zh'?'投诉与服务申请':'Complaints and requests'}</Link></span>;
}
export function ProcurementPaused(){
 const {lang}=useLanguage();
 return <main className="container section preparing-hall"><h1>{lang==='zh'?'功能暂时暂停':'Service temporarily paused'}</h1><p>{lang==='zh'?'请通过厂商官网查看产品，并使用电话或微信联系厂商。':'Browse manufacturer websites and contact them by phone or WeChat.'}</p><Link className="button" to="/">{lang==='zh'?'返回展馆总页':'Back to the exhibition'}</Link><ICPRegistration/></main>;
}
