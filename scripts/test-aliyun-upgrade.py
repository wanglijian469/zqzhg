import hashlib, importlib.util, json, os, tempfile, unittest
from pathlib import Path
from unittest.mock import patch
import subprocess
ROOT=Path(__file__).resolve().parents[1]
spec=importlib.util.spec_from_file_location('upgrade',ROOT/'deploy/linux/aliyun-upgrade-support.py');upgrade=importlib.util.module_from_spec(spec);spec.loader.exec_module(upgrade)
class UpgradeTests(unittest.TestCase):
 def setUp(self):
  self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup);self.root=Path(self.tmp.name);self.install=self.root/'install';self.media=self.root/'media';self.ops=self.root/'operations';self.media.mkdir();self.ops.mkdir();self.env=self.root/'server.env';self.cfg={'APP_ENV':'production','SMS_PROVIDER':'disabled','DB_NAME':'existing_business','DB_PASSWORD':'not-a-real-password # keep me','MEDIA_DIR':str(self.media),'OPERATIONS_DIR':str(self.ops),'WEB_DIR':str(self.install/'current/web/dist'),'PUBLIC_BASE_URL':'https://example.test','HTTP_ADDR':'0.0.0.0:8089','CONFIG_ENCRYPTION_KEY':'11'*32};self.write()
 def write(self):self.env.write_text('\n'.join(k+'='+v for k,v in self.cfg.items())+'\n',encoding='utf8')
 def test_read_only_context_and_secret_redaction(self):
  before=self.env.read_bytes();cfg,info=upgrade.context(self.env,self.install);self.assertEqual(before,self.env.read_bytes());self.assertNotIn(self.cfg['DB_PASSWORD'],json.dumps(info));self.assertNotIn(self.cfg['CONFIG_ENCRYPTION_KEY'],json.dumps(info));self.assertEqual(info['port'],8089);self.assertEqual(cfg['DB_PASSWORD'],self.cfg['DB_PASSWORD'])
 def test_restore_pointer_uses_actual_database_and_media(self):
  restored=self.root/'restored-media';restored.mkdir();(self.ops/'active.json').write_text(json.dumps({'current':{'database':'restored_business','media_dir':str(restored)}}));_,info=upgrade.context(self.env,self.install);self.assertEqual(info['database'],'restored_business');self.assertEqual(info['media'],str(restored.resolve()))
 def test_pending_restore_is_rejected(self):
  (self.ops/'active.json').write_text('{"pending":true}');self.assertRaisesRegex(ValueError,'pending',upgrade.context,self.env,self.install)
 def test_keys_and_persistent_paths_are_checked(self):
  self.cfg['CONFIG_ENCRYPTION_KEY']='';self.write();self.assertRaisesRegex(ValueError,'ENCRYPTION_KEY',upgrade.context,self.env,self.install);self.cfg['CONFIG_ENCRYPTION_KEY']='11'*32;self.cfg['MEDIA_DIR']='relative';self.write();self.assertRaisesRegex(ValueError,'absolute',upgrade.context,self.env,self.install)
 def test_private_backup_preserves_key_names_and_config(self):
  tools=self.root/'bin';tools.mkdir();[(tools/name).write_text('fake executable for subprocess mock') for name in ['mysql','mysqldump']];self.cfg['MYSQL_BIN']=str(tools);self.write();photo=self.media/'companies/1/logo/file';photo.parent.mkdir(parents=True);photo.write_bytes(b'fixture');before=self.env.read_bytes()
  def run(args,**kwargs):
   self.assertNotIn('text',kwargs);self.assertNotIn('capture_output',kwargs)
   if '--help' in args:return subprocess.CompletedProcess(args,0,stdout='no-tablespaces column-statistics set-gtid-purged',stderr='')
   if any('--execute=' in x for x in args):return subprocess.CompletedProcess(args,0,stdout='14\n',stderr='')
   kwargs['stdout'].write(b'-- isolated fake SQL backup\n');return subprocess.CompletedProcess(args,0,stderr=b'')
  out=self.root/'private-backup'
  with patch.object(upgrade.subprocess,'run',side_effect=run):info=upgrade.backup(self.env,self.install,out)
  self.assertEqual(self.env.read_bytes(),before);self.assertEqual((out/'exhibition.env').read_bytes(),before);self.assertEqual(info['previous_schema'],14);self.assertTrue((out/'SHA256SUMS').is_file());self.assertTrue((out/'media.tar.gz').is_file());self.assertEqual(photo.read_bytes(),b'fixture')
  self.assertIn('  context.json', (out/'SHA256SUMS').read_text())
  for line in (out/'SHA256SUMS').read_text().splitlines():
   digest,name=line.split('  ',1);self.assertEqual(digest,hashlib.sha256((out/name).read_bytes()).hexdigest())
 def test_backup_cannot_write_into_media(self):
  self.assertRaisesRegex(ValueError,'outside media',upgrade.backup,self.env,self.install,self.media/'backup')
 def test_schema_16_target_accepts_16_and_rejects_17(self):
  tools=self.root/'bin';tools.mkdir();[(tools/name).write_text('fake executable') for name in ['mysql','mysqldump']];self.cfg['MYSQL_BIN']=str(tools);self.write()
  current=[16]
  def run(args,**kwargs):
   if any('--execute=' in arg for arg in args):return subprocess.CompletedProcess(args,0,stdout=str(current[0])+'\n',stderr='')
   if '--help' in args:return subprocess.CompletedProcess(args,0,stdout='',stderr='')
   kwargs['stdout'].write(b'-- isolated test backup\n');return subprocess.CompletedProcess(args,0,stderr=b'')
  with patch.object(upgrade.subprocess,'run',side_effect=run):
   result=upgrade.backup(self.env,self.install,self.root/'schema-16',max_schema=16);self.assertEqual(result['previous_schema'],16)
   current[0]=17
   self.assertRaisesRegex(ValueError,'refusing downgrade',upgrade.backup,self.env,self.install,self.root/'schema-17',16)
   current[0]=16
   self.assertRaisesRegex(ValueError,'refusing downgrade',upgrade.backup,self.env,self.install,self.root/'legacy-default')
 def test_schema_21_accepts_20_and_21_rejects_22(self):
  tools=self.root/'bin';tools.mkdir();[(tools/name).write_text('fake executable') for name in ['mysql','mysqldump']];self.cfg['MYSQL_BIN']=str(tools);self.write()
  current=[20]
  def run(args,**kwargs):
   if any('--execute=' in arg for arg in args):return subprocess.CompletedProcess(args,0,stdout=str(current[0])+'\n',stderr='')
   if '--help' in args:return subprocess.CompletedProcess(args,0,stdout='',stderr='')
   kwargs['stdout'].write(b'-- isolated test backup\n');return subprocess.CompletedProcess(args,0,stderr=b'')
  with patch.object(upgrade.subprocess,'run',side_effect=run):
   for version in [20,21]:
    current[0]=version;result=upgrade.backup(self.env,self.install,self.root/('schema-'+str(version)),max_schema=21);self.assertEqual(result['previous_schema'],version)
   current[0]=22
   self.assertRaisesRegex(ValueError,'refusing downgrade',upgrade.backup,self.env,self.install,self.root/'schema-22',max_schema=21)
 def test_ecs_never_accepts_development_or_demo_configuration(self):
  for key,value in [('APP_ENV','development'),('SMS_PROVIDER','development'),('PROCUREMENT_DEMO','true')]:
   with self.subTest(key=key):
    before=self.cfg.copy();self.cfg[key]=value;self.write();self.assertRaises(ValueError,upgrade.context,self.env,self.install);self.cfg=before
  self.write();upgrade.context(self.env,self.install)
 def test_web_root_cannot_be_pinned_to_an_old_release(self):
  self.cfg['WEB_DIR']=str(self.install/'releases/old/web/dist');self.write()
  self.assertRaisesRegex(ValueError,'WEB_DIR must follow',upgrade.context,self.env,self.install)
if __name__=='__main__':unittest.main(verbosity=2)
