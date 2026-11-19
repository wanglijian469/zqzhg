import {useEffect,useState} from 'react';
import {api,APIError} from './api';
import {useLoad,ErrorState} from './ui';
import {useRuntimeConfig} from './runtime-config';
export function FilePublicationNotice(){
 const config=useRuntimeConfig();const enabled=config.capabilities?.oss_file_access===true;
 const r=useLoad<{items:{id:string;kind:string;status:string;resource_kind:string;resource_id:number;error?:string}[]}>('/merchant/file-jobs','merchant',enabled);
 const [error,setError]=useState(''),[busy,setBusy]=useState('');
 useEffect(()=>{if(!enabled)return;const timer=setInterval(r.reload,10000);return()=>clearInterval(timer)},[enabled]);
 const jobs=r.data?.items.filter(j=>j.kind==='publish'&&['queued','processing','failed'].includes(j.status))||[];
 if(!enabled||!jobs.length)return null;
 return <section className="notice" aria-label="审核后文件准备"><strong>平台审核已通过，文件准备完成后公开新版本。</strong>{jobs.map(j=><p key={j.id}>资料 #{j.resource_id}：{j.status==='failed'?'文件准备失败，原公开版本保留':'正在准备公开文件'} {j.error&&<small>{j.error}</small>}{j.status==='failed'&&<button disabled={!!busy} onClick={async()=>{setBusy(j.id);setError('');try{await api('/merchant/file-jobs/'+j.id+'/retry',{method:'POST',body:'{}'},'merchant');r.reload()}catch(e){setError((e as APIError).code)}finally{setBusy('')}}}>重试文件准备</button>}</p>)}{error&&<ErrorState code={error}/>}</section>
}
