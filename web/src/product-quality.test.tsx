import {describe,it,expect} from 'vitest';
import {emptyData} from './types';
import {productQuality} from './product-quality';

describe('advisory product quality',()=>{
 it('reports missing fields without mutating supplied data or cover order',()=>{
  const data={...emptyData,name:'农机配件',media_ids:[9,3],model:'A,B'};
  const before=JSON.stringify(data);const result=productQuality(data);
  expect(result.warnings).toHaveLength(2);expect(result.percent).toBe(50);
  expect(JSON.stringify(data)).toBe(before);expect(result.checks.find(c=>c.key==='category')?.ok).toBe(false);
 });
 it('recognizes an existing parameter model and complete information',()=>{
  const data={...emptyData,name:'旋耕机齿轮',category_id:2,media_ids:[1],description:'适用旋耕机传动',parameters:'型号：CL-20\n材质：钢'};
  expect(productQuality(data).percent).toBe(100);expect(productQuality(data).warnings).toHaveLength(0);
 });
 it('does not count an unfilled parameter template as model or specification data',()=>{
  const result=productQuality({...emptyData,parameters:'型号：\n适配动力：\n重量：'});
  expect(result.checks.find(c=>c.key==='model')?.ok).toBe(false);
  expect(result.checks.find(c=>c.key==='parameters')?.ok).toBe(false);
 });
});
