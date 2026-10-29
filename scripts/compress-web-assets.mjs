import {readdir,readFile,writeFile} from 'node:fs/promises';
import path from 'node:path';
import {gzipSync} from 'node:zlib';

export async function compressWebAssets(directory){
 let count=0;
 for(const entry of await readdir(directory,{withFileTypes:true})){
  const file=path.join(directory,entry.name);
  if(entry.isDirectory()){count+=await compressWebAssets(file);continue}
  if(!/\.(js|css|svg)$/.test(entry.name))continue;
  const bytes=await readFile(file);if(bytes.length<1024)continue;
  const zipped=gzipSync(bytes,{level:9});if(zipped.length>=bytes.length)continue;
  await writeFile(file+'.gz',zipped);count++;
 }
 return count;
}
