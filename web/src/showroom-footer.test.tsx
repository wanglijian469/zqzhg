// @vitest-environment jsdom
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen,within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {BrowserRouter} from 'react-router-dom';
import {App} from './main';
import {ShareShowroom} from './company-profile';
import {Language} from './ui';
import {configureRuntime,runtimeConfig} from './runtime-config';
import {emptyData,type Item} from './types';
import {withHallFixture} from './hall-test-fixture';

const vendor={id:14,kind:'vendor',hall_id:1,showroom_slug:'sample',data:{...emptyData,name:'示例生产厂商',main_products:'机械配件'}} as Item;
beforeEach(()=>{
 localStorage.clear();sessionStorage.clear();
 configureRuntime({...runtimeConfig(),version:1,capabilities:{procurement:false},site:{name:'中企展馆',name_en:'ZQZG Exhibition',web_url:'',api_url:'',media_url:'',icp_number:'冀ICP备2026047360号-2'}});
 vi.stubGlobal('scrollTo',vi.fn());
 vi.stubGlobal('fetch',withHallFixture(vi.fn(async(url:string)=>new Response(JSON.stringify(url.includes('/public/showrooms/')?{item:vendor,canonical_url:'https://example.com/v/sample',product_count:0}:url.includes('/public/home')?{vendors:[],products:[],hero:{}}:{items:[],categories:[],regions:[],total:0,page:1,page_size:20})))));
});
afterEach(()=>{cleanup();vi.unstubAllGlobals();localStorage.clear();sessionStorage.clear()});
function mount(path:string){history.replaceState(null,'',path);render(<BrowserRouter><App/></BrowserRouter>)}
it.each(['/v/sample','/v/v-14','/vendors/14'])('keeps company identity, attribution, filing and sharing without platform service links on %s',async path=>{
 mount(path);const footer=await screen.findByRole('contentinfo');const f=within(footer);
 expect(f.getByText(vendor.data.name,{exact:true})).toBeTruthy();expect(footer.textContent).toContain('由中企展馆提供展示服务');expect(f.getByRole('link',{name:'中企展馆'}).getAttribute('href')).toBe('/');
 expect(f.getByRole('link',{name:'冀ICP备2026047360号-2'}).getAttribute('href')).toBe('https://beian.miit.gov.cn/');
 expect(footer.textContent).not.toMatch(/版权所有|技术支持|河北企元数智|入驻与服务说明|入驻使用指南|投诉与服务申请|分享官网/);
 for(const href of ['/services','/guide','/service-requests'])expect(footer.querySelector(`a[href="${href}"]`)).toBeNull();
 await userEvent.click(f.getByRole('button',{name:'分享展厅'}));const modal=screen.getByRole('dialog');expect(within(modal).getByRole('button',{name:'分享展厅'})).toBeTruthy();expect((within(modal).getByRole('textbox',{name:'展厅地址'}) as HTMLInputElement).value).toBe('https://example.com/v/sample');
 expect(within(modal).getByRole('button',{name:'复制地址'})).toBeTruthy();expect(within(modal).getByRole('button',{name:'展厅二维码'})).toBeTruthy();expect(within(modal).getByRole('button',{name:'生成海报预览'})).toBeTruthy();
});
it('leaves the platform footer complete and its complaint entry usable',async()=>{
 mount('/');const footer=await screen.findByRole('contentinfo');const f=within(footer);expect(footer.textContent).toContain('河北企元数智科技有限公司');expect(footer.textContent).not.toMatch(/技术支持/);
 for(const name of ['入驻与服务说明','入驻使用指南','投诉与服务申请','冀ICP备2026047360号-2'])expect(f.getByRole('link',{name})).toBeTruthy();
 await userEvent.click(f.getByRole('link',{name:'投诉与服务申请'}));expect(await screen.findByRole('heading',{name:'投诉与服务申请'})).toBeTruthy();expect(location.pathname).toBe('/service-requests');
});
it('uses the current filing number and preserves configured police registration',async()=>{
 configureRuntime({...runtimeConfig(),site:{...runtimeConfig().site,icp_number:'示例ICP备案号',police_number:'示例公安备案号',police_url:'https://beian.mps.gov.cn/'}});mount('/v/sample');const f=within(await screen.findByRole('contentinfo'));expect(f.getByRole('link',{name:'示例ICP备案号'})).toBeTruthy();expect(f.getByRole('link',{name:'示例公安备案号'}).getAttribute('href')).toBe('https://beian.mps.gov.cn/');
});
it('keeps sharing text on other pages unchanged when no showroom-specific label is supplied',()=>{
 render(<Language.Provider value={{lang:'zh',toggle:()=>{},t:k=>k==='shareShowroom'?'分享官网':k}}><ShareShowroom item={vendor} canonical="https://example.com/v/sample" close={()=>{}} inline/></Language.Provider>);
 expect(screen.getByRole('button',{name:'分享官网'})).toBeTruthy();expect(screen.queryByRole('button',{name:'分享展厅'})).toBeNull();
});
