// Explicitly publish only the reviewed mobile acceptance release. Never replace stable assets.
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {spawnSync,execFileSync} from 'node:child_process';

const root=path.resolve(import.meta.dirname,'..');process.chdir(root);
const repository='wanglijian469/zqzhg',tag='v1.8.0-rc.1',version='1.8.0-rc.1+30';
const release='mobile-acceptance-1.8.0-rc.1-30-20261020';
const directory=path.join(root,'delivery/github',tag);
const planPath=path.join(root,'.local/mobile18-github-plan.json');
const receiptPath=path.join(root,'.local/mobile18-github-receipt.json');
const readJSON=file=>JSON.parse(fs.readFileSync(file,'utf8').replace(/^\uFEFF/,''));
const git=(...args)=>execFileSync('git',args,{encoding:'utf8',env:{...process.env,GIT_TERMINAL_PROMPT:'0'}}).trim();
const writeJSON=(file,data)=>fs.writeFileSync(file,JSON.stringify(data,null,2)+'\n');
async function sha(file){const hash=crypto.createHash('sha256');for await(const chunk of fs.createReadStream(file))hash.update(chunk);return hash.digest('hex')}
function credential(){
 if(process.env.GH_TOKEN||process.env.GITHUB_TOKEN)return process.env.GH_TOKEN||process.env.GITHUB_TOKEN;
 const result=spawnSync('git',['credential','fill'],{input:'protocol=https\nhost=github.com\n\n',encoding:'utf8',env:{...process.env,GIT_TERMINAL_PROMPT:'0',GCM_INTERACTIVE:'never'}});
 if(result.status!==0)throw Error('GitHub authentication unavailable');
 const secret=result.stdout.split(/\r?\n/).find(line=>line.startsWith('password='));
 if(!secret)throw Error('GitHub authentication unavailable');return secret.slice(9);
}
function client(){
 const headers={'User-Agent':'ZQZG-mobile-acceptance','Accept':'application/vnd.github+json','Authorization':'Bearer '+credential(),'X-GitHub-Api-Version':'2022-11-28'};
 return {headers,async api(suffix,method='GET',data){
  const response=await fetch('https://api.github.com/repos/'+repository+suffix,{method,headers:{...headers,...(data?{'Content-Type':'application/json'}:{})},body:data?JSON.stringify(data):undefined,signal:AbortSignal.timeout(60000)});
  if(response.status===404)return null;if(!response.ok)throw Error('GitHub '+method+' '+suffix+': HTTP '+response.status);
  return response.status===204?{}:response.json();
 }};
}
function validatePlan(plan){
 if(plan.repository!==repository||plan.tag!==tag||plan.version!==version||!/^[a-f0-9]{40}$/.test(plan.commit)||plan.prerelease!==true||plan.makeLatest!==false)throw Error('Unexpected acceptance identity');
 for(const asset of plan.assets){
  const file=path.resolve(root,asset.path);
  if(path.dirname(file)!==directory||path.basename(file)!==asset.name||!/^[A-Za-z0-9.+_-]+$/.test(asset.name))throw Error('Unexpected asset path');
 }
}
async function validateFiles(plan){validatePlan(plan);for(const asset of plan.assets){if(fs.statSync(asset.path).size!==asset.size||await sha(asset.path)!==asset.sha256)throw Error('Artifact changed: '+asset.name)}}
async function prepare(){
 if(git('branch','--show-current')!=='main'||git('status','--porcelain'))throw Error('Commit reviewed source on a clean main branch before preparing');
 const commit=git('rev-parse','HEAD');const source=path.join(root,'delivery',release),manifest=readJSON(path.join(source,'manifest.json'));
 if(manifest.version!==version||!manifest.signingMatchesPrevious||manifest.formalMapAccepted||manifest.mapPreviewPackaged||manifest.productionDeployed)throw Error('Acceptance verification receipt mismatch');
 fs.mkdirSync(directory,{recursive:true});const assets=[];
 async function asset(file){const name=path.basename(file);assets.push({name,path:path.relative(root,file).replaceAll('\\','/'),size:fs.statSync(file).size,sha256:await sha(file)})}
 for(const entry of manifest.artifacts){
  const file=path.resolve(source,entry.file);
  if(!file.startsWith(source+path.sep)||await sha(file)!==entry.sha256||fs.statSync(file).size!==entry.bytes)throw Error('Delivery artifact changed');
  const destination=path.join(directory,path.basename(file));fs.copyFileSync(file,destination);await asset(destination);
 }
 const notes=`# 移动端升级验收包 ${version}\n\n源码提交：${commit}\n\n同步厂商入驻登录、企业资料、三步产品发布、工作台、推广和地区统计查询。原包名 com.zqzt.dalunongji，原签名，构建号30；APK独立下载，不替换1.7.0稳定版。请覆盖安装，保留本机数据。\n\nFlutter154项通过、7项跳过，前端185项通过；后端隔离MySQL、go vet、发布构建及168项原生布局检查通过。测试详情见[验收说明](https://github.com/${repository}/blob/${tag}/docs/MOBILE18_ACCEPTANCE.md)。\n\n正式真实地图轮廓仍缺适用资源材料；本机学习边界及其截图未分发。Android真机覆盖安装、相机权限、系统分享、原生媒体，以及Linux实际安装、MySQL8和真实OSS尚待验收。本次未部署生产，不包含AI辅助填写。\n\n附件含原签名APK、源码ZIP和Linux amd64/arm64服务器更新ZIP（含001—016迁移）。服务器包仅供后续授权升级：先备份并运行 upgrade.sh --check，再根据[升级说明](https://github.com/${repository}/blob/${tag}/docs/MOBILE18_ACCEPTANCE.md)操作。密钥、业务数据库、私有媒体不在附件中。\n\n默认API地址保持原值；局域网验收在App设置切换，手机不能以localhost访问电脑。请核对 SHA256SUMS.txt；正式后续发布使用高于30的构建号。\n`;
 const notesFile=path.join(directory,'RELEASE-NOTES.md');fs.writeFileSync(notesFile,notes);await asset(notesFile);
 const publicManifest={repository,tag,version,sourceCommit:commit,packageName:'com.zqzt.dalunongji',certificateSHA256:manifest.certificateSHA256,prerelease:true,makeLatest:false,schema:16,formalMapAccepted:false,mapResourceCount:0,mapPreviewPackaged:false,deviceValidation:false,linuxRuntimeTested:false,productionDeployed:false,unverified:manifest.unverified,artifacts:assets.map(({name,size,sha256})=>({file:name,bytes:size,sha256}))};
 const manifestFile=path.join(directory,'manifest.json');writeJSON(manifestFile,publicManifest);await asset(manifestFile);
 const sums=path.join(directory,'SHA256SUMS.txt');fs.writeFileSync(sums,assets.map(item=>item.sha256+'  '+item.name+'\n').join(''));await asset(sums);
 const plan={repository,tag,version,commit,prerelease:true,makeLatest:false,name:'中企展馆 · 移动端升级验收包 '+version,body:notes,assets};
 validatePlan(plan);writeJSON(planPath,plan);console.log(JSON.stringify({tag,commit,assets:assets.map(({name,size,sha256})=>({name,size,sha256}))},null,2));
}
async function publish(){
 const plan=readJSON(planPath);await validateFiles(plan);
 const c=client(),remote=await c.api('/git/ref/heads/main');if(remote?.object?.sha!==plan.commit)throw Error('Push the exact source commit before publishing');
 const remoteTag=await c.api('/commits/'+encodeURIComponent(tag));if(remoteTag?.sha!==plan.commit)throw Error('Release tag does not point to the source commit');
 const previousReceipt=fs.existsSync(receiptPath)?readJSON(receiptPath):null;
 const latestBefore=previousReceipt?.latestStableBefore??(await c.api('/releases/latest'))?.tag_name??null;
 let current=await c.api('/releases/tags/'+tag);
 if(!current)current=(await c.api('/releases?per_page=100')).find(item=>item.tag_name===tag);
 if(!current)current=await c.api('/releases','POST',{tag_name:tag,target_commitish:plan.commit,name:plan.name,body:plan.body,draft:true,prerelease:true,make_latest:'false'});
 if(current.target_commitish!==plan.commit||!current.prerelease)throw Error('Existing release identity differs; refusing replacement');
 writeJSON(receiptPath,{repository,tag,commit:plan.commit,latestStableBefore:latestBefore,draft:current.draft,assets:[]});
 for(const asset of plan.assets){
  const existing=(await c.api('/releases/'+current.id+'/assets?per_page=100')).find(item=>item.name===asset.name);
  if(existing){
   if(existing.state==='uploaded'&&existing.size===asset.size&&existing.digest==='sha256:'+asset.sha256){console.log('Verified existing asset: '+asset.name);continue}
   if(!current.draft||existing.state==='uploaded')throw Error('Existing asset differs; refusing replacement: '+asset.name);
   await c.api('/releases/assets/'+existing.id,'DELETE');
  }
  const url=new URL(current.upload_url.replace(/\{.*$/,''));
  if(url.protocol!=='https:'||url.hostname!=='uploads.github.com'||!url.pathname.startsWith('/repos/'+repository+'/releases/'))throw Error('Unexpected upload URL');
  url.searchParams.set('name',asset.name);console.log('Uploading '+asset.name+' ('+Math.round(asset.size/1048576)+' MiB)');
  const type=asset.name.endsWith('.apk')?'application/vnd.android.package-archive':asset.name.endsWith('.zip')?'application/zip':asset.name.endsWith('.json')?'application/json':'text/plain';
  const response=await fetch(url,{method:'POST',headers:{...c.headers,'Content-Type':type,'Content-Length':String(asset.size)},body:fs.createReadStream(asset.path),duplex:'half',signal:AbortSignal.timeout(900000)});
  if(!response.ok)throw Error('Upload HTTP '+response.status+': '+asset.name);
  const uploaded=await response.json();if(uploaded.state!=='uploaded'||uploaded.size!==asset.size||uploaded.digest!=='sha256:'+asset.sha256)throw Error('Remote digest mismatch: '+asset.name);
  console.log('GitHub digest verified: '+asset.name);
 }
 if(current.draft)current=await c.api('/releases/'+current.id,'PATCH',{draft:false,prerelease:true,make_latest:'false'});
 const uploaded=await c.api('/releases/'+current.id+'/assets?per_page=100');
 const latestAfter=(await c.api('/releases/latest'))?.tag_name??null;if(latestBefore!==latestAfter)throw Error('Latest stable release changed unexpectedly');
 const receipt={repository,tag,commit:plan.commit,url:current.html_url,draft:current.draft,prerelease:current.prerelease,latestStableBefore:latestBefore,latestStableAfter:latestAfter,assets:uploaded.filter(item=>plan.assets.some(asset=>asset.name===item.name)).map(item=>({name:item.name,url:item.browser_download_url,size:item.size,digest:item.digest}))};
 if(receipt.draft||!receipt.prerelease||receipt.assets.length!==plan.assets.length)throw Error('Published acceptance state mismatch');
 writeJSON(receiptPath,receipt);console.log(JSON.stringify(receipt,null,2));
}
async function verify(){
 const plan=readJSON(planPath);await validateFiles(plan);const receipt=readJSON(receiptPath),c=client();
 const release=await c.api('/releases/tags/'+tag),commit=await c.api('/commits/'+tag);
 if(!release||release.draft||!release.prerelease||commit?.sha!==plan.commit||release.target_commitish!==plan.commit)throw Error('Remote source/tag/release mismatch');
 const assets=await c.api('/releases/'+release.id+'/assets?per_page=100');
 for(const asset of plan.assets){const remote=assets.find(item=>item.name===asset.name);if(!remote||remote.size!==asset.size||remote.digest!=='sha256:'+asset.sha256)throw Error('Remote artifact mismatch: '+asset.name)}
 if(((await c.api('/releases/latest'))?.tag_name??null)!==receipt.latestStableBefore)throw Error('Stable release changed');
 const apk=assets.find(item=>item.name.endsWith('.apk')),expected=plan.assets.find(item=>item.name===apk.name);
 if(!apk.browser_download_url.startsWith('https://github.com/'+repository+'/releases/download/'+tag+'/'))throw Error('Unexpected public download URL');
 console.log('Downloading public APK to verify SHA-256');
 const response=await fetch(apk.browser_download_url,{signal:AbortSignal.timeout(600000)});if(!response.ok)throw Error('Public APK download HTTP '+response.status);
 const hash=crypto.createHash('sha256');let bytes=0;
 for await(const chunk of response.body){hash.update(chunk);bytes+=chunk.length}
 const digest=hash.digest('hex');if(bytes!==expected.size||digest!==expected.sha256)throw Error('Downloaded APK mismatch');
 const result={...receipt,verifiedAt:new Date().toISOString(),downloadedAPK:{bytes,sha256:digest,passed:true},remoteDigestsVerified:plan.assets.length,sourceTagMatches:true,stableReleaseUnchanged:true};writeJSON(receiptPath,result);console.log(JSON.stringify(result,null,2));
}
const mode=process.argv[2];
if(!['prepare','publish','verify'].includes(mode)){console.error('Usage: node scripts/publish-mobile-acceptance.mjs prepare|publish|verify');process.exitCode=1}
else ({prepare,publish,verify}[mode])().catch(error=>{console.error(error.message);process.exitCode=1});
