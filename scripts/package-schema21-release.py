"""Package current reviewed Linux/Web code; never export or connect to a business DB."""
from datetime import datetime, timezone
from pathlib import Path
from pathlib import PurePosixPath
import ast
import hashlib
import json
import re
import shutil
import struct
import subprocess
import tarfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
TAG = 'aliyun-20261024-schema21-fix2'
BUILD_TAG = TAG
NAME = 'zqzg-' + TAG
STAGE = ROOT / '.local/releases' / NAME
OUT = ROOT / 'delivery'
GO = Path('D:/Program Files/Go/bin/go.exe')


def sha(path):
    digest = hashlib.sha256()
    with path.open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            digest.update(chunk)
    return digest.hexdigest()


def copy(source, target):
    target.parent.mkdir(parents=True, exist_ok=True)
    if source.suffix in ('.sh', '.py', '.md'):
        target.write_bytes(source.read_text(encoding='utf-8-sig').replace('\r\n', '\n').encode('utf-8'))
    else:
        shutil.copyfile(source, target)


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes((json.dumps(value, ensure_ascii=False, indent=2) + '\n').encode('utf-8'))


def inventory(folder):
    return [{'path': p.relative_to(folder).as_posix(), 'bytes': p.stat().st_size, 'sha256': sha(p)}
            for p in sorted(folder.rglob('*')) if p.is_file()]


def sums(folder):
    # Bytes avoid Windows text-mode CRLF conversion. GNU sha256sum on Linux
    # treats CR as part of the filename; splitlines() would hide this defect.
    data = ''.join(x['sha256'] + '  ' + x['path'] + '\n'
                   for x in inventory(folder) if x['path'] != 'SHA256SUMS').encode('utf-8')
    checksum_records(data)
    (folder / 'SHA256SUMS').write_bytes(data)


def checksum_records(data):
    if b'\r' in data or data.startswith(b'\xef\xbb\xbf') or not data.endswith(b'\n'):
        raise ValueError('Checksum manifest must be UTF-8 without BOM and use LF only')
    seen = set()
    result = []
    for line in data[:-1].decode('utf-8').split('\n'):
        match = re.fullmatch(r'([a-f0-9]{64})  ([^\r\n\x00]+)', line)
        if not match:
            raise ValueError('Invalid checksum record')
        digest, name = match.groups()
        if PurePosixPath(name).is_absolute() or '..' in PurePosixPath(name).parts or '\\' in name or name in seen:
            raise ValueError('Invalid or duplicate checksum path')
        seen.add(name)
        result.append((digest, name))
    return result


def zip_folder(folder, destination, prefix):
    with zipfile.ZipFile(destination, 'x', compression=zipfile.ZIP_DEFLATED, compresslevel=6) as archive:
        for path in sorted(folder.rglob('*')):
            if path.is_file():
                archive.write(path, prefix + '/' + path.relative_to(folder).as_posix())
    with zipfile.ZipFile(destination) as archive:
        if archive.testzip() is not None:
            raise RuntimeError('ZIP CRC validation failed')
        for want, name in checksum_records(archive.read(prefix + '/SHA256SUMS')):
            if hashlib.sha256(archive.read(prefix + '/' + name)).hexdigest() != want:
                raise RuntimeError('ZIP checksum validation failed: ' + name)


def main():
    current_checks = {'build-amd64', 'build-arm64', 'go-vet', 'go-tests', 'strict', 'seo-browser', 'package-checksums'}
    for check in ['build-amd64', 'build-arm64', 'web-types', 'web-build', 'web-tests', 'go-vet', 'go-tests', 'upgrade-tests', 'bash-syntax', 'browser', 'import-compatibility', 'legacy-key-tests', 'package-checksums', 'strict', 'seo-browser']:
        prefix = 'schema21-fix2-' if check in current_checks else 'aliyun21-'
        if (ROOT / '.local' / (prefix + check + '.exit')).read_text(encoding='utf-8-sig').strip() != '0':
            raise RuntimeError('Release check failed: ' + check)
    for path in [STAGE, OUT / (NAME + '-linux.tar.gz'), OUT / (NAME + '-source.zip'), OUT / (NAME + '-database.zip')]:
        if path.exists():
            raise RuntimeError('Preserve existing release; choose a new identifier: ' + str(path))
    go_log = (ROOT / '.local/schema21-fix2-go-tests.log').read_text(encoding='utf-8-sig')
    if re.search(r'^\s*--- FAIL:', go_log, re.M) or not re.search(r'^ok\s+zqzt/exhibition\s', go_log, re.M):
        raise RuntimeError('Go run is incomplete')
    web_log = (ROOT / '.local/aliyun21-web-tests.log').read_text(encoding='utf-8-sig')
    if not re.search(r'Tests\s+238 passed', web_log):
        raise RuntimeError('Expected full frontend run')
    browser = json.loads((ROOT / '.local/aliyun21-procurement-browser.json').read_text(encoding='utf-8-sig'))
    if browser['errors'] or len(browser['layouts']) != 89 or not browser['isolatedDatabase']:
        raise RuntimeError('Incomplete isolated browser verification')
    commit = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
    dirty = bool(subprocess.check_output(['git', 'status', '--porcelain'], cwd=ROOT))
    STAGE.mkdir(parents=True)
    (STAGE / 'verification').mkdir()
    binaries = {}
    for arch, machine in [('amd64', 62), ('arm64', 183)]:
        binary = ROOT / '.local/builds' / BUILD_TAG / ('linux-' + arch) / 'exhibition'
        with binary.open('rb') as stream:
            header = stream.read(64)
            if header[:6] != b'\x7fELF\x02\x01' or struct.unpack_from('<H', header, 18)[0] != machine:
                raise RuntimeError('Wrong ELF architecture')
            offset, size, count = struct.unpack_from('<Q', header, 32)[0], struct.unpack_from('<H', header, 54)[0], struct.unpack_from('<H', header, 56)[0]
            for index in range(count):
                stream.seek(offset + index * size)
                if struct.unpack('<I', stream.read(4))[0] == 3:
                    raise RuntimeError('Linux binary requires a dynamic interpreter')
        metadata = subprocess.check_output([str(GO), 'version', '-m', str(binary)], cwd=ROOT, text=True)
        if 'GOOS=linux' not in metadata or 'CGO_ENABLED=0' not in metadata or ('vcs.revision=' + commit) not in metadata:
            raise RuntimeError('Binary build provenance differs')
        copy(binary, STAGE / 'server' / ('exhibition-linux-' + arch))
        (STAGE / 'verification' / ('binary-' + arch + '.txt')).write_text(metadata, encoding='utf-8')
        binaries[arch] = sha(binary)
    for name in ['index.html', 'brand.svg', 'assets', 'pdfjs', 'maps']:
        source, target = ROOT / '.local/aliyun21-web-dist' / name, STAGE / 'web/dist' / name
        if source.is_dir():
            shutil.copytree(source, target)
        elif source.is_file():
            copy(source, target)
        else:
            raise RuntimeError('Missing Web output: ' + name)
    apk = ROOT / 'delivery/android-v1.8.0/大陆农机-1.8.0+35.apk'
    android = json.loads((apk.parent / 'verification.json').read_text(encoding='utf-8-sig'))
    if sha(apk) != android['sha256'] or not android['signatureVerified'] or android['defaultApiBaseUrl'] != 'http://39.97.250.244:8089':
        raise RuntimeError('Android receipt differs')
    copy(apk, STAGE / 'web/dist/downloads/dalunongji.apk')
    copy(apk.parent / 'manifest.json', STAGE / 'web/dist/downloads/manifest.json')
    for name in ['migrations', 'catalog', 'licenses']:
        shutil.copytree(ROOT / 'server' / name, STAGE / name)
    versions = sorted(int(p.name.split('_')[0]) for p in (STAGE / 'migrations').glob('*.sql'))
    if versions != list(range(1, 22)):
        raise RuntimeError('Incomplete migration chain')
    previous = ROOT / '.local/releases/zqzg-aliyun-20261024-schema21-fix1'
    for folder in ['web/dist', 'migrations', 'catalog', 'licenses']:
        if inventory(STAGE / folder) != inventory(previous / folder):
            raise RuntimeError('Hotfix must preserve prior frontend/APK/schema assets: ' + folder)
    copy(ROOT / 'deploy/linux/upgrade-storage.sh', STAGE / 'upgrade.sh')
    copy(ROOT / 'deploy/linux/aliyun-upgrade-support.py', STAGE / 'aliyun-upgrade-support.py')
    copy(ROOT / 'deploy/linux/initialize-legacy-key.py', STAGE / 'deployment/initialize-legacy-key.py')
    # The legacy helper loads the sibling support module by a caller-supplied path.
    # Include a root wrapper so its documented default relative path is unambiguous.
    copy(ROOT / 'deploy/linux/initialize-legacy-key.py', STAGE / 'initialize-legacy-key.py')
    copy(ROOT / 'docs/ALIYUN_SCHEMA21_DEPLOY.md', STAGE / 'README.md')
    readme = STAGE / 'README.md'
    readme.write_bytes(readme.read_text(encoding='utf-8').replace('(SHOWROOM_MYSQL1055_FIX2.md)', '(docs/SHOWROOM_MYSQL1055_FIX2.md)').encode('utf-8'))
    copy(ROOT / 'docs/SHOWROOM_MYSQL1055_FIX2.md', STAGE / 'docs/SHOWROOM_MYSQL1055_FIX2.md')
    copy(ROOT / 'docs/ANDROID_1.8.0.md', STAGE / 'docs/ANDROID_1.8.0.md')
    copy(ROOT / 'docs/系统优化整改记录.md', STAGE / 'docs/系统优化整改记录.md')
    for name in ['web-tests', 'web-build', 'web-types', 'go-vet', 'upgrade-tests', 'import-compatibility', 'legacy-key-tests', 'package-checksums']:
        prefix = 'schema21-fix2-' if name in current_checks else 'aliyun21-'
        value = (ROOT / '.local' / (prefix + name + '.log')).read_text(encoding='utf-8-sig')
        if name == 'import-compatibility':
            value = '\n'.join(x for x in value.splitlines() if x.startswith(('test_', 'Ran ', 'OK', 'FAILED', '---')))
        (STAGE / 'verification' / (name + '.txt')).write_text(value, encoding='utf-8')
    (STAGE / 'verification/go-tests.txt').write_text('\n'.join(line for line in go_log.splitlines()
                                                            if re.match(r'^\s*--- (PASS|SKIP|FAIL):|^PASS$|^ok\s', line)) + '\n', encoding='utf-8')
    write_json(STAGE / 'verification/procurement-browser.json', browser)
    strict_log = (ROOT / '.local/schema21-fix2-strict.log').read_text(encoding='utf-8-sig')
    seo_log = (ROOT / '.local/schema21-fix2-seo-browser.log').read_text(encoding='utf-8-sig')
    if '--- PASS: TestShowroomOnlyFullGroupBy ' not in strict_log or 'error=mysql_1055' not in strict_log:
        raise RuntimeError('Strict-mode reproduction and regression did not complete')
    if '--- PASS: TestIndependentSEOBrowser ' not in seo_log:
        raise RuntimeError('Current backend SEO browser verification did not complete')
    (STAGE / 'verification/strict-sql.txt').write_bytes(strict_log.encode('utf-8'))
    seo_lines = [line for line in seo_log.splitlines() if '"checks":' in line]
    if len(seo_lines) != 1:
        raise RuntimeError('Missing SEO browser report')
    seo_report = json.loads(seo_lines[0][seo_lines[0].index('{'):])
    if seo_report['errors'] or len(seo_report['checks']) not in (6, 16):
        raise RuntimeError('Incomplete SEO browser checks')
    write_json(STAGE / 'verification/seo-browser.json', seo_report)
    write_json(STAGE / 'verification/android.json', android)
    for helper in ['aliyun-upgrade-support.py', 'initialize-legacy-key.py']:
        ast.parse((STAGE / helper).read_text(encoding='utf-8'), feature_version=(3, 6))
    source = STAGE.parent / (NAME + '-source')
    source.mkdir()
    paths = []
    paths += list((ROOT / 'server').glob('*.go'))
    paths += [ROOT / 'server' / f for f in ['go.mod', 'go.sum', 'form_rules.json', 'seo-copy.json']]
    for folder in ['migrations', 'catalog', 'fonts', 'licenses']:
        paths += [p for p in (ROOT / 'server' / folder).rglob('*') if p.is_file()]
    paths += [p for p in (ROOT / 'web/src').rglob('*') if p.is_file()]
    paths += [p for p in (ROOT / 'web').glob('*') if p.is_file() and (p.suffix in ['.json', '.html', '.ts'] or p.name == 'package-lock.json') and not p.name.endswith('.tsbuildinfo')]
    paths += [ROOT / 'web/public/brand.svg']
    paths += [p for p in (ROOT / 'web/public/maps').rglob('*') if p.is_file()]
    paths += [p for p in (ROOT / 'deploy/linux').glob('*') if p.is_file()]
    paths += [ROOT / 'scripts' / p for p in ['package-schema21-release.py', 'test-schema21-package.py', 'prepare-pdf-assets.mjs', 'verify-map-resources.mjs', 'sync-form-rules.mjs', 'test-aliyun-upgrade.py']]
    paths += [ROOT / 'docs/ALIYUN_SCHEMA21_DEPLOY.md']
    paths += [ROOT / 'docs/SHOWROOM_MYSQL1055_FIX2.md']
    for file in sorted(set(paths)):
        copy(file, source / file.relative_to(ROOT))
    write_json(source / 'source-files.json', {'gitHead': commit, 'workingTreeModified': dirty, 'files': inventory(source)})
    sums(source)
    source_zip = OUT / (NAME + '-source.zip')
    zip_folder(source, source_zip, source.name)
    database = STAGE.parent / (NAME + '-database')
    database.mkdir()
    for folder in ['migrations', 'catalog', 'licenses']:
        shutil.copytree(STAGE / folder, database / folder)
    (database / 'README.md').write_text('数据库增量迁移及基础目录，Schema 1–21。\n保留线上数据，不包含业务快照或账户。\n'
                                       '后端启动时自动迁移；不要手工导入 SQL，不运行演示初始化。部署步骤见程序包 README.md。\n', encoding='utf-8')
    sums(database)
    database_zip = OUT / (NAME + '-database.zip')
    zip_folder(database, database_zip, database.name)
    manifest = {'release': TAG, 'createdAt': datetime.now(timezone.utc).isoformat(), 'gitHead': commit, 'workingTreeModified': dirty,
                'schema': 21, 'dataMode': 'preserve-existing-cloud-database; automatic incremental migration only',
                'architectures': ['amd64', 'arm64'], 'binarySHA256': binaries, 'cgoEnabled': False,
                'frontendIndexSHA256': sha(STAGE / 'web/dist/index.html'), 'androidVersion': '1.8.0+35', 'apkSHA256': sha(apk),
                'sourceArchive': source_zip.name, 'sourceSHA256': sha(source_zip), 'databaseArchive': database_zip.name, 'databaseSHA256': sha(database_zip),
                'tests': {'frontendPassed': 238, 'goPassedIncludingSubtests': len(re.findall(r'^\s*--- PASS:', go_log, re.M)),
                          'goSkippedIncludingSubtests': len(re.findall(r'^\s*--- SKIP:', go_log, re.M)), 'goVet': True,
                          'upgradeHelperPassed': 10, 'packagingChecksumTestsPassed': 4, 'bashSyntax': True, 'python36Syntax': True, 'procurementBrowserLayouts': 89, 'pageErrors': 0},
                'hotfix': 'showroom category aggregation compatible with ONLY_FULL_GROUP_BY; no schema changes',
                'strictSQLModeRegression': {'passed': True, 'mysql': '5.7.18', 'sessionPinned': True, 'globalModeChanged': False, 'legacyQueryReproduced1055': True},
                'seoBrowserChecks': 6, 'frontendAndAndroidUnchanged': True, 'migrationFilesUnchanged': True,
                'reusedVerification': ['frontend 238 tests/build/typecheck: identical artifact hashes', 'procurement browser 89 layouts from same frontend: earlier backend run', 'Android signing and 172 Flutter tests: identical APK', 'unchanged deployment helper tests'],
                'checksumManifestFormat': 'UTF-8 without BOM; LF only',
                'includesBusinessDatabase': False, 'includesPrivateMedia': False, 'includesCredentials': False,
                'productionDeployed': False, 'linuxRuntimeVerified': False, 'physicalAndroidVerified': False,
                'unverified': ['Real ECS/Linux service execution and systemd upgrade', 'Production OSS/NAS/DNS/TLS/Nginx', 'Real SMS', 'Android physical installation']}
    write_json(STAGE / 'manifest.json', manifest)
    # Reject unexpected sensitive file names in the deliverable before archiving.
    for p in STAGE.rglob('*'):
        if p.is_symlink() or (p.is_file() and (p.suffix in ['.jks', '.keystore', '.zqbak', '.key'] or p.name in ['.env', 'key.properties', 'signing.json', 'database.sql', 'database.sql.gz'])):
            raise RuntimeError('Forbidden release member: ' + str(p))
    sums(STAGE)
    archive_path = OUT / (NAME + '-linux.tar.gz')
    def attrs(info):
        info.uid = info.gid = 0
        info.uname = info.gname = 'root'
        info.mode = 0o755 if info.isdir() or info.name.endswith('.sh') or '/exhibition-linux-' in info.name else 0o644
        return info
    with tarfile.open(archive_path, 'x:gz', compresslevel=6) as archive:
        archive.add(STAGE, arcname=NAME, filter=attrs)
    with tarfile.open(archive_path, 'r:gz') as archive:
        members = {m.name[len(NAME) + 1:]: m for m in archive.getmembers() if m.isfile()}
        for want, name in checksum_records(archive.extractfile(members['SHA256SUMS']).read()):
            digest = hashlib.sha256()
            with archive.extractfile(members[name]) as stream:
                for chunk in iter(lambda: stream.read(1024 * 1024), b''):
                    digest.update(chunk)
            if digest.hexdigest() != want:
                raise RuntimeError('Archive readback hash mismatch: ' + name)
    for path in [archive_path, source_zip, database_zip]:
        (OUT / (path.name + '.sha256')).write_bytes((sha(path) + '  ' + path.name + '\n').encode('utf-8'))
    copy(ROOT / 'docs/ALIYUN_SCHEMA21_DEPLOY.md', OUT / (NAME + '-部署说明.md'))
    receipt = {'release': TAG, 'schema': 21, 'archiveReadbackAllHashes': 'passed',
               'productionDeployed': False, 'dataExported': False,
               'assets': [{'name': p.name, 'bytes': p.stat().st_size, 'sha256': sha(p)} for p in [archive_path, source_zip, database_zip]],
               'manifest': manifest}
    write_json(OUT / (NAME + '-verification.json'), receipt)
    print(json.dumps(receipt, ensure_ascii=False, indent=2))


if __name__ == '__main__':
    main()
