import {runtimeConfig} from './runtime-config';
export const mapDemoActive=()=>runtimeConfig().capabilities?.map_demo===true;
import type {PreviewBoundary} from './industry-map-types';
const resources=new Map<string,PreviewBoundary>();
export async function mapPreviewAvailable(signal:AbortSignal){
 const demo=mapDemoActive();const response=await fetch(demo?'/api/v1/public/map-demo/config':'/api/v1/dev/map-preview/config',{signal,cache:'no-store'});
 if(!response.ok){if(demo)throw Error('map_demo_unavailable');return false;}
 const config=await response.json();return config.enabled===true&&config.usage===(demo?'development-demonstration':'local-learning-only');
}
export async function loadMapPreview(code:string,signal:AbortSignal,fresh=false):Promise<PreviewBoundary>{
 if(signal.aborted)throw new DOMException('Aborted','AbortError');
 const key=(mapDemoActive()?'demo:':'local:')+code;if(!fresh&&resources.has(key))return resources.get(key)!;
 const response=await fetch((mapDemoActive()?'/api/v1/public/map-demo/regions?':'/api/v1/dev/map-preview/regions?')+new URLSearchParams(code?{region_code:code}:{}),{signal,cache:'no-store'});
 if(!response.ok)throw Error('map_preview_unavailable');
 const value=await response.json() as PreviewBoundary;
 if(value.usage!==(mapDemoActive()?'development-demonstration':'local-learning-only')||value.region_code!==code||value.catalog_version!=='2023-06-30'||!/^[a-f0-9]{64}$/.test(value.sha256)||value.geometry?.type!=='FeatureCollection'||!value.geometry.features.length||!Array.isArray(value.missing_codes))throw Error('invalid_map_preview');
 if(signal.aborted)throw new DOMException('Aborted','AbortError');
 resources.set(key,value);if(resources.size>20)resources.delete(resources.keys().next().value!);return value;
}
