import {beforeEach,expect,it,vi} from 'vitest';
import type {PreviewBoundary,RegionStatistics} from './industry-map-types';
const chart=vi.hoisted(()=>({setOption:vi.fn(),on:vi.fn(),dispose:vi.fn(),resize:vi.fn(),dispatchAction:vi.fn()}));
const register=vi.hoisted(()=>vi.fn());
vi.mock('echarts/core',()=>({init:()=>chart,registerMap:register,use:vi.fn()}));
vi.mock('echarts/charts',()=>({MapChart:{}}));vi.mock('echarts/components',()=>({TooltipComponent:{}}));vi.mock('echarts/renderers',()=>({SVGRenderer:{}}));vi.mock('echarts/features',()=>({LabelLayout:{}}));
import {mergedPreviewGeometry,renderOutlineMap} from './industry-map-outline';
const feature=(code:string,x=0,mapped=true)=>({type:'Feature' as const,properties:{adcode:code,name:code==='13'?'河北省':'上下文地区',mapped,cp:[x+.5,.5]},geometry:{type:'Polygon',coordinates:[[[x,0],[x+1,0],[x+1,1],[x,1],[x,0]]]}});
const resource:PreviewBoundary={region_code:'',usage:'local-learning-only',catalog_version:'2023-06-30',sha256:'a'.repeat(64),geometry:{type:'FeatureCollection',features:[feature('13'),feature('context:810000',2,false)]},sources:[],missing_codes:[],unmapped_count:1};
const statistics:RegionStatistics={region:null,trail:[],items:[{code:'13',name:'河北省',full_name:'河北省',parent_code:'',level:1,path:['河北省'],has_children:true,vendor_count:0}],total:0,unassigned_count:0,version:'2023-06-30',historical:true,coverage:'mainland-31'};
beforeEach(()=>vi.clearAllMocks());
it('dissolves adjacent county pieces into one intermediate region without an internal border',()=>{
 const grouped={...resource,geometry:{...resource.geometry,features:[feature('5001',0),feature('5001',1)]}};
 const merged=mergedPreviewGeometry(grouped);expect(merged.features).toHaveLength(1);expect(merged.features[0].properties.name).toBe('5001');expect(merged.features[0].properties).not.toHaveProperty('cp');
 const geometry=merged.features[0].geometry as {type:string;coordinates:number[][][][]};expect(geometry.type).toBe('MultiPolygon');expect(geometry.coordinates).toHaveLength(1);expect(geometry.coordinates[0][0]).toHaveLength(5);
});
it('binds real counts by exact code, preserves zero and keeps outside-directory features uncounted and unclickable',()=>{
 const navigate=vi.fn();renderOutlineMap({clientWidth:800,clientHeight:500} as HTMLDivElement,resource,statistics,navigate,key=>key);
 const option=chart.setOption.mock.calls[0][0],series=option.series[0];expect(series.type).toBe('map');expect(series.label.formatter({name:'13'})).toBe('河北\n0');expect(series.label.formatter({name:'context:810000'})).toBe('');expect(series.data[0].value).toBe(0);expect(series.data[1]).not.toHaveProperty('value');expect(option.tooltip.formatter({name:'context:810000'})).toContain('imOutsideCoverage');
 const click=chart.on.mock.calls[0][1];click({name:'context:810000'});expect(navigate).not.toHaveBeenCalled();click({name:'13'});expect(navigate).toHaveBeenCalledWith('13');expect(JSON.parse(register.mock.calls[0][1]).features).toHaveLength(2);
 expect(series.projection.project([116,40])[1]).toBeLessThan(series.projection.project([116,20])[1]);const point=series.projection.unproject(series.projection.project([116,40]));expect(point[0]).toBeCloseTo(116);expect(point[1]).toBeCloseTo(40);
});
it('updates filtering without replacing the map, resetting zoom or re-registering geometry',()=>{
 const element={clientWidth:800,clientHeight:500};const controls=renderOutlineMap(element as HTMLDivElement,resource,statistics,vi.fn(),key=>key);
 controls.zoom(1.3);controls.update({...statistics,total:8,items:statistics.items.map(item=>({...item,vendor_count:8}))});const [option,settings]=chart.setOption.mock.calls[1];expect(option.series[0]).not.toHaveProperty('zoom');expect(option.series[0]).not.toHaveProperty('center');expect(settings.notMerge).toBe(false);expect(option.series[0].data[0].value).toBe(8);expect(register).toHaveBeenCalledTimes(1);
 controls.resize();expect(chart.resize).not.toHaveBeenCalled();element.clientWidth=500;controls.resize();expect(chart.resize).toHaveBeenCalledTimes(1);controls.reset();expect(chart.setOption.mock.calls[2][0].series[0].zoom).toBe(1);controls.dispose();expect(chart.dispose).toHaveBeenCalledTimes(1);
});
it('renders a leaf outline and total without manufacturing a child or allowing leaf drilldown',()=>{
 const leaf={...statistics.items[0],code:'130528',name:'宁晋县',has_children:false};const source={...resource,region_code:'130528',geometry:{...resource.geometry,features:[feature('130528')]}};const navigate=vi.fn();renderOutlineMap({clientWidth:320,clientHeight:400} as HTMLDivElement,source,{...statistics,region:leaf,items:[],total:8},navigate,key=>key);expect(chart.setOption.mock.calls[0][0].series[0].data[0].value).toBe(8);chart.on.mock.calls[0][1]({name:'130528'});expect(navigate).not.toHaveBeenCalled();
});
it('disambiguates same-name municipal groups with their exact directory codes',()=>{
 const items=['5001','5002'].map(code=>({...statistics.items[0],code,name:'重庆市',vendor_count:0}));const source={...resource,region_code:'50',geometry:{...resource.geometry,features:items.map((item,index)=>feature(item.code,index))}};
 renderOutlineMap({clientWidth:800,clientHeight:500} as HTMLDivElement,source,{...statistics,region:{...items[0],code:'50'},items},vi.fn(),key=>key);const option=chart.setOption.mock.calls[0][0];expect(option.series[0].label.formatter({name:'5001'})).toContain('5001');expect(option.tooltip.formatter({name:'5002'})).toContain('5002');
});
