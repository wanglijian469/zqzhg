"""Website/backend-only v1.12.0 package; credentials and brochure data stay private."""
from datetime import datetime, timezone
from pathlib import Path
import hashlib
import json
import subprocess
import shutil
import tarfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
WORK = ROOT / '.local/release-1.12.0'
OUT = ROOT / 'delivery/release-v1.12.0'
NAME = 'zqzg-v1.12.0'
STAGE = WORK / 'package-final' / NAME


def sha(p):
    with p.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def inventory(folder):
    return {p.relative_to(folder).as_posix(): sha(p) for p in sorted(folder.rglob('*')) if p.is_file()}


def main():
    subprocess.run(['git', 'diff', '--exit-code', 'HEAD'], cwd=ROOT, stdout=subprocess.DEVNULL, check=True)
    commit = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
    assert not STAGE.exists(), 'package_stage_already_exists'
    OUT.mkdir(parents=True, exist_ok=True)
    (STAGE / 'server').mkdir(parents=True)
    for name in ['exhibition', 'bailian-preflight']:
        binary = WORK / 'linux-amd64' / name
        metadata = subprocess.check_output(['go', 'version', '-m', str(binary)], text=True)
        for expected in ['vcs.revision=' + commit, 'vcs.modified=false', 'GOOS=linux', 'GOARCH=amd64', 'CGO_ENABLED=0']:
            assert expected in metadata, 'binary_provenance_' + expected
        assert binary.read_bytes()[:6] == b'\x7fELF\x02\x01'
        shutil.copy2(binary, STAGE / 'server' / name)
    for folder in ['migrations', 'catalog', 'licenses']:
        shutil.copytree(ROOT / 'server' / folder, STAGE / 'server' / folder)
    assert sorted(int(p.name.split('_')[0]) for p in (STAGE / 'server/migrations').glob('*.sql')) == list(range(1, 35))
    (STAGE / 'web/dist').mkdir(parents=True)
    for name in ['index.html', 'brand.svg', 'ByteDanceVerify.html', 'assets', 'pdfjs', 'maps', 'guide-images']:
        src, dest = WORK / 'web-dist' / name, STAGE / 'web/dist' / name
        if src.is_dir(): shutil.copytree(src, dest)
        else: shutil.copy2(src, dest)
    assert not (STAGE / 'web/dist/downloads').exists()
    shutil.copytree(ROOT / '.local/map-preview', STAGE / 'map-demo')
    maps = inventory(STAGE / 'map-demo')
    reference = json.loads((WORK / 'inventory.json').read_text(encoding='utf-8-sig'))
    assert len(maps) == 77 and all(reference['map_files'].get(n) == v for n, v in maps.items())
    for source, target in [('deploy/linux/upgrade-ai-import.py', 'upgrade.py'), ('releases/v1.12.0/README.md', 'README.md'),
                           ('server/.env.example', 'server/.env.example'), ('docs/MAP_DEPLOYMENT_REQUIREMENTS.md', 'MAP_DEPLOYMENT_REQUIREMENTS.md')]:
        (STAGE / target).write_bytes((ROOT / source).read_bytes().replace(b'\r\n', b'\n'))
    (STAGE / 'docs').mkdir()
    for name in ['AI_IMPORT.md', 'AI_IMPORT_MODEL_SETTINGS.md', 'AI_IMPORT_TASK3_FIX.md', 'AI_IMPORT_TEST_RESULTS.md', 'openapi.json']:
        shutil.copy2(ROOT / 'docs' / name, STAGE / 'docs' / name)
    for source in (ROOT / 'deploy/linux').glob('rollback-ai-import-*.sql'):
        shutil.copy2(source, STAGE / source.name)
    manifest = dict(version='1.12.0', gitCommit=commit, schemaVersion=34, architecture='linux-amd64',
                    createdUTC=datetime.now(timezone.utc).isoformat(), binarySHA256=sha(STAGE / 'server/exhibition'),
                    preflightSHA256=sha(STAGE / 'server/bailian-preflight'), mapFiles=maps, mapUsage='development-demonstration',
                    mapMissingCodes=['130571'], android=reference['android'], downloadPolicy='preserve-existing-app',
                    databaseMigrationAdded=[32, 33, 34], defaultModel='qwen3.8-flash', automaticMaxEscalation=False,
                    includesBusinessDatabase=False, includesPrivateMedia=False, includesCredentials=False)
    (STAGE / 'manifest.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n', encoding='utf8')
    hashes = inventory(STAGE)
    (STAGE / 'SHA256SUMS').write_bytes(''.join(d + '  ' + n + '\n' for n, d in hashes.items()).encode())
    archive = OUT / (NAME + '-linux-amd64.tar.gz')
    with tarfile.open(archive, 'x:gz') as bundle:
        for file in sorted(p for p in STAGE.rglob('*') if p.is_file()):
            member = bundle.gettarinfo(str(file), arcname=NAME + '/' + file.relative_to(STAGE).as_posix())
            member.uid = member.gid = 0
            member.uname = member.gname = ''
            member.mode = 0o755 if file.name in ['exhibition', 'bailian-preflight', 'upgrade.py'] else 0o644
            with file.open('rb') as stream: bundle.addfile(member, stream)
    with tarfile.open(archive) as bundle:
        assert set(bundle.getnames()) == {NAME + '/' + n for n in list(hashes) + ['SHA256SUMS']}
        for n, d in hashes.items(): assert hashlib.sha256(bundle.extractfile(NAME + '/' + n).read()).hexdigest() == d
    source_zip = OUT / (NAME + '-source.zip')
    subprocess.run(['git', 'archive', '--format=zip', '--prefix=' + NAME + '-source/', '--output=' + str(source_zip), commit], cwd=ROOT, check=True)
    # Scan archive contents against actual configured secrets without ever printing them.
    secrets = []
    for line in (ROOT / 'server/.env').read_text(encoding='utf-8-sig').splitlines():
        if line.startswith('DASHSCOPE_API_KEY='): secrets.append(line.split('=', 1)[1].strip().strip('"').strip("'").encode())
    for path in [source_zip, archive]:
        if path.suffix == '.zip':
            with zipfile.ZipFile(path) as bundle:
                assert bundle.testzip() is None
                items = [(n, bundle.read(n)) for n in bundle.namelist() if not n.endswith('/')]
        else:
            with tarfile.open(path) as bundle: items = [(m.name, bundle.extractfile(m).read()) for m in bundle if m.isfile()]
        for n, data in items:
            assert not any(s in data for s in secrets if s), 'secret_in_archive'
            assert not any(part in n.split('/') for part in ['.local', '.tools', '厂商信息', 'node_modules'])
            assert not n.endswith(('/.env', '/39.97.250.244.txt', '/阿里云百炼大模型.txt', '.jks', '.keystore', '.apk'))
    shutil.copy2(STAGE / 'manifest.json', OUT / 'manifest-v1.12.0.json')
    shutil.copy2(ROOT / 'releases/v1.12.0/README.md', OUT / 'README.md')
    assets = [archive, source_zip, OUT / 'manifest-v1.12.0.json', OUT / 'README.md']
    (OUT / 'SHA256SUMS.txt').write_bytes(''.join(sha(p) + '  ' + p.name + '\n' for p in assets).encode())
    receipt = {'version': '1.12.0', 'commit': commit, 'package': str(archive), 'mapFiles': len(maps), 'secretScanPassed': True,
               'assets': [{'path': str(p.relative_to(ROOT)), 'sha256': sha(p), 'bytes': p.stat().st_size} for p in assets]}
    (WORK / 'package-receipt.json').write_text(json.dumps(receipt, indent=2) + '\n', encoding='utf8')
    print(json.dumps(receipt, ensure_ascii=False))


if __name__ == '__main__': main()
