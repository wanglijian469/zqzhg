// @vitest-environment jsdom
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen,within,waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {BrowserRouter} from 'react-router-dom';
import {App} from './main';
import {withHallFixture} from './hall-test-fixture';
import {configureRuntime,runtimeConfig} from './runtime-config';

const summary={id:9,hall_id:1,title:'采购旋耕机和配件的长标题展示示例',state:'open',delivery_region:'130528',deadline:'2026-11-20T08:00:00Z',published_at:'2026-10-24T08:00:00Z',items:[{name:'旋耕机',category_id:101,category_name:'耕整地机械',quantity:'2.5',unit:'台'}]};
let calls:{path:string;method:string}[];
beforeEach(()=>{
 calls=[];localStorage.clear();sessionStorage.clear();window.__DLNJ_NATIVE__=true;
 vi.stubGlobal('scrollTo',vi.fn());
 configureRuntime({...runtimeConfig(),capabilities:{procurement:true,procurement_public_summary:true}});
 vi.stubGlobal('fetch',withHallFixture(vi.fn(async(raw:string,init?:RequestInit)=>{
  const u=new URL(raw,'http://localhost');calls.push({path:u.pathname+u.search,method:init?.method||'GET'});
  const payload=u.pathname.endsWith('/procurements/9')?summary:u.pathname.endsWith('/procurements')?{items:[summary],total:1,page:1,page_size:20}:u.pathname.endsWith('/taxonomy')?{categories:[{id:101,hall_id:1,name:'耕整地机械',name_en:'Tillage equipment',enabled:true,parent_id:0}]}:u.pathname.endsWith('/administrative-regions')?(u.searchParams.has('code')?{selected:{full_name:'河北省邢台市宁晋县'}}:{items:u.searchParams.get('parent_code')==='13'?[{code:'1305',name:'邢台市'}]:u.searchParams.get('parent_code')==='1305'?[{code:'130528',name:'宁晋县'}]:[{code:'13',name:'河北省'}]}):{items:[],total:0};
  return new Response(JSON.stringify(payload),{headers:{'Content-Type':'application/json'}});
 })));
 history.replaceState(null,'','/procurement');
});
afterEach(()=>{cleanup();delete window.__DLNJ_NATIVE__;delete window.__DLNJ_BACK__;vi.unstubAllGlobals()});
function mount(){render(<BrowserRouter><App/></BrowserRouter>)}
it('shows an embedded request hall with three shortcuts and no duplicate workspace navigation',async()=>{
 mount();await screen.findByText(summary.title);
 expect(document.querySelector('.workspace-sidebar')).toBeNull();expect(document.querySelector('.workspace-topbar')).toBeNull();expect(document.querySelector('.proc-mobile-header')).toBeNull();
 expect(within(screen.getByRole('navigation',{name:'采购快捷入口'})).getAllByRole('link')).toHaveLength(3);
 expect(screen.queryByRole('combobox')).toBeNull();
 await userEvent.click(screen.getByRole('link',{name:/采购旋耕机和配件/}));await screen.findByRole('heading',{name:'采购需求详情'});
 expect(calls.filter(c=>c.method!=='GET')).toHaveLength(0);
 expect(screen.getByRole('button',{name:'参与并查看完整需求'})).toBeTruthy();
});
it('applies linked filters once, restores them, and lets native Back dismiss only the sheet',async()=>{
 mount();await screen.findByText(summary.title);
 await userEvent.click(screen.getByRole('button',{name:'行业展馆'}));
 const dialog=screen.getByRole('dialog');await within(dialog).findByRole('option',{name:'耕整地机械'});
 await userEvent.selectOptions(within(dialog).getByRole('combobox',{name:'产品分类'}),'101');
 await userEvent.selectOptions(within(dialog).getByRole('combobox',{name:'省份'}),'13');
 await userEvent.selectOptions(within(dialog).getByRole('combobox',{name:'城市'}),'1305');
 await within(dialog).findByRole('option',{name:'宁晋县'});await userEvent.selectOptions(within(dialog).getByRole('combobox',{name:'区县'}),'130528');
 expect(calls.filter(c=>c.path.split('?')[0].endsWith('/public/procurements'))).toHaveLength(1);
 await userEvent.click(within(dialog).getByRole('button',{name:'完成'}));
 await waitFor(()=>expect(location.search).toContain('category_id=101'));expect(location.search).toContain('region=130528');expect(location.search).not.toContain('page=');
 await userEvent.click(screen.getByRole('button',{name:/产品分类/}));expect(document.body.style.overflow).toBe('hidden');
 expect(window.__DLNJ_BACK__?.()).toBe(true);await waitFor(()=>expect(screen.queryByRole('dialog')).toBeNull());expect(document.body.style.overflow).toBe('');
 await userEvent.click(screen.getByRole('button',{name:/产品分类/}));await userEvent.click(within(screen.getByRole('dialog')).getByRole('button',{name:'重置'}));expect(location.search).toBe('');
});
it('submits the product keyword and keeps a public summary read-only',async()=>{
 mount();await screen.findByText(summary.title);await userEvent.type(screen.getByRole('textbox',{name:'项目关键词'}),'旋耕机');await userEvent.click(screen.getByRole('button',{name:'查询'}));
 await waitFor(()=>expect(new URLSearchParams(location.search).get('q')).toBe('旋耕机'));
 await userEvent.click(await screen.findByRole('link',{name:/采购旋耕机和配件/}));await screen.findByRole('heading',{name:'采购需求详情'});expect(calls.some(c=>c.path.includes('/join'))).toBe(false);
});
it('keeps a six-digit province-administered city selected and clears it when the province changes',async()=>{
 mount();await screen.findByText(summary.title);await userEvent.click(screen.getByRole('button',{name:'地区'}));
 const dialog=screen.getByRole('dialog');await userEvent.selectOptions(within(dialog).getByRole('combobox',{name:'省份'}),'42');
 await within(dialog).findByRole('option',{name:'仙桃市'});await userEvent.selectOptions(within(dialog).getByRole('combobox',{name:'城市'}),'429004');
 await waitFor(()=>expect((within(dialog).getByRole('combobox',{name:'城市'}) as HTMLSelectElement).value).toBe('429004'));
 await waitFor(()=>expect((within(dialog).getByRole('combobox',{name:'区县'}) as HTMLSelectElement).disabled).toBe(true));
 await userEvent.selectOptions(within(dialog).getByRole('combobox',{name:'省份'}),'11');
 expect((within(dialog).getByRole('combobox',{name:'城市'}) as HTMLSelectElement).value).toBe('');
 await userEvent.click(within(dialog).getByRole('button',{name:'完成'}));expect(new URLSearchParams(location.search).get('region')).toBe('11');
});
