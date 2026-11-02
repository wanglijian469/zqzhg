// @vitest-environment jsdom
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen,within,waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {BrowserRouter} from 'react-router-dom';
import {Language} from './ui';
import {HallProvider} from './halls';
import {withHallFixture} from './hall-test-fixture';
import {DomesticShareShowroom} from './domestic-share';
import {ProductEditor} from './product-editor';
import {configureRuntime,runtimeConfig} from './runtime-config';
import {emptyData,type Item} from './types';
import {productMessages} from './product-messages';
import {resourceMessages} from './resource-messages';
import {messages} from './i18n';
const vendor={id:21,kind:'vendor',hall_id:1,showroom_slug:'sample',data:{...emptyData,name:'测试厂商',main_products:'机械'}} as Item;
let requests:{url:string;method:string;data:any}[]=[];
const json=(data:unknown,status=200)=>new Response(JSON.stringify(data),{status});
const fetcher=async(url:string,init?:RequestInit)=>{url=url.replace(/^\/api\/v1/,'');
 const method=init?.method||'GET',data=typeof init?.body==='string'?JSON.parse(init.body):undefined;requests.push({url,method,data});
 if(url.includes('/taxonomy'))return json({categories:[],regions:[],halls:[]});
 if(url==='/merchant/contents'&&method==='POST')return json({...vendor,id:5,kind:'product',revision_id:10,status:'draft',data:data.data},201);
 if(url.includes('/merchant/contents?'))return json({items:url.includes('kind=vendor')?[vendor]:[],total:1});
 if(url.startsWith('/merchant/resources')&&method==='POST')return json({id:99,kind:'video',status:'draft',revision_id:90,data:data.data},201);
 return json({items:[],total:0});
};
beforeEach(()=>{requests=[];localStorage.clear();sessionStorage.clear();sessionStorage.setItem('zqzt.merchant','test');sessionStorage.setItem('zqzt.merchant.id','9');history.replaceState(null,'','/merchant/products/new');configureRuntime({...runtimeConfig(),version:1,capabilities:{douyin_video:true},site:{...runtimeConfig().site,api_url:'',media_url:'',web_url:''}});vi.stubGlobal('fetch',withHallFixture(fetcher));vi.stubGlobal('scrollTo',vi.fn());Element.prototype.scrollIntoView=vi.fn();});
afterEach(()=>{cleanup();vi.unstubAllGlobals()});
function shell(children:React.ReactNode){const words={...messages.zh,...productMessages.zh,...resourceMessages.zh};return render(<BrowserRouter><HallProvider><Language.Provider value={{lang:'zh',t:k=>(words as Record<string,string>)[k]||k,toggle:()=>{}}}>{children}</Language.Provider></HallProvider></BrowserRouter>)}
it('offers a visible video section, saves a draft explicitly and starts with neither source selected',async()=>{
 shell(<ProductEditor onSubmitted={()=>{}}/>);await userEvent.type(screen.getByRole('textbox',{name:'产品名称'}),'测试产品');await userEvent.click(screen.getByRole('button',{name:'下一步'}));
 expect(screen.getByRole('heading',{name:'产品视频（选填）'})).toBeTruthy();expect(requests.some(r=>r.method==='POST')).toBe(false);
 await userEvent.click(screen.getByRole('button',{name:'先保存产品草稿'}));await userEvent.click(await screen.findByRole('button',{name:'添加产品视频'}));
 expect(document.querySelectorAll('.douyin-add-choice [aria-pressed=true]')).toHaveLength(0);expect(requests.some(r=>r.url.endsWith('/submit'))).toBe(false);
});
it('pins recovered local video data to the current product when saving',async()=>{
 localStorage.setItem('zqzt.resource-draft.9.video.5',JSON.stringify({revision_id:0,data:{title:'测试视频',title_en:'',description:'',description_en:'',product_id:999,purpose:'company',format:'video',file_media_id:77,cover_media_id:0,page_media_ids:[]}}));
 shell(<ProductEditor onSubmitted={()=>{}}/>);await userEvent.type(screen.getByRole('textbox',{name:'产品名称'}),'测试产品');await userEvent.click(screen.getByRole('button',{name:'下一步'}));await userEvent.click(screen.getByRole('button',{name:'先保存产品草稿'}));await userEvent.click(await screen.findByRole('button',{name:'添加产品视频'}));await userEvent.click(screen.getByRole('button',{name:'上传本地视频'}));
 const form=document.querySelector('.resource-management form')!;await userEvent.click(within(form as HTMLElement).getByRole('button',{name:'保存云端草稿'}));await waitFor(()=>expect(requests.some(r=>r.url==='/merchant/resources')).toBe(true));const saved=requests.find(r=>r.url==='/merchant/resources')!;expect(saved.data.data.product_id).toBe(5);expect(saved.data.data.purpose).toBe('product');
});
it.each(['zh','en'] as const)('offers six domestic channels without invoking native sharing in %s',async lang=>{
 const native=vi.fn();Object.defineProperty(navigator,'share',{configurable:true,value:native});const copy=vi.fn().mockRejectedValue(new Error('denied'));Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:copy}});
 render(<Language.Provider value={{lang,t:k=>k,toggle:()=>{}}}><DomesticShareShowroom item={vendor} canonical="https://example.com/v/sample?q=x#tab" close={()=>{}}/></Language.Provider>);
 const group=screen.getByRole('group',{name:lang==='zh'?'国内分享渠道':'Sharing channels'});expect(within(group).getAllByRole('button')).toHaveLength(4);expect(within(group).getAllByRole('link')).toHaveLength(2);expect(group.textContent).not.toMatch(/whatsapp|telegram|email/i);
 expect(within(group).getByRole('link',{name:'qq'}).getAttribute('href')).toContain('https://connect.qq.com/widget/shareqq/index.html?');
 await userEvent.click(screen.getByRole('button',{name:lang==='zh'?'复制展厅地址':'Copy showroom link'}));expect(copy).toHaveBeenCalledWith('https://example.com/v/sample');expect(native).not.toHaveBeenCalled();expect(screen.getByRole('textbox',{name:lang==='zh'?'手动复制':'Copy manually'})).toHaveProperty('value','https://example.com/v/sample');
});
