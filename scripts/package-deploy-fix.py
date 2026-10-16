"""Ship installer-only fix with a matching complete checksum list; retain old artifacts."""
from pathlib import Path
import hashlib
import json
import shutil
import tarfile
import zipfile
from datetime import datetime, timezone

ROOT = Path(__file__).resolve().parents[1]
DELIVERY = ROOT / 'delivery'
BASE = ROOT / '.local/cloud-package/zqzt-aliyun-v1.5.13'
STAGE = ROOT / '.local/cloud-fix1/zqzt-aliyun-v1.5.13'
if STAGE.exists():
    raise RuntimeError('Fix staging folder already exists')
shutil.copytree(BASE, STAGE)
changed = ['deploy.sh', 'PASSWORD_POLICY_FIX.md', 'verification/deploy-password-policy.json']
for source, target in [('deploy/linux/deploy.sh', changed[0]), ('deploy/linux/PASSWORD_POLICY_FIX.md', changed[1]),
                       ('docs/test-output/deploy-password-policy.json', changed[2])]:
    shutil.copyfile(ROOT / source, STAGE / target)

def sha(file):
    with file.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()

files = sorted(file for file in STAGE.rglob('*') if file.is_file() and file.name != 'SHA256SUMS')
(STAGE / 'SHA256SUMS').write_text(''.join(f'{sha(file)}  {file.relative_to(STAGE).as_posix()}\n' for file in files), encoding='utf-8', newline='\n')
changed.append('SHA256SUMS')

def attributes(info):
    info.uid = info.gid = 0
    info.uname = info.gname = 'root'
    info.mode = 0o755 if info.isdir() or info.name.endswith(('/deploy.sh', '/exhibition')) else 0o644
    return info

patch = DELIVERY / 'zqzt-deploy-v1.5.13-fix1.tar.gz'
full = DELIVERY / 'zqzt-aliyun-v1.5.13-fix1-linux.tar.gz'
for file in [patch, full]:
    if file.exists():
        raise RuntimeError('Refusing to overwrite existing delivery')
with tarfile.open(patch, 'w:gz') as archive:
    for name in changed:
        archive.add(STAGE / name, arcname=STAGE.name + '/' + name, filter=attributes)
with tarfile.open(full, 'w:gz', compresslevel=6) as archive:
    archive.add(STAGE, arcname=STAGE.name, filter=attributes)

# Simulate overlaying only the patch onto the original package, without extraction.
with tarfile.open(patch) as archive:
    overlay = {member.name[len(STAGE.name) + 1:]: archive.extractfile(member).read() for member in archive.getmembers() if member.isfile()}
    for line in overlay['SHA256SUMS'].decode().splitlines():
        expected, name = line.split('  ', 1)
        actual = hashlib.sha256(overlay[name]).hexdigest() if name in overlay else sha(BASE / name)
        assert expected == actual, name

source = DELIVERY / 'zqzt-exhibition-v1.5.13-fix1-source.zip'
updates = ['deploy/linux/deploy.sh', 'deploy/linux/PASSWORD_POLICY_FIX.md', 'scripts/test-deploy-password-policy.py', 'scripts/package-deploy-fix.py']
with zipfile.ZipFile(DELIVERY / 'zqzt-exhibition-v1.5.13-source.zip') as original, zipfile.ZipFile(source, 'x', compression=zipfile.ZIP_DEFLATED, compresslevel=6) as archive:
    for item in original.infolist():
        if item.filename not in updates:
            archive.writestr(item.filename, original.read(item))
    for name in updates:
        archive.write(ROOT / name, name)
shutil.copyfile(ROOT / 'deploy/linux/PASSWORD_POLICY_FIX.md', DELIVERY / 'PASSWORD_POLICY_FIX.md')
shutil.copyfile(ROOT / 'docs/test-output/deploy-password-policy.json', DELIVERY / 'deploy-password-policy-verification.json')
outputs = [patch, full, source]
manifest = {'installerRevision': '1.5.13-fix1', 'applicationVersion': '1.5.13',
            'androidVersion': '1.5.13+21', 'androidChanged': False, 'migrationChanged': False,
            'createdAt': datetime.now(timezone.utc).isoformat(), 'passwordIncluded': False,
            'patchOverlayChecksums': 'passed', 'files': [{'file': file.name, 'bytes': file.stat().st_size, 'sha256': sha(file)} for file in outputs]}
(DELIVERY / 'deploy-fix1-manifest.json').write_text(json.dumps(manifest, indent=2), encoding='utf-8')
(DELIVERY / 'SHA256SUMS-deploy-fix1.txt').write_text(''.join(f'{sha(file)}  {file.name}\n' for file in outputs), encoding='utf-8', newline='\n')
print(json.dumps(manifest, indent=2))
