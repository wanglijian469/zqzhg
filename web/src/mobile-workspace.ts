import {useEffect,useState} from 'react';

declare global {
 interface Window {
  __DLNJ_NATIVE__?: boolean;
  __DLNJ_BACK__?: () => boolean;
  __DLNJ_TOGGLE_LANGUAGE__?: () => void;
 }
}
export const nativeWorkspace=()=>typeof window!=='undefined'&&window.__DLNJ_NATIVE__===true;
export function useMobileWorkspace(){
 const [mobile,setMobile]=useState(()=>nativeWorkspace()||typeof matchMedia==='function'&&matchMedia('(max-width: 767px)').matches);
 useEffect(()=>{if(nativeWorkspace()||typeof matchMedia!=='function')return;const mq=matchMedia('(max-width: 767px)');const change=()=>setMobile(mq.matches);mq.addEventListener('change',change);return()=>mq.removeEventListener('change',change)},[]);
 return mobile;
}
const backHandlers: (()=>boolean)[]=[];
export function useWorkspaceBack(handler:()=>boolean,enabled=true){
 useEffect(()=>{if(!enabled)return;backHandlers.push(handler);window.__DLNJ_BACK__=()=>[...backHandlers].reverse().some(fn=>fn());return()=>{const i=backHandlers.indexOf(handler);if(i>=0)backHandlers.splice(i,1)}},[handler,enabled]);
}
