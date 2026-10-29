import {SMSChallengeField,requireSMSProof,smsRequestID,type SMSProof} from './sms-challenge';
import {useEffect,useId,useRef,useState} from 'react';
import {APIError} from './api';
import {Link} from 'react-router-dom';
import {requestLoginCode,loginWithCode} from './phone-login-api';
import {storeBusinessSession} from './business-session';
import {useRuntimeConfig} from './runtime-config';
import {ErrorState,useLanguage} from './ui';

export function PhoneLogin({onLogin,onBusy}:{onLogin:()=>void;onBusy?:(busy:boolean)=>void}){
 const {t,lang}=useLanguage(),id=useId(),zh=lang==='zh',cfg=useRuntimeConfig();const [phone,setPhone]=useState(''),[code,setCode]=useState(''),[dev,setDev]=useState(''),[error,setError]=useState('');
 const [proof,setProof]=useState<SMSProof>({});const requestID=useRef('');
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
   if(send){requireSMSProof(cfg.capabilities?.sms_challenge===true,proof);requestID.current ||= smsRequestID();const response=await requestLoginCode(phone,proof,requestID.current);requestID.current='';setDev(response.development_code||'');setSeconds(response.retry_after||60)}
   else{const response=await loginWithCode(phone,code);storeBusinessSession(response);setCode('');setDev('');onLogin()}
  }catch(cause){const failure=cause as APIError;const field=failure.code==='invalid_phone'?'phone':['invalid_code','invalid_verification_code'].includes(failure.code)?'code':'';if(field){focusPending.current=true;setFields({[field]:t(failure.code)})}else setError(failure.code||'internal_error')}
  finally{inFlight.current=false;setBusy(false)}
 };
 return <form ref={form} className="auth-card merchant-phone-auth" noValidate aria-busy={busy} onSubmit={event=>{event.preventDefault();event.stopPropagation();void run(false)}}>
  {cfg.capabilities?.development_sms&&<p className="notice">{zh?'本机模拟短信，不发送真实短信。':'Local simulated SMS: no real message is sent.'}</p>}<fieldset disabled={busy}>
   <div className="merchant-auth-field"><label htmlFor={id+'-phone'}>{zh?'手机号':'Mobile number'}</label><input id={id+'-phone'} name="phone" value={phone} onChange={event=>{setPhone(event.target.value);requestID.current='';setCode('');setDev('');setFields(old=>({...old,phone:''}))}} inputMode="tel" autoComplete="tel" aria-invalid={!!fields.phone} aria-describedby={fields.phone?id+'-phone-error':undefined}/>{fields.phone&&<small className="field-error" id={id+'-phone-error'} role="alert">{fields.phone}</small>}</div>
   <SMSChallengeField phone={phone} purpose="login" value={proof} onChange={setProof}/><div className="merchant-auth-field"><label htmlFor={id+'-code'}>{t('code')}</label><div className="merchant-code-control"><input id={id+'-code'} name="code" value={code} onChange={event=>{setCode(event.target.value);setFields(old=>({...old,code:''}))}} inputMode="numeric" autoComplete="one-time-code" aria-invalid={!!fields.code} aria-describedby={fields.code?id+'-code-error':undefined}/><button className="button secondary" type="button" disabled={busy||seconds>0} onClick={()=>void run(true)}>{seconds?(zh?seconds+'秒后重发':seconds+'s'):t('sendCode')}</button></div>{fields.code&&<small className="field-error" id={id+'-code-error'} role="alert">{fields.code}</small>}</div>
  </fieldset>
  {dev&&<p className="notice">{zh?'本机模拟验证码（未发送短信）：':'Local simulated code (no SMS sent): '}<output aria-label={zh?'本机模拟验证码':'Local verification code'}>{dev}</output></p>}{error==='phone_login_unavailable'?<p className="notice error" role="alert">{zh?'该号码尚无已验证的业务账号。请先注册，或使用公司账号登录后绑定手机号。':'No verified business account. Register or sign in with your company account and bind a phone.'}<Link to="/register">{zh?'手机号注册':'Register with mobile'}</Link></p>:error&&<ErrorState code={error}/>}<button type="submit" className="button merchant-auth-submit" disabled={busy}>{busy?t('loading'):t('login')}</button>
 </form>;
}
