"""Create a public, data-free Aliyun upgrade bundle and the exact Git source archive."""
from pathlib import Path
from datetime import datetime, timezone
import hashlib, json, shutil, struct, subprocess, tarfile
ROOT=Path(__file__).resolve().parents[1]
TAG='aliyun-20261018-storage-v1'
NAME='zqzg-'+TAG
OUT=ROOT/'delivery'
STAGE=ROOT/'.local'/'aliyun-release'/NAME

def sha(file):
    h=hashlib.sha256()
    with file.open('rb') as stream:
        for data in iter(lambda:stream.read(1024*1024),b''):h.update(data)
    return h.hexdigest()
def copy(source,dest):
    dest.parent.mkdir(parents=True,exist_ok=True)
    if source.suffix in ['.sh','.py']:dest.write_text(source.read_text(encoding='utf-8-sig').replace('\r\n','\n'),encoding='utf-8',newline='\n')
    else:shutil.copyfile(source,dest)
def run(*args):return subprocess.check_output(args,cwd=ROOT,text=True).strip()
def main():
    if STAGE.exists():raise RuntimeError('Preserve existing stage; use another release identifier')
    if run('git','status','--porcelain'):raise RuntimeError('Commit the reviewed source before packaging')
    commit=run('git','rev-parse','HEAD')
    for check in ['web-tests','web-build','upgrade-tests','backend-tests','video-tests','go-vet','build-amd64','build-arm64']:
        if (ROOT/'.local'/('aliyun-'+check+'-exit.txt')).read_text().strip()!='0':raise RuntimeError('Failed check: '+check)
    web=(ROOT/'.local/aliyun-web-tests.log').read_text(encoding='utf-8-sig');backend=(ROOT/'.local/aliyun-backend-tests.log').read_text(encoding='utf-8-sig')
    if '135 passed' not in web or sum(line.startswith('--- PASS:') for line in backend.splitlines())!=79 or '\nPASS\n' not in backend.replace('\r\n','\n'):raise RuntimeError('Incomplete fresh verification')
    source=OUT/(NAME+'-source.zip');linux=OUT/(NAME+'-linux.tar.gz')
    if source.exists() or linux.exists():raise RuntimeError('Never overwrite an existing release archive')
    OUT.mkdir(exist_ok=True);STAGE.mkdir(parents=True)
    subprocess.run(['git','archive','--format=zip','--prefix='+NAME+'-source/','--output='+str(source),commit],cwd=ROOT,check=True)
    hashes={}
    for arch,machine in [('amd64',62),('arm64',183)]:
        binary=ROOT/'.local/builds'/TAG/('linux-'+arch)/'exhibition'
        header=binary.read_bytes()[:64]
        if header[:4]!=b'\x7fELF' or struct.unpack('<H',header[18:20])[0]!=machine:raise RuntimeError('Unexpected ELF architecture')
        metadata=run('go','version','-m',str(binary))
        if 'vcs.revision='+commit not in metadata or 'vcs.modified=false' not in metadata:raise RuntimeError('Binary Git provenance does not match clean source')
        copy(binary,STAGE/'server'/('exhibition-linux-'+arch));hashes[arch]=sha(binary)
        (STAGE/'verification').mkdir(exist_ok=True);(STAGE/'verification'/('build-'+arch+'.txt')).write_text(metadata+'\n',encoding='utf8')
    shutil.copytree(ROOT/'web/dist',STAGE/'web/dist')
    apk=STAGE/'web/dist/downloads/dalunongji.apk';apk_hash=sha(apk)
    if apk_hash!='5f0a35407fcb6dc2e9001fa4f1578ca7f63463a7ea6feece2ab18866dfe1441d':raise RuntimeError('Previously signed APK changed')
    for directory in ['migrations','catalog','licenses']:shutil.copytree(ROOT/'server'/directory,STAGE/directory)
    copy(ROOT/'deploy/linux/upgrade-storage.sh',STAGE/'upgrade.sh');copy(ROOT/'deploy/linux/aliyun-upgrade-support.py',STAGE/'aliyun-upgrade-support.py')
    for name in ['nginx-storage-domain.example.conf','storage-prepare.example.sh']:copy(ROOT/'deploy/linux'/name,STAGE/'deployment'/name)
    copy(ROOT/'scripts/storage-copy-verify.py',STAGE/'deployment/storage-copy-verify.py')
    for name in ['STORAGE_SETTINGS_UPGRADE.md','STORAGE_SETTINGS_VALIDATION.md','STORAGE_SETTINGS_FILES.md','openapi.json']:copy(ROOT/'docs'/name,STAGE/'docs'/name)
    readme=(ROOT/'releases'/TAG/'README.md').read_text(encoding='utf-8-sig').replace('../../docs/','docs/')
    (STAGE/'README.md').write_text(readme,encoding='utf-8')
    for name in ['web-tests','web-build','upgrade-tests','backend-tests','video-tests','go-vet']:
        text=(ROOT/'.local'/('aliyun-'+name+'.log')).read_text(encoding='utf-8-sig')
        if name in ['backend-tests','video-tests']:text='\n'.join(line for line in text.splitlines() if line.startswith(('--- PASS:','--- SKIP:','PASS','ok ')))+'\n'
        (STAGE/'verification'/(name+'.txt')).write_text(text,encoding='utf-8')
    manifest={'release':TAG,'createdAt':datetime.now(timezone.utc).isoformat(),'repository':'https://github.com/wanglijian469/zqzhg','gitCommit':commit,'schema':15,'migration':'015_storage_settings.sql','linuxArchitectures':['amd64','arm64'],'binarySHA256':hashes,'androidVersion':'1.7.0+29','apkSHA256':apk_hash,'androidRebuilt':False,'sourceSHA256':sha(source),'sourceArchive':source.name,'tests':{'frontend':135,'backend':80,'upgradeHelper':6,'goVet':True,'bashSyntax':True},'productionDeployed':False,'includesBusinessDatabase':False,'includesPrivateMedia':False,'includesCredentials':False,'deploymentMode':'existing managed installation; private backup before additive migration; environment and key retained','unverified':['Actual ECS/Linux startup and systemd upgrade','NAS/OSS and production DNS/TLS/Nginx','Real mobile device native integration'],'frontendIndexSHA256':sha(STAGE/'web/dist/index.html')}
    (STAGE/'manifest.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+'\n',encoding='utf8')
    files=sorted(p for p in STAGE.rglob('*') if p.is_file());(STAGE/'SHA256SUMS').write_text(''.join(sha(p)+'  '+p.relative_to(STAGE).as_posix()+'\n' for p in files),encoding='utf8',newline='\n')
    def attributes(info):
        info.uid=info.gid=0;info.uname=info.gname='root';info.mode=0o755 if info.isdir() or info.name.endswith(('/upgrade.sh','/exhibition-linux-amd64','/exhibition-linux-arm64')) else 0o644
        return info
    with tarfile.open(linux,'w:gz',compresslevel=6) as archive:archive.add(STAGE,arcname=NAME,filter=attributes)
    public_manifest=OUT/('manifest-'+TAG+'.json');public_manifest.write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+'\n',encoding='utf8')
    sums=OUT/('SHA256SUMS-'+TAG+'.txt');sums.write_text(''.join(sha(p)+'  '+p.name+'\n' for p in [linux,source,public_manifest]),encoding='utf8',newline='\n')
    receipt={'tag':TAG,'commit':commit,'assets':[{'path':str(p.relative_to(ROOT)),'name':p.name,'sha256':sha(p),'size':p.stat().st_size} for p in [linux,source,public_manifest,sums]]}
    (ROOT/'.local/aliyun-release-assets.json').write_text(json.dumps(receipt,indent=2)+'\n',encoding='utf8');print(json.dumps(receipt,indent=2))
if __name__=='__main__':main()
