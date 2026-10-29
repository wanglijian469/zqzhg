"""Non-sending production acceptance; rejects invalid credentials and old SMS login."""
import json,urllib.request,urllib.error
from pathlib import Path
opener=urllib.request.build_opener(urllib.request.ProxyHandler({}))
def call(path,data=None):
 req=urllib.request.Request('https://zqzhg.com/api/v1'+path,data=None if data is None else json.dumps(data).encode(),headers={'Content-Type':'application/json'})
 try:r=opener.open(req,timeout=15)
 except urllib.error.HTTPError as e:r=e
 return r.status,json.load(r)
status,cfg=call('/public/config');assert status==200
assert cfg['capabilities']['phone_sms_auth'] is False and cfg['capabilities']['phone_password_reset'] is True
assert cfg['password_policy']['min_length']==8 and cfg['password_policy']['max_length']==20
receipt={'password_policy':True,'sms_login_disabled':True,'password_recovery_enabled':True,'agent_real_sms_sends':0}
for path,data in [('/auth/phone/login',{'phone':'13900008851','code':'000000'}),('/auth/phone',{'phone':'13900008851','code':'000000'}),('/auth/sms',{'phone':'13900008851'}),('/auth/phone/code',{'phone':'13900008851','purpose':'login'})]:
 status,response=call(path,data);assert status==410 and response['error']['code']=='sms_login_disabled',(path,status,response)
status,response=call('/auth/password',{'username':'auth-check-no-account-20261029','password':'bad-pass'});assert status==401 and response['error']['code']=='invalid_credentials';receipt['unknown_login_rejected']=True
for password in ['1234567','a'*21,'abcd1234 ','中文abcd1234']:
 status,response=call('/auth/phone/register',{'phone':'13900008851','password':password});assert status==400 and response['error']['fields'][0]['code']=='password_length'
receipt['invalid_new_passwords_rejected_before_registration']=True
for path in ['/public/procurements','/procurement/projects','/admin/procurement/reviews']:
 status,_=call(path);assert status==503
receipt['procurement_remains_disabled']=True
status,_=call('/dev/map-preview/config');assert status==404;receipt['dev_map_route_closed']=True
Path('docs/test-output/auth-1.10.2-api.json').write_text(json.dumps(receipt,indent=2));print(json.dumps(receipt))
