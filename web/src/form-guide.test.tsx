// @vitest-environment jsdom
import {expect,it,vi,afterEach} from 'vitest';
import {render,screen,cleanup} from '@testing-library/react';
import {BrowserRouter} from 'react-router-dom';
import userEvent from '@testing-library/user-event';
import {App} from './main';
import {emptyData} from './types';
import {validateForm} from './form-guide';
afterEach(()=>{cleanup();vi.unstubAllGlobals();localStorage.clear();sessionStorage.clear()});
it('matches unicode and UTF8 byte limits and identifies nested contacts',()=>{const d={...emptyData,name:'😀'.repeat(200),main_products:'机械',region_name:'河北',phone:'0319-1234567',address:'中'.repeat(166)};expect(validateForm(d,'vendor')).toEqual([]);d.address+='中';expect(validateForm(d,'vendor')).toContainEqual({field:'address',code:'too_long',limit:500,unit:'bytes'});expect(validateForm({...d,address:'',company_profile:{website:'www.example.com',contacts:[{name:'张经理'}]}},'vendor').map(i=>i.field)).toEqual(expect.arrayContaining(['company_profile.website','company_profile.contacts.0.phone']))});
it('shows guest guidelines and inline required errors without losing a local entry',async()=>{vi.stubGlobal('fetch',vi.fn(async()=>new Response(JSON.stringify({categories:[],regions:[],halls:[]}))));vi.stubGlobal('scrollTo',vi.fn());vi.stubGlobal('TextEncoder',globalThis.TextEncoder);Element.prototype.scrollIntoView=vi.fn();history.replaceState(null,'','/join');render(<BrowserRouter><App/></BrowserRouter>);await userEvent.type(await screen.findByRole('textbox',{name:'企业 / 工厂名称'}),'保留的企业名称');await userEvent.click(screen.getByRole('button',{name:'注册／登录并保存草稿'}));expect(screen.getByRole('alert').textContent).toContain('请修改以下项目');expect((screen.getByRole('textbox',{name:'企业 / 工厂名称'}) as HTMLInputElement).value).toBe('保留的企业名称');expect(screen.queryByRole('dialog')).toBeNull();expect(screen.getByText('登录并保存')).toBeTruthy();expect(screen.getByPlaceholderText('13900000000 / 0319-1234567')).toBeTruthy()});

