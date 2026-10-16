import fs from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
const web=fileURLToPath(new URL('../web/',import.meta.url));
const source=path.join(web,'node_modules/pdfjs-dist');
const target=path.join(web,'public/pdfjs');
fs.mkdirSync(target,{recursive:true});
for(const folder of ['cmaps','standard_fonts','wasm'])fs.cpSync(path.join(source,folder),path.join(target,folder),{recursive:true});
fs.copyFileSync(path.join(source,'LICENSE'),path.join(target,'LICENSE'));
