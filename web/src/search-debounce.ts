import {useEffect,useRef,useState} from 'react';
export function useDebouncedSearch(value:string,committed:string,commit:(value:string,replace:boolean)=>void,scope:string,enabled=true){
 const callback=useRef(commit);callback.current=commit;const timer=useRef<ReturnType<typeof setTimeout>|undefined>(undefined);const composing=useRef(false);const [composition,setComposition]=useState(false);
 const cancel=()=>{clearTimeout(timer.current);timer.current=undefined};
 useEffect(()=>{cancel();if(enabled&&!composition&&value.trim()!==committed.trim())timer.current=setTimeout(()=>{if(!composing.current)callback.current(value.trim(),true)},350);return cancel},[value,committed,scope,enabled,composition]);
 return{cancel,waiting:enabled&&(composition||value.trim()!==committed.trim()),submit:()=>{cancel();if(!composing.current)callback.current(value.trim(),false)},onCompositionStart:()=>{composing.current=true;cancel();setComposition(true)},onCompositionEnd:()=>{composing.current=false;setComposition(false)}};
}
