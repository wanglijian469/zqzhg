// @vitest-environment jsdom
import {afterEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen} from '@testing-library/react';
import {ProductSpecifications} from './product-specifications';
import {cleanProductText,legacyParametersVisible} from './product-presentation';
import type {ProductDetails} from './types';
vi.mock('./ui',()=>({useLanguage:()=>({lang:'zh'}),MediaImage:()=>null,Modal:({children}:{children:React.ReactNode})=><div>{children}</div>}));
afterEach(cleanup);
const details:ProductDetails={series_name:'系列A',features:['结构特点'],applications:['耕地'],source_pages:[2],models:[{model_name:'X-01',variant:'双轴',parameters:[{parameter_name:'压力',raw_value:'1.2300e2',raw_unit:'MPa',source_page:2,review_required:true,normalized_unit:null,normalized_value:null}]}],images:[{media_id:3,image_type:'dimensions',source_page:2,caption:'安装尺寸（PDF第2页）'}]};
it('shows standard product facts publicly and preserves evidence in private review',()=>{const r=render(<ProductSpecifications details={details}/>);expect(screen.getByText('1.2300e2')).toBeTruthy();expect(screen.getByText('MPa')).toBeTruthy();expect(screen.queryByText('画册来源')).toBeNull();expect(screen.queryByText(/PDF/)).toBeNull();expect(screen.getByText('安装尺寸')).toBeTruthy();r.rerender(<ProductSpecifications details={details} role="merchant"/>);expect(screen.getByText('画册来源')).toBeTruthy();expect(screen.getByText('PDF 2')).toBeTruthy();expect(details.images[0].caption).toBe('安装尺寸（PDF第2页）')});
it('only hides truly duplicate compatibility text and keeps unmatched manual additions',()=>{const text='型号：X-01 双轴\n压力：1.2300e2 MPa（PDF第2页）';expect(legacyParametersVisible(text,details)).toBe(false);expect(legacyParametersVisible(text+'\n安装环境：室内',details)).toBe(true);expect(cleanProductText('PDF设备\n**画册来源**：来源页\n压力：10~12 MPa（PDF第2页）')).toBe('PDF设备\n\n压力：10~12 MPa')});
