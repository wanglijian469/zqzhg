import {useRef,useState} from 'react';
import {api,APIError,body} from './api';
import {issueText} from './form-guide';
import {ErrorState,PhoneLogin,useLanguage} from './main';
import type {MerchantAuthResponse} from './types';

export type MerchantAuthMode='register'|'account'|'phone';
export function MerchantAuth({onLogin,defaultMode='register',companyName='',onBusy}:{onLogin:()=>void;defaultMode?:MerchantAuthMode;companyName?:string;onBusy?:(busy:boolean)=>void}){
 const {t,lang}=useLanguage();const zh=lang==='zh';const [mode,setMode]=useState(defaultMode);
 const [username,setUsername]=useState(companyName);const [password,setPassword]=useState('');const [confirmation,setConfirmation]=useState('');
 const [error,setError]=useState('');const [fieldError,setFieldError]=useState<Record<string,string>>({});const [busy,setBusy]=useState(false);const inFlight=useRef(false);
 const changeMode=(next:MerchantAuthMode)=>{if(inFlight.current)return;setMode(next);setPassword('');setConfirmation('');setFieldError({});setError('')};
 const submit=async()=>{
  if(inFlight.current)return;setError('');const fields:Record<string,string>={};
  if(!username.trim())fields.login_name=zh?'请填写公司登录账号。':'Enter a company login name.';
  else if(Array.from(username.trim()).length>100)fields.login_name=zh?'公司登录账号最多100字，请设置较短账号，企业名称会完整保留。':'Use a login name up to 100 characters. Your company name is retained.';
  if(mode==='register'){
   const bytes=new TextEncoder().encode(password).length;if(bytes<12||bytes>72||!password.trim())fields.password=zh?'密码需为12–72个 UTF-8 字节，不能全为空白。':'Use 12–72 UTF-8 bytes, not all whitespace.';
   if(password!==confirmation)fields.confirmation=zh?'两次密码不一致。':'Passwords do not match.';
  }else if(!password)fields.password=zh?'请填写密码。':'Enter your password.';
  setFieldError(fields);if(Object.keys(fields).length)return;
  inFlight.current=true;setBusy(true);onBusy?.(true);
  try{const r=await api<MerchantAuthResponse>(mode==='register'?'/auth/merchant/register':'/auth/merchant',{method:'POST',body:body(mode==='register'?{login_name:username,password}:{username,password})});sessionStorage.setItem('zqzt.merchant',r.token);sessionStorage.setItem('zqzt.merchant.id',String(r.account.id));setPassword('');setConfirmation('');window.dispatchEvent(new Event('zqzt.merchant.changed'));onLogin()}
  catch(e){const p=e as APIError;if(p.fields?.length){const f:Record<string,string>={};for(const i of p.fields)f[i.field]=i.code==='password_length'?(zh?'密码需为12–72个 UTF-8 字节，不能全为空白。':'Use 12–72 UTF-8 bytes, not all whitespace.'):issueText(i,lang);setFieldError(f)}else setError(p.code||'internal_error');if(p.code==='network_error'&&mode==='register')setError('registrationUnconfirmed')}
  finally{inFlight.current=false;setBusy(false);onBusy?.(false)}
 };
 return <div className="merchant-auth"><div className="tabs auth-tabs">{(['register','account','phone'] as const).map(key=><button type="button" disabled={busy} key={key} className={mode===key?'active':''} onClick={()=>changeMode(key)}>{key==='register'?(zh?'新厂商注册':'New manufacturer'):key==='account'?(zh?'已有公司账号登录':'Company sign-in'):t('smsLogin')}</button>)}</div>
 {mode==='phone'?<><p className="muted">{zh?'短信登录管理手机号对应账号，不会自动接管公司账号展厅。':'SMS sign-in uses the phone account; it does not claim a company account showroom.'}</p><PhoneLogin onLogin={onLogin} onBusy={onBusy}/></>:<form className="auth-card" noValidate onSubmit={e=>{e.preventDefault();e.stopPropagation();void submit()}}><p className="notice">{mode==='register'?(zh?'设置公司账号和密码即可注册，不需要短信。企业资料提交后仍需管理员审核。':'Create a company account without SMS. Company details still require administrator review.'):(zh?'管理员已建账号的厂商，请使用原公司账号和密码登录。':'Use the company account provided by your administrator.')}</p><fieldset disabled={busy}><label>{zh?'公司登录账号':'Company login name'}<input aria-label={zh?'公司登录账号':'Company login name'} autoComplete="username" value={username} aria-invalid={!!fieldError.login_name} onChange={e=>{setUsername(e.target.value);setFieldError(f=>({...f,login_name:''}))}}/><small className={'field-guide '+(fieldError.login_name?'field-error':'')}>{fieldError.login_name||(zh?`最多100字，可使用企业简称。${Array.from(username).length}/100`:`Use up to 100 characters. ${Array.from(username).length}/100`)}</small></label><label>{mode==='register'?(zh?'设置密码':'Set password'):t('password')}<input aria-label={mode==='register'?(zh?'设置密码':'Set password'):t('password')} type="password" autoComplete={mode==='register'?'new-password':'current-password'} value={password} aria-invalid={!!fieldError.password} onChange={e=>{setPassword(e.target.value);setFieldError(f=>({...f,password:''}))}}/><small className={'field-guide '+(fieldError.password?'field-error':'')}>{fieldError.password||(mode==='register'?(zh?'12–72个 UTF-8 字节，空格保留；中文通常占3字节，emoji通常占4字节。':'12–72 UTF-8 bytes; spaces are preserved. Chinese characters usually use 3 bytes, emoji 4.'):null)}</small></label>{mode==='register'&&<label>{zh?'确认密码':'Confirm password'}<input type="password" autoComplete="new-password" value={confirmation} aria-invalid={!!fieldError.confirmation} onChange={e=>{setConfirmation(e.target.value);setFieldError(f=>({...f,confirmation:''}))}}/>{fieldError.confirmation&&<small className="field-guide field-error">{fieldError.confirmation}</small>}</label>}</fieldset>{error&&<ErrorState code={error}/>} {(mode==='register'&&(fieldError.login_name||error==='registrationUnconfirmed'))&&<button type="button" className="text-link" onClick={()=>changeMode('account')}>{zh?'已有账号？登录继续':'Already registered? Sign in to continue'}</button>}<button type="submit" className="button" disabled={busy}>{busy?t('loading'):mode==='register'?(zh?'注册并继续':'Register and continue'):t('login')}</button></form>}</div>;
}
