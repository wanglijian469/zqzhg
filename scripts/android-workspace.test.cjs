const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const code=fs.readFileSync('mobile/android/app/src/main/assets/native-workspace.js','utf8');
function fixture(){
  const store=()=>{const data=new Map();return {getItem:k=>data.get(k)||null,setItem:(k,v)=>data.set(k,String(v)),removeItem:k=>data.delete(k)}};
  const prompts=[],alerts=[];
  class Anchor{click(){this.navigated=true}}
  const context={sessionStorage:store(),localStorage:store(),URL:{createObjectURL:()=> 'blob:private-file',revokeObjectURL:()=>{}},HTMLAnchorElement:Anchor,document:{addEventListener:()=>{}},window:{open:()=>{}},FileReader:class{readAsDataURL(){this.result='data:application/pdf;base64,cHJpdmF0ZQ==';this.onload()}},prompt:(nonce,raw)=>prompts.push({nonce,body:JSON.parse(raw)}),alert:s=>alerts.push(s)};
  vm.createContext(context);
  const seed={nonce:'fixture-nonce',token:'synthetic-session',account:{id:7,role:'visitor'},language:'zh'};
  return {context,prompts,alerts,run:(value=seed)=>vm.runInContext(code.replace('__DLNJ_BOOTSTRAP__',JSON.stringify(value)),context)};
}
test('native session is initialized without URL credentials; reload does not resurrect logout',()=>{
  const f=fixture();f.context.sessionStorage.setItem('zqzt.admin','old-admin');f.run();
  assert.equal(f.context.sessionStorage.getItem('zqzt.merchant'),'synthetic-session');assert.equal(f.context.sessionStorage.getItem('zqzt.admin'),null);
  f.context.sessionStorage.removeItem('zqzt.merchant');f.run();assert.equal(f.context.sessionStorage.getItem('zqzt.merchant'),null);
});
test('private blob download goes through bounded native transport, not network navigation',()=>{
  const f=fixture();f.run();const url=f.context.URL.createObjectURL({size:7,type:'application/pdf'});const a=new f.context.HTMLAnchorElement();a.href=url;a.download='spec.pdf';a.click();
  assert.equal(a.navigated,undefined);assert.equal(f.prompts.length,1);assert.equal(f.prompts[0].body.name,'spec.pdf');assert(!JSON.stringify(f.prompts).includes('synthetic-session'));
});
test('oversized blobs and revoked object URLs cannot enter native save',()=>{
  const f=fixture();f.run();const a=new f.context.HTMLAnchorElement();a.href=f.context.URL.createObjectURL({size:21*1024*1024,type:'application/pdf'});a.click();assert.equal(f.prompts.length,0);
  a.href=f.context.URL.createObjectURL({size:1,type:'image/png'});f.context.URL.revokeObjectURL(a.href);a.click();assert.equal(f.prompts.length,0);assert.equal(f.alerts.length,2);
});
test('preview preserves intent so PDF and image links can open the native viewer',()=>{
  const f=fixture();f.run();const a=new f.context.HTMLAnchorElement();a.href=f.context.URL.createObjectURL({size:1,type:'image/png'});a.target='_blank';a.download='';a.click();
  assert.equal(f.prompts[0].body.preview,true);assert.equal(f.prompts[0].body.mime,'image/png');
});
