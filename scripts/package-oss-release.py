"""Package committed v1.12.4 code, same-signature Android 53 and 77 map files."""
from pathlib import Path
from datetime import datetime,timezone
import hashlib,json,shutil,subprocess,tarfile,zipfile,sys

ROOT=Path(__file__).resolve().parents[1];WORK=ROOT/'.local/release-1.12.4';OUT=ROOT/'delivery/release-v1.12.4';NAME='zqzg-v1.12.4'
def sha(p):
    with p.open('rb') as stream:return hashlib.file_digest(stream,'sha256').hexdigest()
def inventory(folder):return {p.relative_to(folder).as_posix():sha(p) for p in sorted(folder.rglob('*')) if p.is_file()}
def main():
    source=WORK/'source';commit=subprocess.check_output(['git','rev-parse','HEAD'],cwd=source,text=True).strip()
    assert not subprocess.check_output(['git','status','--porcelain'],cwd=source).strip(),'build_source_not_clean'
    stage=WORK/'package'/NAME;assert not stage.exists();(stage/'server').mkdir(parents=True);OUT.mkdir(parents=True,exist_ok=True)
    binary=WORK/'linux-amd64/exhibition';preflight=WORK/'linux-amd64/bailian-preflight'
    for path in [binary,preflight]:
        metadata=subprocess.check_output(['go','version','-m',str(path)],text=True)
        for expected in ['vcs.revision='+commit,'vcs.modified=false','GOOS=linux','GOARCH=amd64','CGO_ENABLED=0']:assert expected in metadata,'binary_source_mismatch'
        shutil.copy2(path,stage/'server'/path.name)
    for folder in ['migrations','catalog','licenses']:shutil.copytree(source/'server'/folder,stage/'server'/folder)
    assert sorted(int(p.name[:3]) for p in (stage/'server/migrations').glob('*.sql'))==list(range(1,40))
    for path in (ROOT/'.local/oss-web').iterdir():
        if path.name=='downloads':continue
        target=stage/'web/dist'/path.name;target.parent.mkdir(parents=True,exist_ok=True)
        if path.is_dir():shutil.copytree(path,target)
        else:shutil.copy2(path,target)
    shutil.copytree(ROOT/'.local/map-preview',stage/'map-demo');maps=inventory(stage/'map-demo');assert len(maps)==77
    candidate=json.loads((ROOT/'delivery/oss-039-candidate/latest.json').read_text())['manifest'];assert candidate['realOSSWriteFlowAccepted'] and candidate['realOSSPublicPrefixBoundariesAccepted']
    apk=ROOT/'delivery/android-v1.12.0/大陆农机-1.12.0+53.apk';android=json.loads((apk.parent/'manifest.json').read_text(encoding='utf-8-sig'));assert sha(apk)==android['sha256'] and android['signingMatchesPrevious']
    android.update(versionName='1.12.0',versionCode=53,asset='zqzhg-1.12.0-53.apk',sourceCommit=commit,sourceTag='v1.12.4',schemaVersion=39,bytes=apk.stat().st_size,officialDownloadURL='https://zqzhg.com/downloads/dalunongji.apk',releaseStatus='prepared',endpointEditable=False,embeddedWebAssetsVerified=239,releaseBasis='User-authorized upgrade; same certificate, component tests and production-proxy acceptance. Physical-device acceptance not claimed.')
    with zipfile.ZipFile(apk) as package:
        assert package.read('assets/workspace/index.html')==(ROOT/'.local/oss-web/index.html').read_bytes(),'embedded_web_differs'
    (stage/'android').mkdir();shutil.copy2(apk,stage/'android/dalunongji.apk')
    for original,target in [('deploy/linux/upgrade-oss-039.py','upgrade.py'),('server/.env.example','server/.env.example'),('releases/v1.12.4/README.md','README.md')]:
        (stage/target).write_bytes((source/original).read_bytes().replace(b'\r\n',b'\n'))
    shutil.copytree(source/'deploy/oss',stage/'deploy/oss');(stage/'deploy/linux').mkdir();shutil.copy2(source/'deploy/linux/rollback-oss-039.sql',stage/'deploy/linux/rollback-oss-039.sql')
    (stage/'docs').mkdir()
    for name in ['OSS_STORAGE.md','OSS_TEST_RESULTS.md','OSS_ACCEPTANCE_20261119.md','MAP_DEPLOYMENT_REQUIREMENTS.md']:shutil.copy2(source/'docs'/name,stage/'docs'/name)
    manifest={'version':'1.12.4','gitCommit':commit,'schemaVersion':39,'architecture':'linux-amd64','createdUTC':datetime.now(timezone.utc).isoformat(),'binarySHA256':sha(binary),'android':android,'mapFiles':maps,'mapUsage':'development-demonstration; original attribution retained','realOSSWriteFlowAccepted':True,'realOSSPublicPrefixBoundariesAccepted':True,'databaseMigrationAdded':[39],'defaultQuotaBytes':314572800,'migrationScope':[67,79],'originalMediaDeletionAllowed':False,'backupTarget':'operator-workstation-encrypted','policeNumber':'冀公网安备13052802002070号','policeURL':'https://beian.mps.gov.cn/#/query/webSearch?code=13052802002070','includesCredentials':False,'includesPrivateMedia':False,'includesBusinessDatabase':False,'actualProductionAcceptanceCompleted':False}
    (stage/'manifest.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+'\n',encoding='utf-8')
    hashes=inventory(stage);(stage/'SHA256SUMS').write_text(''.join(h+'  '+n+'\n' for n,h in hashes.items()),encoding='utf-8')
    archive=OUT/(NAME+'-linux-amd64.tar.gz')
    with tarfile.open(archive,'x:gz') as bundle:
        for path in sorted(p for p in stage.rglob('*') if p.is_file()):
            member=bundle.gettarinfo(str(path),arcname=NAME+'/'+path.relative_to(stage).as_posix());member.uid=member.gid=0;member.uname=member.gname='';member.mode=0o755 if path.name in ['exhibition','bailian-preflight','upgrade.py'] else 0o644
            with path.open('rb') as stream:bundle.addfile(member,stream)
    with tarfile.open(archive) as bundle:
        for name,digest in hashes.items():assert hashlib.sha256(bundle.extractfile(NAME+'/'+name).read()).hexdigest()==digest
    source_zip=OUT/(NAME+'-source.zip');subprocess.run(['git','archive','--format=zip','--prefix='+NAME+'-source/','--output='+str(source_zip),commit],cwd=source,check=True)
    shutil.copy2(apk,OUT/android['asset']);(OUT/'android-manifest.json').write_text(json.dumps(android,ensure_ascii=False,indent=2)+'\n',encoding='utf-8')
    receipt={'commit':commit,'package':str(archive),'sha256':sha(archive),'stage':str(stage),'sourceZip':str(source_zip),'sourceSHA256':sha(source_zip),'android':android}
    (WORK/'package.json').write_text(json.dumps(receipt,ensure_ascii=False,indent=2)+'\n',encoding='utf-8');print(json.dumps({k:v for k,v in receipt.items() if k!='android'},ensure_ascii=False))
if __name__=='__main__':main()
