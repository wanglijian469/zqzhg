"""ECS operator: back up the active deployment, optionally clone an isolated DB.

Does not switch services, change the active DB, send SMS or print credentials.
Run with Python 3.11. Private files remain on the ECS under the backup directory.
"""
import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys


def read_env(path):
    result = {}
    for line in path.read_text(encoding='utf-8-sig').splitlines():
        if '=' in line and not line.lstrip().startswith('#'):
            key, value = line.split('=', 1)
            result[key.strip()] = value.strip().strip('"')
    return result


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--clone', action='store_true')
    p.add_argument('--env', default='/etc/zqzt/exhibition.env')
    a = p.parse_args()
    config_path = Path(a.env).resolve(strict=True)
    cfg = read_env(config_path)
    database = cfg['DB_NAME']
    media = Path(cfg['MEDIA_DIR'])
    control = Path(cfg.get('OPERATIONS_DIR', str(media.parent / ('.operations-' + media.name))))
    if (control / 'active.json').exists():
        host = json.loads((control / 'active.json').read_text())
        if host.get('pending'):
            raise RuntimeError('active_restore_pending')
        target = host.get('current') or {}
        database = target.get('database') or database
        media = Path(target.get('media_dir') or media)
    if not re.fullmatch(r'[A-Za-z0-9_]{1,64}', database):
        raise RuntimeError('unsafe_database_name')
    stamp = datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ')
    destination = Path('/opt/zqzt/shared/upgrade-backups') / ('sms-' + stamp)
    destination.mkdir(mode=0o700, parents=True, exist_ok=False)
    os.chmod(destination, 0o700)
    shutil.copy2(config_path, destination / 'exhibition.env')
    os.chmod(destination / 'exhibition.env', 0o600)
    current = Path('/opt/zqzt/current').resolve(strict=True)
    (destination / 'previous-release.txt').write_text(str(current) + '\n')
    shutil.copy2(current / 'server/exhibition', destination / 'previous-exhibition')

    def run_mysql(sql, db=None, maintenance=False, input_stream=None):
        user = cfg.get('DB_MAINTENANCE_USER') if maintenance else cfg.get('DB_USER')
        password = cfg.get('DB_MAINTENANCE_PASSWORD') if maintenance else cfg.get('DB_PASSWORD')
        if not user:
            user, password = cfg.get('DB_USER'), cfg.get('DB_PASSWORD')
        env = dict(os.environ, MYSQL_PWD=password or '')
        args = ['mysql', '--host=' + cfg.get('DB_HOST', '127.0.0.1'), '--port=' + cfg.get('DB_PORT', '3306'), '--user=' + user, '--batch', '--skip-column-names', '--binary-mode=1', '--local-infile=0']
        if db:
            args.append(db)
        if sql is not None:
            args.extend(['-e', sql])
        return subprocess.run(args, env=env, stdin=input_stream, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=180)

    counts = run_mysql('SELECT (SELECT COUNT(*) FROM accounts),(SELECT COUNT(*) FROM contents),(SELECT COUNT(*) FROM media),(SELECT MAX(version) FROM schema_migrations);', database)
    if counts.returncode:
        raise RuntimeError('database_baseline_read_failed')
    sql = destination / 'database.sql'
    env = dict(os.environ, MYSQL_PWD=cfg.get('DB_PASSWORD', ''))
    with sql.open('wb') as stream:
        args = ['mysqldump', '--host=' + cfg.get('DB_HOST', '127.0.0.1'), '--port=' + cfg.get('DB_PORT', '3306'), '--user=' + cfg['DB_USER'], '--single-transaction', '--no-tablespaces', '--hex-blob', '--column-statistics=0', '--set-gtid-purged=OFF', '--skip-comments', database]
        dump = subprocess.run(args, env=env, stdout=stream, stderr=subprocess.PIPE, timeout=180)
    os.chmod(sql, 0o600)
    if dump.returncode or sql.stat().st_size == 0:
        raise RuntimeError('database_backup_failed')
    manifest = {'backup_directory': str(destination), 'previous_release': str(current), 'active_database': database, 'baseline_accounts_contents_media_schema': counts.stdout.decode().strip().split('\t'), 'sql_sha256': hashlib.sha256(sql.read_bytes()).hexdigest(), 'sql_bytes': sql.stat().st_size, 'production_switched': False}
    if a.clone:
        candidate = 'zqzt_sms_acceptance_' + stamp.replace('T', '_').replace('Z', '')
        created = run_mysql('CREATE DATABASE `' + candidate + '` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;', maintenance=True)
        if created.returncode:
            manifest['candidate_status'] = 'maintenance_create_database_permission_required'
        else:
            # Current application account remains scoped to its authorized DBs.
            if cfg.get('DB_MAINTENANCE_USER'):
                user, host = run_mysql('SELECT CURRENT_USER();').stdout.decode().strip().split('@')
                if not re.fullmatch(r'[A-Za-z0-9_.%-]+', user) or not re.fullmatch(r'[A-Za-z0-9_.%:-]+', host):
                    raise RuntimeError('unsafe_application_identity')
                grant = run_mysql("GRANT ALL PRIVILEGES ON `" + candidate + "`.* TO '" + user + "'@'" + host + "';", maintenance=True)
                if grant.returncode:
                    raise RuntimeError('candidate_grant_failed')
            with sql.open('rb') as stream:
                restored = run_mysql(None, candidate, maintenance=True, input_stream=stream)
            if restored.returncode:
                raise RuntimeError('candidate_import_failed')
            manifest.update(candidate_database=candidate, candidate_status='cloned')
    (destination / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
    print(json.dumps(manifest))


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        # Do not print raw subprocess/DB messages or any parsed configuration.
        print(json.dumps({'error': str(error) if isinstance(error, RuntimeError) else type(error).__name__}), file=sys.stderr)
        sys.exit(1)
