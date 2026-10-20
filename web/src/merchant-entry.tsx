import {Link,useLocation} from 'react-router-dom';
import {ArrowRight,Building2,Check,ClipboardList,Package,Share2} from 'lucide-react';
import {MerchantAuth} from './merchant-auth';
import {useLanguage} from './main';
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
 const steps=zh?[['创建公司账号','设置账号密码，无需短信注册'],['完善企业资料','到工作台填写资料、上传图片'],['提交审核后展示','审核通过后公开企业展厅']]:[['Create an account','Set a company login and password'],['Complete your profile','Add company information and images'],['Submit for review','Your showroom is published after approval']];
 const features=zh?[['企业展厅','维护企业资料与展示内容'],['产品管理','管理本厂产品与审核状态'],['展厅推广','分享企业展厅与产品']]:[['Company showroom','Maintain your company profile'],['Product management','Manage products and review status'],['Showroom promotion','Share your showroom and products']];
 return <div className={'merchant-entry '+(register?'is-registration':'is-sign-in')}>
  <section className="merchant-entry-intro" aria-label={zh?'厂商服务介绍':'Manufacturer services'}>
   <div className="merchant-entry-eyebrow"><span><Building2 size={19}/></span>{zh?'中企展馆 · 厂商服务':'Manufacturer services'}</div>
   <h1>{register?(zh?'厂商入驻':'Join as a manufacturer'):(zh?'厂商登录':'Manufacturer sign-in')}</h1>
   <p className="merchant-entry-lead">{register?(zh?'创建公司账号，开启企业展厅':'Create your company account and showroom'):(zh?'一个工作台，管理企业展厅与产品':'Your workspace for your showroom and products')}</p>
   {register?<ol className="merchant-entry-steps">{steps.map(([title,description],index)=><li key={title}><span className="merchant-entry-step-number">{index+1}</span><div><strong>{title}</strong><small>{description}</small></div></li>)}</ol>:<div className="merchant-entry-features">{features.map(([title,description],index)=>{const Icon=[Building2,Package,Share2][index];return <div key={title}><span><Icon size={20}/></span><div><strong>{title}</strong><small>{description}</small></div></div>})}</div>}
   <p className="merchant-entry-note"><Check size={15}/>{register?(zh?'企业资料提交后需审核，通过后对外展示。':'Company information is reviewed before publication.'):(zh?'已有公司账号可直接登录，无需重复入驻。':'Use your existing company account without registering again.')}</p>
  </section>
  <section className="merchant-entry-card" aria-labelledby="merchant-entry-card-title">
   <div className="merchant-entry-card-heading"><span><ClipboardList size={17}/>{register?(zh?'新厂商':'New manufacturer'):(zh?'欢迎回来':'Welcome back')}</span><h2 id="merchant-entry-card-title">{register?(zh?'注册公司账号':'Create a company account'):(zh?'登录厂商工作台':'Sign in to your workspace')}</h2><p>{register?(zh?'设置账号密码，下一步完善企业资料。':'Set your account details, then complete your profile.'):(zh?'使用公司账号，继续管理你的企业。':'Use your company account to continue.')}</p></div>
   <MerchantAuth key={kind} defaultMode={register?'register':'account'} companyName={username} presentation={register?'entry-register':'entry-login'} entryLinks={{account:destination('/merchant/login'),register:destination('/join')}} onLogin={onLogin}/>
  </section>
  <Link className="merchant-entry-back" to={href('/')}><ArrowRight size={14}/>{zh?'返回展馆首页':'Back to the exhibition'}</Link>
 </div>;
}

export function MerchantEntryFooter(){const {t}=useLanguage(),{href}=useHall();return <footer className="merchant-entry-footer"><div><span>{t('footerNote')}</span><Link to={href('/app')}>{t('downloadApp')}</Link></div><div><span>{t('copyright')}</span><span>{t('technicalSupport')}</span></div></footer>}
