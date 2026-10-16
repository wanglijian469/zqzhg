import fs from 'node:fs';
const root=new URL('../',import.meta.url);
const rules=JSON.parse(fs.readFileSync(new URL('server/form_rules.json',root),'utf8'));
fs.writeFileSync(new URL('web/src/form-rules.ts',root),'// Generated from server/form_rules.json.\nexport const formRules:Record<string,{max:number;unit:string;zh:string;en:string;sample_zh:string;sample_en:string;required:string[]}> = '+JSON.stringify(rules,null,2)+';\n');
fs.writeFileSync(new URL('mobile/lib/form_rules.dart',root),"// Generated from server/form_rules.json.\npart of 'main.dart';\nconst Map<String,dynamic> formRules = "+JSON.stringify(rules,null,2)+';\n');
