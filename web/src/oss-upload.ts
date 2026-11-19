import {api,body,APIError,token} from './api';
import {runtimeConfig,mediaURL} from './runtime-config';

type Role='merchant'|'admin';
type Session={id:string;state:string;media_id?:number;error?:string;expires_at:string};
type Authorization={session:Session;mode:'single'|'multipart';url?:string;headers?:Record<string,string>;part_size?:number;part_count?:number;uploaded_parts?:number[]};
const sessions=new Map<string,string>(); // Session IDs only; signed URLs are never persisted.
const prefix=(role:Role)=>role==='admin'?'/admin/storage/files':'/merchant/files';
export const directUploadsEnabled=()=>runtimeConfig().capabilities?.oss_direct_upload===true;

function put(url:string,headers:Record<string,string>,data:Blob,signal:AbortSignal,progress:(bytes:number)=>void){
 return new Promise<void>((resolve,reject)=>{
  if(signal.aborted){reject(new DOMException('Cancelled','AbortError'));return}
  const request=new XMLHttpRequest();request.open('PUT',url);request.timeout=300000;
  // The browser supplies Content-Length from Blob.size. Setting it is forbidden.
  for(const [key,value] of Object.entries(headers))if(!['content-length','host','authorization'].includes(key.toLowerCase()))request.setRequestHeader(key,value);
  request.upload.onprogress=e=>progress(e.loaded);
  const cleanup=()=>signal.removeEventListener('abort',abort);
  const abort=()=>request.abort();signal.addEventListener('abort',abort,{once:true});
  request.onload=()=>{cleanup();if(request.status>=200&&request.status<300){progress(data.size);resolve()}else reject(new APIError(request.status===403?'oss_authorization_expired':'oss_upload_failed'))};
  request.onerror=request.ontimeout=()=>{cleanup();reject(new APIError('network_error'))};
  request.onabort=()=>{cleanup();reject(new DOMException('Cancelled','AbortError'))};
  request.send(data);
 });
}
function pause(ms:number,signal:AbortSignal){return new Promise<void>((resolve,reject)=>{const abort=()=>{clearTimeout(timer);reject(new DOMException('Cancelled','AbortError'))};const timer=setTimeout(()=>{signal.removeEventListener('abort',abort);resolve()},ms);signal.addEventListener('abort',abort,{once:true});if(signal.aborted)abort()})}

export async function uploadCloudFile(file:File,kind:string,purpose:string,role:Role,signal:AbortSignal,onProgress:(n:number)=>void,path=''){
 const root=prefix(role),key=JSON.stringify([role,token(role),path,kind,purpose,file.name,file.size,file.lastModified]);
 const business=path.match(/\/(contents|resources)\/(\d+)\//);
 const digest=await crypto.subtle.digest('SHA-256',new TextEncoder().encode(key));const storageKey='zqzt.oss.upload.'+Array.from(new Uint8Array(digest),b=>b.toString(16).padStart(2,'0')).join('');
 let saved:{id?:string;request_id?:string;created?:number}={};try{saved=JSON.parse(sessionStorage.getItem(storageKey)||'{}');if((saved.created||0)<Date.now()-7200000)saved={}}catch{}
 const persist=()=>{try{sessionStorage.setItem(storageKey,JSON.stringify(saved))}catch{}};
 const forget=()=>{sessions.delete(key);try{sessionStorage.removeItem(storageKey)}catch{}};
 let id=sessions.get(key)||saved.id,auth:Authorization;

 if(id){
  const {session}=await api<{session:Session}>(root+'/uploads/'+id,{signal},role);
  if(session.state==='completed'&&session.media_id){forget();return session.media_id;}
  if(['cancelled','reclaimed'].includes(session.state)||new Date(session.expires_at).getTime()<=Date.now()){forget();saved={};id=undefined}
  else if(session.state==='failed'){await api(root+'/uploads/'+id+'/retry',{method:'POST',signal,body:'{}'},role);const result=await waitForFile(root,id,role,signal,onProgress);forget();return result;}
  else if(session.state==='validating'){const result=await waitForFile(root,id,role,signal,onProgress);forget();return result;}
 }
 if(!id){
  saved={request_id:saved.request_id||crypto.randomUUID(),created:saved.created||Date.now()};persist();
  auth=await api<Authorization>(root+'/uploads',{method:'POST',signal,body:body({idempotency_key:saved.request_id,kind,purpose,filename:file.name,size:file.size,mime_type:file.type,business_kind:business?.[1]==='contents'?'content':business?'resource':'',business_id:Number(business?.[2]||0)})},role);
  id=auth.session.id;sessions.set(key,id);saved.id=id;persist();
 }else auth=await api<Authorization>(root+'/uploads/'+id+'/authorization',{method:'POST',signal,body:'{}'},role);
 if(auth.session.state==='completed'&&auth.session.media_id){forget();return auth.session.media_id}
 if(auth.session.state==='validating'){const result=await waitForFile(root,id,role,signal,onProgress);forget();return result}
 if(auth.session.state!=='uploading'){forget();throw new APIError('file_upload_cancelled')}
 const uploadID=id;
 const cancel=()=>{void api(root+'/uploads/'+uploadID+'/cancel',{method:'POST',body:'{}'},role).catch(()=>{});forget()};
 signal.addEventListener('abort',cancel,{once:true});
 try{
  if(auth.mode==='single')await put(auth.url!,auth.headers||{},file,signal,n=>onProgress(Math.floor(n/file.size*100)));
  else {
   const loaded=new Map<number,number>();const count=auth.part_count!;let next=1;const uploaded=new Set(auth.uploaded_parts||[]);for(const part of uploaded)loaded.set(part,Math.min(auth.part_size!,file.size-(part-1)*auth.part_size!));
   const parallel=async()=>{while(next<=count){const part=next++;if(uploaded.has(part))continue;const {items}=await api<{items:{part_number:number;offset:number;size:number;url:string;headers:Record<string,string>}[]}>(root+'/uploads/'+id+'/parts',{method:'POST',signal,body:body({parts:[part]})},role);const p=items[0];await put(p.url,p.headers,file.slice(p.offset,p.offset+p.size),signal,n=>{loaded.set(part,n);onProgress(Math.floor([...loaded.values()].reduce((a,b)=>a+b,0)/file.size*100))})}};
   await Promise.all([parallel(),parallel()]);
  }
  await api(root+'/uploads/'+id+'/complete',{method:'POST',body:'{}',signal},role);
  const media=await waitForFile(root,id,role,signal,onProgress);forget();return media;
 }finally{signal.removeEventListener('abort',cancel)}
}
async function waitForFile(root:string,id:string,role:Role,signal:AbortSignal,progress:(n:number)=>void){
 for(let n=0;n<450;n++){
  const {session}=await api<{session:Session}>(root+'/uploads/'+id,{signal},role);
  if(session.state==='completed'&&session.media_id){progress(100);return session.media_id}
  if(['failed','cancelled','reclaimed'].includes(session.state))throw new APIError(session.error||'oss_upload_failed');
  await pause(2000,signal);
 }
 throw new APIError('file_processing_timeout');
}

export async function privateMediaAccess(id:number,role:Role,signal?:AbortSignal,thumb=false,width=0):Promise<{url:string;headers:Record<string,string>}>{
 if(runtimeConfig().capabilities?.oss_file_access){
  try{const result=await api<{url:string;direct:boolean}>(prefix(role)+'/'+id+'/access'+(thumb&&width<=480?'?thumb=1':''),{signal},role);
  const url=!result.direct&&width>0?mediaURL(id)+'?w='+width:result.url;return {url,headers:result.direct?{}:{Authorization:'Bearer '+token(role)}};}catch(e){if(role!=='admin'||(e as APIError).code!=='forbidden')throw e}
 }
 return {url:mediaURL(id)+(width>0?'?w='+width:thumb?'?thumb=1':''),headers:{Authorization:'Bearer '+token(role)}};
}
