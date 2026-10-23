// @vitest-environment jsdom
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen,waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {useState} from 'react';
import {WatermarkSettings} from './watermark-settings';
import type {ImageWatermarkConfig} from './runtime-config';
vi.mock('./main',()=>({useLanguage:()=>({lang:'zh'})}));
vi.mock('./api',()=>({token:()=> 'preview-token',APIError:class extends Error{},api:vi.fn(async()=>({items:[{id:9,name:'测试农机制造有限公司',short_name:''}]}))}));
const initial:ImageWatermarkConfig={enabled:true,text:'中企展馆',opacity:15,width_percent:22,position:'bottom_right'};
const preview=vi.fn(async(_url:string,_options?:RequestInit)=>new Response(new Blob(['png']),{status:200,headers:{'Content-Type':'image/png'}}));
function Form(){const [value,setValue]=useState(initial);return <WatermarkSettings value={value} onChange={setValue} disabled={false}/>}
beforeEach(()=>{preview.mockClear();vi.stubGlobal('fetch',preview);URL.createObjectURL=vi.fn(()=> 'blob:watermark-preview');URL.revokeObjectURL=vi.fn()});
afterEach(()=>{cleanup();vi.unstubAllGlobals()});
it('previews both backgrounds, resolves manufacturer fallback and keeps changes local',async()=>{render(<Form/>);await screen.findByRole('option',{name:'测试农机制造有限公司'});await userEvent.selectOptions(screen.getByLabelText('预览厂商'),'9');await screen.findByText(/未填写简称，暂用企业名称/);await waitFor(()=>expect(preview.mock.calls.length).toBeGreaterThanOrEqual(2));expect(screen.getAllByRole('img')).toHaveLength(2);expect(preview.mock.calls.every(c=>String(c[0]).endsWith('/watermark/preview'))).toBe(true);const body=JSON.parse((preview.mock.calls.at(-1) as unknown as [string,RequestInit])[1].body as string);expect(body.vendor_id).toBe(9);expect(body.watermark.opacity).toBe(15);await userEvent.selectOptions(screen.getByLabelText('水印位置'),'top_left');await waitFor(()=>expect(JSON.parse((preview.mock.calls.at(-1) as unknown as [string,RequestInit])[1].body as string).watermark.position).toBe('top_left'));expect(screen.getByText(/预览不会保存设置/)).toBeTruthy()});
it('shows the service-upgrade requirement when the old response omits watermark',()=>{render(<WatermarkSettings value={undefined} onChange={vi.fn()} disabled={false}/>);expect(screen.getByText(/配套服务升级/)).toBeTruthy();expect(preview).not.toHaveBeenCalled()});
it('rejects out-of-range styles before requesting previews',async()=>{render(<Form/>);await userEvent.clear(screen.getByLabelText(/^不透明度/));await userEvent.type(screen.getByLabelText(/^不透明度/),'99');expect((await screen.findByRole('alert')).textContent).toContain('请检查水印文字');await new Promise(r=>setTimeout(r,450));expect(preview).not.toHaveBeenCalled()});
