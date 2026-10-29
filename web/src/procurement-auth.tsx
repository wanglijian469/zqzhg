import {useContext,useEffect,useRef,useState} from 'react';
import {Link} from 'react-router-dom';
import {api,body,token} from './api';
import {useLanguage,Confirmation} from './ui';
import {type BusinessAccount,procError,procurementErrorText} from './procurement-api';

import {storeBusinessSession} from './business-session';
export {storeBusinessSession} from './business-session';
export function useBusinessAccount(){
 const [account,setAccount]=useState<BusinessAccount>();const [error,setError]=useState('');const [generation,setGeneration]=useState(0);
 useEffect(()=>{const update=()=>{setAccount(undefined);setError('');setGeneration(v=>v+1)};window.addEventListener('zqzt.merchant.changed',update);return()=>window.removeEventListener('zqzt.merchant.changed',update)},[]);
 useEffect(()=>{let active=true;if(!token('merchant'))return;api<BusinessAccount>('/auth/me',{},'merchant').then(v=>{if(active){setAccount(v);sessionStorage.setItem('zqzt.merchant.role',v.role)}}).catch(e=>{if(active)setError(procError(e))});return()=>{active=false}},[generation]);
 return {account,error,reload:()=>setGeneration(v=>v+1)};
}
export {ProcurementLogin} from './procurement-login';

export function ProcurementPhoneBinding({onBound}:{onBound:()=>void}){
 const {lang,t}=useLanguage(),en=lang==='en';const [phone,setPhone]=useState(''),[code,setCode]=useState(''),[dev,setDev]=useState(''),[error,setError]=useState(''),[busy,setBusy]=useState(false),[seconds,setSeconds]=useState(0);
 useEffect(()=>{if(seconds>0){const timer=setTimeout(()=>setSeconds(v=>v-1),1000);return()=>clearTimeout(timer)}},[seconds]);
 const run=async(send:boolean)=>{setBusy(true);setError('');try{if(send){const r=await api<{development_code?:string}>('/auth/me/phone/code',{method:'POST',body:body({phone})},'merchant');setDev(r.development_code||'');setSeconds(60)}else{const r=await api<{token:string;account:BusinessAccount}>('/auth/me/phone',{method:'PUT',body:body({phone,code})},'merchant');storeBusinessSession(r);onBound()}}catch(e){setError(procError(e))}finally{setBusy(false)}};
 return <section className="proc-card"><h2>{en?'Verify your mobile number':'验证手机号'}</h2><p>{en?'Verify this account before sending inquiries or quotations.':'发起采购或提交报价前，请为当前账号验证手机号。'}</p><form onSubmit={e=>{e.preventDefault();void run(false)}}><fieldset disabled={busy}><label>{en?'Mobile number':'手机号'}<input required inputMode="tel" value={phone} onChange={e=>setPhone(e.target.value)}/></label><label>{en?'Verification code':'验证码'}<div className="proc-inline"><input required inputMode="numeric" value={code} onChange={e=>setCode(e.target.value)}/><button type="button" className="button secondary" disabled={seconds>0} onClick={()=>void run(true)}>{seconds?seconds+'s':en?'Send code':'获取验证码'}</button></div></label>{dev&&<p>{dev}</p>}<button className="button">{en?'Verify and continue':'验证并继续'}</button></fieldset></form>{error&&<p role="alert" className="notice error">{procurementErrorText(error,en,t)}</p>}</section>
}
export function BecomeProcurementSupplier(){
 const {lang,t}=useLanguage(),en=lang==='en';const confirm=useContext(Confirmation);const [error,setError]=useState(''),[busy,setBusy]=useState(false);
 return <section><h3>{en?'Also supply products?':'也有产品供货？'}</h3><button className="button secondary" disabled={busy} onClick={async()=>{if(!await confirm(en?'Keep your buyer profile and use this same account to register a supplier showroom.':'保留当前采购资料，使用同一账号入驻厂商展厅。',en?'Continue with this account':'使用本账号继续',{tone:'primary'}))return;setBusy(true);setError('');try{const r=await api<{token:string;account:BusinessAccount}>('/auth/me/merchant-upgrade',{method:'POST',body:body({confirmed:true})},'merchant');storeBusinessSession(r);window.location.assign('/merchant/booth/edit')}catch(e){setError(procError(e));setBusy(false)}}}>{en?'Register as a supplier with this account':'使用同一账号入驻厂商'}</button>{error&&<p className="notice error">{procurementErrorText(error,en,t)}</p>}</section>
}
