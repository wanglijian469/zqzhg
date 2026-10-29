// @vitest-environment jsdom
import {afterEach,it,expect,vi} from 'vitest';
import {cleanup,render,screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {lazyRoute} from './lazy-route';
import {Component,type ReactNode} from 'react';

afterEach(()=>{cleanup();vi.restoreAllMocks()});
it('retries a rejected route import using a fresh lazy promise',async()=>{
 vi.spyOn(console,'error').mockImplementation(()=>{});
 const load=vi.fn().mockRejectedValueOnce(new Error('chunk network unavailable')).mockResolvedValue({default:()=> <h1>Recovered route</h1>});
 const Page=lazyRoute(load);render(<Page/>);
 await screen.findByRole('alert');await userEvent.click(screen.getByRole('button',{name:'重试'}));
 expect(await screen.findByRole('heading',{name:'Recovered route'})).toBeTruthy();expect(load).toHaveBeenCalledTimes(2);
});

it('does not disguise a page render defect as a resource loading failure',async()=>{
 vi.spyOn(console,'error').mockImplementation(()=>{});
 class RuntimeBoundary extends Component<{children:ReactNode},{error?:Error}>{
  state:{error?:Error}={};static getDerivedStateFromError(error:Error){return {error}}
  render(){return this.state.error?<h1>{this.state.error.message}</h1>:this.props.children}
 }
 const Page=lazyRoute(async()=>({default:()=>{throw new Error('Actual page render defect')}}));
 render(<RuntimeBoundary><Page/></RuntimeBoundary>);
 expect(await screen.findByRole('heading',{name:'Actual page render defect'})).toBeTruthy();
 expect(screen.queryByText('页面资源加载失败，请检查网络后重试。')).toBeNull();
});
