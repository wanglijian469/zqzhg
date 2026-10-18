export type Lang = 'zh' | 'en';
export interface Hall {hero_media_id?:number;version?:number;id:number;slug:string;name:string;name_en:string;icon:string;status:'open'|'preparing';theme:string;description?:string;description_en?:string}
export interface MerchantAuthResponse {token:string;account:{id:number;login_name?:string;role:string}}
export interface MerchantDashboard {products:{total:number;public:number;draft:number;pending:number;rejected:number}}
export interface CompanyContact {area?:string;area_en?:string;name?:string;name_en?:string;phone?:string;wechat?:string}
export interface GalleryAnnotation {media_id:number;title?:string;title_en?:string;description?:string;description_en?:string}
export interface CompanyCredential extends GalleryAnnotation {kind:"qualification"|"award"}
export interface CompanyProfile {credentials?:CompanyCredential[];gallery_annotations?:GalleryAnnotation[];short_name?:string;short_name_en?:string;summary?:string;summary_en?:string;logo_media_id?:number;cover_media_id?:number;established?:string;staff?:string;factory_area?:string;equipment?:string;equipment_en?:string;capacity?:string;capacity_en?:string;website?:string;contact_name?:string;contact_name_en?:string;contacts?:CompanyContact[]}
export interface ShowroomDisplay {vendor_id:number;mode:"auto"|"manual";product_ids:number[];version:number;limit:number;selected:{product_id:number;public_visible:boolean;item?:Item}[];preview_products:Item[]}
export interface ShowroomResponse {business_category?:{id:number;name:string;name_en?:string}|null;representative_products?:Item[];home_products?:Item[];resource_counts?:{video:number;catalog:number};item:Item;categories?:Taxonomy["categories"];product_count?:number;canonical_url:string;products?:Page}
export interface Data {region_code?:string;company_profile?:CompanyProfile;name: string; name_en: string; main_products: string; keywords: string; region_id: number; region_name: string; category_id: number; description: string; description_en: string; phone: string; wechat: string; address: string; parameters: string; media_ids: number[] | null}
export interface VendorSummary {showroom_slug?:string;id:number; name:string; name_en:string; region_id:number; region_name:string}
export interface Item {hall_id?:number;showroom_slug?:string;id: number; kind: 'vendor' | 'product'; vendor_id: number; owner_id: number; data: Data; status: string; revision_id: number; review_note: string; visible: boolean; public_visible: boolean; featured: boolean; sort_order: number; matching_products?: Item[]; vendor?:VendorSummary; published?:Item; product_count?:number; updated_at?:string}
export interface AdminAccount {id:number;username:string;login_name:string|null;vendor_id:number;vendor_name:string;product_count:number;password_login:boolean;phone_login:boolean;created_at:string}
export interface PasswordResetResult {account_id:number;login_name:string;password:string;revoked_sessions:number}
export interface Operation {id:number;actor_id:number;actor_name?:string;target_kind:string;target_id:number;action:string;note:string;created_at:string}
export interface AdminDashboard {counts:Record<string,number>;recent_operations:Operation[];account:{id:number;username:string;role?:string;permissions?:string[]}}
export interface AdminSystem {public_protection?:{enabled:boolean;active_ip_count:number;interceptions:Record<string,number>};service:string;environment:string;database_ok:boolean;media_ok:boolean;media_files:number;media_bytes:number;sms_provider:string;aliyun_configured:boolean;checked_at:string;product_limit:number}
export interface Collection<T> {items:T[];total:number;page:number;page_size:number}
export interface Taxonomy {aliases?:{source_id:number;target_id:number}[];halls: Hall[]; categories: {id: number;suggestion_status?:string;owner_id?:number;reference_count?:number;vendor_name?:string; product_count?:number;parent_id: number; hall_id: number; name: string; name_en: string; enabled: boolean; sort_order: number}[]; regions: {administrative_code?:string;id: number; name: string; name_en: string; enabled: boolean}[]}
export interface Page {items: Item[]; total: number; page: number; page_size: number}
export const emptyData: Data = {name:'', name_en:'', main_products:'', keywords:'', region_id:0, region_name:'', category_id:0, description:'', description_en:'', phone:'', wechat:'', address:'', parameters:'', media_ids:[]};

export interface HomeDisplayConfig {hero_media_id:number;version:number;vendor_limit:number;selected_vendor_count:number;updated_at?:string}
export interface HomeResponse {hall?:Hall;hero?:{media_id:number};vendors:Item[];products:Item[];vendor_count:number}
