import {useEffect,useState} from 'react';
import {api,token} from './api';
import type {Task} from './ai-import-types';
// Schedule after completion, never abort a slow request with the next tick.
export function useAITaskPolling(path:string,initial:Task|undefined,changed:()=>void){
 const session=token('merchant');const [state,setState]=useState<{path:string;session:string;task:Task}>();const [error,setError]=useState('');
 useEffect(()=>{
  if(!initial)return;let active=true,running=false,delay=2500,known=initial.result_version||0,timer:ReturnType<typeof setTimeout>|undefined;const controller=new AbortController();setError('');
  const schedule=(ms:number)=>{clearTimeout(timer);if(active)timer=setTimeout(()=>void run(),ms)};
  const run=async()=>{if(!active||running)return;running=true;try{const next=await api<Task>(path,{signal:controller.signal},'merchant');if(!active||token('merchant')!==session)return;setState({path,session,task:next});setError('');delay=2500;if(next.result_version===undefined||(next.result_version||0)!==known){known=next.result_version||0;changed()}if(['queued','processing'].includes(next.status))schedule(document.hidden?15000:delay)}catch(e){if(active){setError((e as {code?:string}).code||'network_error');delay=Math.min(30000,delay*2);schedule(delay)}}finally{running=false}};
  const wake=()=>{if(!document.hidden&&!running)schedule(0)};window.addEventListener('online',wake);document.addEventListener('visibilitychange',wake);
  if(['queued','processing'].includes(initial.status))schedule(0);
  return()=>{active=false;controller.abort();clearTimeout(timer);window.removeEventListener('online',wake);document.removeEventListener('visibilitychange',wake)};
 },[path,session,initial?.id,initial?.status,changed]);
 const current=state?.path===path&&state.session===session?state.task:undefined;
 return {task:current&&(current.result_version||0)>=(initial?.result_version||0)?current:initial,error};
}
