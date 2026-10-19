"""Unit checks plus a real, isolated MySQL restore/Windows production smoke test."""
import gzip
import importlib.util
import json
import os
from pathlib import Path
import secrets
import shutil
import subprocess
import sys
import tempfile
import time
import unittest
import urllib.request
from unittest.mock import patch, MagicMock

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('snapshot_import', ROOT / 'deploy/linux/import-local-data.py')
imp = importlib.util.module_from_spec(spec)
spec.loader.exec_module(imp)


class ImportTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        (self.root / 'media').mkdir()
        (self.root / 'media/photo').write_bytes(b'picture')
        (self.root / 'database.sql.gz').write_bytes(gzip.compress(b'SELECT 1;'))
        self.manifest = {'schema': 15, 'kind': 'private-local-snapshot', 'objects': [
            {'key': 'photo', 'size': 7, 'sha256': imp.digest(self.root / 'media/photo')}]}
        self.write()

    def write(self):
        (self.root / 'manifest.json').write_text(json.dumps(self.manifest), encoding='utf-8')
        (self.root / 'SHA256SUMS').write_text(''.join(imp.digest(p) + '  ' + p.relative_to(self.root).as_posix() + '\n'
                                                  for p in sorted(self.root.rglob('*')) if p.is_file() and p.name != 'SHA256SUMS'), encoding='utf-8')

    def test_safe_keys(self):
        for value in ('', '../a', '/root', 'a/../b', 'a//b', 'C:/file', 'a\\b', 'a\x00b'):
            self.assertFalse(imp.safe_key(value), value)
        self.assertTrue(imp.safe_key('companies/1/logo/uuid.webp'))

    def test_complete_snapshot(self):
        self.assertEqual(imp.verify_snapshot(self.root), self.manifest)

    def test_corrupt_media_rejected(self):
        (self.root / 'media/photo').write_bytes(b'changed')
        self.assertRaisesRegex(ValueError, 'Checksum', imp.verify_snapshot, self.root)

    def test_missing_sql_checksum_rejected(self):
        path = self.root / 'SHA256SUMS'
        path.write_text('\n'.join(line for line in path.read_text().splitlines() if 'database.sql.gz' not in line))
        self.assertRaisesRegex(ValueError, 'Incomplete', imp.verify_snapshot, self.root)

    def test_unknown_schema_rejected(self):
        self.manifest['schema'] = 16
        self.write()
        self.assertRaisesRegex(ValueError, 'Schema15', imp.verify_snapshot, self.root)

    def test_traversal_manifest_rejected(self):
        with (self.root / 'SHA256SUMS').open('a') as file:
            file.write('0' * 64 + '  ../outside\n')
        self.assertRaisesRegex(ValueError, 'Invalid', imp.verify_snapshot, self.root)

    def test_duplicate_checksum_rejected(self):
        path = self.root / 'SHA256SUMS'
        path.write_text(path.read_text() * 2)
        self.assertRaisesRegex(ValueError, 'Duplicate', imp.verify_snapshot, self.root)

    def test_gzip_decompressed_and_password_not_in_args(self):
        with patch.object(imp.subprocess, 'run', return_value=subprocess.CompletedProcess([], 0, b'ok', b'')) as run:
            with gzip.open(self.root / 'database.sql.gz', 'rb') as source:
                imp.mysql({'DB_PASSWORD': 'private-password'}, 'mysql', database='isolated_db', source=source)
        call = run.call_args
        self.assertEqual(call[1]['input'], b'SELECT 1;')
        self.assertNotIn('private-password', ' '.join(call[0][0]))

    def test_sql_failure_redacts_credentials(self):
        with patch.object(imp.subprocess, 'run', return_value=subprocess.CompletedProcess([], 1, b'', b'secret')):
            with self.assertRaises(RuntimeError) as result:
                imp.mysql({}, 'mysql', 'secret sql')
        self.assertNotIn('secret', str(result.exception))

    def test_database_and_table_names_checked(self):
        self.assertRaises(ValueError, imp.mysql, {}, 'mysql', 'SELECT 1', 'bad`; DROP DATABASE test')
        self.assertRaises(ValueError, imp.table_counts, {}, 'mysql', 'test', ['bad`'])

    def test_pointer_preserves_previous_target(self):
        pointer = self.root / 'active.json'
        old = {'current': {'database': 'old', 'media_dir': '/data/old'}}
        imp.write_pointer(pointer, old)
        new = {'current': {'database': 'new', 'media_dir': '/data/new'}, 'previous': old['current'], 'pending': False}
        imp.write_pointer(pointer, new)
        self.assertEqual(json.loads(pointer.read_text()), new)
        self.assertEqual(list(self.root.glob('*.tmp')), [])

    def test_new_pointer_inherits_service_directory_owner(self):
        pointer = self.root / 'active.json'
        metadata = self.root.stat()
        with patch.object(imp.os, 'fchown', create=True) as chown:
            imp.write_pointer(pointer, {'current': {'database': 'new'}})
        self.assertEqual(chown.call_args[0][1:], (metadata.st_uid, metadata.st_gid))


class ImportFlowTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.ops = self.root / 'operations'
        self.ops.mkdir()
        self.pointer = self.ops / 'active.json'
        self.old = {'current': {'database': 'old_business', 'media_dir': str(self.root / 'old-media')}, 'job_id': 'previous'}
        imp.write_pointer(self.pointer, self.old)
        data = self.root / 'local-data'
        (data / 'media').mkdir(parents=True)
        (data / 'media/photo').write_bytes(b'picture')
        (data / 'database.sql.gz').write_bytes(gzip.compress(b'SELECT 1;'))
        self.manifest = {'created_at': 'isolated-test', 'vendors': 1, 'products': 1, 'table_counts': {'accounts': 1},
                         'objects': [{'key': 'photo', 'size': 7, 'sha256': imp.digest(data / 'media/photo')}]}
        self.info = {'database': 'old_business', 'media': str(self.root / 'old-media'), 'operations': str(self.ops), 'port': 18099}
        self.upgrade = MagicMock()
        self.upgrade.context.return_value = ({'DB_MAINTENANCE_USER': 'maintenance', 'DB_MAINTENANCE_PASSWORD': 'not-logged'}, self.info)
        self.upgrade.tool.return_value = 'mysql'
        self.sql = []
        def mysql(*args, **kwargs):
            text = args[2] if len(args) > 2 else kwargs.get('sql', '')
            self.sql.append(text)
            if text == 'SELECT MAX(version) FROM schema_migrations':
                return '15'
            if text == 'SELECT CURRENT_USER()':
                return 'application@localhost'
            return '1'
        self.mysql = mysql

    def run_flow(self, healthy=True, confirm=True):
        argv = ['import-local-data.py', '--apply', '--install-dir', str(self.root / 'install')]
        if confirm:
            argv.append('--confirm-use-local-data')
        with patch.object(imp, '__file__', str(self.root / 'import-local-data.py')), \
             patch.object(imp, 'load_upgrade', return_value=self.upgrade), \
             patch.object(imp, 'verify_snapshot', return_value=self.manifest), \
             patch.object(imp, 'mysql', side_effect=self.mysql), \
             patch.object(imp, 'table_counts', return_value={'accounts': 1}), \
             patch.object(imp, 'wait_healthy', return_value=healthy), \
             patch.object(imp.sys, 'argv', argv), patch.object(imp.sys, 'platform', 'linux'), \
             patch.object(imp.os, 'geteuid', return_value=0, create=True), \
             patch.object(imp.subprocess, 'run', return_value=subprocess.CompletedProcess([], 0)) as run, \
             patch.object(imp.subprocess, 'check_output', return_value=''):
            self.run = run
            imp.main()

    def test_confirmation_required_before_writes(self):
        self.assertRaisesRegex(ValueError, 'confirm', self.run_flow, confirm=False)
        self.assertFalse(any('CREATE DATABASE' in sql for sql in self.sql))
        self.assertEqual(json.loads(self.pointer.read_text()), self.old)

    def test_new_target_and_old_target_retained(self):
        self.run_flow()
        state = json.loads(self.pointer.read_text())
        self.assertEqual(state['previous']['database'], 'old_business')
        self.assertNotEqual(state['current']['database'], 'old_business')
        self.assertEqual(Path(state['current']['media_dir']).joinpath('photo').read_bytes(), b'picture')
        self.assertEqual(self.upgrade.backup.call_count, 1)
        self.assertFalse(any('DROP DATABASE' in sql for sql in self.sql))
        self.assertTrue(any("'system_config_v3','site_identity'" in sql for sql in self.sql))

    def test_temporary_password_matches_medium_mysql_policy(self):
        import re
        self.run_flow()
        create = next(sql for sql in self.sql if sql.startswith('CREATE USER'))
        password = re.search("IDENTIFIED BY '([^']+)'", create).group(1)
        self.assertGreaterEqual(len(password), 12)
        self.assertRegex(password, '[A-Z]')
        self.assertRegex(password, '[a-z]')
        self.assertRegex(password, '[0-9]')
        self.assertRegex(password, '[^A-Za-z0-9]')

    def test_health_failure_restores_original_pointer(self):
        self.assertRaisesRegex(RuntimeError, 'healthy', self.run_flow, healthy=False)
        self.assertEqual(json.loads(self.pointer.read_text()), self.old)
        self.assertEqual(self.run.call_args[0][0], ['systemctl', 'start', 'zqzt-exhibition.service'])

    def test_backup_failure_prevents_cutover(self):
        self.upgrade.backup.side_effect = RuntimeError('isolated backup failure')
        self.assertRaisesRegex(RuntimeError, 'backup failure', self.run_flow)
        self.assertEqual(json.loads(self.pointer.read_text()), self.old)
        self.assertEqual(self.run.call_args[0][0], ['systemctl', 'start', 'zqzt-exhibition.service'])


def real_restore(stage):
    cfg = {}
    for line in (ROOT / 'server/.env').read_text(encoding='utf-8-sig').splitlines():
        if '=' in line and not line.lstrip().startswith('#'):
            key, value = line.split('=', 1)
            cfg[key.strip()] = value.strip().strip('"')
    tool = str(Path(os.environ.get('MYSQL_BIN', 'D:/InforGuardMC/InforGuardMC/mysql/bin')) / 'mysql.exe')
    manifest = imp.verify_snapshot(stage / 'local-data')
    suffix = secrets.token_hex(6)
    database = 'zqzt_exhibition_test_snapshot_' + suffix
    user = 'zqs_' + suffix
    password = 'Aa1!' + secrets.token_hex(22)
    process = None
    db_created = user_created = False
    work = ROOT / '.local/private-smoke' / suffix
    work.mkdir(parents=True)
    executable = work / 'program/server/exhibition.exe'
    executable.parent.mkdir(parents=True)
    shutil.copyfile(ROOT / 'server/exhibition.exe', executable)
    query = lambda sql, db=None: imp.mysql(cfg, tool, sql, db)
    data = stage / 'local-data'
    source_database = cfg.get('DB_NAME', 'zqzt_exhibition')
    before = query("SELECT kind,COUNT(*),SUM(visible=1 AND published_revision_id>0) FROM contents GROUP BY kind", source_database)
    try:
        query('CREATE DATABASE `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci' % database)
        db_created = True
        query("CREATE USER '%s'@'%%' IDENTIFIED BY '%s'" % (user, password))
        user_created = True
        query("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'%%'" % (database, user))
        with gzip.open(data / 'database.sql.gz', 'rb') as source:
            imp.mysql(cfg, tool, database=database, user=user, password=password, source=source)
        if imp.table_counts(cfg, tool, database, manifest['table_counts']) != manifest['table_counts']:
            raise RuntimeError('Restored counts differ')
        mismatch = query('SELECT COUNT(*) FROM `%s`.accounts a LEFT JOIN `%s`.accounts b ON a.id=b.id WHERE b.id IS NULL OR a.password_hash<>b.password_hash' % (source_database, database))
        if mismatch != '0':
            raise RuntimeError('Password hashes differ')
        env = {**os.environ, 'APP_ENV': 'production', 'HTTP_ADDR': '127.0.0.1:18097',
               'DB_HOST': cfg.get('DB_HOST', '127.0.0.1'), 'DB_PORT': cfg.get('DB_PORT', '13306'),
               'DB_USER': user, 'DB_PASSWORD': password, 'DB_NAME': database,
               'ADMIN_PASSWORD': 'Aa1!' + secrets.token_hex(24), 'SMS_PROVIDER': 'disabled',
               'MEDIA_DIR': str(data / 'media'), 'WEB_DIR': str(ROOT / 'web/dist'),
               'OPERATIONS_DIR': str(work / 'operations'), 'CONFIG_ENCRYPTION_KEY': secrets.token_hex(32),
               'PUBLIC_BASE_URL': 'http://127.0.0.1:18097', 'CORS_ORIGIN': 'http://127.0.0.1:18097'}
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        def get(path):
            with opener.open('http://127.0.0.1:18097' + path, timeout=10) as response:
                return response.read(), response.headers.get_content_type()
        with (work / 'stdout.log').open('wb') as stdout, (work / 'stderr.log').open('wb') as stderr:
            process = subprocess.Popen([str(executable)], cwd=str(work), env=env, stdout=stdout, stderr=stderr,
                                       creationflags=getattr(subprocess, 'CREATE_NO_WINDOW', 0x08000000))
            for _ in range(60):
                if process.poll() is not None:
                    raise RuntimeError('Isolated runtime exited; inspect private smoke logs')
                try:
                    if json.loads(get('/api/v1/health')[0])['status'] == 'ok':
                        break
                except OSError:
                    time.sleep(.25)
            else:
                raise RuntimeError('Isolated runtime did not become healthy')
            home = json.loads(get('/api/v1/public/home')[0])
            vendors = json.loads(get('/api/v1/public/vendors?page=1&page_size=20')[0])
            products = json.loads(get('/api/v1/public/products?page=1&page_size=24')[0])
            if vendors['total'] != manifest['public_vendors'] or products['total'] != manifest['public_products']:
                raise RuntimeError('Public totals differ')
            if len(products['items']) > 24 or len(vendors['items']) > 20:
                raise RuntimeError('Page bounds violated')
            get('/api/v1/public/home?hall=cable')
            checks = []
            ids = query("SELECT DISTINCT m.id FROM media m JOIN media_references mr ON mr.media_id=m.id JOIN contents c ON c.published_revision_id=mr.revision_id WHERE c.visible=1 ORDER BY m.id LIMIT 6", database).splitlines()
            for mid in ids:
                for tail in ('', '?thumb=1'):
                    body, kind = get('/media/' + mid + tail)
                    if not body or not kind.startswith('image/'):
                        raise RuntimeError('Public image failed')
                    checks.append(mid + tail)
            for page in ('/', '/browse', '/browse/products', '/admin'):
                if get(page)[1] != 'text/html':
                    raise RuntimeError('Public page failed')
            result = {'isolated_mysql_restore': 'passed', 'table_counts': 'all tables matched before application startup',
                      'password_hashes': 'all matched; no hashes logged', 'sha256_media_files': len(manifest['objects']),
                      'public_vendors': vendors['total'], 'public_products': products['total'],
                      'public_media_responses': len(checks), 'server_pages': 'passed',
                      'runtime': 'Windows production configuration, scoped MySQL account',
                      'importerSHA256': imp.digest(ROOT / 'deploy/linux/import-local-data.py'),
                      'linux_systemd_import_and_upgrade': 'not executed on a real ECS',
                      'original_database_content_counts_unchanged': before == query("SELECT kind,COUNT(*),SUM(visible=1 AND published_revision_id>0) FROM contents GROUP BY kind", source_database)}
            if not result['original_database_content_counts_unchanged']:
                raise RuntimeError('Original DB changed')
            (stage / 'verification/local-data-validation.json').write_text(json.dumps(result, indent=2) + '\n', encoding='utf-8')
            print(json.dumps(result, ensure_ascii=False, indent=2))
    finally:
        if process:
            process.terminate()
            process.wait(timeout=15)
        if db_created:
            query('DROP DATABASE `%s`' % database)
        if user_created:
            query("DROP USER '%s'@'%%'" % user)


if __name__ == '__main__':
    if '--restore' in sys.argv:
        real_restore(ROOT / '.local/private-release/zqzg-aliyun-20261019-with-local-data-v1')
    else:
        unittest.main(verbosity=2)
