import {useCallback,useEffect,useRef,useState} from 'react';
import {api,token} from './api';

// A progress refresh must retain the current same-account draft tree. useLoad
// intentionally clears data on reload, which unmounted every editor at each poll.
export function useAIImportResult<T>(path:string){
 const session=token('merchant');const key=JSON.stringify([path,session]);
 const [version,setVersion]=useState(0);const [result,setResult]=useState<{key:string;data?:T;error?:string}>();
 const last=useRef(result);last.current=result;
 const reload=useCallback(()=>setVersion(v=>v+1),[]);
 useEffect(()=>{let active=true;const controller=new AbortController();api<T>(path,{signal:controller.signal},'merchant').then(data=>{if(active&&token('merchant')===session)setResult({key,data})}).catch(e=>{if(active&&token('merchant')===session)setResult({key,data:last.current?.key===key?last.current.data:undefined,error:e.code||'network_error'})});return()=>{active=false;controller.abort()}},[key,path,session,version]);
 return {data:result?.key===key?result.data:undefined,error:result?.key===key?result.error||'':'',reload};
}
