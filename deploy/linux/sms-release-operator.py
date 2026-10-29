"""Root ECS operator for the explicitly authorized SMS/demo upgrade.

All existing credentials and full backups stay on ECS. Commands never send SMS.
Candidate uses the existing active DB and media; take a fresh backup first.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import pwd
import re
import secrets
import shutil
import subprocess
import tarfile
import time
import urllib.request

RELEASE = Path('/opt/zqzt/releases/sms-demo-1.10.1-20261028-r3')
ENV = Path('/etc/zqzt/exhibition.env')
CANDIDATE_ENV = Path('/etc/zqzt/sms-candidate.env')

def run(args, **kw):
    result = subprocess.run(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE, **kw)
    if result.returncode:
        error = result.stderr.decode('utf-8', 'replace')
        match = re.search(r'public demo import failed: ([a-z_]+)', error)
        code = match.group(1) if match else 'command_failed:' + Path(args[0]).name
        Path('/opt/zqzt/shared/upgrade-backups/sms-last-operation-error.log').write_text(result.stdout.decode('utf-8', 'replace') + error)
        os.chmod('/opt/zqzt/shared/upgrade-backups/sms-last-operation-error.log', 0o600)
        raise RuntimeError(code)
    return result.stdout.decode('utf-8', 'replace').strip()

def read_env(path):
    return {k.strip(): v.strip().strip('"') for line in path.read_text(encoding='utf-8-sig').splitlines() if '=' in line and not line.lstrip().startswith('#') for k, v in [line.split('=', 1)]}

def write_env(path, values):
    temporary = path.with_suffix('.tmp')
    temporary.write_text(''.join(k + '=' + v + '\n' for k, v in values.items()), encoding='utf-8')
    os.chmod(temporary, 0o600)
    temporary.replace(path)

def health(port):
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    for _ in range(40):
        try:
            with opener.open('http://127.0.0.1:' + str(port) + '/api/v1/health', timeout=2) as r:
                value = json.load(r)
                if value.get('status') == 'ok' and value.get('sms_provider') == 'aliyun':
                    return value
        except Exception:
            pass
        time.sleep(1)
    raise RuntimeError('candidate_health_failed_inspect_private_journal')

def invoke(arguments):
    config = dict(os.environ, **read_env(CANDIDATE_ENV))
    user = pwd.getpwnam('zqzt')
    def demote():
        os.initgroups('zqzt', user.pw_gid)
        os.setgid(user.pw_gid)
        os.setuid(user.pw_uid)
    return run([str(RELEASE / 'server/exhibition')] + arguments, env=config, cwd=str(RELEASE), preexec_fn=demote, timeout=180)

def stage(archive, expected):
    if hashlib.sha256(archive.read_bytes()).hexdigest() != expected:
        raise RuntimeError('archive_hash_mismatch')
    if RELEASE.exists():
        raise RuntimeError('release_already_exists')
    if RELEASE.parent.resolve() != Path('/opt/zqzt/releases').resolve():
        raise RuntimeError('unsafe_release_directory')
    RELEASE.mkdir(mode=0o755)
    with tarfile.open(archive) as bundle:
        for item in bundle.getmembers():
            parts = Path(item.name).parts
            if not item.isfile() or item.name.startswith('/') or '..' in parts:
                raise RuntimeError('unsafe_archive_member')
        for item in bundle.getmembers():
            target = RELEASE / item.name
            target.parent.mkdir(parents=True, exist_ok=True)
            with bundle.extractfile(item) as source, target.open('wb') as destination:
                shutil.copyfileobj(source, destination)
    manifest = json.loads((RELEASE / 'release-manifest.json').read_text())
    for name, info in manifest['files'].items():
        file = RELEASE / name
        if file.stat().st_size != info['bytes'] or hashlib.sha256(file.read_bytes()).hexdigest() != info['sha256']:
            raise RuntimeError('release_file_mismatch')
    current = Path('/opt/zqzt/current').resolve(strict=True)
    preserve = Path('/tmp/zqzt-preserve-downloads.py')
    receipt = run(['python3.11', str(preserve), 'copy', '--source', str(current / 'web/dist/downloads'), '--destination', str(RELEASE / 'web/dist/downloads')])
    user = pwd.getpwnam('zqzt')
    for file in [RELEASE] + list(RELEASE.rglob('*')):
        if file.is_symlink():
            raise RuntimeError('release_link_forbidden')
        os.chown(file, user.pw_uid, user.pw_gid)
        os.chmod(file, 0o755 if file.is_dir() or file == RELEASE / 'server/exhibition' else 0o644)
    cfg = read_env(ENV)
    cfg.update(APP_ENV='production', HTTP_ADDR='127.0.0.1:8091', WEB_DIR=str(RELEASE / 'web/dist'),
               PUBLIC_BASE_URL='https://zqzhg.com', CORS_ORIGIN='https://zqzhg.com',
               MAP_DEMO_ENABLED='true', MAP_DEMO_DIR=str(RELEASE / 'map-demo'), MAP_PREVIEW_ENABLED='false',
               BACKGROUND_WORKERS_ENABLED='false', SMS_ENABLED='true', SMS_PROVIDER='aliyun',
               ALIYUN_SMS_SIGN_NAME='河北企元', ALIYUN_SMS_TEMPLATE_CODE='SMS_512636763',
               ALIYUN_SMS_ENDPOINT='dysmsapi.aliyuncs.com', ALIYUN_SMS_LOGIN_TEMPLATE_CODE='SMS_338956940',
               ALIYUN_SMS_LOGIN_CODE_VARIABLE='code', ALIBABA_CLOUD_IMDSV1_DISABLED='true',
               TRUSTED_PROXIES='127.0.0.1,::1', GOMEMLIMIT='128MiB', GOMAXPROCS='2')
    if len(cfg.get('SMS_HASH_KEY', '')) < 32:
        cfg['SMS_HASH_KEY'] = secrets.token_hex(32)
    for key in ('ALIYUN_ACCESS_KEY_ID', 'ALIYUN_ACCESS_KEY_SECRET', 'ALIBABA_CLOUD_ACCESS_KEY_ID', 'ALIBABA_CLOUD_ACCESS_KEY_SECRET', 'ALIBABA_CLOUD_SECURITY_TOKEN'):
        cfg.pop(key, None)
    write_env(CANDIDATE_ENV, cfg)
    unit = '[Unit]\nDescription=ZQZHG SMS candidate\nAfter=network.target mysqld.service\n[Service]\nUser=zqzt\nGroup=zqzt\nWorkingDirectory=' + str(RELEASE) + '\nEnvironmentFile=' + str(CANDIDATE_ENV) + '\nExecStart=' + str(RELEASE / 'server/exhibition') + '\nRestart=on-failure\nRestartSec=5\nMemoryMax=256M\nNoNewPrivileges=true\n[Install]\nWantedBy=multi-user.target\n'
    Path('/etc/systemd/system/zqzt-sms-candidate.service').write_text(unit)
    run(['systemctl', 'daemon-reload'])
    print(json.dumps({'release': str(RELEASE), 'downloads': json.loads(receipt), 'staged': True}))

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('action', choices=['stage', 'stage-binary', 'start', 'preflight', 'import-demo', 'set-origin', 'switch-main', 'trust-loopback-proxy', 'health'])
    parser.add_argument('--archive', type=Path)
    parser.add_argument('--sha256')
    args = parser.parse_args()
    if args.action == 'stage':
        stage(args.archive, args.sha256)
    elif args.action == 'stage-binary':
        if hashlib.sha256(args.archive.read_bytes()).hexdigest() != args.sha256 or RELEASE.exists():
            raise RuntimeError('binary_hash_or_release_conflict')
        previous = Path('/opt/zqzt/current').resolve(strict=True)
        if previous.parent != RELEASE.parent or RELEASE.parent.resolve() != Path('/opt/zqzt/releases').resolve():
            raise RuntimeError('unsafe_release_directory')
        shutil.copytree(previous, RELEASE, symlinks=False)
        shutil.copy2(args.archive, RELEASE / 'server/exhibition')
        manifest_path = RELEASE / 'release-manifest.json'
        manifest = json.loads(manifest_path.read_text())
        manifest['files']['server/exhibition'] = {'sha256':args.sha256,'bytes':args.archive.stat().st_size}
        manifest_path.write_text(json.dumps(manifest, indent=2))
        user = pwd.getpwnam('zqzt')
        for file in [RELEASE] + list(RELEASE.rglob('*')):
            if file.is_symlink():
                raise RuntimeError('release_link_forbidden')
            os.chown(file, user.pw_uid, user.pw_gid)
            os.chmod(file, 0o755 if file.is_dir() or file == RELEASE / 'server/exhibition' else 0o644)
        cfg = read_env(ENV)
        cfg.update(HTTP_ADDR='127.0.0.1:8091',BACKGROUND_WORKERS_ENABLED='false',WEB_DIR=str(RELEASE/'web/dist'),MAP_DEMO_DIR=str(RELEASE/'map-demo'))
        write_env(CANDIDATE_ENV,cfg)
        unit = Path('/etc/systemd/system/zqzt-sms-candidate.service')
        unit.write_text(unit.read_text().replace(str(previous),str(RELEASE)))
        run(['systemctl','daemon-reload'])
        print(json.dumps({'release':str(RELEASE),'binary_sha256':args.sha256,'staged':True}))
    elif args.action == 'start':
        run(['systemctl', 'start', 'zqzt-sms-candidate'])
        print(json.dumps(health(8091)))
    elif args.action == 'preflight':
        print(invoke(['sms-preflight']))
    elif args.action == 'import-demo':
        print(invoke(['import-public-demo', str(RELEASE / 'public-demo/catalog.json')]))
    elif args.action == 'set-origin':
        print(invoke(['set-public-origin', 'https://zqzhg.com']))
    elif args.action == 'trust-loopback-proxy':
        backup = Path('/opt/zqzt/shared/upgrade-backups/sms-before-proxy.env')
        if not backup.exists():
            shutil.copy2(ENV, backup)
            os.chmod(backup, 0o600)
        for path in (ENV, CANDIDATE_ENV):
            cfg = read_env(path)
            cfg['TRUSTED_PROXIES'] = '127.0.0.1,::1'
            write_env(path, cfg)
        run(['systemctl', 'start', 'zqzt-sms-candidate'])
        health(8091)
        nginx = Path('/etc/nginx/conf.d/72-zqzhg.conf')
        final = nginx.read_text()
        nginx.write_text(final.replace('127.0.0.1:8089', '127.0.0.1:8091'))
        run(['nginx', '-t'])
        run(['systemctl', 'reload', 'nginx'])
        time.sleep(2)
        run(['systemctl', 'restart', 'zqzt-exhibition'])
        health(8089)
        nginx.write_text(final)
        run(['nginx', '-t'])
        run(['systemctl', 'reload', 'nginx'])
        time.sleep(2)
        run(['systemctl', 'stop', 'zqzt-sms-candidate'])
        print(json.dumps({'trusted_proxies': 'loopback_only', 'health': health(8089)}))
    elif args.action == 'switch-main':
        health(8091)
        cfg = read_env(CANDIDATE_ENV)
        cfg.update(HTTP_ADDR='0.0.0.0:8089', BACKGROUND_WORKERS_ENABLED='true')
        write_env(ENV, cfg)
        current = Path('/opt/zqzt/current')
        if not current.is_symlink():
            raise RuntimeError('current_must_be_symlink')
        temporary = Path('/opt/zqzt/current.sms-next')
        if temporary.exists() or temporary.is_symlink():
            raise RuntimeError('temporary_switch_path_exists')
        temporary.symlink_to(RELEASE, target_is_directory=True)
        temporary.replace(current)
        run(['systemctl', 'restart', 'zqzt-exhibition'])
        print(json.dumps({'main': health(8089), 'candidate_kept_for_tls': True}))
    else:
        print(json.dumps(health(8091)))

if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        print(json.dumps({'error': str(error) if isinstance(error, RuntimeError) else type(error).__name__}))
        raise SystemExit(1)
