"""Linux-compatible checksum generation and archive readback regression tests."""
import hashlib
import importlib.util
from pathlib import Path
import tempfile
import unittest
import zipfile

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('package_schema21', ROOT / 'scripts/package-schema21-release.py')
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class PackageChecksumTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)

    def test_generated_manifest_is_lf_and_preserves_payload_bytes(self):
        contents = {'文件 with spaces.txt': b'original\r\ncontent\x00', 'nested/blob.bin': bytes(range(256))}
        for name, data in contents.items():
            path = self.root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(data)
        release.sums(self.root)
        raw = (self.root / 'SHA256SUMS').read_bytes()
        self.assertNotIn(b'\r', raw)
        self.assertFalse(raw.startswith(b'\xef\xbb\xbf'))
        for digest, name in release.checksum_records(raw):
            self.assertEqual(contents[name], (self.root / name).read_bytes())
            self.assertEqual(digest, hashlib.sha256(contents[name]).hexdigest())

    def test_linux_reader_rejects_crlf_bom_and_unsafe_records(self):
        record = b'a' * 64 + b'  file.txt\n'
        for invalid in [record.replace(b'\n', b'\r\n'), b'\xef\xbb\xbf' + record, record[:-1], record + record,
                        b'a' * 64 + b'  ../file\n', b'a' * 64 + b'  /file\n', b'a' * 64 + b'  dir\\file\n']:
            with self.subTest(invalid=invalid):
                self.assertRaises(ValueError, release.checksum_records, invalid)

    def test_zip_checks_manifest_bytes_and_each_payload_digest(self):
        folder = self.root / 'source'
        folder.mkdir()
        (folder / 'file').write_bytes(b'payload')
        release.sums(folder)
        target = self.root / 'source.zip'
        release.zip_folder(folder, target, 'source')
        with zipfile.ZipFile(target) as archive:
            self.assertNotIn(b'\r', archive.read('source/SHA256SUMS'))

    def test_legacy_package_recovery_changes_only_manifest_line_endings(self):
        payload = self.root / 'file.txt'
        payload.write_bytes(b'protected payload\r\n')
        digest = hashlib.sha256(payload.read_bytes()).hexdigest()
        original = (digest + '  file.txt\r\n').encode('utf-8')
        self.assertRaises(ValueError, release.checksum_records, original)
        repaired = original.replace(b'\r\n', b'\n')
        for expected, name in release.checksum_records(repaired):
            self.assertEqual(expected, hashlib.sha256((self.root / name).read_bytes()).hexdigest())
        self.assertEqual(payload.read_bytes(), b'protected payload\r\n')


if __name__ == '__main__':
    unittest.main(verbosity=2)
