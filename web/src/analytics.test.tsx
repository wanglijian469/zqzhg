// @vitest-environment jsdom
import {afterEach,expect,it,vi} from 'vitest';
import {cleanup,fireEvent,render,screen} from '@testing-library/react';
import {BrowserRouter,Link} from 'react-router-dom';
import {flushAnalytics,recordPublicResponse,useAnalyticsNavigation} from './analytics';
afterEach(()=>{cleanup();vi.unstubAllGlobals();localStorage.clear()});
it('counts SPA navigation and contact clicks once, excludes management and bounds reporting failures',async()=>{
 localStorage.clear();history.replaceState(null,'','/products/4');const batches:Record<string,unknown>[][]=[];let fail=false;
 const report=vi.fn(async(_url:string,options:RequestInit)=>{batches.push(JSON.parse(options.body as string).events);if(fail)throw Error('offline');return new Response('{}',{status:202})});vi.stubGlobal('fetch',report);
 function Screen(){useAnalyticsNavigation();return <><a href="tel:000">Phone</a><button data-analytics-contact="wechat">Wechat</button><Link to="/browse/products?q=机械">Search</Link><Link to="/browse/products?q=机械&page=2">Next</Link><Link to="/admin/dashboard">Admin</Link></>}
 render(<BrowserRouter><Screen/></BrowserRouter>);
 recordPublicResponse('/public/products/4',{item:{id:4}});recordPublicResponse('/public/products/4',{item:{id:4}});
 fireEvent.click(screen.getByText('Wechat'));await flushAnalytics();
 expect(batches[0].map(v=>v.event_name)).toEqual(['page_view','product_view','contact_wechat']);
 const visitor=batches[0][0].visitor_id;expect(JSON.stringify(batches[0])).not.toContain('tel:000');
 fireEvent.click(screen.getByText('Search'));await flushAnalytics();expect(batches[1].map(v=>v.event_name)).toEqual(['page_view','search']);expect(batches[1][0].visitor_id).toBe(visitor);expect(batches[1][1].keyword).toBe('机械');
 fireEvent.click(screen.getByText('Next'));fail=true;await flushAnalytics();await flushAnalytics();await flushAnalytics();const attempts=report.mock.calls.length;await flushAnalytics();expect(report).toHaveBeenCalledTimes(attempts);expect(batches.at(-1)?.map(v=>v.event_name)).toEqual(['page_view']);
 fireEvent.click(screen.getByText('Admin'));fireEvent.click(screen.getByText('Wechat'));await flushAnalytics();expect(report).toHaveBeenCalledTimes(attempts);
});it('retains the viewed showroom on a query-only tab change and rejects stale responses',async()=>{
 localStorage.clear();history.replaceState(null,'','/v/fixture');const batches:Record<string,unknown>[][]=[];
 vi.stubGlobal('fetch',vi.fn(async(_url:string,options:RequestInit)=>{batches.push(JSON.parse(options.body as string).events);return new Response('{}',{status:202})}));
 function Screen(){useAnalyticsNavigation();return <><Link to="/v/fixture?tab=contact">Contact tab</Link><button data-analytics-contact="wechat">Copy</button></>}
 render(<BrowserRouter><Screen/></BrowserRouter>);recordPublicResponse('/public/showrooms/fixture',{item:{id:9}});fireEvent.click(screen.getByText('Contact tab'));recordPublicResponse('/public/showrooms/stale',{item:{id:10}});fireEvent.click(screen.getByText('Copy'));await flushAnalytics();const contact=batches.flat().find(v=>v.event_name==='contact_wechat');expect(contact?.target_id).toBe(9);expect(batches.flat().some(v=>v.target_id===10)).toBe(false);
});
