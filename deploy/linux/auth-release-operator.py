"""1.10.2 final candidate upgrade. Two loopback candidates keep HTTPS serving."""
import argparse,importlib.util,json,re,sys,time
from pathlib import Path
spec=importlib.util.spec_from_file_location('sms_operator',Path(__file__).with_name('zqzt-sms-release-operator.py'));base=importlib.util.module_from_spec(spec);spec.loader.exec_module(base)
base.RELEASE=Path('/opt/zqzt/releases/auth-1.10.2-20261029-r2');base.CANDIDATE_ENV=Path('/etc/zqzt/auth102-candidate.env');unit=Path('/etc/systemd/system/zqzt-sms-candidate.service');newunit=Path('/etc/systemd/system/zqzt-auth102-candidate.service')
action=sys.argv[1]
if action=='stage':
 parser=argparse.ArgumentParser();parser.add_argument('action');parser.add_argument('--archive',type=Path,required=True);parser.add_argument('--sha256',required=True);args=parser.parse_args();previous=unit.read_text()
 try:
  base.stage(args.archive,args.sha256)
  newunit.write_text(unit.read_text())
 finally:unit.write_text(previous);base.run(['systemctl','daemon-reload'])
 cfg=base.read_env(base.CANDIDATE_ENV);cfg.update(HTTP_ADDR='127.0.0.1:8092',ALIYUN_SMS_RESET_TEMPLATE_CODE='SMS_338956940',ALIYUN_SMS_RESET_CODE_VARIABLE='code');cfg.pop('ALIYUN_SMS_LOGIN_TEMPLATE_CODE',None);cfg.pop('ALIYUN_SMS_LOGIN_CODE_VARIABLE',None);base.write_env(base.CANDIDATE_ENV,cfg)
elif action=='preflight':
 result=json.loads(base.invoke(['sms-preflight']));reset=next(i for i in result['templates'] if i['purpose']=='reset_password');metadata=' '.join(reset.get(k,'') for k in ('template_name','apply_scene','remark'))
 restricted=bool(re.search(r'(仅|只)[^。；]{0,15}登录|only.{0,20}(login|sign.in)',metadata,re.I))
 if restricted or not reset.get('matches_supplied_approved_template'):
  cfg=base.read_env(base.CANDIDATE_ENV);cfg['ALIYUN_SMS_RESET_TEMPLATE_CODE']='';base.write_env(base.CANDIDATE_ENV,cfg);result['reset_sms_enabled']=False
 else:result['reset_sms_enabled']=True
 (base.RELEASE/'template-preflight.json').write_text(json.dumps(result,ensure_ascii=False,indent=2));print(json.dumps(result,ensure_ascii=False))
elif action=='start':
 base.run(['systemctl','start','zqzt-auth102-candidate']);print(json.dumps(base.health(8092)))
elif action=='switch-main':
 base.health(8092);cfg=base.read_env(base.CANDIDATE_ENV);cfg.update(HTTP_ADDR='0.0.0.0:8089',BACKGROUND_WORKERS_ENABLED='true');base.write_env(base.ENV,cfg)
 current=Path('/opt/zqzt/current');temporary=Path('/opt/zqzt/current.auth-next')
 if not current.is_symlink() or temporary.exists() or temporary.is_symlink():raise RuntimeError('unsafe_switch_paths')
 temporary.symlink_to(base.RELEASE,target_is_directory=True);temporary.replace(current);base.run(['systemctl','restart','zqzt-exhibition']);print(json.dumps({'main':base.health(8089),'candidate_kept_for_https':True}))
else:raise RuntimeError('unknown_action')
