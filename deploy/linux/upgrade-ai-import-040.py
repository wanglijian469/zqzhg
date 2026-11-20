#!/usr/bin/env python3
"""Site-specific Schema40 upgrade checkpoints; reads the existing protected
upgrade-v1.12.4 helpers. Never includes credentials or a database backup.
Use the release README and operator receipts before promotion or rollback.
"""
from pathlib import Path
import runpy,json,sys,hashlib,shutil,os,tarfile,pwd,subprocess,time,secrets,socket
n=runpy.run_path('/opt/zqzt/shared/packages/zqzg-v1.12.4/upgrade.py');S=Path('/opt/zqzt/shared/ai-resilience/deploy-state.json')
def save(s):n['atomic'](S,s)
def read():return json.loads(S.read_text())
def put(p,data,mode=0o644):
 t=p.with_name(p.name+'.video-next');t.write_bytes(data);t.chmod(mode);os.replace(t,p)
def identity():
 u=pwd.getpwnam('zqzt');os.initgroups('zqzt',u.pw_gid);os.setgid(u.pw_gid);os.setuid(u.pw_uid)
action=sys.argv[1]
if action=='stage':
 package=Path(sys.argv[2]);S.parent.mkdir(exist_ok=True);assert n['digest'](package)==sys.argv[3];old=Path('/opt/zqzt/current').resolve();assert old.name=='v1.12.5-1012190'
 with tarfile.open(package) as t:
  m=json.load(t.extractfile('ai-resilience/patch.json'));commit=m['gitCommit'];assert len(commit)==40 and m['schemaVersion']==40
  release=old.with_name('v1.12.6-'+commit[:7]);assert not release.exists();shutil.copytree(old,release,copy_function=os.link)
  for entry in t:
   parts=Path(entry.name).parts;assert entry.isfile() and parts[0]=='ai-resilience' and '..' not in parts and not entry.name.startswith('/')
   if parts[1]=='patch.json':continue
   relative='/'.join(parts[1:]);assert parts[1] in ['server','web','map-demo','docs','deploy'];data=t.extractfile(entry).read();assert hashlib.sha256(data).hexdigest()==m['files'][relative]
   p=release.joinpath(*parts[1:]);p.parent.mkdir(parents=True,exist_ok=True)
   if not p.exists() or n['digest'](p)!=m['files'][relative]:put(p,data,0o755 if relative=='server/exhibition' else 0o644)
 assert sum(k.startswith('map-demo/') for k in m['files'])==77
 for k,h in m['files'].items():assert n['digest'](release/k)==h
 previous=json.loads((old/'manifest.json').read_text());previous.update(version=m['version'],gitCommit=commit,webCommit=commit,schemaVersion=40,videoAutoConversion=True,androidVersion='1.12.1+54');put(release/'manifest.json',(json.dumps(previous,indent=2)+'\n').encode())
 put(release/'SHA256SUMS',''.join(n['digest'](p)+'  '+p.relative_to(release).as_posix()+'\n' for p in sorted(release.rglob('*')) if p.is_file() and p.name!='SHA256SUMS').encode())
 u=pwd.getpwnam('zqzt')
 for p in [release/'map-demo']+[p for p in (release/'map-demo').rglob('*') if p.is_dir()]:os.chown(p,u.pw_uid,u.pw_gid)
 cfg,db,_=n['active']();s={'old':str(old),'release':str(release),'commit':commit,'before_counts':n['legacy']()['counts'](cfg,db),'protected':n['legacy']()['protected'](),'old_env':{k:cfg.get(k) for k in ['WEB_DIR','MAP_DEMO_DIR','MAX_VIDEO_SECONDS','OSS_VIDEO_AUTO_CONVERT_ENABLED']},'candidate_running':False};save(s)
 print(json.dumps({'staged':True,'release':str(release),'maps':77,'schema':40,'android':'1.12.1+54'}),flush=True)
elif action=='candidate':
 s=read();assert not s['candidate_running'];cfg,db,_=n['active']();work=Path('/opt/zqzt/shared/ai-candidate-'+secrets.token_hex(5));work.mkdir(mode=0o750);u=pwd.getpwnam('zqzt');os.chown(work,0,u.pw_gid)
 for f in ['mysql','operations','temp','media']:(work/f).mkdir(mode=0o700)
 for f in ['operations','temp','media']:os.chown(work/f,u.pw_uid,u.pw_gid)
 secret=secrets.token_hex(24);port=13405;app=18130;proxy=18129
 for p in [port,app,proxy]:
  probe=socket.socket();assert probe.connect_ex(('127.0.0.1',p))!=0;probe.close()
 common=['/usr/sbin/mysqld','--no-defaults','--user=root','--datadir='+str(work/'mysql'),'--innodb-buffer-pool-size=32M','--innodb-log-buffer-size=4M','--innodb-redo-log-capacity=32M','--performance-schema=OFF','--max-connections=20','--table-open-cache=128','--mysqlx=OFF','--skip-log-bin','--bind-address=127.0.0.1','--port='+str(port),'--socket='+str(work/'mysql.sock'),'--pid-file='+str(work/'mysql.pid'),'--log-error='+str(work/'mysql.log')]
 subprocess.run(common+['--initialize-insecure'],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 init=work/'init.sql';init.write_text("ALTER USER 'root'@'localhost' IDENTIFIED BY '"+secret+"';");init.chmod(0o600)
 pid=n['start_detached'](common+['--init-file='+str(init)],work/'mysql-stdout.log')
 isolated=dict(cfg,DB_HOST='127.0.0.1',DB_PORT=str(port),DB_USER='root',DB_PASSWORD=secret,DB_MAINTENANCE_USER='',DB_MAINTENANCE_PASSWORD='');access={k:isolated[k] for k in ['DB_HOST','DB_PORT','DB_USER','DB_PASSWORD']};testdb='ai1126_candidate'
 c={'work':str(work),'mysql_pid':pid,'config':access,'database':testdb,'app_port':app,'proxy_port':proxy};s.update(candidate_running=True,candidate=c);save(s)
 for _ in range(100):
  try:
   if n['sql'](isolated,'SELECT 1')=='1':break
  except Exception:pass
  time.sleep(.3)
 else:raise RuntimeError('isolated_mysql_timeout')
 init.unlink();n['sql'](isolated,'CREATE DATABASE '+testdb+' CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci')
 restore=subprocess.Popen(['mysql','--protocol=TCP','--host=127.0.0.1','--port='+str(port),'--user=root','--binary-mode=1',testdb],env=dict(os.environ,MYSQL_PWD=secret),stdin=subprocess.PIPE,stdout=subprocess.DEVNULL,stderr=subprocess.PIPE)
 dump=subprocess.Popen(['mysqldump','--protocol=TCP','--host='+cfg.get('DB_HOST','127.0.0.1'),'--port='+cfg.get('DB_PORT','3306'),'--user='+cfg['DB_USER'],'--single-transaction','--hex-blob','--set-gtid-purged=OFF','--no-tablespaces',db],env=dict(os.environ,MYSQL_PWD=cfg['DB_PASSWORD']),stdout=restore.stdin,stderr=subprocess.PIPE)
 restore.stdin.close();assert dump.wait()==0 and restore.wait()==0,'isolated_stream_failed'
 # No copied queue or public-object garbage collector may act on production objects.
 n['sql'](isolated,"DELETE FROM file_upload_sessions;DELETE FROM media_upload_reservations;DELETE FROM file_storage_job_items;DELETE FROM file_storage_jobs;DELETE FROM file_backup_pins;DELETE FROM sessions;UPDATE enterprise_file_objects SET object_state='fixture-preserved';UPDATE ai_import_tasks SET status='review_pending',lease='',locked_until=NULL WHERE status IN ('processing','queued')",testdb)
 # Read-only replicas let the candidate serve unmigrated local media while its
 # own primary directory remains separate. No original media is copied or moved.
 original_media=n['legacy']()['media_directory'](cfg).as_posix()
 n['sql'](isolated,"INSERT INTO stored_object_replicas(object_key,store_id,local_root,size,sha256,verified_at) SELECT object_key,0,'"+original_media+"',size,sha256,UTC_TIMESTAMP() FROM stored_objects WHERE store_id=0 OR store_id IN (SELECT id FROM storage_profiles WHERE kind='local') ON DUPLICATE KEY UPDATE local_root=VALUES(local_root),size=VALUES(size),sha256=VALUES(sha256)",testdb)
 env=dict(os.environ,**isolated);env.update(APP_ENV='production',HTTP_ADDR='127.0.0.1:'+str(app),DB_NAME=testdb,MEDIA_DIR=str(work/'media'),OPERATIONS_DIR=str(work/'operations'),MEDIA_TEMP_DIR=str(work/'temp'),WEB_DIR=s['release']+'/web/dist',MAP_DEMO_DIR=s['release']+'/map-demo',MAP_DEMO_ENABLED='true',BACKGROUND_WORKERS_ENABLED='false',AI_IMPORT_WORKER_ENABLED='true',OSS_ENABLED='true',OSS_UPLOADS_ENABLED='true',OSS_FILE_WORKER_ENABLED='false',SMS_PROVIDER='disabled',SMS_ENABLED='false',MAX_VIDEO_SECONDS='300',OSS_VIDEO_AUTO_CONVERT_ENABLED='true')
 c['app_pid']=n['start_detached']([s['release']+'/server/exhibition'],work/'application.log',env,identity);save(s);n['wait_health'](app)
 conf=work/'nginx.conf';conf.write_text('pid '+str(work/'nginx.pid')+'; error_log '+str(work/'nginx.log')+'; events {} http { access_log off; server { listen 127.0.0.1:'+str(proxy)+'; location / { proxy_pass http://127.0.0.1:'+str(app)+'; proxy_set_header Host zqzhg.com; proxy_set_header X-Forwarded-Proto https; } } }')
 c['proxy_pid']=n['start_detached'](['nginx','-c',str(conf),'-p',str(work),'-g','daemon off;'],work/'proxy.log');save(s);n['wait_health'](proxy)
 print(json.dumps({'candidate_started':True,'port':proxy,'schema':40,'queues_sanitized':True}),flush=True)
elif action=='stop':
 s=read();c=s['candidate'];n['stop_owned'](c.get('proxy_pid'),c['work']+'/nginx.conf');n['stop_owned'](c.get('app_pid'),s['release']+'/server/exhibition');n['stop_owned'](c.get('mysql_pid'),c['work']+'/mysql');s['candidate_running']=False;save(s);print(json.dumps({'stopped':True}))
elif action in ['copy-dry','copy-apply']:
 s=read();cfg,db,_=n['active']();before=n['sql'](cfg,'SELECT MAX(version) FROM schema_migrations',db);env=dict(os.environ,**cfg);env['DB_NAME']=db;env['OPERATIONS_DIR']=str(n['active']()[2]);env['MEDIA_DIR']=n['legacy']()['media_directory'](cfg).as_posix()
 p=subprocess.run([s['release']+'/server/exhibition','ai-copy-cleanup','--apply' if action=='copy-apply' else '--dry-run'],env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
 assert p.returncode==0,'copy_cleanup_failed'
 rows=json.loads(p.stdout.decode().strip().splitlines()[-1]);assert n['sql'](cfg,'SELECT MAX(version) FROM schema_migrations',db)==before
 n['atomic'](S.parent/(action+'.json'),{'rows':rows,'schema':int(before),'publishedRevisionsUnchanged':True});print(json.dumps({'schema':int(before),'rows':rows}))
elif action=='promote':
 s=read();assert not s['candidate_running'];r=json.loads(Path(sys.argv[2]).read_text());assert r['passed'] and len(r['map'])==4 and len(r['layouts'])==5 and r['mobileRetry'];assert s.get('functional_passed')
 cfg,db,_=n['active']();assert n['sql'](cfg,"SELECT COUNT(*) FROM ai_import_tasks WHERE status IN ('queued','processing')",db)=='0','active_tasks_wait';assert Path('/opt/zqzt/current').resolve()==Path(s['old']);assert n['legacy']()['counts'](cfg,db)==s['before_counts'];s['old_env']={k:cfg.get(k) for k in ['WEB_DIR','MAP_DEMO_DIR']};save(s)
 n['setenv']({'WEB_DIR':s['release']+'/web/dist','MAP_DEMO_DIR':s['release']+'/map-demo'})
 link=Path('/opt/zqzt/current.ai-next');assert not link.exists();link.symlink_to(s['release']);os.replace(link,'/opt/zqzt/current')
 try:
  subprocess.run(['systemctl','restart','zqzt-exhibition'],check=True);n['wait_health'](8089);assert n['sql'](cfg,'SELECT MAX(version) FROM schema_migrations',db)=='40';assert n['legacy']()['counts'](cfg,db)==s['before_counts']
 except Exception:
  n['setenv'](s['old_env']);link.symlink_to(s['old']);os.replace(link,'/opt/zqzt/current');subprocess.run(['systemctl','restart','zqzt-exhibition'],check=True);raise
 s['promoted']=True;save(s);print(json.dumps({'promoted':True,'version':'1.12.6','schema':40,'android':'1.12.1+54','old_business_counts_unchanged':True}))
elif action=='verify':
 s=read();assert s['promoted'];cfg,db,_=n['active']();assert n['legacy']()['protected']()==s['protected'];assert cfg['OSS_ENABLED']=='true' and cfg['OSS_UPLOADS_ENABLED']=='true';assert n['sql'](cfg,'SELECT MAX(version) FROM schema_migrations',db)=='40';print(json.dumps({'healthy':True,'schema':40,'oss_uploads_retained':True,'protected_unchanged':True,'current':str(Path('/opt/zqzt/current').resolve())}))
elif action=='stage-next':
 s=read();package=Path(sys.argv[2]);assert n['digest'](package)==sys.argv[3]
 with tarfile.open(package) as t:
  m=json.load(t.extractfile('ai-resilience/patch.json'));assert m['schemaVersion']==40;release=Path(s['old']).with_name('v1.12.6-'+m['gitCommit'][:7]);assert not release.exists();shutil.copytree(s.get('next_release',s['release']),release,copy_function=os.link)
  for entry in t:
   parts=Path(entry.name).parts;assert entry.isfile() and parts[0]=='ai-resilience' and '..' not in parts and not entry.name.startswith('/')
   if parts[1]=='patch.json':continue
   relative='/'.join(parts[1:]);assert parts[1] in ['server','web','map-demo','docs','deploy'];data=t.extractfile(entry).read();assert hashlib.sha256(data).hexdigest()==m['files'][relative];p=release.joinpath(*parts[1:]);p.parent.mkdir(parents=True,exist_ok=True)
   if not p.exists() or n['digest'](p)!=m['files'][relative]:put(p,data,0o755 if relative=='server/exhibition' else 0o644)
 assert sum(k.startswith('map-demo/') for k in m['files'])==77
 for k,h in m['files'].items():assert n['digest'](release/k)==h
 manifest=json.loads((release/'manifest.json').read_text());manifest.update(version='1.12.6',gitCommit=m['gitCommit'],webCommit=m.get('webCommit',m['gitCommit']),schemaVersion=40,androidVersion='1.12.1+54');manifest.pop('webPatchCommit',None);manifest.pop('webPatch',None);put(release/'manifest.json',(json.dumps(manifest,indent=2)+'\n').encode());put(release/'SHA256SUMS',''.join(n['digest'](p)+'  '+p.relative_to(release).as_posix()+'\n' for p in sorted(release.rglob('*')) if p.is_file() and p.name!='SHA256SUMS').encode())
 u=pwd.getpwnam('zqzt')
 for p in [release/'map-demo']+[p for p in (release/'map-demo').rglob('*') if p.is_dir()]:os.chown(p,u.pw_uid,u.pw_gid)
 s['next_release']=str(release);s['next_commit']=m['gitCommit'];save(s);print(json.dumps({'final_artifact_staged':True,'release':str(release),'schema':40,'maps':77,'android':'1.12.1+54'}))
elif action=='review-candidate':
 s=read();assert 'next_release' in s and not s.get('review_candidate');cfg,db,_=n['active']();c=s['candidate'];work=Path(c['work']);ops=work/'review-operations';ops.mkdir(mode=0o700,exist_ok=True);u=pwd.getpwnam('zqzt');os.chown(ops,u.pw_uid,u.pw_gid)
 isolated=dict(cfg,**c['config']);env=dict(os.environ,**isolated);env.update(APP_ENV='production',HTTP_ADDR='127.0.0.1:18132',DB_NAME=c['database'],MEDIA_DIR=str(work/'media'),OPERATIONS_DIR=str(ops),MEDIA_TEMP_DIR=str(work/'temp'),WEB_DIR=s['next_release']+'/web/dist',MAP_DEMO_DIR=s['next_release']+'/map-demo',MAP_DEMO_ENABLED='true',BACKGROUND_WORKERS_ENABLED='false',AI_IMPORT_WORKER_ENABLED='false',OSS_FILE_WORKER_ENABLED='false',OSS_ENABLED='true',OSS_UPLOADS_ENABLED='true',SMS_PROVIDER='disabled',SMS_ENABLED='false',MAX_VIDEO_SECONDS='300',OSS_VIDEO_AUTO_CONVERT_ENABLED='true')
 app=n['start_detached']([s['next_release']+'/server/exhibition'],work/'review-application.log',env,identity);n['wait_health'](18132);conf=work/'review-nginx.conf';conf.write_text('pid '+str(work/'review-nginx.pid')+'; error_log '+str(work/'review-nginx.log')+'; events {} http { access_log off; server { listen 127.0.0.1:18131; location / { proxy_pass http://127.0.0.1:18132; proxy_set_header Host zqzhg.com; proxy_set_header X-Forwarded-Proto https; } } }');proxy=n['start_detached'](['nginx','-c',str(conf),'-p',str(work),'-g','daemon off;'],work/'review-proxy.log');s['review_candidate']={'app_pid':app,'proxy_pid':proxy,'work':str(work),'release':s['next_release']};save(s);n['wait_health'](18131);print(json.dumps({'final_candidate':True,'port':18131,'worker':False}))
elif action=='accept-final':
 s=read();assert s.get('review_candidate');c=s['candidate'];assert n['sql'](c['config'],"SELECT COUNT(*) FROM ai_import_tasks WHERE status IN ('queued','processing')",c['database'])=='0'
 review=s['review_candidate'];n['stop_owned'](review['proxy_pid'],review['work']+'/review-nginx.conf');n['stop_owned'](review['app_pid'],review['release']+'/server/exhibition');n['stop_owned'](c['proxy_pid'],c['work']+'/nginx.conf');n['stop_owned'](c['app_pid'],s['release']+'/server/exhibition');n['stop_owned'](c['mysql_pid'],c['work']+'/mysql');s['candidate_running']=False;s['real_test_release']=s['release'];s['release']=s['next_release'];s['commit']=s['next_commit'];s['functional_passed']=True;save(s);print(json.dumps({'final_candidate_stopped':True,'preserved_real_test_database':True,'production_untouched':True}))

elif action=='stop-review':
 s=read();r=s.get('review_candidate');assert r;n['stop_owned'](r['proxy_pid'],r['work']+'/review-nginx.conf');n['stop_owned'](r['app_pid'],r['release']+'/server/exhibition');s.setdefault('review_history',[]).append(r);s.pop('review_candidate');save(s);print(json.dumps({'review_candidate_stopped':True,'recognition_worker_untouched':True}))
elif action=='switch-ai-worker':
 s=read();c=s['candidate'];assert n['sql'](c['config'],"SELECT COUNT(*) FROM ai_import_tasks WHERE status IN ('queued','processing')",c['database'])=='0';n['stop_owned'](c['app_pid'],s['release']+'/server/exhibition');cfg,db,_=n['active']();isolated=dict(cfg,**c['config']);work=Path(c['work']);env=dict(os.environ,**isolated);env.update(APP_ENV='production',HTTP_ADDR='127.0.0.1:'+str(c['app_port']),DB_NAME=c['database'],MEDIA_DIR=str(work/'media'),OPERATIONS_DIR=str(work/'operations'),MEDIA_TEMP_DIR=str(work/'temp'),WEB_DIR=s['next_release']+'/web/dist',MAP_DEMO_DIR=s['next_release']+'/map-demo',MAP_DEMO_ENABLED='true',BACKGROUND_WORKERS_ENABLED='false',AI_IMPORT_WORKER_ENABLED='true',OSS_FILE_WORKER_ENABLED='false',OSS_ENABLED='true',OSS_UPLOADS_ENABLED='true',SMS_PROVIDER='disabled',SMS_ENABLED='false',MAX_VIDEO_SECONDS='300',OSS_VIDEO_AUTO_CONVERT_ENABLED='true');c['app_pid']=n['start_detached']([s['next_release']+'/server/exhibition'],work/'final-recognition.log',env,identity);s['recognition_original_release']=s['release'];s['release']=s['next_release'];save(s);n['wait_health'](c['app_port']);print(json.dumps({'recognition_worker_updated':True,'checkpoints_kept':True}))
elif action=='rollback-check':
 s=read();c=s['candidate'];cfg,_,_=n['active']();work=Path(c['work']);ops=work/'rollback-check-operations';ops.mkdir(mode=0o700,exist_ok=True);u=pwd.getpwnam('zqzt');os.chown(ops,u.pw_uid,u.pw_gid);env=dict(os.environ,**dict(cfg,**c['config']));env.update(APP_ENV='production',HTTP_ADDR='127.0.0.1:18134',DB_NAME=c['database'],MEDIA_DIR=str(work/'media'),OPERATIONS_DIR=str(ops),MEDIA_TEMP_DIR=str(work/'temp'),WEB_DIR=s['old']+'/web/dist',MAP_DEMO_DIR=s['old']+'/map-demo',BACKGROUND_WORKERS_ENABLED='false',AI_IMPORT_WORKER_ENABLED='false',OSS_FILE_WORKER_ENABLED='false',SMS_PROVIDER='disabled',SMS_ENABLED='false');before=n['sql'](c['config'],'SELECT COUNT(*) FROM ai_import_product_drafts',c['database']);pid=n['start_detached']([s['old']+'/server/exhibition'],work/'rollback-check.log',env,identity)
 try:
  n['wait_health'](18134);assert n['sql'](c['config'],'SELECT MAX(version) FROM schema_migrations',c['database'])=='40';assert n['sql'](c['config'],'SELECT COUNT(*) FROM ai_import_product_drafts',c['database'])==before;result={'previous_program_started':True,'schema40_retained':True,'ai_drafts_retained':True,'passed':True};n['atomic'](S.parent/'rollback-check.json',result);print(json.dumps(result))
 finally:n['stop_owned'](pid,s['old']+'/server/exhibition')
elif action=='rollback':
 s=read();assert s.get('promoted') and Path('/opt/zqzt/current').resolve()==Path(s['release']);cfg,db,_=n['active']();assert n['sql'](cfg,"SELECT COUNT(*) FROM ai_import_tasks WHERE status IN ('queued','processing')",db)=='0','active_tasks_wait';before=n['legacy']()['counts'](cfg,db);s['forward_env']={k:cfg.get(k) for k in ['WEB_DIR','MAP_DEMO_DIR','AI_IMPORT_ENABLED','AI_IMPORT_WORKER_ENABLED']};save(s);n['setenv'](dict(s['old_env'],AI_IMPORT_ENABLED='false',AI_IMPORT_WORKER_ENABLED='false'));link=Path('/opt/zqzt/current.ai-rollback');assert not link.exists();link.symlink_to(s['old']);os.replace(link,'/opt/zqzt/current');subprocess.run(['systemctl','restart','zqzt-exhibition'],check=True);n['wait_health'](8089);assert n['sql'](cfg,'SELECT MAX(version) FROM schema_migrations',db)=='40';assert n['legacy']()['counts'](cfg,db)==before;s['rolled_back']=True;save(s);print(json.dumps({'program_rolled_back':True,'schema40_kept':True,'business_data_kept':True,'ai_disabled':True}))
elif action=='approve-candidate':
 s=read();assert not s['candidate_running'];web=json.loads(Path(sys.argv[2]).read_text());app=json.loads(Path(sys.argv[3]).read_text());api=json.loads(Path(sys.argv[4]).read_text())
 for r in [web,app]:assert r['passed'] and len(r['map'])==4 and len(r['layouts'])==5 and r['mobileRetry']
 assert app['embeddedAPKAssets'] and api['passed'] and api['technical_values_unchanged'] and api['private_evidence_retained'] and api['cross_enterprise_task_denied'] and api['merchant_cannot_read_admin_prompts']
 cfg,db,_=n['active']();assert n['legacy']()['counts'](cfg,db)==s['before_counts'];assert Path('/opt/zqzt/current').resolve()==Path(s['old']);s['functional_passed']=True;s['candidate_receipts']=[str(Path(v)) for v in sys.argv[2:5]];save(s);print(json.dumps({'candidate_gate_passed':True,'ready_for_promotion':True}))

else:
 raise ValueError('unsupported_checkpoint')
