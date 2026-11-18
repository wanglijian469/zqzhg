// @vitest-environment jsdom
import {afterEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen,waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {api} from './api';
import {AIModelSettings} from './ai-model-settings';
vi.mock('./api',async original=>({...await original<typeof import('./api')>(),api:vi.fn()}));
const initial={version:0,base_url:'https://dashscope.aliyuncs.com/compatible-mode/v1',timeout_seconds:360,allow_manual_max:true,default_model:'qwen3.8-flash',review_model:'qwen3.8-flash',advanced_model:'qwen3.8-max',key_configured:true,key_source:'environment',configured:true,configuration_error:'',enabled:true};
afterEach(()=>{cleanup();vi.clearAllMocks();localStorage.clear();sessionStorage.clear()});
it('submits a replacement once, clears its input, and never stores or receives the saved key',async()=>{
 vi.mocked(api).mockResolvedValueOnce(initial).mockResolvedValueOnce({...initial,version:1,key_source:'admin'});
 render(<AIModelSettings/>);await screen.findByLabelText(/提取模型/);
 await userEvent.selectOptions(screen.getByLabelText('密钥操作'),'replace');const secret='synthetic-user-entered-key';await userEvent.type(screen.getByLabelText(/按量计费API Key/),secret);await userEvent.click(screen.getByRole('button',{name:'保存模型配置'}));await screen.findByText(/配置已保存/);
 const request=vi.mocked(api).mock.calls.find(([,options])=>options?.method==='PUT');expect(JSON.parse(String(request?.[1]?.body))).toMatchObject({key_action:'replace',api_key:secret});expect(screen.queryByDisplayValue(secret)).toBeNull();expect(JSON.stringify(localStorage)+JSON.stringify(sessionStorage)).not.toContain(secret);
});
it('tests flash by default and requires explicit confirmation before a max test',async()=>{
 vi.mocked(api).mockResolvedValueOnce(initial).mockResolvedValueOnce({model:'qwen3.8-flash',usage:{total_tokens:4}}).mockResolvedValueOnce({model:'qwen3.8-max',usage:{total_tokens:5}});
 render(<AIModelSettings/>);await screen.findByLabelText('测试模型');await userEvent.click(screen.getByRole('button',{name:'测试已保存的连接'}));await screen.findByText(/连通成功：qwen3.8-flash/);
 await userEvent.selectOptions(screen.getByLabelText('测试模型'),'max');expect((screen.getByRole('button',{name:'测试已保存的连接'}) as HTMLButtonElement).disabled).toBe(true);await userEvent.click(screen.getByRole('checkbox',{name:/我明确选择max/}));await userEvent.click(screen.getByRole('button',{name:'测试已保存的连接'}));await screen.findByText(/连通成功：qwen3.8-max/);
 await waitFor(()=>expect(vi.mocked(api).mock.calls.filter(([,o])=>o?.method==='POST').map(([,o])=>JSON.parse(String(o?.body)))).toEqual([{model_mode:'flash',confirm_max:false},{model_mode:'max',confirm_max:true}]));
});
