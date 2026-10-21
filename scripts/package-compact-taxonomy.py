"""Package the validated Schema18 revision; never publish or include private state."""
import json
import re
import runpy
import struct
import zipfile
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
base = runpy.run_path(str(ROOT / 'scripts/package-taxonomy-acceptance.py'))
read, sha, copy, json_write, zip_files = (base[k] for k in ['read', 'sha', 'copy', 'json_write', 'zip_files'])
OUT = ROOT / 'delivery/compact-taxonomy-20261020'
STAGE = ROOT / '.local/compact-server-bundle'
CERT = base['CERT']


def main():
    passed, skipped = base['assert_go']('.local/compact-go-tests.txt')
    video, _ = base['assert_go']('.local/compact-go-video.txt')
    base['assert_go']('.local/compact-browser-tests.txt')
    assert '--- PASS: TestMapPreviewBrowser' in read('.local/compact-map-tests.txt')
    assert not read('.local/compact-go-vet.txt').strip()
    web = json.loads(read('.local/compact-web-tests.json'))
    assert web['numPassedTests'] == 191 and web['numFailedTests'] == 0
    native = read('.local/compact-flutter-tests.txt')
    assert '+159 ~7:' in native and 'All tests passed!' in native
    assert 'No issues found!' in read('.local/compact-flutter-analyze.txt')
    assert 'BUILD SUCCESSFUL' in read('.local/compact-android-build.txt')
    assert 'built in' in read('.local/compact-web-build.txt')
    assert "name='com.zqzt.dalunongji' versionCode='32' versionName='1.8.0-rc.3'" in read('.local/compact-apk-metadata.txt')
    assert 'Verifies' in read('.local/compact-apk-signature.txt') and CERT in read('.local/compact-apk-signature.txt')
    browser = json.loads(read('docs/test-output/compact-browser.json'))
    assert len(browser['layouts']) == 50 and not browser['errors'] and not any(x['overflow'] for x in browser['layouts'])
    catalog = json.loads(read('docs/agriculture-taxonomy-catalog.json'))
    assert (catalog['vendor_roots'], catalog['product_roots'], catalog['product_second_level']) == (14, 10, 35)
    mappings = json.loads(read('docs/agriculture-taxonomy-compact-mappings.json'))
    assert len(mappings['mappings']) == 134
    local = json.loads(read('.local/compact-local-start.json'))
    verified = json.loads(read('.local/compact-local-verification.json'))
    assert local['migration']['schema'] == 18 and verified['directoryCount'] == 45
    assert verified['mapBeforeRestart'] and verified['mapAfterRestart'] and verified['outlinePaths'] > 0
    stable = sha(ROOT / 'delivery/android/dalunongji.apk')
    assert sha(ROOT / 'web/public/downloads/dalunongji.apk') == stable
    assert stable == json.loads(read('.local/android-preview-verification.json'))['stableSHA256']
    assert not STAGE.exists(), 'Do not mix staging directories from different builds'
    OUT.mkdir(parents=True, exist_ok=True)
    STAGE.mkdir(parents=True)
    validation = {'backendPassed': passed + video, 'backendSkipped': skipped, 'webPassed': 191,
                  'flutterPassed': 159, 'flutterSkipped': 7, 'browserLayouts': 50, 'browserFlows': len(browser['checks']),
                  'nativeTaxonomyLayouts': 64, 'mapOutlineBrowserPassed': True, 'goVetClean': True,
                  'flutterAnalyzeClean': True, 'localMapPersistsAfterRestart': True, 'physicalDeviceVerified': False}
    logs = ['go-tests.txt', 'go-video.txt', 'go-vet.txt', 'web-tests.json', 'web-build.txt', 'flutter-tests.txt',
            'flutter-analyze.txt', 'browser-tests.txt', 'map-tests.txt', 'android-build.txt', 'apk-metadata.txt', 'apk-signature.txt']
    for name in logs:
        copy(ROOT / '.local' / ('compact-' + name), ROOT / 'docs/test-output' / ('compact-' + name))
    json_write(ROOT / 'docs/test-output/compact-validation.json', validation)
    json_write(ROOT / 'docs/test-output/compact-local-verification.json', verified)
    for p in (ROOT / 'docs/test-output').glob('compact-*'):
        copy(p, OUT / 'verification' / p.name)
    for p in (ROOT / 'docs/screenshots/compact-taxonomy').glob('*.png'):
        assert p.stem != 'failure' and not p.stem.startswith('overflow'), p
        copy(p, OUT / 'screenshots' / p.name)
    docs = ['TAXONOMY_COMPACT.md', 'agriculture-taxonomy-catalog.json', 'agriculture-taxonomy-compact-mappings.json', 'agriculture-taxonomy-schema17.json', 'openapi.json']
    for name in docs:
        copy(ROOT / 'docs' / name, OUT / 'docs' / name)
    copy(ROOT / 'docs/TAXONOMY_COMPACT.md', OUT / 'README.md')
    apk = OUT / 'android/dalunongji-1.8.0-rc.3+32.apk'
    copy(ROOT / 'mobile/build/app/outputs/flutter-apk/app-release.apk', apk)
    with zipfile.ZipFile(apk) as archive:
        assert not any('map-preview' in n or '/maps/' in n for n in archive.namelist())
    for name, machine in [('exhibition-linux-amd64', 62), ('exhibition-linux-arm64', 183)]:
        p = ROOT / '.local/compact-build' / name
        header = p.read_bytes()[:20]
        assert header[:4] == b'\x7fELF' and struct.unpack('<H', header[18:20])[0] == machine
        copy(p, STAGE / 'server' / name)
    copy(ROOT / '.local/compact-build/exhibition-windows-amd64.exe', STAGE / 'server/exhibition-windows-amd64.exe')
    for p in (ROOT / '.local/compact-dist').rglob('*'):
        rel = p.relative_to(ROOT / '.local/compact-dist')
        if p.is_file() and 'previews' not in rel.parts:
            copy(p, STAGE / 'web/dist' / rel)
    for directory in ['migrations', 'catalog', 'licenses']:
        for p in (ROOT / 'server' / directory).rglob('*'):
            if p.is_file():
                copy(p, STAGE / directory / p.relative_to(ROOT / 'server' / directory))
    copy(ROOT / 'deploy/linux/upgrade-storage.sh', STAGE / 'upgrade.sh')
    copy(ROOT / 'deploy/linux/aliyun-upgrade-support.py', STAGE / 'aliyun-upgrade-support.py')
    copy(ROOT / 'docs/TAXONOMY_COMPACT.md', STAGE / 'README.md')
    manifest = {'release': 'compact-taxonomy-20261020', 'schema': 18, 'appVersion': '1.8.0-rc.3+32',
                'package': 'com.zqzt.dalunongji', 'certificateSHA256': CERT, 'signingMatchesPrevious': True,
                'builtAt': datetime.now(timezone.utc).isoformat(), 'stableDownloadSHA256': stable, 'stableDownloadUpdated': False,
                'productionDeployed': False, 'localApplicationStarted': True, 'linuxRuntimeTested': False,
                'catalog': {'vendorRoots': 14, 'productRoots': 10, 'productSecondLevel': 35, 'oldMappingCount': 134},
                'validation': validation, 'mapPreviewPackaged': False,
                'unverified': ['physical-device installation and native keyboard/camera/permissions', 'iOS build', 'Linux host runtime']}
    json_write(STAGE / 'manifest.json', manifest)
    (STAGE / 'SHA256SUMS').write_text(''.join(sha(p) + '  ' + p.relative_to(STAGE).as_posix() + '\n' for p in sorted(STAGE.rglob('*')) if p.is_file() and p.name != 'SHA256SUMS'), encoding='utf-8')
    server_zip = OUT / 'compact-taxonomy-server-update.zip'
    zip_files(server_zip, (p for p in STAGE.rglob('*') if p.is_file()), STAGE)
    sources = set(base['source_files']())
    sources.update((ROOT / 'docs').glob('agriculture-taxonomy*.json'))
    sources.update((ROOT / 'docs/test-output').glob('compact-*'))
    sources.update((ROOT / 'docs/screenshots/compact-taxonomy').glob('*.png'))
    source_zip = OUT / 'compact-taxonomy-source.zip'
    zip_files(source_zip, sources, ROOT)
    for target in [source_zip, server_zip]:
        with zipfile.ZipFile(target) as archive:
            assert not any('/map-preview/' in n or 'factory-map-outline' in n or '/previews/' in n or re.search(r'(^|/)(\.env|key\.properties|local\.properties|signing\.json)$|\.(jks|keystore|zqbak|db|sqlite)$', n) for n in archive.namelist())
    manifest['sourceFiles'] = len(sources)
    manifest['artifacts'] = [{'file': p.relative_to(OUT).as_posix(), 'bytes': p.stat().st_size, 'sha256': sha(p)} for p in [apk, source_zip, server_zip]]
    json_write(OUT / 'manifest.json', manifest)
    (OUT / 'SHA256SUMS.txt').write_text(''.join(sha(p) + '  ' + p.relative_to(OUT).as_posix() + '\n' for p in sorted(OUT.rglob('*')) if p.is_file() and p.name != 'SHA256SUMS.txt'), encoding='utf-8')
    print(json.dumps({'directory': str(OUT), 'validation': validation, 'artifacts': manifest['artifacts']}, ensure_ascii=False, indent=2))


if __name__ == '__main__':
    main()
