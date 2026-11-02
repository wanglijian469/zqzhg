// @vitest-environment jsdom
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen,waitFor,within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {OfficialVideo,VideoModal,VideoSourceFields,VideoAddChoice,douyinHostDocument,trustedPlayer,type VideoCheck} from './video-playback';
import {configureRuntime,runtimeConfig} from './runtime-config';
import {useState} from 'react';
import type {ResourceData} from './resources';
const check:VideoCheck={source:'DOUYIN',video_id:'7690195495465012480',canonical_url:'https://www.douyin.com/video/7690195495465012480',player_url:'https://open.douyin.com/player/video?vid=7690195495465012480&autoplay=0',title:'测试视频',width:1920,height:1080,status:'READY',reason:'',expires_at:'2099-01-01'};
beforeEach(()=>{configureRuntime({...runtimeConfig(),version:1,capabilities:{douyin_video:true}});vi.stubGlobal('fetch',vi.fn(async()=>new Response(JSON.stringify({original_share_url:check.canonical_url,check}))));});
afterEach(()=>{cleanup();vi.unstubAllGlobals()});
it('rejects forged player origins and autoplay',()=>{expect(trustedPlayer(check.player_url!,check.video_id)).toBeTruthy();for(const url of ['https://evil.test/player/video','javascript:alert(1)',check.player_url!.replace('autoplay=0','autoplay=1'),check.player_url!.replace('vid=7690195495465012480','vid=123')])expect(trustedPlayer(url,check.video_id)).toBe('')});
it('uses a restricted iframe and removes it on close',()=>{const r=render(<OfficialVideo check={check}/>);const f=document.querySelector('iframe')!;expect(f.getAttribute('sandbox')).toBe('allow-scripts allow-same-origin allow-presentation');expect(f.getAttribute('allow')).not.toContain('autoplay');expect(f.getAttribute('referrerpolicy')).toBe('origin');r.unmount();expect(document.querySelector('iframe')).toBeNull()});
it('a new video session unloads the previous iframe',async()=>{const r=render(<OfficialVideo check={check}/>);render(<OfficialVideo check={check}/>);await waitFor(()=>expect(document.querySelectorAll('iframe')).toHaveLength(1));r.unmount()});
it('invalidates link checks immediately when share content changes',async()=>{const onCheck=vi.fn();function Form(){const [data,setData]=useState<ResourceData>({title:'',title_en:'',description:'',description_en:'',purpose:'company',format:'video',product_id:0,file_media_id:0,cover_media_id:0,page_media_ids:[],video_source:'DOUYIN'});return <VideoSourceFields data={data} onChange={setData} onCheck={onCheck} role="merchant" disabled={false}/>};render(<Form/>);const field=screen.getByRole('textbox');await userEvent.type(field,check.canonical_url);await userEvent.click(screen.getByRole('button',{name:'检查链接'}));await waitFor(()=>expect(onCheck).toHaveBeenLastCalledWith(check));expect((screen.getByRole('button',{name:'预览播放'}) as HTMLButtonElement).disabled).toBe(false);expect(screen.queryByText('我确认该视频由本人发布，或已取得合法展示授权。')).toBeNull();await userEvent.type(field,'x');expect(onCheck).toHaveBeenLastCalledWith(undefined);expect((screen.getByRole('button',{name:'预览播放'}) as HTMLButtonElement).disabled).toBe(true)});
it('starts with neither add method selected and preserves local upload when disabled',()=>{configureRuntime({...runtimeConfig(),capabilities:{douyin_video:false}});render(<VideoAddChoice onChange={()=>{}}/>);expect(screen.getByRole('button',{name:'粘贴视频链接'}).getAttribute('aria-pressed')).toBe('false');expect((screen.getByRole('button',{name:'粘贴视频链接'}) as HTMLButtonElement).disabled).toBe(true);expect((screen.getByRole('button',{name:'上传本地视频'}) as HTMLButtonElement).disabled).toBe(false)});
it('limits the new policy to the playback host document',()=>{const html=douyinHostDocument(check.player_url!);expect(html).toContain("object-src 'none'");expect(html).toContain("base-uri 'none'");expect(html).toContain('frame-src https://open.douyin.com');expect(douyinHostDocument('https://evil.test/')).toBe('')});
it('opens checked preview in one dialog without playing on initial render, input or check',async()=>{
 function Form(){const [data,setData]=useState<ResourceData>({title:'',title_en:'',description:'',description_en:'',purpose:'company',format:'video',product_id:0,file_media_id:0,cover_media_id:0,page_media_ids:[],video_source:'DOUYIN'});return <VideoSourceFields data={data} onChange={setData} onCheck={()=>{}} role="merchant" disabled={false}/>}
 render(<Form/>);expect(document.querySelector('iframe')).toBeNull();
 await userEvent.type(screen.getByRole('textbox'),check.canonical_url);
 expect(fetch).not.toHaveBeenCalled();
 await userEvent.click(screen.getByRole('button',{name:'检查链接'}));
 expect(document.querySelector('iframe')).toBeNull();expect(screen.queryByRole('dialog')).toBeNull();
 const opener=screen.getByRole('button',{name:'预览播放'});await userEvent.click(opener);
 const dialog=screen.getByRole('dialog',{name:'测试视频'});expect(screen.getAllByRole('dialog')).toHaveLength(1);expect(document.querySelectorAll('iframe')).toHaveLength(1);
 expect(within(dialog).getByText(/请点击官方播放器内的播放按钮/)).toBeTruthy();expect(document.querySelector('.inline-video-region')).toBeNull();
 expect(document.querySelector('iframe')!.srcdoc).toContain('autoplay=0');
 await userEvent.keyboard('{Escape}');expect(document.querySelector('iframe')).toBeNull();expect(document.activeElement).toBe(opener);
});

it('closing a checked video preview never submits its surrounding edit form',async()=>{
 const submitted=vi.fn();
 function Form(){const [data,setData]=useState<ResourceData>({title:'',title_en:'',description:'',description_en:'',purpose:'company',format:'video',product_id:0,file_media_id:0,cover_media_id:0,page_media_ids:[],video_source:'DOUYIN',original_share_url:check.canonical_url});return <form onSubmit={e=>{e.preventDefault();submitted()}}><VideoSourceFields data={data} onChange={setData} onCheck={()=>{}} role="merchant" disabled={false}/></form>}
 render(<Form/>);await userEvent.click(screen.getByRole('button',{name:'检查链接'}));
 const opener=screen.getByRole('button',{name:'预览播放'});await userEvent.click(opener);
 await userEvent.click(within(screen.getByRole('dialog')).getByRole('button',{name:'close'}));
 expect(submitted).not.toHaveBeenCalled();expect(screen.queryByRole('dialog')).toBeNull();expect(document.activeElement).toBe(opener);
 expect((screen.getByRole('textbox') as HTMLTextAreaElement).value).toBe(check.canonical_url);
});

it('Escape exits fullscreen before closing the video dialog',async()=>{
 const close=vi.fn(),exit=vi.fn(async()=>{});
 Object.defineProperty(document,'fullscreenElement',{configurable:true,value:document.body});
 Object.defineProperty(document,'exitFullscreen',{configurable:true,value:exit});
 try {
  render(<VideoModal close={close}>{id=><h2 id={id}>全屏测试</h2>}</VideoModal>);
  await userEvent.keyboard('{Escape}');expect(exit).toHaveBeenCalledTimes(1);expect(close).not.toHaveBeenCalled();
 } finally { delete (document as unknown as Record<string,unknown>).fullscreenElement;delete (document as unknown as Record<string,unknown>).exitFullscreen; }
});
