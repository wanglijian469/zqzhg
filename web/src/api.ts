export class APIError extends Error { constructor(public code: string, public fields:import("./form-guide").FieldIssue[]=[]) { super(code); } }
export function token(role: 'admin' | 'merchant') { return sessionStorage.getItem('zqzt.' + role) || ''; }
export async function api<T>(path: string, options: RequestInit = {}, role?: 'admin' | 'merchant'): Promise<T> {
 const headers = new Headers(options.headers); if (!(options.body instanceof FormData)) headers.set('Content-Type', 'application/json');
 if (role && token(role)) headers.set('Authorization', 'Bearer ' + token(role));
 let res: Response; try {res = await fetch('/api/v1' + path, {...options, headers, signal: options.signal || AbortSignal.timeout(30000)});} catch {throw new APIError('network_error');}
 if (!res.ok) {if(role==='admin'&&res.status===401)window.dispatchEvent(new Event('zqzt.admin.expired'));if(role==='merchant'&&res.status===401)window.dispatchEvent(new Event('zqzt.merchant.expired'));const body = await res.json().catch(() => ({})); throw new APIError(body.error?.code || 'internal_error',body.error?.fields||[]);} return res.json();
}
export const body = (data: unknown) => JSON.stringify(data);
