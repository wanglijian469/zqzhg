import {useEffect,useRef,useState} from 'react';
import type {Data} from './types';
export function useAIEditableData(incoming:Data){
 const [data,setData]=useState(()=>structuredClone(incoming));const current=useRef(data);current.current=data;const baseline=useRef(JSON.stringify(incoming));const [conflict,setConflict]=useState(false);
 useEffect(()=>{const next=JSON.stringify(incoming);if(next===baseline.current)return;if(JSON.stringify(current.current)===baseline.current||JSON.stringify(current.current)===next){setData(structuredClone(incoming));baseline.current=next;setConflict(false)}else setConflict(true)},[incoming]);
 const adopt=()=>{setData(structuredClone(incoming));baseline.current=JSON.stringify(incoming);setConflict(false)};
 const keep=()=>{baseline.current=JSON.stringify(incoming);setConflict(false)};
 const incomingChanged=JSON.stringify(incoming)!==baseline.current&&JSON.stringify(data)!==baseline.current&&JSON.stringify(data)!==JSON.stringify(incoming);
 return {data,setData,conflict:conflict||incomingChanged,adopt,keep};
}
