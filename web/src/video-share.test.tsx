// @vitest-environment jsdom
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen,waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {BrowserRouter} from 'react-router-dom';
import {useState} from 'react';
import {withHallFixture} from './hall-test-fixture';
import {ResourceList,type ResourceData} from './resources';
import {ShareVideoActions} from './video-share';
import {externalVideoURL,VideoAddChoice,VideoSourceFields,videoAPIVersion,type VideoCheck} from './video-playback';
import {configureRuntime,runtimeConfig} from './runtime-config';

const url='https://weixin.qq.com/sph/Aht6MMIJPm';
beforeEach(()=>{localStorage.clear();sessionStorage.clear();configureRuntime({...runtimeConfig(),capabilities:{video_api_version:4,douyin_video:true}})});
afterEach(()=>{cleanup();vi.restoreAllMocks();vi.unstubAllGlobals()});

it('negotiates v4 and offers four sources while older backends retain their choices',()=>{
 const select=vi.fn();render(<VideoAddChoice onChange={select}/>);
 expect(videoAPIVersion()).toBe('4');expect(screen.getByRole('button',{name:'微信视频号分享链接'})).toBeTruthy();expect(screen.getByRole('button',{name:'快手分享链接'})).toBeTruthy();
 cleanup();configureRuntime({...runtimeConfig(),capabilities:{video_api_version:3,douyin_video:true}});render(<VideoAddChoice onChange={select}/>);
 expect(videoAPIVersion()).toBe('3');expect(screen.queryByRole('button',{name:'微信视频号分享链接'})).toBeNull();expect(screen.getByRole('button',{name:'快手分享链接'})).toBeTruthy();
});
it('validates the canonical WeChat URL without accepting another origin or encoded path',()=>{
 expect(externalVideoURL(url,'WECHAT_CHANNELS')).toBe(url);
 for(const raw of [url.replace('weixin.qq.com','weixin.qq.com.evil.test'),url.replace('https:','http:'),url+'?track=1',url+'#x',url.replace('/sph/A','/sph/%41'),url.replace('.com/','.com:443/'),'javascript:alert(1)'])expect(externalVideoURL(raw,'WECHAT_CHANNELS')).toBe('');
 expect(externalVideoURL('https://v.kuaishou.com/Abc123','KUAISHOU')).toBe('https://v.kuaishou.com/Abc123');
});
it.each(['KUAISHOU','WECHAT_CHANNELS'] as const)('opens %s directly without requesting a player or rendering a video modal',async source=>{
 const link=source==='KUAISHOU'?'https://v.kuaishou.com/Abc123':url;const calls:string[]=[];
 vi.stubGlobal('fetch',withHallFixture(async(raw:string)=>{calls.push(raw);return new Response(JSON.stringify({items:[{id:8,kind:'video',vendor_id:3,data:{title:'企业分享视频',description:'原平台观看',video_source:source,canonical_video_url:link,purpose:'company',file_media_id:0,page_media_ids:[]}}],total:1,page_size:12}))}));
 render(<BrowserRouter><ResourceList kind="video"/></BrowserRouter>);
 const watch=await screen.findByRole('link',{name:source==='KUAISHOU'?'前往快手观看':'前往视频号观看'});
 expect(watch.getAttribute('href')).toBe(link);expect(watch.getAttribute('target')).toBe('_blank');expect(watch.getAttribute('rel')).toContain('noopener');
 watch.addEventListener('click',e=>e.preventDefault());await userEvent.click(watch);
 expect(screen.queryByRole('dialog')).toBeNull();expect(document.querySelector('video,iframe')).toBeNull();expect(calls.some(p=>p.includes('/playback')||p.includes('resource-media')||p.includes('resolve'))).toBe(false);
 expect(screen.getByRole('button',{name:'复制链接'})).toBeTruthy();
});
it('copies the original link and offers manual copying if the clipboard is unavailable',async()=>{
 const user=userEvent.setup();const write=vi.spyOn(navigator.clipboard,'writeText').mockResolvedValue();render(<ShareVideoActions source="WECHAT_CHANNELS" url={url}/>);
 await user.click(screen.getByRole('button',{name:'复制链接'}));expect(write).toHaveBeenCalledWith(url);expect(screen.getByRole('status').textContent).toBe('链接已复制');
 write.mockRejectedValueOnce(new Error('clipboard denied'));await user.click(screen.getByRole('button',{name:'复制链接'}));expect((screen.getByRole('textbox',{name:'视频链接'}) as HTMLInputElement).value).toBe(url);
});
it('checks the WeChat format using v4 and invalidates it after the user changes the link',async()=>{
 const calls:{path:string;data:unknown}[]=[];let latest:VideoCheck|undefined;
 vi.stubGlobal('fetch',vi.fn(async(path:string,init:RequestInit)=>{calls.push({path,data:init?.body?JSON.parse(String(init.body)):null});return new Response(JSON.stringify(path.includes('video-links/check')?{original_share_url:url,check:{source:'WECHAT_CHANNELS',status:'EXTERNAL_ONLY',video_id:'sph:Aht6MMIJPm',canonical_url:url}}:{items:[]}))}));
 function Form(){const [data,setData]=useState<ResourceData>({title:'',title_en:'',description:'',description_en:'',purpose:'company',format:'video',product_id:0,file_media_id:0,cover_media_id:0,page_media_ids:[],video_source:'WECHAT_CHANNELS'});return <VideoSourceFields data={data} onChange={setData} onCheck={v=>{latest=v}} role="merchant" disabled={false}/>}
 render(<Form/>);await userEvent.type(screen.getByRole('textbox'),url);await userEvent.click(screen.getByRole('button',{name:'检查链接'}));
 await waitFor(()=>expect(latest?.status).toBe('EXTERNAL_ONLY'));expect(calls.find(c=>c.path.includes('video-links/check'))).toEqual({path:'/api/v1/merchant/video-links/check?video_api_version=4',data:{text:url,source:'WECHAT_CHANNELS'}});
 expect(screen.getByRole('status').textContent).toContain('审核后访客可前往原平台观看');expect(document.querySelector('input[type=file]')).toBeNull();
 await userEvent.type(screen.getByRole('textbox'),'x');expect(latest).toBeUndefined();
});
