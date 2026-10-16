import {useEffect,useRef,useState} from 'react';
import {api,APIError,body} from './api';
import {ErrorState,Modal,useLanguage} from './main';
import type {AdminAccount,Collection,PasswordResetResult} from './types';

export function PasswordResetButton({account,ownerID,companyName,onChanged}:{account?:AdminAccount;ownerID?:number;companyName?:string;onChanged?:()=>void}) {
 const {lang,t}=useLanguage();const zh=lang==='zh';
 const [loaded,setLoaded]=useState(account);const [loadError,setLoadError]=useState('');
 const [open,setOpen]=useState(false);const [mode,setMode]=useState('generate');
 const [password,setPassword]=useState('');const [confirmation,setConfirmation]=useState('');
 const [error,setError]=useState('');const [busy,setBusy]=useState(false);const submitting=useRef(false);
 const [result,setResult]=useState<PasswordResetResult>();const [copyHint,setCopyHint]=useState('');
 useEffect(()=>{if(account){setLoaded(account);return}let active=true;api<Collection<AdminAccount>>('/admin/accounts?account_id='+ownerID,{},'admin').then(r=>{if(active){setLoaded(r.items[0]);if(!r.items[0])setLoadError('not_found')}}).catch(e=>{if(active)setLoadError((e as APIError).code)});return()=>{active=false}},[account,ownerID]);
 const close=()=>{if(submitting.current)return;const completed=!!result;setOpen(false);setPassword('');setConfirmation('');setResult(undefined);setError('');setCopyHint('');setMode('generate');if(completed)onChanged?.()};
 if(loadError)return <ErrorState code={loadError}/>;
 if(!loaded)return <span className="muted">{t('loading')}</span>;
 if(!loaded.login_name?.trim())return <small className="muted">{zh?'此账号未配置公司登录名，无法重置公司密码。':'No company login name is configured; there is no company password to reset.'}</small>;
 const submit=async()=>{
  if(submitting.current)return;
  if(mode==='manual'){
   const bytes=new TextEncoder().encode(password).length;
   if(bytes<12||bytes>72||!password.trim()){setError(zh?'新密码需为12–72个 UTF-8 字节，且不能全部为空白。':'Use 12–72 UTF-8 bytes; the password cannot be all whitespace.');return}
   if(password!==confirmation){setError(zh?'两次输入的密码不一致。':'The passwords do not match.');return}
  }
  submitting.current=true;setBusy(true);setError('');
  try{const r=await api<PasswordResetResult>(`/admin/accounts/${loaded.id}/password-reset`,{method:'POST',body:body(mode==='manual'?{mode,password}:{mode})},'admin');setResult(r);setPassword('');setConfirmation('')}
  catch(e){const code=(e as APIError).code;setError(code==='network_error'?(zh?'未能确认重置结果。请重新确认后再操作；再次重置会使前次密码失效。':'The reset result could not be confirmed. A further reset invalidates any password created by the previous request.'):code==='invalid_request'?(zh?'请检查新密码规范或账号登录方式。':'Check the password requirements and account login method.'):t(code))}
  finally{submitting.current=false;setBusy(false)}
 };
 return <><button type="button" className="text-link" onClick={()=>setOpen(true)}>{zh?'重置密码':'Reset password'}</button>{open&&<Modal close={close} closeDisabled={busy}><div className="password-reset"><h2>{result?(zh?'密码重置成功':'Password reset complete'):(zh?'重置厂商密码':'Reset manufacturer password')}</h2><dl className="reset-identity"><dt>{zh?'企业名称':'Manufacturer'}</dt><dd>{loaded.vendor_name||companyName||t('accountUnassigned')}</dd><dt>{t('loginName')}</dt><dd>{loaded.login_name}</dd></dl>{result?<><p className="notice success">{zh?`已撤销 ${result.revoked_sessions} 个旧会话。新密码只显示此次，请复制后交给厂商。`:`${result.revoked_sessions} previous sessions revoked. Copy the password now; it is displayed only here.`}</p><label>{zh?'新密码':'New password'}<input className="reset-password-value" aria-label={zh?'新密码':'New password'} readOnly value={result.password} onFocus={e=>e.target.select()}/></label><p className="muted">{zh?'关闭弹窗后不会再显示；企业资料、产品和图片归属保持不变。':'Closing removes the password from this dialog. Company information, products and images are unchanged.'}</p>{copyHint&&<p role="status" className="notice">{copyHint}</p>}<div className="form-actions"><button type="button" className="button secondary" onClick={async()=>{try{await navigator.clipboard.writeText(result.login_name+'\n'+result.password);setCopyHint(zh?'已复制账号与密码。':'Login details copied.')}catch{setCopyHint(zh?'复制不可用，请手动选择登录账号和新密码复制。':'Copy is unavailable. Select the account name and password manually.')}}}>{zh?'复制账号与密码':'Copy account and password'}</button><button type="button" className="button" onClick={close}>{t('done')}</button></div></>:<form onSubmit={e=>{e.preventDefault();void submit()}}><p className="notice">{zh?'重置后旧会话将失效，厂商需使用新密码重新登录。企业资料、产品及图片保持不变。':'Previous sessions will expire. The manufacturer must sign in again; company information, products and images are unchanged.'}</p><fieldset disabled={busy}><label>{zh?'重置方式':'Reset method'}<select value={mode} onChange={e=>{setMode(e.target.value);setPassword('');setConfirmation('');setError('')}}><option value="generate">{zh?'随机生成（推荐）':'Generate a random password'}</option><option value="manual">{zh?'手动设置':'Set a password manually'}</option></select></label>{mode==='manual'&&<><label>{zh?'新密码':'New password'}<input aria-label={zh?'新密码':'New password'} type="password" autoComplete="new-password" value={password} onChange={e=>{setPassword(e.target.value);setError('')}}/><small className="field-guide">{zh?'12–72个 UTF-8 字节；中文通常占3字节，emoji通常占4字节。空格会保留。':'12–72 UTF-8 bytes. Chinese characters usually use 3 bytes; emoji usually use 4. Spaces are preserved.'}</small></label><label>{zh?'确认新密码':'Confirm new password'}<input type="password" autoComplete="new-password" value={confirmation} onChange={e=>{setConfirmation(e.target.value);setError('')}}/></label></>}</fieldset>{error&&<p className="notice error" role="alert">{error}</p>}<div className="form-actions"><button type="button" disabled={busy} className="button secondary" onClick={close}>{t('cancel')}</button><button type="submit" disabled={busy} className="button">{busy?t('loading'):(zh?'确认重置':'Confirm reset')}</button></div></form>}</div></Modal>}</>;
}
