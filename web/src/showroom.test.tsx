import {withHallFixture} from './hall-test-fixture';
// @vitest-environment jsdom
import React from 'react';
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen,within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {BrowserRouter} from 'react-router-dom';
import {App} from './main';
import {messages} from './i18n';
import {emptyData} from './types';
import {vendorPath,shareURL,wrapLines} from './showroom-share';
import QRCode from 'qrcode';
import jsQR from 'jsqr';

const vendor={id:1,kind:'vendor',showroom_slug:'sample',data:{...emptyData,name:'测试企业（不入库）',address:'河北省邢台市测试厂址',main_products:'测试翻转犁',phone:'13800000001',company_profile:{short_name:'测试企业',summary:'真实资料测试',established:'1985',contacts:[{area:'测试区域',name:'业务联系人',phone:'13800000002'}]}}};
const product={id:2,kind:'product',data:{...emptyData,name:'TEST-440翻转犁'},vendor:{id:1,name:vendor.data.name,showroom_slug:'sample'}};
const fetcher=vi.fn(async(url:string)=>{
 let data;
 if(url.includes('/public/showrooms/'))data={item:vendor,canonical_url:'https://showroom.example/v/sample',product_count:49,categories:[{id:1,parent_id:0,name:'农业机械',name_en:'Machinery'}]};
 else if(url.includes('/public/products/2'))data={item:product,vendor};
 else if(url.includes('/public/products?')){const query=new URL(url,'http://localhost').searchParams;data={items:query.get('q')==='missing'?[]:[product],total:49,page:Number(query.get('page')||1),page_size:24}}
 else if(url.includes('/taxonomy'))data={halls:[],regions:[],categories:[]};
 else throw new Error('Unexpected request '+url);
 return new Response(JSON.stringify(data));
});
beforeEach(()=>{localStorage.clear();sessionStorage.clear();vi.stubGlobal('fetch',withHallFixture(fetcher));vi.stubGlobal('scrollTo',vi.fn());history.replaceState(null,'','/v/sample')});
afterEach(()=>{cleanup();vi.unstubAllGlobals();fetcher.mockClear()});

it('keeps showroom search, category, page and language across product details and refresh',async()=>{
 render(<BrowserRouter><App/></BrowserRouter>);const user=userEvent.setup();await screen.findByRole('heading',{name:vendor.data.name});
 expect(screen.queryByRole('link',{name:'首页'})).toBeNull();expect(screen.getAllByRole('tab')).toHaveLength(6);await user.click(screen.getByRole('tab',{name:'联系企业'}));expect(screen.getByRole('link',{name:'13800000002'}).getAttribute('href')).toBe('tel:13800000002');await user.click(screen.getByRole('tab',{name:'产品中心'}));
 await user.type(screen.getByRole('textbox',{name:'产品名称 / 型号'}),'TEST');await user.selectOptions(screen.getByRole('combobox',{name:'产品分类'}),'1');await user.click(screen.getByRole('button',{name:'找产品'}));
 await user.click(await screen.findByRole('button',{name:'下一页'}));expect(location.search).toContain('page=2');
 await user.click(screen.getByRole('button',{name:'English'}));expect(location.pathname).toBe('/v/sample');
 await user.click((await screen.findAllByRole('link',{name:product.data.name}))[0]);await screen.findByRole('heading',{name:product.data.name});
 await user.click(within(document.querySelector('main')!).getByRole('link',{name:messages.en.backVendor}));
 await screen.findByRole('heading',{name:vendor.data.name});expect(location.search).toContain('q=TEST');expect(location.search).toContain('category_id=1');expect(location.search).toContain('page=2');expect(location.hash).toBe('#showroom-products');
 cleanup();render(<BrowserRouter><App/></BrowserRouter>);expect(await screen.findByRole('textbox',{name:'Product name / model'})).toHaveProperty('value','TEST');expect((screen.getByRole('combobox',{name:'Product category'}) as HTMLSelectElement).value).toBe('1');
});
it('shares the canonical company URL without filters and provides manual copy fallback',async()=>{
 history.replaceState(null,'','/v/sample?q=TEST&page=2');render(<BrowserRouter><App/></BrowserRouter>);const user=userEvent.setup();await screen.findByRole('heading',{name:vendor.data.name});
 await user.click(screen.getAllByRole('button',{name:'分享企业展厅'})[0]);const modal=screen.getByRole('dialog');expect((within(modal).getByRole('textbox',{name:'展厅地址'}) as HTMLInputElement).value).toBe('https://showroom.example/v/sample');
 vi.spyOn(navigator.clipboard,'writeText').mockRejectedValueOnce(new Error('clipboard denied'));await user.click(within(modal).getByRole('button',{name:'复制地址'}));expect(await within(modal).findByText('自动复制失败，请选中地址手动复制。')).toBeTruthy();
 expect(within(modal).getByRole('button',{name:'展厅二维码'})).toBeTruthy();expect(within(modal).getByRole('button',{name:'生成海报预览'})).toBeTruthy();
});
it('shows an empty result and excludes sections without company data',async()=>{
 history.replaceState(null,'','/v/sample?q=missing');render(<BrowserRouter><App/></BrowserRouter>);await screen.findByText('没有匹配的产品，请调整关键词或分类。');expect(screen.queryByRole('link',{name:'工厂与产品实拍'})).toBeNull();
});
it('encodes a scannable canonical QR code and wraps long poster names',()=>{
 const url='https://showroom.example/v/sample';const qr=QRCode.create(url,{errorCorrectionLevel:'M'});const size=(qr.modules.size+8)*8;const pixels=new Uint8ClampedArray(size*size*4);pixels.fill(255);
 for(let y=0;y<size;y++)for(let x=0;x<size;x++){const mx=Math.floor(x/8)-4,my=Math.floor(y/8)-4;const dark=mx>=0&&my>=0&&mx<qr.modules.size&&my<qr.modules.size&&qr.modules.get(my,mx);if(dark){const i=(y*size+x)*4;pixels[i]=18;pixels[i+1]=63;pixels[i+2]=50}}
 expect(jsQR(pixels,size,size)?.data).toBe(url);expect(vendorPath(vendor)).toBe('/v/sample');expect(shareURL('/v/sample')).toBe(location.origin+'/v/sample');
 const ctx={measureText:(text:string)=>({width:text.length*40})} as CanvasRenderingContext2D;const long='测试企业名称'.repeat(10);const lines=wrapLines(ctx,long,772);expect(lines.join('')).toBe(long);expect(lines.every(line=>line.length*40<=772)).toBe(true);
});
