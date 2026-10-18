import {withHallFixture} from './hall-test-fixture';
// @vitest-environment jsdom
import React from 'react';
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen,within,waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {BrowserRouter} from 'react-router-dom';
import {App} from './main';
import {emptyData} from './types';

const official=[{id:1,parent_id:0,name:'农业机械',name_en:'Agricultural machinery',enabled:true},{id:2,parent_id:1,name:'农机配件',name_en:'Parts',enabled:true}];
const vendor={id:4,kind:'vendor',owner_id:7,revision_id:8,status:'approved',data:{...emptyData,name:'分类验收厂商',region_name:'河北',main_products:'液压机械',phone:'0319-1234567',category_id:2}};
const product={id:5,kind:'product',owner_id:7,revision_id:9,status:'draft',data:{...emptyData,name:'已有液压产品',category_id:2}};
let pending:typeof official=[];let failNext=false;let holdRefresh=false;let productStatus='draft';const refreshes:Array<(r:Response)=>void>=[];
const response=(v:unknown,status=200)=>new Response(JSON.stringify(v),{status});
const taxonomy=()=>({categories:[...official,...pending],regions:[],halls:[],aliases:[]});
const fetchMock=vi.fn(async(url:string,opts?:RequestInit)=>{
 if(url.endsWith('/auth/merchant'))return response({token:'new-merchant-token',account:{id:7}});
 if(url.endsWith('/merchant/categories')&&opts?.method==='POST'){
  if(failNext){failNext=false;throw new TypeError('network unavailable')}
  const input=JSON.parse(opts.body as string);const existing=pending.find(c=>c.name===input.name&&c.parent_id===input.parent_id);
  const category=existing||{id:70+pending.length,parent_id:input.parent_id,name:input.name,name_en:input.name_en,enabled:true,suggestion_status:'pending'};
  if(!existing)pending.push(category);return response(category,201);
 }
 if(url.endsWith('/merchant/taxonomy')){
  if(holdRefresh&&pending.length)return new Promise<Response>(resolve=>refreshes.push(resolve));
  return response(taxonomy());
 }
 if(url.endsWith('/public/taxonomy'))return response({categories:official,regions:[],halls:[]});
 if(url.endsWith('/merchant/contents/4'))return response(vendor);if(url.endsWith('/merchant/contents/5'))return response({...product,status:productStatus});
 if(url.includes('/merchant/dashboard'))return response({products:{total:1,public:0,draft:1,pending:0,rejected:0}});
 if(url.includes('/merchant/contents?'))return response({items:url.includes('kind=vendor')?[vendor]:[{...product,status:productStatus}],total:1,page:1,page_size:20});
 if(url.includes('/history'))return response([]);if(url.includes('/resources'))return response({items:[],total:0,page:1,page_size:20});
 return response({});
});
const mount=(path:string)=>{history.replaceState(null,'',path);render(<BrowserRouter><App/></BrowserRouter>)};
const signed=()=>{sessionStorage.setItem('zqzt.merchant','test-merchant-token');sessionStorage.setItem('zqzt.merchant.id','7')};
beforeEach(()=>{pending=[];failNext=false;holdRefresh=false;productStatus='draft';refreshes.length=0;fetchMock.mockClear();localStorage.clear();sessionStorage.clear();vi.stubGlobal('fetch',withHallFixture(fetchMock));vi.stubGlobal('scrollTo',vi.fn());Element.prototype.scrollIntoView=vi.fn();vi.stubGlobal('URL',Object.assign(URL,{createObjectURL:vi.fn(()=> 'blob:retained-photo'),revokeObjectURL:vi.fn()}))});
afterEach(()=>{holdRefresh=false;for(const finish of refreshes)finish(response(taxonomy()));cleanup();vi.unstubAllGlobals();localStorage.clear();sessionStorage.clear()});

it('creates and immediately selects top-level business and product categories with parent_id zero',async()=>{
 const user=userEvent.setup();signed();mount('/join?tab=boothManage');await user.click(await screen.findByRole('link',{name:'基本信息'}));
 if(!document.querySelector('.company-more-basics')?.hasAttribute('open'))await user.click(document.querySelector('.company-more-basics>summary')!);await user.click(screen.getByRole('button',{name:'新增经营分类'}));let dialog=screen.getByRole('dialog');
 await user.selectOptions(within(dialog).getByRole('combobox',{name:'上级分类'}),'0');await user.type(within(dialog).getByRole('textbox',{name:'中文分类名称'}),'齿轮');await user.click(within(dialog).getByRole('button',{name:'保存并选中'}));
 await waitFor(()=>expect(screen.queryByRole('dialog')).toBeNull());expect(pending[0].parent_id).toBe(0);expect((screen.getByRole('combobox',{name:'企业经营分类'}) as HTMLSelectElement).value).toBe('70');
 await user.click(screen.getByRole('button',{name:'新增经营分类'}));dialog=screen.getByRole('dialog');expect((within(dialog).getByRole('combobox',{name:'上级分类'}) as HTMLSelectElement).value).toBe('0');await user.click(within(dialog).getByRole('button',{name:'取消'}));
 await user.click(screen.getByRole('link',{name:'产品管理'}));if(screen.queryByRole('dialog'))await user.click(within(screen.getByRole('dialog')).getByRole('button',{name:'离开页面'}));await user.click(await screen.findByRole('link',{name:'添加产品'}));await user.click(screen.getByRole('button',{name:'新增产品分类'}));dialog=screen.getByRole('dialog');
 expect((within(dialog).getByRole('combobox',{name:'上级分类'}) as HTMLSelectElement).value).toBe('0');await user.type(within(dialog).getByRole('textbox',{name:'中文分类名称'}),'动力传动');await user.click(within(dialog).getByRole('button',{name:'保存并选中'}));
 await waitFor(()=>expect(screen.queryByRole('dialog')).toBeNull());expect(pending[1].parent_id).toBe(0);expect((screen.getByRole('combobox',{name:'产品分类'}) as HTMLSelectElement).value).toBe('71');
 await user.click(screen.getByRole('button',{name:'English'}));await user.click(screen.getByRole('button',{name:'Add product category'}));expect(screen.getByRole('option',{name:'Top-level (level 1) category'})).toBeTruthy();
});

it('keeps workspace material when a new category dialog is cancelled',async()=>{const user=userEvent.setup();signed();mount('/merchant/booth/edit');const name=await screen.findByRole('textbox',{name:'企业 / 工厂名称'});await user.clear(name);await user.type(name,'保留填写的企业');await user.click(document.querySelector('.company-more-basics>summary')!);await user.selectOptions(screen.getByRole('combobox',{name:'企业经营分类'}),'2');await user.click(screen.getByRole('button',{name:'新增经营分类'}));await user.click(within(screen.getByRole('dialog')).getByRole('button',{name:'取消'}));expect((name as HTMLInputElement).value).toBe('保留填写的企业');expect((screen.getByRole('combobox',{name:'企业经营分类'}) as HTMLSelectElement).value).toBe('2');expect(fetchMock.mock.calls.some(([url,opts])=>url.includes('/merchant/contents')&&opts?.method==='POST')).toBe(false)});

it('keeps failed input, submits only category on Enter and selects the new option before refresh completes',async()=>{
 const user=userEvent.setup();signed();mount('/join?tab=productManage');await user.click(await screen.findByRole('link',{name:'添加产品'}));
 await user.selectOptions(screen.getByRole('combobox',{name:'产品分类'}),'2');await user.click(screen.getByRole('button',{name:'新增产品分类'}));const dialog=await screen.findByRole('dialog');
 const form=dialog.querySelector('form')!;expect(form.parentElement?.closest('form')).toBeNull();
 const input=within(dialog).getByRole('textbox',{name:'中文分类名称'});await user.type(input,'新型液压分类');failNext=true;await user.click(within(dialog).getByRole('button',{name:'保存并选中'}));
 await within(dialog).findByRole('alert');expect((input as HTMLInputElement).value).toBe('新型液压分类');holdRefresh=true;await user.click(input);await user.keyboard('{Enter}');
 await waitFor(()=>expect(screen.queryByRole('dialog')).toBeNull());const select=screen.getByRole('combobox',{name:'产品分类'}) as HTMLSelectElement;expect(select.value).toBe('70');expect(select.selectedOptions[0].textContent).toContain('待管理员规范');
 expect(fetchMock.mock.calls.filter(([url,opts])=>url.endsWith('/merchant/categories')&&opts?.method==='POST').length).toBe(2);
 expect(fetchMock.mock.calls.some(([url,opts])=>url.includes('/merchant/contents')&&(opts?.method==='POST'||opts?.method==='PUT'))).toBe(false);
});

it('reuses an existing suggestion and its official parent without creating duplicate options',async()=>{
 const user=userEvent.setup();signed();pending=[{id:70,parent_id:2,name:'补充液压分类',name_en:'',enabled:true,suggestion_status:'pending'} as typeof official[number]];
 mount('/join?tab=boothManage');await user.click(await screen.findByRole('link',{name:'基本信息'}));await screen.findByRole('option',{name:/补充液压分类/});
 await user.selectOptions(screen.getByRole('combobox',{name:'企业经营分类'}),'70');await user.click(screen.getByRole('button',{name:'新增经营分类'}));const dialog=screen.getByRole('dialog');expect((within(dialog).getByRole('combobox',{name:'上级分类'}) as HTMLSelectElement).value).toBe('2');
 await user.type(within(dialog).getByRole('textbox',{name:'中文分类名称'}),'补充液压分类');await user.click(within(dialog).getByRole('button',{name:'保存并选中'}));await waitFor(()=>expect(screen.queryByRole('dialog')).toBeNull());expect(document.querySelectorAll('select[aria-label="企业经营分类"] option[value="70"]').length).toBe(1);expect(pending.length).toBe(1);
});

it('disables pending product category creation and provides contextual English labels',async()=>{
 const user=userEvent.setup();signed();productStatus='pending';mount('/join?tab=productManage');await user.click(await screen.findByRole('link',{name:'编辑'}));
 expect((screen.getByRole('button',{name:'新增产品分类'}) as HTMLButtonElement).disabled).toBe(true);await user.click(screen.getByRole('button',{name:'English'}));
 expect(screen.getByRole('combobox',{name:'Product category'})).toBeTruthy();expect((screen.getByRole('button',{name:'Add product category'}) as HTMLButtonElement).disabled).toBe(true);expect(screen.queryByRole('dialog')).toBeNull();
});
