import {useEffect,useRef,useState} from 'react';
import {api,APIError,body} from './api';
import {ContentInfo,display,MediaImage,Modal,useLanguage} from './ui';
import type {Item} from './types';

type Decision='approve'|'reject';
type Result={item:Item;ok:boolean;code?:string};
export function ReviewCheckbox({checked,partial=false,disabled,label,onChange}:{checked:boolean;partial?:boolean;disabled?:boolean;label:string;onChange:()=>void}){
 const ref=useRef<HTMLInputElement>(null);useEffect(()=>{if(ref.current)ref.current.indeterminate=partial},[partial]);
 return <input ref={ref} type="checkbox" className="review-checkbox" checked={checked} disabled={disabled} aria-label={label} onChange={onChange}/>;
}
export function useBatchReview(items:Item[]|undefined,scope:string,onChanged:()=>void){
 const {t,lang}=useLanguage();const [selection,setSelection]=useState<{scope:string;ids:number[]}>({scope,ids:[]});
 const eligible=(items||[]).filter(i=>i.status==='pending');const selected=eligible.filter(i=>selection.scope===scope&&selection.ids.includes(i.revision_id));
 const [request,setRequest]=useState<{items:Item[];decision:Decision}>();const [note,setNote]=useState('');const [error,setError]=useState('');const [results,setResults]=useState<Result[]>();
 const [busy,setBusy]=useState(false);const [progress,setProgress]=useState(0);const processing=useRef(false);const mounted=useRef(true);
 const [expanded,setExpanded]=useState<number[]>([]);
 useEffect(()=>{mounted.current=true;return()=>{mounted.current=false}},[]);
 useEffect(()=>{setSelection({scope,ids:[]});if(!processing.current)setRequest(undefined)},[scope]);
 const toggle=(item:Item)=>{if(processing.current||item.status!=='pending')return;setSelection(old=>{const ids=old.scope===scope?old.ids:[];return{scope,ids:ids.includes(item.revision_id)?ids.filter(id=>id!==item.revision_id):[...ids,item.revision_id]}})};
 const selectPage=()=>{if(!processing.current)setSelection({scope,ids:selected.length===eligible.length?[]:eligible.map(i=>i.revision_id)})};
 const clear=()=>{if(!processing.current)setSelection({scope,ids:[]})};
 const open=(decision:Decision,targets=selected)=>{if(processing.current||!targets.length)return;setRequest({decision,items:[...targets]});setNote('');setError('');setResults(undefined);setExpanded([]);setProgress(0)};
 const close=()=>{if(!processing.current)setRequest(undefined)};
 const execute=async()=>{
  if(processing.current||!request||results)return;
  if(request.decision==='reject'&&!note.trim()){setError('rejection_reason_required');return}
  if(Array.from(note).length>1000){setError('reviewNoteLimit');return}
  processing.current=true;setBusy(true);setError('');const out:Result[]=[];
  for(let index=0;index<request.items.length;index++){
   if(!mounted.current)break;const item=request.items[index];let stop=false;
   try{await api('/admin/revisions/'+item.revision_id+'/review',{method:'POST',body:body({decision:request.decision,note})},'admin');out.push({item,ok:true})}
   catch(e){const code=(e as APIError).code||'network_error';out.push({item,ok:false,code});stop=['network_error','unauthorized','forbidden'].includes(code)}
   if(mounted.current)setProgress(index+1);
   if(stop){for(const rest of request.items.slice(index+1))out.push({item:rest,ok:false,code:'reviewNotProcessed'});break}
  }
  processing.current=false;if(!mounted.current)return;setBusy(false);setResults(out);clear();onChanged();
 };
 const modal=request&&<Modal close={close} closeDisabled={busy}><div className="batch-review-dialog"><h2>{t(request.items.length===1?(request.decision==='approve'?'approve':'reject'):(request.decision==='approve'?'batchApprove':'batchReject'))} · {request.items.length} {t('records')}</h2>
 {results?<><p className="notice" role="status">{t('reviewSucceeded')} {results.filter(r=>r.ok).length} · {t('reviewIncomplete')} {results.filter(r=>!r.ok).length}</p><ul className="batch-review-results">{results.map(r=><li key={r.item.revision_id}><strong>{display(r.item.data,lang,'name')} <small>#{r.item.id}</small></strong><span className={r.ok?'review-result-ok':'review-result-error'}>{r.ok?t(request.decision==='approve'?'approved':'rejected'):t(r.code==='network_error'?'reviewUnconfirmed':r.code||'internal_error')}</span></li>)}</ul>{results.some(r=>!r.ok)&&<p className="muted">{t('reviewRefreshHint')}</p>}<div className="form-actions"><button className="button secondary" onClick={onChanged}>{t('refreshReviewList')}</button><button className="button" onClick={close}>{t('done')}</button></div></>:<>
 <p className="notice">{t(request.decision==='approve'?'batchApproveHint':'batchRejectHint')}</p><p className="muted">{t('batchVersionHint')}</p>
 <div className="batch-review-items">{request.items.map(item=><article className="batch-review-item" key={item.revision_id}><div className="batch-review-summary">{item.data.media_ids?.[0]?<MediaImage id={item.data.media_ids[0]} role="admin"/>:null}<div><strong>{display(item.data,lang,'name')}</strong><small>#{item.id} · {item.kind==='product'?(item.vendor?.name||'#'+item.vendor_id):item.data.region_name}</small><p>{item.kind==='vendor'?item.data.main_products:item.data.description}</p></div><button type="button" className="text-link" disabled={busy} onClick={()=>setExpanded(ids=>ids.includes(item.revision_id)?ids.filter(id=>id!==item.revision_id):[...ids,item.revision_id])}>{t(expanded.includes(item.revision_id)?'collapseReviewContent':'inspectReviewContent')}</button></div>{expanded.includes(item.revision_id)&&<div className="batch-review-preview"><ContentInfo item={item} role="admin"/></div>}</article>)}</div>
 <label>{t(request.decision==='reject'?'batchRejectReason':'reviewOptionalNote')}<textarea value={note} disabled={busy} aria-label={t(request.decision==='reject'?'batchRejectReason':'reviewOptionalNote')} rows={3} onChange={e=>{setNote(e.target.value);setError('')}}/><small className="field-guide">{Array.from(note).length}/1000 · {t('batchNoteHint')}</small></label>
 {error&&<p className="notice error" role="alert">{t(error)}</p>}{busy&&<p role="status">{t('reviewProcessing')} {progress}/{request.items.length}</p>}
 <div className="form-actions"><button type="button" className="button secondary" disabled={busy} onClick={close}>{t('cancel')}</button><button type="button" className={'button '+(request.decision==='reject'?'danger':'')} disabled={busy||(request.decision==='reject'&&!note.trim())||Array.from(note).length>1000} onClick={()=>void execute()}>{busy?t('reviewProcessing'):t(request.decision==='approve'?'confirmBatchApprove':'confirmBatchReject')}</button></div></>}
 </div></Modal>;
 const toolbar=<div className="batch-review-toolbar"><div><strong>{t('batchReview')}</strong><span>{t('selectedOption')} {selected.length} {t('records')}</span><small>{t('batchSelectionHint')}</small></div><div className="form-actions"><button className="button secondary" disabled={busy||!eligible.length} onClick={selectPage}>{t(selected.length===eligible.length&&eligible.length?'clearSelection':'selectPendingPage')}</button><button className="button" disabled={busy||!selected.length} onClick={()=>open('approve')}>{t('batchApprove')}</button><button className="button secondary danger" disabled={busy||!selected.length} onClick={()=>open('reject')}>{t('batchReject')}</button>{selected.length>0&&selected.length!==eligible.length&&<button className="text-link" disabled={busy} onClick={clear}>{t('clearSelection')}</button>}</div></div>;
 return {selected,eligible,toggle,selectPage,open,busy,modal,toolbar};
}
