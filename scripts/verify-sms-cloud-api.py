"""Non-sending public production smoke checks. No credentials or users created."""
import json
import argparse
from pathlib import Path
import urllib.request
import urllib.error

opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
def call(path, payload=None, base='https://zqzhg.com', headers=None):
    data = None if payload is None else json.dumps(payload).encode()
    req = urllib.request.Request(base + '/api/v1' + path, data=data, headers={'Content-Type':'application/json', **(headers or {})})
    try:
        response = opener.open(req, timeout=15)
    except urllib.error.HTTPError as error:
        response = error
    return response.status, json.load(response)

receipt = {}
parser = argparse.ArgumentParser()
parser.add_argument('--check-legacy-http', action='store_true')
args = parser.parse_args()
status, cfg = call('/public/config')
assert status == 200 and cfg['service_mode'] == 'free_display'
assert cfg['site']['api_url'] == 'https://zqzhg.com/api/v1'
assert cfg['uploads']['default_quota_bytes'] == 200000000
assert cfg['capabilities']['procurement'] is False and cfg['capabilities']['development_sms'] is False
receipt['https_configuration'] = True
for path in ('/public/procurements', '/procurement/projects', '/admin/procurement/reviews'):
    status, _ = call(path)
    assert status == 503
receipt['procurement_disabled'] = True
status, response = call('/auth/phone/login', {'phone':'13900008888','code':'999999'})
assert status == 401 and response['error']['code'] == 'phone_login_unavailable'
receipt['unregistered_login_does_not_create_user'] = True
status, response = call('/auth/phone/code', {'phone':'13900008888','purpose':'register','request_id':'readonly-cloud-smoke-no-send-20261029'})
assert status == 400 and response['error']['code'] == 'sms_challenge_invalid'
receipt['missing_challenge_rejected_before_send'] = True
if args.check_legacy_http:
    status, response = call('/auth/phone/challenge', {'phone':'13900008888','purpose':'register'}, base='http://39.97.250.244:8089', headers={'X-Forwarded-Proto':'https'})
    assert status == 426 and response['error']['code'] == 'https_required'
    receipt['external_spoofed_https_header_rejected'] = True
else:
    receipt['legacy_http_probe'] = 'separate_on_host_public_ip_probe_426; Windows direct legacy port intermittently reset'
status, response = call('/dev/map-preview/config')
assert status == 404
receipt['development_route_unavailable_in_production'] = True
Path('docs/test-output/sms-1.10.1-cloud-api.json').write_text(json.dumps(receipt, indent=2), encoding='utf-8')
print(json.dumps(receipt))
