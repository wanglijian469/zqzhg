import {expect,it} from 'vitest';
import {canonicalPublicURL,filteredSearch,showroomSearchVariant} from './seo';

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
