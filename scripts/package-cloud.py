"""Build versioned Linux, Android and source artifacts; keep migration data private."""
from pathlib import Path
import gzip
import hashlib
import io
import json
import shutil
import tarfile
import zipfile
from datetime import datetime, timezone

ROOT = Path(__file__).resolve().parents[1]
DELIVERY = ROOT / 'delivery'
VERSION = '1.5.13'
ANDROID = '1.5.13+21'
ORIGIN = 'http://39.97.250.244:8089'
CERT = '85656c85541b188c832dc48d67cbf320046a4311aa9e0fefcc662dbe2e0f1585'
STAGE = ROOT / '.local/cloud-package' / f'zqzt-aliyun-v{VERSION}'
if STAGE.exists():
    raise RuntimeError('Package stage already exists; retain previous results')

def sha(file):
    with file.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()

def copy(source, target):
    target.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(source, target)

apk = DELIVERY / 'android' / f'dalunongji-{ANDROID}.apk'
apk_hash = sha(apk)
for name in ['delivery/android/dalunongji.apk', 'web/public/downloads/dalunongji.apk',
             'web/dist/downloads/dalunongji.apk', 'delivery/web/dist/downloads/dalunongji.apk']:
    if sha(ROOT / name) != apk_hash:
        raise RuntimeError('APK download copies differ: ' + name)
signature = (ROOT / 'docs/test-output/aliyun-apk-signature.txt').read_text(encoding='utf-8-sig')
if f'certificate SHA-256 digest: {CERT}' not in signature:
    raise RuntimeError('Original signing certificate not verified')
release = {'version': VERSION, 'websiteVersion': VERSION, 'backendVersion': VERSION,
           'androidVersion': ANDROID, 'applicationId': 'com.zqzt.dalunongji', 'apiBaseUrl': ORIGIN,
           'signingCertificateSHA256': CERT, 'apkSHA256': apk_hash,
           'createdAt': datetime.now(timezone.utc).isoformat(),
           'linuxArchitectures': ['amd64', 'arm64'], 'databaseMode': 'existing MySQL, new isolated database',
           'smsProvider': 'disabled until real Aliyun SMS credentials are configured',
           'videoTools': 'uses existing FFmpeg/FFprobe or tries configured system repositories',
           'remoteDeploymentTested': False,
           'androidDeviceValidation': {'cleanInstall': False, 'inPlaceUpgrade': False, 'camera': False,
                                       'nativeSharing': False, 'nativeMediaPlayback': False}}
STAGE.mkdir(parents=True)
for arch in ['amd64', 'arm64']:
    binary = ROOT / '.local/cloud-bin' / f'linux-{arch}/exhibition'
    header = binary.read_bytes()[:64]
    if header[:4] != b'\x7fELF' or int.from_bytes(header[18:20], 'little') != {'amd64': 62, 'arm64': 183}[arch]:
        raise RuntimeError('Unexpected Linux binary architecture')
    copy(binary, STAGE / f'bin/linux-{arch}/exhibition')
shutil.copytree(ROOT / 'web/dist', STAGE / 'web/dist')
for name in ['deploy.sh', 'README.md']:
    copy(ROOT / 'deploy/linux' / name, STAGE / name)
copy(ROOT / 'docs/openapi.json', STAGE / 'docs/openapi.json')
copy(ROOT / 'docs/screenshots/cloud-app-icon.png', STAGE / 'docs/app-icon.png')
for name in ['aliyun-flutter-analyze.txt', 'aliyun-flutter-test.txt', 'aliyun-web-test.txt',
             'aliyun-web-build.txt', 'aliyun-apk-metadata.txt', 'aliyun-apk-signature.txt',
             'aliyun-deploy-validation.txt', 'aliyun-artifact-validation.json', 'aliyun-devices.txt',
             'aliyun-adaptive-icon.txt', 'aliyun-runtime-smoke.json']:
    copy(ROOT / 'docs/test-output' / name, STAGE / 'verification' / name)
# Server test output contains only synthetic test accounts, but keep the public package small.
go_test = (ROOT / 'docs/test-output/aliyun-go-test.txt').read_text(encoding='utf-8-sig')
if '\nPASS\n' not in go_test.replace('\r\n', '\n'):
    raise RuntimeError('Backend test result missing')
(STAGE / 'verification/go-test-summary.txt').write_text('\n'.join(
    line for line in go_test.splitlines() if line.startswith(('--- PASS:', '--- SKIP:', 'PASS', 'ok '))) + '\n', encoding='utf-8')
(STAGE / 'manifest.json').write_text(json.dumps(release, indent=2, ensure_ascii=False) + '\n', encoding='utf-8')

def checksums(folder):
    files = sorted(file for file in folder.rglob('*') if file.is_file() and file.name != 'SHA256SUMS')
    (folder / 'SHA256SUMS').write_text(''.join(f'{sha(file)}  {file.relative_to(folder).as_posix()}\n' for file in files), encoding='utf-8', newline='\n')

checksums(STAGE)

def tar(folder, output, private=False):
    if output.exists():
        raise RuntimeError('Refusing to overwrite an existing archive: ' + str(output))
    output.parent.mkdir(parents=True, exist_ok=True)
    def attributes(info):
        info.uid = info.gid = 0
        info.uname = info.gname = 'root'
        info.mode = (0o700 if info.isdir() else 0o600) if private else (
            0o755 if info.isdir() or info.name.endswith(('/deploy.sh', '/exhibition')) else 0o644)
        return info
    with tarfile.open(output, 'w:gz', compresslevel=6) as archive:
        archive.add(folder, arcname=folder.name, filter=attributes)

linux_archive = DELIVERY / f'zqzt-aliyun-v{VERSION}-linux.tar.gz'
tar(STAGE, linux_archive)
migration = ROOT / '.local/cloud-data/migration'
private_archive = DELIVERY / 'private/zqzt-data-20261015.tar.gz'
tar(migration, private_archive, private=True)
(private_archive.parent / 'README.txt').write_text('私有数据库与媒体迁移包。仅上传到 ECS 非网站目录。含企业及账户资料，禁止公开下载。\n', encoding='utf-8')

android_archive = DELIVERY / f'dalunongji-android-v{VERSION}-deploy.zip'
with zipfile.ZipFile(android_archive, 'x', compression=zipfile.ZIP_DEFLATED, compresslevel=6) as archive:
    archive.write(apk, apk.name)
    archive.write(STAGE / 'README.md', 'README.md')
    archive.write(STAGE / 'manifest.json', 'manifest.json')
    archive.write(ROOT / 'docs/screenshots/cloud-app-icon.png', 'app-icon.png')
    archive.writestr('SHA256SUMS', f'{apk_hash}  {apk.name}\n')
    for file in (STAGE / 'verification').iterdir():
        archive.write(file, 'verification/' + file.name)

excluded_dirs = {'node_modules', 'pdfjs', 'dist', 'build', '.dart_tool', '.gradle', '.idea', 'ephemeral',
                 '.symlinks', 'Pods', '.plugin_symlinks', 'data', 'test-output', '__pycache__'}
excluded_files = {'.env', 'key.properties', 'local.properties', '.flutter-plugins-dependencies',
                  'Generated.xcconfig', 'flutter_export_environment.sh', 'GeneratedPluginRegistrant.java'}
excluded_suffixes = {'.exe', '.apk', '.jks', '.keystore', '.log', '.iml', '.tsbuildinfo', '.pyc'}
source_archive = DELIVERY / f'zqzt-exhibition-v{VERSION}-source.zip'
source_files = []
with zipfile.ZipFile(source_archive, 'x', compression=zipfile.ZIP_DEFLATED, compresslevel=6) as archive:
    for name in ['server', 'web', 'mobile', 'docs', 'scripts', 'deploy', '需求文档', 'README.md', '.gitignore']:
        source = ROOT / name
        for file in sorted(source.rglob('*')) if source.is_dir() else [source]:
            rel = file.relative_to(ROOT)
            if not file.is_file() or any(part in excluded_dirs for part in rel.parts) or file.name in excluded_files or file.suffix in excluded_suffixes:
                continue
            archive.write(file, rel.as_posix())
            source_files.append(rel.as_posix())
audit = {'version': VERSION, 'sourceFiles': source_files, 'forbiddenFilesIncluded': [],
         'privateMigrationExcludedFromPublicPackages': True}
(DELIVERY / 'cloud-package-audit.json').write_text(json.dumps(audit, indent=2, ensure_ascii=False), encoding='utf-8')
copy(STAGE / 'README.md', DELIVERY / 'ALIYUN_DEPLOYMENT.md')
copy(STAGE / 'docs/openapi.json', DELIVERY / 'openapi.json')
shutil.copytree(ROOT / 'web/dist', DELIVERY / 'web/dist', dirs_exist_ok=True)

artifacts = [apk, linux_archive, private_archive, android_archive, source_archive]
release['files'] = [{'file': file.relative_to(DELIVERY).as_posix(), 'bytes': file.stat().st_size, 'sha256': sha(file)} for file in artifacts]
(DELIVERY / 'cloud-manifest.json').write_text(json.dumps(release, indent=2, ensure_ascii=False) + '\n', encoding='utf-8')
(DELIVERY / 'SHA256SUMS-cloud.txt').write_text(''.join(f'{sha(file)}  {file.relative_to(DELIVERY).as_posix()}\n' for file in artifacts), encoding='utf-8', newline='\n')
# Retain the historical manifest with the old release before updating current aliases.
old_manifest = DELIVERY / 'manifest.json'
archive_manifest = DELIVERY / 'manifest-v1.5.12.json'
if old_manifest.exists() and not archive_manifest.exists():
    shutil.copyfile(old_manifest, archive_manifest)
(DELIVERY / 'manifest.json').write_text(json.dumps(release, indent=2, ensure_ascii=False) + '\n', encoding='utf-8')
print(json.dumps({'files': release['files'], 'sourceFileCount': len(source_files)}, indent=2))
