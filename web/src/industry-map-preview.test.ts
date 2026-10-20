import {afterEach,beforeEach,expect,it,vi} from 'vitest';
const boundary={region_code:'13',usage:'local-learning-only',catalog_version:'2023-06-30',sha256:'a'.repeat(64),geometry:{type:'FeatureCollection',features:[{type:'Feature',properties:{adcode:'1305',mapped:true},geometry:{type:'Polygon',coordinates:[]}}]},sources:[],missing_codes:[]};
let fetcher:ReturnType<typeof vi.fn>;
beforeEach(()=>{vi.resetModules();fetcher=vi.fn(async()=>new Response(JSON.stringify(boundary)));vi.stubGlobal('fetch',fetcher)});
afterEach(()=>vi.unstubAllGlobals());
it('requires an explicit allowed local-preview capability, treating production and disabled endpoints as off',async()=>{
 const {mapPreviewAvailable}=await import('./industry-map-preview');fetcher.mockResolvedValueOnce(new Response('{}',{status:404}));expect(await mapPreviewAvailable(new AbortController().signal)).toBe(false);fetcher.mockResolvedValueOnce(new Response(JSON.stringify({enabled:true,usage:'production-approved'})));expect(await mapPreviewAvailable(new AbortController().signal)).toBe(false);fetcher.mockResolvedValueOnce(new Response(JSON.stringify({enabled:true,usage:'local-learning-only'})));expect(await mapPreviewAvailable(new AbortController().signal)).toBe(true);
});
it('caches matching regional data, supports explicit refresh and rejects wrong-region or wrong-usage data',async()=>{
 const {loadMapPreview}=await import('./industry-map-preview');const signal=new AbortController().signal;await loadMapPreview('13',signal);await loadMapPreview('13',signal);expect(fetcher).toHaveBeenCalledTimes(1);await loadMapPreview('13',signal,true);expect(fetcher).toHaveBeenCalledTimes(2);await expect(loadMapPreview('42',signal)).rejects.toThrow('invalid_map_preview');fetcher.mockResolvedValueOnce(new Response(JSON.stringify({...boundary,usage:'approved'})));await expect(loadMapPreview('13',signal,true)).rejects.toThrow('invalid_map_preview');
});
it('does not retain failed loads and respects cancellation even for cached resources',async()=>{
 const {loadMapPreview}=await import('./industry-map-preview');const controller=new AbortController();fetcher.mockResolvedValueOnce(new Response('{}',{status:502}));await expect(loadMapPreview('13',controller.signal)).rejects.toThrow('map_preview_unavailable');await loadMapPreview('13',controller.signal);controller.abort();await expect(loadMapPreview('13',controller.signal)).rejects.toMatchObject({name:'AbortError'});expect(fetcher).toHaveBeenCalledTimes(2);
});
