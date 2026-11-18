// @vitest-environment jsdom
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen,waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {BrowserRouter} from 'react-router-dom';
import {api} from './api';
import {configureRuntime} from './runtime-config';
import AIImport from './ai-import';
vi.mock('./api',async original=>({...await original<typeof import('./api')>(),api:vi.fn(),currentAgreementRefs:vi.fn(async()=>['terms','privacy','showroom-agreement'].map(key=>({key,version:2,digest:key+'-digest'})))}));
let submitted:FormData|undefined;
class UploadXHR{
 upload={};status=202;responseText='{"task":{"id":7},"duplicate":false}';onload=()=>{};onerror=()=>{};ontimeout=()=>{};timeout=0;
 open(){}setRequestHeader(){}send(body:FormData){submitted=body;queueMicrotask(()=>this.onload())}
}
beforeEach(()=>{submitted=undefined;configureRuntime({version:1,site:{name:'',name_en:'',web_url:'',api_url:'',media_url:''},uploads:{image_mib:10,video_mib:200,pdf_mib:50,default_quota_bytes:200000000}});history.replaceState(null,'','/merchant/ai-import');vi.mocked(api).mockImplementation(async path=>path.endsWith('/config')?{configured:true,pdf_tools_ready:true,max_file_bytes:30000000,max_pages:100,image_mib:10,allow_manual_max:true}:{items:path.endsWith('/service-documents')?['terms','privacy','showroom-agreement'].map((key,i)=>({id:i+1,key,version:2,title:key,digest:key+'-digest'})):[]});vi.stubGlobal('XMLHttpRequest',UploadXHR)});
afterEach(()=>{cleanup();vi.clearAllMocks();vi.unstubAllGlobals()});
const file=()=>new File([new Uint8Array([1,2,3])],'synthetic-ui.png',{type:'image/png'});
it('has no merchant model choice and starts recognition with the displayed agreement versions',async()=>{
 render(<BrowserRouter><AIImport/></BrowserRouter>);await screen.findByRole('heading',{name:'上传画册'});expect(screen.queryByLabelText('本次任务模型')).toBeNull();expect(screen.queryByRole('button',{name:'确认协议'})).toBeNull();
 await userEvent.upload(screen.getByLabelText(/拖拽文件到这里/),file());await userEvent.click(screen.getByRole('button',{name:'上传并创建识别任务'}));await waitFor(()=>expect(submitted).toBeTruthy());expect(submitted?.has('model_mode')).toBe(false);expect(submitted?.has('confirm_max')).toBe(false);expect(JSON.parse(String(submitted?.get('agreements')))).toHaveLength(3);
});
it('never offers an advanced model even when a legacy config advertises it',async()=>{
 render(<BrowserRouter><AIImport/></BrowserRouter>);await screen.findByText(/PDF最多/);expect(screen.queryByRole('combobox',{name:/模型/})).toBeNull();expect(screen.queryByText(/复杂任务明确选择/)).toBeNull();expect(screen.getByText(/使用本服务并执行上述操作/)).toBeTruthy();expect(submitted).toBeUndefined();
});
