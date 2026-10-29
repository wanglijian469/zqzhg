"""Read-only HTTPS checks, without sending SMS or using customer sessions."""
import json,urllib.request,urllib.error
from pathlib import Path
opener=urllib.request.build_opener(urllib.request.ProxyHandler({}))
origin='https://zqzhg.com'
def get(path):
 try:r=opener.open(origin+path,timeout=30)
 except urllib.error.HTTPError as e:r=e
 return r.status,r.read()
status,body=get('/api/v1/public/config');assert status==200
cfg=json.loads(body)
assert cfg['company_verification_mode']=='content_review'
assert cfg['capabilities']['company_verification'] is False
assert cfg['capabilities']['procurement'] is False
assert cfg['capabilities']['phone_sms_auth'] is False
status,body=get('/api/v1/public/service-documents');assert status==200
docs=json.loads(body)['items'];assert len(docs)==4 and all(d['version']>=2 for d in docs)
assert all('企业提交真实营业执照' not in d['body'] for d in docs)
privacy=next(d for d in docs if d['key']=='privacy')
assert '注册及登录处理' in privacy['body'] and 'Registration and sign-in process' in privacy['body_en']
for path in ['/services','/service-requests','/merchant/verification','/admin/dashboard']:
 status,_=get(path);assert status==200,(path,status)
status,_=get('/api/v1/public/procurements');assert status==503
status,_=get('/api/v1/dev/map-preview/config');assert status==404
receipt={'origin':origin,'company_verification_mode':'content_review','document_versions':{d['key']:d['version'] for d in docs},'procurement_disabled':True,'sms_login_disabled':True,'agent_real_sms_sends':0,'passed':True}
Path('docs/test-output/service-1.10.4-cloud.json').write_text(json.dumps(receipt,ensure_ascii=False,indent=2),encoding='utf-8')
print(json.dumps(receipt,ensure_ascii=False))
