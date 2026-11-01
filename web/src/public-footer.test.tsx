// @vitest-environment jsdom
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen,within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {BrowserRouter} from 'react-router-dom';
import {PublicFooter} from './public-footer';
import {ICPRegistration} from './site-common';
import {Language} from './ui';
import {App} from './main';
import {configureRuntime,runtimeConfig} from './runtime-config';
import {emptyData} from './types';
import {withHallFixture} from './hall-test-fixture';

vi.mock('./industry-map',()=>({IndustryMapPage:()=> <div>厂商地图</div>}));
const words:Record<string,string>={brand:'中企展馆',footer:'连接厂家与采购，让产业被看见。',copyright:'© 2026 河北企元数智科技有限公司 版权所有',technicalSupport:'技术支持：河北企元数智科技有限公司',footerNote:'公开展示与联系平台 · 内容审核不代表企业资质认证'};
const vendor={id:1,kind:'vendor',hall_id:1,showroom_slug:'sample',data:{...emptyData,name:'示例厂家',main_products:'设备'}};
beforeEach(()=>{
 localStorage.clear();sessionStorage.clear();history.replaceState(null,'','/');
 configureRuntime({...runtimeConfig(),version:1,capabilities:{procurement:false,service_documents:true},site:{name:'中企展馆',name_en:'ZQZG Exhibition',web_url:'',api_url:'',media_url:'',icp_number:'冀ICP备2026047360号-2'}});
 vi.stubGlobal('scrollTo',vi.fn());
 vi.stubGlobal('fetch',withHallFixture(vi.fn(async(url:string)=>new Response(JSON.stringify(url.includes('/showrooms/')?{item:vendor,canonical_url:'https://example.com/v/sample',product_count:0}:url.includes('/public/products/18')?{item:{...vendor,id:18,kind:'product',data:{...emptyData,name:'示例产品'}},vendor}:url.includes('/public/home')?{vendors:[],products:[],hero:{}}:url.includes('/service-documents')?{items:[{key:'service-info',title:'入驻与服务说明',title_en:'Services',body:'基础入驻免费。',body_en:'Free admission.',version:1,effective_at:'2026-10-31'}]}:{items:[],categories:[],regions:[],total:0,page:1,page_size:20})))));
});
afterEach(()=>{cleanup();vi.unstubAllGlobals();localStorage.clear();sessionStorage.clear()});
function footer(lang:'zh'|'en'='zh'){render(<Language.Provider value={{lang,t:k=>words[k]||k,toggle:()=>{}}}><BrowserRouter><PublicFooter/></BrowserRouter></Language.Provider>);return screen.getByRole('contentinfo')}
it('groups each real service route exactly once and keeps administration in the legal row',()=>{
 const root=footer();const f=within(root);const manufacturers=f.getByRole('navigation',{name:'厂商服务'}),support=f.getByRole('navigation',{name:'帮助与支持'});
 for(const [label,href] of [['免费入驻','/join'],['入驻使用指南','/guide'],['入驻与服务说明','/services']])expect(within(manufacturers).getByRole('link',{name:label}).getAttribute('href')).toBe(href);
 for(const [label,href] of [['投诉与服务申请','/service-requests'],['下载 App','/app'],['企业登录','/merchant/login']])expect(within(support).getByRole('link',{name:label}).getAttribute('href')).toBe(href);
 for(const href of ['/join','/guide','/services','/service-requests','/app','/merchant/login'])expect(root.querySelectorAll(`a[href="${href}"]`)).toHaveLength(1);
 const admin=f.getByRole('link',{name:'管理后台'});expect(admin.getAttribute('href')).toBe('/admin');expect(admin.closest('.public-footer-legal')).toBeTruthy();expect(admin.closest('.public-footer-columns')).toBeNull();expect(root.querySelector('.footer-download')).toBeNull();
 for(const [key,text] of Object.entries(words))if(!['technicalSupport','footerNote'].includes(key))expect(root.textContent).toContain(text);
 expect(root.textContent).not.toContain('技术支持');expect(root.textContent).not.toContain(words.footerNote);expect(root.querySelector('.public-footer-bottom .public-footer-registration')).toBeTruthy();expect(root.querySelector('.public-footer-statement')).toBeNull();
});
it('preserves current filings and official destinations, and keeps legal documents reachable',()=>{
 configureRuntime({...runtimeConfig(),site:{...runtimeConfig().site,icp_number:'实际ICP备案号',police_number:'实际公安备案号',police_url:'https://beian.mps.gov.cn/'}});
 const f=within(footer());for(const [label,href] of [['实际ICP备案号','https://beian.miit.gov.cn/'],['实际公安备案号','https://beian.mps.gov.cn/'],['用户协议','/services/terms'],['隐私政策','/services/privacy'],['企业展厅服务协议','/services/showroom-agreement']])expect(f.getByRole('link',{name:label}).getAttribute('href')).toBe(href);
});
it('can reach administration with keyboard focus and normal navigation',async()=>{
 footer();const admin=screen.getByRole('link',{name:'管理后台'});admin.focus();expect(document.activeElement).toBe(admin);await userEvent.keyboard('{Enter}');expect(location.pathname).toBe('/admin');
});
it('retains the original compact registration/service entries without the three-column footer',()=>{
 render(<Language.Provider value={{lang:'zh',t:k=>words[k]||k,toggle:()=>{}}}><BrowserRouter><ICPRegistration/></BrowserRouter></Language.Provider>);
 expect(document.querySelector('.public-footer')).toBeNull();for(const label of ['入驻与服务说明','入驻使用指南','投诉与服务申请','冀ICP备2026047360号-2'])expect(screen.getByRole('link',{name:label})).toBeTruthy();
});
it('offers equivalent English navigation without changing its destinations',()=>{
 const f=within(footer('en'));expect(f.getByRole('navigation',{name:'Manufacturer services'})).toBeTruthy();expect(f.getByRole('navigation',{name:'Help and support'})).toBeTruthy();expect(f.getByRole('link',{name:'Administration'}).getAttribute('href')).toBe('/admin');expect(f.getByRole('link',{name:'Company sign-in'}).getAttribute('href')).toBe('/merchant/login');
});
it.each(['/','/browse','/browse/products','/products/18','/guide','/services','/service-requests'])('reuses the public footer on %s',async path=>{
 history.replaceState(null,'',path);render(<BrowserRouter><App/></BrowserRouter>);const f=await screen.findByRole('contentinfo');expect(f.classList.contains('public-footer')).toBe(true);expect(within(f).getByRole('link',{name:'管理后台'})).toBeTruthy();
});
it.each(['/join','/merchant/login','/regions','/v/sample'])('keeps the dedicated footer on %s',async path=>{
 history.replaceState(null,'',path);render(<BrowserRouter><App/></BrowserRouter>);const f=await screen.findByRole('contentinfo');expect(f.classList.contains('public-footer')).toBe(false);expect(within(f).queryByRole('link',{name:'管理后台'})).toBeNull();
});
