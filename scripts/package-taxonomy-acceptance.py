"""Package the locally validated Schema17 upgrade, without publishing or private state.

Prerequisites: final logs under .local/taxonomy-*, compiled backend binaries under
.local/taxonomy-build, final web assets under .local/taxonomy-dist and signed APK.
Run with Python 3 from any directory. Existing stable downloads are read only.
"""
import hashlib
import json
import re
import shutil
import struct
import subprocess
import zipfile
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / 'delivery/agriculture-taxonomy-20261020'
STAGE = ROOT / '.local/taxonomy-server-bundle'
CERT = '85656c85541b188c832dc48d67cbf320046a4311aa9e0fefcc662dbe2e0f1585'


def read(name):
    return (ROOT / name).read_text(encoding='utf-8-sig')


def sha(path):
    result = hashlib.sha256()
    with path.open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            result.update(chunk)
    return result.hexdigest()


def copy(source, target):
    target.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(source, target)


def json_write(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')


def assert_go(name):
    value = read(name)
    assert re.search(r'^ok\s', value, re.M) and not re.search(r'^--- FAIL|^FAIL', value, re.M), name
    return len(re.findall(r'^--- PASS:', value, re.M)), len(re.findall(r'^--- SKIP:', value, re.M))


def source_files():
    # Git's inventory excludes ignored dependencies, builds, private state and downloads.
    names = subprocess.check_output(['git', 'ls-files', '-z', '--cached', '--others', '--exclude-standard'], cwd=ROOT).decode('utf-8').strip('\0').split('\0')
    for name in names:
        p = Path(name)
        allowed = p.parts[0] in {'web', 'server', 'mobile', 'scripts', 'deploy'}
        allowed |= name in {'README.md', 'LICENSE', '.gitignore', '.gitattributes', 'docs/openapi.json', 'docs/agriculture-taxonomy-catalog.json'}
        allowed |= p.parent.as_posix() == 'docs' and p.suffix == '.md'
        allowed |= name.startswith(('docs/test-output/taxonomy-', 'docs/screenshots/taxonomy/'))
        if not allowed or not (ROOT / p).is_file():
            continue
        assert not re.search(r'(^|/)(\.env(?!\.example$)[^/]*|key\.properties|local\.properties|signing\.json)$|\.(jks|keystore|p12|pfx|pem|key|zqbak|db|sqlite|apk|exe)$|(^|/)(\.local|data|node_modules|build)/', name, re.I), name
        yield ROOT / p


def zip_files(target, files, base):
    with zipfile.ZipFile(target, 'w', zipfile.ZIP_DEFLATED, compresslevel=6) as archive:
        for path in sorted(files):
            archive.write(path, path.relative_to(base).as_posix())
    with zipfile.ZipFile(target) as archive:
        assert archive.testzip() is None, target


def main():
    passed, skipped = assert_go('.local/taxonomy-go-verified.txt')
    video_passed, _ = assert_go('.local/taxonomy-go-video.txt')
    assert_go('.local/taxonomy-browser-final.txt')
    assert not read('.local/taxonomy-go-vet-final.txt').strip(), 'go vet'
    web = json.loads(read('.local/taxonomy-web-final.json'))
    assert web['numFailedTests'] == 0 and web['numPassedTests'] == 189
    assert 'built in' in read('.local/taxonomy-web-build.txt')
    native = read('.local/taxonomy-flutter-final2.txt')
    assert '+157 ~7:' in native and ('All other tests passed!' in native or 'All tests passed!' in native)
    assert 'No issues found!' in read('.local/taxonomy-flutter-analyze-final.txt')
    assert 'BUILD SUCCESSFUL' in read('.local/taxonomy-gradle-direct.txt')
    browser = json.loads(read('docs/test-output/taxonomy-browser.json'))
    assert len(browser['checks']) == 6 and len(browser['layouts']) == 24
    assert not browser['errors'] and not any(x['overflow'] for x in browser['layouts'])
    catalog = json.loads(read('docs/agriculture-taxonomy-catalog.json'))
    assert (catalog['vendor_roots'], catalog['product_roots'], catalog['product_second_level']) == (14, 20, 114)
    assert len(catalog['categories']) == 148
    assert CERT in read('.local/taxonomy-apk-signature.txt')
    assert CERT in read('.local/taxonomy-apk-previous-signature.txt')
    metadata = read('docs/test-output/taxonomy-apk-metadata.txt')
    assert "name='com.zqzt.dalunongji' versionCode='31' versionName='1.8.0-rc.2'" in metadata
    stable = ROOT / 'delivery/android/dalunongji.apk'
    stable_sha = sha(stable)
    receipt = json.loads(read('.local/android-preview-verification.json'))
    assert stable_sha == receipt['stableSHA256']
    assert sha(ROOT / 'web/public/downloads/dalunongji.apk') == stable_sha
    assert not json.loads(read('web/public/maps/manifest.json'))['resources']
    OUT.mkdir(parents=True, exist_ok=True)
    assert not STAGE.exists(), 'Use a new staging directory; do not silently mix releases.'
    STAGE.mkdir(parents=True)
    log_names = {
        'go-verified.txt': 'backend-tests.txt', 'go-video.txt': 'video-tests.txt',
        'go-vet-final.txt': 'go-vet.txt', 'browser-final.txt': 'browser-tests.txt',
        'web-build.txt': 'web-build.txt', 'web-final.json': 'web-tests.json',
        'flutter-final2.txt': 'flutter-tests.txt', 'flutter-analyze-final.txt': 'flutter-analyze.txt',
        'flutter-layout.txt': 'flutter-layout.txt',
        'gradle-direct.txt': 'android-build.txt', 'apk-signature.txt': 'apk-signature.txt',
        'apk-previous-signature.txt': 'previous-apk-signature.txt',
    }
    for source, target in log_names.items():
        copy(ROOT / '.local' / ('taxonomy-' + source), ROOT / 'docs/test-output' / ('taxonomy-' + target))
    validation = {'backendPassed': passed + video_passed, 'backendSkipped': skipped, 'backendVideoSeparate': True,
                  'webPassed': 189, 'webFiles': len(web['testResults']), 'flutterPassed': 157, 'flutterSkipped': 7,
                  'nativeTaxonomyLayouts': 64, 'keyboardVerification': 'simulated view insets, no physical device',
                  'browserFlows': 6, 'browserLayouts': 24, 'browserErrors': [], 'goVetClean': True,
                  'flutterAnalyzeClean': True, 'webBuild': True, 'androidBuild': 'Gradle assembleRelease --no-daemon --offline'}
    json_write(ROOT / 'docs/test-output/taxonomy-validation.json', validation)
    for p in (ROOT / 'docs/test-output').glob('taxonomy-*'):
        copy(p, OUT / 'verification' / p.name)
    for p in (ROOT / 'docs/screenshots/taxonomy').glob('*.png'):
        assert p.stem in {'company-1440', 'company-390', 'product-1440', 'product-390', 'admin-1440', 'admin-390'}, p
        copy(p, OUT / 'screenshots' / p.name)
    for name in ['AGRICULTURE_TAXONOMY.md', 'AGRICULTURE_TAXONOMY_MIGRATION.md', 'agriculture-taxonomy-catalog.json', 'openapi.json']:
        copy(ROOT / 'docs' / name, OUT / 'docs' / name)
    copy(ROOT / 'docs/AGRICULTURE_TAXONOMY.md', OUT / 'README.md')
    apk = OUT / 'android/dalunongji-1.8.0-rc.2+31.apk'
    copy(ROOT / 'mobile/build/app/outputs/flutter-apk/app-release.apk', apk)
    with zipfile.ZipFile(apk) as archive:
        assert not any('map-preview' in n or '/maps/' in n for n in archive.namelist())
    for arch, machine in [('amd64', 62), ('arm64', 183)]:
        binary = ROOT / '.local/taxonomy-build' / ('exhibition-linux-' + arch)
        header = binary.read_bytes()[:20]
        assert header[:4] == b'\x7fELF' and struct.unpack('<H', header[18:20])[0] == machine
        copy(binary, STAGE / 'server' / binary.name)
    binary = ROOT / '.local/taxonomy-build/exhibition-windows-amd64.exe'
    assert binary.open('rb').read(2) == b'MZ'
    copy(binary, STAGE / 'server' / binary.name)
    for p in (ROOT / '.local/taxonomy-dist').rglob('*'):
        relative = p.relative_to(ROOT / '.local/taxonomy-dist')
        if p.is_file() and 'previews' not in relative.parts:
            copy(p, STAGE / 'web/dist' / relative)
    for directory in ['migrations', 'catalog', 'licenses']:
        for p in (ROOT / 'server' / directory).rglob('*'):
            if p.is_file():
                copy(p, STAGE / directory / p.relative_to(ROOT / 'server' / directory))
    copy(ROOT / 'deploy/linux/upgrade-storage.sh', STAGE / 'upgrade.sh')
    copy(ROOT / 'deploy/linux/aliyun-upgrade-support.py', STAGE / 'aliyun-upgrade-support.py')
    copy(ROOT / 'docs/AGRICULTURE_TAXONOMY.md', STAGE / 'README.md')
    copy(ROOT / 'docs/AGRICULTURE_TAXONOMY_MIGRATION.md', STAGE / 'MIGRATION.md')
    manifest = {'release': 'agriculture-taxonomy-20261020', 'schema': 17, 'appVersion': '1.8.0-rc.2+31',
                'package': 'com.zqzt.dalunongji', 'certificateSHA256': CERT, 'signingMatchesPrevious': True,
                'builtAt': datetime.now(timezone.utc).isoformat(), 'stableDownloadSHA256': stable_sha,
                'stableDownloadUpdated': False, 'productionDeployed': False, 'businessDatabaseMigrated': False,
                'linuxRuntimeTested': False, 'nativeDeviceVerified': False, 'mapResourceCount': 0,
                'catalog': {'vendorRoots': 14, 'productRoots': 20, 'productSecondLevel': 114}, 'validation': validation,
                'migrationValidation': {'isolatedDatabase': True, 'sampleHistoricalRevisions': 3,
                                        'automaticMappings': 1, 'pendingBeforeManual': 2, 'manualConfirmed': 2,
                                        'rawSnapshotsPreserved': True, 'restartAndRepeatVerified': True},
                'unverified': ['Android/iOS physical device and cover installation', 'system keyboard, camera and permissions on devices', 'Linux host runtime/install', 'iOS build']}
    json_write(STAGE / 'manifest.json', manifest)
    (STAGE / 'SHA256SUMS').write_text(''.join(sha(p) + '  ' + p.relative_to(STAGE).as_posix() + '\n' for p in sorted(STAGE.rglob('*')) if p.is_file() and p.name != 'SHA256SUMS'), encoding='utf-8')
    server_zip = OUT / 'agriculture-taxonomy-server-update.zip'
    zip_files(server_zip, (p for p in STAGE.rglob('*') if p.is_file()), STAGE)
    sources = list(source_files())
    source_zip = OUT / 'agriculture-taxonomy-source.zip'
    zip_files(source_zip, sources, ROOT)
    manifest['sourceFiles'] = len(sources)
    manifest['artifacts'] = [{'file': p.relative_to(OUT).as_posix(), 'bytes': p.stat().st_size, 'sha256': sha(p)} for p in [apk, source_zip, server_zip]]
    json_write(OUT / 'manifest.json', manifest)
    (OUT / 'SHA256SUMS.txt').write_text(''.join(sha(p) + '  ' + p.relative_to(OUT).as_posix() + '\n' for p in sorted(OUT.rglob('*')) if p.is_file() and p.name != 'SHA256SUMS.txt'), encoding='utf-8')
    print(json.dumps({'directory': str(OUT), 'validation': validation, 'sourceFiles': len(sources), 'artifacts': manifest['artifacts']}, ensure_ascii=False, indent=2))


if __name__ == '__main__':
    main()
