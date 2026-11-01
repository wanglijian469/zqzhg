// @vitest-environment jsdom
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen,waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {BrowserRouter} from 'react-router-dom';
import {App} from './main';
import {Language} from './ui';
import {HallProvider} from './halls';
import {ExhibitionPortal} from './exhibition-portal';
import {CompanyOverview,CompanyProduction,CompanyCredentials} from './company-portal';
import {CompanyWebsitePreview} from './company-website-preview';
import {configureRuntime,runtimeConfig} from './runtime-config';
import {withHallFixture} from './hall-test-fixture';
import {emptyData,type Lang} from './types';

const data={...emptyData,name:'ACME 示例厂商',main_products:'RX-800 设备',description:'ACME 产品介绍',region_name:'河北省',company_profile:{equipment:'AX-900 生产设备',capacity:'每月30台'}};
const vendor={id:1,kind:'vendor',hall_id:1,showroom_slug:'sample',data};
const product={id:2,kind:'product',vendor_id:1,data:{...emptyData,name:'RX-800 设备'}};
beforeEach(()=>{
 localStorage.clear();sessionStorage.clear();history.replaceState(null,'','/');vi.stubGlobal('scrollTo',vi.fn());
 configureRuntime({...runtimeConfig(),version:1,site:{...runtimeConfig().site,name:'中企展馆',name_en:'ZQZG Exhibition'},capabilities:{procurement:false}});
 vi.stubGlobal('fetch',withHallFixture(vi.fn(async(url:string)=>new Response(JSON.stringify(url.includes('/public/home')?{vendors:[vendor],products:[product],hero:{}}:url.includes('/showrooms/')?{item:vendor,canonical_url:'https://example.com/v/sample',product_count:1,home_products:[product]}:{items:[],categories:[],regions:[],total:0})))));
});
afterEach(()=>{cleanup();vi.unstubAllGlobals();localStorage.clear();sessionStorage.clear()});
it.each(['zh','en'] as Lang[])('keeps credential names while showing decorative qualification headings only in %s',lang=>{
 const credentialData={...data,company_profile:{...data.company_profile,credentials:[{kind:'qualification' as const,media_id:0,title:'ISO 9001 示例证书'},{kind:'award' as const,media_id:0,title:'ACME 示例奖项'}]}};
 render(<Language.Provider value={{lang,t:k=>k,toggle:()=>{}}}><CompanyCredentials data={credentialData}/></Language.Provider>);
 for(const text of ['QUALIFICATIONS','HONORS & AWARDS'])expect(screen.queryByText(text,{exact:true})!==null).toBe(lang==='en');
 expect(screen.getByText('ISO 9001 示例证书')).toBeTruthy();expect(screen.getByText('ACME 示例奖项')).toBeTruthy();
});
function mount(path='/'){history.replaceState(null,'',path);render(<BrowserRouter><App/></BrowserRouter>)}
function noEnglishDecorations(){for(const text of ['ZQZG / INDUSTRIAL EXHIBITION','FOR MANUFACTURERS','INDUSTRIAL EXHIBITION','MANUFACTURING / PRODUCTS / CONNECTIONS','ABOUT THE COMPANY','MANUFACTURING'])expect(screen.queryByText(text,{exact:true})).toBeNull()}
it('removes homepage decoration nodes in Chinese while preserving hall, company and model information',async()=>{
 mount();await screen.findByRole('heading',{name:'ACME 示例厂商'});noEnglishDecorations();expect(document.querySelector('.hero-eyebrow')).toBeNull();expect(document.querySelector('.stage-baseline')).toBeNull();expect(document.querySelector('.stage-english-label')).toBeNull();expect(document.querySelector('.stage-overline')?.textContent).toBe('农业机械厅');expect(screen.getAllByText('RX-800 设备').length).toBeGreaterThan(0);expect(screen.getAllByRole('link',{name:'下载 App'}).length).toBeGreaterThan(0);
});
it('shows all three English homepage eyebrows and other labels only in English, persists and switches back',async()=>{
 mount();await screen.findByRole('heading',{name:'ACME 示例厂商'});await userEvent.click(screen.getByRole('button',{name:'English'}));
 await waitFor(()=>expect(screen.getAllByText('ZQZG / INDUSTRIAL EXHIBITION',{exact:true})).toHaveLength(3));expect(screen.getByText('FOR MANUFACTURERS',{exact:true})).toBeTruthy();expect(screen.getByText('MANUFACTURING / PRODUCTS / CONNECTIONS',{exact:true})).toBeTruthy();expect(document.querySelector('.stage-english-label')?.textContent).toBe('INDUSTRIAL EXHIBITION');expect(localStorage.getItem('zqzt.language')).toBe('en');
 cleanup();mount();await waitFor(()=>expect(screen.getAllByText('ZQZG / INDUSTRIAL EXHIBITION',{exact:true})).toHaveLength(3));await userEvent.click(screen.getByRole('button',{name:'中文'}));await screen.findByRole('heading',{name:'中企展馆-农业机械厅'});noEnglishDecorations();expect(document.querySelector('.stage-baseline')).toBeNull();
});
it('uses the same language rule for a preparing hall without removing its Chinese heading',async()=>{
 mount('/?hall=cable');await screen.findByRole('heading',{name:'中企展馆 · 电线电缆厅'});noEnglishDecorations();await userEvent.click(screen.getByRole('button',{name:'English'}));expect(screen.getByText('ZQZG / INDUSTRIAL EXHIBITION',{exact:true})).toBeTruthy();await userEvent.click(screen.getByRole('button',{name:'中文'}));noEnglishDecorations();expect(screen.getByRole('heading',{name:'中企展馆 · 电线电缆厅'})).toBeTruthy();
});
it.each(['zh','en'] as Lang[])('renders the hall selection decoration only when appropriate for %s',async lang=>{
 render(<Language.Provider value={{lang,t:k=>k==='brand'?(lang==='zh'?'中企展馆':'ZQZG Exhibition'):k,toggle:()=>{}}}><BrowserRouter><HallProvider><ExhibitionPortal/></HallProvider></BrowserRouter></Language.Provider>);
 await screen.findByRole('heading',{name:lang==='zh'?'农业机械厅':'Agricultural Machinery Hall'});if(lang==='zh')expect(screen.queryByText('ZQZG',{exact:true})).toBeNull();else expect(screen.getByText('ZQZG',{exact:true})).toBeTruthy();
});
it.each(['zh','en'] as Lang[])('applies language visibility to company sections and their preview in %s without mutating data',lang=>{
 const before=JSON.stringify(data);render(<Language.Provider value={{lang,t:k=>k,toggle:()=>{}}}><BrowserRouter><HallProvider><CompanyOverview data={data}/><CompanyProduction data={data}/><CompanyWebsitePreview data={data}/></HallProvider></BrowserRouter></Language.Provider>);
 for(const text of ['ABOUT THE COMPANY','MANUFACTURING']){if(lang==='zh')expect(screen.queryByText(text,{exact:true})).toBeNull();else expect(screen.getAllByText(text,{exact:true})).toHaveLength(2)}expect(screen.getAllByText('AX-900 生产设备').length).toBeGreaterThan(0);expect(screen.getAllByText('ACME 产品介绍').length).toBeGreaterThan(0);expect(JSON.stringify(data)).toBe(before);
});
