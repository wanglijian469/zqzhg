// @vitest-environment jsdom
import {afterEach,expect,it,vi} from 'vitest';
import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {useAIImportResult} from './ai-import-result';
import {api} from './api';
vi.mock('./api',()=>({api:vi.fn(),token:()=>sessionStorage.getItem('test-session')||'owner'}));
afterEach(()=>{cleanup();sessionStorage.clear();vi.clearAllMocks()});
function View({path='/task/3'}:{path?:string}){const r=useAIImportResult<{name:string}>(path);return <><button onClick={r.reload}>刷新进度</button>{r.data?<input aria-label="草稿编辑" defaultValue={r.data.name}/>:<p>加载</p>}{r.error&&<p>{r.error}</p>}</>}
it('retains the editor node and unsaved input throughout a background progress refresh and failure',async()=>{
 let finish:(value:{name:string})=>void=()=>{};
 vi.mocked(api).mockResolvedValueOnce({name:'已识别草稿'}).mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve})).mockRejectedValueOnce({code:'network_error'});
 render(<View/>);const input=await screen.findByLabelText('草稿编辑');fireEvent.change(input,{target:{value:'尚未保存的人工修改'}});
 fireEvent.click(screen.getByText('刷新进度'));await waitFor(()=>expect(api).toHaveBeenCalledTimes(2));expect(screen.getByLabelText('草稿编辑')).toBe(input);expect(screen.queryByText('加载')).toBeNull();
 finish({name:'已识别草稿'});await waitFor(()=>expect((input as HTMLInputElement).value).toBe('尚未保存的人工修改'));
 fireEvent.click(screen.getByText('刷新进度'));await screen.findByText('network_error');expect(screen.getByLabelText('草稿编辑')).toBe(input);
});
it('does not display the previous task data after changing tasks',async()=>{
 vi.mocked(api).mockResolvedValueOnce({name:'任务3'}).mockImplementationOnce(()=>new Promise(()=>{}));
 const r=render(<View/>);await screen.findByLabelText('草稿编辑');r.rerender(<View path="/task/4"/>);expect(screen.queryByLabelText('草稿编辑')).toBeNull();
});
