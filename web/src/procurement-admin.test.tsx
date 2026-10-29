import {configureRuntime,runtimeConfig} from './runtime-config';
// @vitest-environment jsdom
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen,waitFor,within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {BrowserRouter} from 'react-router-dom';
import {App} from './main';
import {withHallFixture} from './hall-test-fixture';
import {procurementAdminPage} from './procurement-admin-routes';

let mode:'empty'|'error'|'loading'|'records'='empty';
let release:((response:Response)=>void)|undefined;
let paths:string[]=[];
const json=(data:unknown,status=200)=>new Response(JSON.stringify(data),{status});
beforeEach(()=>{configureRuntime({...runtimeConfig(),capabilities:{procurement:true}});
 mode='empty';paths=[];release=undefined;localStorage.clear();sessionStorage.clear();sessionStorage.setItem('zqzt.admin','fixture');
 vi.stubGlobal('scrollTo',vi.fn());
 vi.stubGlobal('fetch',withHallFixture(async(url:string)=>{
  paths.push(url);
  if(url.includes('/admin/dashboard'))return json({account:{id:1,username:'fixture',role:'admin',permissions:[]},counts:{},recent_operations:[]});
  if(url.includes('/admin/procurement/reviews')){
   if(mode==='loading')return new Promise<Response>(resolve=>{release=resolve});
   if(mode==='error')return json({error:{code:'internal_error'}},500);
   const q=new URL(url,'http://localhost').searchParams;
   return json({items:mode==='records'?[{id:5,project_id:9,revision_id:4,status:'approved',audience:'public',action:'publish'}]:[],page:Number(q.get('page'))||1,total:mode==='records'?1:0,page_size:20});
  }
  if(url.includes('/admin/procurement/stats'))return json({projects:{open:1},quotes:0,invitations:0,pending_reviews:0});
  return json({items:[],page:1,total:0,page_size:20});
 }));
});
afterEach(()=>{release?.(json({items:[],page:1,total:0,page_size:20}));cleanup();vi.unstubAllGlobals()});
function mount(path='/admin/procurement/reviews'){history.replaceState({idx:0},'',path);render(<BrowserRouter><App/></BrowserRouter>)}

it('distinguishes an empty review queue from loading and keeps filters available',async()=>{
 mode='loading';mount();await screen.findByRole('heading',{name:'采购需求审核',level:1});
 expect(screen.getByRole('status').textContent).toContain('正在加载');expect(screen.queryByText('暂无待审核需求。')).toBeNull();
 await waitFor(()=>expect(release).toBeTypeOf('function'));
 release?.(json({items:[],total:0,page:1,page_size:20}));
 expect(await screen.findByText('暂无待审核需求。')).toBeTruthy();expect(screen.getByRole('combobox',{name:'审核状态'})).toBeTruthy();expect(screen.getByText('共 0 条记录')).toBeTruthy();
});
it('shows a genuine failure and retries the same filter without stale results',async()=>{
 mode='error';mount('/admin/procurement/reviews?status=rejected&page=2');
 const alert=await screen.findByRole('alert');expect(within(alert).getByRole('button',{name:'重试'})).toBeTruthy();expect(screen.queryByText('暂无采购审核记录。')).toBeNull();
 mode='empty';await userEvent.click(within(alert).getByRole('button',{name:'重试'}));await screen.findByText('当前筛选条件下暂无审核记录，可清空筛选查看全部。');
 expect(paths.filter(p=>p.includes('/reviews?page=2&status=rejected'))).toHaveLength(2);
});
it('clears status and pagination to all reviews and offers read-only history links',async()=>{
 mode='records';mount('/admin/procurement/reviews?status=approved&page=2');await screen.findByRole('link',{name:'查看资料'});
 expect(screen.queryByRole('link',{name:'查看资料并审核'})).toBeNull();
 await userEvent.click(screen.getByRole('button',{name:'清空筛选'}));
 await waitFor(()=>expect(paths.some(p=>p.endsWith('/reviews?page=1&status=all'))).toBe(true));
 expect(screen.getByRole('combobox',{name:'审核状态'})).toHaveProperty('value','all');expect(new URLSearchParams(location.search).has('page')).toBe(false);
 await userEvent.selectOptions(screen.getByRole('combobox',{name:'审核状态'}),'pending');
 await waitFor(()=>expect(paths.some(p=>p.endsWith('/reviews?page=1&status=pending'))).toBe(true));
});
it('uses procurement-specific headings and breadcrumbs with one primary heading',async()=>{
 mount('/admin/procurement/stats');await screen.findByRole('heading',{level:1,name:'采购询比价统计'});
 expect(screen.getAllByRole('heading',{level:1})).toHaveLength(1);
 expect(document.querySelector('.admin-breadcrumb')?.textContent).toContain('采购询比价采购询比价统计');
 await userEvent.click(screen.getByRole('link',{name:'需求与询价'}));
 await screen.findByRole('heading',{level:2,name:'需求与询价'});
 expect(screen.getAllByRole('heading',{level:1})).toHaveLength(1);expect(screen.queryByRole('heading',{name:'我的询价与采购需求'})).toBeNull();
});
it('keeps list/detail navigation in the requests tab with explicit bilingual titles',()=>{
 expect(procurementAdminPage('/admin/procurement/projects/9')?.tab).toBe('requests');
 expect(procurementAdminPage('/admin/procurement/reviews')?.en).toBe('Procurement reviews');
 expect(procurementAdminPage('/admin/procurement/stats/')?.zh).toBe('采购询比价统计');
 expect(procurementAdminPage('/admin/products')).toBeUndefined();
});
