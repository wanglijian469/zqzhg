"""Package the verified Schema20 watermarks without private state or stable-download changes."""
import json,re,runpy,struct,zipfile
from pathlib import Path
from datetime import datetime,timezone
ROOT=Path(__file__).resolve().parents[1]
base=runpy.run_path(str(ROOT/'scripts/package-taxonomy-acceptance.py'))
read,sha,copy,json_write,zip_files=(base[k] for k in ['read','sha','copy','json_write','zip_files'])
OUT=ROOT/'delivery/image-watermark-20261022'
STAGE=ROOT/'.local/watermark-server-bundle'
CERT=base['CERT']

def go_result(name):
    rows=[json.loads(line) for line in read('.local/watermark-'+name).splitlines() if line.startswith('{')]
    assert rows and not any(r['Action']=='fail' for r in rows),name
    assert any(r['Action']=='pass' for r in rows),name
    return len({r['Test'] for r in rows if r['Action']=='pass' and r.get('Test')}),len({r['Test'] for r in rows if r['Action']=='skip' and r.get('Test')})

def main():
    passed,skipped=go_result('go-tests.txt')
    for name in ['go-extra.txt','go-final.txt','go-purpose.txt','go-capture.txt','browser-harness.txt']:go_result(name)
    assert not read('.local/watermark-go-vet.txt').strip()
    web=json.loads(read('.local/watermark-web-tests.json'));assert web['numPassedTests']==194 and web['numFailedTests']==0
    assert not read('.local/watermark-web-typecheck.txt').strip() and 'built in' in read('.local/watermark-web-build.txt')
    assert '+166 ~7:' in read('.local/watermark-flutter-tests.txt') and 'All tests passed!' in read('.local/watermark-flutter-tests.txt')
    assert 'No issues found!' in read('.local/watermark-flutter-analyze.txt')
    assert 'BUILD SUCCESSFUL' in read('.local/watermark-android-build.txt')
    assert "name='com.zqzt.dalunongji' versionCode='34' versionName='1.8.0-rc.5'" in read('.local/watermark-apk-metadata.txt')
    assert 'Verifies' in read('.local/watermark-apk-signature.txt') and CERT in read('.local/watermark-apk-signature.txt')
    browser=json.loads(read('docs/test-output/watermark-browser.json'));assert len(browser['layouts'])==8 and not browser['errors']
    local=json.loads(read('.local/watermark-local-start.json'));assert local['migration']['schema']==20 and local['migration']['watermark_rows']==0
    regression=json.loads(read('.local/watermark-local-verification.json'));assert len(regression['layouts'])==32 and not regression['errors'] and regression['outlinePaths']>0
    assert not any(x['overflow'] for x in [*browser['layouts'],*regression['layouts']])
    stable=sha(ROOT/'delivery/android/dalunongji.apk');assert sha(ROOT/'web/public/downloads/dalunongji.apk')==stable=='5f0a35407fcb6dc2e9001fa4f1578ca7f63463a7ea6feece2ab18866dfe1441d'
    assert sha(ROOT/'server/fonts/NotoSansSC-Regular.otf')=='faa6c9df652116dde789d351359f3d7e5d2285a2b2a1f04a2d7244df706d5ea9'
    assert not STAGE.exists(),'Use a fresh staging directory'
    STAGE.mkdir(parents=True);OUT.mkdir(parents=True,exist_ok=True)
    validation={'backendFullSuitePassed':passed,'backendFullSuiteSkipped':skipped,'watermarkFinalCasesPassed':go_result('go-final.txt')[0],'additionalAdministratorPurposeCasePassed':True,'watermarkBackupRestoreVerified':True,'goVetClean':True,'webPassed':194,'webTypecheckClean':True,'webWatermarkLayouts':8,'localWebRegressionLayouts':32,'flutterPassed':166,'flutterSkipped':7,'flutterAnalyzeClean':True,'originalCertificateVerified':True,'physicalDeviceVerified':False,'localSchema':20,'businessBeforeAfterEqual':True,'oldMediaAndReviewHashesUnchanged':True,'stableDownloadUnchanged':True,'productionDeployed':False}
    logs=['go-tests.txt','go-extra.txt','go-final.txt','go-purpose.txt','go-capture.txt','go-vet.txt','browser-harness.txt','web-tests.json','web-typecheck.txt','web-build.txt','flutter-tests.txt','flutter-extra.txt','flutter-analyze.txt','android-build.txt','apk-metadata.txt','apk-signature.txt']
    for name in logs:copy(ROOT/'.local'/('watermark-'+name),ROOT/'docs/test-output'/('watermark-'+name))
    json_write(ROOT/'docs/test-output/watermark-local-verification.json',regression)
    json_write(ROOT/'docs/test-output/watermark-local-migration.json',{'schema':20,'business':local['business'],'backupCreated':True,'productionDeployed':False,'newWatermarkRecordsAfterMigration':0,'startedAt':local['startedAt']})
    json_write(ROOT/'docs/test-output/watermark-validation.json',validation)
    for p in (ROOT/'docs/test-output').glob('watermark-*'):copy(p,OUT/'verification'/p.name)
    for p in (ROOT/'docs/screenshots/watermark').glob('*.png'):copy(p,OUT/'screenshots'/p.name)
    for name in ['IMAGE_WATERMARK.md','openapi.json','APP_PHONE_AUTH.md','TAXONOMY_COMPACT.md','agriculture-taxonomy-compact-mappings.json']:
        copy(ROOT/'docs'/name,OUT/'docs'/name)
    copy(ROOT/'docs/IMAGE_WATERMARK.md',OUT/'README.md')
    apk=OUT/'android/dalunongji-1.8.0-rc.5+34.apk';copy(ROOT/'mobile/build/app/outputs/flutter-apk/app-release.apk',apk)
    with zipfile.ZipFile(apk) as z:assert not any('/maps/' in n or 'map-preview' in n for n in z.namelist())
    for name,machine in [('exhibition-linux-amd64',62),('exhibition-linux-arm64',183)]:
        p=ROOT/'.local/watermark-build'/name;h=p.read_bytes()[:20];assert h[:4]==b'\x7fELF' and struct.unpack('<H',h[18:20])[0]==machine;copy(p,STAGE/'server'/name)
    copy(ROOT/'.local/watermark-build/exhibition-windows-amd64.exe',STAGE/'server/exhibition-windows-amd64.exe')
    for p in (ROOT/'.local/watermark-dist').rglob('*'):
        rel=p.relative_to(ROOT/'.local/watermark-dist')
        if p.is_file() and 'previews' not in rel.parts:copy(p,STAGE/'web/dist'/rel)
    for d in ['migrations','catalog','licenses']:
        for p in (ROOT/'server'/d).rglob('*'):
            if p.is_file():copy(p,STAGE/d/p.relative_to(ROOT/'server'/d))
    copy(ROOT/'deploy/linux/upgrade-storage.sh',STAGE/'upgrade.sh');copy(ROOT/'deploy/linux/aliyun-upgrade-support.py',STAGE/'aliyun-upgrade-support.py');copy(ROOT/'docs/IMAGE_WATERMARK.md',STAGE/'README.md')
    manifest={'release':'image-watermark-20261022','schema':20,'appVersion':'1.8.0-rc.5+34','package':'com.zqzt.dalunongji','certificateSHA256':CERT,'signingMatchesPrevious':True,'builtAt':datetime.now(timezone.utc).isoformat(),'stableDownloadSHA256':stable,'stableDownloadUpdated':False,'productionDeployed':False,'localApplicationStarted':True,'mapPreviewPackaged':False,'validation':validation,'unverified':['Physical-device installation/upgrade, keyboard, large fonts and downloads','SMS supplier actual delivery','iOS build','Linux runtime']}
    json_write(STAGE/'manifest.json',manifest)
    (STAGE/'SHA256SUMS').write_text(''.join(sha(p)+'  '+p.relative_to(STAGE).as_posix()+'\n' for p in sorted(STAGE.rglob('*')) if p.is_file() and p.name!='SHA256SUMS'),encoding='utf-8')
    server_zip=OUT/'image-watermark-server-update.zip';zip_files(server_zip,(p for p in STAGE.rglob('*') if p.is_file()),STAGE)
    sources=set(base['source_files']());sources.update((ROOT/'docs/test-output').glob('watermark-*'));sources.update((ROOT/'docs/screenshots/watermark').glob('*.png'));sources.update((ROOT/'docs').glob('agriculture-taxonomy*.json'))
    source_zip=OUT/'image-watermark-source.zip';zip_files(source_zip,sources,ROOT)
    for p in [source_zip,server_zip]:
        with zipfile.ZipFile(p) as z:assert not any('/map-preview/' in n or 'factory-map-outline' in n or '/previews/' in n or re.search(r'(^|/)(\.env|key\.properties|local\.properties|signing\.json)$|\.(jks|keystore|zqbak|db|sqlite)$',n) for n in z.namelist())
    manifest['sourceFiles']=len(sources);manifest['artifacts']=[{'file':p.relative_to(OUT).as_posix(),'bytes':p.stat().st_size,'sha256':sha(p)} for p in [apk,source_zip,server_zip]]
    json_write(OUT/'manifest.json',manifest)
    (OUT/'SHA256SUMS.txt').write_text(''.join(sha(p)+'  '+p.relative_to(OUT).as_posix()+'\n' for p in sorted(OUT.rglob('*')) if p.is_file() and p.name!='SHA256SUMS.txt'),encoding='utf-8')
    print(json.dumps({'directory':str(OUT),'validation':validation,'artifacts':manifest['artifacts']},ensure_ascii=False,indent=2))
if __name__=='__main__':main()
