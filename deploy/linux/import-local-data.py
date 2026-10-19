#!/usr/bin/env python3
"""Import the private local snapshot into a NEW DB, then switch the restore pointer.

Never merge IDs or overwrite the server database/media. Upgrade programs first.
"""
import argparse
import getpass
import gzip
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import subprocess
import sys
import time
import urllib.request


def load_upgrade(bundle):
    spec = importlib.util.spec_from_file_location('upgrade_support', bundle / 'aliyun-upgrade-support.py')
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def safe_key(value):
    return bool(value) and not any(c in value for c in ':\\\x00\r\n') and all(
        part not in ('', '.', '..') for part in value.split('/'))


def digest(path):
    h = hashlib.sha256()
    with path.open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            h.update(chunk)
    return h.hexdigest()


def verify_snapshot(directory):
    manifest = json.loads((directory / 'manifest.json').read_text(encoding='utf-8'))
    if manifest.get('schema') != 15 or manifest.get('kind') != 'private-local-snapshot':
        raise ValueError('Expected the Schema15 private local snapshot')
    checked = set()
    for line in (directory / 'SHA256SUMS').read_text(encoding='utf-8').splitlines():
        want, name = line.split('  ', 1)
        if not safe_key(name) or not re.fullmatch('[a-f0-9]{64}', want):
            raise ValueError('Invalid checksum entry')
        path = directory / name
        if path.is_symlink() or not path.is_file() or digest(path) != want:
            raise ValueError('Checksum mismatch: ' + name)
        if name in checked:
            raise ValueError('Duplicate checksum entry')
        checked.add(name)
    required = {'database.sql.gz', 'manifest.json'} | {'media/' + o['key'] for o in manifest['objects']}
    if not required.issubset(checked):
        raise ValueError('Incomplete checksum manifest')
    for item in manifest['objects']:
        if not safe_key(item['key']):
            raise ValueError('Invalid media key')
        path = directory / 'media' / item['key']
        if path.stat().st_size != item['size'] or digest(path) != item['sha256']:
            raise ValueError('Media integrity failure: ' + item['key'])
    return manifest


def mysql(cfg, tool, sql=None, database=None, user=None, password=None, source=None):
    args = [tool, '--no-defaults', '--host=' + cfg.get('DB_HOST', '127.0.0.1'),
            '--port=' + cfg.get('DB_PORT', '3306'), '--user=' + (user or cfg.get('DB_USER', 'root')),
            '--default-character-set=utf8mb4', '--batch', '--raw', '--skip-column-names',
            '--local-infile=0', '--binary-mode=1']
    if database:
        if not re.fullmatch(r'[A-Za-z0-9_]{1,64}', database):
            raise ValueError('Invalid database')
        args.append(database)
    env = {**os.environ, 'MYSQL_PWD': cfg.get('DB_PASSWORD', '') if password is None else password}
    # gzip.GzipFile.fileno() points at compressed bytes: pass decompressed SQL.
    kwargs = {'input': source.read()} if source is not None else {'input': sql.encode('utf-8')}
    result = subprocess.run(args, **kwargs, env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if result.returncode:
        # Never echo SQL, passwords or server credentials in diagnostic output.
        raise RuntimeError('MySQL failed (exit %d); check privileges, connectivity and private server logs' % result.returncode)
    return result.stdout.decode('utf-8').replace('\r\n', '\n').strip()


def table_counts(cfg, tool, database, names):
    if any(not re.fullmatch(r'[A-Za-z0-9_]+', name) for name in names):
        raise ValueError('Invalid table name')
    query = ' UNION ALL '.join("SELECT '%s',COUNT(*) FROM `%s`" % (n, n) for n in sorted(names))
    return {line.split('\t')[0]: int(line.split('\t')[1]) for line in mysql(cfg, tool, query, database).splitlines()}


def write_pointer(path, value):
    metadata = path.stat() if path.exists() else path.parent.stat()
    temp = path.with_name(path.name + '.' + secrets.token_hex(6) + '.tmp')
    with temp.open('x', encoding='utf-8') as file:
        os.chmod(temp, 0o600)
        if hasattr(os, 'fchown'):
            # Root runs the importer; the service user must read the pointer.
            os.fchown(file.fileno(), metadata.st_uid, metadata.st_gid)
        json.dump(value, file, ensure_ascii=False, indent=2)
        file.flush()
        os.fsync(file.fileno())
    os.replace(temp, path)


def wait_healthy(port):
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    for _ in range(45):
        try:
            with opener.open('http://127.0.0.1:%d/api/v1/health' % port, timeout=2) as response:
                if json.load(response).get('status') == 'ok':
                    return True
        except (OSError, ValueError):
            pass
        time.sleep(1)
    return False


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--apply', action='store_true')
    parser.add_argument('--check', action='store_true')
    parser.add_argument('--confirm-use-local-data', action='store_true')
    parser.add_argument('--env-file', default='/etc/zqzt/exhibition.env')
    parser.add_argument('--install-dir', default='/opt/zqzt')
    parser.add_argument('--service', default='zqzt-exhibition.service')
    parser.add_argument('--db-admin', default='root')
    args = parser.parse_args()
    bundle = Path(__file__).resolve().parent
    upgrade = load_upgrade(bundle)
    cfg, info = upgrade.context(args.env_file, args.install_dir)
    manifest = verify_snapshot(bundle / 'local-data')
    tool = upgrade.tool(cfg, 'mysql')
    schema = mysql(cfg, tool, 'SELECT MAX(version) FROM schema_migrations', info['database'])
    if schema != '15':
        raise ValueError('Upgrade the programs to this Schema15 release first')
    if cfg.get('MEDIA_REQUIRE_MOUNT') == 'true':
        raise ValueError('Mount-required storage needs an operator-prepared restore target; not supported by this local-disk helper')
    if not re.fullmatch(r'[A-Za-z0-9_.@-]+', args.service):
        raise ValueError('Invalid service')
    print(json.dumps({'snapshot': manifest['created_at'], 'vendors': manifest['vendors'],
                      'products': manifest['products'], 'media_files': len(manifest['objects']),
                      'current_database_retained': info['database'], 'mode': 'switch, never merge or overwrite'}, ensure_ascii=False))
    if not args.apply:
        print('Preflight passed. No database, media or service was changed.')
        return
    if not args.confirm_use_local_data:
        raise ValueError('Use --confirm-use-local-data to switch the active site to this local snapshot (not a merge)')
    if sys.platform != 'linux' or os.geteuid() != 0:
        raise ValueError('Apply must run as root on the Linux ECS server')
    subprocess.run(['systemctl', 'is-active', '--quiet', args.service], check=True)
    admin = cfg.get('DB_MAINTENANCE_USER') or args.db_admin
    admin_password = cfg.get('DB_MAINTENANCE_PASSWORD') if cfg.get('DB_MAINTENANCE_USER') else getpass.getpass('Existing MySQL administrator password (not saved): ')
    def maintenance(sql):
        return mysql(cfg, tool, sql, user=admin, password=admin_password)
    application_user = mysql(cfg, tool, 'SELECT CURRENT_USER()')
    parts = application_user.split('@')
    if len(parts) != 2 or any(not re.fullmatch(r'[A-Za-z0-9_.%:\-]+', p) for p in parts):
        raise ValueError('Application user format is not supported')
    stamp = time.strftime('%Y%m%dT%H%M%SZ', time.gmtime()) + '-' + secrets.token_hex(3)
    target = Path(args.install_dir).resolve() / 'shared' / 'local-data' / stamp
    target.parent.mkdir(parents=True, exist_ok=True)
    os.chmod(target.parent, 0o700)
    if shutil.disk_usage(target.parent).free < sum(o['size'] for o in manifest['objects']) * 2 + (1 << 30):
        raise ValueError('Insufficient free disk space')
    database = 'zqzt_exhibition_restore_' + secrets.token_hex(8)
    # CREATE has no IF NOT EXISTS: an existing DB can never be imported over.
    maintenance('CREATE DATABASE `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci' % database)
    temp_user = 'zqi_' + secrets.token_hex(6)
    temp_password = 'Aa1!' + secrets.token_hex(24)
    maintenance("CREATE USER '%s'@'%%' IDENTIFIED BY '%s'" % (temp_user, temp_password))
    try:
        maintenance("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'%%'" % (database, temp_user))
        with gzip.open(bundle / 'local-data/database.sql.gz', 'rb') as source:
            # Import only with a temporary account scoped to the new database.
            mysql(cfg, tool, database=database, user=temp_user, password=temp_password, source=source)
        maintenance("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'%s'" % (database, parts[0], parts[1]))
    finally:
        maintenance("DROP USER IF EXISTS '%s'@'%%'" % temp_user)
    if table_counts(cfg, tool, database, manifest['table_counts']) != manifest['table_counts']:
        raise ValueError('Imported table counts differ; original site is unchanged')
    if mysql(cfg, tool, "SELECT COUNT(*) FROM accounts WHERE role='admin' AND password_hash<>''", database) == '0':
        raise ValueError('Snapshot has no administrator')
    target.mkdir(mode=0o700)
    media = target / 'media'
    shutil.copytree(bundle / 'local-data/media', media)
    for item in manifest['objects']:
        if digest(media / item['key']) != item['sha256']:
            raise ValueError('Copied media differs; original site is unchanged')
    mysql(cfg, tool, "UPDATE stored_objects SET store_id=0; UPDATE merchant_storage_quotas SET initialized=0;", database)
    owner = subprocess.check_output(['systemctl', 'show', args.service, '--property=User', '--value'], text=True).strip()
    if owner:
        # The service must traverse the private parent as well as the media dir.
        shutil.chown(target.parent, user=owner)
        for path in [target, *target.rglob('*')]:
            shutil.chown(path, user=owner)
    pointer = Path(info['operations']) / 'active.json'
    previous_pointer = pointer.read_bytes() if pointer.exists() else None
    subprocess.run(['systemctl', 'stop', args.service], check=True)
    switched = False
    try:
        _, latest = upgrade.context(args.env_file, args.install_dir)
        if latest != info or (pointer.read_bytes() if pointer.exists() else None) != previous_pointer:
            raise ValueError('Restore target changed concurrently; retry after reviewing current state')
        backup = Path(args.install_dir) / 'shared' / 'upgrade-backups' / ('before-local-data-' + stamp)
        upgrade.backup(args.env_file, args.install_dir, backup)
        # Preserve the server's domain/upload settings and site identity, not the local ones.
        maintenance("INSERT INTO `%s`.system_settings SELECT * FROM `%s`.system_settings WHERE setting_key IN ('system_config_v3','site_identity')" % (database, info['database']))
        write_pointer(pointer, {'current': {'database': database, 'media_dir': str(media)},
                                'previous': {'database': info['database'], 'media_dir': info['media']},
                                'pending': False, 'outcome': 'committed', 'job_id': 'local-data-' + stamp})
        switched = True
        subprocess.run(['systemctl', 'start', args.service], check=True)
        if not wait_healthy(info['port']):
            raise RuntimeError('New data target did not become healthy')
        print('Healthy. Local snapshot is active. Original DB/media retained. Backup: ' + str(backup))
        print('Login using the local snapshot accounts; old server accounts are retained in the previous DB, not merged.')
    except BaseException:
        if switched:
            subprocess.run(['systemctl', 'stop', args.service], check=False)
            if previous_pointer is None:
                pointer.unlink()
            else:
                write_pointer(pointer, json.loads(previous_pointer))
        subprocess.run(['systemctl', 'start', args.service], check=False)
        print('Import/cutover failed; original data target restored. New DB/media retained for diagnosis.', file=sys.stderr)
        raise


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError, RuntimeError, subprocess.SubprocessError) as error:
        print('Import stopped: ' + str(error), file=sys.stderr)
        sys.exit(1)
