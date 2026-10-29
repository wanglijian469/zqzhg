import {api,body} from './api';
import {smsRequestID,type SMSProof} from './sms-challenge';
import type {BusinessAccount} from './procurement-api';

// All Web SMS sign-in uses purpose-bound codes and never registers an account.
export function requestSMSCode(phone:string,purpose:string,proof:SMSProof={},requestID=smsRequestID()){
 return api<{development_code?:string;retry_after?:number;status?:string}>('/auth/phone/code',{method:'POST',headers:{'Idempotency-Key':requestID},body:body({phone,purpose,...proof,request_id:requestID})});
}
export function requestLoginCode(phone:string,proof:SMSProof={},requestID?:string){return requestSMSCode(phone,'login',proof,requestID)}
export function loginWithCode(phone:string,code:string){
 return api<{token:string;account:BusinessAccount}>('/auth/phone/login',{method:'POST',body:body({phone,code})});
}
