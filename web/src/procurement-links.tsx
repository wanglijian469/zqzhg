import {useEffect,useState} from 'react';
import {Link,NavLink} from 'react-router-dom';
import {useLanguage} from './ui';
import {token} from './api';
import {runtimeConfig} from './runtime-config';
import {useMobileWorkspace} from './mobile-workspace';
function useWords(){const {lang}=useLanguage();return {L:(zh:string,en:string)=>lang==='en'?en:zh}}
export function InquiryButton({productID}:{productID:number}){const {L}=useWords();if(runtimeConfig().capabilities?.procurement===false)return null;return <Link className="button proc-inquire" to={'/buyer/new?product='+productID}>{L('立即询价','Request a quote')}</Link>}
export function ProcurementNav(){const {L}=useWords();const mobile=useMobileWorkspace();const [signed,setSigned]=useState(!!token('merchant'));useEffect(()=>{const changed=()=>setSigned(!!token('merchant'));window.addEventListener('zqzt.merchant.changed',changed);return()=>window.removeEventListener('zqzt.merchant.changed',changed)},[]);if(runtimeConfig().capabilities?.procurement===false)return null;return <NavLink to={signed&&!mobile?'/buyer':'/procurement'}>{L('采购中心','Procurement center')}</NavLink>}
