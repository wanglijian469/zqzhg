// @vitest-environment jsdom
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen,waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {BrowserRouter} from 'react-router-dom';
import {App} from './main';
import {configureRuntime,runtimeConfig} from './runtime-config';
import {testHalls} from './hall-test-fixture';

let calls:string[]=[];
const categories=[{id:101,parent_id:0,hall_id:1,category_kind:'vendor',name:'耕整地机械及零部件',enabled:true},{id:102,parent_id:101,hall_id:1,category_kind:'vendor',name:'刀片类零部件',enabled:true}];
beforeEach(()=>{
 localStorage.clear();sessionStorage.clear();calls=[];configureRuntime({...runtimeConfig(),version:1,capabilities:{procurement:false}});
 vi.stubGlobal('scrollTo',vi.fn());vi.stubGlobal('fetch',vi.fn(async(url:string)=>{calls.push(url);const data=url.includes('/public/halls')?{items:testHalls}:url.includes('/taxonomy')?{categories,regions:[],aliases:[{source_id:9,target_id:102}]}:url.includes('/home')?{vendors:[],products:[],hero:{}}:{items:[],total:0,page:1,page_size:20};return new Response(JSON.stringify(data));}));
});
afterEach(()=>{cleanup();vi.unstubAllGlobals()});
function mount(path:string){history.replaceState(null,'',path);render(<BrowserRouter><App/></BrowserRouter>)}
it('opens the agriculture homepage directly and keeps the industry switcher available',async()=>{
 mount('/');await screen.findByRole('heading',{name:'中企展馆-农业机械厅'});
 expect(screen.queryByRole('heading',{name:'行业展厅'})).toBeNull();
 expect(document.querySelector('.industry-hall-grid')).toBeNull();
 expect(calls.some(p=>p.includes('/public/home'))).toBe(true);
 expect(screen.queryByRole('link',{name:'采购中心'})).toBeNull();
 expect(document.title).toBe('中企展馆-农业机械厅');
 expect(document.querySelector('link[rel="canonical"]')?.getAttribute('href')).not.toContain('hall=agriculture');
 expect(screen.getByRole('link',{name:'冀ICP备2026047360号-2'}).getAttribute('href')).toBe('https://beian.miit.gov.cn/');
});
it.each(['/buyer','/buyer/new?product=1','/procurement/1','/merchant/opportunities','/admin/procurement/reviews'])('pauses historical procurement route %s before any private request',async path=>{
 mount(path);await screen.findByRole('heading',{name:'功能暂时暂停'});
 expect(calls.some(p=>/\/(procurement|auth\/me)/.test(p))).toBe(false);
 expect(screen.getByRole('link',{name:'返回展馆总页'}).getAttribute('href')).toBe('/');
});
it('clears an ambiguous historical category before issuing a result request',async()=>{
 mount('/browse?category_id=999&page=3');await screen.findByText('原分类已更新，请重新选择类别。');
 await waitFor(()=>expect(calls.some(p=>p.includes('/public/vendors?'))).toBe(true));
 expect(calls.filter(p=>p.includes('/public/vendors?')).every(p=>!p.includes('category_id=999'))).toBe(true);
 expect(location.search).not.toContain('category_id');expect(location.search).not.toContain('page=');
});
it('resolves a unique historical category to its CIAME target',async()=>{
 mount('/browse?category_id=9');await waitFor(()=>expect(new URLSearchParams(location.search).get('category_id')).toBe('102'));
});
