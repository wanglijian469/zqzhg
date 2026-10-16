// @vitest-environment jsdom
import React from 'react';
import {afterEach,beforeEach,describe,it,expect,vi} from 'vitest';
import {cleanup,render,screen,within,waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {BrowserRouter} from 'react-router-dom';
import {App} from './main';
import {emptyData} from './types';
const account={id:2,username:'managed:test',login_name:'密码重置验收厂商',vendor_id:4,vendor_name:'密码重置验收厂商',product_count:1,password_login:true,phone_login:false};
const password='Only-Reset-In-Test-123!';
const response=(data:unknown)=>new Response(JSON.stringify(data),{status:200});
let finish:((r:Response)=>void)|undefined;
const fetchMock=vi.fn(async(url:string,opts?:RequestInit)=>{
 if(url.endsWith('/password-reset'))return new Promise<Response>(resolve=>{finish=resolve});
 if(url.includes('/admin/accounts'))return response({items:[account,...(url.includes('account_id=')?[]:[{...account,id:3,login_name:null,password_login:false,phone_login:true}])],total:2,page:1,page_size:20});
 if(url.includes('/dashboard'))return response({counts:{},recent_operations:[],account:{id:1,username:'admin'}});
 if(url.includes('/history'))return response([]);
 if(url.includes('/admin/contents/4'))return response({id:4,kind:'vendor',owner_id:2,revision_id:8,data:{...emptyData,name:account.vendor_name},status:'draft'});
 if(url.includes('/admin/taxonomy'))return response({categories:[],regions:[]});
 return response({});
});
beforeEach(()=>{localStorage.clear();sessionStorage.clear();sessionStorage.setItem('zqzt.admin','test-token');fetchMock.mockClear();finish=undefined;vi.stubGlobal('fetch',fetchMock);vi.stubGlobal('scrollTo',vi.fn())});
afterEach(()=>{cleanup();vi.unstubAllGlobals()});
function mount(path:string){history.replaceState(null,'',path);render(<BrowserRouter><App/></BrowserRouter>)}
describe('Manufacturer password reset',()=>{
 it('cancels without mutation, validates manual passwords, prevents duplicate submission, clears secrets and allows manual copy',async()=>{
  const user=userEvent.setup();mount('/admin/accounts');
  const button=await screen.findByRole('button',{name:'重置密码'});
  expect(screen.getByText('此账号未配置公司登录名，无法重置公司密码。')).toBeTruthy();
  await user.click(button);let dialog=screen.getByRole('dialog');await user.click(within(dialog).getByRole('button',{name:'取消'}));
  expect(fetchMock.mock.calls.some(([url])=>url.endsWith('/password-reset'))).toBe(false);
  await user.click(button);dialog=screen.getByRole('dialog');
  await user.selectOptions(within(dialog).getByRole('combobox'),'manual');
  await user.type(within(dialog).getByLabelText('新密码',{exact:true}),'short');await user.click(within(dialog).getByRole('button',{name:'确认重置'}));
  expect(within(dialog).getByRole('alert').textContent).toContain('12–72');
  await user.clear(within(dialog).getByLabelText('新密码',{exact:true}));await user.type(within(dialog).getByLabelText('新密码',{exact:true}),password);await user.click(within(dialog).getByRole('button',{name:'确认重置'}));
  expect(within(dialog).getByRole('alert').textContent).toContain('不一致');
  await user.type(within(dialog).getByLabelText('确认新密码'),password);
  await user.dblClick(within(dialog).getByRole('button',{name:'确认重置'}));
  expect(fetchMock.mock.calls.filter(([url])=>url.endsWith('/password-reset')).length).toBe(1);
  expect((dialog.querySelector('button[type=submit]') as HTMLButtonElement).disabled).toBe(true);
  finish!(response({account_id:2,login_name:account.login_name,password,revoked_sessions:2}));
  await within(dialog).findByRole('heading',{name:'密码重置成功'});
  Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:vi.fn().mockRejectedValue(new Error('unavailable'))}});
  await user.click(within(dialog).getByRole('button',{name:'复制账号与密码'}));
  expect(within(dialog).getByRole('status').textContent).toContain('手动选择');
  expect(within(dialog).getByLabelText('新密码').getAttribute('readonly')).not.toBeNull();
  expect([...Object.values(localStorage),...Object.values(sessionStorage)].join()).not.toContain(password);
  await user.click(within(dialog).getByRole('button',{name:'完成'}));
  expect(screen.queryByLabelText('新密码')).toBeNull();await user.click(await screen.findByRole('button',{name:'重置密码'}));
  expect(screen.queryByRole('heading',{name:'密码重置成功'})).toBeNull();
 });
 it('uses exact owner lookup on the manufacturer detail and generates a password in English',async()=>{
  const user=userEvent.setup();mount('/admin/contents/4');await screen.findByRole('button',{name:'重置密码'});
  expect(fetchMock.mock.calls.some(([url])=>url.includes('accounts?account_id=2'))).toBe(true);
  await user.click(screen.getByRole('button',{name:'English'}));await user.click(screen.getByRole('button',{name:'Reset password'}));
  const dialog=screen.getByRole('dialog');await user.click(within(dialog).getByRole('button',{name:'Confirm reset'}));
  const request=fetchMock.mock.calls.find(([url])=>url.endsWith('/password-reset'))!;
  expect(JSON.parse(request[1]!.body as string)).toEqual({mode:'generate'});
  finish!(response({account_id:2,login_name:account.login_name,password,revoked_sessions:0}));
  await waitFor(()=>expect(within(dialog).getByRole('heading',{name:'Password reset complete'})).toBeTruthy());
 });
});
