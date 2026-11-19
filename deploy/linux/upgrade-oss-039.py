#!/usr/bin/env python3
"""Host-specific, disk-bounded v1.12.4 upgrade. No legacy-media deletion.

Commands are explicit checkpoints. Workstation-encrypted backup is separately
verified before promotion. Candidate SQL is isolated; production data is never
restored automatically during ordinary rollback.
"""
from pathlib import Path
import argparse,hashlib,json,os,subprocess,secrets,shutil,socket,time,runpy,urllib.request,sys

BASE=Path('/opt/zqzt');STATE=BASE/'shared/upgrade-v1.12.4-private.json'
ENV=Path('/etc/zqzt/exhibition.env');UNIT='zqzt-exhibition'

def digest(path):
    with Path(path).open('rb') as stream:return hashlib.file_digest(stream,'sha256').hexdigest()
def atomic(path,value):
    temp=path.with_name(path.name+'.'+secrets.token_hex(4));temp.write_text(json.dumps(value,ensure_ascii=False,indent=2));temp.chmod(0o600);os.replace(temp,path)
def state():return json.loads(STATE.read_text())
def save(value):atomic(STATE,value)
def legacy():return runpy.run_path(str(BASE/'shared/packages/zqzg-v1.12.3/upgrade.py'))
def config():return legacy()['config']()
def sql(cfg,query,db=None):return legacy()['mysql'](cfg,query,db)
def active():
    cfg=config();name,ops=legacy()['database'](cfg);return cfg,name,ops
def get(path,port=8089):
    request=urllib.request.Request('http://127.0.0.1:'+str(port)+path,headers={'Host':'zqzhg.com','X-Forwarded-Proto':'https'})
    with urllib.request.urlopen(request,timeout=20) as r:return r.status,r.read()
def setenv(updates):
    lines=ENV.read_text().splitlines();seen=set();result=[]
    for line in lines:
        key=line.split('=',1)[0] if '=' in line and not line.lstrip().startswith('#') else ''
        if key in updates:result.append(key+'='+updates[key]);seen.add(key)
        else:result.append(line)
    result.extend(k+'='+v for k,v in updates.items() if k not in seen)
    temp=ENV.with_name('exhibition.env.oss-next');temp.write_text('\n'.join(result)+'\n');temp.chmod(0o600);os.replace(temp,ENV)
def wait_health(port):
    for _ in range(100):
        try:
            if get('/api/v1/health',port)[0]==200:return
        except Exception:pass
        time.sleep(.3)
    raise RuntimeError('service_health_timeout')
def media_inventory(root):return {p.relative_to(root).as_posix():digest(p) for p in root.rglob('*') if p.is_file() and not p.is_symlink()}
def filing(cfg,name):
    current=json.loads(sql(cfg,"SELECT value FROM system_settings WHERE setting_key='system_config_v3'",name));current['site']['police_number']='冀公网安备13052802002070号';current['site']['police_url']='https://beian.mps.gov.cn/#/query/webSearch?code=13052802002070';current['version']+=1;current['probe_token']=''
    encoded=json.dumps(current,ensure_ascii=False).encode().hex();sql(cfg,"UPDATE system_settings SET value=CONVERT(0x"+encoded+" USING utf8mb4),updated_at=UTC_TIMESTAMP() WHERE setting_key='system_config_v3'",name)
def start_detached(command,log,env=None,identity=None):
    with open(log,'ab') as stream:
        child=subprocess.Popen(command,env=env,stdout=stream,stderr=stream,stdin=subprocess.DEVNULL,start_new_session=True,preexec_fn=identity)
    return child.pid
def stop_owned(pid,needle):
    if not pid:return
    path=Path('/proc')/str(pid)/'cmdline'
    if not path.exists():return
    assert needle in path.read_bytes().replace(b'\0',b' ').decode(), 'process_identity_mismatch'
    os.kill(pid,15)
    for _ in range(100):
        if not path.exists():return
        stat=(Path('/proc')/str(pid)/'stat').read_text()
        if stat.split(') ',1)[1].startswith('Z'):return
        time.sleep(.2)
    raise RuntimeError('owned_process_stop_timeout')

def check():
    cfg,name,ops=active();old=Path('/opt/zqzt/current').resolve();assert old.name.startswith('v1.12.3-')
    assert sql(cfg,'SELECT MAX(version) FROM schema_migrations',name)=='38'
    media=legacy()['media_directory'](cfg)
    value={'version':'1.12.4','previous_release':str(old),'database':name,'media_root':str(media),'operations_dir':str(ops),'before_counts':legacy()['counts'](cfg,name),'before_media':media_inventory(media),'protected':legacy()['protected'](),'environment_sha256':digest(ENV),'created':time.time()}
    assert shutil.disk_usage(BASE).free>1<<30
    if STATE.exists():
        current=state();assert current['previous_release']==str(old) and not current.get('promoted');value.update(current)
    save(value);print(json.dumps({'checked':True,'schema':38,'media_files':len(value['before_media']),'free_bytes':shutil.disk_usage(BASE).free}))

def verify_candidate(report_path):
    value=state();candidate=value['candidate'];assert value.get('candidate_running')
    report=json.loads(Path(report_path).read_text());assert report['passed'] and len(report['map'])==4 and report['mobileRetry']
    manifest=value['manifest']
    for entry in report['map']:
        code=entry['code'] or 'national';reference=json.loads((Path(value['release'])/'map-demo'/(code+'.mapping.json')).read_text());assert entry['sha256']==reference['sha256']
    assert media_inventory(Path(value['media_root']))==value['before_media']
    current=json.loads(get('/api/v1/public/config',candidate['proxy_port'])[1]);assert current['site']['police_number']=='冀公网安备13052802002070号';assert current['uploads']['default_quota_bytes']==314572800
    value['candidate_verified']=True;save(value);print(json.dumps({'candidate_verified':True,'filing_verified':True,'workstation_backup_restored_sha256':value['backup_restore_sha256']}))

def stage(package):
    value=state();package=Path(package).resolve();manifest=json.loads((package/'manifest.json').read_text())
    assert manifest['version']=='1.12.4' and manifest['schemaVersion']==39 and manifest['realOSSWriteFlowAccepted']
    for line in (package/'SHA256SUMS').read_text().splitlines():
        sha,name=line.split('  ',1);path=(package/name).resolve();assert package in path.parents and digest(path)==sha
    release=BASE/'releases'/('v1.12.4-'+manifest['gitCommit'][:7]);assert not release.exists()
    shutil.copytree(package,release,copy_function=os.link)
    release.chmod(0o755)
    old_download=Path(value['previous_release'])/'web/dist/downloads';download=release/'web/dist/downloads'
    assert not download.exists();shutil.copytree(old_download,download,copy_function=os.link)
    old_manifest=json.loads((old_download/'manifest.json').read_text());archive=download/'archive';archive.mkdir(exist_ok=True)
    old_name='dalunongji-'+old_manifest['version']+'.apk'
    if not (archive/old_name).exists():os.link(old_download/'dalunongji.apk',archive/old_name)
    apk=release/'android/dalunongji.apk';assert digest(apk)==manifest['android']['sha256']
    temporary=download/'dalunongji.apk.next';os.link(apk,temporary);os.replace(temporary,download/'dalunongji.apk')
    # Replace the directory entry, never truncate a retained hard-linked file.
    temporary=download/'manifest.json.next';temporary.write_text(json.dumps(manifest['android'],ensure_ascii=False,indent=2)+'\n');os.replace(temporary,download/'manifest.json')
    versioned=download/manifest['android']['asset'];os.link(apk,versioned)
    for p in release.rglob('*'):
        if p.is_dir():p.chmod(0o755)
        elif 'web/dist/downloads' not in str(p):p.chmod(0o755 if p.name=='exhibition' else 0o644)
    import pwd
    service=pwd.getpwnam('zqzt')
    for p in [release/'map-demo']+list((release/'map-demo').rglob('*')):os.chown(p,service.pw_uid,service.pw_gid)
    assert digest(old_download/'dalunongji.apk')==old_manifest['sha256']
    value.update(release=str(release),manifest=manifest,staged=True);save(value)
    print(json.dumps({'staged':True,'release':str(release),'old_app_retained':True,'new_app':manifest['android']['version']}))

def candidate_start():
    value=state();assert value['staged'] and not value.get('candidate_running')
    cfg,name,_=active();work=BASE/'shared'/('oss1124-candidate-'+secrets.token_hex(6));work.mkdir(mode=0o750)
    for folder in ['mysql','operations','temp']: (work/folder).mkdir(mode=0o700)
    import pwd
    service=pwd.getpwnam('zqzt');os.chown(work,0,service.pw_gid)
    for folder in ['operations','temp']:os.chown(work/folder,service.pw_uid,service.pw_gid)
    def identity():os.initgroups('zqzt',service.pw_gid);os.setgid(service.pw_gid);os.setuid(service.pw_uid)
    secret=secrets.token_hex(24);port=13404;app_port=18124;proxy_port=18123
    for p in [port,app_port,proxy_port]:
        probe=socket.socket();assert probe.connect_ex(('127.0.0.1',p))!=0;probe.close()
    common=['/usr/sbin/mysqld','--no-defaults','--user=root','--datadir='+str(work/'mysql'),'--innodb-buffer-pool-size=32M','--innodb-log-buffer-size=4M','--innodb-redo-log-capacity=32M','--performance-schema=OFF','--max-connections=20','--table-open-cache=128','--mysqlx=OFF','--skip-log-bin','--bind-address=127.0.0.1','--port='+str(port),'--socket='+str(work/'mysql.sock'),'--pid-file='+str(work/'mysql.pid'),'--log-error='+str(work/'mysql.log')]
    subprocess.run(common+['--initialize-insecure'],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
    init=work/'init.sql';init.write_text("ALTER USER 'root'@'localhost' IDENTIFIED BY '"+secret+"';");init.chmod(0o600)
    mysql_pid=start_detached(common+['--init-file='+str(init)],work/'mysql-stdout.log')
    isolated=dict(cfg,DB_HOST='127.0.0.1',DB_PORT=str(port),DB_USER='root',DB_PASSWORD=secret,DB_MAINTENANCE_USER='',DB_MAINTENANCE_PASSWORD='')
    value.update(candidate_running=True,candidate={'work':str(work),'mysql_pid':mysql_pid,'app_pid':None,'proxy_pid':None,'config':isolated,'database':'oss1124_candidate','proxy_port':proxy_port,'app_port':app_port});save(value)
    for _ in range(100):
        try:
            if sql(isolated,'SELECT 1')=='1':break
        except Exception:pass
        time.sleep(.3)
    else:raise RuntimeError('isolated_mysql_timeout')
    init.unlink();db='oss1124_candidate';sql(isolated,'CREATE DATABASE '+db+' CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci')
    restore=subprocess.Popen(['mysql','--protocol=TCP','--host=127.0.0.1','--port='+str(port),'--user=root','--binary-mode=1',db],env=dict(os.environ,MYSQL_PWD=secret),stdin=subprocess.PIPE,stdout=subprocess.DEVNULL,stderr=subprocess.PIPE)
    restored_sha=hashlib.sha256();restored_bytes=0
    while True:
        block=sys.stdin.buffer.read(1024*1024)
        if not block:break
        restored_sha.update(block);restored_bytes+=len(block);restore.stdin.write(block)
    restore.stdin.close();assert restore.wait()==0 and restored_bytes>0,'candidate_restore_failed'
    filing(isolated,db)
    env=dict(os.environ,**isolated);release=Path(value['release'])
    env.update(APP_ENV='production',HTTP_ADDR='127.0.0.1:'+str(app_port),DB_NAME=db,MEDIA_DIR=value['media_root'],OPERATIONS_DIR=str(work/'operations'),MEDIA_TEMP_DIR=str(work/'temp'),WEB_DIR=str(release/'web/dist'),MAP_DEMO_ENABLED='true',MAP_DEMO_DIR=str(release/'map-demo'),BACKGROUND_WORKERS_ENABLED='false',AI_IMPORT_WORKER_ENABLED='false',SMS_PROVIDER='disabled',SMS_ENABLED='false',OSS_ENABLED='true',OSS_UPLOADS_ENABLED='true',OSS_FILE_WORKER_ENABLED='true',OSS_UPLOAD_TIMEOUT='30')
    app_pid=start_detached([str(release/'server/exhibition')],work/'application.log',env,identity);wait_health(app_port)
    value['candidate']['app_pid']=app_pid;save(value)
    nginx=work/'nginx.conf';nginx.write_text('pid '+str(work/'nginx.pid')+'; error_log '+str(work/'nginx.log')+'; events {} http { access_log off; server { listen 127.0.0.1:'+str(proxy_port)+'; location / { proxy_pass http://127.0.0.1:'+str(app_port)+'; proxy_set_header Host zqzhg.com; proxy_set_header X-Forwarded-Proto https; } } }')
    proxy_pid=start_detached(['nginx','-c',str(nginx),'-p',str(work),'-g','daemon off;'],work/'nginx-stdout.log');wait_health(proxy_port)
    value.update(candidate_running=True,backup_restore_sha256=restored_sha.hexdigest(),candidate={'work':str(work),'mysql_pid':mysql_pid,'app_pid':app_pid,'proxy_pid':proxy_pid,'config':isolated,'database':db,'proxy_port':proxy_port,'app_port':app_port});save(value)
    assert sql(isolated,'SELECT MAX(version) FROM schema_migrations',db)=='39'
    assert legacy()['counts'](isolated,db)==value['before_counts']
    print(json.dumps({'candidate_started':True,'proxy_port':proxy_port,'schema':39,'production_configuration':True,'service_user':'zqzt','source_media_copied_or_deleted':False}))

def candidate_stop():
    value=state();candidate=value['candidate'];work=Path(candidate['work'])
    stop_owned(candidate['proxy_pid'],str(work/'nginx.conf'));stop_owned(candidate['app_pid'],value['release']+'/server/exhibition');stop_owned(candidate['mysql_pid'],str(work/'mysql'))
    value['candidate_running']=False;value['candidate_stopped']=True;save(value)
    # Only the newly created isolated database is reclaimed, never media/backups.
    target=work/'mysql';assert work.parent==BASE/'shared' and work.name.startswith('oss1124-candidate-') and target.resolve().parent==work.resolve()
    if target.exists():shutil.rmtree(target)
    print(json.dumps({'candidate_stopped':True,'isolated_database_reclaimed':True}))

def freeze():
    value=state();assert value.get('candidate_verified') and not value.get('candidate_running')
    assert Path('/opt/zqzt/current').resolve()==Path(value['previous_release'])
    subprocess.run(['systemctl','stop',UNIT],check=True);value['frozen_at']=time.time();save(value);print(json.dumps({'frozen':True,'refresh_workstation_backup_now':True}))

def promote(backup_receipt):
    value=state();assert value.get('candidate_verified') and not value.get('candidate_running')
    assert subprocess.run(['systemctl','is-active','--quiet',UNIT]).returncode!=0
    backup=json.loads(Path(backup_receipt).read_text());assert backup['verified'] and backup['captured_at']>=value['frozen_at']
    assert legacy()['protected']()==value['protected']
    oldenv=Path('/etc/zqzt/exhibition.env.before-v1.12.4');assert not oldenv.exists();shutil.copy2(ENV,oldenv);oldenv.chmod(0o600)
    cfg,name,_=active();assert name==value['database'];assert media_inventory(Path(value['media_root']))==value['before_media']
    # Only public filing fields change; all contact data, credentials, AI models,
    # tenant overrides and other settings are retained.
    filing(cfg,name)
    release=Path(value['release']);setenv({'WEB_DIR':str(release/'web/dist'),'MAP_DEMO_DIR':str(release/'map-demo'),'MAP_DEMO_ENABLED':'true','OSS_ENABLED':'true','OSS_UPLOADS_ENABLED':'false','OSS_DIRECT_UPLOAD_ENABLED':'true','OSS_FILE_WORKER_ENABLED':'false','OSS_ALLOW_LOCAL_WRITES':'false'})
    link=BASE/'current.oss-next';assert not link.exists();link.symlink_to(release);os.replace(link,BASE/'current')
    try:
        subprocess.run(['systemctl','start',UNIT],check=True);wait_health(8089)
        assert sql(config(),'SELECT MAX(version) FROM schema_migrations',name)=='39'
        assert media_inventory(Path(value['media_root']))==value['before_media']
        assert legacy()['counts'](config(),name)==value['before_counts']
        assert legacy()['protected']()==value['protected']
    except Exception:
        value['promotion_failed']=True;save(value);raise
    value.update(promoted=True,backup_receipt=backup);save(value);print(json.dumps({'promoted':True,'version':'1.12.4','schema':39,'legacy_media_unchanged':True,'app':value['manifest']['android']['version']}))

def enable_uploads():
    value=state();assert value.get('promoted') and value.get('production_verified')
    setenv({'OSS_UPLOADS_ENABLED':'true'});subprocess.run(['systemctl','restart',UNIT],check=True);wait_health(8089)
    app=dict(value['manifest']['android'],productionDeployed=True,releaseStatus='stable');path=Path(value['release'])/'web/dist/downloads/manifest.json';temp=path.with_suffix('.next');temp.write_text(json.dumps(app,ensure_ascii=False,indent=2)+'\n');temp.chmod(0o644);os.replace(temp,path)
    value['uploads_enabled']=True;save(value);print(json.dumps({'new_uploads_enabled':True,'app_published':app['version']}))

def pause_uploads():
    value=state();assert value.get('promoted');setenv({'OSS_UPLOADS_ENABLED':'false'});subprocess.run(['systemctl','restart',UNIT],check=True);wait_health(8089);print(json.dumps({'new_uploads_paused':True,'existing_reads_retained':True}))

def read_fallback():
    value=state();assert value.get('promoted');setenv({'OSS_ENABLED':'false','OSS_UPLOADS_ENABLED':'false','OSS_DIRECT_UPLOAD_ENABLED':'false','OSS_FILE_WORKER_ENABLED':'false','OSS_ALLOW_LOCAL_WRITES':'false'});subprocess.run(['systemctl','restart',UNIT],check=True);wait_health(8089);print(json.dumps({'read_compatible_fallback':True,'schema_retained':39,'database_restored':False,'legacy_release_not_reactivated':True}))

def rollback():
    value=state();cfg,name,_=active();version=int(sql(cfg,'SELECT MAX(version) FROM schema_migrations',name))
    cloud=0
    if version>=39:
        cloud=int(sql(cfg,"SELECT (SELECT COUNT(*) FROM stored_objects WHERE remote_key<>'' OR store_id IN (SELECT id FROM storage_profiles WHERE auth_mode='ecs_role'))+(SELECT COUNT(*) FROM file_upload_sessions)+(SELECT COUNT(*) FROM file_storage_jobs)",name))
    if cloud:
        value['promoted']=True;save(value);read_fallback();return
    subprocess.run(['systemctl','stop',UNIT],check=True)
    original=Path('/etc/zqzt/exhibition.env.before-v1.12.4')
    if original.exists():
        temporary=ENV.with_name('exhibition.env.rollback');shutil.copy2(original,temporary);temporary.chmod(0o600);os.replace(temporary,ENV)
    link=BASE/'current.rollback';assert not link.exists();link.symlink_to(value['previous_release']);os.replace(link,BASE/'current')
    subprocess.run(['systemctl','start',UNIT],check=True);wait_health(8089);value['rolled_back_before_cloud_files']=True;save(value);print(json.dumps({'previous_program_restored':True,'database_restored':False,'new_schema_retained':version}))

def main():
    assert os.geteuid()==0
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('action',choices=['check','stage','candidate-start','verify-candidate','candidate-stop','freeze','promote','enable-uploads','pause-uploads','read-fallback','rollback']);parser.add_argument('path',nargs='?');args=parser.parse_args()
    if args.action=='check':check()
    elif args.action=='stage':stage(args.path)
    elif args.action=='candidate-start':candidate_start()
    elif args.action=='verify-candidate':verify_candidate(args.path)
    elif args.action=='candidate-stop':candidate_stop()
    elif args.action=='freeze':freeze()
    elif args.action=='promote':promote(args.path)
    elif args.action=='enable-uploads':enable_uploads()
    elif args.action=='pause-uploads':pause_uploads()
    elif args.action=='rollback':rollback()
    else:read_fallback()

if __name__=='__main__':main()
