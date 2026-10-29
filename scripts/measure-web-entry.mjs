import fs from 'node:fs';
import path from 'node:path';
import {gzipSync} from 'node:zlib';
import {fileURLToPath} from 'node:url';

// Includes every static import of the entry, rather than comparing one chunk
// while silently moving its bytes to another eagerly loaded file.
export function measureEntry(directory){
 const manifest=JSON.parse(fs.readFileSync(path.join(directory,'.vite/manifest.json'),'utf8'));
 const entry=Object.keys(manifest).find(key=>manifest[key].isEntry);
 if(!entry)throw new Error('Vite entry missing');
 const visited=new Set();const files=[];
 const walk=key=>{if(visited.has(key))return;visited.add(key);const item=manifest[key];if(!item)throw new Error('Missing manifest dependency: '+key);if(item.file.endsWith('.js'))files.push(item.file);for(const dependency of item.imports||[])walk(dependency)};
 walk(entry);
 return {directory,staticJS:files.map(file=>{const raw=fs.readFileSync(path.join(directory,file));return {file,bytes:raw.length,gzip:gzipSync(raw).length}}),bytes:files.reduce((sum,f)=>sum+fs.statSync(path.join(directory,f)).size,0),gzip:files.reduce((sum,f)=>sum+gzipSync(fs.readFileSync(path.join(directory,f))).length,0),dynamicEntries:Object.values(manifest).filter(x=>x.isDynamicEntry).map(x=>x.file)};
}
if(process.argv[1]&&path.resolve(process.argv[1])===path.resolve(fileURLToPath(import.meta.url)))console.log(JSON.stringify(process.argv.slice(2).map(measureEntry),null,2));
