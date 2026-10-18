import {apiURL,loadRuntime} from './runtime-config';
import {recordPublicResponse} from './analytics';
let publicRetryUntil=0;
export const publicRetrySeconds=()=>Math.max(0,Math.ceil((publicRetryUntil-Date.now())/1000));
export class APIError extends Error { constructor(public code: string, public fields:import("./form-guide").FieldIssue[]=[]) { super(code); } }
export function token(role: 'admin' | 'merchant') { return sessionStorage.getItem('zqzt.' + role) || ''; }
export async function api<T>(path: string, options: RequestInit = {}, role?: 'admin' | 'merchant'): Promise<T> {
 await loadRuntime();
 const protectedRead=/^\/public\/(?:vendors|products|showrooms|resources|home|favorites\/resolve)(?:\/|\?|$)/.test(path);
 if(!role&&protectedRead&&publicRetrySeconds()>0)throw new APIError('public_rate_limited');
 const headers = new Headers(options.headers); if (!(options.body instanceof FormData) && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json');
 if (role && token(role)) headers.set('Authorization', 'Bearer ' + token(role));
 let res: Response; try {res = await fetch(apiURL(path), {credentials:'include',...options, headers, signal: options.signal || AbortSignal.timeout(30000)});} catch {throw new APIError('network_error');}
 if (!res.ok) {if(!role&&protectedRead&&res.status===429){const delay=Number(res.headers.get('Retry-After')||60);publicRetryUntil=Date.now()+Math.min(3600,Math.max(1,Number.isFinite(delay)?delay:60))*1000;} if(role==='admin'&&res.status===401)window.dispatchEvent(new Event('zqzt.admin.expired'));if(role==='merchant'&&res.status===401)window.dispatchEvent(new Event('zqzt.merchant.expired'));const body = await res.json().catch(() => ({})); throw new APIError(!role&&res.status===429?'public_rate_limited':body.error?.code || 'internal_error',body.error?.fields||[]);} const result=await res.json();if(!role)recordPublicResponse(path,result);return result;
}
export const body = (data: unknown) => JSON.stringify(data);
