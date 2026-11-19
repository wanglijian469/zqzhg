"""Package an OSS REVIEW candidate. This script never deploys or changes downloads.

Run after Go/web/APK checks. Output intentionally records outstanding cloud,
Linux/ECS proxy and physical-device acceptance, even if local map checks pass.
"""
from datetime import datetime, timezone
from pathlib import Path
import hashlib
import json
import shutil
import struct
import subprocess
import tarfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
WORK = ROOT / '.local/oss-candidate'
OUT = ROOT / 'delivery/oss-039-candidate'


def sha(file):
    h = hashlib.sha256()
    with file.open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            h.update(block)
    return h.hexdigest()


def inventory(folder):
    return {p.relative_to(folder).as_posix(): sha(p)
            for p in sorted(folder.rglob('*')) if p.is_file()}


def main():
    stamp = datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ')
    stage = WORK / 'packages' / stamp / 'zqzg-oss-039-candidate'
    stage.mkdir(parents=True, exist_ok=False)
    binary = WORK / 'exhibition-linux-amd64'
    with binary.open('rb') as stream:
        header = stream.read(64)
    assert header[:6] == b'\x7fELF\x02\x01' and struct.unpack_from('<H', header, 18)[0] == 62
    (stage / 'server').mkdir()
    shutil.copy2(binary, stage / 'server/exhibition')
    (stage / 'server/exhibition').chmod(0o755)
    for name in ['migrations', 'catalog', 'licenses']:
        shutil.copytree(ROOT / 'server' / name, stage / 'server' / name)
    shutil.copy2(ROOT / 'server/.env.example', stage / 'server/.env.example')
    # Never replace official downloads. They are carried forward by the operator
    # as a separate, verified existing directory during activation.
    for source in (ROOT / '.local/oss-web').iterdir():
        if source.name == 'downloads':
            continue
        target = stage / 'web/dist' / source.name
        target.parent.mkdir(parents=True, exist_ok=True)
        if source.is_dir():
            shutil.copytree(source, target)
        else:
            shutil.copy2(source, target)
    shutil.copytree(ROOT / '.local/map-preview', stage / 'map-demo')
    maps = inventory(stage / 'map-demo')
    assert len(maps) == 77, 'all development boundary snapshots required'
    shutil.copytree(ROOT / 'deploy/oss', stage / 'deploy/oss')
    (stage / 'deploy/linux').mkdir()
    shutil.copy2(ROOT / 'deploy/linux/rollback-oss-039.sql', stage / 'deploy/linux/rollback-oss-039.sql')
    (stage / 'docs').mkdir()
    for name in ['OSS_STORAGE.md', 'OSS_TEST_RESULTS.md', 'OSS_CANDIDATE_DEPLOYMENT.md', 'OSS_ACCEPTANCE_20261119.md', 'MAP_DEPLOYMENT_REQUIREMENTS.md']:
        shutil.copy2(ROOT / 'docs' / name, stage / 'docs' / name)
    (stage / 'evidence').mkdir()
    evidence = {}
    for name in ['ecs-readiness-readonly.json', 'map-production-proxy.json', 'map-apk-assets.json']:
        file = WORK / name
        if file.exists():
            data = json.loads(file.read_text(encoding='utf-8'))
            evidence[name] = data
            shutil.copy2(file, stage / 'evidence' / name)
    live_folder = WORK / 'live-20261119'
    live = {}
    boundaries = {}
    for name in ['acceptance.json', 'cors-preflight.json', 'private-cors-final.json', 'public-read-diagnostic.json', 'public-prefix-boundaries.json', 'policy-readonly-check.json', 'production-health.json', 'post-test-safety.json']:
        file = live_folder / name
        if file.exists() and file.stat().st_size:
            data = json.loads(file.read_text(encoding='utf-8'))
            (stage / 'evidence/live-20261119').mkdir(exist_ok=True)
            shutil.copy2(file, stage / 'evidence/live-20261119' / name)
            if name == 'acceptance.json':
                live = data
            if name == 'public-prefix-boundaries.json':
                boundaries = data
    scoped_public_accepted = bool(boundaries.get('passed') and boundaries.get('synthetic_cleanup_completed'))
    runtime_scopes_accepted = any('public prefixes: platform and company readable; upload-temp denied' in c for c in live.get('checks', []))
    full_cloud_accepted = bool(live.get('passed') and live.get('synthetic_cleanup_completed') and scoped_public_accepted and runtime_scopes_accepted)
    commit = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
    changed = subprocess.check_output(['git', 'diff', '--name-only', '-z', 'HEAD'], cwd=ROOT).decode('utf-8').split('\0')
    new = subprocess.check_output(['git', 'ls-files', '--others', '--exclude-standard', '-z'], cwd=ROOT).decode('utf-8').split('\0')
    source_files = sorted({n for n in changed + new if n and
                           (n.startswith(('server/', 'web/src/', 'mobile/lib/', 'mobile/test/', 'mobile/assets/i18n/', 'deploy/oss/'))
                            or n in ['.gitignore', 'mobile/pubspec.yaml', 'mobile/pubspec.lock',
                                     'mobile/android/app/src/main/assets/native-workspace.js',
                                     'mobile/android/app/src/main/kotlin/com/zqzt/dalunongji/WorkspaceActivity.kt',
                                     'deploy/linux/rollback-oss-039.sql', 'scripts/package-oss-candidate.py',
                                     'docs/OSS_STORAGE.md', 'docs/OSS_TEST_RESULTS.md', 'docs/OSS_CANDIDATE_DEPLOYMENT.md', 'docs/OSS_ACCEPTANCE_20261119.md'])
                           and n.endswith(('.go', '.sql', '.tsx', '.ts', '.dart', '.json', '.md', '.yaml', '.lock', '.kt', '.js', '.py', '.example', '.gitignore'))
                           and (ROOT / n).is_file()})
    assert all(not any(part in {'.secrets', '.local', 'data', 'node_modules'} for part in Path(n).parts) for n in source_files)
    source_hashes = {n: sha(ROOT / n) for n in source_files}
    apk = json.loads((ROOT / 'delivery/android-v1.12.0/manifest.json').read_text(encoding='utf-8-sig'))
    apk_file = ROOT / 'delivery/android-v1.12.0/大陆农机-1.12.0+53.apk'
    assert sha(apk_file) == apk['sha256'] and apk['signingMatchesPrevious']
    assert not apk['deviceValidation'] and not apk['productionDeployed']
    manifest = {
        'status': 'review-candidate-not-production-approved', 'schemaVersion': 39,
        'createdAt': stamp, 'gitBaseCommit': commit, 'workingTreeContainsChanges': True,
        'sourceChangesSHA256': source_hashes, 'binarySHA256': sha(binary), 'mapFiles': maps,
        'mapUsage': 'development-demonstration; retain original attribution',
        'localWindowsProductionProxyMapPassed': evidence.get('map-production-proxy.json', {}).get('passed', False),
        'embeddedAPKAssetsMapPassed': evidence.get('map-apk-assets.json', {}).get('passed', False),
        'actualLinuxECSProxyAccepted': False,
        'realRAMMultipartActionsAccepted': sum('ListMultipartUploads + ListParts + AbortMultipartUpload' in c for c in live.get('checks', [])) == 2,
        'realOSSPrivateFilesAccepted': all(any(name in c for c in live.get('checks', [])) for name in ['synthetic-photo.png:', 'synthetic-drawing.png:', 'synthetic-catalog.pdf:', 'synthetic-video.mp4:', 'synthetic-multipart.pdf:']),
        'realOSSPublicPrefixBoundariesAccepted': scoped_public_accepted,
        'realOSSWriteFlowAccepted': full_cloud_accepted,
        'verifiedDefaultEnterpriseQuotaBytes': 314572800 if 'default_quota_314572800_bytes' in live.get('checks', []) else None,
        'realQuotaExhaustionRejected': 'quota exhaustion rejected before upload' in live.get('checks', []),
        'realCompanyMigrationPerformed': False, 'productionDatabaseChanged': False,
        'productionDeployed': False, 'physicalAndroidDeviceAccepted': False,
        'cloudBlocker': None if full_cloud_accepted else 'Public-prefix and privacy-boundary validation incomplete; inspect current live evidence',
        'androidCandidate': apk, 'officialDownloadsIncludedOrReplaced': False,
        'includesBusinessDatabase': False, 'includesPrivateMedia': False, 'includesCredentials': False,
    }
    (stage / 'manifest.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    (stage / 'SHA256SUMS').write_text(''.join(f'{h}  {n}\n' for n, h in inventory(stage).items()), encoding='utf-8')
    OUT.mkdir(parents=True, exist_ok=True)
    archive = OUT / f'zqzg-oss-039-review-{stamp}-linux-amd64.tar.gz'
    with tarfile.open(archive, 'w:gz') as package:
        package.add(stage, arcname=stage.name)
    with tarfile.open(archive) as package:
        root = stage.name + '/'
        for name, digest in inventory(stage).items():
            h = hashlib.sha256()
            with package.extractfile(root + name) as stream:
                for block in iter(lambda: stream.read(1024 * 1024), b''):
                    h.update(block)
            assert h.hexdigest() == digest, 'archive content differs: ' + name
    source_archive = OUT / f'zqzg-oss-039-source-changes-{stamp}.zip'
    with zipfile.ZipFile(source_archive, 'w', zipfile.ZIP_DEFLATED) as package:
        for name in source_files:
            package.write(ROOT / name, name)
        package.writestr('SOURCE_BASE.json', json.dumps({'baseCommit': commit, 'files': source_hashes}, indent=2))
    receipt = {'candidate': str(archive), 'sha256': sha(archive), 'sourceChanges': str(source_archive),
               'sourceSHA256': sha(source_archive), 'stage': str(stage), 'manifest': manifest}
    (OUT / 'latest.json').write_text(json.dumps(receipt, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    print(json.dumps({k: v for k, v in receipt.items() if k != 'manifest'}, ensure_ascii=False))


if __name__ == '__main__':
    main()
