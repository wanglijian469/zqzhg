import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {spawnSync} from 'node:child_process';
const root=process.cwd();
const androidVersion=fs.readFileSync('mobile/pubspec.yaml','utf8').match(/^version:\s*(\S+)/m)[1];
const version=androidVersion.split('+')[0],websiteVersion=JSON.parse(fs.readFileSync('web/package.json','utf8')).version;
const backendVersion=JSON.parse(fs.readFileSync('docs/openapi.json','utf8')).info.version;
const apk=`delivery/android/dalunongji-${androidVersion}.apk`;
const hash=file=>crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex'),apkHash=hash(apk);
for(const file of ['delivery/android/dalunongji.apk','web/public/downloads/dalunongji.apk','web/dist/downloads/dalunongji.apk','delivery/web/dist/downloads/dalunongji.apk'])if(hash(file)!==apkHash)throw Error(`APK differs: ${file}`);
const signingCertificate=fs.readFileSync('docs/test-output/android-sync-apk-signature.txt','utf8').match(/Signer #1 certificate SHA-256 digest: ([a-f0-9]{64})/)[1];
if(signingCertificate!=='85656c85541b188c832dc48d67cbf320046a4311aa9e0fefcc662dbe2e0f1585')throw Error('Original signing certificate changed');
const excludedDirs=new Set(['node_modules','pdfjs','dist','build','.dart_tool','.gradle','.idea','ephemeral','.symlinks','Pods','.plugin_symlinks','data','test-output']);
const excludedFiles=new Set(['.env','key.properties','local.properties','.flutter-plugins-dependencies','Generated.xcconfig','flutter_export_environment.sh','GeneratedPluginRegistrant.java']);
const audit=[];
function copy(src,dst){if(fs.statSync(src).isDirectory()){if(excludedDirs.has(path.basename(src)))return;fs.mkdirSync(dst,{recursive:true});for(const name of fs.readdirSync(src))copy(path.join(src,name),path.join(dst,name));}else{if(excludedFiles.has(path.basename(src))||/\.(exe|apk|jks|keystore|log|iml|tsbuildinfo)$/.test(src))return;fs.copyFileSync(src,dst);audit.push(path.relative(root,src).replaceAll('\\','/'));}}
const sourceStage=path.join(root,`.local/android-source-${version}`),deployStage=path.join(root,`.local/android-deploy-${version}`);
const sourceZip=`delivery/zqzt-exhibition-v${version}-source.zip`,deployZip=`delivery/dalunongji-android-v${version}-deploy.zip`;
for(const file of [sourceZip,deployZip])if(fs.existsSync(file))throw Error(`Archive already exists: ${file}`);
fs.mkdirSync(sourceStage,{recursive:true});
for(const name of ['server','web','mobile','docs','scripts','需求文档','README.md','.gitignore'])copy(path.join(root,name),path.join(sourceStage,name));
fs.mkdirSync(deployStage,{recursive:true});fs.copyFileSync(apk,path.join(deployStage,`dalunongji-${androidVersion}.apk`));fs.copyFileSync('docs/ANDROID_SYNC.md',path.join(deployStage,'README.md'));
for(const folder of ['screenshots','verification'])fs.mkdirSync(path.join(deployStage,folder),{recursive:true});
for(const name of fs.readdirSync('docs/screenshots').filter(n=>/^android-sync-.*\.png$/.test(n)))fs.copyFileSync(`docs/screenshots/${name}`,path.join(deployStage,'screenshots',name));
for(const name of ['android-sync-analyze.txt','android-sync-test.txt','android-sync-apk-metadata.txt','android-sync-apk-signature.txt','android-sync-devices.txt','android-sync-api-regression.txt'])fs.copyFileSync(`docs/test-output/${name}`,path.join(deployStage,'verification',name));
const release={androidVersion,websiteVersion,backendVersion,applicationId:'com.zqzt.dalunongji',apiBaseUrl:'http://172.27.35.2:8080',signingCertificate,sha256:apkHash,createdAt:new Date().toISOString(),deviceValidation:{connectedDevices:0,configuredEmulators:0,cleanInstall:'not_tested',inPlaceUpgrade:'not_tested',camera:'not_tested',nativeSharing:'not_tested',nativeVideoPdfPlayback:'not_tested'}};
fs.writeFileSync(path.join(deployStage,'manifest.json'),JSON.stringify(release,null,2));fs.writeFileSync(path.join(deployStage,'SHA256SUMS.txt'),`${apkHash}  dalunongji-${androidVersion}.apk\n`);
for(const [stage,file] of [[sourceStage,sourceZip],[deployStage,deployZip]]){const r=spawnSync('pwsh',['-NoProfile','-Command','Add-Type -AssemblyName System.IO.Compression.FileSystem; [IO.Compression.ZipFile]::CreateFromDirectory($env:ZQZT_ANDROID_STAGE,$env:ZQZT_ANDROID_ZIP)'],{env:{...process.env,ZQZT_ANDROID_STAGE:stage,ZQZT_ANDROID_ZIP:path.join(root,file)},encoding:'utf8'});if(r.status!==0)throw Error(r.stderr||'ZIP creation failed');}
const manifest=JSON.parse(fs.readFileSync('delivery/manifest.json','utf8'));Object.assign(manifest,{version,androidVersion,websiteVersion,backendVersion,createdAt:release.createdAt,androidRelease:release});
for(const file of ['delivery/android/dalunongji.apk',apk,sourceZip,deployZip]){const entry={file:file.replace(/^delivery\//,''),bytes:fs.statSync(file).size,sha256:hash(file)};manifest.files=manifest.files.filter(r=>r.file!==entry.file);manifest.files.push(entry);}
fs.writeFileSync('delivery/manifest.json',JSON.stringify(manifest,null,2));
fs.writeFileSync('delivery/android-sync-package-audit.json',JSON.stringify({androidVersion,files:audit,forbiddenFilesIncluded:audit.filter(f=>/(^|\/)(\.env|key\.properties|local\.properties|signing\.json)$|\.(jks|keystore)$/.test(f))},null,2));
console.log(JSON.stringify({androidVersion,websiteVersion,backendVersion,apk,apkHash,sourceZip,deployZip,sourceFiles:audit.length},null,2));
