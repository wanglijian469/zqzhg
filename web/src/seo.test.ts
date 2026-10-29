import {expect,it} from 'vitest';
import {canonicalPublicURL,filteredSearch,showroomSearchVariant} from './seo';
import {companySEO,productSEO,publicPageSEO,seoKeywords} from './seo';
import {emptyData,type Hall} from './types';

const hall:Hall={id:1,slug:'agriculture',name:'农业机械厅',name_en:'Agricultural Machinery',icon:'tractor',status:'open',theme:'green'};
it('targets search phrases only on platform landing pages, without declaring brand aliases',()=>{
 const home=publicPageSEO('/','中企展馆',hall,'zh')!;
 expect(home.description).toContain('内卷网');
 expect(home.keywords).toContain('河北内卷网,内卷网,不卷网');
 expect(home.title).toBe('中企展馆-农业机械厅');
 expect(publicPageSEO('/browse','中企展馆',hall,'zh')?.title).toContain('企业名单与厂家展厅');
 expect(publicPageSEO('/browse/products','中企展馆',hall,'zh')?.keywords).not.toContain('内卷网');
 expect(publicPageSEO('/merchant','中企展馆',hall,'zh')).toBeUndefined();
 expect(publicPageSEO('/','中企展馆',{...hall,status:'preparing'},'zh')).toBeUndefined();
});
it('builds company and product metadata from actual names, with English fallback and bounded keywords',()=>{
 const vendor={...emptyData,name:'河北冀农农机具有限公司',main_products:'耕地机械',region_name:'河北省',keywords:'耕地机械，农机',company_profile:{short_name:'冀农',summary:'真实企业简介'}};
 const zh=companySEO(vendor,'zh','about','企业介绍','中企展馆');
 expect(zh.title).toBe(vendor.name+' · 企业展厅 · 企业介绍 | 中企展馆');
 expect(zh.description).toBe(vendor.name+'企业展厅。真实企业简介');
 expect(zh.keywords).toContain('冀农');expect(zh.keywords).not.toContain('内卷网');
 expect(companySEO({...vendor,name_en:'Jinong',company_profile:{short_name_en:'JN',summary_en:'Company profile'}},'en','home','Home','Exhibition').title).toBe('Jinong · Company showroom | Exhibition');
 const p=productSEO({...emptyData,name:'旋耕机',parameters:'型号：NJ-800\n功率：80千瓦'},vendor,'zh','中企展馆',hall.name);
 expect(p.keywords).toContain('NJ-800');expect(p.description).toContain('型号 NJ-800');
 expect(seoKeywords(' 企业名单，企业展厅;企业名单|Tractor\ntractor',' 河北  农机 ', '长'.repeat(81))).toBe('企业名单,企业展厅,Tractor,河北 农机');
 expect(seoKeywords(...Array.from({length:30},(_,i)=>'词'+i)).split(',')).toHaveLength(24);
});

it('consolidates showroom tabs, language, filters and pagination at the company address',()=>{
 for(const query of ['?tab=about','?tab=contact&lang=en','?tab=products&q=机械&category_id=2&page=3','?tab=home&utm_source=share']){
  expect(canonicalPublicURL('/v/brand',query,'https://example.com')).toBe('https://example.com/v/brand');
  expect(canonicalPublicURL('/vendors/21',query,'https://example.com','https://example.com/v/brand')).toBe('https://example.com/v/brand');
 }
 expect(canonicalPublicURL('/products/7','?hall=cable&page=3#spec','https://example.com')).toBe('https://example.com/products/7');
});

it('excludes showroom result variants without excluding its normal tabs or other list pagination',()=>{
 expect(showroomSearchVariant('/v/brand','?tab=products&page=2')).toBe(true);
 expect(showroomSearchVariant('/vendors/21','?page=02')).toBe(true);
 expect(showroomSearchVariant('/v/brand','?tab=about&page=1')).toBe(false);
 expect(showroomSearchVariant('/v/brand','?tab=contact')).toBe(false);
 expect(showroomSearchVariant('/browse','?page=2')).toBe(false);
 expect(filteredSearch('?q=机械')).toBe(true);
});
