import type {PreviewBoundary} from './industry-map-types';
const resources=new Map<string,PreviewBoundary>();
export async function mapPreviewAvailable(signal:AbortSignal){
 const response=await fetch('/api/v1/dev/map-preview/config',{signal,cache:'no-store'});
 if(!response.ok)return false;
 const config=await response.json();return config.enabled===true&&config.usage==='local-learning-only';
}
export async function loadMapPreview(code:string,signal:AbortSignal,fresh=false):Promise<PreviewBoundary>{
 if(signal.aborted)throw new DOMException('Aborted','AbortError');
 if(!fresh&&resources.has(code))return resources.get(code)!;
 const response=await fetch('/api/v1/dev/map-preview/regions?'+new URLSearchParams(code?{region_code:code}:{}),{signal,cache:'no-store'});
 if(!response.ok)throw Error('map_preview_unavailable');
 const value=await response.json() as PreviewBoundary;
 if(value.usage!=='local-learning-only'||value.region_code!==code||value.catalog_version!=='2023-06-30'||!/^[a-f0-9]{64}$/.test(value.sha256)||value.geometry?.type!=='FeatureCollection'||!value.geometry.features.length||!Array.isArray(value.missing_codes))throw Error('invalid_map_preview');
 if(signal.aborted)throw new DOMException('Aborted','AbortError');
 resources.set(code,value);if(resources.size>20)resources.delete(resources.keys().next().value!);return value;
}
