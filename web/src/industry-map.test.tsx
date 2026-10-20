// @vitest-environment jsdom
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen,waitFor,within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {BrowserRouter} from 'react-router-dom';
import {App} from './main';
import {fittedVendorPageSize} from './industry-map';
import {administrativeFixture,withHallFixture} from './hall-test-fixture';
import {emptyData} from './types';

const graphic=vi.hoisted(()=>({fail:false}));
vi.mock('./industry-map-schematic',()=>({renderSchematicMap:(element:HTMLDivElement)=>{if(graphic.fail)throw Error('diagram failed');element.innerHTML='<svg aria-label="test diagram"></svg>';return {dispose:()=>{element.innerHTML=''},resize:vi.fn(),zoom:vi.fn(),reset:vi.fn()}}}));
let requests:string[]=[];let failStats=false;let failList=false;let delayProvince=false;let releaseProvince:(()=>void)|undefined;
const fixtures=[{id:1,code:'130528',name:'厂商地图测试企业（不入库）'},{id:2,code:'1305',name:'市级资料测试企业（不入库）'},{id:3,code:'110108',name:'北京测试企业（不入库）'},{id:4,code:'',name:'待完善地区测试企业（不入库）'}];
const response=(data:unknown,status=200)=>new Response(JSON.stringify(data),{status});
async function handle(url:string){
 requests.push(url);
 if(url==='/maps/manifest.json')return response({version:1,catalog_version:'2023-06-30',resources:{}});
 if(url.includes('/taxonomy'))return response({categories:[{id:1,name:'农业机械',parent_id:0}],regions:[],halls:[]});
 const p=new URL(url,'http://localhost').searchParams;const code=p.get('region_code')||'';
 let vendors=fixtures.filter(v=>!code||v.code.startsWith(code));if(p.get('q'))vendors=vendors.filter(v=>v.name.includes(p.get('q')!));
 const dir=administrativeFixture('http://localhost?parent_code='+code+'&code='+code);
 if(url.includes('/industry-map/regions')){
  if(failStats)return response({error:{code:'network_error'}},503);
  if(delayProvince&&code==='13')await new Promise<void>(resolve=>{releaseProvince=resolve});
  const items=dir.items.map(node=>({...node,vendor_count:vendors.filter(v=>v.code.startsWith(node.code)).length}));
  return response({region:dir.selected,trail:dir.trail,items,total:vendors.length,unassigned_count:items.length?vendors.filter(v=>!items.some(node=>v.code.startsWith(node.code))).length:0,version:'2023-06-30',coverage:'mainland-31',historical:true});
 }
 if(url.includes('/industry-map/vendors')){
  if(failList)return response({error:{code:'network_error'}},503);
  if(p.get('unassigned')==='1')vendors=vendors.filter(v=>!dir.items.some(node=>v.code.startsWith(node.code)));
  const size=Number(p.get('page_size')||20),page=Number(p.get('page')||1);
  return response({items:vendors.slice((page-1)*size,page*size).map(v=>({id:v.id,kind:'vendor',data:{...emptyData,name:v.name,main_products:'测试设备',region_name:v.code?'河北省（仅测试）':''},showroom_slug:'v-'+v.id})),total:vendors.length,page,page_size:size});
 }
 throw Error('Unexpected request '+url);
}
beforeEach(()=>{requests=[];failStats=false;failList=false;graphic.fail=false;delayProvince=false;releaseProvince=undefined;history.replaceState(null,'','/regions');localStorage.clear();sessionStorage.clear();vi.stubGlobal('fetch',withHallFixture(vi.fn(handle)));vi.stubGlobal('scrollTo',vi.fn());vi.stubGlobal('matchMedia',vi.fn(()=>({matches:false,addEventListener:vi.fn(),removeEventListener:vi.fn()})))});
afterEach(()=>{releaseProvince?.();cleanup();vi.unstubAllGlobals()});
function mount(){return render(<BrowserRouter><App/></BrowserRouter>)}
function vendors(){return screen.getByRole('region',{name:'厂商'})}
async function chartReady(){await waitFor(()=>expect(document.querySelector('.industry-chart svg')).toBeTruthy())}
async function choose(name:RegExp){await userEvent.click(await screen.findByRole('button',{name:'选择地区'}));await userEvent.click(await within(screen.getByRole('dialog',{name:'选择地区'})).findByRole('button',{name}))}

it('shows a schematic with an empty verified manifest without requesting boundary assets, fits sidebar pages and keeps data notes compact',async()=>{
 mount();await chartReady();await within(vendors()).findByRole('heading',{name:'厂商地图测试企业（不入库）'});
 expect(screen.getByRole('heading',{level:1,name:'厂商地图'})).toBeTruthy();expect(document.querySelector('.factory-map-shell')).toBeTruthy();expect(document.querySelector('.industry-region-grid')).toBeNull();
 expect(requests.findIndex(url=>url.includes('/industry-map/regions'))).toBeLessThan(requests.findIndex(url=>url.includes('/industry-map/vendors')));
 expect(requests.find(url=>url.includes('/industry-map/vendors'))).toContain('page_size=4');expect(screen.queryByText(/历史目录不代表/)).toBeNull();
 expect(requests.some(url=>url.startsWith('/maps/')&&url!=='/maps/manifest.json')).toBe(false);expect(screen.getByText('行政区划示意图，非真实地理边界')).toBeTruthy();
 await userEvent.click(screen.getByRole('button',{name:'数据说明'}));expect(screen.getByText(/历史目录不代表/)).toBeTruthy();
});
it('navigates through the searchable region picker, restores history and retains a showroom return location',async()=>{
 mount();await chartReady();await choose(/河北省.*2/);await waitFor(()=>expect(location.search).toContain('region_code=13'));
 await choose(/邢台市.*2/);await choose(/宁晋县.*1/);await within(vendors()).findByRole('heading',{name:'厂商地图测试企业（不入库）'});
 expect(screen.getByRole('button',{name:'选择地区'}).hasAttribute('disabled')).toBe(true);
 history.back();await waitFor(()=>expect(new URLSearchParams(location.search).get('region_code')).toBe('1305'));
 history.forward();await waitFor(()=>expect(new URLSearchParams(location.search).get('region_code')).toBe('130528'));
 expect((await within(vendors()).findByRole('link',{name:'进入展厅'})).getAttribute('href')).toBe('/v/v-1');
 await userEvent.click(screen.getByRole('button',{name:'返回上一级'}));await waitFor(()=>expect(new URLSearchParams(location.search).get('region_code')).toBe('1305'));
 await userEvent.click(screen.getByRole('button',{name:'返回全国'}));await waitFor(()=>expect(new URLSearchParams(location.search).has('region_code')).toBe(false));
});
it('filters region choices without rendering the full catalogue and closes the picker on Escape',async()=>{
 mount();await chartReady();await userEvent.click(screen.getByRole('button',{name:'选择地区'}));
 await userEvent.type(screen.getByRole('textbox',{name:'搜索下级地区'}),'河北');const dialog=screen.getByRole('dialog',{name:'选择地区'});expect(within(dialog).getAllByRole('button')).toHaveLength(1);
 await userEvent.keyboard('{Escape}');expect(screen.queryByRole('dialog',{name:'选择地区'})).toBeNull();
});
it('keeps unresolved manufacturers separate from real region geometry',async()=>{
 mount();await userEvent.click(await screen.findByRole('button',{name:/待完善地区 1/}));await within(vendors()).findByRole('heading',{name:'待完善地区测试企业（不入库）'});expect(within(vendors()).queryByRole('heading',{name:'厂商地图测试企业（不入库）'})).toBeNull();
 await userEvent.click(screen.getByRole('button',{name:'全部厂商'}));await within(vendors()).findByRole('heading',{name:'厂商地图测试企业（不入库）'});
});
it('clears pagination on filtering and suppresses late regional responses',async()=>{
 history.replaceState(null,'','/regions?page=2&view=vendors');mount();await chartReady();await userEvent.selectOptions(screen.getByRole('combobox',{name:'行业分类'}),'1');await waitFor(()=>expect(location.search).not.toContain('page='));
 delayProvince=true;await choose(/河北省.*2/);await waitFor(()=>expect(releaseProvince).toBeTruthy());await userEvent.click(within(screen.getByRole('navigation',{name:'地区路径'})).getByRole('button',{name:'全国'}));
 await chartReady();releaseProvince?.();await new Promise(resolve=>setTimeout(resolve,20));expect(new URLSearchParams(location.search).has('region_code')).toBe(false);
});
it('preserves filters and supports map/list view switching and empty results',async()=>{
 mount();await chartReady();await userEvent.click(screen.getByRole('button',{name:/^厂商 /}));expect(document.querySelector('.industry-map-page')?.getAttribute('data-view')).toBe('vendors');
 await userEvent.type(screen.getByRole('textbox',{name:'关键词'}),'不存在的厂商');await userEvent.click(screen.getByRole('button',{name:'搜索'}));await within(vendors()).findByText('当前条件下暂无公开厂商');await userEvent.click(screen.getAllByRole('button',{name:'清除筛选'})[0]);await within(vendors()).findByRole('heading',{name:'厂商地图测试企业（不入库）'});
});
it('retries map and list failures independently',async()=>{
 graphic.fail=true;failList=true;mount();await screen.findByText(/地图图形加载失败/);await within(vendors()).findByRole('alert');failList=false;await userEvent.click(within(vendors()).getByRole('button',{name:'重试'}));await within(vendors()).findByRole('heading',{name:'厂商地图测试企业（不入库）'});
 graphic.fail=false;await userEvent.click(within(screen.getByRole('region',{name:'厂商分布地图'})).getByRole('button',{name:'重试'}));await chartReady();
});
it('does not request a manufacturer page when statistics fail',async()=>{
 failStats=true;mount();await within(await screen.findByRole('region',{name:'厂商分布地图'})).findByRole('alert');expect(requests.some(url=>url.includes('/industry-map/vendors'))).toBe(false);failStats=false;await userEvent.click(within(screen.getByRole('region',{name:'厂商分布地图'})).getByRole('button',{name:'重试'}));await chartReady();
});
it('caps page size to the visible card capacity and never requests more than eight manufacturers',()=>{
 expect(fittedVendorPageSize(96)).toBe(1);expect(fittedVendorPageSize(199)).toBe(1);expect(fittedVendorPageSize(200)).toBe(2);expect(fittedVendorPageSize(312)).toBe(3);expect(fittedVendorPageSize(2000)).toBe(8);
});
