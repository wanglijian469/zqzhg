import fs from 'node:fs';
import crypto from 'node:crypto';
import assert from 'node:assert/strict';
const manifest=JSON.parse(fs.readFileSync('delivery/manifest.json','utf8'));
assert.equal(manifest.androidVersion,'1.5.12+20');assert.equal(manifest.websiteVersion,'1.5.11');assert.equal(manifest.backendVersion,'1.5.11');
for(const entry of manifest.files){const file=`delivery/${entry.file}`;assert.equal(fs.statSync(file).size,entry.bytes);assert.equal(crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex'),entry.sha256,file);}
const expected=manifest.androidRelease.sha256;
for(const file of ['delivery/android/dalunongji.apk','delivery/android/dalunongji-1.5.12+20.apk','web/public/downloads/dalunongji.apk','web/dist/downloads/dalunongji.apk','delivery/web/dist/downloads/dalunongji.apk'])assert.equal(crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex'),expected,file);
const audit=JSON.parse(fs.readFileSync('delivery/android-sync-package-audit.json','utf8'));assert.deepEqual(audit.forbiddenFilesIncluded,[]);
const results=await Promise.all(['http://127.0.0.1:8080','http://172.27.35.2:8080'].map(async base=>{
  const health=await fetch(`${base}/api/v1/health`,{signal:AbortSignal.timeout(15000)});assert.equal(health.status,200);
  const download=await fetch(`${base}/downloads/dalunongji.apk`,{signal:AbortSignal.timeout(60000)});assert.equal(download.status,200);
  const hash=crypto.createHash('sha256');let bytes=0;for await(const chunk of download.body){hash.update(chunk);bytes+=chunk.length;}assert.equal(hash.digest('hex'),expected);
  return {base,health:200,download:200,bytes,sha256:expected};
}));
fs.writeFileSync('docs/test-output/android-sync-delivery.json',JSON.stringify({versions:{android:manifest.androidVersion,website:manifest.websiteVersion,backend:manifest.backendVersion},filesChecked:manifest.files.length,sourceFiles:audit.files.length,forbiddenFilesIncluded:[],downloads:results},null,2));
console.log('PASS: manifest hashes, APK copies, source exclusions and local/LAN downloads.');
