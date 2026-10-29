// @vitest-environment jsdom
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {cleanup,fireEvent,render,screen,waitFor,within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {BrowserRouter} from 'react-router-dom';
import {App} from './main';
import {withHallFixture} from './hall-test-fixture';
import {emptyData} from './types';
import {configureRuntime,runtimeConfig} from './runtime-config';

let noticeItems=false;let registered=false,draftScope=false,delayedProduct=false,expandedScope=false,accountRole='merchant',notificationUnread=0,failDraft=false;let calls:{url:string;method:string;data:Record<string,unknown>}[]=[];
const vendor={id:3,kind:'vendor',hall_id:1,showroom_slug:'supplier-test',status:'approved',public_visible:true,revision_id:1,data:{...emptyData,name:'已有厂商'}};
const product={id:7,kind:'product',hall_id:1,vendor_id:3,data:{...emptyData,name:'测试旋耕机',category_id:101,parameters:'型号：T-7'}};
const items=[{id:11,category_id:101,name:'旋耕机',specification:'80千瓦',quantity:'2.500',unit:'台'},{id:12,category_id:101,name:'配件',specification:'',quantity:'10.000',unit:'件'}];
const project={id:9,hall_id:1,state:'open',audience:'direct',version:2,published_revision_id:4,draft_revision_id:4,source_product_id:7,owner:false,participant:{id:1,vendor_id:3,state:'invited',intent:false,source:'direct'},revision:{id:4,status:'approved',title:'采购测试项目',delivery_region:'13',expected_delivery:'15天',deadline:new Date(Date.now()+86400000).toISOString(),notes:'技术要求'},items,attachments:[]};
const response=(data:unknown)=>new Response(JSON.stringify(data));
beforeEach(()=>{configureRuntime({...runtimeConfig(),capabilities:{procurement:true}});noticeItems=false;registered=false;draftScope=false;delayedProduct=false;expandedScope=false;accountRole='merchant';notificationUnread=0;failDraft=false;calls=[];localStorage.clear();sessionStorage.clear();vi.stubGlobal('scrollTo',vi.fn());vi.stubGlobal('fetch',withHallFixture(vi.fn(async(url:string,options?:RequestInit)=>{
 const method=options?.method||'GET';const data=typeof options?.body==='string'?JSON.parse(options.body):{};calls.push({url,method,data});const account={id:3,role:accountRole,phone:'13900000003',phone_verified:true,buyer_registered:registered};
 if(url.includes('/public/config'))return response({version:1,site:{name:'中企展馆',name_en:'ZQZG',web_url:'',api_url:'',media_url:''},uploads:{image_mib:10,video_mib:200,pdf_mib:50,default_quota_bytes:2147483648},capabilities:{procurement:true}});
 if(url.endsWith('/auth/me'))return response(account);
 if(url.endsWith('/auth/phone/code'))return response({sent:true,development_code:'420731',retry_after:60});
 if(url.endsWith('/auth/phone/login'))return response({token:'sms-business-token',account});
 if(url.endsWith('/procurement/profile')){if(method==='PUT')registered=true;return response({profile:registered?{account_id:3,company:data.company||'已有单位',contact_name:data.contact_name||'采购员',region_code:'',version:1,suspended:false}:null,account})}
 if(url.includes('/public/products/7')){if(delayedProduct)await new Promise(resolve=>setTimeout(resolve,200));return response({item:product,vendor})}
 if(url.includes('/procurement/projects/9/quotes'))return response(method==='POST'?{quote:{id:1,version:1},revision:{id:8}}:{items:[]});
 if(url.includes('/procurement/projects/9/messages'))return response({items:[],total:0,page:1,page_size:20});
 if(url.includes('/procurement/projects/9/draft')&&method==='PUT')return response({id:9,version:3});
 if(url.includes('/procurement/projects/9'))return response({...project,audience:expandedScope?'public':project.audience,draft_revision_id:draftScope?5:project.draft_revision_id,revision:{...project.revision,id:draftScope?5:project.revision.id,...(draftScope?{requested_audience:'designated'}:{})},requested_vendor_ids:draftScope?[3]:[],contact:(location.pathname.startsWith('/buyer')&&!location.pathname.startsWith('/merchant/opportunities'))?{name:'项目采购员',phone:'13900000003'}:null,owner:!url.includes('supplier')&&(location.pathname.startsWith('/buyer')&&!location.pathname.startsWith('/merchant/opportunities'))});
 if(url.endsWith('/procurement/projects')&&method==='POST')return failDraft?new Response(JSON.stringify({error:{code:'internal_error'}}),{status:500}):response({id:9,version:1});
 if(url.includes('/procurement/notifications'))return response({items:noticeItems&&method==='GET'?[{id:2,project_id:9,title:'身份消息',kind:'message',created_at:'2026-10-23T03:00:00Z',target_scope:url.includes('scope=supplier')?'supplier':'buyer'}]:[],total:0,page:1,page_size:20,unread:notificationUnread});
 if(url.includes('/public/administrative-regions'))return response({items:[{code:'13',name:'河北省'}],selected:null});
 if(url.includes('/taxonomy'))return response({categories:[{id:101,name:'整机',parent_id:0,hall_id:1,enabled:true,selectable:true,category_kind:'product'}],regions:[],halls:[]});
 if(url.includes('/merchant/contents?'))return response({items:[vendor],total:1,page:1,page_size:20});
 if(url.includes('/merchant/dashboard'))return response({products:{total:0,public:0,draft:0,pending:0,rejected:0}});
 return response({items:[],total:0,page:1,page_size:20});
 })))});
afterEach(()=>{cleanup();vi.unstubAllGlobals();localStorage.clear();sessionStorage.clear()});
function mount(path:string,signed=false){if(signed){sessionStorage.setItem('zqzt.merchant','existing-token');sessionStorage.setItem('zqzt.merchant.id','3')}history.replaceState(null,'',path);render(<BrowserRouter><App/></BrowserRouter>)}
it('keeps the product inquiry destination while an anonymous visitor signs in',async()=>{
 mount('/products/7');await userEvent.click(await screen.findByRole('link',{name:'立即询价'}));expect(await screen.findByRole('heading',{name:'采购商登录'})).toBeTruthy();expect(location.pathname+location.search).toBe('/buyer/new?product=7');expect(calls.some(c=>c.url.endsWith('/auth/sms'))).toBe(false);
});
it('adds a buyer profile to the signed-in company account and continues the original product inquiry',async()=>{
 mount('/buyer/new?product=7',true);await userEvent.type(await screen.findByRole('textbox',{name:'联系人 *'}),'原账号采购员');await userEvent.click(screen.getByRole('button',{name:'保存并继续'}));await screen.findByRole('heading',{name:'产品询价'});await screen.findByDisplayValue('测试旋耕机');expect(sessionStorage.getItem('zqzt.merchant')).toBe('existing-token');expect(calls.filter(c=>c.url.includes('/register'))).toHaveLength(0);expect(calls.find(c=>c.url.endsWith('/procurement/profile')&&c.method==='PUT')?.data).toMatchObject({contact_name:'原账号采购员',version:0});expect(location.search).toContain('product=7');
});
it('submits a decimal quote for one item while leaving the other item explicitly unquoted',async()=>{
 registered=true;mount('/merchant/opportunities/projects/9',true);const statuses=await screen.findAllByRole('combobox',{name:'供货状态'});expect(screen.getByRole('link',{name:'询价与报价'}).getAttribute('aria-current')).toBe('page');await userEvent.selectOptions(statuses[0],'quoted');fireEvent.change(screen.getByLabelText('单价（元）*',{exact:true}),{target:{value:'12.3456'}});await userEvent.click(screen.getByRole('button',{name:'提交报价'}));await waitFor(()=>expect(calls.some(c=>c.method==='POST'&&c.url.includes('/projects/9/quotes'))).toBe(true));const sent=calls.find(c=>c.method==='POST'&&c.url.includes('/projects/9/quotes'))!.data;expect(sent).toMatchObject({demand_revision_id:4,submit:true});expect(sent.items).toEqual([expect.objectContaining({item_id:11,status:'quoted',unit_price:'12.3456'}),expect.objectContaining({item_id:12,status:'unquoted',unit_price:''})]);
});
it('retains the draft invitation scope and selected suppliers when resuming and saving',async()=>{
 registered=true;draftScope=true;mount('/buyer/projects/9/edit',true);
 const scope=await screen.findByRole('combobox',{name:'参与方式'});expect((scope as HTMLSelectElement).value).toBe('designated');
 await screen.findByDisplayValue('项目采购员');await userEvent.click(screen.getByRole('button',{name:'保存草稿'}));
 await waitFor(()=>expect(calls.some(c=>c.method==='PUT'&&c.url.endsWith('/projects/9/draft'))).toBe(true));
 expect(calls.find(c=>c.method==='PUT'&&c.url.endsWith('/projects/9/draft'))?.data).toMatchObject({audience:'designated',vendor_ids:[3],contact_name:'项目采购员'});
});

it('waits for the source product before allowing inquiry edits',async()=>{
 registered=true;delayedProduct=true;mount('/buyer/new?product=7',true);
 const title=await screen.findByRole('textbox',{name:'项目名称 *'});expect(title.matches(':disabled')).toBe(true);
 await waitFor(()=>expect(title.matches(':disabled')).toBe(false));
 await userEvent.clear(title);await userEvent.type(title,'我的采购项目名称');
 expect((title as HTMLInputElement).value).toBe('我的采购项目名称');
});

it('uses the approved expanded scope when revising the current published demand',async()=>{
 registered=true;expandedScope=true;mount('/buyer/projects/9/edit',true);
 expect((await screen.findByRole('combobox',{name:'参与方式'}) as HTMLSelectElement).value).toBe('public');
});

it('offers one public procurement entry and opens the public hall for guests',async()=>{
 mount('/products/7');const entry=await screen.findByRole('link',{name:'采购中心'});expect(screen.getAllByRole('link',{name:'采购中心'})).toHaveLength(1);expect(entry.getAttribute('href')).toBe('/procurement');expect(screen.queryByRole('link',{name:'采购需求'})).toBeNull();
 await userEvent.click(entry);expect(await screen.findByRole('heading',{name:'采购需求大厅'})).toBeTruthy();expect(calls.some(c=>c.url.endsWith('/auth/me'))).toBe(false);
 const nav=screen.getByRole('navigation',{name:'采购中心导航'});expect(within(nav).getAllByRole('link').map(link=>link.textContent)).toEqual(['我的询价与需求','发布采购需求','采购需求大厅','消息通知','采购商资料']);expect(within(nav).queryByRole('link',{name:'厂商商机'})).toBeNull();
});
it('opens personal requests from the merged entry for a signed-in business account',async()=>{
 registered=true;mount('/products/7',true);const entry=await screen.findByRole('link',{name:'采购中心'});expect(entry.getAttribute('href')).toBe('/buyer');await userEvent.click(entry);expect(await screen.findByRole('heading',{name:'我的询价与采购需求'})).toBeTruthy();
});
it('keeps request details active, notification counts and independent collapse preferences',async()=>{
 registered=true;notificationUnread=4;sessionStorage.setItem('zqzt.merchant.collapsed','1');mount('/buyer/projects/9/edit',true);
 const nav=await screen.findByRole('navigation',{name:'采购中心导航'});expect(within(nav).getByRole('link',{name:'我的询价与需求'}).getAttribute('aria-current')).toBe('page');expect(await within(nav).findByRole('link',{name:'消息通知 (4)'})).toBeTruthy();
 expect(document.querySelector('.procurement-workspace')!.classList.contains('is-collapsed')).toBe(false);await userEvent.click(screen.getByRole('button',{name:'收起采购导航'}));expect(sessionStorage.getItem('zqzt.procurement.collapsed')).toBe('1');expect(sessionStorage.getItem('zqzt.merchant.collapsed')).toBe('1');await userEvent.click(screen.getByRole('button',{name:'展开采购导航'}));expect(sessionStorage.getItem('zqzt.procurement.collapsed')).toBe('0');
});
it('offers same-account registration in the supplier workspace without loading merchant data',async()=>{
 registered=true;accountRole='visitor';mount('/merchant/opportunities',true);
 expect(await screen.findByText('厂商商机面向已入驻厂商开放，可使用当前账号入驻，无须重新注册。')).toBeTruthy();expect(screen.getByRole('button',{name:'使用同一账号入驻厂商'})).toBeTruthy();expect(location.pathname+location.search).toBe('/merchant/opportunities');expect(calls.some(c=>c.url.includes('/merchant/dashboard'))).toBe(false);
});
it('routes an existing buyer from the public join entry to same-account onboarding',async()=>{
 registered=true;accountRole='visitor';sessionStorage.setItem('zqzt.merchant.role','visitor');mount('/join',true);
 expect(await screen.findByRole('button',{name:'使用同一账号入驻厂商'})).toBeTruthy();expect(location.pathname).toBe('/merchant');
 expect(calls.some(c=>c.url.includes('/merchant/contents')||c.url.includes('/merchant/dashboard'))).toBe(false);
 expect(calls.some(c=>c.method==='POST')).toBe(false);
});
it('closes the procurement drawer on Escape, overlay and navigation without leaving a scroll lock',async()=>{
 mount('/procurement');const toggle=await screen.findByRole('button',{name:'打开采购中心导航'});await userEvent.click(toggle);expect(screen.getByRole('dialog',{name:'采购中心'})).toBeTruthy();expect(document.body.style.overflow).toBe('hidden');await userEvent.keyboard('{Escape}');expect(screen.queryByRole('dialog')).toBeNull();expect(document.body.style.overflow).toBe('');expect(document.activeElement).toBe(toggle);
 await userEvent.click(toggle);await userEvent.click(screen.getByRole('button',{name:'关闭采购导航遮罩'}));expect(screen.queryByRole('dialog')).toBeNull();await userEvent.click(toggle);await userEvent.click(within(screen.getByRole('navigation',{name:'采购中心导航'})).getByRole('link',{name:'发布采购需求'}));expect(await screen.findByRole('heading',{name:'采购商登录'})).toBeTruthy();expect(screen.queryByRole('dialog')).toBeNull();expect(document.body.style.overflow).toBe('');expect(location.pathname).toBe('/buyer/new');
});
it('refreshes sidebar notifications when a buyer signs in without leaving the procurement shell',async()=>{
 mount('/procurement');const nav=await screen.findByRole('navigation',{name:'采购中心导航'});expect(within(nav).getByRole('link',{name:'消息通知'})).toBeTruthy();
 registered=true;accountRole='visitor';notificationUnread=7;sessionStorage.setItem('zqzt.merchant','buyer-session');sessionStorage.setItem('zqzt.merchant.id','3');fireEvent(window,new Event('zqzt.merchant.changed'));
 expect(await within(nav).findByRole('link',{name:'消息通知 (7)'})).toBeTruthy();sessionStorage.removeItem('zqzt.merchant');fireEvent(window,new Event('zqzt.merchant.changed'));await waitFor(()=>expect(within(nav).getByRole('link',{name:'消息通知'})).toBeTruthy());expect(within(nav).queryByRole('link',{name:'消息通知 (7)'})).toBeNull();
});
it('signs in with a displayed dynamic SMS code and resumes the supplier workspace',async()=>{
 mount('/merchant/opportunities');await userEvent.type(await screen.findByRole('textbox',{name:'手机号'}),'13900009002');await userEvent.click(screen.getByRole('button',{name:'获取验证码'}));expect(await screen.findByLabelText('本机模拟验证码')).toHaveProperty('textContent','420731');await userEvent.type(screen.getByRole('textbox',{name:'短信验证码'}),'420731');await userEvent.click(screen.getByRole('button',{name:'登录'}));
 expect(await screen.findByRole('heading',{name:'收到的询价与采购邀请'})).toBeTruthy();expect(location.pathname).toBe('/merchant/opportunities');expect(document.querySelector('.merchant-workspace')).toBeTruthy();expect(calls.find(c=>c.url.endsWith('/auth/phone/code'))?.data).toEqual({phone:'13900009002',purpose:'login'});expect(calls.some(c=>c.url.endsWith('/auth/sms')||c.url.endsWith('/auth/phone'))).toBe(false);
});
it('redirects legacy supplier links with their query and fragment intact',async()=>{
 mount('/buyer/opportunities/projects/9?view=history#quote',true);await screen.findByRole('heading',{name:'我的报价与历史'});expect(location.pathname+location.search+location.hash).toBe('/merchant/opportunities/projects/9?view=history#quote');expect(document.querySelector('.merchant-workspace')).toBeTruthy();
});
it('offers demo account switching immediately after a buyer SMS login',async()=>{
 registered=true;accountRole='visitor';configureRuntime({...runtimeConfig(),capabilities:{procurement:true,phone_sms_auth:true,development_sms:true,procurement_demo:true},demo_accounts:[{label:'演示采购商',phone:'13900009001'}]});mount('/buyer');
 await userEvent.click(await screen.findByRole('button',{name:/演示采购商13900009001/}));await userEvent.click(screen.getByRole('button',{name:'获取验证码'}));await screen.findByLabelText('本机模拟验证码');await userEvent.type(screen.getByRole('textbox',{name:'短信验证码'}),'420731');await userEvent.click(screen.getByRole('button',{name:'登录'}));const change=await screen.findByRole('button',{name:'切换体验账号'});await userEvent.click(change);expect(await screen.findByRole('heading',{name:'采购商登录'})).toBeTruthy();expect(sessionStorage.getItem('zqzt.merchant')).toBeNull();expect(calls.some(c=>c.url.endsWith('/auth/logout'))).toBe(true);
 configureRuntime({...runtimeConfig(),capabilities:{procurement:true},demo_accounts:[]});
});

it('scopes notification reads and destinations to the current workspace',async()=>{
 registered=true;noticeItems=true;mount('/buyer/notifications',true);expect((await screen.findByRole('link',{name:'查看'})).getAttribute('href')).toBe('/buyer/projects/9');await userEvent.click(screen.getByRole('button',{name:'全部标为已读'}));await waitFor(()=>expect(calls.some(c=>c.url.endsWith('/notifications/read')&&c.data.scope==='buyer'&&Array.isArray(c.data.ids))).toBe(true));cleanup();mount('/merchant/opportunities/notifications',true);expect((await screen.findByRole('link',{name:'查看'})).getAttribute('href')).toBe('/merchant/opportunities/projects/9');await userEvent.click(screen.getByRole('button',{name:'全部标为已读'}));await waitFor(()=>expect(calls.some(c=>c.url.endsWith('/notifications/read')&&c.data.scope==='supplier')).toBe(true));expect(calls.some(c=>c.url.includes('/notifications?scope=supplier'))).toBe(false);
});
it('resets request hall filters and pagination without a supplier menu',async()=>{
 mount('/procurement?q=tractor&hall_id=1&category_id=101&region=13&state=closing_soon&page=2');expect((await screen.findByRole('textbox',{name:'项目关键词'})) as HTMLInputElement).toHaveProperty('value','tractor');await userEvent.click(screen.getByRole('button',{name:'重置'}));await waitFor(()=>expect(location.search).toBe(''));expect(screen.getByRole('textbox',{name:'项目关键词'})).toHaveProperty('value','');expect(screen.getByRole('combobox',{name:'状态'})).toHaveProperty('value','open');expect(within(screen.getByRole('navigation',{name:'采购中心导航'})).getAllByRole('link')).toHaveLength(5);
});


it('protects unsaved procurement input and allows cancelling or confirming navigation',async()=>{
 registered=true;mount('/buyer/new',true);const title=await screen.findByRole('textbox',{name:'项目名称 *'});await screen.findByDisplayValue('采购员');
 await userEvent.type(title,'未保存采购项目');await userEvent.click(within(screen.getByRole('navigation',{name:'采购中心导航'})).getByRole('link',{name:'我的询价与需求'}));
 let dialog=screen.getByRole('dialog',{name:'离开编辑页面？'});expect(location.pathname).toBe('/buyer/new');await userEvent.click(within(dialog).getByRole('button',{name:'继续编辑'}));expect(title).toHaveProperty('value','未保存采购项目');
 const unload=new Event('beforeunload',{cancelable:true});window.dispatchEvent(unload);expect(unload.defaultPrevented).toBe(true);
 await userEvent.click(screen.getByRole('link',{name:'返回'}));dialog=screen.getByRole('dialog',{name:'离开编辑页面？'});await userEvent.click(within(dialog).getByRole('button',{name:'离开页面'}));await screen.findByRole('heading',{name:'我的询价与采购需求'});expect(location.pathname).toBe('/buyer');expect(calls.some(c=>c.method!=='GET')).toBe(false);
});
it('does not mark async buyer and product defaults as unsaved edits',async()=>{
 registered=true;mount('/buyer/new?product=7',true);await screen.findByDisplayValue('测试旋耕机 · 采购询价');await screen.findByDisplayValue('采购员');
 await userEvent.click(screen.getByRole('link',{name:'返回'}));await screen.findByRole('heading',{name:'我的询价与采购需求'});expect(screen.queryByRole('dialog')).toBeNull();
});
it('keeps the leave protection after a failed draft save and releases it after a successful save',async()=>{
 registered=true;failDraft=true;mount('/buyer/new',true);const title=await screen.findByRole('textbox',{name:'项目名称 *'});await screen.findByDisplayValue('采购员');await userEvent.type(title,'需要保留的输入');
 await userEvent.click(screen.getByRole('button',{name:'保存草稿'}));await screen.findByRole('alert');await userEvent.click(screen.getByRole('link',{name:'返回'}));let dialog=screen.getByRole('dialog',{name:'离开编辑页面？'});await userEvent.click(within(dialog).getByRole('button',{name:'继续编辑'}));expect(title).toHaveProperty('value','需要保留的输入');
 failDraft=false;await userEvent.click(screen.getByRole('button',{name:'保存草稿'}));await waitFor(()=>expect(location.pathname).toBe('/buyer/projects/9/edit'));expect(screen.queryByRole('dialog')).toBeNull();
});
it('tracks edits and reversions on an existing demand without a false leave warning',async()=>{
 registered=true;mount('/buyer/projects/9/edit',true);const title=await screen.findByRole('textbox',{name:'项目名称 *'});await screen.findByDisplayValue('项目采购员');
 fireEvent.change(title,{target:{value:'变更需求名称'}});await userEvent.click(screen.getByRole('link',{name:'返回'}));const dialog=screen.getByRole('dialog',{name:'离开编辑页面？'});await userEvent.click(within(dialog).getByRole('button',{name:'继续编辑'}));
 fireEvent.change(title,{target:{value:'采购测试项目'}});await userEvent.click(screen.getByRole('link',{name:'返回'}));await waitFor(()=>expect(location.pathname).toBe('/buyer/projects/9'));expect(screen.queryByRole('dialog')).toBeNull();
});
