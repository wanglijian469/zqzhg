"""Regression checks for preserving published APKs before a release switch."""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('preserve_downloads', ROOT / 'deploy/linux/preserve-downloads.py')
helper = importlib.util.module_from_spec(spec)
spec.loader.exec_module(helper)


class PreserveDownloadsTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.source = self.root / 'old/downloads'
        self.source.mkdir(parents=True)
        (self.source / 'dalunongji.apk').write_bytes(b'published APK\x00\r\n')
        (self.source / 'manifest.json').write_text(json.dumps({'version': 'existing'}), encoding='utf-8')
        (self.source / 'empty').mkdir()
        self.target = self.root / 'new/downloads'
        self.target.parent.mkdir()

    def test_check_is_read_only(self):
        before = helper.inventory(self.source)
        report = helper.preserve(self.source)
        self.assertEqual(report['files'], 2)
        self.assertFalse(report['copied'])
        self.assertFalse(self.target.exists())
        self.assertEqual(before, helper.inventory(self.source))

    def test_copy_keeps_every_published_byte_and_empty_directory(self):
        checked = helper.preserve(self.source)
        copied = helper.preserve(self.source, self.target)
        self.assertTrue(copied['copied'])
        self.assertEqual(checked['inventory_sha256'], copied['inventory_sha256'])
        self.assertEqual(helper.inventory(self.source), helper.inventory(self.target))
        self.assertTrue((self.target / 'empty').is_dir())

    def test_existing_target_is_never_overwritten(self):
        self.target.mkdir()
        (self.target / 'dalunongji.apk').write_bytes(b'do not replace')
        with self.assertRaisesRegex(ValueError, 'must not already exist'):
            helper.preserve(self.source, self.target)
        self.assertEqual((self.target / 'dalunongji.apk').read_bytes(), b'do not replace')

    def test_nested_target_and_missing_source_are_rejected(self):
        with self.assertRaisesRegex(ValueError, 'outside the source'):
            helper.preserve(self.source, self.source / 'nested')
        with self.assertRaisesRegex(ValueError, 'real directory'):
            helper.preserve(self.root / 'missing')

    def test_copy_detects_a_concurrent_change_before_release_switch(self):
        original = helper.shutil.copytree
        def changing_copy(source, target, *args, **kwargs):
            result = original(source, target, *args, **kwargs)
            if Path(source) == self.source:
                (self.source / 'dalunongji.apk').write_bytes(b'changed concurrently')
            return result
        with mock.patch.object(helper.shutil, 'copytree', side_effect=changing_copy):
            with self.assertRaisesRegex(ValueError, 'changed during copying'):
                helper.preserve(self.source, self.target)

    def test_source_symlink_is_rejected(self):
        with mock.patch.object(Path, 'is_symlink', return_value=True):
            with self.assertRaisesRegex(ValueError, 'real directory'):
                helper.preserve(self.source)


if __name__ == '__main__':
    unittest.main(verbosity=2)
