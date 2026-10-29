"""Read the complete official HTTPS APK and verify it against controlled metadata."""
import hashlib
import json
import argparse
from pathlib import Path
import urllib.request

root = Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--tag', default='v1.10.4')
args = parser.parse_args()
if not args.tag.startswith('v') or any(c not in 'v0123456789.-' for c in args.tag):
    parser.error('Expected a release version tag')
manifest = json.loads((root / 'releases' / args.tag / 'manifest.json').read_text(encoding='utf-8'))
opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
with opener.open(manifest['apiBaseUrl'] + '/api/v1/public/app', timeout=30) as response:
    config = json.load(response)
assert config['base_url'] == manifest['apiBaseUrl']
assert config['android_url'] == '/downloads/dalunongji.apk'
assert config['qr_url'] == manifest['officialDownloadURL']
with opener.open(manifest['officialDownloadURL'], timeout=60) as response:
    assert response.status == 200
    header_bytes = int(response.headers['Content-Length'])
    h, received = hashlib.sha256(), 0
    while block := response.read(1024 * 1024):
        h.update(block)
        received += len(block)
assert received == header_bytes == manifest['bytes']
assert h.hexdigest() == manifest['sha256']
with opener.open(manifest['apiBaseUrl'] + '/downloads/manifest.json', timeout=30) as response:
    assert json.load(response) == manifest
receipt = {'url': manifest['officialDownloadURL'], 'version': manifest['version'], 'bytes': received, 'sha256': h.hexdigest(), 'qr_url': config['qr_url'], 'https_full_download_verified': True, 'agent_real_sms_sends': 0}
(root / f'docs/test-output/github-{args.tag}-official-download.json').write_text(json.dumps(receipt, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
print(json.dumps(receipt, ensure_ascii=False))
