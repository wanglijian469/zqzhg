import {Link} from 'react-router-dom';
import {FilingLinks} from './site-common';
import {useLanguage} from './ui';
import './public-footer.css';

export function PublicFooter(){
 const {lang,t}=useLanguage(),zh=lang==='zh';
 const groups=[
  {id:'manufacturers',title:zh?'厂商服务':'Manufacturer services',links:[
   {to:'/join',label:zh?'免费入驻':'Join for free',featured:true},
   {to:'/guide',label:zh?'入驻使用指南':'Manufacturer guide (Chinese)'},
   {to:'/services',label:zh?'入驻与服务说明':'Admission and services'},
  ]},
  {id:'support',title:zh?'帮助与支持':'Help and support',links:[
   {to:'/service-requests',label:zh?'投诉与服务申请':'Complaints and service requests'},
   {to:'/app',label:zh?'下载 App':'Download App'},
   {to:'/merchant/login',label:zh?'企业登录':'Company sign-in'},
  ]},
 ];
 return <footer className="footer public-footer">
  <div className="container public-footer-columns">
   <div className="public-footer-intro"><Link className="public-footer-brand" to="/">{t('brand')}</Link><p>{t('footer')}</p></div>
   {groups.map(group=><nav key={group.id} className="public-footer-group" aria-labelledby={'public-footer-'+group.id}><h2 id={'public-footer-'+group.id}>{group.title}</h2><ul>{group.links.map(link=><li key={link.to}><Link className={'featured' in link&&link.featured?'public-footer-join':undefined} to={link.to}>{link.label}</Link></li>)}</ul></nav>)}
  </div>
  <div className="container public-footer-legal">
   <div className="public-footer-bottom"><div className="public-footer-registration"><span>{t('copyright')}</span><span className="public-footer-filings"><FilingLinks/></span></div><nav className="public-footer-legal-nav" aria-label={zh?'协议与管理入口':'Legal and administration links'}><Link to="/services/terms">{zh?'用户协议':'User agreement'}</Link><Link to="/services/privacy">{zh?'隐私政策':'Privacy policy'}</Link><Link to="/services/showroom-agreement">{zh?'企业展厅服务协议':'Showroom service agreement'}</Link><Link to="/admin" className="public-footer-admin">{zh?'管理后台':'Administration'}</Link></nav></div>
  </div>
 </footer>;
}
