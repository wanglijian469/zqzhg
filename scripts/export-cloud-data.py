"""Export a private, portable MySQL/media snapshot and verify it by restoring a NEW test DB.

Run while application writes are paused. Credentials are read from server/.env;
they are never included in the archive. Existing sessions and SMS codes are excluded.
"""
import gzip
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
from datetime import datetime, timezone

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / '.local/cloud-data/migration'
MYSQL_BIN = Path(os.environ.get('MYSQL_BIN', 'D:/InforGuardMC/InforGuardMC/mysql/bin'))
config = {}
for line in (ROOT / 'server/.env').read_text(encoding='utf-8-sig').splitlines():
    if '=' in line and not line.lstrip().startswith('#'):
        key, value = line.split('=', 1)
        config[key.strip()] = value.strip().strip('"')
db = config.get('DB_NAME', 'zqzt_exhibition')
if not re.fullmatch(r'zqzt_exhibition', db):
    raise RuntimeError('This export is restricted to the local application database')
env = {**os.environ, 'MYSQL_PWD': config.get('DB_PASSWORD', 'root')}
args = ['--no-defaults', '--host=' + config.get('DB_HOST', '127.0.0.1'),
        '--port=' + config.get('DB_PORT', '13306'), '--user=' + config.get('DB_USER', 'root'),
        '--default-character-set=utf8mb4']

def mysql(sql, database=None):
    command = [str(MYSQL_BIN / 'mysql.exe'), *args, '--batch', '--skip-column-names']
    if database:
        command.append(database)
    result = subprocess.run(command, input=sql.encode('utf-8'), stdout=subprocess.PIPE,
                            stderr=subprocess.PIPE, env=env, check=False)
    if result.returncode:
        raise RuntimeError('MySQL operation failed: ' + result.stderr.decode('utf-8', errors='replace'))
    return result.stdout.decode('utf-8').replace('\r\n', '\n')

if OUT.exists():
    raise RuntimeError('Existing snapshot retained. Choose a new output directory before exporting again.')
OUT.mkdir(parents=True)
tables = mysql('SHOW TABLES', db).strip().splitlines()
if not tables or any(not re.fullmatch('[a-zA-Z0-9_]+', table) for table in tables):
    raise RuntimeError('Unexpected schema')
excluded = {'sessions', 'sms_codes'}
count_sql = '\nUNION ALL\n'.join(
    f"SELECT '{table}', COUNT(*) FROM `{table}`" for table in sorted(tables)) + ';\n'
expected = mysql(count_sql, db)
expected = '\n'.join(f'{line.split(chr(9))[0]}\t0' if line.split('\t')[0] in excluded else line
                     for line in expected.strip().splitlines()) + '\n'
(OUT / 'counts.sql').write_text(count_sql, encoding='utf-8', newline='\n')
(OUT / 'expected-counts.tsv').write_text(expected, encoding='utf-8', newline='\n')
dump_common = [str(MYSQL_BIN / 'mysqldump.exe'), *args, '--column-statistics=0',
               '--set-gtid-purged=OFF', '--no-tablespaces', '--skip-comments']
plain = OUT.parent / 'database.sql'
with plain.open('wb') as file:
    for options in [
        ['--no-data'],
        ['--no-create-info', '--skip-triggers', '--single-transaction',
         *[f'--ignore-table={db}.{name}' for name in sorted(excluded)]],
    ]:
        subprocess.run([*dump_common, *options, db], stdout=file, stderr=subprocess.PIPE, env=env, check=True)
        file.write(b'\n')
with plain.open('rb') as source, gzip.open(OUT / 'database.sql.gz', 'wb', compresslevel=9) as target:
    shutil.copyfileobj(source, target)
shutil.copytree(ROOT / 'server/data/media', OUT / 'media')

test_db = 'zqzt_exhibition_test_' + datetime.now().strftime('%Y%m%d%H%M%S')
if mysql(f"SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='{test_db}'").strip() != '0':
    raise RuntimeError('Restore test database already exists')
mysql(f'CREATE DATABASE `{test_db}` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci')
try:
    with plain.open('rb') as source:
        subprocess.run([str(MYSQL_BIN / 'mysql.exe'), *args, test_db], stdin=source,
                       stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env, check=True)
    actual = mysql(count_sql, test_db)
    if actual != expected:
        raise RuntimeError('Restored table counts differ')
    # Compare password hashes inside MySQL without emitting account information.
    matches = mysql(f'SELECT COUNT(*) FROM `{db}`.accounts a LEFT JOIN `{test_db}`.accounts b '
                    'ON a.id=b.id WHERE b.id IS NULL OR a.password_hash<>b.password_hash').strip()
    if matches != '0':
        raise RuntimeError('Restored account passwords differ')
finally:
    mysql(f'DROP DATABASE `{test_db}`')
plain.unlink()

def sha(file):
    with file.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()

source_media = ROOT / 'server/data/media'
media_files = list((OUT / 'media').rglob('*'))
media_files = [file for file in media_files if file.is_file()]
for file in media_files:
    if sha(file) != sha(source_media / file.relative_to(OUT / 'media')):
        raise RuntimeError('Copied media differs')
manifest = {'createdAt': datetime.now(timezone.utc).isoformat(), 'sourceDatabase': db,
            'schemaAndData': True, 'excludedData': sorted(excluded), 'mediaFiles': len(media_files),
            'restoreTest': 'passed: new isolated database, all table counts and password hashes match',
            'containsPrivateData': True, 'tableCounts': dict(line.split('\t') for line in expected.strip().splitlines())}
(OUT / 'manifest.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
files = sorted(file for file in OUT.rglob('*') if file.is_file())
(OUT / 'SHA256SUMS').write_text(''.join(f'{sha(file)}  {file.relative_to(OUT).as_posix()}\n' for file in files), encoding='utf-8', newline='\n')
print(json.dumps(manifest, ensure_ascii=False, indent=2))
