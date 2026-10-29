import {useRef,useState,useEffect} from 'react';
import {Link} from 'react-router-dom';
import {requestLoginCode,loginWithCode} from './phone-login-api';
import {api,body} from './api';
import {useLanguage} from './ui';
import {useRuntimeConfig} from './runtime-config';
import {storeBusinessSession} from './business-session';
import {type BusinessAccount,procError,procurementErrorText} from './procurement-api';

export function ProcurementLogin({supplier=false}:{supplier?:boolean}){
 const {lang,t}=useLanguage(),en=lang==='en',cfg=useRuntimeConfig();const sms=cfg.capabilities?.phone_sms_auth!==false;
 const [mode,setMode]=useState<'sms'|'password'|'company'|'register'>(sms?'sms':'password');const [phone,setPhone]=useState(''),[password,setPassword]=useState(''),[code,setCode]=useState(''),[development,setDevelopment]=useState(''),[error,setError]=useState(''),[busy,setBusy]=useState(false),[seconds,setSeconds]=useState(0);const flight=useRef(false);
 useEffect(()=>{if(seconds>0){const timer=setTimeout(()=>setSeconds(s=>s-1),1000);return()=>clearTimeout(timer)}},[seconds]);
 useEffect(()=>{if(!sms&&mode==='sms')setMode('password')},[sms,mode]);
 const change=(next:typeof mode)=>{setMode(next);setCode('');setDevelopment('');setPassword('');setError('')};
 const run=async(send=false)=>{if(flight.current)return;flight.current=true;setBusy(true);setError('');try{
  if(send){const r=mode==='register'?await api<{development_code?:string;retry_after?:number}>('/auth/phone/code',{method:'POST',body:body({phone,purpose:'register'})}):await requestLoginCode(phone);setDevelopment(r.development_code||'');setSeconds(r.retry_after||60)}
  else{const path=mode==='sms'?'/auth/phone/login':mode==='register'?'/auth/phone/register':mode==='company'?'/auth/merchant':'/auth/phone/password';const value=mode==='sms'?{phone,code}:mode==='register'?{phone,password,code,role:'visitor'}:mode==='company'?{username:phone,password}:{phone,password};const result=mode==='sms'?await loginWithCode(phone,code):await api<{token:string;account:BusinessAccount}>(path,{method:'POST',body:body(value)});storeBusinessSession(result);setPassword('');setCode('');setDevelopment('')}
 }catch(e){setError(procError(e))}finally{flight.current=false;setBusy(false)}};
 return <section className="proc-card proc-auth"><h1>{supplier?(en?'Supplier sign-in':'厂商登录'):(en?'Buyer sign-in':'采购商登录')}</h1><p>{en?'Use one account for buying and supplying.':'已有账号直接登录，采购与厂商使用同一账号。'}</p>
 {cfg.capabilities?.development_sms&&<p className="notice">{en?'Local simulated SMS: no real message is sent. The code appears here after requesting it.':'本机模拟短信，不发送真实短信。点击获取验证码后，动态验证码将在页面显示。'}</p>}
 {!!cfg.demo_accounts?.length&&<div className="proc-demo-accounts"><p>{en?'Choose a demo account':'选择演示账号'}</p>{cfg.demo_accounts.map((entry,index)=><button className="button secondary" type="button" key={entry.phone} disabled={busy} onClick={()=>{change('sms');setPhone(entry.phone);setSeconds(0)}}>{en?['Demo buyer','Supplier A','Supplier B'][index]:entry.label}<small>{entry.phone}</small></button>)}</div>}
 <div className="proc-tabs">{([...(sms?['sms']:[]),'password','company','register'] as (typeof mode)[]).map(value=><button disabled={busy} key={value} aria-pressed={mode===value} onClick={()=>change(value)}>{({sms:en?'SMS code':'验证码登录',password:en?'Password':'密码登录',company:en?'Company account':'公司账号',register:en?'Register':'手机注册'})[value]}</button>)}</div>
 <form onSubmit={e=>{e.preventDefault();void run()}}><fieldset disabled={busy}><label>{mode==='company'?(en?'Company login name':'公司登录账号'):(en?'Mobile number':'手机号')}<input required autoComplete="username" inputMode={mode==='company'?'text':'tel'} value={phone} onChange={e=>{setPhone(e.target.value);setCode('');setDevelopment('');setSeconds(0)}}/></label>
 {mode!=='sms'&&<label>{en?'Password':'密码'}<input required type="password" minLength={mode==='register'?12:undefined} maxLength={72} value={password} onChange={e=>setPassword(e.target.value)} autoComplete={mode==='register'?'new-password':'current-password'}/></label>}
 {(mode==='sms'||mode==='register')&&<><label>{en?'SMS verification code':'短信验证码'}<div className="proc-inline"><input aria-label={en?'SMS verification code':'短信验证码'} required autoComplete="one-time-code" inputMode="numeric" pattern="[0-9]{6}" maxLength={6} value={code} onChange={e=>setCode(e.target.value)}/><button type="button" className="button secondary" disabled={seconds>0} onClick={()=>void run(true)}>{seconds?seconds+'s':en?'Get code':'获取验证码'}</button></div></label>{development&&<p className="notice" role="status">{en?'Local simulated code (no SMS sent): ':'本机模拟验证码（未发送短信）：'}<output aria-label={en?'Local verification code':'本机模拟验证码'}>{development}</output></p>}</>}
 {mode==='sms'&&<p className="muted">{en?'Sign in to an existing verified account. New users must register first.':'仅登录已有且验证过手机号的账号；新用户请先选择手机注册。'}</p>}{mode==='register'&&<small>{en?'Password: 12–72 UTF-8 bytes.':'密码需为12–72个 UTF-8 字节。'}</small>}<button className="button">{busy?t('loading'):mode==='register'?(en?'Register and continue':'注册并继续'):(en?'Sign in':'登录')}</button></fieldset></form>
 {error&&<p className="notice error" role="alert">{error==='phone_login_unavailable'?(en?'This number has no verified business account. Register or sign in with your company account and bind a phone.':'该号码尚无已验证的业务账号，请先注册，或用公司账号登录后绑定手机号。'):procurementErrorText(error,en,t)}</p>}<Link to="/procurement">{en?'Browse procurement requests':'先浏览采购需求'}</Link></section>;
}
