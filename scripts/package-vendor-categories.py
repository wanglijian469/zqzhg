"""Build verified v1.11.0 delivery archives from committed sources and signed APK."""
from datetime import datetime, timezone
from pathlib import Path
import hashlib
import json
import shutil
import subprocess
import tarfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
WORK = ROOT / '.local/release-1.11.0'
OUT = ROOT / 'delivery/release-v1.11.0'
NAME = 'zqzg-v1.11.0'
STAGE = WORK / 'package-final' / NAME


def sha(file):
    digest = hashlib.sha256()
    with file.open('rb') as stream:
        for block in iter(lambda: stream.read(1048576), b''):
            digest.update(block)
    return digest.hexdigest()


def inventory(folder):
    return {file.relative_to(folder).as_posix(): sha(file) for file in sorted(folder.rglob('*')) if file.is_file()}


def main():
    subprocess.run(['git', 'diff', '--exit-code', 'HEAD'], cwd=ROOT, stdout=subprocess.DEVNULL, check=True)
    commit = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
    assert not STAGE.exists(), 'package_stage_already_exists'
    OUT.mkdir(parents=True, exist_ok=True)
    binary = WORK / 'linux-amd64/exhibition'
    metadata = subprocess.check_output(['go', 'version', '-m', str(binary)], text=True)
    for expected in ['vcs.revision=' + commit, 'vcs.modified=false', 'GOOS=linux', 'GOARCH=amd64', 'CGO_ENABLED=0']:
        assert expected in metadata, 'binary_provenance_' + expected
    assert binary.read_bytes()[:6] == b'\x7fELF\x02\x01'
    (STAGE / 'server').mkdir(parents=True)
    shutil.copy2(binary, STAGE / 'server/exhibition')
    for folder in ['migrations', 'catalog', 'licenses']:
        shutil.copytree(ROOT / 'server' / folder, STAGE / 'server' / folder)
    (STAGE / 'web/dist').mkdir(parents=True)
    for entry in ['index.html', 'brand.svg', 'ByteDanceVerify.html', 'assets', 'pdfjs', 'maps', 'guide-images']:
        src, dest = ROOT / 'web/dist' / entry, STAGE / 'web/dist' / entry
        if src.is_dir():
            shutil.copytree(src, dest)
        else:
            shutil.copy2(src, dest)
    assert not (STAGE / 'web/dist/downloads').exists()
    shutil.copytree(ROOT / '.local/map-preview', STAGE / 'map-demo')
    maps = inventory(STAGE / 'map-demo')
    reference = json.loads((WORK / 'inventory.json').read_text(encoding='utf-8-sig'))['map_files']
    assert len(maps) == 77 and all(reference.get(name) == digest for name, digest in maps.items())
    apk_dir = ROOT / 'delivery/android-v1.11.0'
    android = json.loads((apk_dir / 'manifest.json').read_text(encoding='utf-8-sig'))
    apk = apk_dir / '大陆农机-1.11.0+51.apk'
    assert android['sha256'] == sha(apk)
    assert android['certificateSHA256'] == '85656c85541b188c832dc48d67cbf320046a4311aa9e0fefcc662dbe2e0f1585'
    assert android['version'] == '1.11.0+51' and android['apiBaseUrl'] == 'https://zqzhg.com'
    android.update(versionName='1.11.0', versionCode=51, asset='zqzhg-1.11.0-51.apk', sourceCommit=commit,
                   sourceTag='v1.11.0', schemaVersion=31, bytes=apk.stat().st_size,
                   officialDownloadURL='https://zqzhg.com/downloads/dalunongji.apk',
                   productionDeployed=True, releaseStatus='stable', endpointEditable=False,
                   deviceValidation=False, releaseBasis='User-authorized release; build, signature, component and production-proxy checks; no new physical-device acceptance.')
    (STAGE / 'android').mkdir()
    shutil.copy2(apk, STAGE / 'android/dalunongji.apk')
    with zipfile.ZipFile(apk) as bundle:
        embedded = json.loads(bundle.read('assets/workspace/bundle-manifest.json'))
        for item in embedded['files']:
            data = bundle.read('assets/workspace/' + item['path'])
            assert len(data) == item['bytes'] and hashlib.sha256(data).hexdigest() == item['sha256']
            assert data == (ROOT / 'web/dist' / item['path']).read_bytes(), 'web_app_asset_mismatch_' + item['path']
        android['embeddedWebAssetsVerified'] = len(embedded['files'])
        for abi in ['arm64-v8a', 'armeabi-v7a', 'x86_64']:
            assert b'https://zqzhg.com' in bundle.read('lib/' + abi + '/libapp.so')
    (STAGE / 'android/manifest.json').write_text(json.dumps(android, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    for source, target in [('deploy/linux/upgrade-vendor-categories.py', 'upgrade.py'),
                           ('releases/v1.11.0/README.md', 'README.md'),
                           ('docs/VENDOR_PRODUCT_CATEGORIES_20261115.md', 'VENDOR_PRODUCT_CATEGORIES.md'),
                           ('docs/MAP_DEPLOYMENT_REQUIREMENTS.md', 'MAP_DEPLOYMENT_REQUIREMENTS.md')]:
        (STAGE / target).write_bytes((ROOT / source).read_bytes().replace(b'\r\n', b'\n'))
    manifest = dict(version='1.11.0', gitCommit=commit, schemaVersion=31, architecture='linux-amd64',
                    createdUTC=datetime.now(timezone.utc).isoformat(), binarySHA256=sha(binary), mapFiles=maps,
                    mapUsage='development-demonstration', mapMissingCodes=['130571'], android=android,
                    databaseMigrationAdded=True, downloadPolicy='archive-old-promote-new-signed-app',
                    includesBusinessDatabase=False, includesPrivateMedia=False, includesCredentials=False)
    (STAGE / 'manifest.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    hashes = inventory(STAGE)
    (STAGE / 'SHA256SUMS').write_bytes(''.join(digest + '  ' + name + '\n' for name, digest in hashes.items()).encode())
    archive = OUT / (NAME + '-linux-amd64.tar.gz')
    with tarfile.open(archive, 'x:gz') as out:
        for file in sorted(p for p in STAGE.rglob('*') if p.is_file()):
            member = out.gettarinfo(str(file), arcname=NAME + '/' + file.relative_to(STAGE).as_posix())
            member.uid = member.gid = 0
            member.uname = member.gname = ''
            member.mode = 0o755 if file == STAGE / 'server/exhibition' or file.name == 'upgrade.py' else 0o644
            with file.open('rb') as stream:
                out.addfile(member, stream)
    with tarfile.open(archive) as bundle:
        assert set(bundle.getnames()) == {NAME + '/' + name for name in list(hashes) + ['SHA256SUMS']}
        for name, digest in hashes.items():
            assert hashlib.sha256(bundle.extractfile(NAME + '/' + name).read()).hexdigest() == digest
    source_zip = OUT / (NAME + '-source.zip')
    subprocess.run(['git', 'archive', '--format=zip', '--prefix=' + NAME + '-source/', '--output=' + str(source_zip), commit], cwd=ROOT, check=True)
    with zipfile.ZipFile(source_zip) as bundle:
        assert bundle.testzip() is None
        assert not any('/.local/' in n or n.endswith(('/.env', '/key.properties', '/local.properties', '.jks', '.keystore', '/39.97.250.244.txt')) for n in bundle.namelist())
    delivered_apk = OUT / android['asset']
    shutil.copy2(apk, delivered_apk)
    shutil.copy2(STAGE / 'manifest.json', OUT / 'manifest-v1.11.0.json')
    shutil.copy2(ROOT / 'releases/v1.11.0/README.md', OUT / 'README.md')
    assets = [archive, source_zip, delivered_apk, OUT / 'manifest-v1.11.0.json', OUT / 'README.md']
    (OUT / 'SHA256SUMS.txt').write_bytes(''.join(sha(p) + '  ' + p.name + '\n' for p in assets).encode())
    receipt = {'version': '1.11.0', 'commit': commit, 'package': str(archive), 'mapFiles': len(maps), 'embeddedWebAssets': android['embeddedWebAssetsVerified'],
               'assets': [{'path': str(p.relative_to(ROOT)), 'sha256': sha(p), 'bytes': p.stat().st_size} for p in assets]}
    (WORK / 'package-receipt.json').write_text(json.dumps(receipt, indent=2) + '\n', encoding='utf-8')
    print(json.dumps(receipt, ensure_ascii=False))


if __name__ == '__main__':
    main()
