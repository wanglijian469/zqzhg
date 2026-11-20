// @vitest-environment jsdom
import {afterEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen} from '@testing-library/react';
import {AIErrorState} from './ai-error-state';
vi.mock('./ui',()=>({ErrorState:()=> <p>服务暂时异常，请稍后重试</p>}));
afterEach(cleanup);
it('does not send a Chinese AI error message back through translation as a missing key',()=>{render(<AIErrorState code="ai_invalid_schema" retry={()=>{}}/>);expect(screen.getByText(/模型结果格式不符合要求/)).toBeTruthy();expect(screen.getByRole('button',{name:'重新加载'})).toBeTruthy();expect(screen.queryByText('服务暂时异常，请稍后重试')).toBeNull()});
