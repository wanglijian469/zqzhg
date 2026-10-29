import {withHallFixture} from './hall-test-fixture';
import {uiMessages} from './ui-messages';
// @vitest-environment jsdom
import React from 'react';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import {cleanup, render, screen, waitFor, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {BrowserRouter} from 'react-router-dom';
import {App} from './main';
import {emptyData} from './types';
import {messages} from './i18n';

const taxonomy={halls:[],categories:[{id:1,parent_id:0,name:'农业机械',name_en:'Agricultural machinery'},{id:2,parent_id:0,name:'农机配件',name_en:'Machinery parts'}],regions:[{id:1,name:'测试地区',name_en:'Test region'}]};
let offline=false;
const defaultFetch=async (url:string, options?:RequestInit)=>{
  let data:unknown;
  if(url.includes('/taxonomy'))data=taxonomy;
  else if(url.includes('/admin/dashboard'))data={counts:{vendor_total:1,product_total:0},recent_operations:[],account:{id:1,username:'admin'}};
  else if(url.includes('/offline')&&options?.method==='POST'){offline=true;data={ok:true}}
  else if(url.includes('/history'))data=[];
  else if(url.includes('/admin/contents/1'))data={id:1,kind:'vendor',owner_id:2,data:{...emptyData,name:'交互测试厂家（不入库）'},status:'approved',revision_id:1,visible:!offline,public_visible:!offline,featured:false,sort_order:0};
  else if(url.includes('/admin/contents'))data={items:[{id:1,kind:'vendor',data:{...emptyData,name:'交互测试厂家（不入库）'},status:'approved',revision_id:1,visible:!offline,public_visible:!offline,featured:false,sort_order:0}],total:1,page:1,page_size:20};
  else if(url.includes('/public/vendors'))data={items:[],total:0,page:1,page_size:20};
  else if(url.includes('/public/products'))data={items:[],total:0,page:1,page_size:20};
  else if(url.includes('/public/home'))data={vendors:[],products:[],vendor_count:0};
  else throw new Error('Unexpected request: '+url);
  return new Response(JSON.stringify(data),{status:200,headers:{'Content-Type':'application/json'}});
};
const fetchMock=vi.fn(defaultFetch);
beforeEach(()=>{
  offline=false;fetchMock.mockReset();fetchMock.mockImplementation(defaultFetch);localStorage.clear();sessionStorage.clear();
  vi.stubGlobal('fetch',withHallFixture(fetchMock));vi.stubGlobal('scrollTo',vi.fn());
});
afterEach(()=>{cleanup();vi.unstubAllGlobals()});

describe('Browser workflows',()=>{
  it('searches for manufacturers by default and switches home search to products',async()=>{
    history.replaceState(null,'','/?hall=agriculture');
    render(<BrowserRouter><App/></BrowserRouter>);
    const user=userEvent.setup();
    await user.type(await screen.findByRole('textbox',{name:'关键词'}),'440');
    expect(screen.getByRole('button',{name:'找厂家',pressed:true})).toBeTruthy();
    await user.click(screen.getAllByRole('button',{name:'找厂家'})[1]);
    expect(location.pathname).toBe('/browse');
    expect(new URLSearchParams(location.search).get('q')).toBe('440');
    await user.click(screen.getByRole('link',{name:'首页'}));
    await user.type(await screen.findByRole('textbox',{name:'关键词'}),'550');
    await user.click(screen.getByRole('button',{name:'找产品'}));
    expect((screen.getByRole('textbox',{name:'关键词'}) as HTMLInputElement).value).toBe('550');
    await user.click(screen.getAllByRole('button',{name:'找产品'})[1]);
    expect(location.pathname).toBe('/browse/products');
    expect(new URLSearchParams(location.search).get('q')).toBe('550');
    expect(await screen.findByText('暂未找到匹配的产品')).toBeTruthy();
  });
  it('preserves product filters, pagination and language when returning from product or vendor details',async()=>{
    const original='/browse/products?q=TEST&category_id=2&region_id=1&page=2&page_size=1';
    history.replaceState(null,'',original);
    const vendor={id:1,kind:'vendor',data:{...emptyData,name:'产品浏览测试厂家（不入库）',phone:'13800000001',region_name:'测试地区'}};
    const product={id:2,kind:'product',data:{...emptyData,name:'TEST-440翻转犁（不入库）'},vendor:{id:1,name:vendor.data.name,name_en:'',region_id:1,region_name:'测试地区'}};
    fetchMock.mockImplementation(async(url:string,options?:RequestInit)=>{
      let data;
      if(url.startsWith('/api/v1/public/products/2'))data={item:product,vendor};
      else if(url.startsWith('/api/v1/public/products?'))data={items:[product],total:2,page:Number(new URL(url,'http://localhost').searchParams.get('page')||1),page_size:1};
      else if((url.startsWith('/api/v1/public/vendors/1')||url.startsWith('/api/v1/public/showrooms/v-1')))data={item:vendor,products:{items:[product],total:1,page:1,page_size:20}};
      else return defaultFetch(url,options);
      return new Response(JSON.stringify(data));
    });
    render(<BrowserRouter><App/></BrowserRouter>);
    const user=userEvent.setup();
    await user.click(await screen.findByRole('link',{name:product.data.name}));
    await screen.findByRole('heading',{name:product.data.name});
    await user.click(screen.getByRole('button',{name:'English'}));
    await user.click(within(document.querySelector('main')!).getByRole('link',{name:uiMessages.en.navProducts}));
    await screen.findByRole('link',{name:product.data.name});
    expect(location.pathname+location.search).toBe(original);
    expect((screen.getByRole('textbox',{name:'Keyword'}) as HTMLInputElement).value).toBe('TEST');
    expect((screen.getByRole('combobox',{name:'Product category'}) as HTMLSelectElement).value).toBe('2');
    expect((screen.getByRole('combobox',{name:'Region'}) as HTMLSelectElement).value).toBe('1');
    await user.click(screen.getByRole('link',{name:vendor.data.name}));
    await screen.findByRole('heading',{name:vendor.data.name});
    await user.click(within(document.querySelector('main')!).getByRole('link',{name:uiMessages.en.navProducts}));
    await screen.findByRole('link',{name:product.data.name});
    expect(location.pathname+location.search).toBe(original);
    await user.click(within(document.querySelector('header')!).getByRole('link',{name:'Find manufacturers'}));
    expect(location.pathname).toBe('/browse');
    const params=new URLSearchParams(location.search);
    expect(params.get('q')).toBe('TEST');expect(params.get('category_id')).toBe('2');expect(params.get('region_id')).toBe('1');expect(params.has('page')).toBe(false);
    await user.click(within(document.querySelector('header')!).getByRole('link',{name:uiMessages.en.navProducts}));
    expect(location.pathname).toBe('/browse/products');
    expect((screen.getByRole('textbox',{name:'Keyword'}) as HTMLInputElement).value).toBe('TEST');
  });
  it('routes popular products to the catalog and gives direct product details a catalog return link',async()=>{
    history.replaceState(null,'','/?hall=agriculture');
    const product={id:2,kind:'product',data:{...emptyData,name:'首页测试产品（不入库）'}};
    const vendor={id:1,kind:'vendor',data:{...emptyData,name:'首页测试厂家（不入库）'}};
    fetchMock.mockImplementation(async(url:string,options?:RequestInit)=>{
      if(url==='/api/v1/public/home')return new Response(JSON.stringify({vendors:[],products:[product],vendor_count:0}));
      if(url.startsWith('/api/v1/public/products/2'))return new Response(JSON.stringify({item:product,vendor}));
      return defaultFetch(url,options);
    });
    render(<BrowserRouter><App/></BrowserRouter>);
    const user=userEvent.setup();
    const popular=await screen.findByRole('heading',{name:'精选工业产品'});
    expect(within(popular.closest('section')!).getByRole('link',{name:'查看全部'}).getAttribute('href')).toBe('/browse/products');
    await user.click(await within(popular.closest('section')!).findByRole('link',{name:'查看产品 '+product.data.name}));
    await screen.findByRole('heading',{name:product.data.name});
    expect(within(document.querySelector('main')!).getByRole('link',{name:'找产品'}).getAttribute('href')).toBe('/browse/products');
  });
  it('opens a product from its vendor gallery without rendering the previous page response',async()=>{
    history.replaceState(null,'','/vendors/1?tab=products');
    const vendor={id:1,kind:'vendor',data:{...emptyData,name:'跳转测试厂家（不入库）',phone:'13800000001'}};
    const product={id:2,kind:'product',data:{...emptyData,name:'跳转测试翻转犁（不入库）',parameters:'测试参数'}};
    let resolveProduct!:(response:Response)=>void;
    fetchMock.mockImplementation(async(url:string,options?:RequestInit)=>{
      if(url.startsWith('/api/v1/public/products?'))return new Response(JSON.stringify({items:[product],total:1,page:1,page_size:12}));
      if(url.startsWith('/api/v1/public/vendors/1')||url.startsWith('/api/v1/public/showrooms/v-1'))return new Response(JSON.stringify({item:vendor,products:{items:[product],total:1,page:1,page_size:20}}));
      if(url.startsWith('/api/v1/public/products/2'))return new Promise<Response>(resolve=>{resolveProduct=resolve});
      return defaultFetch(url,options);
    });
    render(<BrowserRouter><App/></BrowserRouter>);
    const user=userEvent.setup();
    await user.click((await screen.findAllByRole('link',{name:'查看产品 '+product.data.name}))[0]);
    expect(location.pathname).toBe('/products/2');
    expect(screen.queryByRole('heading',{name:vendor.data.name})).toBeNull();
    expect(screen.getByText('正在加载…')).toBeTruthy();
    resolveProduct(new Response(JSON.stringify({item:product,vendor})));
    await screen.findByRole('heading',{name:product.data.name});
    expect(screen.getByRole('link',{name:vendor.data.phone}).getAttribute('href')).toBe('tel:'+vendor.data.phone);
    expect(screen.getByText('测试参数')).toBeTruthy();
    await user.click(screen.getByRole('link',{name:'所属厂商 '+vendor.data.name}));
    await screen.findByRole('heading',{name:vendor.data.name});
    expect(location.pathname).toBe('/v/v-1');
    await user.click(screen.getByRole('tab',{name:'产品中心'}));
    expect(screen.getAllByRole('link',{name:'查看产品 '+product.data.name})[0]).toBeTruthy();
  });
  it('signs in with a company account and loads its existing booth for editing',async()=>{
    history.replaceState(null,'','/merchant/login');
    const fixture={id:9,kind:'vendor',data:{...emptyData,name:'登录交互测试企业（不入库）',region_id:1,region_name:'测试地区'},status:'approved',revision_id:7,visible:true,public_visible:true,featured:false,sort_order:0};
    fetchMock.mockImplementation(async(url:string,options?:RequestInit)=>{
      let data:unknown;
      if(url==='/api/v1/auth/merchant'){
        expect(JSON.parse(options!.body as string)).toEqual({username:'测试企业账号',password:'Test-Merchant-Password!'});
        data={token:'test-merchant-token',account:{id:123}};
      }else if(url.includes('/taxonomy'))data=taxonomy;
      else if(url.includes('/merchant/dashboard'))data={products:{total:0,public:0,draft:0,pending:0,rejected:0}};
      else if(url.endsWith('/merchant/contents/9'))data=fixture;else if(url.includes('/merchant/contents'))data={items:[fixture],total:1,page:1,page_size:20};
      else throw new Error('Unexpected request: '+url);
      return new Response(JSON.stringify(data),{status:200});
    });
    render(<BrowserRouter><App/></BrowserRouter>);
    const user=userEvent.setup();
    await user.type(await screen.findByRole('textbox',{name:'公司登录账号'}),'测试企业账号');
    await user.type(screen.getByLabelText('密码',{exact:true}),'Test-Merchant-Password!');
    await user.click(screen.getByRole('button',{name:'登录'}));
    await screen.findByRole('heading',{name:'厂商工作台'});
    expect(sessionStorage.getItem('zqzt.merchant.id')).toBe('123');
    await user.click((await screen.findAllByRole('link',{name:'编辑展厅'}))[0]);
    expect((await screen.findByRole('textbox',{name:'企业 / 工厂名称'}) as HTMLInputElement).value).toBe(fixture.data.name);
    expect(location.pathname).toBe('/merchant/booth/edit');
  });
  it('keeps a typed keyword across category, region and language changes',async()=>{
    history.replaceState(null,'','/browse');
    render(<BrowserRouter><App/></BrowserRouter>);
    const user=userEvent.setup();
    await screen.findByRole('option',{name:'农机配件'});
    await user.type(await screen.findByRole('textbox',{name:'关键词'}),'component{Enter}');
    await user.selectOptions(screen.getByRole('combobox',{name:'厂家分类'}),'2');
    await user.selectOptions(screen.getByRole('combobox',{name:'所在地区'}),'1');
    await user.click(screen.getByRole('button',{name:'English'}));
    expect((screen.getByRole('textbox',{name:'Keyword'}) as HTMLInputElement).value).toBe('component');
    expect((screen.getByRole('combobox',{name:'Manufacturer category'}) as HTMLSelectElement).value).toBe('2');
    expect((screen.getByRole('combobox',{name:'Region'}) as HTMLSelectElement).value).toBe('1');
    const params=new URLSearchParams(location.search);
    expect(params.get('q')).toBe('component');expect(params.get('category_id')).toBe('2');expect(params.get('region_id')).toBe('1');
  });
  it('requires an explicit confirmation before an administrator takes content offline',async()=>{
    history.replaceState(null,'','/admin/contents/1');sessionStorage.setItem('zqzt.admin','test-only-token');
    render(<BrowserRouter><App/></BrowserRouter>);
    const user=userEvent.setup();
    await user.click(await screen.findByRole('button',{name:'下架'}));
    expect(screen.getAllByRole('dialog')).toHaveLength(1);
    await user.click(within(screen.getByRole('dialog')).getByRole('button',{name:'取消'}));
    expect(offline).toBe(false);
    await user.click(screen.getByRole('button',{name:'下架'}));
    await user.click(within(screen.getByRole('dialog')).getByRole('button',{name:'下架'}));
    await waitFor(()=>expect(screen.queryAllByRole('dialog')).toHaveLength(0));
    expect(offline).toBe(true);
    expect(fetchMock.mock.calls.filter(([url])=>url.includes('/offline'))).toHaveLength(1);
    expect(await screen.findByText('未展示',{exact:true})).toBeTruthy();
  });
});

