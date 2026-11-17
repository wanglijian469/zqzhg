// @vitest-environment jsdom
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen,waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {BrowserRouter} from 'react-router-dom';
import {api} from './api';
import {configureRuntime} from './runtime-config';
import AIImport from './ai-import';
vi.mock('./api',async original=>({...await original<typeof import('./api')>(),api:vi.fn()}));
let submitted:FormData|undefined;
class UploadXHR{
 upload={};status=202;responseText='{"task":{"id":7},"duplicate":false}';onload=()=>{};onerror=()=>{};ontimeout=()=>{};timeout=0;
 open(){}setRequestHeader(){}send(body:FormData){submitted=body;queueMicrotask(()=>this.onload())}
}
beforeEach(()=>{submitted=undefined;configureRuntime({version:1,site:{name:'',name_en:'',web_url:'',api_url:'',media_url:''},uploads:{image_mib:10,video_mib:200,pdf_mib:50,default_quota_bytes:200000000}});history.replaceState(null,'','/merchant/ai-import');vi.mocked(api).mockImplementation(async path=>path.endsWith('/config')?{configured:true,pdf_tools_ready:true,max_file_bytes:30000000,max_pages:100,image_mib:10,allow_manual_max:true}:{items:[]});vi.stubGlobal('XMLHttpRequest',UploadXHR)});
afterEach(()=>{cleanup();vi.clearAllMocks();vi.unstubAllGlobals()});
const file=()=>new File([new Uint8Array([1,2,3])],'synthetic-ui.png',{type:'image/png'});
it('uploads with flash unless the user makes an explicit advanced choice',async()=>{
 render(<BrowserRouter><AIImport/></BrowserRouter>);const select=await screen.findByLabelText('本次任务模型');expect((select as HTMLSelectElement).value).toBe('flash');await userEvent.upload(screen.getByLabelText(/拖拽文件到这里/),file());await userEvent.click(screen.getByRole('button',{name:'上传并创建识别任务'}));await waitFor(()=>expect(submitted).toBeTruthy());expect(submitted?.get('model_mode')).toBe('flash');expect(submitted?.has('confirm_max')).toBe(false);
});
it('blocks a max upload until the higher-fee confirmation is checked',async()=>{
 render(<BrowserRouter><AIImport/></BrowserRouter>);await screen.findByLabelText('本次任务模型');await userEvent.selectOptions(screen.getByLabelText('本次任务模型'),'max');await userEvent.upload(screen.getByLabelText(/拖拽文件到这里/),file());expect((screen.getByRole('button',{name:'上传并创建识别任务'}) as HTMLButtonElement).disabled).toBe(true);expect(submitted).toBeUndefined();await userEvent.click(screen.getByRole('checkbox',{name:/我为本次复杂任务明确选择/}));await userEvent.click(screen.getByRole('button',{name:'上传并创建识别任务'}));await waitFor(()=>expect(submitted?.get('model_mode')).toBe('max'));expect(submitted?.get('confirm_max')).toBe('true');
});
