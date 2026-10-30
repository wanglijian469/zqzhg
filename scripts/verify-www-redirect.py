"""Read-only public routing, trusted TLS and download regression checks."""
import json
from pathlib import Path
import socket
import ssl
import urllib.error
import urllib.parse
import urllib.request


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args):
        return None


opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())


def get(url, method='GET'):
    try:
        response = opener.open(urllib.request.Request(url, method=method), timeout=20)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        return response.status, dict(response.headers), response.read()


receipt = {'redirects': [], 'tls': [], 'real_sms_sent': 0}
origins = ['http://zqzhg.com', 'http://www.zqzhg.com', 'https://www.zqzhg.com']
paths = ['/', '/regions?region=130528&lang=en', '/app', '/downloads/dalunongji.apk']
for origin in origins:
    for path in paths:
        status, headers, _ = get(origin + path, 'HEAD')
        expected = 'https://zqzhg.com' + path
        assert status == 301 and headers.get('Location') == expected, (origin, path, status)
        receipt['redirects'].append({'url': origin + path, 'status': status, 'location': expected})
for host in ['zqzhg.com', 'www.zqzhg.com']:
    with socket.create_connection((host, 443), timeout=15) as connection:
        with ssl.create_default_context().wrap_socket(connection, server_hostname=host) as secure:
            certificate = secure.getpeercert()
    domains = [name for kind, name in certificate['subjectAltName'] if kind == 'DNS']
    assert {'zqzhg.com', 'www.zqzhg.com'} <= set(domains)
    receipt['tls'].append({'host': host, 'verified': True, 'domains': domains, 'not_after': certificate['notAfter']})
for path in ['/', '/login', '/app', '/regions']:
    status, _, _ = get('https://zqzhg.com' + path, 'HEAD')
    assert status == 200, (path, status)
status, _, body = get('https://zqzhg.com/api/v1/public/home?hall=agriculture')
assert status == 200
home = json.loads(body)
slug = next(v.get('showroom_slug') for v in home['vendors'] if v.get('showroom_slug'))
showroom = '/v/' + urllib.parse.quote(slug, safe='')
status, headers, _ = get('https://www.zqzhg.com' + showroom, 'HEAD')
assert status == 301 and headers.get('Location') == 'https://zqzhg.com' + showroom
assert get('https://zqzhg.com' + showroom, 'HEAD')[0] == 200
status, _, body = get('https://zqzhg.com/downloads/manifest.json')
assert status == 200
manifest = json.loads(body)
status, headers, _ = get('https://zqzhg.com/downloads/dalunongji.apk', 'HEAD')
assert status == 200 and int(headers['Content-Length']) == manifest['bytes']
status, _, body = get('https://zqzhg.com/api/v1/public/app')
assert status == 200 and json.loads(body)['qr_url'] == 'https://zqzhg.com/downloads/dalunongji.apk'
assert get('https://gosyncr.com/', 'HEAD')[0] == 200
status, headers, _ = get('https://www.gosyncr.com/', 'HEAD')
assert status == 301 and headers.get('Location') == 'https://gosyncr.com/'
receipt.update(showroom_checked=True, download_version=manifest['version'], download_bytes=manifest['bytes'],
               qr_url_verified=True, other_site_available=True, passed=True)
target = Path(__file__).resolve().parents[1] / 'docs/test-output/www-public-routing-20261030.json'
target.write_text(json.dumps(receipt, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
print(json.dumps(receipt, ensure_ascii=False))
