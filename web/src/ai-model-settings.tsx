import {AIPromptSettings} from './ai-prompt-settings';
import {useEffect,useState} from 'react';
import {api,APIError} from './api';
import {Loading,useLoad} from './ui';
type Settings={version:number;base_url:string;timeout_seconds:number;default_model:string;extraction_model?:string;review_model:string;key_configured:boolean;key_source:string;configured:boolean;configuration_error:string;enabled:boolean};
const path='/admin/settings/ai-models',flash='qwen3.8-flash',max='qwen3.8-max';
const messages:Record<string,string>={ai_invalid_model_settings:'请使用百炼官方HTTPS业务空间地址，选择flash或max，超时应在15至600秒之间。',ai_model_encryption_required:'后端加密配置尚未就绪，请配置CONFIG_ENCRYPTION_KEY。',ai_model_key_unreadable:'密钥无法解密，请检查服务器加密配置。',bailian_configuration_required:'请配置按量计费业务密钥，不使用Coding Plan密钥。',ai_max_confirmation_required:'请明确确认max调用成本较高。',stale_revision:'配置已被其他管理员修改，请重新加载。',bailian_http_401:'密钥无效，请重新填写。',bailian_http_403:'当前业务空间没有该模型权限。'};
export function AIModelSettings(){
 const request=useLoad<Settings>(path,'admin');const [settings,setSettings]=useState<Settings>();const [keyAction,setKeyAction]=useState('keep'),[key,setKey]=useState('');
 const [tab,setTab]=useState<'models'|'prompts'>('models');const [busy,setBusy]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState(''),[testMode,setTestMode]=useState('flash'),[confirmMax,setConfirmMax]=useState(false),[confirmConfigMax,setConfirmConfigMax]=useState(false);
 useEffect(()=>{if(request.data){setSettings({...request.data,extraction_model:request.data.extraction_model||request.data.default_model||flash});setConfirmConfigMax(false)}},[request.data]);
 const text=(e:unknown)=>messages[(e as APIError).code]||(e as APIError).code||'请求失败';
 const save=async()=>{if(!settings||busy)return;setBusy(true);setError('');setNotice('');try{
  const v=await api<Settings>(path,{method:'PUT',body:JSON.stringify({version:settings.version,base_url:settings.base_url,timeout_seconds:settings.timeout_seconds,extraction_model:settings.extraction_model,review_model:settings.review_model,confirm_max:confirmConfigMax,key_action:keyAction,...(keyAction==='replace'?{api_key:key}:{})})},'admin');
  setSettings(v);setKey('');setKeyAction('keep');setConfirmConfigMax(false);setNotice('配置已保存，仅影响新任务；已有任务与重试保持原模型。');
 }catch(e){setError(text(e))}finally{setBusy(false)}};
 const test=async()=>{setBusy(true);setError('');setNotice('');try{const v=await api<{model:string;usage:{total_tokens?:number}|null}>(path+'/test',{method:'POST',body:JSON.stringify({model_mode:testMode,confirm_max:confirmMax}),signal:AbortSignal.timeout(15*60*1000)},'admin');setNotice(`连通成功：${v.model}；本次Token：${v.usage?.total_tokens??'未返回'}。实际费用以百炼账单为准。`)}catch(e){setError(text(e))}finally{setBusy(false)}};
 if(request.error)return <div className="notice error" role="alert">{messages[request.error]||request.error}<button onClick={request.reload}>重新加载</button></div>;if(!settings)return <Loading/>;
 const tabs=<div className="settings-actions ai-model-tabs" role="tablist" aria-label="AI管理配置"><button role="tab" aria-selected={tab==='models'} onClick={()=>setTab('models')}>模型连接</button><button role="tab" aria-selected={tab==='prompts'} onClick={()=>setTab('prompts')}>内容提取规则</button></div>;
 if(tab==='prompts')return <div>{tabs}<AIPromptSettings/></div>;
 const costly=settings.extraction_model===max||settings.review_model===max;
 return <div className="ai-model-settings">{tabs}<h2>AI模型配置 · 阿里云百炼</h2><p className="notice">模型由管理员统一配置，厂商无需选择。默认提取、复核均使用flash；不会按文档长度或失败次数自动升级。新任务固化模型，后续配置不改变已有任务。</p>
  <p>密钥状态：{settings.key_configured?'已配置':'未配置'} · 来源：{settings.key_source==='admin'?'后台加密保存':settings.key_source==='disabled'?'已停用':'后端环境变量'} · AI功能：{settings.enabled?'已启用':'未启用（由后端AI_IMPORT_ENABLED控制）'}</p>
  {settings.configuration_error&&<p className="notice error">{messages[settings.configuration_error]||settings.configuration_error}</p>}
  <fieldset disabled={busy}><div className="settings-grid">
   <label>提取模型<select aria-label="提取模型" value={settings.extraction_model||flash} onChange={e=>{setSettings({...settings,extraction_model:e.target.value});setConfirmConfigMax(false)}}><option value={flash}>{flash}（默认）</option><option value={max}>{max}（较高费用）</option></select></label>
   <label>复核模型<select aria-label="复核模型" value={settings.review_model} onChange={e=>{setSettings({...settings,review_model:e.target.value});setConfirmConfigMax(false)}}><option value={flash}>{flash}（默认）</option><option value={max}>{max}（较高费用）</option></select></label>
   <label>百炼API地址<input type="url" value={settings.base_url} onChange={e=>setSettings({...settings,base_url:e.target.value})}/><small>使用对应地域和业务空间的 /compatible-mode/v1 地址。</small></label>
   <label>单次请求超时（秒）<input type="number" min={15} max={600} value={settings.timeout_seconds} onChange={e=>setSettings({...settings,timeout_seconds:Number(e.target.value)})}/></label>
   <label>密钥操作<select aria-label="密钥操作" value={keyAction} onChange={e=>{setKeyAction(e.target.value);setKey('')}}><option value="keep">保留当前密钥</option><option value="replace">填写／更换后台密钥</option><option value="environment">清除后台密钥，使用后端环境变量</option><option value="disable">停用业务密钥</option></select></label>
   {keyAction==='replace'&&<label>按量计费API Key<input aria-label="按量计费API Key" type="password" autoComplete="new-password" value={key} onChange={e=>setKey(e.target.value)}/><small>保存后不回显，不使用Coding Plan密钥。</small></label>}
  </div>{costly&&<label className="notice"><input type="checkbox" checked={confirmConfigMax} onChange={e=>setConfirmConfigMax(e.target.checked)}/>我确认新任务使用max的较高调用费用</label>}
   <div className="settings-actions"><button className="button" disabled={busy||keyAction==='replace'&&!key.trim()||costly&&!confirmConfigMax} onClick={()=>void save()}>保存模型配置</button><button className="button secondary" onClick={()=>{setKey('');setKeyAction('keep');request.reload()}}>重新加载</button></div>
  </fieldset><hr/><h3>连通性测试</h3><p>测试会产生API费用，仅发送短文本，不上传画册，不修改企业和产品。</p>
  <label>测试模型<select aria-label="测试模型" value={testMode} disabled={busy} onChange={e=>{setTestMode(e.target.value);setConfirmMax(false)}}><option value="flash">{flash}（默认）</option><option value="max">{max}（手动，高费用）</option></select></label>
  {testMode==='max'&&<label><input type="checkbox" checked={confirmMax} disabled={busy} onChange={e=>setConfirmMax(e.target.checked)}/>我明确选择max，并确认费用较高</label>}
  <button className="button secondary" disabled={busy||!settings.configured||testMode==='max'&&!confirmMax} onClick={()=>void test()}>测试已保存的连接</button>
  {busy&&<p role="status">正在处理，请稍候…</p>}{error&&<p role="alert" className="notice error">{error}</p>}{notice&&<p role="status" className="notice">{notice}</p>}
  <p className="muted">API Key不写入浏览器存储；后台使用AES-GCM加密保存。生产环境需保留CONFIG_ENCRYPTION_KEY。</p>
 </div>;
}
