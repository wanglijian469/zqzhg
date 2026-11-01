"""Package integrity checks without connecting to a server or database."""
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

root = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('guide_upgrade', root / 'deploy/linux/upgrade-guide-1.10.6.py')
upgrade = importlib.util.module_from_spec(spec)
# fcntl is Linux-only; integrity functions do not use it on Windows.
import sys
if sys.platform == 'win32':
    from types import ModuleType
    sys.modules['fcntl'] = ModuleType('fcntl')
spec.loader.exec_module(upgrade)


class PackageChecks(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.package = Path(self.temp.name) / 'package'
        self.package.mkdir()
        self.write('web/dist/downloads/dalunongji.apk', b'fixture-only')
        self.write('map-demo/fixture.json', b'{}')
        android = {'versionCode': 44, 'sha256': upgrade.sha(self.package / 'web/dist/downloads/dalunongji.apk')}
        self.write('web/dist/downloads/manifest.json', json.dumps(android).encode())
        for i in range(1, 29):
            self.write('server/migrations/%03d_fixture.sql' % i, b'-- fixture')
        manifest = {'version': '1.10.6', 'schemaVersion': 28, 'gitCommit': 'a' * 40, 'android': android, 'mapFiles': upgrade.inventory(self.package / 'map-demo')}
        self.write('manifest.json', json.dumps(manifest).encode())
        self.checksums()

    def write(self, name, data):
        p = self.package / name
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_bytes(data)

    def checksums(self):
        files = upgrade.inventory(self.package)
        self.write('SHA256SUMS', ''.join(d + '  ' + n + '\n' for n, d in files.items() if n != 'SHA256SUMS').encode())

    def test_complete_package(self):
        self.assertEqual(upgrade.verify_package(self.package)['version'], '1.10.6')

    def test_modified_apk_is_rejected(self):
        self.write('web/dist/downloads/dalunongji.apk', b'changed')
        with self.assertRaises(AssertionError): upgrade.verify_package(self.package)

    def test_unlisted_file_is_rejected(self):
        self.write('unlisted.env', b'fixture')
        with self.assertRaises(AssertionError): upgrade.verify_package(self.package)

    def test_parent_path_is_rejected(self):
        other = self.package.parent / 'outside.txt'; other.write_bytes(b'fixture')
        with (self.package / 'SHA256SUMS').open('ab') as out:
            out.write((upgrade.sha(other) + '  ../outside.txt\n').encode())
        with self.assertRaises(AssertionError): upgrade.verify_package(self.package)

    def test_missing_migration_is_rejected_even_with_updated_hashes(self):
        (self.package / 'server/migrations/028_fixture.sql').unlink()
        self.checksums()
        with self.assertRaises(AssertionError): upgrade.verify_package(self.package)


if __name__ == '__main__': unittest.main()
