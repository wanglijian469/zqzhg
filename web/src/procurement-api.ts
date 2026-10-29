import {api,body,token} from './api';
import {apiURL} from './runtime-config';
export type ProcRole='admin'|'merchant';
export const procRoot=(admin=false)=>admin?'/admin/procurement':'/procurement';
export const operationID=()=>{if(typeof crypto.randomUUID==='function')return crypto.randomUUID();const data=new Uint8Array(24);crypto.getRandomValues(data);return Array.from(data).map(v=>v.toString(16).padStart(2,'0')).join('')};
export async function procWrite<T>(path:string,value:unknown,method='POST',admin=false,explicitKey?:string):Promise<T>{
 const input=body(value);const encoded=new TextEncoder().encode(method+path+input);let hash:string;if(crypto.subtle){const digest=await crypto.subtle.digest('SHA-256',encoded);hash=Array.from(new Uint8Array(digest)).map(n=>n.toString(16).padStart(2,'0')).join('')}else{let h=1469598103934665603n;for(const n of encoded)h=BigInt.asUintN(64,(h^BigInt(n))*1099511628211n);hash=h.toString(16)+'-'+encoded.length}
 const storageKey='zqzt.proc.operation.'+(admin?'admin':sessionStorage.getItem('zqzt.merchant.id')||'business')+'.'+hash;
 const key=explicitKey||sessionStorage.getItem(storageKey)||operationID();sessionStorage.setItem(storageKey,key);
 try{const result=await api<T>(path,{method,body:input,headers:{'Idempotency-Key':key}},admin?'admin':'merchant');sessionStorage.removeItem(storageKey);return result}
 catch(error){if((error as {code?:string}).code!=='network_error')sessionStorage.removeItem(storageKey);throw error}
}
export interface BusinessAccount{id:number;role:string;phone?:string;phone_verified:boolean;buyer_registered?:boolean;capabilities?:{buyer:boolean;supplier:boolean;procurement:boolean}}
export interface BuyerProfile{account_id:number;company:string;contact_name:string;region_code:string;version:number;suspended:boolean}
export interface ProcItem{id:number;category_id:number;name:string;specification:string;quantity:string;unit:string}
export interface ProcFile{id:number;name:string;content_type:string;size:number}
export interface ProcRevision{requested_audience?:string;id:number;status:string;title:string;delivery_region:string;expected_delivery:string;deadline:string;notes:string}
export interface ProcParticipant{id:number;vendor_id:number;source:string;state:string;intent:boolean;match_reason:string}
export interface ProcReview{id:number;project_id:number;revision_id:number;action:string;audience:string;status:string;reason:string;created_at:string}
export interface ProcProject{requested_vendor_ids?:number[];id:number;hall_id:number;state:string;audience:string;version:number;suspended:boolean;published_revision_id:number;draft_revision_id?:number;source_product_id?:number;source_vendor_id?:number;revision:ProcRevision;items:ProcItem[];attachments:ProcFile[];owner:boolean;historical?:boolean;versions?:ProcRevision[];contact?:{name:string;phone:string};participants?:ProcParticipant[];participant?:ProcParticipant;reviews?:ProcReview[];events?:{id:number;action:string;created_at:string}[]}
export interface ProcQuoteLine{item_id:number;status:'quoted'|'unavailable'|'unquoted';unit_price:string;minimum_quantity:string;delivery_days:number;notes:string}
export interface ProcQuoteVersion{revision:{id:number;demand_revision_id:number;status:string;tax_mode:string;freight_mode:string;freight:string;valid_until:string;notes:string};items:ProcQuoteLine[];demand_items:ProcItem[];attachments:ProcFile[];stale:boolean;expired:boolean;comparable_total:string|null}
export interface ProcQuote{quote:{id:number;vendor_id:number;version:number;submitted_revision_id:number;draft_revision_id:number;withdrawn:boolean;excluded:boolean};vendor_name:string;history:ProcQuoteVersion[]}
export interface ProcPage<T>{items:T[];total:number;page:number;page_size:number}
export const procError=(error:unknown)=>error instanceof Error?(error as Error&{code?:string}).code||error.message:'internal_error';
const errors:Record<string,[string,string]>={proc_disabled:['采购询比价尚未启用','Procurement is not enabled'],proc_invalid:['请检查必填项、分类、数量及截止时间','Check required fields, category, quantity and deadline'],proc_conflict:['资料已更新，请重新读取后再操作','This record changed. Reload before continuing'],proc_register_buyer:['请先登记采购商资料','Register your buyer profile first'],proc_suspended:['当前采购操作已被平台暂停','Procurement access is suspended'],proc_not_open:['项目未开放或已经结束','The project is not open'],proc_expired:['报价截止时间已过','The quotation deadline has passed'],proc_review_required:['邀请其他厂商前需先提交扩散审核','Review is required before inviting other suppliers'],proc_supplier_required:['请使用已审核的厂商账号参与','An approved supplier account is required'],proc_no_match:['当前厂商的产品分类或供货范围不符合需求','Your categories or delivery coverage do not match'],proc_category_invalid:['请选择有效的产品分类','Select an available product category'],proc_self_inquiry:['不能向自己所属厂商询价','You cannot inquire with your own company'],proc_demand_changed:['采购需求已变更，请读取新版后重新报价','Reload the updated demand before quoting'],proc_quote_empty:['至少填写一项有效报价','Quote at least one item'],proc_invite_limit:['本项目邀请已达到100家上限','This project has reached its invitation limit'],proc_operation_required:['请刷新页面后重试','Refresh and try again']};
export function procurementErrorText(code:string,en:boolean,t:(s:string)=>string){return errors[code]?.[en?1:0]||t(code)}
export async function openProcFile(file:ProcFile,admin=false,preview=false){
 const role:ProcRole=admin?'admin':'merchant';const response=await fetch(apiURL(procRoot(admin)+'/attachments/'+file.id+(preview?'?preview=1':'')),{headers:{Authorization:'Bearer '+token(role)}});
 if(!response.ok)throw new Error('not_found');const blob=await response.blob();const url=URL.createObjectURL(blob);const a=document.createElement('a');a.href=url;if(preview){a.target='_blank';a.rel='noopener'}else a.download=file.name;a.click();setTimeout(()=>URL.revokeObjectURL(url),60000);
}

export type ProcProjectSummary=Pick<ProcProject,"id"|"hall_id"|"state"|"audience"> & {revision:Pick<ProcRevision,"id"|"status"|"title"|"delivery_region"|"deadline">;items:Omit<ProcItem,"specification">[];participant?:Pick<ProcParticipant,"state"|"intent">};
