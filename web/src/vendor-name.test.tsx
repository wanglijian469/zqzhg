// @vitest-environment jsdom
import React,{useRef} from 'react';
import {afterEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen} from '@testing-library/react';
import {FormGuide,issueText} from './form-guide';
import {companyEditorMessages} from './company-editor-messages';
import {emptyData} from './types';
import {Language} from './ui';

afterEach(cleanup);
it('explains a company name conflict and keeps the entered name focused',()=>{
 Element.prototype.scrollIntoView=vi.fn();
 const data={...emptyData,name:'河北春耕机械制造有限公司'};
 function Fixture(){const form=useRef<HTMLFormElement>(null);return <form ref={form}><label data-field="name">企业名称<input defaultValue={data.name}/></label><FormGuide form={form} data={data} kind="vendor" issues={[{field:'name',code:'vendor_name_exists'}]}/></form>}
 render(<Language.Provider value={{lang:'zh',toggle:()=>{},t:key=>companyEditorMessages.zh[key]||key}}><Fixture/></Language.Provider>);
 const input=screen.getByRole('textbox',{name:'企业名称'}) as HTMLInputElement;
 expect(input.value).toBe(data.name);
 expect(input.getAttribute('aria-invalid')).toBe('true');
 expect(document.activeElement).toBe(input);
 expect(screen.getByRole('alert').textContent).toContain('企业名称已存在');
 expect(issueText({field:'name',code:'vendor_name_exists'},'en')).toContain('company name already exists');
 expect(companyEditorMessages.zh.vendor_name_exists).toContain('企业名称已存在');
 expect(companyEditorMessages.en.vendor_name_exists).toContain('company name already exists');
});
