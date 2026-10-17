// @vitest-environment jsdom
import React from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { BrowserRouter } from 'react-router-dom';
import { App } from './main';
import { emptyData } from './types';
const vendor = { id: 4, kind: 'vendor', owner_id: 2, data: { ...emptyData, name: '仅界面测试厂商', main_products: '测试机械', region_name: '测试地区', phone: '13900004001' }, status: 'draft', revision_id: 8, visible: false, public_visible: false, featured: false, sort_order: 0, product_count: 1 };
const taxonomy = { halls: [], categories: [{ id: 1, parent_id: 0, name: '机械', name_en: 'Machinery', enabled: true }], regions: [{ id: 1, name: '测试地区', name_en: 'Test region', enabled: true }] };
const dashboard = { counts: { vendor_total: 1, product_total: 1, vendor_public: 0, product_public: 0, vendor_pending: 0, product_pending: 0 }, recent_operations: [], account: { id: 1, username: 'admin' } };
let status = 'draft';
let expired = false;
const response = (data: unknown, code = 200) => new Response(JSON.stringify(data), { status: code });
const fetchMock = vi.fn(async (url: string, opts?: RequestInit) => {
    if (url.includes('/auth/admin')) {
        expired = false;
        return response({ token: 'test-only-token' });
    }
    if (expired)
        return response({ error: { code: 'unauthorized' } }, 401);
    if (url.includes('/dashboard'))
        return response(dashboard);
    if (url.includes('/taxonomy'))
        return response(taxonomy);
    if (url.includes('/history'))
        return response([]);
    if (url.includes('/admin/vendors') && opts?.method === 'POST')
        return response({ item: vendor, login_name: vendor.data.name, initial_password: 'Only-In-UI-Test!' }, 201);
    if (url.includes('/admin/contents/4/draft')) {
        status = 'draft';
        return response({ ...vendor, revision_id: 9 });
    }
    if (url.includes('/admin/contents/4'))
        return response({ ...vendor, status });
    if (url.includes('/admin/contents?'))
        return response({ items: [vendor], total: 1, page: 1, page_size: 20 });
    if (url.includes('/admin/accounts'))
        return response({ items: [], total: 0, page: 1, page_size: 20 });
    return response({ ok: true });
});
beforeEach(() => { localStorage.clear(); sessionStorage.clear(); sessionStorage.setItem('zqzt.admin', 'test-only-token'); fetchMock.mockClear(); status = 'draft'; expired = false; vi.stubGlobal('fetch', fetchMock); vi.stubGlobal('scrollTo', vi.fn()); });
afterEach(() => { cleanup(); vi.unstubAllGlobals(); });
const mount = (path: string) => { history.replaceState(null, '', path); render(<BrowserRouter><App /></BrowserRouter>); };
describe('Administrator console', () => {
    it('uses an independent layout and remembers desktop navigation collapse', async () => {
        mount('/admin');
        const user = userEvent.setup();
        await screen.findByRole('heading', { name: '工作台' });
        expect(location.pathname).toBe('/admin/dashboard');
        expect(document.querySelector('.header')).toBeNull();
        expect(document.querySelector('.footer')).toBeNull();
        await user.click(screen.getByRole('button', { name: '收起导航' }));
        expect(localStorage.getItem('zqzt.admin.collapsed')).toBe('1');
        await user.click(screen.getByRole('link', { name: '厂商管理' }));
        expect(document.querySelector('.admin-app')?.classList.contains('collapsed')).toBe(true);
    });
    it('retains deep routes and URL filters when switching languages', async () => {
        mount('/admin/products?q=440&vendor_id=4&category_id=1&page=2');
        const user = userEvent.setup();
        await screen.findByText(vendor.data.name);
        await user.click(screen.getByRole('button', { name: 'English' }));
        expect(location.pathname).toBe('/admin/products');
        expect(new URLSearchParams(location.search).get('page')).toBe('2');
        expect((screen.getByRole('textbox', { name: 'Keyword' }) as HTMLInputElement).value).toBe('440');
        await user.selectOptions(screen.getByRole('combobox', { name: 'Review status' }), 'pending');
        expect(new URLSearchParams(location.search).get('page')).toBeNull();
        expect(new URLSearchParams(location.search).get('vendor_id')).toBe('4');
    });
    it('returns to the requested page after session expiry and sign-in', async () => {
        expired = true;
        mount('/admin/products?q=440&page=2');
        const user = userEvent.setup();
        await screen.findByRole('heading', { name: '管理员登录' });
        expect(sessionStorage.getItem('zqzt.admin')).toBeNull();
        await user.type(screen.getByLabelText('密码'), 'UI-Test-Password!');
        await user.click(screen.getByRole('button', { name: '登录' }));
        await screen.findByRole('heading', { name: '产品管理' });
        expect(location.search).toBe('?q=440&page=2');
        expect(sessionStorage.getItem('zqzt.admin')).toBe('test-only-token');
    });
    it('shows a generated password only after creation and clears it after closing', async () => {
        mount('/admin/vendors?new=1');
        const user = userEvent.setup();
        const dialog = await screen.findByRole('dialog');
        await user.type(within(dialog).getByLabelText('企业 / 工厂名称'), vendor.data.name);
        expect((within(dialog).getByLabelText('公司登录名') as HTMLInputElement).value).toBe(vendor.data.name);
        await user.type(within(dialog).getByLabelText('所在地区'), '测试地区');
        await user.type(within(dialog).getByLabelText('主营产品'), '机械');
        await user.type(within(dialog).getByLabelText('电话'), '13900004001');
        await user.click(within(dialog).getByRole('button', { name: '创建草稿' }));
        await screen.findByText('Only-In-UI-Test!');
        expect(localStorage.getItem('Only-In-UI-Test!')).toBeNull();
        await user.click(screen.getByRole('button', { name: '完成' }));
        await screen.findByRole('button', { name: '保存云端草稿' });
        expect(screen.queryByText('Only-In-UI-Test!')).toBeNull();
        expect(location.pathname).toBe('/admin/contents/4');
        expect(fetchMock.mock.calls.filter(([url]) => url.includes('/review'))).toHaveLength(0);
    });
    it('locks pending submissions and requires a rejection reason', async () => {
        status = 'pending';
        mount('/admin/contents/4?edit=1');
        await screen.findByRole('button', { name: '通过审核' });
        expect((screen.getByRole('button', { name: '保存云端草稿' }) as HTMLButtonElement).disabled).toBe(true);
        const reject = screen.getByRole('button', { name: '驳回' }) as HTMLButtonElement;
        expect(reject.disabled).toBe(true);
        await userEvent.type(screen.getByRole('textbox', { name: '审核原因 / 备注' }), '请补充图片');
        expect(reject.disabled).toBe(false);
    });
    it('renders extension explanations without executable actions or fake data', async () => {
        mount('/admin/extensions/capture');
        await screen.findByRole('heading', { name: '数据采集', level: 2 });
        expect(document.querySelector('.admin-extension-page')).toBeTruthy();
        expect(document.querySelector('.admin-extension-page button')).toBeNull();
        expect((document.querySelector('.admin-extension-menu') as HTMLDetailsElement).open).toBe(false);
    });
});
