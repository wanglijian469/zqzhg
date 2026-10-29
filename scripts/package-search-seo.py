"""Package validated SEO code for manual Linux upgrade; never connect to production."""
import ast
from datetime import datetime, timezone
import hashlib
import importlib.util
import json
from pathlib import Path, PurePosixPath
import shutil
import struct
import subprocess
import tarfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
RELEASE = 'zqzg-seo-20261025'
BASE = ROOT / 'delivery/search-seo-20261024'
STAGE = ROOT / '.local/releases' / RELEASE
OUT = ROOT / 'delivery'
TAR = OUT / (RELEASE + '-linux.tar.gz')
spec = importlib.util.spec_from_file_location('release_checksums', ROOT / 'scripts/package-schema21-release.py')
shared = importlib.util.module_from_spec(spec)
spec.loader.exec_module(shared)


def sha(file):
    value = hashlib.sha256()
    with file.open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            value.update(chunk)
    return value.hexdigest()


def copy(source, target):
    target.parent.mkdir(parents=True, exist_ok=True)
    data = source.read_bytes()
    if source.suffix in ('.py', '.sh', '.md'):
        data = data.decode('utf-8-sig').replace('\r\n', '\n').encode('utf-8')
    target.write_bytes(data)


def write_json(file, value):
    file.parent.mkdir(parents=True, exist_ok=True)
    file.write_bytes((json.dumps(value, ensure_ascii=False, indent=2) + '\n').encode('utf-8'))


def validate_elf(file, machine, arch):
    with file.open('rb') as stream:
        header = stream.read(64)
        if header[:6] != b'\x7fELF\x02\x01' or struct.unpack_from('<H', header, 18)[0] != machine:
            raise ValueError('Incorrect Linux architecture: ' + arch)
        offset = struct.unpack_from('<Q', header, 32)[0]
        size, count = struct.unpack_from('<H', header, 54)[0], struct.unpack_from('<H', header, 56)[0]
        for index in range(count):
            stream.seek(offset + index * size)
            if struct.unpack('<I', stream.read(4))[0] == 3:
                raise ValueError('Linux binary requires a dynamic loader: ' + arch)


def main():
    if STAGE.exists() or TAR.exists():
        raise ValueError('Preserve existing release; choose a new release identifier')
    for expected, name in shared.checksum_records((BASE / 'SHA256SUMS').read_bytes()):
        if sha(BASE / name) != expected:
            raise ValueError('Validated input changed: ' + name)
    receipt = json.loads((BASE / 'verification.json').read_text(encoding='utf-8'))
    web = BASE / 'web-dist.zip'
    if sha(web) != receipt['webArchiveSHA256']:
        raise ValueError('Validated website archive changed')
    amd64 = ROOT / '.local/seo-manual-exhibition-linux-amd64'
    if sha(amd64) != receipt['binarySHA256']:
        raise ValueError('Runtime code differs from the validated SEO build; rerun validation first')
    binaries = {}
    for arch, machine in [('amd64', 62), ('arm64', 183)]:
        binary = ROOT / '.local' / ('seo-manual-exhibition-linux-' + arch)
        validate_elf(binary, machine, arch)
        metadata = subprocess.check_output(['go', 'version', '-m', str(binary)], cwd=ROOT, text=True)
        if 'GOOS=linux' not in metadata or ('GOARCH=' + arch) not in metadata or 'CGO_ENABLED=0' not in metadata:
            raise ValueError('Incorrect build settings: ' + arch)
        copy(binary, STAGE / 'server' / ('exhibition-linux-' + arch))
        (STAGE / 'verification').mkdir(exist_ok=True)
        (STAGE / 'verification' / ('binary-' + arch + '.txt')).write_bytes(metadata.encode('utf-8'))
        binaries[arch] = sha(binary)
    expected_files = {row['path']: row['sha256'] for row in receipt['frontendFiles']}
    with zipfile.ZipFile(web) as archive:
        if set(archive.namelist()) != set(expected_files):
            raise ValueError('Website file inventory differs')
        for name in archive.namelist():
            path = PurePosixPath(name)
            if path.is_absolute() or '..' in path.parts or '\\' in name or path.parts[0] == 'downloads':
                raise ValueError('Unsafe or replacement download path: ' + name)
            data = archive.read(name)
            if hashlib.sha256(data).hexdigest() != expected_files[name]:
                raise ValueError('Website asset changed: ' + name)
            target = STAGE / 'web/dist' / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(data)
    for folder in ['migrations', 'catalog', 'licenses']:
        for file in (BASE / 'linux-amd64' / folder).rglob('*'):
            if file.is_file():
                copy(file, STAGE / folder / file.relative_to(BASE / 'linux-amd64' / folder))
    versions = sorted(int(file.name.split('_')[0]) for file in (STAGE / 'migrations').glob('*.sql'))
    if versions != list(range(1, 22)):
        raise ValueError('Existing Schema 1–21 migration chain is incomplete')
    for source, destination in [
        ('deploy/linux/upgrade-storage.sh', 'upgrade.sh'),
        ('deploy/linux/aliyun-upgrade-support.py', 'aliyun-upgrade-support.py'),
        ('deploy/linux/preserve-downloads.py', 'preserve-downloads.py'),
        ('docs/SEO_MANUAL_UPGRADE_20261025.md', 'README.md'),
        ('docs/SEARCH_VISIBILITY_20261024.md', 'docs/SEARCH_VISIBILITY_20261024.md')
    ]:
        copy(ROOT / source, STAGE / destination)
    for file in (BASE / 'verification').rglob('*'):
        if file.is_file():
            copy(file, STAGE / 'verification' / file.relative_to(BASE / 'verification'))
    for name in ['downloads-tests', 'upgrade-tests', 'package-tests', 'bash-syntax']:
        log = ROOT / '.local' / ('seo-manual-' + name + '.log')
        if not log.is_file():
            raise ValueError('Missing manual upgrade check: ' + name)
        copy(log, STAGE / 'verification' / log.name)
    for helper in ['aliyun-upgrade-support.py', 'preserve-downloads.py']:
        ast.parse((STAGE / helper).read_text(encoding='utf-8'), feature_version=(3, 6))
    source_files = list((ROOT / 'server').glob('*.go')) + list((ROOT / 'web/src').rglob('*'))
    source_files += [ROOT / 'server/seo-copy.json', ROOT / 'web/tsconfig.json', ROOT / 'deploy/linux/upgrade-storage.sh', ROOT / 'deploy/linux/preserve-downloads.py']
    write_json(STAGE / 'verification/source-files.json', [
        {'path': file.relative_to(ROOT).as_posix(), 'sha256': sha(file)}
        for file in sorted(source_files) if file.is_file()
    ])
    write_json(STAGE / 'manifest.json', {
        'release': RELEASE, 'createdAt': datetime.now(timezone.utc).isoformat(),
        'gitHead': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip(),
        'workingTreeModified': True, 'runtimeSnapshot': 'validated-search-seo-20261024',
        'schema': 21, 'newDatabaseMigrationsForSEO': False, 'preserveDownloads': True,
        'binarySHA256': binaries, 'frontendFiles': len(expected_files),
        'deploymentMode': 'manual; preserve current database, media, configuration and published downloads',
        'productionDeployed': False, 'searchAcceptance': 'not_passed',
        'tests': {'webPassed': 246, 'typeScript': True, 'goSEO': True, 'goVet': True,
                  'browserConfigurations': 16, 'browserErrors': 0, 'downloadsPreservation': 6,
                  'upgradeHelper': 10, 'packageChecksums': 4, 'bashSyntax': True},
        'limitations': ['No actual Linux/systemd upgrade was executed', 'DNS/HTTPS and search-console submission remain separate manual steps']
    })
    shared.sums(STAGE)
    with tarfile.open(TAR, 'x:gz') as archive:
        for file in sorted(STAGE.rglob('*')):
            if file.is_file():
                info = archive.gettarinfo(str(file), arcname=RELEASE + '/' + file.relative_to(STAGE).as_posix())
                info.uid = info.gid = 0
                info.uname = info.gname = 'root'
                info.mode = 0o755 if file.parent.name == 'server' or file.suffix == '.sh' else 0o644
                with file.open('rb') as stream:
                    archive.addfile(info, stream)
    with tarfile.open(TAR, 'r:gz') as archive:
        prefix = RELEASE + '/'
        names = {member.name for member in archive.getmembers()}
        for expected, name in shared.checksum_records(archive.extractfile(prefix + 'SHA256SUMS').read()):
            if hashlib.sha256(archive.extractfile(prefix + name).read()).hexdigest() != expected:
                raise ValueError('Archive readback failed: ' + name)
        if any(name.startswith(prefix + 'web/dist/downloads/') for name in names):
            raise ValueError('Archive contains replacement downloads')
    checksum = sha(TAR)
    Path(str(TAR) + '.sha256').write_bytes((checksum + '  ' + TAR.name + '\n').encode('utf-8'))
    copy(ROOT / 'docs/SEO_MANUAL_UPGRADE_20261025.md', OUT / (RELEASE + '-README.md'))
    write_json(OUT / (RELEASE + '-manifest.json'), json.loads((STAGE / 'manifest.json').read_text(encoding='utf-8')))
    print(json.dumps({'package': str(TAR), 'bytes': TAR.stat().st_size, 'sha256': checksum,
                      'frontendFiles': len(expected_files), 'architectures': ['amd64', 'arm64'],
                      'replacesPublishedDownloads': False}, ensure_ascii=False))


if __name__ == '__main__':
    main()
