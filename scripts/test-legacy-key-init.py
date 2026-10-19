import base64
import importlib.util
import os
from pathlib import Path
import runpy
import subprocess
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location('keyinit', ROOT / 'deploy/linux/initialize-legacy-key.py')
keyinit = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(keyinit)
HELPER = ROOT / 'deploy/linux/aliyun-upgrade-support.py'


class KeyTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.ops = self.root / 'operations'
        self.ops.mkdir()
        self.media = self.root / 'media'
        self.media.mkdir()
        self.cfg = self.root / 'exhibition.env'
        self.initial = 'DB_NAME=existing_db\nDB_PASSWORD=do-not-log\nMEDIA_DIR=%s\nOPERATIONS_DIR=%s\nPUBLIC_BASE_URL=http://example.test\nCONFIG_ENCRYPTION_KEY=\n' % (self.media, self.ops)
        self.cfg.write_text(self.initial, encoding='utf-8')
        self.helper = runpy.run_path(str(HELPER))
        self.helper['tool'] = lambda cfg, name: 'mysql'

    def call(self, output='8\t0\n', code=0):
        with patch.object(keyinit.runpy, 'run_path', return_value=self.helper), \
             patch.object(keyinit.subprocess, 'run', return_value=subprocess.CompletedProcess([], code, output, '')) as run, \
             patch.object(keyinit.os, 'chown', create=True):
            result = keyinit.initialize(str(self.cfg), str(HELPER))
            self.assertNotIn('do-not-log', ' '.join(run.call_args[0][0]))
            return result

    def test_verified_legacy_schema_can_initialize_once(self):
        backup = self.call()
        cfg = self.helper['read_env'](self.cfg)
        self.assertEqual(len(bytes.fromhex(cfg['CONFIG_ENCRYPTION_KEY'])), 32)
        self.assertEqual((backup / 'exhibition.env.before').read_text(), self.initial)
        self.assertEqual((backup / 'CONFIG_ENCRYPTION_KEY').read_text().strip(), cfg['CONFIG_ENCRYPTION_KEY'])
        self.assertEqual(cfg['DB_PASSWORD'], 'do-not-log')
        self.assertEqual(cfg['PUBLIC_BASE_URL'], 'http://example.test')
        saved = self.cfg.read_bytes()
        with patch.object(keyinit.runpy, 'run_path', return_value=self.helper):
            self.assertIsNone(keyinit.initialize(str(self.cfg), str(HELPER)))
        self.assertEqual(self.cfg.read_bytes(), saved)

    def test_invalid_existing_key_never_replaced(self):
        value = self.initial.replace('CONFIG_ENCRYPTION_KEY=', 'CONFIG_ENCRYPTION_KEY=invalid')
        self.cfg.write_text(value)
        self.assertRaisesRegex(ValueError, 'existing key', self.call)
        self.assertEqual(self.cfg.read_text(), value)

    def test_newer_schema_rejected(self):
        self.assertRaisesRegex(ValueError, 'could not be verified', self.call, '10\t0\n')
        self.assertEqual(self.cfg.read_text(), self.initial)

    def test_encryption_tables_rejected_even_with_old_version(self):
        self.assertRaisesRegex(ValueError, 'could not be verified', self.call, '8\t2\n')
        self.assertEqual(self.cfg.read_text(), self.initial)

    def test_db_connection_failure_never_generates_key(self):
        self.assertRaises(ValueError, self.call, '', 1)
        self.assertEqual(self.cfg.read_text(), self.initial)
        self.assertFalse(list(self.root.glob('key-init-backup-*')))

    def test_existing_operations_key_or_backup_prevents_new_key(self):
        (self.ops / 'development.key').write_bytes(b'a' * 32)
        self.assertRaisesRegex(ValueError, 'Existing operations', self.call)
        self.assertEqual(self.cfg.read_text(), self.initial)


if __name__ == '__main__':
    unittest.main(verbosity=2)
