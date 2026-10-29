import React,{Component,Suspense,lazy,useMemo,useState,type ComponentType,type ReactNode} from 'react';
import {Loading,useLanguage} from './ui';

class RouteLoadError extends Error {
 constructor(cause:unknown){super('Route module failed to load',{cause})}
}
class ChunkBoundary extends Component<{children:ReactNode;fallback:ReactNode;onError?:()=>void},{error?:unknown}> {
 state:{error?:unknown}={};
 static getDerivedStateFromError(error:unknown){return {error}}
 componentDidCatch(error:unknown){if(error instanceof RouteLoadError)this.props.onError?.()}
 render(){
  if(this.state.error && !(this.state.error instanceof RouteLoadError))throw this.state.error;
  return this.state.error?this.props.fallback:this.props.children;
 }
}

// A new lazy instance on retry avoids React.lazy retaining a rejected promise.
export function lazyRoute<T extends ComponentType<any>>(load:()=>Promise<{default:T}>){
 return function LazyRoute(props:React.ComponentProps<T>){
  const {lang}=useLanguage();const [attempt,retry]=useState(0);
  const Page=useMemo(()=>lazy(()=>load().catch(cause=>{throw new RouteLoadError(cause)})),[attempt]);
  const fallback=<section className="notice error" role="alert"><p>{lang==='zh'?'页面资源加载失败，请检查网络后重试。':'Page resources could not load. Check your connection and retry.'}</p><button className="button secondary" onClick={()=>retry(n=>n+1)}>{lang==='zh'?'重试':'Retry'}</button><button className="button secondary" onClick={()=>window.location.reload()}>{lang==='zh'?'刷新页面':'Reload page'}</button></section>;
  // Browsers can cache failed ES module fetches. After an explicit retry also
  // fails, reload the current URL once; initial failures never auto-reload.
  return <ChunkBoundary key={attempt} fallback={fallback} onError={attempt>0?()=>window.location.reload():undefined}><Suspense fallback={<Loading/>}><Page {...props}/></Suspense></ChunkBoundary>;
 }
}
