"""Inspect real local brochure tasks; never publish or expose credentials."""
import argparse
import json
from collections import Counter
from pathlib import Path
from urllib.request import ProxyHandler, Request, build_opener
from urllib.parse import urlparse

root = Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser()
parser.add_argument('--base', default='http://127.0.0.1:8094')
parser.add_argument('--require-review', action='store_true')
args = parser.parse_args()
if urlparse(args.base).hostname not in ('127.0.0.1', 'localhost', '::1'):
    raise SystemExit('This verifier sends private test sessions only to loopback.')
opener = build_opener(ProxyHandler({}))
reports = []
for source in sorted((root / '.local/ai-import-samples').glob('*.json')):
    credentials = json.loads(source.read_text(encoding='utf-8'))
    task_id = credentials['response']['task']['id']
    request = Request(args.base + '/api/v1/merchant/ai-import/tasks/' + str(task_id) + '/result',
                      headers={'Authorization': 'Bearer ' + credentials['token']})
    with opener.open(request, timeout=20) as response:
        data = json.load(response)
    company = data.get('company')
    products = []
    for draft in data['products']:
        details = draft['data'].get('product_details') or {}
        products.append({'name': draft['data']['name'], 'models': [m['model_name'] for m in (details.get('models') or [])],
                         'parameters': sum(len(m.get('parameters') or []) for m in (details.get('models') or [])),
                         'source_pages': draft['source_pages'], 'review_issues': draft['review_issues'],
                         'image_types': dict(Counter(im['image_type'] for im in (details.get('images') or []))),
                         'business_content_id': draft['content_id']})
    calls = [c for c in data['calls'] if c['error'] != 'bailian_attempts_recorded']
    known = [c['usage'] for c in calls if c.get('usage')]
    report = {'sample': source.stem, 'task_id': task_id, 'real_model_inference': True,
              'task': data['task'], 'company': company['data']['name'] if company else None,
              'products': products, 'asset_types': dict(Counter(a['image_type'] for a in data['assets'])),
              'calls_with_usage': len(known), 'calls_with_unknown_usage': sum(c.get('usage') is None for c in calls),
              'known_total_tokens': sum(u.get('total_tokens', 0) for u in known),
              'pending_manufacturer_review': True, 'production_deployed': False}
    reports.append(report)
output = root / '.local/ai-import-inspection/real-samples.json'
output.write_text(json.dumps(reports, ensure_ascii=False, indent=2), encoding='utf-8')
print(json.dumps([{'sample': r['sample'], 'task': r['task_id'], 'status': r['task']['status'],
                   'stage': r['task']['stage'], 'progress': r['task']['progress'], 'error': r['task']['error'],
                   'products': len(r['products']), 'known_total_tokens': r['known_total_tokens']} for r in reports],
                 ensure_ascii=True))
if args.require_review and (len(reports) != 2 or any(r['task']['status'] != 'review_pending' for r in reports)):
    raise SystemExit('Real brochure acceptance has not reached manufacturer review for both tasks.')
