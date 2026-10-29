// @vitest-environment jsdom
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {act,cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {OptimizedMediaImage} from './media-image';

let observers:{callback:IntersectionObserverCallback;node?:Element;disconnect:()=>void}[]=[];
beforeEach(()=>{
 observers=[];sessionStorage.clear();
 vi.stubGlobal('IntersectionObserver',class{entry:{callback:IntersectionObserverCallback;node?:Element;disconnect:()=>void};constructor(callback:IntersectionObserverCallback){this.entry={callback,disconnect:vi.fn()};observers.push(this.entry)}observe(node:Element){this.entry.node=node}disconnect(){this.entry.disconnect()}});
 vi.stubGlobal('URL',Object.assign(URL,{createObjectURL:vi.fn(()=> 'blob:private-image'),revokeObjectURL:vi.fn()}));
});
afterEach(()=>{cleanup();vi.unstubAllGlobals();sessionStorage.clear()});
const reveal=()=>act(()=>{for(const observer of observers)observer.callback([{isIntersecting:true,target:observer.node} as IntersectionObserverEntry],{} as IntersectionObserver)});
it('defers images until near the viewport and uses responsive candidates instead of full-size files',()=>{
 const view=render(<OptimizedMediaImage id={7} alt="设备" unavailableLabel="无法加载"/>);
 expect(view.container.querySelector('img')).toBeNull();reveal();
 const image=screen.getByRole('img',{name:'设备'});expect(image.getAttribute('srcset')).toContain('/7?w=480 480w');expect(image.getAttribute('srcset')).toContain('/7?w=960 960w');expect(image.getAttribute('sizes')).toBeTruthy();expect(image.getAttribute('decoding')).toBe('async');
 expect(image.getAttribute('src')).not.toContain('thumb=');
});
it('prioritizes the hero immediately and preserves HD requests for a clicked viewer',()=>{
 const view=render(<OptimizedMediaImage id={9} preset="hero" priority alt="封面" unavailableLabel="失败"/>);
 expect(screen.getByRole('img').getAttribute('fetchpriority')).toBe('high');expect(screen.getByRole('img').getAttribute('srcset')).toContain('1440');
 view.rerender(<OptimizedMediaImage id={9} thumb={false} priority alt="高清" unavailableLabel="失败"/>);
 expect(screen.getByRole('img').getAttribute('src')).toBe('/api/v1/media/9');expect(screen.getByRole('img').getAttribute('srcset')).toBeNull();
});
it('does not fetch private media offscreen and discards the previous account blob on session change',async()=>{
 const fetch=vi.fn().mockResolvedValue(new Response(new Blob(['image'])));vi.stubGlobal('fetch',fetch);sessionStorage.setItem('zqzt.merchant','first-account');
 const view=render(<OptimizedMediaImage id={8} role="merchant" alt="私有图片" unavailableLabel="失败"/>);
 expect(fetch).not.toHaveBeenCalled();reveal();await waitFor(()=>expect(screen.getByRole('img').getAttribute('src')).toBe('blob:private-image'));
 expect(fetch.mock.calls[0][1].headers.Authorization).toBe('Bearer first-account');expect(fetch.mock.calls[0][1].cache).toBe('no-store');
 sessionStorage.setItem('zqzt.merchant','second-account');view.rerender(<OptimizedMediaImage id={8} role="merchant" alt="私有图片" unavailableLabel="失败"/>);
 expect(view.container.querySelector('img')).toBeNull();expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:private-image');reveal();await waitFor(()=>expect(fetch).toHaveBeenCalledTimes(2));expect(fetch.mock.calls[1][1].headers.Authorization).toBe('Bearer second-account');
});
it('shows an image fallback and resets the failed state for the next image',()=>{
 const view=render(<OptimizedMediaImage id={1} priority alt="封面" fallback={<span>暂无封面</span>} unavailableLabel="失败"/>);fireEvent.error(screen.getByRole('img'));expect(screen.getByText('暂无封面')).toBeTruthy();
 view.rerender(<OptimizedMediaImage id={2} priority alt="封面" unavailableLabel="失败"/>);expect(screen.getByRole('img').getAttribute('src')).toContain('/2?w=');
});
