"""Operator-only encrypted workstation backup. Never writes backup data on ECS.

SSH helper owns host verification and credentials; they are never printed.
AES-256-GCM objects are content-addressed and authenticated against their SHA256.
The key stays under the operator-only, Git-ignored .secrets directory.
"""
from pathlib import Path
import argparse, hashlib, json, os, runpy, secrets, shlex, sys, datetime

ROOT=Path(__file__).resolve().parents[1]

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--refresh-sql',action='store_true')
    parser.add_argument('--verify',action='store_true')
    args=parser.parse_args()
    # This private helper is deliberately excluded from source/release archives.
    connector=runpy.run_path(str(ROOT/'.local/sms/ecs.py'))['connect']
    from cryptography.hazmat.primitives.ciphers.aead import AESGCM
    folder=ROOT/'.secrets/ecs-v1.12.4-backup';folder.mkdir(exist_ok=True)
    keyfile=folder/'backup.key'
    if not keyfile.exists():
        fd=os.open(keyfile,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
        with os.fdopen(fd,'wb') as stream:stream.write(secrets.token_bytes(32))
    key=keyfile.read_bytes();assert len(key)==32
    cipher=AESGCM(key);objects=folder/'objects';objects.mkdir(exist_ok=True)
    def seal(path,data,aad):
        nonce=secrets.token_bytes(12);temporary=path.with_suffix(path.suffix+'.tmp')
        temporary.write_bytes(b'ZQOSSBK1'+nonce+cipher.encrypt(nonce,data,aad.encode()))
        os.replace(temporary,path)
    def unseal(path,aad):
        raw=path.read_bytes();assert raw[:8]==b'ZQOSSBK1'
        return cipher.decrypt(raw[8:20],raw[20:],aad.encode())
    manifestfile=folder/'manifest.enc'
    if args.verify:
        manifest=json.loads(unseal(manifestfile,'manifest'))
        expected={item['sha256']:item['bytes'] for item in manifest['media_files'].values()}
        assert set(expected)==set(manifest['unique_objects']),'backup_inventory_incomplete'
        for digest,item in manifest['unique_objects'].items():
            data=unseal(objects/(digest+'.enc'),digest)
            assert len(data)==item['bytes']==expected[digest] and hashlib.sha256(data).hexdigest()==digest
        sql=unseal(folder/'database.sql.enc','database.sql')
        assert hashlib.sha256(sql).hexdigest()==manifest['database_sql_sha256']
        receiptfile=ROOT/'.local/oss-deploy/backup-receipt.json'
        if receiptfile.exists():
            receipt=json.loads(receiptfile.read_text());receipt['verified']=True;receipt['verified_at']=datetime.datetime.now(datetime.timezone.utc).timestamp();receiptfile.write_text(json.dumps(receipt,indent=2))
        print(json.dumps({'verified':True,'logical_files':len(manifest['media_files']),'unique_objects':len(manifest['unique_objects']),'database':manifest['database'],'backup_directory':str(folder),'plaintext_backup_files':0}))
        return
    before=json.loads((ROOT/'.local/oss-deploy/before.json').read_text())
    manifest=json.loads(unseal(manifestfile,'manifest')) if manifestfile.exists() else {'format':1,'media_files':before['media_files'],'database':before['database'],'media_root':before['media_root'],'unique_objects':{},'created_at':datetime.datetime.now(datetime.timezone.utc).isoformat()}
    wanted={item['sha256']:item['bytes'] for item in manifest['media_files'].values()}
    names={item['sha256']:name for name,item in manifest['media_files'].items()}
    client=connector();sftp=client.open_sftp();client.get_transport().set_keepalive(15)
    try:
        if not args.refresh_sql:
            local={}
            for base in [ROOT/'server/data/media',ROOT/'厂商信息']:
                if base.exists():
                    for path in base.rglob('*'):
                        if path.is_file() and not path.is_symlink() and path.stat().st_size in set(wanted.values()):
                            with path.open('rb') as stream:digest=hashlib.file_digest(stream,'sha256').hexdigest()
                            if digest in wanted:local[digest]=path
            remote_files=remote_bytes=local_files=0
            for index,(digest,size) in enumerate(wanted.items(),1):
                target=objects/(digest+'.enc')
                if target.exists():data=unseal(target,digest)
                elif digest in local:data=local[digest].read_bytes();local_files+=1
                else:
                    path=manifest['media_root']+'/'+names[digest]
                    assert '..' not in Path(names[digest]).parts and not names[digest].startswith('/')
                    with sftp.open(path,'rb') as source:data=source.read()
                    remote_files+=1;remote_bytes+=len(data)
                assert len(data)==size and hashlib.sha256(data).hexdigest()==digest,'source_checksum_mismatch'
                if not target.exists():seal(target,data,digest)
                manifest['unique_objects'][digest]={'bytes':size}
                if index%25==0 or index==len(wanted):
                    seal(manifestfile,json.dumps(manifest).encode(),'manifest')
                    print(json.dumps({'objects_verified':index,'objects_total':len(wanted),'remote_bytes':remote_bytes,'local_objects_reused':local_files}),flush=True)
            for source,name in [('/etc/zqzt/exhibition.env','environment')]:
                with sftp.open(source,'rb') as stream:seal(folder/(name+'.enc'),stream.read(),name)
        script="""import runpy,subprocess,os,sys
n=runpy.run_path('/opt/zqzt/shared/packages/zqzg-v1.12.3/upgrade.py');cfg=n['config']();db,_=n['database'](cfg)
command=['mysqldump','--protocol=TCP','--host='+cfg.get('DB_HOST','127.0.0.1'),'--port='+cfg.get('DB_PORT','3306'),'--user='+cfg['DB_USER'],'--single-transaction','--hex-blob','--set-gtid-purged=OFF','--no-tablespaces',db]
result=subprocess.run(command,env=dict(os.environ,MYSQL_PWD=cfg.get('DB_PASSWORD','')),stdout=sys.stdout.buffer,stderr=subprocess.PIPE)
if result.returncode:raise RuntimeError('database_dump_failed')
"""
        _,stdout,stderr=client.exec_command('python3 -c '+shlex.quote(script),timeout=120)
        sql=stdout.read();assert stdout.channel.recv_exit_status()==0 and sql.startswith(b'-- MySQL')
        seal(folder/'database.sql.enc',sql,'database.sql');manifest['database_sql_sha256']=hashlib.sha256(sql).hexdigest();manifest['database_sql_bytes']=len(sql);manifest['sql_captured_at']=datetime.datetime.now(datetime.timezone.utc).isoformat();seal(manifestfile,json.dumps(manifest).encode(),'manifest')
        receipt={'backup_directory':str(folder),'database':manifest['database'],'logical_files':len(manifest['media_files']),'unique_objects':len(manifest['unique_objects']),'sql_bytes':len(sql),'sql_sha256':manifest['database_sql_sha256'],'captured_at':datetime.datetime.now(datetime.timezone.utc).timestamp(),'plaintext_backup_files':0,'production_files_changed':False}
        (ROOT/'.local/oss-deploy/backup-receipt.json').write_text(json.dumps(receipt,indent=2))
        print(json.dumps(receipt),flush=True)
    finally:sftp.close();client.close()

if __name__=='__main__':main()
