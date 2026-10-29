#!/usr/bin/env python3
"""Preserve the currently published downloads without shipping a replacement APK."""
import argparse
import hashlib
import json
import os
import shutil
import sys
from pathlib import Path


def digest(file):
    value = hashlib.sha256()
    with file.open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            value.update(chunk)
    return value.hexdigest()


def inventory(source):
    source = Path(source)
    if source.is_symlink() or not source.is_dir():
        raise ValueError('Existing downloads must be a real directory; check its configured location before upgrading')
    files = {}
    for folder, directories, names in os.walk(str(source), followlinks=False):
        for name in directories + names:
            file = Path(folder) / name
            if file.is_symlink():
                raise ValueError('Downloads contain a symbolic link; inspect it before upgrading')
            if name in directories:
                continue
            if not file.is_file():
                raise ValueError('Downloads contain a non-regular file')
            files[file.relative_to(source).as_posix()] = {'bytes': file.stat().st_size, 'sha256': digest(file)}
    return files


def preserve(source, destination=None):
    source = Path(source)
    before = inventory(source)
    if destination is not None:
        destination = Path(destination)
        if destination.exists() or destination.is_symlink():
            raise ValueError('Downloads destination must not already exist; refusing to replace files')
        if os.path.commonpath([str(source.resolve()), str(destination.resolve())]) == str(source.resolve()):
            raise ValueError('Downloads destination must be outside the source')
        if not destination.parent.is_dir():
            raise ValueError('Create the new release web directory first')
        shutil.copytree(str(source), str(destination))
        if inventory(destination) != before or inventory(source) != before:
            raise ValueError('Downloads changed during copying; do not switch the release')
    encoded = json.dumps(before, sort_keys=True, separators=(',', ':')).encode('utf-8')
    return {'files': len(before), 'bytes': sum(row['bytes'] for row in before.values()),
            'inventory_sha256': hashlib.sha256(encoded).hexdigest(), 'copied': destination is not None}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('mode', choices=['check', 'copy'])
    parser.add_argument('--source', required=True)
    parser.add_argument('--destination')
    args = parser.parse_args()
    if (args.mode == 'copy') != bool(args.destination):
        parser.error('copy requires --destination; check does not accept a destination')
    print(json.dumps(preserve(args.source, args.destination), sort_keys=True))


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError) as error:
        print('ERROR: ' + str(error), file=sys.stderr)
        sys.exit(1)
