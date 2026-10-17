// @vitest-environment jsdom
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen,within,waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {BrowserRouter} from 'react-router-dom';
import {App} from './main';
import {emptyData} from './types';
let role='operator';let permissions:string[]=[];
const json=(data:unknown,status=200)=>new Response(JSON.stringify(data),{status});
const fetchMock=vi.fn(async(url:string,options?:RequestInit):Promise<Response>=>{
 if(url.includes('/admin/dashboard'))return json({counts:{vendor_total:1},recent_operations:[],account:{id:4,username:'operator',role,permissions}});
 if(url.includes('/admin/contents?'))return json({items:[{id:1,kind:'vendor',status:'draft',revision_id:2,data:{...emptyData,name:'权限测试厂商'},visible:true,public_visible:false}],total:1,page:1,page_size:20});
 if(url.includes('/taxonomy'))return json({categories:[],regions:[],halls:[]});
 if(url.includes('/admin/analytics'))return json({summary:{pv:3,uv:1,visits:1,contacts:2,searches:1},daily:[],rankings:{},sources:[],devices:[],referrers:[],detail_available:true});
 if(url.includes('/admin/permissions'))return json({items:[{key:'vendor.read',name:'查看厂商',name_en:'View'},{key:'vendor.review',name:'审核厂商',name_en:'Review'}]});
 if(url.endsWith('/admin/operators')&&options?.method==='POST')return json({username:'tester',initial_password:'TestOnly-Password!'},201);
 if(url.endsWith('/admin/operators'))return json({items:[]});
 if(url.includes('/admin/storage/profiles'))return json({items:[{profile:{id:0,name:'本地目录',kind:'local',local_root:'D:/private'},active:true,credentials_configured:false}]});
 if(url.includes('/admin/backups'))return json({items:[],policy:{enabled:false,time:'02:00',keep:7,target_id:0,timezone:'Asia/Shanghai'}});
 if(url.includes('/admin/jobs'))return json({items:[]});
 return json({ok:true});
});
beforeEach(()=>{sessionStorage.clear();localStorage.clear();sessionStorage.setItem('zqzt.admin','test-only');permissions=[];role='operator';fetchMock.mockClear();vi.stubGlobal('fetch',fetchMock);vi.stubGlobal('scrollTo',vi.fn())});
afterEach(()=>{cleanup();vi.unstubAllGlobals()});
function mount(path:string){history.replaceState(null,'',path);render(<BrowserRouter><App/></BrowserRouter>)}
it('uses effective permissions for the menu and vendor action buttons',async()=>{permissions=['vendor.read'];mount('/admin/vendors');await screen.findByText('权限测试厂商');const nav=document.querySelector('.admin-navigation')!;expect(within(nav as HTMLElement).queryByText('厂商账号')).toBeNull();expect(within(nav as HTMLElement).queryByText('产品管理')).toBeNull();expect(screen.queryByRole('button',{name:'新增厂商'})).toBeNull();expect(screen.queryByRole('button',{name:'下架'})).toBeNull();expect(screen.getAllByRole('checkbox').every(v=>(v as HTMLInputElement).disabled)).toBe(true);expect(screen.getByRole('link',{name:'查看内容'}).getAttribute('href')).toBe('/admin/contents/1')});
it('blocks a direct privileged route before mounting the page',async()=>{permissions=['analytics.read'];mount('/admin/storage');await screen.findByRole('alert');expect(screen.getByRole('alert').textContent).toContain('403');expect(fetchMock.mock.calls.some(([url])=>url.includes('/storage/profiles'))).toBe(false)});
it('shows analytics to readers and keeps export restricted',async()=>{permissions=['analytics.read'];mount('/admin/analytics');await screen.findByText('匿名访客');expect(screen.queryByRole('button',{name:'CSV'})).toBeNull();expect(screen.getByText('联系点击')).toBeTruthy()});
it('saves dependent operator grants and shows generated credentials once',async()=>{role='admin';mount('/admin/operators');await screen.findByRole('button',{name:'新增操作员'});await userEvent.click(screen.getByRole('button',{name:'新增操作员'}));await userEvent.type(screen.getByRole('textbox',{name:'登录账号'}),'tester');await userEvent.click(screen.getByRole('checkbox',{name:'审核厂商'}));expect((screen.getByRole('checkbox',{name:'查看厂商'}) as HTMLInputElement).checked).toBe(true);await userEvent.click(screen.getByRole('button',{name:'保存并撤销旧会话'}));await screen.findByText('TestOnly-Password!');const call=fetchMock.mock.calls.find(([url,options])=>url.endsWith('/admin/operators')&&options?.method==='POST');expect(JSON.parse(call![1]!.body as string).permissions).toEqual(expect.arrayContaining(['vendor.read','vendor.review']))});
it('renders disabled backup policy and separate local storage without secrets',async()=>{role='admin';mount('/admin/backups');await screen.findByRole('button',{name:'创建完整备份'});await waitFor(()=>expect((screen.getByRole('checkbox',{name:'启用每天备份'}) as HTMLInputElement).checked).toBe(false));expect(screen.getByRole('button',{name:'准备回退'})).toBeTruthy();expect(screen.getByRole('option',{name:'本机私有备份目录'})).toBeTruthy()});