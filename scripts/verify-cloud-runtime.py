"""Smoke test production settings and restored data on Windows, using an isolated DB/user."""
from pathlib import Path
import gzip
import json
import os
import secrets
import subprocess
import time
import urllib.request
import urllib.error

ROOT = Path(__file__).resolve().parents[1]
config = {}
for line in (ROOT / 'server/.env').read_text(encoding='utf-8-sig').splitlines():
    if '=' in line and not line.lstrip().startswith('#'):
        key, value = line.split('=', 1)
        config[key.strip()] = value.strip().strip('"')
admin_env = {**os.environ, 'MYSQL_PWD': config.get('DB_PASSWORD', 'root')}
mysql_exe = os.environ.get('MYSQL_BIN', 'D:/InforGuardMC/InforGuardMC/mysql/bin') + '/mysql.exe'
args = ['--no-defaults', '--host=' + config.get('DB_HOST', '127.0.0.1'),
        '--port=' + config.get('DB_PORT', '13306'), '--user=' + config.get('DB_USER', 'root'),
        '--default-character-set=utf8mb4', '--batch', '--skip-column-names']

def mysql(sql, database=None):
    result = subprocess.run([mysql_exe, *args, *([database] if database else [])], input=sql.encode(),
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=admin_env)
    if result.returncode:
        raise RuntimeError('Test database operation failed: ' + result.stderr.decode(errors='replace'))
    return result.stdout.decode().replace('\r\n', '\n')

suffix = secrets.token_hex(5)
database = 'zqzt_exhibition_test_cloud_' + suffix
user = 'zqzt_test_' + suffix
password = secrets.token_hex(24)
process = None
created_db = False
created_user = False
opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
base = 'http://127.0.0.1:18089'

def get(path):
    with opener.open(base + path, timeout=5) as response:
        return response.read(), response.headers.get_content_type()

try:
    mysql(f'CREATE DATABASE `{database}` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci')
    created_db = True
    for host in ['localhost', '127.0.0.1']:
        mysql(f"CREATE USER '{user}'@'{host}' IDENTIFIED BY '{password}'; GRANT ALL ON `{database}`.* TO '{user}'@'{host}'")
        created_user = True
    with gzip.open(ROOT / '.local/cloud-data/migration/database.sql.gz', 'rb') as source:
        mysql(source.read().decode('utf-8'), database)
    env = {**os.environ, 'APP_ENV': 'production', 'HTTP_ADDR': '127.0.0.1:18089',
           'DB_HOST': config.get('DB_HOST', '127.0.0.1'), 'DB_PORT': config.get('DB_PORT', '13306'),
           'DB_USER': user, 'DB_PASSWORD': password, 'DB_NAME': database,
           'ADMIN_PASSWORD': secrets.token_hex(24), 'SMS_PROVIDER': 'disabled',
           'PUBLIC_BASE_URL': 'http://39.97.250.244:8089', 'APP_DOWNLOAD_BASE_URL': 'http://39.97.250.244:8089',
           'CORS_ORIGIN': 'http://39.97.250.244:8089', 'WEB_DIR': str(ROOT / 'web/dist'),
           'MEDIA_DIR': str(ROOT / '.local/cloud-data/migration/media')}
    cwd = ROOT / '.local/cloud-smoke'
    cwd.mkdir(exist_ok=True)
    with (cwd / 'stdout.log').open('wb') as stdout, (cwd / 'stderr.log').open('wb') as stderr:
        process = subprocess.Popen([str(ROOT / '.local/cloud-bin/windows/exhibition.exe')], cwd=cwd,
                                   env=env, stdout=stdout, stderr=stderr, creationflags=subprocess.CREATE_NO_WINDOW)
        for attempt in range(40):
            if process.poll() is not None:
                raise RuntimeError('Production application exited; inspect private smoke-test logs')
            try:
                health = json.loads(get('/api/v1/health')[0])
                if health.get('status') == 'ok':
                    break
            except (OSError, ValueError):
                time.sleep(.25)
        else:
            raise RuntimeError('Production health check timed out')
        assert health['sms_provider'] == 'disabled'
        for page in ['/', '/admin', '/app']:
            body, mime = get(page)
            assert mime == 'text/html' and b'<div id="root">' in body
        home = json.loads(get('/api/v1/public/home')[0])
        assert home['vendor_count'] > 0
        body, _ = get('/api/v1/public/app')
        assert b'http://39.97.250.244:8089' in body and b'172.27.' not in body
        media_ids = mysql('SELECT DISTINCT m.id FROM media m JOIN media_references r ON r.media_id=m.id '
                          'JOIN contents c ON c.published_revision_id=r.revision_id WHERE c.visible=1 AND '
                          "(c.kind='vendor' OR EXISTS (SELECT 1 FROM contents v WHERE v.id=c.vendor_id AND v.visible=1))", database).strip().splitlines()
        checked = 0
        for media_id in media_ids:
            for tail in ['', '?thumb=1']:
                body, mime = get('/api/v1/media/' + media_id + tail)
                assert body and mime.startswith('image/')
                checked += 1
        for path in ['/api/v1/auth/sms', '/api/v1/auth/phone']:
            request = urllib.request.Request(base + path, data=b'{"phone":"13900000001","code":"123456"}', headers={'Content-Type': 'application/json'})
            try:
                opener.open(request, timeout=5)
                raise AssertionError('SMS should be disabled')
            except urllib.error.HTTPError as error:
                assert error.code == 503 and b'sms_disabled' in error.read()
        result = {'platform': 'Windows runtime; Linux executables are cross-compiled and not run here',
                  'productionStartupWithDatabaseScopedAccount': 'passed', 'restoredSnapshot': 'passed',
                  'publicPages': ['/', '/admin', '/app'], 'publicMediaResponsesChecked': checked,
                  'appDownloadOrigin': 'http://39.97.250.244:8089', 'disabledSMSEndpoints': '503 sms_disabled'}
        (ROOT / 'docs/test-output/aliyun-runtime-smoke.json').write_text(json.dumps(result, indent=2), encoding='utf-8')
        print(json.dumps(result, indent=2))
finally:
    if process is not None:
        process.terminate()
        process.wait(timeout=10)
    if created_db:
        mysql(f'DROP DATABASE `{database}`')
    if created_user:
        for host in ['localhost', '127.0.0.1']:
            mysql(f"DROP USER IF EXISTS '{user}'@'{host}'")
