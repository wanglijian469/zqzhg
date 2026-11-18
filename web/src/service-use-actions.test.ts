// @vitest-environment jsdom
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {api} from './api';
import {configureRuntime} from './runtime-config';
const refs=['terms','privacy','showroom-agreement'].map(key=>({key,version:2,digest:key+'-v2'}));
beforeEach(()=>{sessionStorage.setItem('zqzt.merchant','test-owner');configureRuntime({version:1,site:{name:'',name_en:'',web_url:'',api_url:'',media_url:''},uploads:{image_mib:10,video_mib:200,pdf_mib:50,default_quota_bytes:200000000}})});
afterEach(()=>{vi.unstubAllGlobals();sessionStorage.clear()});
it('sends current versions on a merchant submission without making a separate acceptance request',async()=>{const fetcher=vi.fn(async(url:string,_options?:RequestInit)=>new Response(JSON.stringify(url.endsWith('/public/service-documents')?{items:refs}:{ok:true})));vi.stubGlobal('fetch',fetcher);await api('/merchant/contents/4/submit',{method:'POST',body:JSON.stringify({revision_id:9})},'merchant');expect(fetcher).toHaveBeenCalledTimes(2);const options=fetcher.mock.calls[1][1] as RequestInit;expect(JSON.parse(String(options.body))).toMatchObject({revision_id:9,agreements:refs});expect(fetcher.mock.calls.some(([url])=>url.includes('service-acceptances'))).toBe(false)});
it('does not add acceptance to an ordinary draft save',async()=>{const fetcher=vi.fn(async()=>new Response('{}'));vi.stubGlobal('fetch',fetcher);await api('/merchant/contents/4/draft',{method:'PUT',body:JSON.stringify({revision_id:9,data:{name:'草稿'}})},'merchant');expect(fetcher).toHaveBeenCalledTimes(1)});
