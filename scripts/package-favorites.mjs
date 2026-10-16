import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {spawnSync} from 'node:child_process';
import {fileURLToPath} from 'node:url';
import assert from 'node:assert/strict';

const root=fileURLToPath(new URL('../',import.meta.url));
process.chdir(root);
const version='1.5.16',android='1.5.16+24';
const certificate='85656c85541b188c832dc48d67cbf320046a4311aa9e0fefcc662dbe2e0f1585';
const hash=file=>crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const apk=`delivery/android/dalunongji-${android}.apk`,apkHash=hash(apk);
const apkCopies=[apk,'delivery/android/dalunongji.apk','web/public/downloads/dalunongji.apk','web/dist/downloads/dalunongji.apk','delivery/web/dist/downloads/dalunongji.apk'];
for(const file of apkCopies)assert.equal(hash(file),apkHash,file);
assert.ok(fs.readFileSync('docs/test-output/favorites-apk-signature.txt','utf8').includes(`Signer #1 certificate SHA-256 digest: ${certificate}`));
const apkMetadata=fs.readFileSync('docs/test-output/favorites-apk-metadata.txt','utf8');
assert.ok(apkMetadata.includes("name='com.zqzt.dalunongji' versionCode='24' versionName='1.5.16'"));
assert.ok(fs.readFileSync('docs/test-output/favorites-analyze-final.txt','utf8').includes('No issues found!'));
const flutterLog=fs.readFileSync('docs/test-output/favorites-flutter-final.txt','utf8');
assert.ok(flutterLog.includes('All tests passed!'));
const goLog=fs.readFileSync('docs/test-output/favorites-go-regression.txt','utf8');
assert.match(goLog,/(?:\r?\n)PASS(?:\r?\n)/);
const stage=path.join(root,`.local/favorites-backend-v${version}`);
const archive=`delivery/zqzt-favorites-v${version}-backend.zip`;
if(fs.existsSync(archive))throw Error('Versioned backend archive already exists; retain previous artifacts.');
function copy(source,target){const output=path.join(stage,target);fs.mkdirSync(path.dirname(output),{recursive:true});fs.copyFileSync(source,output);}
const binaries=[];
for(const [platform,arch] of [['windows','amd64'],['linux','amd64'],['linux','arm64']]){
  const name=platform==='windows'?'exhibition.exe':'exhibition';
  const file=`.local/favorites-bin/${platform}-${arch}/${name}`;
  const data=fs.readFileSync(file);
  if(platform==='linux'){
    assert.equal(data.subarray(0,4).toString('hex'),'7f454c46');
    assert.equal(data.readUInt16LE(18),arch==='amd64'?62:183);
  } else {assert.equal(data.subarray(0,2).toString(),'MZ');assert.equal(data.readUInt16LE(data.readUInt32LE(0x3c)+4),0x8664);}
  copy(file,`bin/${platform}-${arch}/${name}`);
  const output=`delivery/server/${version}/${platform}-${arch}/${name}`;
  fs.mkdirSync(path.dirname(output),{recursive:true});fs.copyFileSync(file,output);
  binaries.push({file:output,packagePath:`bin/${platform}-${arch}/${name}`,bytes:data.length,sha256:hash(file)});
}
copy('docs/FAVORITES.md','README.md');copy('docs/openapi.json','docs/openapi.json');
const packageReadme=path.join(stage,'README.md');
fs.writeFileSync(packageReadme,fs.readFileSync(packageReadme,'utf8').replaceAll('](test-output/','](verification/').replaceAll('](openapi.json)','](docs/openapi.json)').replaceAll('verification/favorites-go-regression.txt','verification/favorites-go-summary.txt'));
copy('server/.env.example','server.env.example');copy('server/migrations/009_favorites.sql','migrations/009_favorites.sql');
for(const name of ['favorites-analyze-final.txt','favorites-flutter-final.txt','favorites-final-ui.txt','favorites-go-vet.txt','favorites-apk-metadata.txt','favorites-apk-signature.txt','favorites-devices.txt']){
  copy(`docs/test-output/${name}`,`verification/${name}`);
}
const goSummary=goLog.split(/\r?\n/).filter(line=>/^(--- PASS:|--- SKIP:|PASS$|ok\s)/.test(line)).join('\n')+'\n';
fs.writeFileSync(path.join(stage,'verification/favorites-go-summary.txt'),goSummary);
fs.mkdirSync(path.join(stage,'screenshots'),{recursive:true});
const screenshots=fs.readdirSync('docs/screenshots').filter(name=>/^favorites-.*\.png$/.test(name));
for(const name of screenshots)copy(`docs/screenshots/${name}`,`screenshots/${name}`);
const release={version,androidVersion:android,backendVersion:version,websiteVersion:'1.5.13',applicationId:'com.zqzt.dalunongji',
  apiBaseUrl:'http://39.97.250.244:8089',signingCertificateSHA256:certificate,apkSHA256:apkHash,migration:9,
  createdAt:new Date().toISOString(),remoteDeployment:false,productionDatabaseMigrated:false,
  connectedAndroidDevices:0,deviceInstallation:'not_tested',flutterTestsPassed:Number(flutterLog.match(/\+(\d+)(?: ~\d+)?: All tests passed!/)[1]),
  goTestsPassed:(goLog.match(/^--- PASS:/gm)??[]).length,screenWidths:[360,390,768],textScale:1.4,languages:['zh','en'],screenshots:screenshots.length,
  binaries,apkCopies};
fs.writeFileSync(path.join(stage,'manifest.json'),JSON.stringify(release,null,2)+'\n');
function files(dir){return fs.readdirSync(dir,{withFileTypes:true}).flatMap(entry=>entry.isDirectory()?files(path.join(dir,entry.name)):[path.join(dir,entry.name)]);}
// A reusable staging folder may contain the preceding checksum list. The list
// must never hash itself, even when preparing a corrected unpublished package.
const staged=files(stage).filter(file=>path.basename(file)!=='SHA256SUMS.txt');for(const file of staged)assert.ok(!/(?:^|[/\\])(?:\.env|key\.properties|signing\.json)$|\.(?:jks|keystore)$/.test(file),file);
fs.writeFileSync(path.join(stage,'SHA256SUMS.txt'),staged.map(file=>`${hash(file)}  ${path.relative(stage,file).replaceAll('\\','/')}`).join('\n')+'\n');
const result=spawnSync('pwsh',['-NoProfile','-Command','Add-Type -AssemblyName System.IO.Compression.FileSystem; [IO.Compression.ZipFile]::CreateFromDirectory($env:ZQZT_FAVORITES_STAGE,$env:ZQZT_FAVORITES_ARCHIVE)'],{
  env:{...process.env,ZQZT_FAVORITES_STAGE:stage,ZQZT_FAVORITES_ARCHIVE:path.join(root,archive)},encoding:'utf8',windowsHide:true});
if(result.status!==0)throw Error(result.stderr||'Archive creation failed');
release.files=[apk,archive,...binaries.map(row=>row.file)].map(file=>({file,bytes:fs.statSync(file).size,sha256:hash(file)}));
fs.writeFileSync('delivery/manifest-favorites-1.5.16.json',JSON.stringify(release,null,2)+'\n');
const sums=release.files.map(row=>`${row.sha256}  ${row.file.replace(/^delivery\//,'')}`).join('\n')+'\n';
fs.writeFileSync('delivery/SHA256SUMS-favorites-1.5.16.txt',sums);
fs.writeFileSync(`delivery/android/SHA256SUMS-${android}.txt`,`${apkHash}  dalunongji-${android}.apk\n`);
fs.writeFileSync(`delivery/android/manifest-${android}.json`,JSON.stringify(release,null,2)+'\n');
fs.copyFileSync('docs/FAVORITES.md','delivery/FAVORITES-1.5.16.md');
const current=JSON.parse(fs.readFileSync('delivery/manifest.json','utf8'));
if(!fs.existsSync('.local/favorites-original-delivery-manifest.json'))fs.copyFileSync('delivery/manifest.json','.local/favorites-original-delivery-manifest.json');
Object.assign(current,{version,androidVersion:android,backendVersion:version,apkSHA256:apkHash,createdAt:release.createdAt,androidRelease:release,backendInstallPackage:archive});
for(const row of [...release.files,{file:'delivery/android/dalunongji.apk',bytes:fs.statSync(apk).size,sha256:apkHash}]){
  const entry={...row,file:row.file.replace(/^delivery\//,'')};current.files=current.files.filter(old=>old.file!==entry.file);current.files.push(entry);
}
fs.writeFileSync('delivery/manifest.json',JSON.stringify(current,null,2)+'\n');
fs.writeFileSync('docs/test-output/favorites-delivery.json',JSON.stringify({...release,packageFileCount:staged.length+1,forbiddenFilesIncluded:[]},null,2)+'\n');
console.log(JSON.stringify({androidVersion:android,backendArchive:archive,apkSHA256:apkHash,flutterTestsPassed:release.flutterTestsPassed,goTestsPassed:release.goTestsPassed,files:release.files},null,2));
