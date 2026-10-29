// @vitest-environment jsdom
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen,within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {BrowserRouter} from 'react-router-dom';
import {App} from './main';
import {authReturnPath} from './auth-routes';
import {withHallFixture} from './hall-test-fixture';
const json=(v:unknown)=>new Response(JSON.stringify(v));
beforeEach(()=>{localStorage.clear();sessionStorage.clear();history.replaceState(null,'','/join');vi.stubGlobal('scrollTo',vi.fn());vi.stubGlobal('fetch',withHallFixture(vi.fn(async()=>json({items:[],total:0}))))});
afterEach(()=>{cleanup();vi.unstubAllGlobals()});
const mount=()=>render(<BrowserRouter><App/></BrowserRouter>);
it('uses the original split layout with mobile registration as default',async()=>{mount();await screen.findByLabelText('手机号');expect(document.querySelector('.merchant-entry-intro')).toBeTruthy();expect(document.querySelector('.merchant-entry-card')).toBeTruthy();expect(screen.getByRole('tab',{name:'手机验证码注册'}).getAttribute('aria-selected')).toBe('true');expect(screen.queryByText(/UTF-8|emoji/)).toBeNull();expect(document.querySelector('.merchant-entry-footer')).toBeTruthy()});
it('old register and login URLs redirect to the shared pages with the hall query retained',async()=>{history.replaceState(null,'','/register?hall=agriculture');mount();await screen.findByLabelText('手机号');expect(location.pathname).toBe('/join');expect(location.search).toContain('hall=agriculture');await userEvent.click(within(document.querySelector('.merchant-entry-card')!).getByRole('link',{name:'登录'}));await screen.findByLabelText('手机号或用户名');expect(location.pathname).toBe('/merchant/login');expect(screen.queryByRole('button',{name:'短信验证码'})).toBeNull();await userEvent.click(screen.getByRole('button',{name:'忘记密码？'}));await screen.findByLabelText('手机号');expect(location.pathname).toBe('/merchant/login')});
it('keeps English registration and password login on the same layout',async()=>{mount();await screen.findByLabelText('手机号');await userEvent.click(screen.getByRole('button',{name:'English'}));expect(screen.getByRole('tab',{name:'Register with mobile'})).toBeTruthy();expect(screen.getByLabelText('Mobile number')).toBeTruthy();expect(document.querySelector('.merchant-entry-card')).toBeTruthy()});
it('return addresses cannot be external or authentication loops',()=>{for(const path of ['https://bad.example/','//bad.example/','/merchant/login','/register'])expect(authReturnPath('?next='+encodeURIComponent(path))).toBe('/');expect(authReturnPath('?next='+encodeURIComponent('/merchant/products?x=1'))).toBe('/merchant/products?x=1')});

it('keeps sign-in accessible when a cached session has expired',async()=>{sessionStorage.setItem('zqzt.merchant','expired-token');history.replaceState(null,'','/merchant/login');mount();await screen.findByLabelText('手机号或用户名');expect(location.pathname).toBe('/merchant/login')});
