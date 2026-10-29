import type {AdministrativeRegion} from './company-region-field';
export interface RegionCount extends AdministrativeRegion {vendor_count:number}
export interface RegionStatistics {
 region:AdministrativeRegion|null;trail:AdministrativeRegion[];items:RegionCount[];
 total:number;unassigned_count:number;version:string;historical:boolean;coverage:string;
}
export interface BoundaryResource {
 url:string;sha256:string;version:string;source:string;license:string;review_number:string;
 review_evidence:string;publication_scope:string;verified_by:string;verified_at:string;
 presentation:'industry-choropleth';approved:true;catalog_version:string;
 coordinate_system?:'WGS84'|'GCJ-02'|'CGCS2000';platforms?:('web'|'android')[];cache_allowed?:boolean;
 code_field?:'adcode'|'code';code_map?:Record<string,string>;
}
export interface BoundaryManifest {version:1;catalog_version:string;resources:Record<string,BoundaryResource>}
export interface BoundaryGeometry {type:'FeatureCollection';features:{type:'Feature';properties:{adcode:string;code?:string;name?:string};geometry:unknown}[]}
export interface PreviewBoundary {
 region_code:string;usage:'local-learning-only'|'development-demonstration';catalog_version:string;sha256:string;
 geometry:{type:'FeatureCollection';features:{type:'Feature';properties:{adcode:string;name?:string;mapped:boolean;decoration?:boolean;cp?:number[];source_adcode?:string};geometry:unknown}[]};
 sources:{url:string;sha256:string;acquired_at:string;usage:string}[];
 missing_codes:string[];unmapped_count:number;
}

export type OutlineBoundary=Pick<PreviewBoundary,'region_code'|'sha256'|'geometry'|'missing_codes'>;
