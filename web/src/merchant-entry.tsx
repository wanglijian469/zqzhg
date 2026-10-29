import {ICPRegistration} from './site-common';
import {Link,useLocation} from 'react-router-dom';
import {ArrowRight,Building2,Check,ClipboardList,Package,Share2} from 'lucide-react';
import {MerchantAuth} from './merchant-auth';
import {useLanguage} from './ui';
import {useHall} from './halls';
import './merchant-entry.css';

export function merchantReturnPath(search:string){
 const candidate=new URLSearchParams(search).get('next');
 if(!candidate||!/^\/merchant(?:\/|\?|$)/.test(candidate)||/[\\\u0000-\u0020]/.test(candidate))return '/merchant';
 try{const url=new URL(candidate,'https://merchant.invalid');if(url.origin!=='https://merchant.invalid'||!/^\/merchant(?:\/|$)/.test(url.pathname)||url.pathname.startsWith('/merchant/login'))return '/merchant';return url.pathname+url.search+url.hash}catch{return '/merchant'}
}

export function MerchantEntryPage({kind,onLogin}:{kind:'register'|'login';onLogin:(registered?:boolean)=>void}){
 const {lang}=useLanguage(),{href}=useHall(),location=useLocation();const zh=lang==='zh',register=kind==='register',next=merchantReturnPath(location.search);
 const destination=(path:string)=>href(path+(next!=='/merchant'?'?'+new URLSearchParams({next}):''));
 const username=typeof location.state?.entryLoginName==='string'?location.state.entryLoginName:'';
 const steps=zh?[['注册普通账号','手机验证或设置用户名密码'],['完善企业资料','可选择申请企业展厅'],['提交审核后展示','审核通过后公开企业展厅']]:[['Create an account','Verify your mobile or choose a username'],['Complete your profile','Add company information and images'],['Submit for review','Your showroom is published after approval']];
 const features=zh?[['企业展厅','维护企业资料与展示内容'],['产品管理','管理本厂产品与审核状态'],['展厅推广','分享企业展厅与产品']]:[['Company showroom','Maintain your company profile'],['Product management','Manage products and review status'],['Showroom promotion','Share your showroom and products']];
 return <div className={'merchant-entry '+(register?'is-registration':'is-sign-in')}>
  <section className="merchant-entry-intro" aria-label={zh?'厂商服务介绍':'Manufacturer services'}>
   <div className="merchant-entry-eyebrow"><span><Building2 size={19}/></span>{zh?'中企展馆 · 免费展示服务':'Manufacturer services'}</div>
   <h1>{register?(zh?'免费注册':'Register free'):(zh?'登录中企展馆':'Sign in to the exhibition')}</h1>
   <p className="merchant-entry-lead">{register?(zh?'注册后浏览展馆，按需申请企业展厅':'Register to explore or apply for a showroom'):(zh?'登录后继续浏览产品或管理企业':'Your workspace for your showroom and products')}</p>
   {register?<ol className="merchant-entry-steps">{steps.map(([title,description],index)=><li key={title}><span className="merchant-entry-step-number">{index+1}</span><div><strong>{title}</strong><small>{description}</small></div></li>)}</ol>:<div className="merchant-entry-features">{features.map(([title,description],index)=>{const Icon=[Building2,Package,Share2][index];return <div key={title}><span><Icon size={20}/></span><div><strong>{title}</strong><small>{description}</small></div></div>})}</div>}
   <p className="merchant-entry-note"><Check size={15}/>{register?(zh?'企业资料提交后需审核，通过后对外展示。':'Company information is reviewed before publication.'):(zh?'原有账号和密码可继续使用，无需重复注册。':'Use your existing company account without registering again.')}</p>
  </section>
  <section className="merchant-entry-card" aria-labelledby="merchant-entry-card-title">
   <div className="merchant-entry-card-heading"><span><ClipboardList size={17}/>{register?(zh?'新厂商':'New manufacturer'):(zh?'欢迎回来':'Welcome back')}</span><h2 id="merchant-entry-card-title">{register?(zh?'注册中企展馆':'Register your account'):(zh?'登录中企展馆':'Sign in to the exhibition')}</h2><p>{register?(zh?'注册成功后自动登录，无需立即填写企业资料。':'Registration signs you in immediately. Company information is optional.'):(zh?'使用手机号或用户名和密码登录。':'Use your mobile number or username and password.')}</p></div>
   <MerchantAuth key={kind} defaultMode={register?'register':'account'} companyName={username} presentation={register?'entry-register':'entry-login'} entryLinks={{account:destination('/merchant/login'),register:destination('/join')}} onLogin={onLogin}/>
  </section>
  <Link className="merchant-entry-back" to={href('/')}><ArrowRight size={14}/>{zh?'返回展馆首页':'Back to the exhibition'}</Link>
 </div>;
}

export function MerchantEntryFooter(){const {t}=useLanguage(),{href}=useHall();return <footer className="merchant-entry-footer"><div><span>{t('footerNote')}</span><Link to={href('/app')}>{t('downloadApp')}</Link></div><div><span>{t('copyright')}</span><span>{t('technicalSupport')}</span><ICPRegistration/></div></footer>}
