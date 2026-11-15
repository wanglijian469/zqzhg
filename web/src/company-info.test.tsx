// @vitest-environment jsdom
import {afterEach,expect,it} from 'vitest';
import {cleanup,render,screen,within} from '@testing-library/react';
import {Language} from './ui';
import {TaxonomySummary} from './taxonomy-summary';
import {CompanyProduction} from './company-portal';
import {emptyData,type Item,type Data,type Taxonomy} from './types';
const categories=[{id:1,parent_id:0,name:'液压系统',name_en:'Hydraulics'},{id:2,parent_id:1,name:'液压泵',name_en:'Hydraulic pumps'},{id:3,parent_id:1,name:'液压阀',name_en:''}] as Taxonomy['categories'];
const item=(data:Partial<Data>={})=>({id:1,kind:'vendor',data:{...emptyData,...data}} as Item);
const labels:Record<string,string>={portalProduction:'生产能力',factory_area:'厂房面积',equipment:'设备情况',capacity:'供货能力'};
function shell(node:React.ReactNode,lang:'zh'|'en'='zh'){return render(<Language.Provider value={{lang,t:key=>labels[key]||key,toggle:()=>{}}}>{node}</Language.Provider>)}
afterEach(cleanup);
it('separates leaf names and parent paths for each secondary category inside one business panel',()=>{
 shell(<TaxonomySummary variant="panel" item={item({primary_category_id:1,secondary_category_ids:[2,3],business_nature:'manufacturer',industrial_belt_ids:[8]})} categories={categories} belts={[{id:8,name:'宁晋产业带',name_en:'Ningjin'} as any]}/>);
 const panel=screen.getByRole('heading',{name:'经营信息'}).closest('section')!;
 expect(panel.querySelectorAll('.taxonomy-attribute-row')).toHaveLength(4);
 expect([...panel.querySelectorAll('.taxonomy-category-name')].map(n=>n.textContent)).toEqual(['液压系统','液压泵','液压阀']);
 expect([...panel.querySelectorAll('.taxonomy-category-parent')].map(n=>n.textContent)).toEqual(['液压系统','液压系统']);
 expect(within(panel).getByText('生产制造')).toBeTruthy();expect(within(panel).getByText('宁晋产业带')).toBeTruthy();
});
it('falls back to Chinese category names in English without inventing missing parent paths',()=>{
 shell(<TaxonomySummary variant="panel" item={item({primary_category_id:3,secondary_category_ids:[999],business_nature:'dealer'})} categories={categories} belts={[]}/>, 'en');
 expect(screen.getByRole('heading',{name:'Business information'})).toBeTruthy();expect(screen.getByText('液压阀')).toBeTruthy();expect(screen.getByText('Hydraulics')).toBeTruthy();expect(screen.getByText('#999')).toBeTruthy();expect(screen.getByText('Dealer / agent')).toBeTruthy();
 expect(document.querySelectorAll('.taxonomy-category-parent')).toHaveLength(1);
});
it('keeps long category names complete and hides an entirely empty business panel',()=>{
 const long='适用于大型工程机械和农业机械的液压分配器及配套零部件';
 const view=shell(<TaxonomySummary variant="panel" item={item({primary_category_id:4})} categories={[...categories,{id:4,parent_id:1,name:long,name_en:''} as any]} belts={[]}/>);
 expect(screen.getByText(long)).toBeTruthy();
 view.rerender(<Language.Provider value={{lang:'zh',t:k=>k,toggle:()=>{}}}><TaxonomySummary variant="panel" item={item()} categories={[]} belts={[]}/></Language.Provider>);
 expect(view.container.textContent).toBe('');
});
it('keeps compact product summaries without the showroom business panel',()=>{
 shell(<TaxonomySummary item={{...item({product_category_id:2,model:'LF-440'}),kind:'product'}} categories={categories} belts={[]}/>);
 expect(screen.getByText('液压系统 / 液压泵')).toBeTruthy();expect(screen.getByText('LF-440')).toBeTruthy();expect(document.querySelector('.company-business-info')).toBeNull();
});
it('keeps raw production values and line breaks while hiding empty optional facts',()=>{
 shell(<CompanyProduction data={{...emptyData,company_profile:{factory_area:'1000',equipment:'   ',capacity:'按订单生产\n可定制'}}}/>);
 const facts=document.querySelector('.portal-production')!;expect(facts.children).toHaveLength(2);
 expect(facts.querySelector('dd')?.textContent).toBe('1000');expect(facts.querySelectorAll('dd')[1].textContent).toBe('按订单生产\n可定制');expect(screen.queryByText('设备情况')).toBeNull();
});
it('uses English production text when provided and preserves Chinese fallback',()=>{
 shell(<CompanyProduction data={{...emptyData,company_profile:{equipment:'数控设备',equipment_en:'CNC machines',capacity:'按订单生产'}}}/>, 'en');
 expect(screen.getByText('CNC machines')).toBeTruthy();expect(screen.getByText('按订单生产')).toBeTruthy();expect(screen.queryByText('数控设备')).toBeNull();
});
it('does not render an empty production section',()=>{
 const view=shell(<CompanyProduction data={{...emptyData,company_profile:{factory_area:' ',equipment:'',capacity:'\n'}}}/>);expect(view.container.textContent).toBe('');
});
