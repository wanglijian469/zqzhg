// @vitest-environment jsdom
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {cleanup,fireEvent,render,screen,waitFor,within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {BrowserRouter} from 'react-router-dom';
import {AdmissionGuide} from './admission-guide';
import {ICPRegistration} from './site-common';
import {Language} from './ui';
import {configureRuntime,runtimeConfig} from './runtime-config';
import {indexablePublicPath,publicPageSEO} from './seo';

beforeEach(()=>{history.replaceState(null,'','/guide');configureRuntime({...runtimeConfig(),version:1,capabilities:{phone_sms_register:false,procurement:false},site:{...runtimeConfig().site,icp_number:'冀ICP备2026047360号-2'}});vi.stubGlobal('requestAnimationFrame',(f:FrameRequestCallback)=>{f(0);return 1});vi.stubGlobal('cancelAnimationFrame',vi.fn());Element.prototype.scrollIntoView=vi.fn()});
afterEach(()=>{cleanup();vi.unstubAllGlobals()});
function mount(footer=false){return render(<Language.Provider value={{lang:'zh',t:k=>k==='close'?'关闭':k,toggle:()=>{}}}><BrowserRouter><AdmissionGuide/>{footer&&<footer><ICPRegistration/></footer>}</BrowserRouter></Language.Provider>)}
it('has ten chapters, working directory targets and real entry links without purchase links',()=>{
 mount(true);expect(screen.getByRole('heading',{name:'厂商入驻与使用指南'})).toBeTruthy();
 const directory=screen.getByRole('navigation',{name:'指南目录'});expect(within(directory).getAllByRole('link').filter(a=>a.getAttribute('href')?.startsWith('#'))).toHaveLength(10);
 for(const link of within(directory).getAllByRole('link')){const href=link.getAttribute('href')!;if(href.startsWith('#'))expect(document.getElementById(href.slice(1))).toBeTruthy()}
 for(const link of screen.getAllByRole('link',{name:'立即免费入驻'}))expect(link.getAttribute('href')).toBe('/join');
 for(const link of screen.getAllByRole('link',{name:'登录'}))expect(link.getAttribute('href')).toBe('/merchant/login');
 expect(within(screen.getByRole('contentinfo')).getByRole('link',{name:'入驻使用指南'}).getAttribute('href')).toBe('/guide');
 expect(screen.getByText(/线上询价和采购中心暂未开放/)).toBeTruthy();expect(screen.queryByRole('link',{name:/支付|充值|立即询价/})).toBeNull();
});
it('shows the mobile tutorial only when SMS registration is actually available',()=>{
 const view=mount();expect(screen.queryByRole('heading',{name:'也可以用手机验证码注册'})).toBeNull();
 configureRuntime({...runtimeConfig(),capabilities:{phone_sms_register:true,procurement:false}});view.rerender(<BrowserRouter><AdmissionGuide/></BrowserRouter>);
 expect(screen.getByRole('heading',{name:'也可以用手机验证码注册'})).toBeTruthy();
});
it('keeps all FAQ answers collapsed until the reader chooses a question',async()=>{
 mount();const section=document.getElementById('faq')!;const details=Array.from(section.querySelectorAll('details'));expect(details).toHaveLength(10);expect(details.every(d=>!d.open)).toBe(true);
 await userEvent.click(within(section).getByText('已有账号，还要重新注册吗？'));expect(details[0].open).toBe(true);expect(details.slice(1).every(d=>!d.open)).toBe(true);
});
it('opens an actual screenshot in a labelled modal and restores focus after Escape',async()=>{
 mount();const button=screen.getByRole('button',{name:'放大查看：选择账号密码注册，填写后注册'});await userEvent.click(button);
 const modal=screen.getByRole('dialog',{name:'选择账号密码注册，填写后注册'});expect(within(modal).getByRole('img').getAttribute('src')).toBe('/guide-images/register-account.png');
 await userEvent.keyboard('{Escape}');expect(screen.queryByRole('dialog')).toBeNull();expect(document.activeElement).toBe(button);
});
it('retains the text instructions and offers a retry if an illustration fails',async()=>{
 mount();const img=screen.getByRole('img',{name:'选择账号密码注册，填写后注册，示例资料'});fireEvent.error(img);
 expect(screen.getByText('这张操作图暂时无法显示，照着上面的文字也能完成。')).toBeTruthy();expect(screen.getByText(/填写“用户名”“设置密码”“确认密码”/)).toBeTruthy();
 await userEvent.click(screen.getByRole('button',{name:'重新加载图片'}));expect(screen.getByRole('img',{name:'选择账号密码注册，填写后注册，示例资料'}).getAttribute('src')).toContain('?retry=1');
});
it('supports direct section navigation and independent indexable metadata',async()=>{
 history.replaceState(null,'','/guide?hall=cable#review');mount();await waitFor(()=>expect(document.activeElement).toBe(document.getElementById('review')));
 expect(indexablePublicPath('/guide')).toBe(true);expect(indexablePublicPath('/guide/unknown')).toBe(false);expect(publicPageSEO('/guide','中企展馆',undefined,'zh')?.title).toBe('厂商入驻与使用指南 | 中企展馆');
});
