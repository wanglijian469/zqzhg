// @vitest-environment jsdom
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {act,cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import type {RegionStatistics} from './industry-map-types';
const mock=vi.hoisted(()=>({render:vi.fn(),t:(key:string)=>key}));
vi.mock('./main',()=>({useLanguage:()=>({lang:'zh',t:mock.t})}));
vi.mock('./industry-map-schematic',()=>({renderSchematicMap:mock.render}));
import IndustryMapChart from './industry-map-chart';
const instances:Array<{dispose:ReturnType<typeof vi.fn>;resize:ReturnType<typeof vi.fn>;zoom:ReturnType<typeof vi.fn>;reset:ReturnType<typeof vi.fn>}>=[];
const observers:Array<{callback:()=>void;disconnect:ReturnType<typeof vi.fn>}>=[];
const statistics:RegionStatistics={region:null,trail:[],items:[{code:'13',name:'河北省',full_name:'河北省',parent_code:'',path:['河北省'],level:1,has_children:true,vendor_count:2}],total:2,unassigned_count:0,version:'2023-06-30',historical:true,coverage:'mainland-31'};
beforeEach(()=>{
 vi.stubGlobal('fetch',vi.fn(async()=>new Response('{}',{status:404})));
 instances.length=0;observers.length=0;mock.render.mockReset();mock.render.mockImplementation(()=>{const instance={dispose:vi.fn(),resize:vi.fn(),zoom:vi.fn(),reset:vi.fn()};instances.push(instance);return instance});
 vi.stubGlobal('ResizeObserver',class{disconnect=vi.fn();constructor(callback:()=>void){observers.push({callback,disconnect:this.disconnect})}observe(){}});
});
afterEach(()=>{cleanup();vi.unstubAllGlobals()});

it('disposes and disconnects the previous diagram on region changes and unmount, and wires controls',async()=>{
 const select=vi.fn();const view=render(<IndustryMapChart statistics={statistics} onSelect={select}/>);await screen.findByRole('button',{name:'imResetMap'});
 fireEvent.click(screen.getByRole('button',{name:'imZoomIn'}));expect(instances[0].zoom).toHaveBeenCalledWith(1.3);fireEvent.click(screen.getByRole('button',{name:'imResetMap'}));expect(instances[0].reset).toHaveBeenCalledTimes(1);
 const leaf={...statistics,region:{...statistics.items[0],has_children:false},items:[],total:2};view.rerender(<IndustryMapChart statistics={leaf} onSelect={select}/>);await waitFor(()=>expect(mock.render).toHaveBeenCalledTimes(2));
 expect(instances[0].dispose).toHaveBeenCalledTimes(1);expect(observers[0].disconnect).toHaveBeenCalledTimes(1);expect(screen.getByText('imLeaf')).toBeTruthy();view.unmount();expect(instances[1].dispose).toHaveBeenCalledTimes(1);expect(observers[1].disconnect).toHaveBeenCalledTimes(1);
});
it('recovers from a resize rendering failure by disposing and creating a fresh chart',async()=>{
 render(<IndustryMapChart statistics={statistics} onSelect={vi.fn()}/>);await screen.findByRole('button',{name:'imResetMap'});instances[0].resize.mockImplementation(()=>{throw Error('resize failed')});
 await act(async()=>observers[0].callback());await screen.findByText('imMapFailed');expect(instances[0].dispose).toHaveBeenCalledTimes(1);expect(observers[0].disconnect).toHaveBeenCalledTimes(1);
 fireEvent.click(screen.getByRole('button',{name:'retry'}));await screen.findByRole('button',{name:'imResetMap'});expect(mock.render).toHaveBeenCalledTimes(2);expect(instances[0].dispose).toHaveBeenCalledTimes(1);
});
