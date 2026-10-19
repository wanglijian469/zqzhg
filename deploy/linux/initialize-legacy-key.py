#!/usr/bin/env python3
"""Initialize a missing key only for a positively verified pre-operations install.

Run from the extracted bundle as root. Does not change application data or print keys.
"""
import os
import base64
import pathlib
import re
import runpy
import secrets
import shutil
import stat
import subprocess
import time


def initialize(env_file='/etc/zqzt/exhibition.env', helper_file='aliyun-upgrade-support.py'):
    helper = runpy.run_path(helper_file)
    path = pathlib.Path(env_file)
    before = path.read_bytes()
    cfg = helper['read_env'](path)
    if cfg.get('CONFIG_ENCRYPTION_KEY'):
        value = cfg['CONFIG_ENCRYPTION_KEY']
        try:
            decoded = bytes.fromhex(value) if re.fullmatch('[0-9A-Fa-f]{64}', value) else base64.b64decode(value, validate=True)
        except ValueError:
            decoded = b''
        if len(decoded) != 32:
            raise ValueError('An existing key is configured but invalid. Recover its original valid value; it will not be replaced.')
        print('Existing valid key preserved. No key or environment changes.')
        return None
    media = pathlib.Path(cfg.get('MEDIA_DIR', ''))
    operations = pathlib.Path(cfg.get('OPERATIONS_DIR') or str(media.parent / ('.operations-' + media.name)))
    if not media.is_absolute() or not media.is_dir() or not operations.is_absolute() or not operations.is_dir():
        raise ValueError('Validate the existing absolute media and operations directories first')
    if any(p.is_file() or p.is_symlink() for p in operations.rglob('*')):
        raise ValueError('Existing operations files found. Recover the original key instead of generating a replacement.')
    database = cfg.get('DB_NAME', 'zqzt_exhibition')
    if not re.fullmatch(r'[A-Za-z0-9_]{1,64}', database):
        raise ValueError('Invalid database name')
    query = "SELECT COALESCE(MAX(version),0),(SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name IN ('storage_profiles','system_settings')) FROM schema_migrations"
    result = subprocess.run([helper['tool'](cfg, 'mysql'), *helper['db_args'](cfg),
                             '--batch', '--skip-column-names', database, '--execute=' + query],
                            env=dict(os.environ, MYSQL_PWD=cfg.get('DB_PASSWORD', '')),
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE, universal_newlines=True)
    match = re.fullmatch(r'([0-9]+)\s+0', result.stdout.strip())
    if result.returncode or not match or not 1 <= int(match.group(1)) <= 9:
        raise ValueError('A pre-operations schema (1-9, without encryption tables) could not be verified. No key was changed.')
    metadata = path.stat()
    backup = path.parent / ('key-init-backup-' + time.strftime('%Y%m%d-%H%M%S') + '-' + secrets.token_hex(3))
    backup.mkdir(mode=0o700)
    shutil.copyfile(path, backup / 'exhibition.env.before')
    os.chmod(backup / 'exhibition.env.before', 0o600)
    key = secrets.token_hex(32)
    keyfile = backup / 'CONFIG_ENCRYPTION_KEY'
    with keyfile.open('x', encoding='utf-8') as stream:
        os.chmod(keyfile, 0o600)
        stream.write(key + '\n')
        stream.flush()
        os.fsync(stream.fileno())
    lines = [line for line in before.decode('utf-8-sig').splitlines()
             if line.split('=', 1)[0].strip() != 'CONFIG_ENCRYPTION_KEY']
    temp = path.with_name(path.name + '.key-init-' + secrets.token_hex(4))
    try:
        with temp.open('x', encoding='utf-8') as stream:
            os.chmod(temp, 0o600)
            stream.write('\n'.join(lines) + '\nCONFIG_ENCRYPTION_KEY=' + key + '\n')
            stream.flush()
            os.fsync(stream.fileno())
        os.chown(temp, metadata.st_uid, metadata.st_gid)
        os.chmod(temp, stat.S_IMODE(metadata.st_mode))
        if path.read_bytes() != before:
            raise ValueError('Environment changed concurrently. No replacement performed.')
        os.replace(temp, path)
    finally:
        if temp.exists():
            temp.unlink()
    print('Legacy schema verified. New key saved without displaying it. Private backup: ' + str(backup))
    return backup


if __name__ == '__main__':
    try:
        initialize()
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        raise SystemExit('Initialization stopped: ' + str(error))
