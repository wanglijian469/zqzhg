#!/usr/bin/env python3
"""Upgrade an existing zqzt Linux installation to the showroom SEO release."""
import argparse
import fcntl
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
import sys
from urllib.error import HTTPError
import xml.etree.ElementTree as ET


def load(name, filename):
    spec = importlib.util.spec_from_file_location(name, str(Path(__file__).resolve().parent / filename))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


# Keep the extracted package byte-for-byte verifiable after importing its helpers.
sys.dont_write_bytecode = True
ops = load('zqzt_upgrade', 'upgrade-video-1.10.9.py')
downloads = load('zqzt_downloads', 'preserve-downloads.py')
ops.VERSION = '1.10.10'
ops.STATE = ops.BASE / 'shared/upgrade-v1.10.10-private.json'
ORIGIN = 'https://zqzhg.com'
SAMPLE = '/v/v-21'
APK = 'dalunongji.apk'
NS = {'s': 'http://www.sitemaps.org/schemas/sitemap/0.9'}


def verify_package(package):
    package = package.resolve(strict=True)
    manifest = json.loads((package / 'manifest.json').read_text())
    assert manifest['version'] == ops.VERSION and manifest['schemaVersion'] == 30
    assert manifest['downloadPolicy'] == 'preserve-live-directory'
    assert re.fullmatch(r'[a-f0-9]{40}', manifest['gitCommit'])
    assert re.fullmatch(r'[a-f0-9]{64}', manifest['android']['sha256'])
    ops.verify_inventory(package, manifest)
    assert len(manifest['mapFiles']) == 77
    assert ops.sha(package / 'server/exhibition') == manifest['binarySHA256']
    with (package / 'server/exhibition').open('rb') as binary:
        header = binary.read(20)
    assert header[:6] == b'\x7fELF\x02\x01' and int.from_bytes(header[18:20], 'little') == 62
    assert not (package / 'web/dist/downloads').exists()
    return manifest


def published_downloads(folder, expected):
    metadata = json.loads((folder / 'manifest.json').read_text())
    assert all(metadata.get(key) == value for key, value in expected.items()), 'published_download_version_changed'
    assert ops.sha(folder / APK) == expected['sha256'], 'published_apk_hash_mismatch'


def prepare_web(release, manifest):
    previous = Path(ops.state()['previous'])
    web = Path(ops.config()['WEB_DIR']).resolve(strict=True)
    assert web == previous / 'web/dist', 'unexpected_active_web_directory'
    source = web / 'downloads'
    destination = release / 'web/dist/downloads'
    published_downloads(source, manifest['android'])
    preserved = downloads.preserve(source, destination)
    published_downloads(destination, manifest['android'])
    recorded = dict(manifest, downloadsInventorySHA256=preserved['inventory_sha256'])
    (release / 'manifest.json').write_text(json.dumps(recorded, indent=2) + '\n')


def seo_check(fetch):
    status, body = fetch(SAMPLE)
    assert status == 200
    page = body.decode('utf-8')
    for value in ('<title>河北丹顿液压科技有限公司 - 企业展厅 - 中企展馆</title>',
                  "name='description'", "href='" + ORIGIN + SAMPLE + "'", "content='index,follow'",
                  'data-seo-showroom', '液压油缸', '"@type":"Organization"'):
        assert value in page, 'showroom_html_missing_' + value
    assert 'BreadcrumbList' not in page and 'aria-label="面包屑导航"' not in page
    for path in ['/v/v-seo-release-missing', '/unknown-showroom-seo-release']:
        assert fetch(path)[0] == 404, 'soft_404'
    status, body = fetch('/regions?region_code=130528')
    assert status == 200 and ('href="' + SAMPLE + '"').encode() in body, 'regional_discovery_missing'
    status, body = fetch('/robots.txt')
    assert status == 200 and ('Sitemap: ' + ORIGIN + '/sitemap.xml').encode() in body
    status, body = fetch('/sitemap.xml')
    assert status == 200
    root = ET.fromstring(body)
    assert root.tag == '{' + NS['s'] + '}urlset', 'root_sitemap_must_be_flat_at_current_size'
    entries = root.findall('s:url', NS)
    entry = next(row for row in entries if row.findtext('s:loc', namespaces=NS) == ORIGIN + SAMPLE)
    assert entry.findtext('s:lastmod', namespaces=NS), 'showroom_lastmod_missing'
    status, body = fetch('/admin/login')
    assert status == 200 and b"content='noindex,follow'" in body
    return {'showroomStatus': 200, 'unknownStatus': 404, 'regionalDiscovery': True,
            'showroomBreadcrumb': False, 'sitemapURLs': len(entries), 'privateNoindex': True}


def candidate_check(port, release):
    def fetch(path):
        try:
            return ops.get(path, port)
        except HTTPError as error:
            return error.code, error.read()
    result = seo_check(fetch)
    published_downloads(release / 'web/dist/downloads', ops.state()['android'])
    return result


def production_fetch(path):
    raw = subprocess.check_output(['curl', '--noproxy', '*', '-sS', '--max-time', '15',
                                   '--resolve', 'zqzhg.com:443:127.0.0.1', '-w', '\n%{http_code}', ORIGIN + path])
    body, status = raw.rsplit(b'\n', 1)
    return int(status), body


def production_check(release):
    result = seo_check(production_fetch)
    expected = ops.state()
    for row in expected['candidateMap']:
        query = '?region_code=' + row['code'] if row['code'] else ''
        status, body = production_fetch('/api/v1/public/map-demo/regions' + query)
        assert status == 200 and json.loads(body)['sha256'] == row['sha256']
        status, stats = production_fetch('/api/v1/public/industry-map/regions' + query)
        assert status == 200
        status, vendors = production_fetch('/api/v1/public/industry-map/vendors' + query)
        assert status == 200 and json.loads(vendors)['total'] == json.loads(stats)['total']
    assert production_fetch('/api/v1/dev/map-preview/config')[0] == 404
    status, body = production_fetch('/downloads/manifest.json')
    assert status == 200 and all(json.loads(body).get(key) == value for key, value in expected['android'].items())
    published_downloads(release / 'web/dist/downloads', expected['android'])
    result.update(mapLevels=4, existingAPKVerified=True)
    return result


def apply():
    value = ops.state()
    release = Path(value['release'])
    manifest = json.loads((release / 'manifest.json').read_text())
    current = Path(ops.config()['WEB_DIR']) / 'downloads'
    staged = release / 'web/dist/downloads'
    for path in [current, staged]:
        assert downloads.preserve(path)['inventory_sha256'] == manifest['downloadsInventorySHA256']
    ops.apply(production_check, extra_environment=[])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['check', 'backup', 'stage', 'candidate', 'apply', 'rollback', 'status'])
    parser.add_argument('--package', type=Path, default=Path(__file__).resolve().parent)
    args = parser.parse_args()
    if args.action == 'check':
        print(json.dumps({'packageVerified': bool(verify_package(args.package))}))
        return
    assert os.geteuid() == 0, 'root_required'
    os.umask(0o077)
    ops.verify_package = verify_package
    with (ops.BASE / 'shared/upgrade-v1.10.9.lock').open('a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        if args.action == 'status':
            value = ops.state()
            keys = ['backupVerified', 'staged', 'isolatedRestoreVerified', 'candidateVerified',
                    'candidateSEO', 'candidateMap', 'promoted', 'productionSEO', 'rolledBack', 'gitCommit', 'schema']
            print(json.dumps({key: value.get(key) for key in keys}))
        else:
            actions = {'backup': ops.backup, 'stage': lambda: ops.stage(args.package, prepare_web),
                       'candidate': lambda: ops.candidate(candidate_check), 'apply': apply, 'rollback': ops.rollback}
            actions[args.action]()


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        print(json.dumps({'failed': type(error).__name__, 'reason': str(error)}))
        sys.exit(1)
