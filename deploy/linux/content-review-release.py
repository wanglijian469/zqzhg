"""Stage/switch the authorized 1.10.4 content-review release; never send SMS."""
import argparse,importlib.util,json,sys,os,shutil
from pathlib import Path
spec=importlib.util.spec_from_file_location('release_base',Path('/tmp/zqzt-sms-release-operator.py'))
base=importlib.util.module_from_spec(spec);spec.loader.exec_module(base)
base.RELEASE=Path('/opt/zqzt/releases/content-review-1.10.4-20261029-r2')
base.CANDIDATE_ENV=Path('/etc/zqzt/service104-final.env')
unit=Path('/etc/systemd/system/zqzt-sms-candidate.service')
candidate_unit=Path('/etc/systemd/system/zqzt-service104-final.service')
proxy=Path('/etc/nginx/conf.d/72-zqzhg.conf')
action=sys.argv[1]
if action=='backup-config':
 target=Path(sys.argv[2]).resolve(strict=True)
 if target.parent != Path('/opt/zqzt/shared/upgrade-backups'):raise RuntimeError('unsafe_backup_path')
 for source,name in [(proxy,'72-zqzhg.conf'),(Path('/etc/systemd/system/zqzt-exhibition.service'),'zqzt-exhibition.service'),(Path('/opt/zqzt/shared/.operations-media/active.json'),'active.json')]:
  shutil.copy2(source,target/name);os.chmod(target/name,0o600)
 print(json.dumps({'configuration_backup':str(target)}))
elif action=='stage':
 p=argparse.ArgumentParser();p.add_argument('action');p.add_argument('--archive',type=Path,required=True);p.add_argument('--sha256',required=True);a=p.parse_args()
 previous=unit.read_text()
 try:
  base.stage(a.archive,a.sha256)
  candidate_unit.write_text(unit.read_text())
 finally:unit.write_text(previous);base.run(['systemctl','daemon-reload'])
 cfg=base.read_env(base.CANDIDATE_ENV);cfg.update(HTTP_ADDR='127.0.0.1:8093')
 cfg.pop('ALIYUN_SMS_LOGIN_TEMPLATE_CODE',None);cfg.pop('ALIYUN_SMS_LOGIN_CODE_VARIABLE',None)
 base.write_env(base.CANDIDATE_ENV,cfg)
elif action=='start':
 base.run(['systemctl','start','zqzt-service104-final']);print(json.dumps(base.health(8093)))
elif action in ('proxy-candidate','proxy-main'):
 old=proxy.read_text();source,target=('8092','8093') if action=='proxy-candidate' else ('8093','8089')
 before='proxy_pass http://127.0.0.1:'+source+';';after='proxy_pass http://127.0.0.1:'+target+';'
 if old.count(before)!=1:raise RuntimeError('unexpected_proxy_configuration')
 proxy.write_text(old.replace(before,after))
 try:base.run(['nginx','-t'])
 except Exception:proxy.write_text(old);raise
 base.run(['systemctl','reload','nginx']);print(json.dumps({'https_upstream':target}))
elif action=='switch-main':
 base.health(8093)
 cfg=base.read_env(base.CANDIDATE_ENV);cfg.update(HTTP_ADDR='0.0.0.0:8089',BACKGROUND_WORKERS_ENABLED='true')
 base.write_env(base.ENV,cfg)
 current=Path('/opt/zqzt/current');temporary=Path('/opt/zqzt/current.service104-next')
 if not current.is_symlink() or temporary.exists() or temporary.is_symlink():raise RuntimeError('unsafe_switch_paths')
 temporary.symlink_to(base.RELEASE,target_is_directory=True);temporary.replace(current)
 base.run(['systemctl','restart','zqzt-exhibition']);print(json.dumps({'main':base.health(8089),'candidate_kept_for_https':True}))
elif action=='stop-candidate':
 base.health(8089);base.run(['systemctl','stop','zqzt-service104-final']);print(json.dumps({'candidate_stopped':True}))
else:raise RuntimeError('unknown_action')

