import {withHallFixture} from './hall-test-fixture';
// @vitest-environment jsdom
import {beforeEach,afterEach,it,expect,vi} from 'vitest';
import {render,screen,within,cleanup,waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {BrowserRouter} from 'react-router-dom';
import {App} from './main';
import {emptyData} from './types';
const vendor={id:1,kind:'vendor',status:'pending',public_visible:true,visible:true,showroom_slug:'company',revision_id:2,data:{...emptyData,name:'测试资料修改（非正式展示）',region_name:'测试地区',main_products:'测试机械',phone:'0319-0000000'}};
const publicVendor={...vendor,status:'approved',data:{...vendor.data,name:'公开版本企业（测试）',company_profile:{summary:'仅公开简介'}}};
const product={...vendor,id:8,kind:'product',data:{...emptyData,name:'尚未审核的新名称',parameters:'型号：1LF-440\n规格：液压设备'}};
let calls:{url:string;method?:string}[]=[], hidden=false, failVendor=false;
const response=(data:unknown,status=200)=>new Response(JSON.stringify(data),{status});
beforeEach(()=>{calls=[];hidden=failVendor=false;localStorage.clear();sessionStorage.clear();sessionStorage.setItem('zqzt.merchant','owner-token');sessionStorage.setItem('zqzt.merchant.id','9');vi.stubGlobal('scrollTo',vi.fn());Element.prototype.scrollIntoView=vi.fn();vi.stubGlobal('fetch',withHallFixture(vi.fn(async(url:string,options?:RequestInit)=>{
  calls.push({url,method:options?.method});
  if(url.includes('/merchant/dashboard'))return response({products:{total:40,public:30,pending:6,draft:3,rejected:1}});
  if(url.includes('/merchant/contents?')){if(url.includes('kind=vendor'))return failVendor?response({error:{code:'network_error'}},503):response({items:[{...vendor,public_visible:!hidden}]});return response({items:[product],total:1,page:1,page_size:20});}
  if(url.endsWith('/public/products/8'))return response({item:{...product,data:{...product.data,name:'正式公开产品（测试）'}},vendor:publicVendor});
  if(url.includes('/public/showrooms/'))return response({item:publicVendor,canonical_url:'https://business.example/v/company?private=1#draft'});
  if(url.includes('/taxonomy'))return response({categories:[],regions:[],halls:[]});
  if(url.endsWith('/offline'))return response({ok:true});
  if(url.endsWith('/history'))return response([]);
  return response({items:[],total:0});
})));});
afterEach(()=>{cleanup();vi.unstubAllGlobals();vi.restoreAllMocks()});
const mount=(path='/merchant')=>{history.replaceState(null,'',path);render(<BrowserRouter><App/></BrowserRouter>)};
it('shows three compact counts, four primary tasks and independent draft/display states',async()=>{
  mount();await screen.findByText('30');expect(screen.getByText('6')).toBeTruthy();expect(screen.getByText('3')).toBeTruthy();expect(screen.queryByText('40')).toBeNull();
  const actions=document.querySelector('.workspace-quick-actions')!;
  for(const label of ['编辑展厅','添加产品','管理产品','分享官网'])expect(within(actions as HTMLElement).getByRole('link',{name:label})).toBeTruthy();
  const company=document.querySelector('.compact-company')!;expect(within(company as HTMLElement).getByText('待审核')).toBeTruthy();expect(within(company as HTMLElement).getByText('已展示')).toBeTruthy();
  expect(screen.getByText('建议补充（非必填）')).toBeTruthy();expect(document.querySelector('.workspace-topbar-actions')!.textContent).not.toContain('退出登录');
});
it('keeps vendor and full-account stats across language and management routes',async()=>{
  mount();await screen.findByText('30');const user=userEvent.setup();await user.click(screen.getByRole('button',{name:'Switch to English'}));await user.click(screen.getByRole('link',{name:'Manage showroom'}));await screen.findByText('Company information');
  expect(calls.filter(c=>c.url.includes('kind=vendor'))).toHaveLength(1);expect(calls.filter(c=>c.url.includes('/merchant/dashboard'))).toHaveLength(1);
});
it('debounces model search and keeps the same query after explicit submit',async()=>{
  mount('/merchant/products');await screen.findByText('尚未审核的新名称');const user=userEvent.setup();await user.type(screen.getByRole('textbox',{name:'搜索产品名称、型号或规格'}),'1LF');
  await waitFor(()=>expect(calls.filter(c=>c.url.includes('kind=product')&&c.url.includes('q=1LF'))).toHaveLength(1));
  await user.click(screen.getByRole('button',{name:'找产品'}));expect(calls.filter(c=>c.url.includes('kind=product')&&c.url.includes('q=1LF'))).toHaveLength(1);
  expect(screen.getByText('型号：1LF-440 · 规格：液压设备')).toBeTruthy();await user.click(screen.getByRole('button',{name:'已展示'}));expect(new URLSearchParams(location.search).get('public_state')).toBe('public');
});
it('opens promotion directly with public data, a normalized address and a share fallback',async()=>{
  mount('/merchant/promotion');await screen.findByText('公开版本企业（测试）');expect(screen.queryByRole('dialog')).toBeNull();expect(document.querySelector('.share-company-preview')!.textContent).not.toContain('测试资料修改');
  expect(screen.getByRole('textbox',{name:'展厅地址'})).toHaveProperty('value','https://business.example/v/company');const user=userEvent.setup();await user.click(screen.getByRole('button',{name:'分享官网'}));await screen.findByText('当前无法调用系统分享，可复制链接或保存素材发送。');
  for(const name of ['复制地址','展厅二维码','生成海报预览'])expect(screen.getByRole('button',{name})).toBeTruthy();
});
it('does not build promotional materials for an unpublished company',async()=>{
  hidden=true;mount('/merchant/promotion');await screen.findByText('未展示');expect(screen.queryByRole('button',{name:'分享官网'})).toBeNull();expect(calls.some(c=>c.url.includes('/public/showrooms/'))).toBe(false);
});
it('shares the current public product name and never its unreviewed revision',async()=>{
  const share=vi.fn().mockResolvedValue(undefined);Object.defineProperty(navigator,'share',{configurable:true,value:share});
  try{mount('/merchant/products');await screen.findByText('尚未审核的新名称');const user=userEvent.setup();await user.click(screen.getByText('更多'));await user.click(screen.getByRole('button',{name:'分享产品'}));await waitFor(()=>expect(share).toHaveBeenCalledWith({title:'正式公开产品（测试）',url:'https://business.example/products/8'}));expect(screen.queryByRole('button',{name:'删除'})).toBeNull();}finally{delete (navigator as {share?:unknown}).share;}
});
it('requires confirmation before offline and keeps logout inside settings',async()=>{
  mount('/merchant/products');await screen.findByText('尚未审核的新名称');const user=userEvent.setup();await user.click(screen.getByText('更多'));await user.click(screen.getByRole('button',{name:'下架'}));await user.click(within(screen.getByRole('dialog')).getByRole('button',{name:'取消'}));expect(calls.some(c=>c.method==='POST')).toBe(false);
  await user.click(screen.getByRole('button',{name:'下架'}));await user.click(within(screen.getByRole('dialog')).getByRole('button',{name:'下架'}));await waitFor(()=>expect(calls.some(c=>c.url.endsWith('/offline')&&c.method==='POST')).toBe(true));
  await user.click(screen.getByRole('link',{name:'设置'}));await user.click(screen.getByRole('button',{name:'退出登录'}));expect(screen.getByRole('dialog')).toBeTruthy();await user.click(within(screen.getByRole('dialog')).getByRole('button',{name:'取消'}));expect(sessionStorage.getItem('zqzt.merchant')).toBe('owner-token');
});
