import {beforeEach,expect,it,vi} from 'vitest';
import type {RegionCount,RegionStatistics} from './industry-map-types';
const chart=vi.hoisted(()=>({setOption:vi.fn(),on:vi.fn(),dispose:vi.fn(),resize:vi.fn(),dispatchAction:vi.fn()}));
vi.mock('echarts/core',()=>({init:()=>chart,use:vi.fn()}));
vi.mock('echarts/charts',()=>({GraphChart:{}}));vi.mock('echarts/components',()=>({TooltipComponent:{}}));vi.mock('echarts/renderers',()=>({SVGRenderer:{}}));
import {renderSchematicMap,schematicColor,schematicLabel,schematicLayout} from './industry-map-schematic';
const region=(code:string,count=0,has_children=true):RegionCount=>({code,name:'同名区域',full_name:'同名区域',parent_code:'',path:['同名区域'],level:1,has_children,vendor_count:count});
const stats=(items:RegionCount[]):RegionStatistics=>({region:null,trail:[],items,total:items.reduce((n,item)=>n+item.vendor_count,0),unassigned_count:0,version:'2023-06-30',historical:true,coverage:'mainland-31'});
const codes=['11','12','13','14','15','21','22','23','31','32','33','34','35','36','37','41','42','43','44','45','46','50','51','52','53','54','61','62','63','64','65'];
beforeEach(()=>vi.clearAllMocks());

it('lays out all 31 provincial nodes without overlapping or clipping on desktop, mobile and landscape',()=>{
 for(const [width,height] of [[1000,620],[680,300],[368,540],[338,350],[520,170]]){
  const layout=schematicLayout(codes.map(code=>region(code)),width,height,true);expect(layout.nodes).toHaveLength(31);
  expect(layout.spatial).toBe(width>=680&&height>=300);
  for(const node of layout.nodes){expect(node.x-layout.tileW/2).toBeGreaterThanOrEqual(0);expect(node.y-layout.tileH/2).toBeGreaterThanOrEqual(0);expect(node.x+layout.tileW/2).toBeLessThanOrEqual(width);expect(node.y+layout.tileH/2).toBeLessThanOrEqual(height)}
  for(let i=0;i<31;i++)for(let j=i+1;j<31;j++){const a=layout.nodes[i],b=layout.nodes[j];expect(Math.abs(a.x-b.x)>=layout.tileW||Math.abs(a.y-b.y)>=layout.tileH).toBe(true)}
 }
});
it('sorts mobile and child grids by exact codes and displays no invented children',()=>{
 const nodes=[region('429004',1,false),region('4201',0)];const layout=schematicLayout(nodes,340,400,false);
 expect(layout.nodes.map(node=>node.code)).toEqual(['4201','429004']);expect(layout.nodes[1].has_children).toBe(false);
});
it('binds nodes by code, preserves zero values and only navigates through actual child nodes',()=>{
 const items=[region('13',0),region('42',3)];const navigate=vi.fn();renderSchematicMap({clientWidth:800,clientHeight:500} as HTMLDivElement,stats(items),navigate,key=>key);
 const option=chart.setOption.mock.calls[0][0],series=option.series[0];expect(series.type).toBe('graph');expect(series.data.map((node:{id:string;value:number})=>[node.id,node.value])).toEqual([['13',0],['42',3]]);
 expect(series.label.formatter({data:series.data[0]})).toContain('{count|0}');expect(option.tooltip.formatter({data:series.data[1]})).toBe('同名区域\n3 imCompanies');
 const click=chart.on.mock.calls[0][1];click({name:'81'});expect(navigate).not.toHaveBeenCalled();click({name:'42'});expect(navigate).toHaveBeenCalledWith('42');
 expect(series.data[0].itemStyle.color).not.toBe(series.data[1].itemStyle.color);
});
it('shows a leaf total without navigating to a synthetic child and supports zoom, reset, resize and disposal',()=>{
 const leaf=region('429004',4,false),statistics={...stats([]),region:leaf,total:4};const navigate=vi.fn();const element={clientWidth:340,clientHeight:400};const controls=renderSchematicMap(element as HTMLDivElement,statistics,navigate,key=>key);
 expect(chart.setOption.mock.calls[0][0].series[0].data.map((node:{id:string;value:number})=>[node.id,node.value])).toEqual([['429004',4]]);chart.on.mock.calls[0][1]({name:'429004'});expect(navigate).not.toHaveBeenCalled();
 controls.zoom(1.3);expect(chart.dispatchAction).toHaveBeenCalledWith({type:'graphRoam',seriesIndex:0,zoom:1.3,originX:170,originY:200});controls.resize();expect(chart.resize).not.toHaveBeenCalled();controls.reset();expect(chart.setOption).toHaveBeenCalledTimes(2);element.clientWidth=400;controls.resize();expect(chart.resize).toHaveBeenCalledTimes(1);expect(chart.setOption).toHaveBeenCalledTimes(3);controls.dispose();element.clientWidth=500;controls.resize();expect(chart.dispose).toHaveBeenCalledTimes(1);expect(chart.resize).toHaveBeenCalledTimes(1);
});
it('uses a distinct zero colour and a monotonic positive quantity scale',()=>{
 expect(schematicColor(0,0)).toBe('#edf3fc');const colors=[0,1,25,100].map(n=>schematicColor(n,100));expect(new Set(colors).size).toBe(4);
});
it('keeps long region labels within tiles while preserving full names for tooltips',()=>{
 expect(schematicLabel('鄂温克族自治旗',56,10)).toBe('鄂温克…');expect(schematicLabel('河北',56,10)).toBe('河北');
 const statistics=stats([region('15')]);statistics.items[0].name='内蒙古自治区';renderSchematicMap({clientWidth:338,clientHeight:350} as HTMLDivElement,statistics,vi.fn(),key=>key);const data=chart.setOption.mock.calls[0][0].series[0].data[0];expect(data.labelName).toBe('内蒙古');expect(data.regionName).toBe('内蒙古自治区');
});
it('disposes an instance when initial rendering fails',()=>{
 chart.setOption.mockImplementationOnce(()=>{throw Error('render failed')});expect(()=>renderSchematicMap({clientWidth:340,clientHeight:400} as HTMLDivElement,stats([region('13')]),vi.fn(),key=>key)).toThrow('render failed');expect(chart.dispose).toHaveBeenCalledTimes(1);
});
