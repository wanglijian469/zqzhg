// @vitest-environment jsdom
import {afterEach,expect,it,vi} from 'vitest';
import {cleanup,fireEvent,render,waitFor} from '@testing-library/react';
import {Link,MemoryRouter,useLocation} from 'react-router-dom';
import {HallProvider,usePageMetadata} from './halls';
import {companySEO} from './seo';
import {emptyData} from './types';

afterEach(()=>{cleanup();vi.unstubAllGlobals();document.head.innerHTML=''});
function Pages(){
 const {pathname}=useLocation(),privatePage=pathname==='/procurement';
 const metadata=companySEO({...emptyData,name:pathname==='/v/a'?'厂家甲':'厂家乙'},'zh','home','','中企展馆');
 usePageMetadata(privatePage?'':metadata.title,metadata.description,undefined,false,0,metadata.keywords);
 return <><Link to="/v/b">厂家乙</Link><Link to="/procurement">采购</Link></>;
}
it('deduplicates keywords, replaces the previous company, and clears public terms on private routes',async()=>{
 vi.stubGlobal('fetch',vi.fn().mockResolvedValue({ok:true,json:async()=>({items:[{id:1,slug:'agriculture',status:'open',name:'农业机械厅'}]})}));
 document.head.innerHTML='<meta name="keywords" content="stale"><meta name="keywords" content="duplicate">';
 const page=render(<MemoryRouter initialEntries={['/v/a']}><HallProvider><Pages/></HallProvider></MemoryRouter>);
 await waitFor(()=>expect(document.querySelectorAll('meta[name="keywords"]')).toHaveLength(1));
 expect(document.querySelector<HTMLMetaElement>('meta[name="keywords"]')?.content).toContain('厂家甲');
 fireEvent.click(page.getByText('厂家乙'));
 await waitFor(()=>expect(document.title).toContain('厂家乙'));
 expect(document.querySelector<HTMLMetaElement>('meta[name="keywords"]')?.content).not.toContain('厂家甲');
 fireEvent.click(page.getByText('采购'));
 await waitFor(()=>expect(document.querySelector('meta[name="keywords"]')).toBeNull());
});
