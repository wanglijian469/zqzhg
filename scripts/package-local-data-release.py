"""Build a PRIVATE ECS upgrade bundle with the current local DB/media snapshot.

The public program release is reused byte-for-byte. Never publish this output.
Application writes must be stopped before running this script.
"""
from pathlib import Path
from datetime import datetime, timezone
import gzip
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
import tarfile

ROOT = Path(__file__).resolve().parents[1]
NAME = 'zqzg-aliyun-20261019-with-local-data-v1'
STAGE = ROOT / '.local/private-release' / NAME
BASE = ROOT / '.local/aliyun-release/zqzg-aliyun-20261018-storage-v1'
OUT = ROOT / 'delivery/private'
MYSQL_BIN = Path(os.environ.get('MYSQL_BIN', 'D:/InforGuardMC/InforGuardMC/mysql/bin'))
EXCLUDE = {'sessions', 'sms_codes', 'system_settings', 'storage_profiles', 'stored_object_replicas',
           'media_object_owners', 'media_upload_reservations', 'analytics_events', 'analytics_daily'}


def sha(file):
    h = hashlib.sha256()
    with file.open('rb') as stream:
        for data in iter(lambda: stream.read(1024 * 1024), b''):
            h.update(data)
    return h.hexdigest()


def read_config():
    cfg = {}
    for line in (ROOT / 'server/.env').read_text(encoding='utf-8-sig').splitlines():
        if '=' in line and not line.lstrip().startswith('#'):
            key, value = line.split('=', 1)
            cfg[key.strip()] = value.strip().strip('"')
    return cfg


def main():
    if STAGE.exists() or (OUT / (NAME + '.tar.gz')).exists():
        raise RuntimeError('Use a new release name; existing private packages are retained')
    cfg = read_config()
    database = cfg.get('DB_NAME', 'zqzt_exhibition')
    media = (ROOT / 'server' / cfg.get('MEDIA_DIR', './data/media')).resolve()
    operations = Path(cfg.get('OPERATIONS_DIR') or media.parent / ('.operations-' + media.name))
    if not operations.is_absolute():
        operations = (ROOT / 'server' / operations).resolve()
    pointer = operations / 'active.json'
    if pointer.exists():
        state = json.loads(pointer.read_text(encoding='utf-8-sig'))
        if state.get('pending'):
            raise RuntimeError('Pending restore; finish it before exporting')
        database = state.get('current', {}).get('database') or database
        media = Path(state.get('current', {}).get('media_dir') or media).resolve()
    if database != 'zqzt_exhibition':
        raise RuntimeError('Review effective restored database before extending this local exporter')
    # Refuse a concurrent local application: all writes must be paused.
    running = subprocess.run(['powershell', '-NoProfile', '-Command',
                              "if (Get-Process exhibition -ErrorAction SilentlyContinue) {exit 1}"], capture_output=True)
    if running.returncode:
        raise RuntimeError('Stop the local exhibition application before exporting')
    common = ['--no-defaults', '--host=' + cfg.get('DB_HOST', '127.0.0.1'),
              '--port=' + cfg.get('DB_PORT', '13306'), '--user=' + cfg.get('DB_USER', 'root'),
              '--default-character-set=utf8mb4']
    env = {**os.environ, 'MYSQL_PWD': cfg.get('DB_PASSWORD', '')}
    def query(sql, db=database):
        result = subprocess.run([str(MYSQL_BIN / 'mysql.exe'), *common, '--batch', '--raw', '--skip-column-names', db],
                                input=sql.encode('utf-8'), env=env, capture_output=True)
        if result.returncode:
            raise RuntimeError('MySQL snapshot query failed; credentials were not logged')
        return result.stdout.decode('utf-8').replace('\r\n', '\n').strip()
    if query('SELECT MAX(version) FROM schema_migrations') != '15':
        raise RuntimeError('Expected Schema15')
    if query('SELECT COUNT(*) FROM stored_objects WHERE store_id<>0') != '0':
        raise RuntimeError('Nonlocal objects require verified materialization before export')
    tables = query('SHOW TABLES').splitlines()
    if any(not re.fullmatch(r'[A-Za-z0-9_]+', name) for name in tables):
        raise RuntimeError('Unexpected table name')
    counts = {table: 0 if table in EXCLUDE else int(query('SELECT COUNT(*) FROM `%s`' % table)) for table in tables}
    shutil.copytree(BASE, STAGE)
    data = STAGE / 'local-data'
    data.mkdir()
    shutil.copytree(media, data / 'media')
    objects = []
    for file in sorted(media.rglob('*')):
        if file.is_symlink():
            raise RuntimeError('Media contains a symlink; inspect manually')
        if not file.is_file():
            continue
        relative = file.relative_to(media).as_posix()
        digest = sha(file)
        if sha(data / 'media' / relative) != digest:
            raise RuntimeError('Copied media mismatch')
        objects.append({'key': relative, 'size': file.stat().st_size, 'sha256': digest})
    inventory = {o['key']: o for o in objects}
    for line in query('SELECT object_key,size,sha256 FROM stored_objects').splitlines():
        key, size, digest = line.split('\t')
        item = inventory.get(key)
        if not item or item['size'] != int(size) or item['sha256'] != digest:
            raise RuntimeError('Stored object missing or corrupt: ' + key)
    for key in query("SELECT path FROM media WHERE path<>'' UNION SELECT thumb_path FROM media WHERE thumb_path<>'' UNION SELECT source_path FROM media_metadata WHERE source_path<>''").splitlines():
        if key not in inventory:
            raise RuntimeError('Referenced media missing: ' + key)
    dump = [str(MYSQL_BIN / 'mysqldump.exe'), *common, '--column-statistics=0',
            '--set-gtid-purged=OFF', '--no-tablespaces', '--skip-comments', '--hex-blob']
    plain = data / 'database.sql'
    with plain.open('xb') as stream:
        for options in [ ['--no-data'], ['--no-create-info', '--skip-triggers', '--single-transaction',
                          *['--ignore-table=' + database + '.' + name for name in sorted(EXCLUDE)]] ]:
            result = subprocess.run([*dump, *options, database], env=env, stdout=stream, stderr=subprocess.PIPE)
            if result.returncode:
                raise RuntimeError('Database dump failed; inspect private server logs')
            stream.write(b'\n')
    with plain.open('rb') as source, gzip.open(data / 'database.sql.gz', 'wb', compresslevel=9) as target:
        shutil.copyfileobj(source, target)
    plain.unlink()
    manifest = {'kind': 'private-local-snapshot', 'schema': 15,
                'created_at': datetime.now(timezone.utc).isoformat(), 'table_counts': counts,
                'vendors': int(query("SELECT COUNT(*) FROM contents WHERE kind='vendor'")),
                'products': int(query("SELECT COUNT(*) FROM contents WHERE kind='product'")),
                'public_vendors': int(query("SELECT COUNT(*) FROM contents WHERE kind='vendor' AND visible=1 AND published_revision_id>0")),
                'public_products': int(query("SELECT COUNT(*) FROM contents WHERE kind='product' AND visible=1 AND published_revision_id>0")),
                'objects': objects, 'excluded_rows': sorted(EXCLUDE),
                'contains_private_data': True, 'contains_account_password_hashes': True,
                'contains_server_environment_or_keys': False,
                'server_settings': 'Server system_config_v3/site_identity copied during cutover; env/key unchanged',
                'mode': 'new database and media target; not a merge'}
    (data / 'manifest.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    (data / 'SHA256SUMS').write_text(''.join(sha(p) + '  ' + p.relative_to(data).as_posix() + '\n'
                                             for p in sorted(data.rglob('*')) if p.is_file()), encoding='utf-8', newline='\n')
    (STAGE / 'import-local-data.py').write_text((ROOT / 'deploy/linux/import-local-data.py').read_text(encoding='utf-8'), encoding='utf-8', newline='\n')
    shutil.move(STAGE / 'README.md', STAGE / 'README-PROGRAM.md')
    shutil.copyfile(ROOT / 'docs/ALIYUN_LOCAL_DATA_DEPLOY.md', STAGE / 'README.md')
    public = json.loads((STAGE / 'manifest.json').read_text(encoding='utf-8'))
    public.update({'program_release': public['release'], 'release': NAME, 'packagedAt': manifest['created_at'],
                   'includesBusinessDatabase': True, 'includesPrivateMedia': True, 'privateDistributionOnly': True,
                   'snapshotVendors': manifest['vendors'], 'snapshotProducts': manifest['products'],
                   'snapshotMediaFiles': len(objects), 'newImporterLinuxRuntimeTested': False})
    (STAGE / 'manifest.json').write_text(json.dumps(public, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    # Final archive is created only after independent restore/smoke validation.
    print(json.dumps({'stage': str(STAGE), 'vendors': manifest['vendors'], 'products': manifest['products'],
                      'media_files': len(objects), 'media_bytes': sum(o['size'] for o in objects)}, ensure_ascii=False))


def finalize():
    archive = OUT / (NAME + '.tar.gz')
    if archive.exists():
        raise RuntimeError('Never overwrite an existing private delivery')
    result = json.loads((STAGE / 'verification/local-data-validation.json').read_text(encoding='utf-8'))
    if result.get('isolated_mysql_restore') != 'passed' or not result.get('original_database_content_counts_unchanged'):
        raise RuntimeError('Independent restore and smoke verification required')
    if result.get('importerSHA256') != sha(ROOT / 'deploy/linux/import-local-data.py'):
        raise RuntimeError('Importer changed since validation')
    tests = subprocess.run([sys.executable, str(ROOT / 'scripts/test-local-data-import.py')], capture_output=True)
    if tests.returncode:
        raise RuntimeError('Importer tests failed')
    (STAGE / 'verification/import-unit-tests.txt').write_bytes(tests.stdout + tests.stderr)
    (STAGE / 'import-local-data.py').write_text((ROOT / 'deploy/linux/import-local-data.py').read_text(encoding='utf-8'), encoding='utf-8', newline='\n')
    (STAGE / 'README.md').write_text((ROOT / 'docs/ALIYUN_LOCAL_DATA_DEPLOY.md').read_text(encoding='utf-8'), encoding='utf-8', newline='\n')
    # Verify reused executable and static assets against the previously published program release.
    original = json.loads((BASE / 'manifest.json').read_text(encoding='utf-8'))
    for arch, want in original['binarySHA256'].items():
        if sha(STAGE / 'server' / ('exhibition-linux-' + arch)) != want:
            raise RuntimeError('Program release binary changed')
    if sha(STAGE / 'web/dist/index.html') != original['frontendIndexSHA256']:
        raise RuntimeError('Program release frontend changed')
    (STAGE / 'SHA256SUMS').write_text(''.join(sha(p) + '  ' + p.relative_to(STAGE).as_posix() + '\n'
                                            for p in sorted(STAGE.rglob('*')) if p.is_file() and p != STAGE / 'SHA256SUMS'), encoding='utf-8', newline='\n')
    OUT.mkdir(parents=True, exist_ok=True)
    def attrs(info):
        info.uid = info.gid = 0
        info.uname = info.gname = 'root'
        info.mode = 0o700 if info.isdir() else (0o700 if '/server/exhibition-linux-' in info.name else 0o600)
        return info
    with tarfile.open(archive, 'w:gz', compresslevel=6) as output:
        output.add(STAGE, arcname=NAME, filter=attrs)
    checksum = sha(archive)
    (OUT / (archive.name + '.sha256')).write_text(checksum + '  ' + archive.name + '\n', encoding='utf-8', newline='\n')
    # Independently read back every archive member and its full-file digest.
    with tarfile.open(archive, 'r:gz') as check:
        files = {m.name[len(NAME) + 1:]: m for m in check.getmembers() if m.isfile()}
        sums = check.extractfile(files['SHA256SUMS']).read().decode('utf-8')
        for line in sums.splitlines():
            want, name = line.split('  ', 1)
            if name not in files:
                raise RuntimeError('Missing archive file: ' + name)
            digest = hashlib.sha256()
            with check.extractfile(files[name]) as stream:
                for chunk in iter(lambda: stream.read(1024 * 1024), b''):
                    digest.update(chunk)
            if digest.hexdigest() != want:
                raise RuntimeError('Archive checksum mismatch: ' + name)
    receipt = {'file': archive.name, 'bytes': archive.stat().st_size, 'sha256': checksum,
               'private': True, 'public_upload': False, 'import_unit_tests': 17, 'restore': result,
               'program_release': original['release'], 'program_git_commit': original['gitCommit'],
               'archive_readback_all_file_checksums': 'passed'}
    (OUT / (NAME + '-verification.json')).write_text(json.dumps(receipt, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    shutil.copyfile(ROOT / 'docs/ALIYUN_LOCAL_DATA_DEPLOY.md', OUT / (NAME + '-部署说明.md'))
    print(json.dumps(receipt, ensure_ascii=False, indent=2))


if __name__ == '__main__':
    if '--finalize' in sys.argv:
        finalize()
    else:
        main()
