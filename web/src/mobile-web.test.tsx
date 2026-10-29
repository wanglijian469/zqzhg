// @vitest-environment jsdom
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen,waitFor,within,fireEvent} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {BrowserRouter} from 'react-router-dom';
import {App} from './main';
import {emptyData} from './types';
import {withHallFixture} from './hall-test-fixture';
import {phoneNumbers} from './public-mobile';
const vendor={id:21,kind:'vendor',hall_id:1,showroom_slug:'mobile',data:{...emptyData,name:'真实制造企业',main_products:'工业设备',phone:'0319-1234567 / 13800000001（王经理）'}};
const product={id:100,kind:'product',vendor_id:21,hall_id:1,data:{...emptyData,name:'完整轮廓设备',parameters:'型号：M-100',media_ids:[4,5],category_id:1}};
const calls:string[]=[];
const fetcher=vi.fn(async(url:string)=>{calls.push(url);let data:unknown;
 if(url.includes('/taxonomy'))data={categories:[{id:1,name:'设备',name_en:'Equipment'}],regions:[{id:1,name:'河北',name_en:'Hebei'}]};
 else if(url.includes('/showrooms/'))data={item:vendor,home_products:[product,product],product_count:49,resource_counts:{},categories:[{id:1,name:'设备'}]};
 else if(url.includes('/products/100'))data={item:product,vendor};
 else if(url.includes('/products?'))data={items:[product,product],total:49,page:1,page_size:24};
 else if(url.includes('/vendors?'))data={items:[vendor],total:41,page:1,page_size:20};
 else if(url.includes('/home'))data={vendors:[vendor],products:[product],hero:{media_id:0}};
 else data={items:[],total:0};return new Response(JSON.stringify(data));});
beforeEach(()=>{calls.length=0;localStorage.clear();sessionStorage.clear();vi.stubGlobal('scrollTo',vi.fn());vi.stubGlobal('fetch',withHallFixture(fetcher));vi.stubGlobal('matchMedia',vi.fn(()=>({matches:true,addEventListener:vi.fn(),removeEventListener:vi.fn()})))});
afterEach(()=>{cleanup();vi.unstubAllGlobals()});
function mount(path:string){history.replaceState(null,'',path);render(<BrowserRouter><App/></BrowserRouter>)}
it('keeps compact filters visible with debounced and explicit submission',async()=>{
 mount('/browse?hall=agriculture&page=3');const user=userEvent.setup();const input=await screen.findByRole('textbox',{name:'关键词'});expect(document.querySelector('.browse-page .responsive-filters')).toBeNull();
 await user.type(input,'M-100');await user.selectOptions(screen.getByRole('combobox',{name:'厂家分类'}),'1');await waitFor(()=>expect(new URLSearchParams(location.search).get('q')).toBe('M-100'));expect(input).toHaveProperty('value','M-100');expect(location.search).not.toContain('page=3');
 await user.type(input,'{Enter}');await waitFor(()=>expect(location.search).toContain('q=M-100'));expect(location.search).toContain('hall=agriculture');
 await user.click(screen.getByRole('button',{name:'移除条件: M-100'}));expect(new URLSearchParams(location.search).has('q')).toBe(false);expect(location.search).toContain('category_id=1');expect(input).toHaveProperty('value','');
});
it('shows desktop controls and returns focus when closing platform navigation',async()=>{
 vi.stubGlobal('matchMedia',vi.fn(()=>({matches:false,addEventListener:vi.fn(),removeEventListener:vi.fn()})));mount('/browse');await screen.findByRole('combobox',{name:'排序方式'});expect(document.querySelector('.browse-compact-options')).toBeTruthy();
 const button=document.querySelector<HTMLButtonElement>('.menu-button')!;await userEvent.click(button);expect(button.getAttribute('aria-expanded')).toBe('true');fireEvent.keyDown(document,{key:'Escape'});expect(button.getAttribute('aria-expanded')).toBe('false');expect(document.activeElement).toBe(button);
});
it('hides empty media tabs, deduplicates company products and uses bounded requests',async()=>{
 mount('/v/mobile?tab=products');await screen.findByRole('heading',{name:vendor.data.name});expect(screen.getAllByRole('tab')).toHaveLength(4);expect(document.querySelector('.sr-menu-toggle')).toBeNull();await waitFor(()=>expect(document.querySelectorAll('.showroom-product-grid article')).toHaveLength(1));expect(document.querySelector('.sr-home-hero')).toBeTruthy();expect(calls.some(u=>u.includes('vendor_id=21')&&u.includes('page_size=24'))).toBe(true);expect(document.querySelector('.responsive-filters')).toHaveProperty('open',false);
});
it('selects complete product images and closes the lightbox by keyboard',async()=>{
 mount('/products/100');await screen.findByRole('heading',{name:product.data.name});await userEvent.click(screen.getByRole('button',{name:'查看图片 2'}));const gallery=document.querySelector('.mobile-product-gallery')!;expect(gallery.querySelector('.product-main-photo img')?.getAttribute('src')).toBe('/api/v1/media/5?w=480');await userEvent.click(within(gallery as HTMLElement).getByRole('button',{name:'工厂 / 产品实拍'}));expect(screen.getByRole('dialog')).toBeTruthy();await userEvent.keyboard('{ArrowLeft}');expect(within(screen.getByRole('dialog')).getByRole('img').getAttribute('src')).toBe('/api/v1/media/4');await userEvent.keyboard('{Escape}');expect(screen.queryByRole('dialog')).toBeNull();
});
it('offers real numbers without remarks and a manual copy fallback',async()=>{
 expect(phoneNumbers('0319-1234567 / 13800000001（王经理）')).toEqual(['03191234567','13800000001']);expect(phoneNumbers('销售经理')).toEqual([]);mount('/v/mobile?tab=contact');await screen.findByRole('heading',{name:vendor.data.name});await userEvent.click(within(document.querySelector('.sr-contact-card')!).getByRole('button',{name:vendor.data.phone}));const dialog=screen.getByRole('dialog');expect(within(dialog).getByRole('link',{name:'03191234567'}).getAttribute('href')).toBe('tel:03191234567');const user=userEvent.setup();vi.spyOn(navigator.clipboard,'writeText').mockRejectedValueOnce(Error('denied'));await user.click(within(dialog).getByRole('button',{name:'复制电话'}));expect(await within(dialog).findByRole('textbox',{name:'电话'})).toHaveProperty('value',vendor.data.phone);
});

for(const route of ['/browse','/browse/products'])it('clears obsolete hidden query conditions before requesting '+route,async()=>{
 mount(route+'?q=test&industrial_belt_id=77&business_nature=dealer&page=3');await screen.findByRole('combobox',{name:'排序方式'});
 await waitFor(()=>expect(location.search).not.toContain('industrial_belt_id'));expect(location.search).not.toContain('business_nature');expect(location.search).not.toContain('page=3');expect(location.search).toContain('q=test');
 expect(screen.queryByRole('combobox',{name:'产业带'})).toBeNull();expect(screen.queryByRole('combobox',{name:'经营性质'})).toBeNull();
 const requests=calls.filter(u=>u.includes(route==='/browse'?'/public/vendors?':'/public/products?'));await waitFor(()=>expect(calls.filter(u=>u.includes(route==='/browse'?'/public/vendors?':'/public/products?')).length).toBeGreaterThan(0));expect(requests.every(u=>!u.includes('industrial_belt_id')&&!u.includes('business_nature')&&!u.includes('page=3'))).toBe(true)
});
