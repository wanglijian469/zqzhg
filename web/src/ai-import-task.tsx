import {useEffect,useRef,useState} from 'react';
import {Link} from 'react-router-dom';
import {api,APIError} from './api';
import {ErrorState,Loading,Modal} from './ui';
import {PublicationAgreements} from './service-center';
import {useAIImportResult} from './ai-import-result';
import {CompanyReview,ProductReview,CropEditor} from './ai-import-editors';
import {base,stages,aiMessages,errorText} from './ai-import-common';
import type {Result,Draft,BatchEdit,BatchResult} from './ai-import-types';

export function batchMissing(row:Draft,edit:BatchEdit,kind:'company'|'product',existing:boolean){
 const missing:string[]=[];const d=edit.data;
 if(!(kind==='company'&&existing&&!edit.selected_fields?.length)&&!d.name?.trim())missing.push('缺少名称');
 if(kind==='product'){
  if(!d.media_ids?.length)missing.push('缺少有效主图');
  if(!d.product_category_id&&!d.category_id&&!d.vendor_product_category_id)missing.push('未选择产品分类');
  if(edit.publish_mode==='undecided')missing.push('未选择更新或另建产品');
 }else if(!existing&&!d.primary_category_id)missing.push('未选择企业经营分类');
 const requires=!(kind==='company'&&existing&&!edit.selected_fields?.length)&&((row.review_issues?.length||0)>0||d.product_details?.models.some(m=>m.parameters.some(p=>p.review_required)));
 return {missing,requires};
}
export function TaskReview({id}:{id:number}){
 const result=useAIImportResult<Result>(base+'/tasks/'+id+'/result');const data=result.data;
 const [tab,setTab]=useState<'company'|'product'>('company');const [filter,setFilter]=useState('unsubmitted');const [page,setPage]=useState(1);
 const [selected,setSelected]=useState<number[]>([]);const [busy,setBusy]=useState(false);const [error,setError]=useState('');
 const [crop,setCrop]=useState(false);const [mergeName,setMergeName]=useState('');const [confirm,setConfirm]=useState<{kind:'company'|'product';items:BatchEdit[];key:string}>();
 const [resolution,setResolution]=useState('');const [response,setResponse]=useState<BatchResult>();
 const snapshots=useRef<Record<string,BatchEdit>>({});const pending=useRef<{fingerprint:string;key:string}|undefined>(undefined);
 useEffect(()=>{if(!data||!['queued','processing'].includes(data.task.status))return;const timer=setInterval(result.reload,2500);return()=>clearInterval(timer)},[data?.task.status,result.reload]);
 useEffect(()=>setPage(1),[filter]);
 const act=async(fn:()=>Promise<unknown>)=>{if(busy)return;setBusy(true);setError('');try{await fn();result.reload()}catch(e){setError(errorText(e))}finally{setBusy(false)}};
 if(!data)return result.error?<ErrorState code={aiMessages[result.error]||result.error} retry={result.reload}/>:<Loading/>;
 const task=data.task,working=['queued','processing'].includes(task.status),editable=['review_pending','completed'].includes(task.status);
 const filtered=data.products.filter(p=>filter==='all'||filter==='submitted'?filter==='all'||p.content_id>0:filter==='failed'?!!p.error&&!p.content_id:!p.content_id);
 const pages=Math.max(1,Math.ceil(filtered.length/5)),current=Math.min(page,pages),visible=new Set(filtered.slice((current-1)*5,current*5).map(p=>p.id));
 const chosen=data.products.filter(p=>selected.includes(p.id)&&!p.content_id);
 const editFor=(row:Draft,kind:'company'|'product'):BatchEdit=>snapshots.current[kind+row.id]||{id:row.id,version:row.version,data:row.data,resolution:'',selected_fields:row.selected_fields,publish_mode:row.publish_mode||'create',target_content_id:row.target_content_id,target_revision_id:kind==='company'?data.existing_company?.revision_id:row.target_product?.revision_id};
 const begin=(kind:'company'|'product')=>{
  const rows=kind==='company'?(data.company?[data.company]:[]):chosen;if(!rows.length)return;
  const items=rows.map(row=>({...editFor(row,kind)}));const fingerprint=JSON.stringify({kind,items});
  if(pending.current?.fingerprint!==fingerprint)pending.current={fingerprint,key:crypto.randomUUID()};
  setResolution('');setError('');setConfirm({kind,items,key:pending.current!.key});
 };
 const submit=async()=>{
  if(!confirm||busy)return;const request=confirm;const items=request.items.map(item=>({...item,resolution:item.resolution.trim()||resolution.trim()}));
  setBusy(true);setError('');try{
   const r=await api<BatchResult>(base+'/tasks/'+id+'/'+(request.kind==='company'?'company':'products')+'/submit',{method:'POST',body:JSON.stringify({idempotency_key:request.key,items}),signal:AbortSignal.timeout(10*60*1000)},'merchant');
   setResponse(r);setConfirm(undefined);if(r.failed===0)pending.current=undefined;
   setSelected(ids=>ids.filter(n=>!r.results.some(v=>v.id===n&&v.status==='submitted')));result.reload();
  }catch(e){setError(errorText(e));if((e as APIError).code==='stale_revision')result.reload()}finally{setBusy(false)}
 };
 const saveSelected=()=>act(async()=>{
  for(const row of chosen){const v=editFor(row,'product');const saved=await api<{version:number}>(base+'/products/'+row.id,{method:'PUT',body:JSON.stringify(v)},'merchant');snapshots.current['product'+row.id]={...v,version:saved.version}}
 });
 const allVisible=filtered.filter(p=>visible.has(p.id)&&!p.content_id&&!p.active_batch_id);
 const modalRows=confirm?.items.map(edit=>({edit,row:confirm.kind==='company'?data.company!:data.products.find(p=>p.id===edit.id)!})).filter(v=>!!v.row)||[];
 const checks=modalRows.map(({row,edit})=>({...batchMissing(row,edit,confirm!.kind,!!data.existing_company),name:edit.data.name,resolution:edit.resolution}));
 const unresolved=checks.some(v=>v.requires&&!v.resolution.trim()&&!resolution.trim());
 return <>
  <Link to="/merchant/ai-import">返回任务列表</Link>
  <section className="workspace-panel ai-task-summary"><h2>{task.file_name}</h2><p role="status">{working?'AI识别进行中':task.status==='failed'?'识别失败，可从已完成阶段重试':'识别完成，请分批核对提交'} · {task.progress}% · 共 {task.total} 页</p><progress max={100} value={task.progress}/>
   {task.work&&working&&<p>{task.work.message} {task.work.product||''} · 已完成 {task.work.done}／{task.work.total} 个步骤</p>}
   {task.error&&<p className="notice error">{aiMessages[task.error]||task.error}</p>}{task.status==='failed'&&<button onClick={()=>void act(()=>api(base+'/tasks/'+id+'/retry',{method:'POST',body:'{}'},'merchant'))}>从已完成阶段重试</button>}
   <p>识别完成不等于公开。您确认并提交后，仍须平台审核通过。</p>
   <details><summary>调用与用量记录</summary>{data.calls.map((c,i)=><p key={i}>{c.stage} · {c.error||'成功'} · Token {c.usage?.total_tokens??'未知'} · 估算费用 {c.estimated_cost==null?'未配置价格／未知':c.estimated_cost+'元'}</p>)}</details>
  </section>
  {!!data.batches?.length&&<details className="workspace-panel"><summary>提交批次记录（{data.batches.length}）</summary>{data.batches.map(batch=><div key={batch.id}><p>批次 {batch.id} · {batch.kind==='company'?'企业资料':batch.kind==='product'?'产品资料':'历史提交'} · {batch.status==='submitted'?'处理完成':batch.status==='partial_failed'?'有失败项':batch.status==='legacy'?'旧版提交记录':'等待处理'} · {batch.created_at}</p>{batch.kind!=='legacy'&&batch.status!=='submitted'&&<button disabled={busy} onClick={()=>void act(async()=>{const r=await api<BatchResult>(base+'/tasks/'+id+'/batches/'+batch.id+'/retry',{method:'POST',body:'{}',signal:AbortSignal.timeout(10*60*1000)},'merchant');setResponse(r)})}>重试批次 {batch.id} 的未完成项</button>}{batch.kind==='legacy'&&<p>历史成功项保持不变。未提交产品可在产品资料中编辑，并以新批次提交。</p>}</div>)}</details>}
  {result.error&&<p className="notice error">更新失败，当前编辑已保留。<button onClick={result.reload}>重新加载</button></p>}
  {error&&<p role="alert" className="notice error">{error}</p>}
  {response&&<section className="notice" role="status">{response.results.some(v=>v.status==='linked')?'已沿用现有企业资料，没有更新企业内容。':<>本批次已提交 {response.submitted} 项，失败 {response.failed} 项。已提交内容等待平台审核。</>}{response.results.filter(v=>v.error).map(v=><p key={v.id}>条目 {data.products.find(p=>p.id===v.id)?.data.name||v.id}：{aiMessages[v.error]||v.error}{v.fields?.map(f=>' · '+f.field).join('')}</p>)}</section>}
  <div role="tablist" className="ai-review-tabs" aria-label="AI识别资料"><button role="tab" aria-selected={tab==='company'} onClick={()=>setTab('company')}>企业资料</button><button role="tab" aria-selected={tab==='product'} onClick={()=>setTab('product')}>产品资料（{data.products.length}）</button></div>
  <section role="tabpanel" hidden={tab!=='company'} aria-label="企业资料">
   {data.company?<><CompanyReview key={'company'+data.company.version} draft={data.company} existing={data.existing_company} assets={data.assets} locked={busy||!editable||!!data.company.content_id||!!data.company.active_batch_id} reload={result.reload} onSnapshot={v=>{snapshots.current['company'+v.id]=v}}/>
    {data.company.content_id?<p className="notice">企业资料已处理，当前平台状态：{data.existing_company?.status==='pending'?'待审核':data.existing_company?.status==='approved'?'已通过':data.existing_company?.status||'已提交'}。<Link to="/merchant/booth">查看企业资料</Link></p>:<div className="ai-submit-bar"><PublicationAgreements/><button className="button" disabled={busy||!editable||!!data.company.active_batch_id} onClick={()=>begin('company')}>确认并提交企业资料</button></div>}
   </>:<p>企业资料尚未生成。</p>}
  </section>
  <section role="tabpanel" hidden={tab!=='product'} aria-label="产品资料">
   <section className="workspace-panel"><h2>分批处理产品</h2>{data.existing_company?<p>本批次关联企业：{data.existing_company.data.name}。提交产品不会更新企业资料。</p>:<p className="notice">请先在“企业资料”中单独提交企业，取得企业记录后再提交产品。</p>}
    <div className="ai-actions"><label>产品状态<select value={filter} onChange={e=>setFilter(e.target.value)}><option value="unsubmitted">未提交</option><option value="submitted">已提交</option><option value="failed">失败</option><option value="all">全部</option></select></label><button disabled={!allVisible.length||busy} onClick={()=>setSelected(ids=>Array.from(new Set([...ids,...allVisible.map(p=>p.id)])))}>选择当前页</button><button onClick={()=>setSelected([])}>清空选择</button><span>已提交 {data.products.filter(p=>p.content_id).length}／{data.products.length}，已公开 {data.products.filter(p=>p.public_visible).length}</span></div>
   </section>
   {data.products.map(p=><div key={p.id+':'+p.version+':'+p.reviewed_version} hidden={!visible.has(p.id)}><ProductReview draft={p} assets={data.assets} locked={busy||!editable||!!p.content_id||!!p.active_batch_id} selected={selected.includes(p.id)} onSelect={()=>setSelected(ids=>ids.includes(p.id)?ids.filter(n=>n!==p.id):[...ids,p.id])} reload={result.reload} onSnapshot={v=>{snapshots.current['product'+v.id]=v}}/></div>)}
   {!filtered.length&&<p className="notice">当前筛选下没有产品。</p>}
   <div className="ai-actions ai-pagination"><button disabled={current<=1} onClick={()=>setPage(current-1)}>上一页</button><span>第 {current}／{pages} 页，每页5项</span><button disabled={current>=pages} onClick={()=>setPage(current+1)}>下一页</button></div>
   <details className="workspace-panel"><summary>图片裁剪、合并产品</summary><div className="ai-actions"><button disabled={busy||!editable} onClick={()=>setCrop(true)}>从来源页裁剪图片／图纸</button><input aria-label="合并后产品名称" placeholder="合并后产品名称" value={mergeName} onChange={e=>setMergeName(e.target.value)}/><button disabled={busy||chosen.length<2||!mergeName.trim()} onClick={()=>void act(()=>api(base+'/tasks/'+id+'/products/merge',{method:'POST',body:JSON.stringify({name:mergeName,products:chosen.map(p=>({id:p.id,version:p.version}))})},'merchant'))}>合并所选产品</button></div></details>
   <div className="ai-submit-bar"><PublicationAgreements/><div className="ai-actions"><strong>已选择 {chosen.length} 项</strong><button disabled={busy||!chosen.length||!editable} onClick={()=>void saveSelected()}>保存草稿</button><button className="button" disabled={busy||!editable||!chosen.length||!data.existing_company} onClick={()=>begin('product')}>确认并提交所选产品</button></div></div>
  </section>
  {confirm&&<Modal close={()=>{if(!busy)setConfirm(undefined)}} closeDisabled={busy}><h2>{confirm.kind==='company'?'确认本次企业资料':'确认本批次产品资料'}</h2><p>只提交下列当前编辑版本；平台审核通过前不会公开。</p><ul>{checks.map((v,i)=><li key={i}><strong>{v.name}</strong>{v.missing.length?<p className="notice error">{v.missing.join('；')}</p>:<span> · 必填项初检通过</span>}</li>)}</ul>
   {checks.some(v=>v.requires)&&<label>本批次待确认事项的核对／修正说明<textarea value={resolution} onChange={e=>setResolution(e.target.value)} placeholder="请对照原始画册核对后填写，不自动代填确认。"/></label>}
   <PublicationAgreements/>{error&&<p role="alert" className="notice error">{error}</p>}<div className="ai-actions"><button disabled={busy} onClick={()=>setConfirm(undefined)}>返回修改</button><button className="button" disabled={busy||unresolved||checks.some(v=>v.missing.length>0)} onClick={()=>void submit()}>{busy?'正在保存并提交…':'确认内容并提交平台审核'}</button></div>
  </Modal>}
  {crop&&<Modal close={()=>setCrop(false)}><CropEditor task={task} pages={data.assets.filter(v=>v.image_type==='page')} onDone={()=>{setCrop(false);result.reload()}}/></Modal>}
 </>;
}
