import {beforeEach,expect,it,vi} from 'vitest';
vi.mock('./api',()=>({api:vi.fn(),body:JSON.stringify,token:()=>''}));
import {api} from './api';
import {procWrite} from './procurement-api';
beforeEach(()=>{const values=new Map<string,string>();vi.stubGlobal('sessionStorage',{getItem:(key:string)=>values.get(key)||null,setItem:(key:string,value:string)=>values.set(key,value),removeItem:(key:string)=>values.delete(key)});vi.mocked(api).mockReset()});
it('reuses the same operation key after an ambiguous network failure',async()=>{
 const failure=Object.assign(new Error('network_error'),{code:'network_error'});vi.mocked(api).mockRejectedValueOnce(failure).mockResolvedValueOnce({id:1});
 await expect(procWrite('/procurement/projects',{title:'test'})).rejects.toThrow('network_error');await procWrite('/procurement/projects',{title:'test'});
 const first=vi.mocked(api).mock.calls[0][1]!.headers as Record<string,string>,retry=vi.mocked(api).mock.calls[1][1]!.headers as Record<string,string>;expect(first['Idempotency-Key']).toBe(retry['Idempotency-Key']);
});
it('preserves decimal strings and scopes pending operations to the account',async()=>{
 vi.mocked(api).mockRejectedValue(Object.assign(new Error('network_error'),{code:'network_error'}));sessionStorage.setItem('zqzt.merchant.id','7');const payload={items:[{unit_price:'12.3456',quantity:'2.500'}]};await procWrite('/procurement/projects/1/quotes',payload).catch(()=>{});sessionStorage.setItem('zqzt.merchant.id','8');await procWrite('/procurement/projects/1/quotes',payload).catch(()=>{});
 expect(JSON.parse(vi.mocked(api).mock.calls[0][1]!.body as string)).toEqual(payload);expect((vi.mocked(api).mock.calls[0][1]!.headers as Record<string,string>)['Idempotency-Key']).not.toBe((vi.mocked(api).mock.calls[1][1]!.headers as Record<string,string>)['Idempotency-Key']);
});
