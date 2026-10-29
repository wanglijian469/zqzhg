// @vitest-environment jsdom
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen,within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {BrowserRouter} from 'react-router-dom';
import {ResourceList} from './resources';
import {withHallFixture} from './hall-test-fixture';

let kind:'video'|'catalog'='video',untitled=false;
beforeEach(()=>{untitled=false;localStorage.clear();sessionStorage.clear();vi.stubGlobal('fetch',withHallFixture(async()=>new Response(JSON.stringify({items:[{id:1,kind,vendor_id:3,status:'approved',revision_id:1,public_visible:true,data:{title:untitled?'':'示例资料',title_en:untitled?'':'Sample media',format:'images',purpose:'company',file_media_id:42,page_media_ids:kind==='catalog'?[43]:[],description:''}}],total:1,page:1,page_size:12}))))});
afterEach(()=>{cleanup();vi.unstubAllGlobals()});
it.each(['video','catalog'] as const)('names the %s dialog from its visible heading and restores keyboard focus',async value=>{
 kind=value;render(<BrowserRouter><ResourceList kind={kind}/></BrowserRouter>);
 const opener=await screen.findByRole('button',{name:/示例资料/});await userEvent.click(opener);
 const dialog=screen.getByRole('dialog',{name:'示例资料'}),heading=within(dialog).getByRole('heading',{name:'示例资料'});
 expect(dialog.getAttribute('aria-labelledby')).toBe(heading.id);expect(heading.id).not.toBe('');
 const close=within(dialog).getByRole('button',{name:'close'});expect(document.activeElement).toBe(close);
 await userEvent.keyboard('{Escape}');expect(screen.queryByRole('dialog')).toBeNull();expect(document.activeElement).toBe(opener);
});

it.each(['video','catalog'] as const)('keeps an untitled %s preview dialog named',async value=>{
 kind=value;untitled=true;render(<BrowserRouter><ResourceList kind={kind}/></BrowserRouter>);await screen.findByRole('button',{name:kind==='video'?/companyVideo/:/pageNumber/});await userEvent.click(document.querySelector<HTMLButtonElement>('.resource-card')!);
 expect(screen.getByRole('dialog',{name:kind==='video'?'shortVideos':'electronicCatalogs'})).toBeTruthy();
});
