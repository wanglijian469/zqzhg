import {useEffect,useRef,useState} from 'react';
import {api,body,APIError} from './api';
import {useRuntimeConfig} from './runtime-config';
import {useLanguage} from './ui';

export type SMSProof={challenge_id?:string;challenge_answer?:string};
export function smsRequestID(){return Array.from(crypto.getRandomValues(new Uint8Array(24)),v=>v.toString(16).padStart(2,'0')).join('')}
export function SMSChallengeField({phone,purpose,value,onChange}:{phone:string;purpose:string;value:SMSProof;onChange:(p:SMSProof)=>void}){
 const {lang}=useLanguage(),required=useRuntimeConfig().capabilities?.sms_challenge===true,zh=lang==='zh';
 const [image,setImage]=useState(''),[error,setError]=useState(''),[loading,setLoading]=useState(false),[generation,setGeneration]=useState(0);
 const current=useRef(0),changed=useRef(onChange);changed.current=onChange;
 useEffect(()=>{const id=++current.current;setImage('');setError('');changed.current({});if(!required||!/^1[3-9]\d{9}$/.test(phone))return;
  const timer=setTimeout(()=>{setLoading(true);void api<{challenge_id:string;image:string}>('/auth/phone/challenge',{method:'POST',body:body({phone,purpose})}).then(r=>{if(current.current!==id)return;setImage(r.image);changed.current({challenge_id:r.challenge_id,challenge_answer:''})}).catch(()=>{if(current.current===id)setError(zh?'图形验证码加载失败，请重试。':'Challenge failed to load. Retry.')}).finally(()=>{if(current.current===id)setLoading(false)})},250);
  return()=>{clearTimeout(timer);current.current++}
 },[phone,purpose,required,generation,zh]);
 if(!required)return null;
 return <div className="merchant-auth-field sms-challenge"><label htmlFor={'sms-human-'+purpose}>{zh?'图形验证码':'Image verification code'}</label><div className="merchant-code-control"><input id={'sms-human-'+purpose} name="challenge_answer" autoComplete="off" inputMode="numeric" maxLength={5} value={value.challenge_answer||''} onChange={e=>onChange({...value,challenge_answer:e.target.value})}/>{image&&<img src={image} width={156} height={52} alt={zh?'图形中的五位数字':'Five digits shown in the image'}/>}<button className="text-link" type="button" disabled={loading} onClick={()=>setGeneration(n=>n+1)}>{loading?(zh?'加载中':'Loading'):(zh?'换一张':'Refresh')}</button></div>{error&&<small role="alert">{error}</small>}</div>
}
export function requireSMSProof(required:boolean,proof:SMSProof){if(required&&(!proof.challenge_id||!/^\d{5}$/.test(proof.challenge_answer||'')))throw new APIError('sms_challenge_invalid')}
export const smsMessages={zh:{https_required:'请使用 https://zqzhg.com 进行短信验证。',phone_login_unavailable:'该手机号尚未注册或尚未验证，请先注册；已有公司账号请登录后绑定手机号。',sms_template_unavailable:'当前短信用途尚未启用，请使用密码登录或联系管理员。',sms_challenge_invalid:'请填写正确的图形验证码，失败后刷新图片重试。',sms_send_unknown:'短信发送状态尚未确认，请勿连续重发。如已收到短信，可直接填写验证码。',sms_send_pending:'该短信请求正在处理，请稍后重试。',sms_request_conflict:'请求信息已改变，请刷新后重试。',sms_request_id_required:'请更新客户端后重试。',sms_code_expired:'本次验证码已过期，请重新获取。',password_length:'请输入8–20位密码',password_confirmation_mismatch:'两次密码不一致。'},en:{https_required:'Use https://zqzhg.com for SMS verification.',phone_login_unavailable:'This number is not registered or verified. Register, or sign in with your company account to link it.',sms_template_unavailable:'SMS is unavailable for this purpose. Use password sign-in or contact support.',sms_challenge_invalid:'Enter the image code. Refresh the image after a failed attempt.',sms_send_unknown:'The sending status is uncertain. Avoid repeated requests. If a message arrives, enter its code.',sms_send_pending:'This request is processing. Retry shortly.',sms_request_conflict:'Request details changed. Refresh and retry.',sms_request_id_required:'Update your client and retry.',sms_code_expired:'This code has expired. Request a new one.',password_length:'Enter an 8–20 character password',password_confirmation_mismatch:'Passwords do not match.'}};
