#!/usr/bin/env python3
"""Existing /opt/zqzt installation only. Keep credentials and backups on the host."""
import argparse
import datetime
import fcntl
import gzip
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import subprocess
import sys
import tarfile
import time
import urllib.request

VERSION = '1.10.6'
BASE = Path('/opt/zqzt')
ENV = Path('/etc/zqzt/exhibition.env')
STATE = BASE / 'shared/upgrade-v1.10.6-private.json'
UNIT = 'zqzt-exhibition.service'


def sha(path):
    h = hashlib.sha256()
    with Path(path).open('rb') as f:
        for part in iter(lambda: f.read(1024 * 1024), b''):
            h.update(part)
    return h.hexdigest()


def inventory(folder):
    return {str(p.relative_to(folder)): sha(p) for p in sorted(Path(folder).rglob('*')) if p.is_file()}


def config():
    result = {}
    for line in ENV.read_text().splitlines():
        if '=' in line and not line.lstrip().startswith('#'):
            k, v = line.split('=', 1)
            result[k.strip()] = v.strip().strip('"').strip("'")
    assert result['APP_ENV'] == 'production'
    return result


def database(cfg):
    media = Path(cfg['MEDIA_DIR'])
    ops = Path(cfg.get('OPERATIONS_DIR') or str(media.parent / ('.operations-' + media.name)))
    active = json.loads((ops / 'active.json').read_text()) if (ops / 'active.json').exists() else {}
    assert not active.get('pending'), 'restore_in_progress'
    name = active.get('current', {}).get('database') or cfg['DB_NAME']
    assert re.fullmatch(r'[A-Za-z0-9_]+', name)
    return name, ops


def mysql(cfg, sql, name=None):
    args = ['mysql', '--protocol=TCP', '--host=' + cfg.get('DB_HOST', '127.0.0.1'), '--port=' + cfg.get('DB_PORT', '3306'), '--user=' + cfg['DB_USER'], '--batch', '--skip-column-names', '--binary-mode=1']
    r = subprocess.run(args + ([name] if name else []), input=sql.encode(), env=dict(os.environ, MYSQL_PWD=cfg.get('DB_PASSWORD', '')), stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if r.returncode:
        raise RuntimeError('database_operation_failed')
    return r.stdout.decode().strip()


def protected():
    result = {}
    for folder in ['/etc/nginx', '/etc/letsencrypt/live']:
        for p in Path(folder).rglob('*'):
            if p.is_file():
                result[str(p)] = sha(p)
    for name in ['zqzt-exhibition.service', 'zqzhg-cert-renew.service', 'zqzhg-cert-renew.timer', 'qiyuan-gosyncr.service', 'gosync-server.service', 'dalu-parts.service']:
        p = Path('/etc/systemd/system') / name
        if p.is_file():
            result[str(p)] = sha(p)
    return result


def counts(cfg, name):
    return mysql(cfg, 'SELECT (SELECT COUNT(*) FROM accounts),(SELECT COUNT(*) FROM contents),(SELECT COUNT(*) FROM revisions),(SELECT COUNT(*) FROM media)', name)


def save(value):
    tmp = STATE.with_suffix('.next')
    tmp.write_text(json.dumps(value, indent=2))
    os.chmod(str(tmp), 0o600)
    os.replace(str(tmp), str(STATE))


def state():
    return json.loads(STATE.read_text())


def get(path, port=8089):
    request = urllib.request.Request('http://127.0.0.1:' + str(port) + path, headers={'Host': 'zqzhg.com', 'X-Forwarded-Proto': 'https'})
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    with opener.open(request, timeout=15) as r:
        return r.status, r.read()


def verify_package(package):
    package = package.resolve(strict=True)
    manifest = json.loads((package / 'manifest.json').read_text())
    assert manifest['version'] == VERSION and manifest['schemaVersion'] == 28
    assert re.fullmatch(r'[a-f0-9]{40}', manifest['gitCommit'])
    assert manifest['android']['versionCode'] == 44
    sums = (package / 'SHA256SUMS').read_bytes()
    assert b'\r' not in sums and sums.endswith(b'\n')
    seen = set()
    for line in sums.decode().splitlines():
        digest, rel = line.split('  ', 1)
        p = (package / rel).resolve(strict=True)
        assert package in p.parents and rel not in seen and not (package / rel).is_symlink()
        assert re.fullmatch(r'[a-f0-9]{64}', digest) and sha(p) == digest, 'package_hash_mismatch'
        seen.add(rel)
    assert seen == set(inventory(package)) - {'SHA256SUMS'}, 'unlisted_package_file'
    assert inventory(package / 'map-demo') == manifest['mapFiles']
    apk = package / 'web/dist/downloads/dalunongji.apk'
    assert sha(apk) == manifest['android']['sha256']
    assert json.loads((apk.parent / 'manifest.json').read_text()) == manifest['android']
    assert sorted(int(p.name.split('_')[0]) for p in (package / 'server/migrations').glob('*.sql')) == list(range(1, 29))
    return manifest


def backup():
    assert not STATE.exists(), 'state_exists_use_status_or_rollback'
    cfg = config(); name, ops = database(cfg)
    assert mysql(cfg, 'SELECT MAX(version) FROM schema_migrations', name) == '28', 'unexpected_schema'
    previous = (BASE / 'current').resolve(strict=True)
    assert previous.parent == BASE / 'releases'
    dest = BASE / 'shared/upgrade-backups' / ('guide-1.10.6-' + datetime.datetime.now().strftime('%Y%m%d-%H%M%S'))
    dest.mkdir(mode=0o700, parents=True)
    shutil.copy2(str(ENV), str(dest / 'exhibition.env'))
    before = protected()
    with tarfile.open(str(dest / 'configurations.tar.gz'), 'w:gz') as out:
        for p in ['/etc/nginx', '/etc/systemd/system/' + UNIT, '/etc/systemd/system/zqzhg-cert-renew.timer', '/etc/systemd/system/zqzhg-cert-renew.service']:
            if Path(p).exists(): out.add(p, arcname=p.lstrip('/'))
        if ops.exists(): out.add(str(ops), arcname='private-operation-state')
    subprocess.run(['tar', '-czf', str(dest / 'release.tar.gz'), '-C', str(previous), '.'], check=True)
    args = ['mysqldump', '--protocol=TCP', '--host=' + cfg.get('DB_HOST', '127.0.0.1'), '--port=' + cfg.get('DB_PORT', '3306'), '--user=' + cfg['DB_USER'], '--single-transaction', '--hex-blob', '--set-gtid-purged=OFF', '--no-tablespaces', name]
    proc = subprocess.Popen(args, env=dict(os.environ, MYSQL_PWD=cfg.get('DB_PASSWORD', '')), stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    with gzip.open(str(dest / 'database.sql.gz'), 'wb') as out:
        shutil.copyfileobj(proc.stdout, out)
    proc.stderr.read()
    assert proc.wait() == 0, 'database_backup_failed'
    tail = b''
    with gzip.open(str(dest / 'database.sql.gz'), 'rb') as inp:
        for part in iter(lambda: inp.read(1024 * 1024), b''): tail = (tail + part)[-4096:]
    assert b'Dump completed' in tail
    media = Path(cfg['MEDIA_DIR']).resolve(strict=True)
    media_before = inventory(media)
    subprocess.run(['tar', '-czf', str(dest / 'media.tar.gz'), '-C', str(media), '.'], check=True)
    assert inventory(media) == media_before, 'media_changed_during_backup_retry_required'
    for n in ['release.tar.gz', 'media.tar.gz', 'configurations.tar.gz']:
        subprocess.run(['tar', '-tzf', str(dest / n)], stdout=subprocess.DEVNULL, check=True)
    assert protected() == before
    value = {'previous': str(previous), 'backup': str(dest), 'database': name, 'schema': 28, 'counts': counts(cfg, name), 'configSHA256': sha(ENV), 'protected': before, 'media': media_before, 'backupVerified': True, 'backupFiles': inventory(dest)}
    save(value)
    print(json.dumps({'backupVerified': True, 'backup': str(dest), 'schema': 28, 'mediaFiles': len(media_before), 'counts': value['counts']}))


def stage(package):
    value = state(); assert value['backupVerified']
    manifest = verify_package(package)
    release = BASE / 'releases' / ('v1.10.6-' + manifest['gitCommit'][:7])
    assert not release.exists()
    release.mkdir(mode=0o755)
    for n in ['server', 'web', 'map-demo']:
        shutil.copytree(str(package / n), str(release / n))
    arch = subprocess.check_output(['uname', '-m']).decode().strip()
    assert arch == 'x86_64', 'this_package_requires_linux_amd64'
    os.chmod(str(release / 'server/exhibition'), 0o755)
    subprocess.run(['chown', '-R', 'root:zqzt', str(release)], check=True)
    # The snapshot is public development-demo geometry, with unchanged source metadata.
    for p in release.rglob('*'): os.chmod(str(p), 0o755 if p.is_dir() or p == release / 'server/exhibition' else 0o644)
    assert inventory(release / 'map-demo') == manifest['mapFiles']
    value.update(release=str(release), staged=True, gitCommit=manifest['gitCommit'], android=manifest['android'])
    save(value)
    print(json.dumps({'staged': True, 'release': str(release), 'mapFiles': len(manifest['mapFiles']), 'android': manifest['android']['version']}))


def candidate():
    value = state(); assert value['staged']
    release = Path(value['release']); work = BASE / 'shared' / ('guide106-candidate-' + secrets.token_hex(6))
    work.mkdir(mode=0o700)
    for n in ['mysql', 'media', 'operations', 'temp', 'backups']: (work / n).mkdir(mode=0o700)
    secret = secrets.token_hex(24); db = 'guide106_candidate'
    common = ['/usr/sbin/mysqld', '--no-defaults', '--user=root', '--datadir=' + str(work / 'mysql'), '--innodb-buffer-pool-size=32M', '--innodb-log-buffer-size=4M', '--innodb-redo-log-capacity=32M', '--performance-schema=OFF', '--max-connections=12', '--table-open-cache=128', '--mysqlx=OFF', '--skip-log-bin', '--bind-address=127.0.0.1', '--port=13399', '--socket=' + str(work / 'mysql.sock'), '--pid-file=' + str(work / 'mysql.pid'), '--log-error=' + str(work / 'mysql.log')]
    isolated = app = proxy = None
    try:
        subprocess.run(common + ['--initialize-insecure'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True)
        init = work / 'mysql-init.sql'; init.write_text("ALTER USER 'root'@'localhost' IDENTIFIED BY '" + secret + "';"); os.chmod(str(init), 0o600)
        isolated = subprocess.Popen(common + ['--init-file=' + str(init)], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        cfg = dict(config(), DB_HOST='127.0.0.1', DB_PORT='13399', DB_USER='root', DB_PASSWORD=secret, DB_MAINTENANCE_USER='', DB_MAINTENANCE_PASSWORD='')
        for i in range(100):
            assert isolated.poll() is None, 'isolated_mysql_failed'
            try:
                if mysql(cfg, 'SELECT 1') == '1': break
            except Exception: time.sleep(.2)
        else: raise RuntimeError('isolated_mysql_timeout')
        init.unlink(); mysql(cfg, 'CREATE DATABASE ' + db + ' CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci')
        restore = subprocess.Popen(['mysql', '--protocol=TCP', '--host=127.0.0.1', '--port=13399', '--user=root', '--binary-mode=1', db], env=dict(os.environ, MYSQL_PWD=secret), stdin=subprocess.PIPE, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)
        with gzip.open(str(Path(value['backup']) / 'database.sql.gz'), 'rb') as source: shutil.copyfileobj(source, restore.stdin)
        restore.stdin.close(); restore.stderr.read(); assert restore.wait() == 0, 'isolated_restore_failed'
        subprocess.run(['tar', '-xzf', str(Path(value['backup']) / 'media.tar.gz'), '-C', str(work / 'media')], check=True)
        assert inventory(work / 'media') == value['media']
        env = dict(os.environ, **cfg)
        env.update(APP_ENV='production', HTTP_ADDR='127.0.0.1:18099', DB_NAME=db, MEDIA_DIR=str(work / 'media'), OPERATIONS_DIR=str(work / 'operations'), MEDIA_TEMP_DIR=str(work / 'temp'), BACKUP_DIR=str(work / 'backups'), WEB_DIR=str(release / 'web/dist'), MAP_DEMO_ENABLED='true', MAP_DEMO_DIR=str(release / 'map-demo'), BACKGROUND_WORKERS_ENABLED='false', SMS_PROVIDER='disabled', SMS_ENABLED='false')
        with (work / 'candidate.log').open('wb') as log:
            app = subprocess.Popen([str(release / 'server/exhibition')], cwd=str(work), env=env, stdout=log, stderr=log)
            for i in range(100):
                assert app.poll() is None, 'candidate_start_failed'
                try:
                    if get('/api/v1/health', 18099)[0] == 200: break
                except Exception: time.sleep(.2)
            else: raise RuntimeError('candidate_timeout')
            proxycfg = work / 'nginx.conf'
            proxycfg.write_text('pid ' + str(work / 'nginx.pid') + '; error_log ' + str(work / 'nginx.log') + '; events {} http { access_log off; server { listen 127.0.0.1:18098; location / { proxy_pass http://127.0.0.1:18099; proxy_set_header Host zqzhg.com; proxy_set_header X-Forwarded-Proto https; } } }')
            subprocess.run(['nginx', '-t', '-c', str(proxycfg), '-p', str(work)], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True)
            proxy = subprocess.Popen(['nginx', '-c', str(proxycfg), '-p', str(work), '-g', 'daemon off;'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            time.sleep(.3)
            checks = []
            for path in ['/guide', '/join', '/merchant/login', '/service-requests', '/api/v1/health', '/api/v1/public/home', '/sitemaps/pages.xml', '/guide-images/register.png']:
                if path.endswith('register.png'):
                    images = sorted((release / 'web/dist/guide-images').glob('*.png')); assert images
                    path = '/guide-images/' + images[0].name
                code, body = get(path, 18098); assert code == 200 and body
                if path == '/guide': assert '厂商入驻与使用指南' in body.decode()
                if path == '/sitemaps/pages.xml': assert b'/guide</loc>' in body
                checks.append(path)
            maps = []
            for code in ['', '13', '1305', '130528']:
                suffix = '?region_code=' + code if code else ''
                stats = json.loads(get('/api/v1/public/industry-map/regions' + suffix, 18098)[1])
                boundary = json.loads(get('/api/v1/public/map-demo/regions' + suffix, 18098)[1])
                expected = json.loads((release / 'map-demo' / ((code or 'national') + '.mapping.json')).read_text())
                assert boundary['sha256'] == expected['sha256'] and boundary['geometry'] == expected['geometry']
                assert boundary['geometry']['features']
                maps.append({'code': code, 'total': stats['total'], 'sha256': boundary['sha256']})
            assert mysql(cfg, 'SELECT MAX(version) FROM schema_migrations', db) == '28'
            assert counts(cfg, db) == value['counts']
            value.update(candidateVerified=True, candidateChecks=checks, candidateMap=maps, isolatedRestoreVerified=True)
            save(value)
            print(json.dumps({'candidateVerified': True, 'productionConfiguration': True, 'separateNginxProxy': True, 'isolatedRestoreVerified': True, 'checks': checks, 'map': maps}))
    finally:
        for p in [proxy, app, isolated]:
            if p is not None and p.poll() is None:
                p.terminate()
                try: p.wait(timeout=30)
                except subprocess.TimeoutExpired: p.kill(); p.wait()
        # The isolated data directory stays private for inspection; production DB/users are untouched.


def replace_environment(source):
    temporary = ENV.with_suffix('.upgrade-next')
    temporary.write_bytes(source); os.chmod(str(temporary), 0o600)
    os.replace(str(temporary), str(ENV))


def link_to(target):
    link = BASE / 'current-guide-next'
    assert not link.exists() and not link.is_symlink()
    link.symlink_to(target); os.replace(str(link), str(BASE / 'current'))


def health():
    for i in range(100):
        try:
            if get('/api/v1/health')[0] == 200: return
        except Exception: time.sleep(.2)
    raise RuntimeError('health_timeout')


def rollback():
    value = state(); current = (BASE / 'current').resolve()
    assert str(current) in [value['previous'], value.get('release')]
    replace_environment((Path(value['backup']) / 'exhibition.env').read_bytes())
    link_to(Path(value['previous']))
    subprocess.run(['systemctl', 'restart', UNIT], check=True); health()
    value['rolledBack'] = True; save(value)
    print(json.dumps({'rolledBack': True, 'databaseRestored': False}))


def apply():
    value = state(); assert value.get('candidateVerified')
    assert str((BASE / 'current').resolve()) == value['previous'] and sha(ENV) == value['configSHA256']
    assert protected() == value['protected'], 'protected_configuration_changed'
    cfg = config(); name, ops = database(cfg)
    assert name == value['database'] and mysql(cfg, 'SELECT MAX(version) FROM schema_migrations', name) == '28'
    release = Path(value['release']); env = ENV.read_text()
    for k, v in [('WEB_DIR', str(release / 'web/dist')), ('MAP_DEMO_DIR', str(release / 'map-demo'))]:
        assert re.search(r'^' + k + r'=', env, re.M)
        env = re.sub(r'^' + k + r'=.*$', k + '=' + v, env, flags=re.M)
    started = time.time()
    try:
        replace_environment(env.encode()); link_to(release)
        subprocess.run(['systemctl', 'restart', UNIT], check=True); health()
        assert get('/guide')[0] == 200
        assert sha(release / 'web/dist/downloads/dalunongji.apk') == value['android']['sha256']
        assert protected() == value['protected']
        assert mysql(config(), 'SELECT MAX(version) FROM schema_migrations', name) == '28'
    except Exception:
        rollback(); raise
    value.update(promoted=True, restartHealthSeconds=round(time.time() - started, 3))
    save(value)
    print(json.dumps({'promoted': True, 'version': VERSION, 'android': value['android']['version'], 'schema': 28, 'protectedConfigurationsUnchanged': True, 'restartHealthSeconds': value['restartHealthSeconds']}))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['check', 'backup', 'stage', 'candidate', 'apply', 'rollback', 'status'])
    parser.add_argument('--package', type=Path, default=Path(__file__).resolve().parent)
    args = parser.parse_args()
    assert os.geteuid() == 0, 'root_required'
    os.umask(0o077)
    with (BASE / 'shared/upgrade-v1.10.6.lock').open('a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        if args.action == 'check': print(json.dumps({'packageVerified': bool(verify_package(args.package))}))
        elif args.action == 'status':
            v = state(); print(json.dumps({k: v.get(k) for k in ['backupVerified', 'staged', 'candidateVerified', 'promoted', 'rolledBack', 'backup', 'release', 'restartHealthSeconds']}))
        elif args.action == 'stage': stage(args.package)
        else: {'backup': backup, 'candidate': candidate, 'apply': apply, 'rollback': rollback}[args.action]()


if __name__ == '__main__':
    try: main()
    except Exception as e:
        print(json.dumps({'failed': type(e).__name__, 'reason': str(e)})); sys.exit(1)
