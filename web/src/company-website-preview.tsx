import type {ReactNode} from 'react';
import {MapPin} from 'lucide-react';
import {useLanguage,display,Contact} from './ui';
import {VendorCover,VendorLogo} from './vendor-brand';
import {CompanyOverview,CompanyProduction,CompanyCredentials,AnnotatedGallery,companyGallery} from './company-portal';
import {ICPRegistration} from './site-common';
import type {Data} from './types';

export function CompanyWebsiteHero({data,role,actions}:{data:Data;role?:'merchant'|'admin';actions?:ReactNode}){
 const {lang}=useLanguage();const name=display(data,lang,'name');
 return <section className="sr-home-hero"><div className="container sr-hero-grid"><div className="sr-hero-copy"><div className="sr-logo"><VendorLogo data={data} role={role} alt={name}/></div><h1>{name}</h1>{data.main_products&&<p className="sr-main-products">{data.main_products}</p>}{data.region_name&&<p className="sr-region"><MapPin size={15}/>{data.region_name}</p>}{actions&&<div className="showroom-actions">{actions}</div>}</div><div className="sr-hero-photo"><VendorCover data={data} role={role} alt={name} priority/></div></div></section>;
}
export function CompanyWebsitePreview({data}:{data:Data}){
 const {lang,t}=useLanguage();const gallery=companyGallery(data);
 return <div className="showroom showroom-v2 company-website-preview"><p className="notice">{lang==='zh'?'官网草稿预览，尚未发布。审核通过后使用此模板公开展示。':'Website draft preview. Publication follows approval.'}</p><CompanyWebsiteHero data={data} role="merchant"/><div className="container showroom-section"><CompanyOverview data={data}/><CompanyProduction data={data}/><CompanyCredentials data={data} role="merchant"/>{gallery.length>0&&<section className="portal-section"><h2>{t('showroomGallery')}</h2><AnnotatedGallery items={gallery} role="merchant"/></section>}<section className="portal-section"><h2>{t('showroomContact')}</h2><Contact data={data}/></section><ICPRegistration/></div></div>;
}
