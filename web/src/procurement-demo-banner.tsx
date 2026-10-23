import {useNavigate} from 'react-router-dom';
import {useEffect,useState} from 'react';
import {api,token} from './api';
import {useLanguage} from './main';
import {useRuntimeConfig} from './runtime-config';

export function ProcurementDemoBanner({supplier=false}:{supplier?:boolean}){
 const cfg=useRuntimeConfig(),{lang}=useLanguage(),navigate=useNavigate();const [busy,setBusy]=useState(false),[session,setSession]=useState(token('merchant'));useEffect(()=>{const changed=()=>setSession(token('merchant'));window.addEventListener('zqzt.merchant.changed',changed);return()=>window.removeEventListener('zqzt.merchant.changed',changed)},[]);if(!cfg.capabilities?.procurement_demo)return null;const en=lang==='en';
 return <aside className="proc-demo-banner"><span>{en?'Isolated local demo · fictional data · no real SMS':'本机隔离演示 · 虚构资料 · 不发送真实短信'}</span>{session&&<button className="button secondary" disabled={busy} onClick={async()=>{setBusy(true);try{await api('/auth/logout',{method:'POST',body:'{}'},'merchant')}catch{/* Clear this browser session even if the server session expired. */}finally{sessionStorage.removeItem('zqzt.merchant');sessionStorage.removeItem('zqzt.merchant.id');sessionStorage.removeItem('zqzt.merchant.role');window.dispatchEvent(new Event('zqzt.merchant.changed'));navigate(supplier?'/merchant/opportunities':'/buyer');setBusy(false)}}}>{en?'Switch demo account':'切换体验账号'}</button>}</aside>;
}
