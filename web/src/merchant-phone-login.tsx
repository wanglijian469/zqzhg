import {useEffect,useId,useRef,useState} from 'react';
import {api,APIError,body} from './api';
import {ErrorState,useLanguage} from './main';

export function PhoneLogin({onLogin,onBusy}:{onLogin:()=>void;onBusy?:(busy:boolean)=>void}){
 const {t,lang}=useLanguage(),id=useId(),zh=lang==='zh';const [phone,setPhone]=useState(''),[code,setCode]=useState(''),[dev,setDev]=useState(''),[error,setError]=useState('');
 const [busy,setBusy]=useState(false),[seconds,setSeconds]=useState(0),[fields,setFields]=useState<Record<string,string>>({});const inFlight=useRef(false),focusPending=useRef(false),form=useRef<HTMLFormElement>(null);
 useEffect(()=>{onBusy?.(busy)},[busy,onBusy]);
 useEffect(()=>{if(!seconds)return;const timer=setTimeout(()=>setSeconds(n=>n-1),1000);return()=>clearTimeout(timer)},[seconds]);
 useEffect(()=>{if(!busy&&focusPending.current){focusPending.current=false;const field=fields.phone?'phone':fields.code?'code':'';if(field)(form.current?.elements.namedItem(field) as HTMLElement)?.focus()}},[fields,busy]);
 const run=async(send:boolean)=>{
  if(inFlight.current||send&&seconds>0)return;
  const invalid:Record<string,string>={};if(!phone.trim())invalid.phone=zh?'请填写手机号。':'Enter your phone number.';if(!send&&!code.trim())invalid.code=zh?'请填写验证码。':'Enter your verification code.';
  focusPending.current=Object.keys(invalid).length>0;setFields(invalid);setError('');if(Object.keys(invalid).length){(form.current?.elements.namedItem(invalid.phone?'phone':'code') as HTMLElement)?.focus();return}
  inFlight.current=true;setBusy(true);
  try{
   if(send){const response=await api<{development_code?:string}>('/auth/sms',{method:'POST',body:body({phone})});setDev(response.development_code||'');setSeconds(60)}
   else{const response=await api<{token:string;account:{id:number}}>('/auth/phone',{method:'POST',body:body({phone,code})});sessionStorage.setItem('zqzt.merchant',response.token);sessionStorage.setItem('zqzt.merchant.id',String(response.account.id));window.dispatchEvent(new Event('zqzt.merchant.changed'));onLogin()}
  }catch(cause){const failure=cause as APIError;const field=failure.code==='invalid_phone'?'phone':failure.code==='invalid_code'?'code':'';if(field){focusPending.current=true;setFields({[field]:t(failure.code)})}else setError(failure.code||'internal_error')}
  finally{inFlight.current=false;setBusy(false)}
 };
 return <form ref={form} className="auth-card merchant-phone-auth" noValidate aria-busy={busy} onSubmit={event=>{event.preventDefault();event.stopPropagation();void run(false)}}>
  <fieldset disabled={busy}>
   <div className="merchant-auth-field"><label htmlFor={id+'-phone'}>{zh?'手机号':'Mobile number'}</label><input id={id+'-phone'} name="phone" value={phone} onChange={event=>{setPhone(event.target.value);setCode('');setDev('');setFields(old=>({...old,phone:''}))}} inputMode="tel" autoComplete="tel" aria-invalid={!!fields.phone} aria-describedby={fields.phone?id+'-phone-error':undefined}/>{fields.phone&&<small className="field-error" id={id+'-phone-error'} role="alert">{fields.phone}</small>}</div>
   <div className="merchant-auth-field"><label htmlFor={id+'-code'}>{t('code')}</label><div className="merchant-code-control"><input id={id+'-code'} name="code" value={code} onChange={event=>{setCode(event.target.value);setFields(old=>({...old,code:''}))}} inputMode="numeric" autoComplete="one-time-code" aria-invalid={!!fields.code} aria-describedby={fields.code?id+'-code-error':undefined}/><button className="button secondary" type="button" disabled={busy||seconds>0} onClick={()=>void run(true)}>{seconds?(zh?seconds+'秒后重发':seconds+'s'):t('sendCode')}</button></div>{fields.code&&<small className="field-error" id={id+'-code-error'} role="alert">{fields.code}</small>}</div>
  </fieldset>
  {dev&&<p className="notice">{t('devCode')}: <b>{dev}</b></p>}{error&&<ErrorState code={error}/>}<button type="submit" className="button merchant-auth-submit" disabled={busy}>{busy?t('loading'):t('verify')}</button>
 </form>;
}
