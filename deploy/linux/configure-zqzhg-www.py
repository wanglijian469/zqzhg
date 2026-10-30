"""Stage HTTP aliases before ACME, then enable WWW TLS after SAN validation.

Run on the authorized ECS. Backups and private keys stay on the server.
"""
import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import urllib.error
import urllib.request

CONF = Path('/etc/nginx/conf.d/72-zqzhg.conf')
BACKUPS = Path('/opt/zqzt/shared/nginx-backups')
MARKER = '# Enable this block only after the certificate covers both domain names.'


def run(command):
    result = subprocess.run(command, capture_output=True, text=True, timeout=30)
    if result.returncode:
        raise RuntimeError('command_failed:' + command[0])
    return result.stdout


def digest(file):
    return hashlib.sha256(file.read_bytes()).hexdigest()


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args):
        return None


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('phase', choices=['http', 'tls'])
    parser.add_argument('--template', required=True)
    parser.add_argument('--backup')
    args = parser.parse_args()
    desired = Path(args.template).read_text(encoding='utf-8-sig')
    assert desired.count(MARKER) == 1
    assert 'server_name zqzhg.com www.zqzhg.com;' in desired
    assert 'return 301 https://zqzhg.com$request_uri;' in desired
    if args.phase == 'http':
        desired = desired.split(MARKER)[0]
        stamp = datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ')
        backup = BACKUPS / ('www-' + stamp)
        backup.mkdir(parents=True, mode=0o700)
        os.chmod(backup, 0o700)
        for source in [CONF, Path('/etc/letsencrypt/renewal/zqzhg.com.conf'),
                       Path('/etc/letsencrypt/live/zqzhg.com/fullchain.pem'),
                       Path('/etc/letsencrypt/live/zqzhg.com/privkey.pem'),
                       Path('/etc/systemd/system/zqzhg-cert-renew.service'),
                       Path('/etc/systemd/system/zqzhg-cert-renew.timer')]:
            assert source.is_file()
            destination = backup / source.name
            shutil.copy2(source, destination)
            os.chmod(destination, 0o600)
    else:
        backup = Path(args.backup).resolve(strict=True)
        assert backup.parent == BACKUPS and (backup / CONF.name).is_file()
        certificate = '/etc/letsencrypt/live/zqzhg.com/fullchain.pem'
        for domain in ['zqzhg.com', 'www.zqzhg.com']:
            result = run(['openssl', 'x509', '-in', certificate, '-noout', '-checkhost', domain])
            assert 'does match certificate' in result
    other = {str(p): digest(p) for p in CONF.parent.glob('*.conf') if p != CONF}
    website = Path('/opt/zqzt/current').resolve()
    apk = website / 'web/dist/downloads/dalunongji.apk'
    apk_before = digest(apk)
    original = CONF.read_bytes()
    temporary = CONF.with_suffix('.conf.next')
    try:
        temporary.write_text(desired, encoding='utf-8')
        os.chmod(temporary, 0o644)
        os.replace(temporary, CONF)
        run(['nginx', '-t'])
        run(['systemctl', 'reload', 'nginx'])
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
        for host in ['zqzhg.com', 'www.zqzhg.com']:
            for attempt in range(20):
                request = urllib.request.Request('http://127.0.0.1/regions?region=130528', headers={'Host': host})
                try:
                    response = opener.open(request, timeout=6)
                except urllib.error.HTTPError as error:
                    response = error
                with response:
                    ready = response.status == 301 and response.headers.get('Location') == 'https://zqzhg.com/regions?region=130528'
                if ready:
                    break
                # Reload signals the master before new workers start accepting.
                time.sleep(0.25)
            assert ready, 'canonical_redirect_not_active:' + host
        assert other == {str(p): digest(Path(p)) for p in other}
        assert website == Path('/opt/zqzt/current').resolve()
        assert apk_before == digest(apk)
    except Exception:
        CONF.write_bytes(original)
        run(['nginx', '-t'])
        run(['systemctl', 'reload', 'nginx'])
        raise
    receipt = {'phase': args.phase, 'backup': str(backup), 'local_http_redirects': True,
               'other_sites_unchanged': True, 'apk_unchanged': True, 'nginx_reloaded': True}
    (backup / (args.phase + '-receipt.json')).write_text(json.dumps(receipt, indent=2) + '\n')
    print(json.dumps(receipt))


if __name__ == '__main__':
    main()
