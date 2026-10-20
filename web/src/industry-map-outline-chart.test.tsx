// @vitest-environment jsdom
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import type {RegionStatistics} from './industry-map-types';
const mocks=vi.hoisted(()=>({available:vi.fn(),load:vi.fn(),outline:vi.fn(),schematic:vi.fn(),approved:vi.fn(),t:(key:string)=>key}));
vi.mock('./main',()=>({useLanguage:()=>({lang:'zh',t:mocks.t})}));
vi.mock('./industry-map-resources',()=>({loadBoundary:mocks.approved}));
vi.mock('./industry-map-preview',()=>({mapPreviewAvailable:mocks.available,loadMapPreview:mocks.load}));
vi.mock('./industry-map-outline',()=>({renderOutlineMap:mocks.outline}));
vi.mock('./industry-map-schematic',()=>({renderSchematicMap:mocks.schematic}));
import IndustryMapChart from './industry-map-chart';
const statistics:RegionStatistics={region:null,trail:[],items:[{code:'13',name:'河北省',full_name:'河北省',parent_code:'',path:['河北省'],level:1,has_children:true,vendor_count:8}],total:8,unassigned_count:0,version:'2023-06-30',historical:true,coverage:'mainland-31'};
const controls=()=>({dispose:vi.fn(),resize:vi.fn(),update:vi.fn(),zoom:vi.fn(),reset:vi.fn()});
beforeEach(()=>{vi.clearAllMocks();mocks.approved.mockResolvedValue(null);mocks.available.mockResolvedValue(true);mocks.load.mockReset();mocks.load.mockResolvedValue({missing_codes:[]});mocks.outline.mockImplementation(controls);mocks.schematic.mockImplementation(controls)});
afterEach(cleanup);
it('updates filtering in place while preserving the outline instance and disposes on unmount',async()=>{
 const select=vi.fn(),view=render(<IndustryMapChart statistics={statistics} onSelect={select}/>);await screen.findByRole('button',{name:'imResetMap'});const instance=mocks.outline.mock.results[0].value;
 const filtered={...statistics,total:1,items:statistics.items.map(item=>({...item,vendor_count:1}))};view.rerender(<IndustryMapChart statistics={filtered} onSelect={select}/>);expect(mocks.outline).toHaveBeenCalledTimes(1);expect(instance.update).toHaveBeenCalledWith(filtered);expect(mocks.load).toHaveBeenCalledTimes(1);view.unmount();expect(instance.dispose).toHaveBeenCalledTimes(1);
});
it('cancels an old regional load and ignores its completion after navigating',async()=>{
 let release:(value:unknown)=>void=()=>{};let oldSignal:AbortSignal|undefined;mocks.load.mockImplementationOnce((_code:string,signal:AbortSignal)=>{oldSignal=signal;return new Promise(resolve=>{release=resolve})});
 const select=vi.fn(),view=render(<IndustryMapChart statistics={statistics} onSelect={select}/>);await waitFor(()=>expect(mocks.load).toHaveBeenCalledTimes(1));const province={...statistics,region:statistics.items[0]};view.rerender(<IndustryMapChart statistics={province} onSelect={select}/>);await screen.findByRole('button',{name:'imResetMap'});release({missing_codes:[]});await waitFor(()=>expect(mocks.outline).toHaveBeenCalledTimes(1));expect(oldSignal?.aborted).toBe(true);expect(mocks.outline.mock.calls[0][2]).toBe(province);
});
it('offers manual schematic fallback on resource failure and can return to outlines',async()=>{
 mocks.load.mockRejectedValueOnce(Error('resource failed'));render(<IndustryMapChart statistics={statistics} onSelect={vi.fn()}/>);await screen.findByText('imMapFailed');fireEvent.click(screen.getByRole('button',{name:'imUseSchematic'}));await screen.findByRole('button',{name:'imReturnOutline'});expect(mocks.schematic).toHaveBeenCalledTimes(1);fireEvent.click(screen.getByRole('button',{name:'imReturnOutline'}));await waitFor(()=>expect(mocks.outline).toHaveBeenCalledTimes(1));expect(mocks.load).toHaveBeenCalledTimes(2);
});
it('reports missing geometry and opens the existing region chooser without making a polygon',async()=>{
 mocks.load.mockResolvedValueOnce({missing_codes:['130571']});const missing=vi.fn();render(<IndustryMapChart statistics={statistics} onSelect={vi.fn()} onMissing={missing}/>);fireEvent.click(await screen.findByRole('button',{name:'imMissingOutlines 1 · imChooseRegion'}));expect(missing).toHaveBeenCalledTimes(1);
});

it('uses verified public geometry before local preview and labels its real source',async()=>{mocks.approved.mockResolvedValue({resource:{source:'SYNTHETIC UNIT FIXTURE',review_number:'TEST-ONLY',sha256:'a'.repeat(64)},geometry:{type:'FeatureCollection',features:[{type:'Feature',properties:{adcode:'13',name:'河北省'},geometry:{type:'Polygon',coordinates:[]}}]}});render(<IndustryMapChart statistics={statistics} onSelect={vi.fn()}/>);await screen.findByRole('button',{name:'imResetMap'});expect(mocks.available).not.toHaveBeenCalled();expect(mocks.outline).toHaveBeenCalledTimes(1);expect(screen.getByText('SYNTHETIC UNIT FIXTURE · TEST-ONLY')).toBeTruthy();expect(screen.queryByText('imPreviewNote')).toBeNull()});
