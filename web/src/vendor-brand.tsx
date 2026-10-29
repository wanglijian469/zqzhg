import {Building2} from 'lucide-react';
import {MediaImage,useLanguage} from './ui';
import type {Data} from './types';

export const vendorCoverID=(data:Data)=>data.company_profile?.cover_media_id||data.media_ids?.[0]||0;
export function IndustrialCover(){const {t}=useLanguage();return <div className="company-cover-placeholder" role="img" aria-label={t('companyCoverPlaceholder')}><Building2 size={52} strokeWidth={1.2}/><span>{t('showroomIdentity')}</span></div>}
export function VendorCover({data,role,alt='',priority=false}:{data:Data;role?:'admin'|'merchant';alt?:string;priority?:boolean}){const id=vendorCoverID(data);return id?<MediaImage key={id} id={id} preset={priority?'hero':'card'} role={role} fit="cover" alt={alt} priority={priority} fallback={<IndustrialCover/>}/>:<IndustrialCover/>}
export function VendorLogo({data,role,alt=''}:{data:Data;role?:'admin'|'merchant';alt?:string}){const id=data.company_profile?.logo_media_id;return id?<MediaImage key={id} id={id} preset="thumbnail" role={role} fit="contain" alt={alt} fallback={<Building2 size={26}/>}/>:<Building2 size={26}/>}
