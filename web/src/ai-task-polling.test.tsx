// @vitest-environment jsdom
import {act,cleanup,render,screen,fireEvent} from '@testing-library/react';
import {afterEach,expect,it,vi} from 'vitest';
import {useAITaskPolling} from './ai-task-polling';
import {useAIEditableData} from './ai-editable-data';
import {api} from './api';
import {emptyData} from './types';
import type {Task} from './ai-import-types';
vi.mock('./api',()=>({api:vi.fn(),token:()=> 'fixture-owner'}));
afterEach(()=>{cleanup();vi.useRealTimers();vi.clearAllMocks()});
const task={id:1,status:'processing',result_version:1} as Task;
function Poll({changed}:{changed:()=>void}){const v=useAITaskPolling('/merchant/ai-import/tasks/1',task,changed);return <p>{v.task?.status}</p>}
it('does not overlap or abort slow polling and refreshes drafts only on a result-version change',async()=>{vi.useFakeTimers();let finish:(v:Task)=>void=()=>{};vi.mocked(api).mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve})).mockResolvedValue({...task,status:'review_pending',result_version:2});const changed=vi.fn();render(<Poll changed={changed}/>);await act(async()=>{await vi.advanceTimersByTimeAsync(20000)});expect(api).toHaveBeenCalledTimes(1);expect((vi.mocked(api).mock.calls[0][1]?.signal as AbortSignal).aborted).toBe(false);await act(async()=>{finish(task)});expect(changed).not.toHaveBeenCalled();await act(async()=>{await vi.advanceTimersByTimeAsync(2500)});expect(changed).toHaveBeenCalledTimes(1);expect(screen.getByText('review_pending')).toBeTruthy()});
function Edit({incoming}:{incoming:typeof emptyData}){const r=useAIEditableData(incoming);return <><input aria-label="编辑" value={r.data.description} onChange={e=>r.setData({...r.data,description:e.target.value})}/>{r.conflict&&<><p>发现后台更新</p><button onClick={r.adopt}>采用后台</button><button onClick={r.keep}>保留人工</button></>}</>}
it('hydrates untouched drafts but preserves unsaved edits and requires an explicit choice on conflict',()=>{const r=render(<Edit incoming={{...emptyData,description:'初稿'}}/>);r.rerender(<Edit incoming={{...emptyData,description:'识别补齐'}}/>);expect((screen.getByLabelText('编辑') as HTMLInputElement).value).toBe('识别补齐');fireEvent.change(screen.getByLabelText('编辑'),{target:{value:'人工未保存'}});r.rerender(<Edit incoming={{...emptyData,description:'后台新结果'}}/>);expect(screen.getByText('发现后台更新')).toBeTruthy();expect((screen.getByLabelText('编辑') as HTMLInputElement).value).toBe('人工未保存');fireEvent.click(screen.getByText('保留人工'));expect(screen.queryByText('发现后台更新')).toBeNull();expect((screen.getByLabelText('编辑') as HTMLInputElement).value).toBe('人工未保存')});
