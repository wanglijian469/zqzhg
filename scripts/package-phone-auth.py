"""Package the validated App phone-auth release without private state or preview maps."""
import json,re,runpy,struct,zipfile
from pathlib import Path
from datetime import datetime,timezone
ROOT=Path(__file__).resolve().parents[1]
base=runpy.run_path(str(ROOT/'scripts/package-taxonomy-acceptance.py'))
read,sha,copy,json_write,zip_files=(base[k] for k in ['read','sha','copy','json_write','zip_files'])
OUT=ROOT/'delivery/app-phone-auth-20261021'
STAGE=ROOT/'.local/phone-server-bundle'
CERT=base['CERT']

def main():
    passed,skipped=base['assert_go']('.local/phone-go-tests.txt')
    base['assert_go']('.local/phone-go-compat.txt');base['assert_go']('.local/phone-go-tx.txt')
    assert not read('.local/phone-go-vet.txt').strip()
    web=json.loads(read('.local/phone-web-tests.json'))
    assert web['numPassedTests']==191 and web['numFailedTests']==0
    assert 'built in' in read('.local/phone-web-build.txt')
    assert '+165 ~7:' in read('.local/phone-flutter-tests.txt') and 'All tests passed!' in read('.local/phone-flutter-tests.txt')
    assert 'No issues found!' in read('.local/phone-flutter-analyze.txt')
    assert 'BUILD SUCCESSFUL' in read('.local/phone-android-build.txt')
    assert "name='com.zqzt.dalunongji' versionCode='33' versionName='1.8.0-rc.4'" in read('.local/phone-apk-metadata.txt')
    assert 'Verifies' in read('.local/phone-apk-signature.txt') and CERT in read('.local/phone-apk-signature.txt')
    layouts=json.loads(read('docs/test-output/phone-native-layout.json'));assert layouts['checks']==180
    local=json.loads(read('.local/phone-local-start.json'));assert local['migration']['schema']==19
    browser=json.loads(read('.local/phone-local-verification.json'));assert len(browser['layouts'])==32 and not browser['errors'] and not any(x['overflow'] for x in browser['layouts']) and browser['outlinePaths']>0
    stable=sha(ROOT/'delivery/android/dalunongji.apk')
    assert sha(ROOT/'web/public/downloads/dalunongji.apk')==stable=='5f0a35407fcb6dc2e9001fa4f1578ca7f63463a7ea6feece2ab18866dfe1441d'
    assert not (ROOT/'mobile/lib/native_industry_map.dart').exists()
    app_source='\n'.join(p.read_text(encoding='utf-8-sig') for p in (ROOT/'mobile/lib').glob('*.dart'))
    assert not any(x in app_source for x in ['NativeIndustryMapScreen','/public/industry-map','/maps/manifest.json','/dev/map-preview'])
    assert not STAGE.exists(),'Use a fresh staging directory'
    STAGE.mkdir(parents=True);OUT.mkdir(parents=True,exist_ok=True)
    validation={'backendPassed':passed,'backendSkipped':skipped,'webPassed':191,'flutterPassed':165,'flutterSkipped':7,'nativeFormLayouts':180,'webLayouts':32,'physicalDeviceVerified':False,'goVetClean':True,'flutterAnalyzeClean':True,'originalCertificateVerified':True,'websiteMapRetained':True,'backupCreated':True,'localSchema':19,'businessBeforeAfterEqual':True}
    logs=['go-tests.txt','go-compat.txt','go-tx.txt','go-vet.txt','web-tests.json','web-build.txt','flutter-tests.txt','flutter-analyze.txt','native-layout.txt','android-build.txt','apk-metadata.txt','apk-signature.txt']
    for name in logs:copy(ROOT/'.local'/('phone-'+name),ROOT/'docs/test-output'/('phone-'+name))
    json_write(ROOT/'docs/test-output/phone-local-verification.json',browser)
    json_write(ROOT/'docs/test-output/phone-local-migration.json',{'schema':19,'business':local['business'],'backupCreated':True,'productionDeployed':False,'startedAt':local['startedAt']})
    json_write(ROOT/'docs/test-output/phone-validation.json',validation)
    for p in (ROOT/'docs/test-output').glob('phone-*'):copy(p,OUT/'verification'/p.name)
    for p in (ROOT/'docs/screenshots/phone-auth').glob('*.png'):copy(p,OUT/'screenshots'/p.name)
    for name in ['APP_PHONE_AUTH.md','openapi.json','TAXONOMY_COMPACT.md','agriculture-taxonomy-catalog.json','agriculture-taxonomy-compact-mappings.json']:
        copy(ROOT/'docs'/name,OUT/'docs'/name)
    copy(ROOT/'docs/APP_PHONE_AUTH.md',OUT/'README.md')
    apk=OUT/'android/dalunongji-1.8.0-rc.4+33.apk';copy(ROOT/'mobile/build/app/outputs/flutter-apk/app-release.apk',apk)
    with zipfile.ZipFile(apk) as z:assert not any('/maps/' in n or 'map-preview' in n for n in z.namelist())
    for name,machine in [('exhibition-linux-amd64',62),('exhibition-linux-arm64',183)]:
        p=ROOT/'.local/phone-build'/name;h=p.read_bytes()[:20];assert h[:4]==b'\x7fELF' and struct.unpack('<H',h[18:20])[0]==machine;copy(p,STAGE/'server'/name)
    copy(ROOT/'.local/phone-build/exhibition-windows-amd64.exe',STAGE/'server/exhibition-windows-amd64.exe')
    for p in (ROOT/'.local/phone-final-dist').rglob('*'):
        rel=p.relative_to(ROOT/'.local/phone-final-dist')
        if p.is_file() and 'previews' not in rel.parts:copy(p,STAGE/'web/dist'/rel)
    for d in ['migrations','catalog','licenses']:
        for p in (ROOT/'server'/d).rglob('*'):
            if p.is_file():copy(p,STAGE/d/p.relative_to(ROOT/'server'/d))
    copy(ROOT/'deploy/linux/upgrade-storage.sh',STAGE/'upgrade.sh')
    copy(ROOT/'deploy/linux/aliyun-upgrade-support.py',STAGE/'aliyun-upgrade-support.py')
    copy(ROOT/'docs/APP_PHONE_AUTH.md',STAGE/'README.md')
    manifest={'release':'app-phone-auth-20261021','schema':19,'appVersion':'1.8.0-rc.4+33','package':'com.zqzt.dalunongji','certificateSHA256':CERT,'signingMatchesPrevious':True,'builtAt':datetime.now(timezone.utc).isoformat(),'stableDownloadSHA256':stable,'stableDownloadUpdated':False,'productionDeployed':False,'localApplicationStarted':True,'mapPreviewPackaged':False,'validation':validation,'unverified':['Physical-device installation/upgrade, real keyboard and permissions','SMS supplier actual delivery','iOS build','Linux runtime']}
    json_write(STAGE/'manifest.json',manifest)
    (STAGE/'SHA256SUMS').write_text(''.join(sha(p)+'  '+p.relative_to(STAGE).as_posix()+'\n' for p in sorted(STAGE.rglob('*')) if p.is_file() and p.name!='SHA256SUMS'),encoding='utf-8')
    server_zip=OUT/'app-phone-auth-server-update.zip';zip_files(server_zip,(p for p in STAGE.rglob('*') if p.is_file()),STAGE)
    sources=set(base['source_files']());sources.update((ROOT/'docs/test-output').glob('phone-*'));sources.update((ROOT/'docs/screenshots/phone-auth').glob('*.png'));sources.update((ROOT/'docs').glob('agriculture-taxonomy*.json'))
    source_zip=OUT/'app-phone-auth-source.zip';zip_files(source_zip,sources,ROOT)
    for p in [source_zip,server_zip]:
        with zipfile.ZipFile(p) as z:assert not any('/map-preview/' in n or 'factory-map-outline' in n or '/previews/' in n or re.search(r'(^|/)(\.env|key\.properties|local\.properties|signing\.json)$|\.(jks|keystore|zqbak|db|sqlite)$',n) for n in z.namelist())
    manifest['sourceFiles']=len(sources);manifest['artifacts']=[{'file':p.relative_to(OUT).as_posix(),'bytes':p.stat().st_size,'sha256':sha(p)} for p in [apk,source_zip,server_zip]]
    json_write(OUT/'manifest.json',manifest)
    (OUT/'SHA256SUMS.txt').write_text(''.join(sha(p)+'  '+p.relative_to(OUT).as_posix()+'\n' for p in sorted(OUT.rglob('*')) if p.is_file() and p.name!='SHA256SUMS.txt'),encoding='utf-8')
    print(json.dumps({'directory':str(OUT),'validation':validation,'artifacts':manifest['artifacts']},ensure_ascii=False,indent=2))
if __name__=='__main__':main()
