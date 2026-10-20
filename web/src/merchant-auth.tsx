import {useCallback,useEffect,useId,useRef,useState} from 'react';
import {Link} from 'react-router-dom';
import {Eye,EyeOff} from 'lucide-react';
import {api,APIError,body} from './api';
import {issueText} from './form-guide';
import {ErrorState,useLanguage} from './main';
import {PhoneLogin} from './merchant-phone-login';
import type {MerchantAuthResponse} from './types';
import './merchant-entry.css';

export type MerchantAuthMode='register'|'account'|'phone';
export function MerchantAuth({onLogin,defaultMode='register',companyName='',onBusy,presentation='embedded',entryLinks}:{onLogin:(registered?:boolean)=>void;defaultMode?:MerchantAuthMode;companyName?:string;onBusy?:(busy:boolean)=>void;presentation?:'embedded'|'entry-register'|'entry-login';entryLinks?:{account:string;register:string}}){
 const {t,lang}=useLanguage(),id=useId(),zh=lang==='zh',entry=presentation!=='embedded';const [mode,setMode]=useState(defaultMode);
 const [username,setUsername]=useState(companyName),[password,setPassword]=useState(''),[confirmation,setConfirmation]=useState('');
 const [visible,setVisible]=useState(false),[confirmVisible,setConfirmVisible]=useState(false);
 const [error,setError]=useState(''),[fieldError,setFieldError]=useState<Record<string,string>>({}),[busy,setBusy]=useState(false);const inFlight=useRef(false),focusPending=useRef(false),form=useRef<HTMLFormElement>(null);
 const changeMode=(next:MerchantAuthMode)=>{if(inFlight.current||busy)return;setMode(next);setPassword('');setConfirmation('');setVisible(false);setConfirmVisible(false);setFieldError({});setError('')};
 const phoneBusy=useCallback((value:boolean)=>{setBusy(value);onBusy?.(value)},[onBusy]);
 const focusError=(fields:Record<string,string>)=>{const field=['login_name','password','confirmation'].find(key=>fields[key]);if(field)(form.current?.elements.namedItem(field) as HTMLElement)?.focus()};
 useEffect(()=>{if(!busy&&focusPending.current){focusPending.current=false;focusError(fieldError)}},[fieldError,busy]);
 const submit=async()=>{
  if(inFlight.current)return;setError('');const fields:Record<string,string>={};
  if(!username.trim())fields.login_name=zh?'请填写公司登录账号。':'Enter a company login name.';
  else if(Array.from(username.trim()).length>100)fields.login_name=zh?'公司登录账号最多100字，请设置较短账号，企业名称会完整保留。':'Use a login name up to 100 characters. Your company name is retained.';
  if(mode==='register'){
   const bytes=new TextEncoder().encode(password).length;if(bytes<12||bytes>72||!password.trim())fields.password=zh?'密码需为12–72个 UTF-8 字节，不能全为空白。':'Use 12–72 UTF-8 bytes, not all whitespace.';
   if(password!==confirmation)fields.confirmation=zh?'两次密码不一致。':'Passwords do not match.';
  }else if(!password)fields.password=zh?'请填写密码。':'Enter your password.';
  focusPending.current=Object.keys(fields).length>0;setFieldError(fields);if(Object.keys(fields).length){focusError(fields);return}
  inFlight.current=true;setBusy(true);onBusy?.(true);
  try{const response=await api<MerchantAuthResponse>(mode==='register'?'/auth/merchant/register':'/auth/merchant',{method:'POST',body:body(mode==='register'?{login_name:username,password}:{username,password})});sessionStorage.setItem('zqzt.merchant',response.token);sessionStorage.setItem('zqzt.merchant.id',String(response.account.id));setPassword('');setConfirmation('');setVisible(false);setConfirmVisible(false);window.dispatchEvent(new Event('zqzt.merchant.changed'));onLogin(mode==='register')}
  catch(cause){const failure=cause as APIError;if(failure.fields?.length){const fields:Record<string,string>={};for(const issue of failure.fields)fields[issue.field]=issue.code==='password_length'?(zh?'密码需为12–72个 UTF-8 字节，不能全为空白。':'Use 12–72 UTF-8 bytes, not all whitespace.'):issueText(issue,lang);focusPending.current=true;setFieldError(fields)}else setError(failure.code||'internal_error');if(failure.code==='network_error'&&mode==='register')setError('registrationUnconfirmed')}
  finally{inFlight.current=false;setBusy(false);onBusy?.(false)}
 };
 const routeLink=(target:'account'|'register',label:string)=>entryLinks?<Link className="text-link" to={entryLinks[target]} state={{entryLoginName:username}} aria-disabled={busy||undefined} tabIndex={busy?-1:undefined} onClick={event=>{if(busy)event.preventDefault()}}>{label}</Link>:<button type="button" className="text-link" onClick={()=>changeMode(target)}>{label}</button>;
 const passwordLabel=mode==='register'?(zh?'设置密码':'Set password'):t('password');
 const passwordControl=(confirm=false)=>{const label=confirm?(zh?'确认密码':'Confirm password'):passwordLabel,show=confirm?confirmVisible:visible;return <div className="merchant-password-control"><input id={id+(confirm?'-confirmation':'-password')} name={confirm?'confirmation':'password'} aria-label={label} type={show?'text':'password'} autoComplete={mode==='register'?'new-password':'current-password'} value={confirm?confirmation:password} aria-invalid={!!fieldError[confirm?'confirmation':'password']} aria-describedby={confirm?(fieldError.confirmation?id+'-confirmation-help':undefined):mode==='register'||fieldError.password?id+'-password-help':undefined} onChange={event=>{confirm?setConfirmation(event.target.value):setPassword(event.target.value);setFieldError(old=>({...old,[confirm?'confirmation':'password']:''}))}}/><button className="merchant-password-toggle" type="button" aria-label={(show?(zh?'隐藏':'Hide '):(zh?'显示':'Show '))+label} aria-pressed={show} onClick={()=>confirm?setConfirmVisible(value=>!value):setVisible(value=>!value)}>{show?<EyeOff size={18}/>:<Eye size={18}/>}</button></div>};
 return <div className={'merchant-auth'+(entry?' merchant-auth-entry':'')}>
  {presentation!=='entry-register'&&<div className={'tabs auth-tabs'+(entry?' merchant-entry-tabs':'')} aria-label={zh?'登录方式':'Sign-in method'}>{(presentation==='entry-login'?['account','phone'] as const:['register','account','phone'] as const).map(key=><button type="button" disabled={busy} key={key} className={mode===key?'active':''} aria-pressed={mode===key} onClick={()=>changeMode(key)}>{key==='register'?(zh?'新厂商注册':'New manufacturer'):key==='account'?(entry?(zh?'账号密码':'Company account'):(zh?'已有公司账号登录':'Company sign-in')):entry?(zh?'短信验证码':'SMS code'):t('smsLogin')}</button>)}</div>}
  {mode==='phone'?<><p className="merchant-phone-note">{zh?'短信登录管理手机号对应账号，不会自动接管公司账号展厅。':'SMS sign-in uses the phone account; it does not claim a company account showroom.'}</p><PhoneLogin onLogin={onLogin} onBusy={phoneBusy}/></>:<form ref={form} className="auth-card" noValidate aria-busy={busy} onSubmit={event=>{event.preventDefault();event.stopPropagation();void submit()}}>
   {!entry&&<p className="notice">{mode==='register'?(zh?'设置公司账号和密码即可注册，不需要短信。企业资料提交后仍需管理员审核。':'Create a company account without SMS. Company details still require administrator review.'):(zh?'管理员已建账号的厂商，请使用原公司账号和密码登录。':'Use the company account provided by your administrator.')}</p>}
   <fieldset disabled={busy}>
    <div className="merchant-auth-field"><label htmlFor={id+'-username'}>{zh?'公司登录账号':'Company login name'}</label><input id={id+'-username'} name="login_name" aria-label={zh?'公司登录账号':'Company login name'} autoComplete="username" autoCapitalize="none" spellCheck={false} value={username} aria-invalid={!!fieldError.login_name} aria-describedby={mode==='register'||!entry||fieldError.login_name?id+'-username-help':undefined} onChange={event=>{setUsername(event.target.value);setFieldError(old=>({...old,login_name:''}))}}/>{(mode==='register'||!entry||fieldError.login_name)&&<small id={id+'-username-help'} className={'field-guide '+(fieldError.login_name?'field-error':'')} role={fieldError.login_name?'alert':undefined}>{fieldError.login_name||(zh?`最多100字，可使用企业简称。${Array.from(username).length}/100`:`Up to 100 characters. ${Array.from(username).length}/100`)}</small>}</div>
    <div className="merchant-auth-field"><label htmlFor={id+'-password'}>{passwordLabel}</label>{passwordControl()}{(mode==='register'||fieldError.password)&&<small id={id+'-password-help'} className={'field-guide '+(fieldError.password?'field-error':'')} role={fieldError.password?'alert':undefined}>{fieldError.password||(zh?'12–72字节，建议组合字母、数字或符号。':'12–72 bytes; consider letters, numbers and symbols.')}</small>}</div>
    {mode==='register'&&<div className="merchant-auth-field"><label htmlFor={id+'-confirmation'}>{zh?'确认密码':'Confirm password'}</label>{passwordControl(true)}{fieldError.confirmation&&<small id={id+'-confirmation-help'} className="field-guide field-error" role="alert">{fieldError.confirmation}</small>}</div>}
   </fieldset>
   {error&&<ErrorState code={error}/>} {mode==='register'&&(fieldError.login_name||error==='registrationUnconfirmed')&&routeLink('account',zh?'已有账号？登录继续':'Already registered? Sign in to continue')}
   <button type="submit" className="button merchant-auth-submit" disabled={busy}>{busy?t('loading'):mode==='register'?(zh?'注册并继续':'Register and continue'):t('login')}</button>
   {mode==='register'&&<details className="merchant-auth-help"><summary>{zh?'密码要求说明':'Password requirements'}</summary><p>{zh?'密码需为12–72个 UTF-8 字节，不能全为空白。中文通常占3字节，emoji通常占4字节，空格会保留。':'Use 12–72 UTF-8 bytes, not all whitespace. Chinese characters usually use 3 bytes, emoji 4; spaces are preserved.'}</p></details>}
  </form>}
  {entry&&<><div className="merchant-entry-switch">{presentation==='entry-register'?<><span>{zh?'已经有账号？':'Already have an account?'}</span>{routeLink('account',zh?'已有公司账号登录':'Company sign-in')}</>:<><span>{zh?'还没有账号？':'New to the exhibition?'}</span>{routeLink('register',zh?'新厂商入驻':'Register your company')}</>}</div>{presentation==='entry-login'&&<details className="merchant-auth-help"><summary>{zh?'登录帮助':'Sign-in help'}</summary><p>{zh?'公司账号由厂商自行注册或由管理员开通。忘记密码时，请联系为你开通账号的平台管理员重置。':'Company accounts are registered by manufacturers or created by an administrator. Ask your platform administrator to reset a forgotten password.'}</p><p>{zh?'已有展厅请使用原公司账号登录，避免重复注册。':'Use the original company account for an existing showroom.'}</p></details>}</>}
 </div>;
}
