#!/usr/bin/env python3
"""Atomically promote a previously verified APK; keep old downloads on this host."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil


def sha256(file):
    h = hashlib.sha256()
    with file.open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            h.update(block)
    return h.hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--apk', required=True)
    parser.add_argument('--manifest', required=True)
    parser.add_argument('--backup', required=True)
    args = parser.parse_args()
    source = Path(args.apk).resolve(strict=True)
    manifest = json.loads(Path(args.manifest).read_text(encoding='utf-8-sig'))
    assert manifest['applicationId'] == 'com.zqzt.dalunongji'
    assert manifest['apiBaseUrl'] == 'https://zqzhg.com'
    assert manifest['certificateSHA256'] == '85656c85541b188c832dc48d67cbf320046a4311aa9e0fefcc662dbe2e0f1585'
    assert source.stat().st_size == manifest['bytes'] and sha256(source) == manifest['sha256']
    release = Path('/opt/zqzt/current').resolve(strict=True)
    assert release.parent == Path('/opt/zqzt/releases')
    downloads = release / 'web/dist/downloads'
    assert downloads.is_dir() and not downloads.is_symlink()
    backup = Path(args.backup).resolve(strict=True)
    assert backup.parent == Path('/opt/zqzt/shared/upgrade-backups')
    assert (backup / 'database.sql').is_file()
    previous = backup / 'official-downloads'
    assert not previous.exists()
    shutil.copytree(downloads, previous)
    old_files = {str(p.relative_to(previous)): sha256(p) for p in previous.rglob('*') if p.is_file()}
    for p in previous.rglob('*'):
        if p.is_file():
            assert sha256(downloads / p.relative_to(previous)) == sha256(p)
    for name in ['/etc/nginx/conf.d/72-zqzhg.conf', '/etc/systemd/system/zqzt-exhibition.service', '/opt/zqzt/shared/.operations-media/active.json']:
        file = Path(name)
        if file.is_file():
            target = backup / file.name
            shutil.copy2(file, target)
            os.chmod(target, 0o600)
    target = downloads / 'dalunongji.apk'
    staged = downloads / '.dalunongji.apk.next'
    shutil.copyfile(source, staged)
    os.chmod(staged, 0o644)
    assert sha256(staged) == manifest['sha256']
    os.replace(staged, target)
    next_manifest = downloads / '.manifest.json.next'
    next_manifest.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    os.chmod(next_manifest, 0o644)
    os.replace(next_manifest, downloads / 'manifest.json')
    preview = downloads / 'previews/manifest.json'
    if preview.is_file():
        preview.rename(backup / 'previous-preview-manifest.json')
    assert Path('/opt/zqzt/current').resolve() == release
    assert sha256(target) == manifest['sha256']
    receipt = {'version': manifest['version'], 'release': str(release), 'backup': str(backup), 'previous_files': old_files, 'sha256': sha256(target), 'bytes': target.stat().st_size, 'website_restarted': False}
    (backup / 'android-promotion.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print(json.dumps(receipt))


if __name__ == '__main__':
    main()
