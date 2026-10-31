// @vitest-environment jsdom
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen,waitFor,fireEvent} from '@testing-library/react';
import {BrowserRouter} from 'react-router-dom';
import {SearchEnginesAdmin} from './search-engines';
import {PlatformAccess} from './platform-access';
import {Language} from './ui';
import {configureRuntime} from './runtime-config';

const options={engines:{baidu:{enabled:true,automatic:false,daily_limit:100},indexnow:{enabled:false,automatic:false,daily_limit:200},google:{enabled:false,automatic:false,daily_limit:2}},verification:{},google_property:''};
const writes=vi.fn();
beforeEach(()=>{writes.mockReset();sessionStorage.clear();localStorage.clear();sessionStorage.setItem('zqzt.admin','fixture-session');configureRuntime({version:1,site:{api_url:'/api/v1',web_url:'',media_url:''},uploads:{}});vi.stubGlobal('fetch',vi.fn(async(input:string,request?:RequestInit)=>{const url=String(input);if(request?.method==='PUT'||request?.method==='POST'){writes(url,JSON.parse(request.body as string));return new Response(JSON.stringify(url.endsWith('/submit')?{validated_urls:1,has_more:false,next_cursor:0}:{}))}let data:unknown={};if(url.includes('/seo/settings'))data={version:1,options,configured:{baidu:true,indexnow:false,google:false},base_url:'https://seo.example',sitemap_url:'https://seo.example/sitemap.xml',indexnow_key_url:'https://seo.example/indexnow.txt'};else if(url.includes('/overview'))data={stats:[],quotas:[],public_counts:{vendor:11,product:46},day:'2026-10-31'};else if(url.includes('/jobs'))data={items:[],total:0,page:1,page_size:20};return new Response(JSON.stringify(data))}));});
afterEach(()=>{cleanup();vi.unstubAllGlobals();sessionStorage.clear();localStorage.clear()});
it('keeps provider credentials blank and out of browser storage while saving typed secrets only',async()=>{
 render(<BrowserRouter><SearchEnginesAdmin/></BrowserRouter>);const input=await screen.findByLabelText('API Token') as HTMLInputElement;expect(input.value).toBe('');expect(input.type).toBe('password');fireEvent.change(input,{target:{value:'typed-private-token'}});fireEvent.click(screen.getByRole('button',{name:'保存配置'}));await waitFor(()=>expect(writes).toHaveBeenCalled());const write=writes.mock.calls[0];expect(write[0]).toContain('/seo/settings');expect(write[1].secrets.baidu_token).toBe('typed-private-token');expect(JSON.stringify({...localStorage,...sessionStorage})).not.toContain('typed-private-token');await waitFor(()=>expect((screen.getByLabelText('API Token') as HTMLInputElement).value).toBe(''));
});
it('queues canonical inputs without claiming indexing and respects read-only SEO roles',async()=>{
 const view=render(<BrowserRouter><SearchEnginesAdmin/></BrowserRouter>);const input=await screen.findByLabelText('规范URL，每行一个，最多500个');fireEvent.change(input,{target:{value:'https://seo.example/v/company\nhttps://seo.example/v/company'}});fireEvent.click(screen.getByRole('button',{name:'核验并加入队列'}));await waitFor(()=>expect(writes).toHaveBeenCalled());expect(writes.mock.calls[0][1].urls).toEqual(['https://seo.example/v/company']);expect((await screen.findByRole('status')).textContent).toContain('入队不代表已发送或已收录');view.unmount();render(<PlatformAccess.Provider value={{role:'operator',permissions:['seo.read']}}><BrowserRouter><SearchEnginesAdmin/></BrowserRouter></PlatformAccess.Provider>);await screen.findByLabelText('API Token');expect(screen.getByRole('button',{name:'保存配置'}).matches(':disabled')).toBe(true);expect(screen.getByRole('button',{name:'处理下一任务（可能真实发送）'}).matches(':disabled')).toBe(true);
});
it('offers manual console paths for other engines and distinguishes IndexNow ownership from private tokens in English',async()=>{
 render(<Language.Provider value={{lang:'en',toggle:()=>{},t:k=>k}}><BrowserRouter><SearchEnginesAdmin/></BrowserRouter></Language.Provider>);await screen.findByText('Engines and automatic submission');expect(screen.getByText(/separate from private provider credentials/)).toBeTruthy();expect(screen.getByRole('link',{name:/360: use the official console/}).getAttribute('href')).toBe('https://zhanzhang.so.com/');expect(screen.getByRole('link',{name:/Sogou: manually/})).toBeTruthy();expect(screen.getByRole('link',{name:/Shenma: submit/})).toBeTruthy();expect(screen.getByText(/never uses the jobs\/livestream Indexing API/)).toBeTruthy();
});
