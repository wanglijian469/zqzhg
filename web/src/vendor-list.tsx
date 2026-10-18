import {usePhoneViewport} from './public-mobile';
import {ArrowUpRight,MapPin,Phone,ImageIcon} from 'lucide-react';
import {Link} from 'react-router-dom';
import {display,MediaImage,useLanguage} from './main';
import {VendorCover,VendorLogo} from './vendor-brand';
import {vendorPath} from './showroom-share';
import {useHall} from './halls';
import type {Item} from './types';

export function VendorBrandCard({item,returnTo='',filtered=false,preview=false,role}:{item:Pick<Item,'id'|'data'|'showroom_slug'|'matching_products'>;returnTo?:string;filtered?:boolean;preview?:boolean;role?:'admin'|'merchant'}){
 const {t,lang}=useLanguage();const {href}=useHall();const data=item.data,name=display(data,lang,'name')||t('company');const path=href(vendorPath(item));const products=(item.matching_products||[]).slice(0,3);const state={returnTo};
 const phone=usePhoneViewport()&&!preview;
 const cover=<VendorCover data={data} role={role} alt={name}/>;
 const actions=<div className="vendor-brand-actions">{preview?<><span className="button">{t('enterShowroom')}<ArrowUpRight size={16}/></span><span className="text-link"><Phone size={15}/>{t('contactManufacturer')}</span></>:<><Link className="button" to={path} state={state}>{t('enterShowroom')}<ArrowUpRight size={16}/></Link><Link className="text-link" to={href(vendorPath(item)+'?tab=contact#showroom-tabs')} state={state}><Phone size={15}/>{t('contactManufacturer')}</Link></>}</div>;
 return <article className={'vendor-brand-card'+(preview?' is-preview':'')+(!products.length?' no-products':'')}>
  {preview?<div className="vendor-brand-photo">{cover}</div>:<Link className="vendor-brand-photo" to={path} state={state} aria-label={t('enterShowroom')+' '+name}>{cover}<span>{t('showroomIdentity')}<ArrowUpRight size={16}/></span></Link>}
  <div className="vendor-brand-info"><div className="vendor-brand-heading"><div className="vendor-brand-logo"><VendorLogo data={data} role={role} alt={name+' Logo'}/></div><div><span className="eyebrow">{t('showroomIdentity')}</span>{preview?<h2>{name}</h2>:<Link to={path} state={state}><h2>{name}</h2></Link>}</div></div><p className="vendor-brand-main-products">{data.main_products||t('noMainProducts')}</p><p className="vendor-brand-region"><MapPin size={15}/>{data.region_name||t('regionNotProvided')}</p>{!phone&&actions}</div>
  {phone&&actions}
  {!preview&&products.length>0&&<div className="vendor-brand-products"><h3>{t(filtered?'matchedExhibits':'exhibitOverview')}</h3>{products.length?<div className="vendor-brand-product-grid">{products.map(product=><Link aria-label={display(product.data,lang,'name')} key={product.id} to={href('/products/'+product.id)} state={state}><div className="vendor-brand-product-image">{product.data.media_ids?.[0]?<MediaImage id={product.data.media_ids[0]} fit="contain" alt={display(product.data,lang,'name')}/>:<ImageIcon size={25}/>}</div><span>{display(product.data,lang,'name')}</span></Link>)}</div>:<p className="vendor-brand-empty-products">{t(filtered?'noMatchedExhibits':'noPublicExhibits')}</p>}</div>}
 </article>;
}
