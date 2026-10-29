import {useContext,useEffect,useRef} from 'react';
import {UNSAFE_NavigationContext,useLocation} from 'react-router-dom';

const popGuards=new Set<(event:PopStateEvent)=>void>();
// Register before BrowserRouter installs its history listener. A guard mounted
// later cannot rely on capture ordering for a popstate targeted at window:
// Router may unmount that editor before its listener gets called.
if(typeof window!=='undefined')window.addEventListener('popstate',event=>{
 const guard=Array.from(popGuards).at(-1);
 guard?.(event);
},true);

// BrowserRouter has no data-router blocker. Share the editors' link/unload
// protection, and restore a POP before asking so cancelled Back stays in place.
export function useUnsavedChanges(isDirty:()=>boolean,confirmLeave:()=>Promise<boolean>){
 const {navigator}=useContext(UNSAFE_NavigationContext),location=useLocation();
 const current=useRef({isDirty,confirmLeave});current.current={isDirty,confirmLeave};
 const index=useRef<number|undefined>(window.history.state?.idx);
 useEffect(()=>{index.current=window.history.state?.idx},[location.key]);
 useEffect(()=>{
  let active=true,bypass=false,asking=false;
  let restoring:{origin:number;delta:number;asking:boolean}|undefined;
  let allowedPop:number|undefined;
  const request=async(run:()=>void)=>{
   if(asking)return;
   asking=true;
   try{if(await current.current.confirmLeave()&&active){bypass=true;try{run()}finally{bypass=false}}}finally{asking=false}
  };
  const originalPush=navigator.push,originalReplace=navigator.replace;
  const push:typeof navigator.push=(...args)=>{
   if(bypass||!current.current.isDirty())originalPush.apply(navigator,args);
   else void request(()=>originalPush.apply(navigator,args));
  };
  const replace:typeof navigator.replace=(...args)=>{
   if(bypass||!current.current.isDirty())originalReplace.apply(navigator,args);
   else void request(()=>originalReplace.apply(navigator,args));
  };
  navigator.push=push;navigator.replace=replace;
  const leave=(event:BeforeUnloadEvent)=>{
   if(!bypass&&current.current.isDirty()){event.preventDefault();event.returnValue=''}
  };
  const click=(event:MouseEvent)=>{
   const anchor=(event.target as Element)?.closest?.('a[href]') as HTMLAnchorElement|null;
   if(!anchor||bypass||!current.current.isDirty()||event.defaultPrevented||event.button!==0||event.ctrlKey||event.metaKey||event.shiftKey||event.altKey||anchor.hasAttribute('download')||anchor.target&&anchor.target!=='_self')return;
   const target=new URL(anchor.href,window.location.href);
   if(!['http:','https:'].includes(target.protocol)||target.pathname===window.location.pathname&&target.search===window.location.search&&target.origin===window.location.origin)return;
   event.preventDefault();event.stopPropagation();
   void request(()=>anchor.click());
  };
  const pop=(event:PopStateEvent)=>{
   const next=event.state?.idx as number|undefined;
   if(restoring){
    event.stopImmediatePropagation();
    const transition=restoring;
    if(typeof next==='number'&&next!==transition.origin){window.history.go(transition.origin-next);return}
    if(transition.asking)return;
    transition.asking=true;
    void request(()=>{allowedPop=transition.origin+transition.delta;window.history.go(transition.delta)}).finally(()=>{restoring=undefined});
    return;
   }
   if(next===allowedPop&&typeof next==='number'){allowedPop=undefined;index.current=next;return}
   if(!current.current.isDirty()||typeof next!=='number'||typeof index.current!=='number'||next===index.current){index.current=next;return}
   event.stopImmediatePropagation();
   restoring={origin:index.current,delta:next-index.current,asking:false};
   window.history.go(-restoring.delta);
  };
  window.addEventListener('beforeunload',leave);
  document.addEventListener('click',click,true);
  popGuards.add(pop);
  return()=>{
   active=false;
   if(navigator.push===push)navigator.push=originalPush;
   if(navigator.replace===replace)navigator.replace=originalReplace;
   window.removeEventListener('beforeunload',leave);
   document.removeEventListener('click',click,true);
   popGuards.delete(pop);
  };
 },[navigator]);
}
