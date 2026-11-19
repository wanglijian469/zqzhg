// @vitest-environment jsdom
import {beforeEach,afterEach,expect,it,vi} from 'vitest';
import {configureRuntime} from './runtime-config';
import {privateMediaAccess,uploadCloudFile} from './oss-upload';

const sent:{url:string;headers:Record<string,string>;bytes:number}[]=[];
class UploadXHR {
 url='';headers:Record<string,string>={};status=200;timeout=0;
 upload:{onprogress?:(e:{loaded:number})=>void}={};
 onload?:()=>void;onerror?:()=>void;ontimeout?:()=>void;onabort?:()=>void;
 open(_method:string,url:string){this.url=url}
 setRequestHeader(k:string,v:string){this.headers[k.toLowerCase()]=v}
 send(data:Blob){sent.push({url:this.url,headers:this.headers,bytes:data.size});this.upload.onprogress?.({loaded:data.size});queueMicrotask(()=>this.onload?.())}
 abort(){this.onabort?.()}
}
const json=(v:unknown)=>new Response(JSON.stringify(v),{status:200});
beforeEach(()=>{
 sent.length=0;sessionStorage.setItem('zqzt.merchant','SITE-PRIVATE-BEARER');
 configureRuntime({version:1,site:{name:'',name_en:'',web_url:'',api_url:'',media_url:''},uploads:{image_mib:10,video_mib:200,pdf_mib:50,default_quota_bytes:314572800},capabilities:{oss_direct_upload:true,oss_file_access:true}});
 vi.stubGlobal('XMLHttpRequest',UploadXHR);
});
afterEach(()=>{vi.unstubAllGlobals();sessionStorage.clear()});
it('sends exact file bytes to OSS without site credentials or forbidden headers',async()=>{
 const requests:string[]=[];
 vi.stubGlobal('fetch',vi.fn(async(url:string,options:RequestInit)=>{
  requests.push(url);
  if(url.endsWith('/uploads')){const req=JSON.parse(String(options.body));expect(req.size).toBe(6);expect(req.kind).toBe('image');return json({session:{id:'upload1',state:'uploading'},mode:'single',url:'https://upload.zqzhg.com/private-object?signature=temporary',headers:{'Content-Length':'6','Content-Type':'image/png',Host:'upload.zqzhg.com',Authorization:'FORBIDDEN','x-oss-forbid-overwrite':'true'}})}
  return json({session:{id:'upload1',state:'completed',media_id:42}});
 }));
 const progress:number[]=[];
 expect(await uploadCloudFile(new File(['abcdef'],'photo.png',{type:'image/png'}),'image','product_image','merchant',new AbortController().signal,n=>progress.push(n))).toBe(42);
 expect(sent).toEqual([{url:expect.stringContaining('https://upload.zqzhg.com/'),headers:{'content-type':'image/png','x-oss-forbid-overwrite':'true'},bytes:6}]);
 expect(progress.at(-1)).toBe(100);expect(requests.some(p=>p.endsWith('/complete'))).toBe(true);
 expect(Object.values(localStorage)).not.toContain(expect.stringContaining('signature='));
});
it('keeps site bearer off signed private download URLs',async()=>{
 vi.stubGlobal('fetch',vi.fn(async()=>json({url:'https://upload.zqzhg.com/file?signature=temporary',direct:true})));
 const access=await privateMediaAccess(42,'merchant');expect(access.headers).toEqual({});expect(access.url).toContain('upload.zqzhg.com');
});
it('does not start an OSS request when already cancelled',async()=>{
 const controller=new AbortController();controller.abort();vi.stubGlobal('fetch',vi.fn().mockRejectedValue(new DOMException('Cancelled','AbortError')));
 await expect(uploadCloudFile(new File(['abc'],'cancel.png',{type:'image/png'}),'image','product_image','merchant',controller.signal,()=>{})).rejects.toBeDefined();expect(sent).toHaveLength(0);
});
it('shows server processing progress and resumes after leaving without uploading or cancelling again',async()=>{
 const controller=new AbortController(),file=new File(['video data'],'continue-video.mp4',{type:'video/mp4',lastModified:123});let ready=false;const paths:string[]=[];const stages:string[]=[];
 vi.stubGlobal('fetch',vi.fn(async(url:string)=>{
  paths.push(url);
  if(url.endsWith('/uploads'))return json({session:{id:'continue-video',state:'uploading',expires_at:'2099-01-01'},mode:'single',url:'https://upload.zqzhg.com/test-video',headers:{'Content-Type':'video/mp4'}});
  if(url.endsWith('/complete'))return json({});
  return json({session:ready?{id:'continue-video',state:'completed',media_id:88,expires_at:'2099-01-01'}:{id:'continue-video',state:'validating',processing_stage:'processing_video',processing_percent:37,expires_at:'2099-01-01'}});
 }));
 await expect(uploadCloudFile(file,'video','','merchant',controller.signal,(_n,p)=>{if(p){stages.push(p.stage);expect(p.percent).toBe(37);controller.abort()}},'/merchant/resource-media?kind=video')).rejects.toMatchObject({name:'AbortError'});
 expect(stages).toEqual(['processing_video']);expect(paths.some(p=>p.endsWith('/cancel'))).toBe(false);ready=true;
 expect(await uploadCloudFile(file,'video','','merchant',new AbortController().signal,()=>{},'/merchant/resource-media?kind=video')).toBe(88);expect(sent).toHaveLength(1);expect(paths.filter(p=>p.endsWith('/uploads'))).toHaveLength(1);
});
