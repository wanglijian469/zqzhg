import {HomeDisplay} from './home-display';
import {useBatchReview,ReviewCheckbox} from './batch-review';
import {PasswordResetButton} from './password-reset';
import {FormGuide,useGuidedForm} from './form-guide';
import {CategoryPicker,CategorySuggestions} from './category-picker';
import React, { useContext, useEffect, useRef, useState } from 'react';
import {ProfileEditor,ShowroomTools} from './company-profile';
import { Link, NavLink, Navigate, Route, Routes, useLocation, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { Activity, ArrowLeft, ArrowUpRight, Building2, ChevronLeft, ChevronRight, FileText, FolderTree, Globe2, Home, ImageIcon, LayoutDashboard, Leaf, LogOut, Menu, Plus, Search, Settings, ShieldCheck, Star, Tractor, Users, X } from 'lucide-react';
import { api, APIError, body, token } from './api';
import { emptyData, type AdminAccount, type AdminDashboard, type AdminSystem, type Collection, type Data, type Item, type Operation, type Page, type Taxonomy } from './types';
import { Confirmation, Contact, ContentInfo, ErrorState, Gallery, Loading, MediaImage, Modal, Pager, ReviewHistory, display, useLanguage, useLoad } from './main';
import './admin.css';
import {ResourceManager} from './resources';
const groups = [
    { key: 'overview', items: [{ key: 'dashboard', path: 'dashboard', icon: LayoutDashboard }] },
    { key: 'business', items: [{ key: 'vendors', path: 'vendors', icon: Building2 }, { key: 'productManagement', path: 'products', icon: Tractor }, { key: 'vendorReviews', path: 'vendor-reviews', icon: ShieldCheck }, { key: 'productReviews', path: 'product-reviews', icon: ShieldCheck }] },
    { key: 'presentation', items: [{key:'resourceVideos',path:'videos',icon:FileText},{key:'resourceCatalogs',path:'catalogs',icon:FileText},{ key: 'categories', path: 'categories', icon: FolderTree }, { key: 'regions', path: 'regions', icon: Globe2 }, { key: 'homeDisplay', path: 'featured', icon: Star }] },
    { key: 'systemManagement', items: [{ key: 'accounts', path: 'accounts', icon: Users }, { key: 'logs', path: 'logs', icon: FileText }, { key: 'systemStatus', path: 'system', icon: Activity }] },
    { key: 'extensions', items: ['capture', 'banners', 'articles', 'analytics', 'roles', 'backups'].map(key => ({ key, path: 'extensions/' + key, icon: Settings })) }
];
const errorCode = (e: unknown) => (e as APIError).code || 'internal_error';
const timeText = (value?: string) => value ? new Date(value).toLocaleString() : '—';
const objectLabel = (kind: string) => kind === 'vendor' ? 'vendors' : kind === 'product' ? 'productManagement' : kind === 'region' ? 'regionObject' : kind === 'category' ? 'categoryObject' : kind;
function Notice({ children }: {
    children: React.ReactNode;
}) { return <p className="admin-notice">{children}</p>; }
export function AdminConsole() {
    const { t, lang, toggle } = useLanguage();
    const location = useLocation();
    const navigate = useNavigate();
    const [signed, setSigned] = useState(!!token('admin'));
    const [expired, setExpired] = useState(false);
    const [username, setUsername] = useState('admin');
    const [password, setPassword] = useState('');
    const [account, setAccount] = useState('');
    const [error, setError] = useState('');
    const [busy, setBusy] = useState(false);
    const [mobile, setMobile] = useState(window.innerWidth <= 900);
    useEffect(() => { const resize = () => setMobile(window.innerWidth <= 900); window.addEventListener('resize', resize); return () => window.removeEventListener('resize', resize); }, []);
    const [collapsed, setCollapsed] = useState(() => localStorage.getItem('zqzt.admin.collapsed') === '1');
    const [drawer, setDrawer] = useState(false);
    const drawerRef = useRef<HTMLElement>(null);
    const menuRef = useRef<HTMLButtonElement>(null);
    useEffect(() => { const expire = () => { sessionStorage.removeItem('zqzt.admin'); setSigned(false); setExpired(true); setPassword(''); setAccount(''); }; window.addEventListener('zqzt.admin.expired', expire); return () => window.removeEventListener('zqzt.admin.expired', expire); }, []);
    useEffect(() => { if (signed)
        api<AdminDashboard>('/admin/dashboard', {}, 'admin').then(r => setAccount(r.account.username)).catch(e => setError(errorCode(e))); }, [signed]);
    useEffect(() => { setDrawer(false); }, [location.pathname]);
    useEffect(() => { if (!drawer)
        return; const old = document.body.style.overflow; document.body.style.overflow = 'hidden'; const sidebar = drawerRef.current; sidebar?.querySelector<HTMLButtonElement>('button')?.focus(); const keyboard = (e: KeyboardEvent) => { if (e.key === 'Escape') {
        setDrawer(false);
        menuRef.current?.focus();
    } if (e.key === 'Tab') {
        const nodes = Array.from(sidebar?.querySelectorAll<HTMLElement>('a,button,summary') || []).filter(el => el.getClientRects().length > 0);
        const first = nodes[0], last = nodes.at(-1);
        if (e.shiftKey && document.activeElement === first) {
            e.preventDefault();
            last?.focus();
        }
        else if (!e.shiftKey && document.activeElement === last) {
            e.preventDefault();
            first?.focus();
        }
    } }; document.addEventListener('keydown', keyboard); return () => { document.body.style.overflow = old; document.removeEventListener('keydown', keyboard); menuRef.current?.focus(); }; }, [drawer]);
    const active = groups.flatMap(g => g.items).find(i => location.pathname === '/admin/' + i.path);
    const title = t(active?.key || (location.pathname.includes('/contents/') ? 'details' : 'dashboard'));
    if (!signed)
        return <div className="admin-login"><div className="admin-login-story"><Link to="/" className="admin-brand"><Tractor />{t('brand')}</Link><span className="eyebrow">{t('console')}</span><h1>{t('loginTitle')}</h1><p>{t('loginSubtitle')}</p><Leaf size={140} strokeWidth={.7}/></div><div className="admin-login-form"><button className="text-link" onClick={toggle}><Globe2 size={16}/>{lang === 'zh' ? 'English' : '中文'}</button><h2>{t('adminLogin')}</h2>{expired && <Notice>{t('sessionExpired')}</Notice>}<form onSubmit={async (e) => { e.preventDefault(); setBusy(true); setError(''); try {
            const r = await api<{
                token: string;
            }>('/auth/admin', { method: 'POST', body: body({ username, password }) });
            sessionStorage.setItem('zqzt.admin', r.token);
            setPassword('');
            setSigned(true);
            setExpired(false);
            if (location.pathname === '/admin' || location.pathname === '/admin/login')
                navigate('/admin/dashboard', { replace: true });
        }
        catch (e) {
            setError(errorCode(e));
        }
        finally {
            setBusy(false);
        } }}><label>{t('username')}<input required autoComplete="username" value={username} onChange={e => setUsername(e.target.value)}/></label><label>{t('password')}<input required type="password" autoComplete="current-password" value={password} onChange={e => setPassword(e.target.value)}/></label>{error && <ErrorState code={error}/>}<button className="button" disabled={busy}>{busy ? t('loading') : t('login')}<ArrowUpRight size={17}/></button></form><Link className="text-link" to="/">{t('website')}</Link></div></div>;
    return <div className={'admin-app ' + (collapsed ? 'collapsed ' : '') + (drawer ? 'drawer-open' : '')}>{drawer && <button className="admin-shade" aria-label={t('close')} onClick={() => setDrawer(false)}/>}<aside ref={drawerRef} className="admin-sidebar" inert={mobile && !drawer} aria-hidden={mobile && !drawer} aria-label={t('console')} {...(drawer ? { role: 'dialog', 'aria-modal': true } : {})}><Link className="admin-brand" to="/admin/dashboard"><span><Tractor size={25}/></span><b>{t('brand')}<small>{t('console')}</small></b></Link><button className="admin-drawer-close icon-button" aria-label={t('close')} onClick={() => setDrawer(false)}><X /></button><nav className="admin-navigation">{groups.map(g => g.key === 'extensions' ? <details key={g.key} className="admin-extension-menu"><summary title={t('extensions')}><Settings size={17}/><span>{t('extensions')}</span><small>{t('planned')}</small></summary>{g.items.map(i => <NavLink key={i.path} title={t(i.key)} to={'/admin/' + i.path}><i.icon size={18}/><span>{t(i.key)}</span><small>{t('planned')}</small></NavLink>)}</details> : <div className="admin-nav-group" key={g.key}><p>{t(g.key)}</p>{g.items.map(i => <NavLink title={t(i.key)} key={i.path} to={'/admin/' + i.path}><i.icon size={18}/><span>{t(i.key)}</span></NavLink>)}</div>)}</nav><button className="admin-collapse" onClick={() => { setCollapsed(!collapsed); localStorage.setItem('zqzt.admin.collapsed', collapsed ? '0' : '1'); }} aria-label={t(collapsed ? 'expand' : 'collapse')}>{collapsed ? <ChevronRight size={18}/> : <ChevronLeft size={18}/>}<span>{t('collapse')}</span></button></aside><div className="admin-workspace" inert={drawer}><header className="admin-topbar"><div className="admin-breadcrumb"><button ref={menuRef} className="admin-menu icon-button" aria-label={t('openNavigation')} onClick={() => setDrawer(true)}><Menu /></button><span>{t('console')}</span><ChevronRight size={14}/><b>{title}</b></div><div className="admin-tools"><span className="admin-account"><ShieldCheck size={15}/>{account || username}</span><button className="text-link" onClick={toggle}><Globe2 size={16}/>{lang === 'zh' ? 'English' : '中文'}</button><Link className="text-link" to="/"><Home size={16}/><span>{t('website')}</span></Link><button className="text-link" aria-label={t('logout')} onClick={async () => { await api('/auth/logout', { method: 'POST', body: '{}' }, 'admin').catch(() => { }); sessionStorage.removeItem('zqzt.admin'); setSigned(false); setPassword(''); setAccount(''); }}><LogOut size={16}/><span>{t('logout')}</span></button></div></header><main className="admin-main"><div className="admin-page-heading"><span className="eyebrow">{t('hall')} / {t('console')}</span><h1>{title}</h1></div><Routes><Route path="/admin" element={<Navigate to="/admin/dashboard" replace/>}/><Route path="/admin/login" element={<Navigate to="/admin/dashboard" replace/>}/><Route path="/admin/dashboard" element={<Dashboard />}/><Route path="/admin/vendors" element={<ContentTable key="vendors" kind="vendor"/>}/><Route path="/admin/products" element={<ContentTable key="products" kind="product"/>}/><Route path="/admin/vendor-reviews" element={<ContentTable key="vendor-reviews" kind="vendor" review/>}/><Route path="/admin/product-reviews" element={<ContentTable key="product-reviews" kind="product" review/>}/><Route path="/admin/contents/:id" element={<ContentDetail key={location.pathname}/>}/><Route path="/admin/videos" element={<ResourceManager kind="video" admin/>}/><Route path="/admin/catalogs" element={<ResourceManager kind="catalog" admin/>}/><Route path="/admin/categories" element={<TaxonomyPage key="categories" kind="categories"/>}/><Route path="/admin/regions" element={<TaxonomyPage key="regions" kind="regions"/>}/><Route path="/admin/featured" element={<HomeDisplay />}/><Route path="/admin/accounts" element={<Accounts />}/><Route path="/admin/logs" element={<Logs />}/><Route path="/admin/system" element={<SystemStatus />}/><Route path="/admin/extensions/:module" element={<Extension />}/><Route path="*" element={<ErrorState code="not_found"/>}/></Routes></main></div></div>;
}
function Dashboard() { const { t } = useLanguage(); const r = useLoad<AdminDashboard>('/admin/dashboard', 'admin'); if (r.error)
    return <ErrorState code={r.error} retry={r.reload}/>; if (!r.data)
    return <Loading />; return <><div className="admin-stats">{[['vendor_total', 'totalVendors', 'vendors'], ['product_total', 'totalProducts', 'products'], ['vendor_public', 'publicVendors', 'vendors?public_state=public'], ['product_public', 'publicProducts', 'products?public_state=public'], ['vendor_pending', 'pendingVendors', 'vendor-reviews'], ['product_pending', 'pendingProducts', 'product-reviews']].map(([key, label, path]) => <Link key={key} className={'admin-stat ' + (key.includes('pending') ? 'pending-stat' : '')} to={'/admin/' + path}><span>{t(label)}</span><strong>{r.data!.counts[key] ?? 0}</strong><ArrowUpRight size={18}/></Link>)}</div><div className="admin-dashboard-grid"><section className="admin-panel"><h2>{t('recentOperations')}</h2><OperationTable rows={r.data.recent_operations}/><Link className="text-link" to="/admin/logs">{t('all')}<ArrowUpRight size={15}/></Link></section><section className="admin-panel"><h2>{t('shortcuts')}</h2><div className="admin-shortcuts">{[['addVendor', 'vendors?new=1'], ['addProduct', 'products?new=1'], ['vendorReviews', 'vendor-reviews'], ['productReviews', 'product-reviews'], ['featured', 'featured'], ['systemStatus', 'system']].map(([label, path]) => <Link key={path} to={'/admin/' + path}><Plus size={17}/>{t(label)}<ArrowUpRight size={16}/></Link>)}</div><Notice>{t('submitNotice')}</Notice></section></div></>; }
function useFilters() { const [params, setParams] = useSearchParams(); const put = (key: string, value: string) => { const next = new URLSearchParams(params); value ? next.set(key, value) : next.delete(key); next.delete('page'); setParams(next); }; const paging = (p: number) => { const next = new URLSearchParams(params); next.set('page', String(p)); setParams(next); }; return { params, setParams, put, paging }; }
function EmptyRows({ columns }: {
    columns: number;
}) { const { t } = useLanguage(); return <tr><td colSpan={columns} className="admin-empty">{t('noData')}</td></tr>; }
function Status({ item }: {
    item: Item;
}) { const { t } = useLanguage(); return <><span className={'badge ' + item.status}>{t(item.status)}</span><small className={item.public_visible ? 'admin-public' : 'muted'}>{t(item.public_visible ? 'published' : 'unpublished')}</small></>; }
function SearchField({ value, onChange, label }: {
    value: string;
    onChange: (v: string) => void;
    label?: string;
}) { const { t } = useLanguage(); return <label className="admin-search"><span>{label || t('keyword')}</span><div><Search size={16}/><input value={value} onChange={e => onChange(e.target.value)} placeholder={t('searchOptions')}/></div></label>; }
function RemotePicker({ kind, value, onChange }: {
    kind: 'vendor' | 'account';
    value: number;
    onChange: (id: number) => void;
}) { const { t, lang } = useLanguage(); const [q, setQ] = useState(''); const [page, setPage] = useState(1); const [selected, setSelected] = useState(''); const path = kind === 'vendor' ? '/admin/contents?kind=vendor' : '/admin/accounts?unassigned=1'; const r = useLoad<Collection<Item | AdminAccount>>(path + '&q=' + encodeURIComponent(q) + '&page=' + page + '&page_size=10', 'admin'); return <div className="admin-picker"><SearchField label={t(kind === 'vendor' ? 'chooseVendor' : 'chooseAccount')} value={q} onChange={v => { setQ(v); setPage(1); }}/>{value > 0 && <p className="admin-picker-selection">{t('selectedOption')} · {selected || '#' + value}<button type="button" onClick={() => onChange(0)} aria-label={t('clear')}><X size={14}/></button></p>}{r.error ? <ErrorState code={r.error} retry={r.reload}/> : !r.data ? <Loading /> : <><div className="admin-picker-options">{r.data.items.map(row => { const label = 'data' in row ? display(row.data, lang, 'name') : row.login_name || row.username; return <button type="button" className={row.id === value ? 'selected' : ''} key={row.id} onClick={() => { onChange(row.id); setSelected(label); }}><span>{label}</span><small>#{row.id} · {t('select')}</small></button>; })}{!r.data.items.length && <p>{t('noData')}</p>}</div><Pager page={page} total={r.data.total} size={10} onPage={setPage}/></>}</div>; }
function ContentTable({ kind, review = false }: {
    kind: 'vendor' | 'product';
    review?: boolean;
}) {
    const { t, lang } = useLanguage();
    const { params, setParams, put, paging } = useFilters();
    const location = useLocation();
    const ask = useContext(Confirmation);
    const tax = useLoad<Taxonomy>('/admin/taxonomy', 'admin');
    const [error, setError] = useState('');
    const [busy, setBusy] = useState(false);
    const [create, setCreate] = useState(params.get('new') === '1');
    const [vendorFilter, setVendorFilter] = useState(false);
    const query = new URLSearchParams(params);
    query.delete('new');
    query.delete('account_id');
    query.set('kind', kind);
    if (review && !query.has('status'))
        query.set('status', 'pending');
    const r = useLoad<Page>('/admin/contents?' + query, 'admin');
    const batch=useBatchReview(r.data?.items,location.pathname+'?'+query.toString(),r.reload);
    const close = () => { setCreate(false); const next = new URLSearchParams(params); next.delete('new'); next.delete('account_id'); setParams(next, { replace: true }); };
    useEffect(() => { if (params.get('new') === '1')
        setCreate(true); }, [params]);
    const act = async (item: Item, action: string) => { if (action === 'offline' && !await ask(t('offlineConfirm')))
        return; setBusy(true); setError(''); try {
        await api('/admin/contents/' + item.id + '/' + action, { method: 'POST', body: body({ revision_id: item.revision_id }) }, 'admin');
        r.reload();
    }
    catch (e) {
        setError(errorCode(e));
    }
    finally {
        setBusy(false);
    } };
    return <>{review && <Notice>{t('reviewHelp')}</Notice>}<div className="admin-panel"><div className="admin-list-tools"><span>{t('total')} <b>{r.data?.total ?? '—'}</b> {t('records')}</span>{!review && <button className="button" onClick={() => setCreate(true)}><Plus size={16}/>{t(kind === 'vendor' ? 'addVendor' : 'addProduct')}</button>}</div><div className="admin-filters"><SearchField value={params.get('q') || ''} onChange={v => put('q', v)}/>{kind === 'vendor' ? <label>{t('region')}<select value={params.get('region_id') || ''} onChange={e => put('region_id', e.target.value)}><option value="">{t('allRegions')}</option>{tax.data?.regions.map(c => <option key={c.id} value={c.id}>{lang === 'en' && c.name_en ? c.name_en : c.name}</option>)}</select></label> : <><label>{t('vendor')}<button className="admin-select-button" onClick={() => setVendorFilter(true)}>{params.get('vendor_id') ? '#' + params.get('vendor_id') : t('allVendors')}</button></label><label>{t('category')}<select value={params.get('category_id') || ''} onChange={e => put('category_id', e.target.value)}><option value="">{t('allCategories')}</option>{tax.data?.categories.map(c => <option key={c.id} value={c.id}>{lang === 'en' && c.name_en ? c.name_en : c.name}</option>)}</select></label></>}<label>{t('reviewStatus')}<select value={params.get('status') ?? (review ? 'pending' : '')} onChange={e => { const next = new URLSearchParams(params); next.set('status', e.target.value); next.delete('page'); setParams(next); }}><option value="">{t('allStatus')}</option>{['draft', 'pending', 'approved', 'rejected'].map(s => <option key={s} value={s}>{t(s)}</option>)}</select></label><label>{t('visibility')}<select value={params.get('public_state') || ''} onChange={e => put('public_state', e.target.value)}><option value="">{t('allVisibility')}</option><option value="public">{t('published')}</option><option value="unpublished">{t('unpublished')}</option></select></label><button className="text-link" onClick={() => setParams({})}>{t('resetFilter')}</button></div>{error && <ErrorState code={error}/>} {batch.toolbar}<div className="admin-table-scroll">{r.error ? <ErrorState code={r.error} retry={r.reload}/> : !r.data ? <Loading /> : <table className="admin-table"><thead><tr><th className="admin-review-select"><ReviewCheckbox label={t('selectPendingPage')} checked={!!batch.eligible.length&&batch.selected.length===batch.eligible.length} partial={batch.selected.length>0&&batch.selected.length<batch.eligible.length} disabled={busy||batch.busy||!batch.eligible.length} onChange={batch.selectPage}/></th><th>{t(kind === 'vendor' ? 'company' : 'productName')}</th><th>{t(kind === 'vendor' ? 'regionName' : 'vendor')}</th><th>{t(kind === 'vendor' ? 'contact' : 'category')}</th><th>{t('reviewStatus')} / {t('visibility')}</th>{kind === 'vendor' && <th>{t('productCount')}</th>}<th>{t('updatedAt')}</th><th>{t('actions')}</th></tr></thead><tbody>{r.data.items.map(item => <tr key={item.id}><td className="admin-review-select"><ReviewCheckbox label={t('selectReviewItem')+' '+display(item.data,lang,'name')+' #'+item.id} checked={batch.selected.some(v=>v.revision_id===item.revision_id)} disabled={busy||batch.busy||item.status!=='pending'} onChange={()=>batch.toggle(item)}/></td><td><div className="admin-item-name">{kind === 'product' && (item.data.media_ids?.[0] ? <MediaImage id={item.data.media_ids[0]} role="admin"/> : <span className="admin-image-placeholder"><ImageIcon size={20}/></span>)}<div><Link to={'/admin/contents/' + item.id} state={{ returnTo: location.pathname + location.search }}>{display(item.data, lang, 'name')}</Link><small>#{item.id}</small></div></div></td><td>{kind === 'vendor' ? item.data.region_name : <Link to={'/admin/contents/' + item.vendor_id}>{lang === 'en' && item.vendor?.name_en ? item.vendor.name_en : item.vendor?.name || '#' + item.vendor_id}</Link>}</td><td>{kind === 'vendor' ? <>{item.data.phone || '—'}<small>{item.data.wechat}</small></> : tax.data?.categories.find(c => c.id === item.data.category_id)?.[lang === 'en' ? 'name_en' : 'name'] || tax.data?.categories.find(c => c.id === item.data.category_id)?.name || '—'}</td><td><Status item={item}/></td>{kind === 'vendor' && <td><Link to={'/admin/products?vendor_id=' + item.id}>{item.product_count || 0}</Link></td>}<td className="admin-time">{timeText(item.updated_at)}</td><td><div className="admin-row-actions"><Link to={'/admin/contents/' + item.id + (!review && item.status !== 'pending' ? '?edit=1' : '')} state={{ returnTo: location.pathname + location.search }}>{t(review || item.status === 'pending' ? 'view' : 'edit')}</Link>{kind==='vendor'&&!review&&(item.status==='pending'?<span className="admin-cover-locked"><button disabled title={t('coverPendingLocked')}>{t('setVendorCover')}</button><small>{t('coverPendingLocked')}</small></span>:<Link to={'/admin/contents/'+item.id+'?edit=1&section=brand#brand-images'} state={{returnTo:location.pathname+location.search}}>{t('setVendorCover')}</Link>)}{item.status==='pending'&&<><button disabled={busy||batch.busy} onClick={()=>batch.open('approve',[item])}>{t('approve')}</button><button className="danger" disabled={busy||batch.busy} onClick={()=>batch.open('reject',[item])}>{t('reject')}</button></>}{!review && item.status === 'draft' && <button disabled={busy} onClick={() => void act(item, 'submit')}>{t('submit')}</button>}{item.visible && <button className="danger" disabled={busy} onClick={() => void act(item, 'offline')}>{t('offline')}</button>}</div></td></tr>)}{!r.data.items.length && <EmptyRows columns={kind === 'vendor' ? 8 : 7}/>}</tbody></table>}</div>{r.data && <Pager page={r.data.page} size={r.data.page_size} total={r.data.total} onPage={paging}/>}</div>{batch.modal}{create && <CreateContent kind={kind} defaultVendor={Number(params.get('vendor_id') || 0)} defaultAccount={Number(params.get('account_id') || 0)} close={close}/>}{vendorFilter && <Modal close={() => setVendorFilter(false)}><RemotePicker kind="vendor" value={Number(params.get('vendor_id') || 0)} onChange={id => { put('vendor_id', id ? String(id) : ''); setVendorFilter(false); }}/></Modal>}</>;
}
function CreateContent({ kind, defaultVendor, defaultAccount, close }: {
    kind: 'vendor' | 'product';
    defaultVendor: number;
    defaultAccount: number;
    close: () => void;
}) {
    const { t } = useLanguage();
    const navigate = useNavigate();
    const [data, setData] = useState({ ...emptyData });
    const [mode, setMode] = useState(defaultAccount ? 'existing' : 'new');
    const [accountId, setAccountId] = useState(defaultAccount);
    const [vendorId, setVendorId] = useState(defaultVendor);
    const [loginName, setLoginName] = useState('');
    const [customLogin, setCustomLogin] = useState(false);
 const formRef=useRef<HTMLFormElement>(null);const extra=kind==='vendor'&&mode==='new'?{login_name:loginName}:{};const guide=useGuidedForm(data,kind,extra);
    const [error, setError] = useState('');
    const [busy, setBusy] = useState(false);
    const [credentials, setCredentials] = useState<{
        login_name: string;
        initial_password: string;
        id: number;
    }>();
    const [copied, setCopied] = useState(false);
    const tax = useLoad<Taxonomy>('/admin/taxonomy', 'admin');
    const set = (key: keyof Data, value: string | number) => setData(d => ({ ...d, [key]: value }));
    const finish = (id: number) => { close(); navigate('/admin/contents/' + id + '?edit=1'); };
    if (credentials)
        return <Modal close={() => finish(credentials.id)}><h2>{t('initialPassword')}</h2><Notice>{t('passwordOnce')}</Notice><dl className="admin-credentials"><dt>{t('loginName')}</dt><dd>{credentials.login_name}</dd><dt>{t('password')}</dt><dd>{credentials.initial_password}</dd></dl><div className="form-actions"><button className="button secondary" onClick={() => navigator.clipboard.writeText(credentials.login_name + '\n' + credentials.initial_password).then(() => setCopied(true)).catch(() => setError('network_error'))}>{t(copied ? 'copied' : 'copyPassword')}</button><button className="button" onClick={() => finish(credentials.id)}>{t('done')}</button></div>{error && <ErrorState code={error}/>}</Modal>;
    return <Modal close={close}><h2>{t(kind === 'vendor' ? 'addVendor' : 'addProduct')}</h2><Notice>{t(kind === 'vendor' ? 'newAccountHint' : 'productCapacity')}</Notice>{kind==='vendor'&&<p className="muted">{t('brandImagesAfterCreate')}</p>}<form ref={formRef} onSubmit={async (e) => { e.preventDefault(); setError('');if(!guide.check(formRef.current))return; if (kind === 'vendor' && mode === 'existing' && !accountId || kind === 'product' && !vendorId) {
        guide.setIssues([{field:kind==='vendor'?'account_id':'vendor_id',code:'required'}]);
        return;
    } setBusy(true); try {
        if (kind === 'vendor') {
            const result = await api<{
                item: Item;
                login_name?: string;
                initial_password?: string;
            }>('/admin/vendors', { method: 'POST', body: body({ data, login_name: loginName, account_id: mode === 'existing' ? accountId : 0 }) }, 'admin');
            if (result.initial_password)
                setCredentials({ login_name: result.login_name!, initial_password: result.initial_password, id: result.item.id });
            else
                finish(result.item.id);
        }
        else {
            const item = await api<Item>('/admin/vendors/' + vendorId + '/products', { method: 'POST', body: body({ data }) }, 'admin');
            finish(item.id);
        }
    }
    catch (e) {
        guide.setIssues((e as APIError).fields||[]);setError((e as APIError).fields?.length?'':errorCode(e));
    }
    finally {
        setBusy(false);
    } }}><FormGuide form={formRef} data={data} kind={kind} extra={extra} issues={guide.issues}/>{kind === 'vendor' ? <><label>{t('accountMode')}<select value={mode} onChange={e => setMode(e.target.value)}><option value="new">{t('addCompanyAccount')}</option><option value="existing">{t('existingAccount')}</option></select></label>{mode === 'existing' && <div data-field="account_id"><RemotePicker kind="account" value={accountId} onChange={setAccountId}/></div>}</> : <div data-field="vendor_id"><RemotePicker kind="vendor" value={vendorId} onChange={setVendorId}/></div>}<label data-field="name">{t(kind === 'vendor' ? 'company' : 'productName')}<input required maxLength={200} value={data.name} onChange={e => { set('name', e.target.value); if (!customLogin)
        setLoginName(e.target.value); }}/></label>{kind === 'vendor' ? <><label data-field="region_name">{t('regionName')}<input required maxLength={100} value={data.region_name} onChange={e => set('region_name', e.target.value)} list="admin-create-regions"/></label><datalist id="admin-create-regions">{tax.data?.regions.filter(c => c.enabled).map(c => <option key={c.id} value={c.name}/>)}</datalist><label data-field="main_products">{t('mainProducts')}<input required maxLength={500} value={data.main_products} onChange={e => set('main_products', e.target.value)}/></label><Notice>{t('contactRequired')}</Notice><div className="admin-form-grid"><label data-field="phone">{t('phone')}<input maxLength={32} value={data.phone} onChange={e => set('phone', e.target.value)}/></label><label data-field="wechat">{t('wechat')}<input maxLength={100} value={data.wechat} onChange={e => set('wechat', e.target.value)}/></label></div>{mode === 'new' && <label data-field="login_name">{t('loginName')}<input required maxLength={100} value={loginName} onChange={e => { setCustomLogin(true); setLoginName(e.target.value); }} autoComplete="off"/></label>}</> : <><label>{t('category')}<select value={data.category_id} onChange={e => set('category_id', Number(e.target.value))}><option value={0}>{t('choose')}</option>{tax.data?.categories.filter(c => c.enabled).map(c => <option key={c.id} value={c.id}>{c.name}</option>)}</select></label><label data-field="keywords">{t('keywords')}<input maxLength={500} value={data.keywords} onChange={e => set('keywords', e.target.value)}/></label><Notice>{t('optionalName')}</Notice></>}{error && <ErrorState code={error}/>}<div className="form-actions"><button type="button" className="button secondary" onClick={close}>{t('cancel')}</button><button className="button" disabled={busy}>{t('createDraft')}</button></div></form></Modal>;
}
function ContentDetail() {
    const { id } = useParams();
    const { t } = useLanguage();
    const location = useLocation();
    const [params, setParams] = useSearchParams();
    const r = useLoad<Item>('/admin/contents/' + id, 'admin');
    const [error, setError] = useState('');
    const [note, setNote] = useState('');
    const [busy, setBusy] = useState(false);
    const ask = useContext(Confirmation);
    const [tab, setTab] = useState('latest');
    const item = r.data;
    const returnTo = location.state?.returnTo;
    const back = typeof returnTo === 'string' && returnTo.startsWith('/admin/') ? returnTo : item?.kind === 'product' ? '/admin/products' : '/admin/vendors';
    const act = async (action: string) => { if (!item)
        return; if (action === 'offline' && !await ask(t('offlineConfirm')))
        return; setBusy(true); setError(''); try {
        if (action === 'approve' || action === 'reject')
            await api('/admin/revisions/' + item.revision_id + '/review', { method: 'POST', body: body({ decision: action, note }) }, 'admin');
        else
            await api('/admin/contents/' + item.id + '/' + action, { method: 'POST', body: body({ revision_id: item.revision_id }) }, 'admin');
        setNote('');
        r.reload();
    }
    catch (e) {
        setError(errorCode(e));
    }
    finally {
        setBusy(false);
    } };
    if (r.error)
        return <ErrorState code={r.error} retry={r.reload}/>;
    if (!item)
        return <Loading />;
    return <><div className="admin-detail-toolbar"><Link className="text-link" to={back}><ArrowLeft size={16}/>{t('backList')}</Link><div><span className="muted">{t('owner')} #{item.owner_id}</span>{item.kind === 'vendor' && <Link className="button secondary" to={'/admin/products?vendor_id=' + item.id}>{t('productList')} ({item.product_count || 0})</Link>}</div></div><div className="admin-detail-grid"><section className="admin-panel"><div className="tabs"><button className={tab === 'latest' ? 'active' : ''} onClick={() => setTab('latest')}>{t('latestVersion')}</button><button className={tab === 'public' ? 'active' : ''} onClick={() => setTab('public')}>{t('publicVersion')}</button></div>{tab === 'public' ? item.published ? <><Status item={item}/><ContentInfo item={item.published} role="admin"/><Contact data={item.published.data}/></> : <Notice>{t('noPublicVersion')}</Notice> : <><div className="admin-detail-meta"><Status item={item}/><span>#{item.id} · V{item.revision_id}</span></div>{params.get('edit') === '1' ? <ContentEditor key={item.revision_id} item={item} focusBrand={params.get('section')==='brand'} saved={() => { r.reload(); setParams({}); }}/> : <><ContentInfo item={item} role="admin"/><Contact data={item.data}/>{item.status !== 'pending' && <button className="button secondary" onClick={() => setParams({ edit: '1' })}>{t('editDraft')}</button>}</>}{item.review_note && <Notice>{item.review_note === 'offline' ? t('offline') : item.review_note}</Notice>}</>}</section><aside className="admin-detail-side">{item.kind==='vendor'&&<section className="admin-panel"><h2>{t('accounts')}</h2><PasswordResetButton ownerID={item.owner_id} companyName={item.data.name}/></section>}{item.kind==='vendor'&&<ShowroomTools item={item} admin reload={r.reload}/>}<section className="admin-panel"><h2>{t('actions')}</h2><Notice>{t('draftHelp')}</Notice>{item.status === 'pending' && <><Notice>{t('pendingLocked')}</Notice><label>{t('note')}<textarea maxLength={1000} value={note} onChange={e => setNote(e.target.value)} placeholder={t('reason')}/></label></>}{error && <ErrorState code={error}/>}<div className="admin-detail-actions">{item.status === 'pending' ? <><button className="button" disabled={busy} onClick={() => void act('approve')}>{t('approve')}</button><button className="button secondary" disabled={busy || !note.trim()} onClick={() => void act('reject')}>{t('reject')}</button></> : item.status === 'draft' && <button className="button" disabled={busy || params.get('edit') === '1'} onClick={() => void act('submit')}>{t('submit')}</button>}{item.visible && <button className="button danger secondary" disabled={busy} onClick={() => void act('offline')}>{t('offline')}</button>}</div>{params.get('edit') === '1' && <Notice>{t('saveFirst')}</Notice>}</section><section className="admin-panel"><ReviewHistory key={item.revision_id + item.status} id={item.id} role="admin"/></section></aside></div></>;
}
function ContentEditor({ item, saved,focusBrand=false }: {
    item: Item;
    saved: () => void;
    focusBrand?:boolean;
}) {
    const { t, lang } = useLanguage();
    const [data, setData] = useState<Data>({ ...item.data, media_ids: item.data.media_ids || [] });
    const [error, setError] = useState('');
    const [busy, setBusy] = useState(false);
    const [failedFiles, setFailedFiles] = useState<File[]>([]);
    const tax = useLoad<Taxonomy>('/admin/taxonomy', 'admin');
    const [brandPending,setBrandPending]=useState<Record<string,boolean>>({});const formRef=useRef<HTMLFormElement>(null);const guide=useGuidedForm(data,item.kind);
    const locked = item.status === 'pending';
    const set = (key: keyof Data, value: string | number) => setData(d => ({ ...d, [key]: value }));
    const upload = async (files: File[]) => { setBusy(true); setError(''); const failed: File[] = []; let capacity = 12 - (data.media_ids?.length || 0); for (const file of files) {
        if (capacity <= 0) {
            failed.push(file);
            setError('invalid_request');
            continue;
        }
        try {
            const form = new FormData();
            form.append('file', file);
            const result = await api<{
                id: number;
            }>('/admin/contents/' + item.id + '/media', { method: 'POST', body: form }, 'admin');
            setData(d => ({ ...d, media_ids: [...(d.media_ids || []), result.id] }));
            capacity--;
        }
        catch (e) {
            failed.push(file);
            guide.setIssues((e as APIError).fields||[]);setError(errorCode(e));
        }
    } setFailedFiles(failed); setBusy(false); };
    return <form ref={formRef} className="admin-editor" onSubmit={async (e) => { e.preventDefault();if(Object.values(brandPending).some(Boolean)){setError('brandImagePending');return};if(!guide.check(formRef.current))return; setBusy(true); setError(''); try {
        await api('/admin/contents/' + item.id + '/draft', { method: 'PUT', body: body({ revision_id: item.revision_id, data }) }, 'admin');
        saved();
    }
    catch (e) {
        setError(errorCode(e));
    }
    finally {
        setBusy(false);
    } }}><FormGuide form={formRef} data={data} kind={item.kind} issues={guide.issues}/><fieldset disabled={locked || busy}><div className="admin-form-grid"><label data-field="name">{t(item.kind === 'vendor' ? 'company' : 'productName')}<input required maxLength={200} value={data.name} onChange={e => set('name', e.target.value)}/></label><label data-field="name_en">{t('nameEN')}<input maxLength={200} value={data.name_en} onChange={e => set('name_en', e.target.value)}/></label>{item.kind === 'vendor' && <><label data-field="region_name">{t('regionName')}<select value={data.region_id || 0} onChange={e => { const id = Number(e.target.value); setData(d => ({ ...d, region_id: id, region_name: id ? tax.data?.regions.find(c => c.id === id)?.name || '' : d.region_name })); }}><option value={0}>{t('optional')}</option>{tax.data?.regions.map(c => <option disabled={!c.enabled} key={c.id} value={c.id}>{lang === 'en' && c.name_en ? c.name_en : c.name}</option>)}</select><input required value={data.region_name} maxLength={100} onChange={e => setData(d => ({ ...d, region_id: 0, region_name: e.target.value }))}/></label><label data-field="main_products">{t('mainProducts')}<input required maxLength={500} value={data.main_products} onChange={e => set('main_products', e.target.value)}/></label><label data-field="phone">{t('phone')}<input maxLength={32} value={data.phone} onChange={e => set('phone', e.target.value)}/></label><label data-field="wechat">{t('wechat')}<input maxLength={100} value={data.wechat} onChange={e => set('wechat', e.target.value)}/></label><label data-field="address">{t('address')}<input maxLength={500} value={data.address} onChange={e => set('address', e.target.value)}/></label></>}<CategoryPicker kind={item.kind} disabled={locked||busy} role="admin" ownerID={item.owner_id} value={data.category_id} onChange={id=>set('category_id',id)}/><label data-field="keywords">{t('keywords')}<input maxLength={500} value={data.keywords} onChange={e => set('keywords', e.target.value)}/></label></div><label data-field="description">{t('description')}<textarea value={data.description} maxLength={10000} onChange={e => set('description', e.target.value)}/></label><label data-field="description_en">{t('englishDescription')}<textarea value={data.description_en} maxLength={10000} onChange={e => set('description_en', e.target.value)}/></label>{item.kind === 'product' && <label data-field="parameters">{t('parameters')}<textarea maxLength={3500} value={data.parameters} onChange={e => set('parameters', e.target.value)}/></label>}{item.kind==='vendor'&&<ProfileEditor focusBrand={focusBrand} data={data} onChange={setData} role="admin" contentID={item.id} onBusy={setBusy} onPending={(slot,pending)=>{setBrandPending(p=>({...p,[slot]:pending}));if(!pending)setError(old=>old==='brandImagePending'?'':old)}}/>}<label>{t('photos')}<input type="file" accept="image/jpeg,image/png,image/webp" multiple disabled={(data.media_ids?.length || 0) >= 12} onChange={e => { void upload(Array.from(e.target.files || [])); e.target.value = ''; }}/></label><div className="admin-edit-images">{data.media_ids?.map((id, i) => <div key={id}><MediaImage id={id} role="admin"/><button type="button" aria-label={t('remove') + ' ' + (i + 1)} onClick={() => setData(d => ({ ...d, media_ids: d.media_ids!.filter(mid => mid !== id) }))}><X size={16}/></button></div>)}</div></fieldset>{failedFiles.length > 0 && <Notice>{t('photoRetry')} · {failedFiles.map(f => f.name).join(', ')} <button type="button" className="text-link" disabled={busy} onClick={() => void upload(failedFiles)}>{t('retry')}</button></Notice>}{error && <ErrorState code={error}/>}<button className="button" disabled={busy || locked}>{busy ? t('loading') : t('saveCloud')}</button></form>;
}
function TaxonomyPage({ kind }: {
    kind: 'categories' | 'regions';
}) {
    const { t, lang } = useLanguage();
    const { params, put, paging } = useFilters();
    const r = useLoad<Taxonomy>('/admin/taxonomy', 'admin');
    type Edit = {
        id?: number;
        name: string;
        name_en: string;
        parent_id: number;
        sort_order: number;
        enabled: boolean;
    };
    const [editing, setEditing] = useState<Edit>();
    const [error, setError] = useState('');
    const [busy, setBusy] = useState(false);
    const filtered = (r.data?.[kind] || []).filter(row => (!('suggestion_status' in row)||!row.suggestion_status||row.suggestion_status==='approved') && (row.name + ' ' + row.name_en).toLowerCase().includes((params.get('q') || '').toLowerCase()) && (!params.has('enabled') || String(Number(row.enabled)) === params.get('enabled')));
    const page = Math.max(1, Number(params.get('page')) || 1);
    const label = (id: number): string => { const c = r.data?.categories.find(c => c.id === id); return c ? (lang === 'en' && c.name_en ? c.name_en : c.name) : '—'; };
    const path = (id: number) => { const names: string[] = []; const seen = new Set<number>(); let current = id; while (current && !seen.has(current)) {
        seen.add(current);
        names.unshift(label(current));
        current = r.data?.categories.find(c => c.id === current)?.parent_id || 0;
    } return names.join(' / '); };
    return <>{kind==='categories'&&<CategorySuggestions onChanged={r.reload}/>}<div className="admin-panel"><div className="admin-list-tools"><b>{filtered.length} {t('records')}</b><button className="button" onClick={() => { setError(''); setEditing({ name: '', name_en: '', parent_id: 0, sort_order: 0, enabled: true }); }}><Plus size={16}/>{t(kind === 'categories' ? 'addCategory' : 'addRegion')}</button></div><div className="admin-filters"><SearchField value={params.get('q') || ''} onChange={v => put('q', v)}/><label>{t('enable')}<select value={params.get('enabled') || ''} onChange={e => put('enabled', e.target.value)}><option value="">{t('allStatus')}</option><option value="1">{t('on')}</option><option value="0">{t('off')}</option></select></label></div>{r.error ? <ErrorState code={r.error} retry={r.reload}/> : !r.data ? <Loading /> : <div className="admin-table-scroll"><table className="admin-table"><thead><tr><th>{t('name')}</th><th>{t('nameEN')}</th>{kind === 'categories' && <><th>{t('categoryTree')}</th><th>{t('sort')}</th></>}<th>{t('enable')}</th><th>{t('actions')}</th></tr></thead><tbody>{filtered.slice((page - 1) * 20, page * 20).map(row => <tr key={row.id}><td>{row.name}<small>#{row.id}</small></td><td>{row.name_en || '—'}</td>{'parent_id' in row && <><td>{path(row.id)}</td><td>{Number('sort_order' in row ? row.sort_order : 0)}</td></>}<td>{t(row.enabled ? 'on' : 'off')}</td><td><button className="text-link" onClick={() => { setError(''); setEditing({ ...row, parent_id: 'parent_id' in row ? Number(row.parent_id) : 0, sort_order: 'sort_order' in row ? Number(row.sort_order) : 0 }); }}>{t('edit')}</button></td></tr>)}{!filtered.length && <EmptyRows columns={6}/>}</tbody></table></div>}<Pager page={page} total={filtered.length} size={20} onPage={paging}/></div>{editing && <Modal close={() => setEditing(undefined)}><h2>{t(kind)}</h2><form onSubmit={async (e) => { e.preventDefault(); setBusy(true); setError(''); try {
        await api('/admin/' + kind + (editing.id ? '/' + editing.id : ''), { method: editing.id ? 'PUT' : 'POST', body: body({ ...editing, hall_id: 1 }) }, 'admin');
        setEditing(undefined);
        r.reload();
    }
    catch (e) {
        setError(errorCode(e));
    }
    finally {
        setBusy(false);
    } }}><label>{t('name')}<input required maxLength={100} value={editing.name} onChange={e => setEditing({ ...editing, name: e.target.value })}/></label><label>{t('nameEN')}<input maxLength={100} value={editing.name_en} onChange={e => setEditing({ ...editing, name_en: e.target.value })}/></label>{kind === 'categories' && <><label>{t('parent')}<select value={editing.parent_id} onChange={e => setEditing({ ...editing, parent_id: Number(e.target.value) })}><option value={0}>{t('root')}</option>{r.data?.categories.filter(c => c.id !== editing.id).map(c => <option key={c.id} value={c.id}>{path(c.id)}</option>)}</select></label><label>{t('sort')}<input type="number" value={editing.sort_order} onChange={e => setEditing({ ...editing, sort_order: Number(e.target.value) })}/></label></>}<label className="checkbox"><input type="checkbox" checked={editing.enabled} onChange={e => setEditing({ ...editing, enabled: e.target.checked })}/>{t('enable')}</label>{error && <ErrorState code={error}/>}<button className="button" disabled={busy}>{t('save')}</button></form></Modal>}</>;
}
function Accounts() { const { t } = useLanguage(); const { params, put, paging } = useFilters(); const r = useLoad<Collection<AdminAccount>>('/admin/accounts?' + params, 'admin'); return <><Notice>{t('accountHelp')}</Notice><div className="admin-panel"><div className="admin-list-tools"><span>{r.data?.total ?? '—'} {t('records')}</span><Link className="button" to="/admin/vendors?new=1"><Plus size={16}/>{t('addVendor')}</Link></div><div className="admin-filters"><SearchField value={params.get('q') || ''} onChange={v => put('q', v)}/><label className="checkbox"><input type="checkbox" checked={params.get('unassigned') === '1'} onChange={e => put('unassigned', e.target.checked ? '1' : '')}/>{t('accountUnassigned')}</label></div>{r.error ? <ErrorState code={r.error} retry={r.reload}/> : !r.data ? <Loading /> : <div className="admin-table-scroll"><table className="admin-table"><thead><tr><th>{t('username')}</th><th>{t('myBooth')}</th><th>{t('accountMode')}</th><th>{t('productCount')}</th><th>{t('actions')}</th></tr></thead><tbody>{r.data.items.map(row => <tr key={row.id}><td>{row.login_name || row.username}<small>#{row.id}</small></td><td>{row.vendor_id ? <Link to={'/admin/contents/' + row.vendor_id}>{row.vendor_name}</Link> : t('accountUnassigned')}</td><td>{[row.password_login ? t('passwordLogin') : '', row.phone_login ? t('phoneLogin') : ''].filter(Boolean).join(' / ') || '—'}</td><td>{row.product_count}</td><td>{row.vendor_id ? <Link className="text-link" to={'/admin/contents/' + row.vendor_id}>{t('view')}</Link> : <Link className="text-link" to={'/admin/vendors?new=1&account_id=' + row.id}>{t('bindVendor')}</Link>}<div className="admin-row-actions"><PasswordResetButton account={row} onChanged={r.reload}/></div></td></tr>)}{!r.data.items.length && <EmptyRows columns={5}/>}</tbody></table></div>}{r.data && <Pager page={r.data.page} total={r.data.total} size={r.data.page_size} onPage={paging}/>}</div></>; }
function OperationTable({ rows }: {
    rows: Operation[];
}) { const { t } = useLanguage(); return <div className="admin-table-scroll"><table className="admin-table"><thead><tr><th>{t('action')}</th><th>{t('actor')}</th><th>{t('target')}</th><th>{t('note')}</th><th>{t('reviewTime')}</th></tr></thead><tbody>{rows.map(row => <tr key={row.id}><td>{t(row.action)}</td><td>{row.actor_name || '#' + row.actor_id}</td><td>{['vendor', 'product'].includes(row.target_kind) ? <Link to={'/admin/contents/' + row.target_id}>{t(objectLabel(row.target_kind))} #{row.target_id}</Link> : t(objectLabel(row.target_kind)) + ' #' + row.target_id}</td><td className="admin-log-note">{row.note || '—'}</td><td className="admin-time">{timeText(row.created_at)}</td></tr>)}{!rows.length && <EmptyRows columns={5}/>}</tbody></table></div>; }
function Logs() { const { t } = useLanguage(); const { params, setParams, put, paging } = useFilters(); const r = useLoad<Collection<Operation>>('/admin/operation-logs?' + params, 'admin'); const dates = (value: string) => value ? new Date(value).toISOString() : ''; const local = (value: string | null) => { const date = value ? new Date(value) : null; return date && !Number.isNaN(date.getTime()) ? new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16) : ''; }; return <div className="admin-panel"><div className="admin-filters"><label>{t('action')}<select value={params.get('action') || ''} onChange={e => put('action', e.target.value)}><option value="">{t('allActions')}</option>{['content_created', 'draft_saved', 'account_created', 'account_linked', 'media_uploaded', 'submit', 'approve', 'reject', 'admin_offline', 'offline', 'featured_changed', 'category_saved', 'region_saved', 'account_login_configured', 'home_display_saved', 'home_display_image_uploaded','showroom_representatives_saved'].map(s => <option key={s} value={s}>{t(s)}</option>)}</select></label><label>{t('target')}<select value={params.get('target_kind') || ''} onChange={e => put('target_kind', e.target.value)}><option value="">{t('allTargets')}</option>{['vendor', 'product', 'account', 'media', 'category', 'region', 'home_display'].map(s => <option key={s} value={s}>{t(objectLabel(s))}</option>)}</select></label><label>{t('actor')}<input type="number" min="1" value={params.get('actor_id') || ''} onChange={e => put('actor_id', e.target.value)}/></label><label>{t('from')}<input type="datetime-local" value={local(params.get('from'))} onChange={e => put('from', dates(e.target.value))}/></label><label>{t('to')}<input type="datetime-local" value={local(params.get('to'))} onChange={e => put('to', dates(e.target.value))}/></label><button className="text-link" onClick={() => setParams({})}>{t('resetFilter')}</button></div>{r.error ? <ErrorState code={r.error} retry={r.reload}/> : !r.data ? <Loading /> : <><OperationTable rows={r.data.items}/><Pager page={r.data.page} total={r.data.total} size={r.data.page_size} onPage={paging}/></>}</div>; }
function SystemStatus() { const { t } = useLanguage(); const r = useLoad<AdminSystem>('/admin/system', 'admin'); if (r.error)
    return <ErrorState code={r.error} retry={r.reload}/>; if (!r.data)
    return <Loading />; const data = r.data; return <><Notice>{t('systemHelp')}</Notice><div className="admin-system-grid">{[['service', data.service === 'ok'], ['database', data.database_ok], ['mediaStorage', data.media_ok], ['smsAdapter', data.sms_provider === 'development' || data.aliyun_configured]].map(([label, ok]) => <section className="admin-panel" key={String(label)}><Activity size={22}/><h2>{t(String(label))}</h2><b className={ok ? 'admin-public' : 'danger'}>{t(ok ? 'healthy' : 'unavailable')}</b>{label === 'smsAdapter' && <p>{data.sms_provider} · {t(data.aliyun_configured ? 'configured' : 'notConfigured')}</p>}{label === 'mediaStorage' && <p>{t('files')}: {data.media_files} · {(data.media_bytes / 1024 / 1024).toFixed(2)} MB</p>}</section>)}</div><section className="admin-panel"><dl className="admin-system-info"><dt>{t('environment')}</dt><dd>{t(data.environment)}</dd><dt>{t('productCapacity')}</dt><dd>{data.product_limit}</dd><dt>{t('checkedAt')}</dt><dd>{timeText(data.checked_at)}</dd></dl></section></>; }
function Extension() { const { module } = useParams(); const { t } = useLanguage(); if (!groups.at(-1)!.items.some(i => i.key === module))
    return <ErrorState code="not_found"/>; return <section className="admin-panel admin-extension-page"><Settings size={40}/><span className="badge">{t('planned')}</span><h2>{t(module!)}</h2><Notice>{t('extensionHelp')}</Notice><h3>{t('purpose')}</h3><p>{t(module + 'Purpose')}</p><h3>{t('conditions')}</h3><p>{t(module + 'Condition')}</p></section>; }
