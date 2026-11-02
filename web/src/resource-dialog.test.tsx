// @vitest-environment jsdom
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen,within,waitFor,fireEvent} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {BrowserRouter} from 'react-router-dom';
import {StrictMode} from 'react';
import {ResourceList,ResourceManager} from './resources';
import {withHallFixture} from './hall-test-fixture';

let kind:'video'|'catalog'='video',untitled=false;
let play:ReturnType<typeof vi.spyOn>,pause:ReturnType<typeof vi.spyOn>;
beforeEach(()=>{play=vi.spyOn(HTMLMediaElement.prototype,'play').mockResolvedValue();pause=vi.spyOn(HTMLMediaElement.prototype,'pause').mockImplementation(()=>{})});
afterEach(()=>{vi.restoreAllMocks()});
beforeEach(()=>{untitled=false;localStorage.clear();sessionStorage.clear();vi.stubGlobal('fetch',withHallFixture(async()=>new Response(JSON.stringify({items:[{id:1,kind,vendor_id:3,status:'approved',revision_id:1,public_visible:true,data:{title:untitled?'':'示例资料',title_en:untitled?'':'Sample media',format:'images',purpose:'company',file_media_id:42,page_media_ids:kind==='catalog'?[43]:[],description:''}}],total:1,page:1,page_size:12}))))});
afterEach(()=>{cleanup();vi.unstubAllGlobals()});
it.each(['video','catalog'] as const)('opens a named %s dialog and restores focus on Escape',async value=>{
 kind=value;render(<BrowserRouter><ResourceList kind={kind}/></BrowserRouter>);
 const opener=await screen.findByRole('button',{name:/示例资料/});await userEvent.click(opener);
 const dialog=screen.getByRole('dialog',{name:'示例资料'}),heading=within(dialog).getByRole('heading',{name:'示例资料'});
 expect(dialog.getAttribute('aria-labelledby')).toBe(heading.id);expect(heading.id).not.toBe('');
 const close=within(dialog).getByRole('button',{name:'close'});expect(document.activeElement).toBe(close);
 await userEvent.keyboard('{Escape}');expect(screen.queryByRole('dialog')).toBeNull();expect(document.activeElement).toBe(opener);
});

it.each(['video','catalog'] as const)('keeps an untitled %s dialog named',async value=>{
 kind=value;untitled=true;render(<BrowserRouter><ResourceList kind={kind}/></BrowserRouter>);await screen.findByRole('button',{name:kind==='video'?/companyVideo/:/pageNumber/});await userEvent.click(document.querySelector<HTMLButtonElement>('.resource-card-trigger')!);
 expect(screen.getByRole('dialog',{name:kind==='video'?'shortVideos':'electronicCatalogs'})).toBeTruthy();
});

it('keeps the video cover and list title while opening exactly one popup player',async()=>{
 kind='video';render(<BrowserRouter><ResourceList kind="video"/></BrowserRouter>);
 const opener=await screen.findByRole('button',{name:/示例资料/});await userEvent.click(opener);
 const dialog=screen.getByRole('dialog',{name:'示例资料'});
 expect(screen.getAllByRole('dialog')).toHaveLength(1);expect(document.querySelectorAll('video')).toHaveLength(1);
 expect(document.querySelectorAll('.resource-card-cover')).toHaveLength(1);
 expect(within(dialog).getAllByRole('heading',{name:'示例资料'})).toHaveLength(1);
 expect(document.querySelector('.inline-video-region')).toBeNull();
 await userEvent.click(within(dialog).getByRole('button',{name:'close'}));
 expect(document.querySelector('video')).toBeNull();expect(document.activeElement).toBe(opener);
});

it('opens one official popup and preserves its cover and canonical autoplay=0',async()=>{
 const id='7690195495465012480';
 const item={id:2,kind:'video',vendor_id:3,public_visible:true,data:{title:'抖音示例',video_source:'DOUYIN',external_video_id:id,purpose:'company',page_media_ids:[]}};
 vi.stubGlobal('fetch',withHallFixture(async(url:string)=>new Response(JSON.stringify(url.includes('/playback')?{status:'READY',video_id:id,player_url:'https://open.douyin.com/player/video?vid='+id+'&autoplay=0',title:'抖音示例',width:1920,height:1080}:{items:[item],total:1}))));
 render(<BrowserRouter><ResourceList kind="video"/></BrowserRouter>);
 await userEvent.click(await screen.findByRole('button',{name:/抖音示例/}));
 await waitFor(()=>expect(document.querySelectorAll('iframe')).toHaveLength(1));
 const dialog=screen.getByRole('dialog',{name:'抖音示例'});
 expect(document.querySelectorAll('.resource-card-cover')).toHaveLength(1);expect(document.querySelector('.inline-video-region')).toBeNull();
 expect(within(dialog).getAllByRole('heading',{name:'抖音示例'})).toHaveLength(1);
 expect(within(dialog).getByText(/请点击官方播放器内的播放按钮/)).toBeTruthy();
 expect(document.querySelector('iframe')!.srcdoc).toContain('autoplay=0');expect(play).not.toHaveBeenCalled();
 await userEvent.click(within(dialog).getByRole('button',{name:'close'}));expect(document.querySelector('iframe')).toBeNull();
});

it('starts the local video after clicking its cover and keeps a poster while loading',async()=>{
 try{
  kind='video';render(<BrowserRouter><ResourceList kind="video"/></BrowserRouter>);
  const opener=await screen.findByRole('button',{name:/示例资料/});expect(play).not.toHaveBeenCalled();
  await userEvent.click(opener);
  await waitFor(()=>expect(play).toHaveBeenCalledTimes(1));
  const video=document.querySelector('video')!;
  expect(video.preload).toBe('metadata');expect(video.getAttribute('poster')).toBe('/api/v1/media/42?thumb=1');
  fireEvent.loadedMetadata(video);fireEvent.playing(video);
  expect(play).toHaveBeenCalledTimes(1);
  await userEvent.click(screen.getByRole('button',{name:'close'}));expect(pause).toHaveBeenCalled();
 }finally{cleanup()}
});

it('offers an explicit play button if the browser blocks the first play request',async()=>{
 play.mockRejectedValueOnce(new DOMException('Blocked','NotAllowedError')).mockResolvedValue();
 try{
  kind='video';render(<BrowserRouter><ResourceList kind="video"/></BrowserRouter>);
  await userEvent.click(await screen.findByRole('button',{name:/示例资料/}));
  await userEvent.click(await screen.findByRole('button',{name:'点击开始播放'}));
  expect(play).toHaveBeenCalledTimes(2);
 }finally{cleanup()}
});

it.each(['company','product','manager','editor'] as const)('requests one unmuted local play only after opening the %s popup',async entry=>{
 kind='video';const media={media_id:42,kind:'video',state:'ready',error:'',duration:3,size:100,page_count:0,width:1920,height:1080};
 const item={id:1,kind:'video',vendor_id:3,status:'draft',revision_id:1,public_visible:false,data:{title:'示例资料',title_en:'',description:'',description_en:'',format:'video',purpose:'company',product_id:0,file_media_id:42,cover_media_id:0,page_media_ids:[]},file:media};
 vi.stubGlobal('fetch',withHallFixture(async(url:string)=>url.includes('/media/42')?new Response(new Blob(['video'])):new Response(JSON.stringify(url.includes('/resource-media/')?media:url.includes('/contents?')?{items:[],total:0}:{items:[item],total:1}))));
 URL.createObjectURL=vi.fn(()=> 'blob:local-video');URL.revokeObjectURL=vi.fn();
 render(<BrowserRouter>{entry==='company'||entry==='product'?<ResourceList kind="video" productID={entry==='product'?7:undefined}/>:<ResourceManager kind="video"/>}</BrowserRouter>);
 if(entry==='editor')await userEvent.click(await screen.findByRole('button',{name:'edit'}));
 const opener=entry==='company'||entry==='product'?await screen.findByRole('button',{name:/示例资料/}):await screen.findByRole('button',{name:'resourcePreview'});
 expect(document.querySelector('video')).toBeNull();expect(play).not.toHaveBeenCalled();
 await userEvent.click(opener);const dialog=screen.getByRole('dialog',{name:'示例资料'});
 await waitFor(()=>expect(play).toHaveBeenCalledTimes(1));
 const video=dialog.querySelector('video')!;expect(video.muted).toBe(false);expect(video.defaultMuted).toBe(false);
 fireEvent.loadedMetadata(video);fireEvent.playing(video);expect(play).toHaveBeenCalledTimes(1);
 fireEvent(window,new Event('fullscreenchange'));expect(screen.getAllByRole('dialog')).toHaveLength(1);expect(play).toHaveBeenCalledTimes(1);
 await userEvent.click(within(dialog).getByRole('button',{name:'close'}));
 expect(document.querySelector('video')).toBeNull();expect(pause).toHaveBeenCalled();expect(document.activeElement).toBe(opener);
 if(entry==='manager'||entry==='editor')expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:local-video');
});

it('keeps the popup on fullscreen Escape and closes it on the next Escape',async()=>{
 kind='video';render(<BrowserRouter><ResourceList kind="video"/></BrowserRouter>);
 await userEvent.click(await screen.findByRole('button',{name:/示例资料/}));
 Object.defineProperty(document,'fullscreenElement',{configurable:true,value:document.querySelector('video')});
 await userEvent.keyboard('{Escape}');expect(screen.getByRole('dialog')).toBeTruthy();
 Object.defineProperty(document,'fullscreenElement',{configurable:true,value:null});fireEvent(document,new Event('fullscreenchange'));
 await userEvent.keyboard('{Escape}');expect(screen.queryByRole('dialog')).toBeNull();
});

it.each(['pagehide','hidden'] as const)('stops and unloads a local player on %s without resuming automatically',async event=>{
 kind='video';render(<BrowserRouter><ResourceList kind="video"/></BrowserRouter>);
 await userEvent.click(await screen.findByRole('button',{name:/示例资料/}));await waitFor(()=>expect(play).toHaveBeenCalledTimes(1));
 if(event==='hidden'){vi.spyOn(document,'hidden','get').mockReturnValue(true);fireEvent(document,new Event('visibilitychange'))}else fireEvent(window,new Event('pagehide'));
 expect(document.querySelector('video')).toBeNull();expect(pause).toHaveBeenCalled();
 fireEvent(document,new Event('visibilitychange'));expect(play).toHaveBeenCalledTimes(1);
 await userEvent.click(screen.getByRole('button',{name:'重试播放'}));await waitFor(()=>expect(play).toHaveBeenCalledTimes(2));
});

it('does not repeat the opening play request under StrictMode',async()=>{
 kind='video';render(<StrictMode><BrowserRouter><ResourceList kind="video"/></BrowserRouter></StrictMode>);
 await userEvent.click(await screen.findByRole('button',{name:/示例资料/}));await waitFor(()=>expect(play).toHaveBeenCalledTimes(1));
 expect(screen.getAllByRole('dialog')).toHaveLength(1);fireEvent.playing(document.querySelector('video')!);expect(play).toHaveBeenCalledTimes(1);
});
