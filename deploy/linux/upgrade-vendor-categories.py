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
from urllib.error import HTTPError
from urllib.parse import quote

VERSION = '1.11.0'
BASE = Path('/opt/zqzt')
ENV = Path('/etc/zqzt/exhibition.env')
STATE = BASE / 'shared/upgrade-v1.11.0-private.json'
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
    assert manifest['version'] == VERSION and manifest['schemaVersion'] == 31
    assert re.fullmatch(r'[a-f0-9]{40}', manifest['gitCommit'])
    assert manifest['android']['versionCode'] == 51
    assert manifest['android']['applicationId'] == 'com.zqzt.dalunongji'
    assert manifest['android']['apiBaseUrl'] == 'https://zqzhg.com'
    verify_inventory(package, manifest)
    apk = package / 'android/dalunongji.apk'
    assert sha(apk) == manifest['android']['sha256']
    assert json.loads((apk.parent / 'manifest.json').read_text()) == manifest['android']
    assert sorted(int(p.name.split('_')[0]) for p in (package / 'server/migrations').glob('*.sql')) == list(range(1, 32))
    return manifest


def verify_inventory(package, manifest):
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


def backup():
    assert not STATE.exists(), 'state_exists_use_status_or_rollback'
    cfg = config(); name, ops = database(cfg)
    assert mysql(cfg, 'SELECT MAX(version) FROM schema_migrations', name) in ['30', '31'], 'unexpected_schema'
    previous = (BASE / 'current').resolve(strict=True)
    assert previous.parent == BASE / 'releases'
    dest = BASE / 'shared/upgrade-backups' / ('categories-1.11.0-' + datetime.datetime.now().strftime('%Y%m%d-%H%M%S'))
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
    value = {'previous': str(previous), 'backup': str(dest), 'database': name, 'schema': int(mysql(cfg, 'SELECT MAX(version) FROM schema_migrations', name)), 'counts': counts(cfg, name), 'configSHA256': sha(ENV), 'protected': before, 'media': media_before, 'revisionSnapshot': revision_snapshot(cfg, name), 'backupVerified': True, 'backupFiles': inventory(dest)}
    save(value)
    print(json.dumps({'backupVerified': True, 'backup': str(dest), 'schema': value['schema'], 'mediaFiles': len(media_before), 'counts': value['counts']}))


def stage(package, prepare_web=None):
    value = state(); assert value['backupVerified']
    manifest = verify_package(package)
    release = BASE / 'releases' / ('v' + VERSION + '-' + manifest['gitCommit'][:7])
    assert not release.exists()
    release.mkdir(mode=0o755)
    for n in ['server', 'web', 'map-demo', 'android']:
        shutil.copytree(str(package / n), str(release / n))
    downloads_before = prepare_downloads(release, manifest)
    if prepare_web is not None:
        prepare_web(release, manifest)
    arch = subprocess.check_output(['uname', '-m']).decode().strip()
    assert arch == 'x86_64', 'this_package_requires_linux_amd64'
    os.chmod(str(release / 'server/exhibition'), 0o755)
    subprocess.run(['chown', '-R', 'root:zqzt', str(release)], check=True)
    os.chmod(str(release), 0o755)
    # The snapshot is public development-demo geometry, with unchanged source metadata.
    for p in release.rglob('*'): os.chmod(str(p), 0o755 if p.is_dir() or p == release / 'server/exhibition' else 0o644)
    for p in [release, release / 'server/exhibition', release / 'web/dist/index.html', release / 'map-demo/national.mapping.json']:
        subprocess.run(['runuser', '-u', 'zqzt', '--', 'test', '-r', str(p)], check=True)
    subprocess.run(['runuser', '-u', 'zqzt', '--', 'test', '-x', str(release / 'server/exhibition')], check=True)
    # The existing resolver atomically refreshes mapping JSON on every request.
    # Keep this cache writable by the service, outside the public Web directory.
    map_cache = release / 'map-demo'
    subprocess.run(['chown', '-R', 'zqzt:zqzt', str(map_cache)], check=True)
    for p in [map_cache] + list(map_cache.rglob('*')):
        os.chmod(str(p), 0o700 if p.is_dir() else 0o600)
    subprocess.run(['runuser', '-u', 'zqzt', '--', 'test', '-w', str(map_cache)], check=True)
    assert inventory(release / 'map-demo') == manifest['mapFiles']
    value.update(release=str(release), downloadsBefore=downloads_before, staged=True, candidateVerified=False, isolatedRestoreVerified=False, promoted=False, rolledBack=False, gitCommit=manifest['gitCommit'], android=manifest['android'])
    save(value)
    print(json.dumps({'staged': True, 'release': str(release), 'mapFiles': len(manifest['mapFiles']), 'android': manifest['android']['version']}))


def candidate(check=None):
    value = state(); assert value['staged']
    release = Path(value['release']); work = BASE / 'shared' / ('categories111-candidate-' + secrets.token_hex(6))
    work.mkdir(mode=0o700)
    for n in ['mysql', 'media', 'operations', 'temp', 'backups']: (work / n).mkdir(mode=0o700)
    import pwd
    service = pwd.getpwnam('zqzt')
    os.chown(str(work), 0, service.pw_gid); os.chmod(str(work), 0o750)
    for n in ['media', 'operations', 'temp', 'backups']:
        os.chown(str(work / n), service.pw_uid, service.pw_gid)
    def service_identity():
        os.initgroups('zqzt', service.pw_gid)
        os.setgid(service.pw_gid); os.setuid(service.pw_uid)
    secret = secrets.token_hex(24); db = 'categories111_candidate'
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
        env.update(APP_ENV='production', HTTP_ADDR='127.0.0.1:18099', DB_NAME=db, MEDIA_DIR=str(work / 'media'), OPERATIONS_DIR=str(work / 'operations'), MEDIA_TEMP_DIR=str(work / 'temp'), BACKUP_DIR=str(work / 'backups'), WEB_DIR=str(release / 'web/dist'), MAP_DEMO_ENABLED='true', MAP_DEMO_DIR=str(release / 'map-demo'), BACKGROUND_WORKERS_ENABLED='false', SMS_PROVIDER='disabled', SMS_ENABLED='false', DOUYIN_VIDEO_ENABLED='true')
        with (work / 'candidate.log').open('wb') as log:
            app = subprocess.Popen([str(release / 'server/exhibition')], cwd=str(work), env=env, stdout=log, stderr=log, preexec_fn=service_identity)
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
            assert mysql(cfg, 'SELECT MAX(version) FROM schema_migrations', db) == '31'
            assert counts(cfg, db) == value['counts']
            assert revision_snapshot(cfg, db) == value['revisionSnapshot'], 'audit_payload_changed'
            value['candidateCategories'] = category_checks(lambda path: get(path, 18098), cfg, db)
            if check is not None:
                value['candidateSEO'] = check(18098, release)
            value.update(candidateVerified=True, candidateChecks=checks, candidateMap=maps, isolatedRestoreVerified=True)
            save(value)
            print(json.dumps({'candidateVerified': True, 'productionConfiguration': True, 'serviceUser': 'zqzt', 'separateNginxProxy': True, 'isolatedRestoreVerified': True, 'checks': checks, 'map': maps}))
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


def apply(check=None, extra_environment=None):
    value = state(); assert value.get('candidateVerified')
    assert str((BASE / 'current').resolve()) == value['previous'] and sha(ENV) == value['configSHA256']
    assert protected() == value['protected'], 'protected_configuration_changed'
    cfg = config(); name, ops = database(cfg)
    assert name == value['database'] and int(mysql(cfg, 'SELECT MAX(version) FROM schema_migrations', name)) == value['schema']
    release = Path(value['release']); env = ENV.read_text()
    updates = [('WEB_DIR', str(release / 'web/dist')), ('MAP_DEMO_DIR', str(release / 'map-demo'))]
    updates += [('DOUYIN_VIDEO_ENABLED', 'true')] if extra_environment is None else extra_environment
    for k, v in updates:
        if not re.search(r'^' + k + r'=', env, re.M): env += '\n' + k + '=' + v + '\n'
        env = re.sub(r'^' + k + r'=.*$', k + '=' + v, env, flags=re.M)
    assert counts(cfg, name) == value['counts'], 'business_changed_since_backup'
    assert revision_snapshot(cfg, name) == value['revisionSnapshot'], 'revisions_changed_since_backup'
    assert inventory(Path(value['previous']) / 'web/dist/downloads') == value['downloadsBefore'], 'downloads_changed_since_stage'
    started = time.time()
    try:
        replace_environment(env.encode()); link_to(release)
        subprocess.run(['systemctl', 'restart', UNIT], check=True); health()
        assert get('/guide')[0] == 200
        assert sha(release / 'web/dist/downloads/dalunongji.apk') == value['android']['sha256']
        assert protected() == value['protected']
        assert counts(config(), name) == value['counts'], 'business_counts_changed'
        assert revision_snapshot(config(), name) == value['revisionSnapshot'], 'audit_payload_changed'
        value['productionCategories'] = category_checks(production_fetch, config(), name)
        assert mysql(config(), 'SELECT MAX(version) FROM schema_migrations', name) == '31'
        if check is not None:
            value['productionSEO'] = check(release)
    except Exception:
        rollback(); raise
    value.update(promoted=True, rolledBack=False, restartHealthSeconds=round(time.time() - started, 3))
    save(value)
    print(json.dumps({'promoted': True, 'version': VERSION, 'android': value['android']['version'], 'schema': 31, 'protectedConfigurationsUnchanged': True, 'restartHealthSeconds': value['restartHealthSeconds']}))


def revision_snapshot(cfg, name):
    raw = mysql(cfg, 'SELECT id,content_id,SHA2(payload,256) FROM revisions ORDER BY id', name)
    return hashlib.sha256(raw.encode('utf-8')).hexdigest()


def prepare_downloads(release, manifest):
    source = Path(config()['WEB_DIR']).resolve(strict=True) / 'downloads'
    previous = json.loads((source / 'manifest.json').read_text())
    assert previous['versionCode'] < manifest['android']['versionCode'], 'app_version_not_newer'
    assert previous['certificateSHA256'] == manifest['android']['certificateSHA256'], 'app_signing_changed'
    assert sha(source / 'dalunongji.apk') == previous['sha256'], 'previous_apk_mismatch'
    before = inventory(source)
    dest = release / 'web/dist/downloads'
    assert not dest.exists() and not source.is_symlink()
    assert not any(p.is_symlink() for p in source.rglob('*')), 'download_symlink'
    shutil.copytree(str(source), str(dest))
    assert inventory(dest) == before and inventory(source) == before
    archive = dest / 'archive'
    archive.mkdir(exist_ok=True)
    old = archive / ('dalunongji-' + previous['sha256'] + '.apk')
    if not old.exists():
        shutil.copy2(str(source / 'dalunongji.apk'), str(old))
    assert sha(old) == previous['sha256']
    (archive / ('manifest-' + previous['sha256'] + '.json')).write_text(json.dumps(previous, indent=2) + '\n')
    apk = release / 'android/dalunongji.apk'
    for name in ['dalunongji.apk', manifest['android']['asset']]:
        shutil.copy2(str(apk), str(dest / name))
    (dest / 'manifest.json').write_text(json.dumps(manifest['android'], indent=2) + '\n')
    for rel, digest in before.items():
        if rel not in ['dalunongji.apk', 'manifest.json']:
            assert sha(dest / rel) == digest, 'previous_download_changed'
    return before


def category_checks(fetch, cfg, name):
    def read(path):
        status, body = fetch('/api/v1' + path)
        assert status == 200, 'category_api_error_' + path
        return json.loads(body)
    bad = mysql(cfg, 'SELECT COUNT(*) FROM revision_vendor_product_categories b LEFT JOIN revisions r ON r.id=b.revision_id LEFT JOIN contents c ON c.id=r.content_id LEFT JOIN vendor_product_categories v ON v.id=b.category_id WHERE r.id IS NULL OR c.id IS NULL OR v.id IS NULL OR c.vendor_id<>b.vendor_id OR v.vendor_id<>b.vendor_id', name)
    assert bad == '0', 'company_category_isolation_failed'
    products = read('/public/products?page_size=100')['items']
    classified = [p for p in products if p['data'].get('vendor_product_category_id')]
    assert classified, 'no_migrated_public_product'
    product = classified[0]
    vid = product['vendor_id']
    cat = product['vendor_product_category']
    assert cat['vendor_id'] == vid
    query = '/public/products?vendor_id=' + str(vid) + '&vendor_product_category_id=' + str(cat['id'])
    selected = read(query)
    assert selected['total'] > 0 and all(p['vendor_id'] == vid and p['data']['vendor_product_category_id'] == cat['id'] for p in selected['items'])
    assert read('/public/products?vendor_product_category_id=' + str(cat['id']))['total'] == 0
    assert read('/public/products?vendor_id=18446744073709551614&vendor_product_category_id=' + str(cat['id']))['total'] == 0
    assert read('/public/products?q=' + quote(cat['name']))['total'] > 0
    slug = mysql(cfg, 'SELECT showroom_slug FROM contents WHERE id=' + str(vid), name)
    if not slug:
        slug = 'v-' + str(vid)
    showroom = read('/public/showrooms/' + slug)
    matching = [c for c in showroom['vendor_product_categories'] if c['id'] == cat['id']]
    assert len(matching) == 1 and matching[0]['product_count'] == selected['total'], 'public_category_count_mismatch'
    status, body = fetch('/products/' + str(product['id']))
    assert status == 200 and ('产品分类：' + cat['name']).encode('utf-8') in body
    scripts = re.findall(r'<script type="application/ld\+json">(.*?)</script>', body.decode('utf-8'), re.S)
    entities = []
    for raw in scripts:
        data = json.loads(raw)
        entities.extend(data.get('@graph', [data]))
    crumbs = next(v for v in entities if v.get('@type') == 'BreadcrumbList')
    assert len(crumbs['itemListElement']) == 2, 'product_breadcrumb_not_two_levels'
    return {'isolation': True, 'filter': True, 'search': True, 'publicCount': selected['total'], 'initialHTML': True, 'breadcrumbLevels': 2,
            'categories': int(mysql(cfg, 'SELECT COUNT(*) FROM vendor_product_categories', name)),
            'revisionBindings': int(mysql(cfg, 'SELECT COUNT(*) FROM revision_vendor_product_categories', name)),
            'unresolvedIndustries': int(mysql(cfg, 'SELECT COUNT(*) FROM vendor_product_categories WHERE industry_category_id=0', name))}


def production_fetch(path):
    raw = subprocess.check_output(['curl', '--noproxy', '*', '-sS', '--max-time', '30', '--resolve', 'zqzhg.com:443:127.0.0.1', '-w', '\n%{http_code}', 'https://zqzhg.com' + path])
    body, status = raw.rsplit(b'\n', 1)
    return int(status), body


def production_check(release):
    value = state()
    for row in value['candidateMap']:
        suffix = '?region_code=' + row['code'] if row['code'] else ''
        status, body = production_fetch('/api/v1/public/map-demo/regions' + suffix)
        assert status == 200 and json.loads(body)['sha256'] == row['sha256']
        status, body = production_fetch('/api/v1/public/industry-map/regions' + suffix)
        assert status == 200 and json.loads(body)['total'] == row['total']
    assert production_fetch('/api/v1/dev/map-preview/config')[0] == 404
    status, body = production_fetch('/downloads/manifest.json')
    assert status == 200 and json.loads(body) == value['android']
    status, body = production_fetch('/ByteDanceVerify.html')
    assert status == 200 and body == (release / 'web/dist/ByteDanceVerify.html').read_bytes()
    return {'https': True, 'mapLevels': 4, 'apk': True, 'byteDanceVerification': True}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['check', 'backup', 'stage', 'candidate', 'apply', 'rollback', 'status'])
    parser.add_argument('--package', type=Path, default=Path(__file__).resolve().parent)
    args = parser.parse_args()
    assert os.geteuid() == 0, 'root_required'
    os.umask(0o077)
    with (BASE / 'shared/upgrade-v1.11.0.lock').open('a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        if args.action == 'check': print(json.dumps({'packageVerified': bool(verify_package(args.package))}))
        elif args.action == 'status':
            v = state(); print(json.dumps({k: v.get(k) for k in ['backupVerified', 'staged', 'candidateVerified', 'promoted', 'rolledBack', 'backup', 'release', 'restartHealthSeconds']}))
        elif args.action == 'stage': stage(args.package)
        elif args.action == 'candidate': candidate()
        elif args.action == 'apply': apply(production_check)
        else: {'backup': backup, 'rollback': rollback}[args.action]()


if __name__ == '__main__':
    try: main()
    except Exception as e:
        print(json.dumps({'failed': type(e).__name__, 'reason': str(e)})); sys.exit(1)
