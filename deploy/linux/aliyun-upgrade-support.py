#!/usr/bin/env python3
"""Read existing ECS configuration and make a private pre-upgrade backup. No restore/import."""
import argparse, base64, hashlib, json, os, re, shutil, subprocess, sys, tarfile
from pathlib import Path
from urllib.parse import urlparse

def read_env(file):
    values = {}
    for line in Path(file).read_text(encoding='utf-8-sig').splitlines():
        line = line.strip()
        if not line or line.startswith('#'): continue
        key, sep, value = line.partition('=')
        if sep and re.fullmatch(r'[A-Z][A-Z0-9_]*',key.strip()): values[key.strip()] = value.strip().strip('"\'')
    return values

def contains(parent, child):
    try: return os.path.commonpath([str(parent),str(child)]) == str(parent)
    except ValueError: return False

def context(env_file, install):
    cfg = read_env(env_file)
    def absolute(value, label):
        if not value or not Path(value).is_absolute(): raise ValueError(label+' must be an absolute server path; preserve its current actual location')
        return Path(value).resolve()
    media = absolute(cfg.get('MEDIA_DIR',''),'MEDIA_DIR')
    operations = absolute(cfg.get('OPERATIONS_DIR') or str(media.parent/('.operations-'+media.name)),'OPERATIONS_DIR')
    database = cfg.get('DB_NAME','zqzt_exhibition')
    pointer = operations/'active.json'
    if pointer.exists():
        state = json.loads(pointer.read_text(encoding='utf-8-sig'))
        if state.get('pending'): raise ValueError('A restore/cutover is pending. Finish it before upgrading.')
        current = state.get('current') or {}
        if current.get('database'): database = current['database']
        if current.get('media_dir'): media = absolute(current['media_dir'],'active media_dir')
    if not re.fullmatch(r'[A-Za-z0-9_]+',database): raise ValueError('Unexpected effective database name')
    web = absolute(cfg.get('WEB_DIR',str(Path(install)/'current/web/dist')),'WEB_DIR')
    release = (Path(install)/'current').resolve()
    for location in [media,operations]:
        if location == Path(location.anchor) or contains(release,location) or contains(web,location): raise ValueError('Persistent storage must be outside the program and web directories')
    if not media.is_dir(): raise ValueError('Effective media directory does not exist')
    if not operations.is_dir(): raise ValueError('Create and permission the stable OPERATIONS_DIR before upgrade; do not discard existing control state')
    key = cfg.get('CONFIG_ENCRYPTION_KEY','')
    try: key_bytes = bytes.fromhex(key) if re.fullmatch(r'[A-Fa-f0-9]{64}',key) else base64.b64decode(key,validate=True)
    except ValueError:
        try: key_bytes = base64.b64decode(key,validate=True)
        except ValueError: key_bytes = b''
    if len(key_bytes)!=32: raise ValueError('Configure and separately back up the existing CONFIG_ENCRYPTION_KEY (32 bytes). Never replace a key used by existing encrypted data.')
    addr = cfg.get('HTTP_ADDR','0.0.0.0:8089')
    try: port = int(addr.rsplit(':',1)[1])
    except (ValueError,IndexError): raise ValueError('Cannot read HTTP_ADDR port')
    if not 1<=port<=65535: raise ValueError('Invalid HTTP_ADDR port')
    origin = cfg.get('PUBLIC_BASE_URL') or cfg.get('APP_DOWNLOAD_BASE_URL')
    if not origin: raise ValueError('Set PUBLIC_BASE_URL to the current site origin before upgrade; do not change domains during this operation')
    parsed = urlparse(origin)
    if parsed.scheme not in ['http','https'] or not parsed.netloc or parsed.path not in ['', '/'] or parsed.username or parsed.query or parsed.fragment: raise ValueError('Invalid public origin')
    if not cfg.get('PUBLIC_BASE_URL'): raise ValueError('Set PUBLIC_BASE_URL explicitly to the current site origin before upgrade (keep APP_DOWNLOAD_BASE_URL unchanged)')
    return cfg, {'database':database,'media':str(media),'operations':str(operations),'web':str(web),'port':port,'public_url':origin,'release':str(release)}

def tool(cfg,name):
    folder=cfg.get('MYSQL_BIN','')
    result=str(Path(folder)/name) if folder else shutil.which(name)
    if not result or not Path(result).is_file(): raise ValueError(name+' not found; set MYSQL_BIN or PATH')
    return result

def db_args(cfg):
    return ['--no-defaults','--host='+cfg.get('DB_HOST','127.0.0.1'),'--port='+cfg.get('DB_PORT','3306'),'--user='+cfg.get('DB_USER','root')]

def backup(env_file, install, directory):
    cfg,info=context(env_file,install)
    directory=Path(directory).resolve()
    for parent in [Path(info['media']),Path(info['operations']),Path(info['web']),Path(info['release'])]:
        if contains(parent,directory): raise ValueError('Backup directory must be outside media, operations and releases')
    if directory==Path(directory.anchor) or directory.exists(): raise ValueError('Backup destination must be a new, private directory')
    directory.parent.mkdir(parents=True,exist_ok=True)
    estimate=sum(p.stat().st_size for root in [Path(info['media']),Path(info['operations'])] if root.exists() for p in root.rglob('*') if p.is_file() and not p.is_symlink())
    if shutil.disk_usage(str(directory.parent)).free < estimate+(1<<30): raise ValueError('Insufficient backup disk space; choose another protected disk')
    os.mkdir(directory,0o700)
    shutil.copyfile(env_file,directory/'exhibition.env');os.chmod(directory/'exhibition.env',0o600)
    dump=tool(cfg,'mysqldump');client=tool(cfg,'mysql');env={**os.environ,'MYSQL_PWD':cfg.get('DB_PASSWORD','')}
    common=db_args(cfg)
    schema=subprocess.run([client,*common,'--batch','--skip-column-names',info['database'],'--execute=SELECT COALESCE(MAX(version),0) FROM schema_migrations'],env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,universal_newlines=True)
    if schema.returncode or not schema.stdout.strip().isdigit(): raise ValueError('Cannot read existing schema version; current service can be restarted unchanged')
    if int(schema.stdout.strip())>15: raise ValueError('Database is newer than this build; refusing downgrade')
    help_text=subprocess.run([dump,'--no-defaults','--help'],stdout=subprocess.PIPE,stderr=subprocess.PIPE,universal_newlines=True).stdout
    flags=['--single-transaction','--skip-lock-tables','--hex-blob','--default-character-set=utf8mb4']
    for flag,feature in [('--no-tablespaces','no-tablespaces'),('--column-statistics=0','column-statistics'),('--set-gtid-purged=OFF','set-gtid-purged')]:
        if feature in help_text: flags.append(flag)
    with (directory/'database.sql').open('xb') as output:
        os.chmod(directory/'database.sql',0o600)
        result=subprocess.run([dump,*common,*flags,info['database']],env=env,stdout=output,stderr=subprocess.PIPE)
    if result.returncode or (directory/'database.sql').stat().st_size==0: raise ValueError('Database backup failed; no migration or program switch was performed')
    for label,source in [('media',info['media']),('operations',info['operations'])]:
        source=Path(source)
        if source.exists():
            target=directory/(label+'.tar.gz')
            with target.open('xb') as stream:
                os.chmod(target,0o600)
                with tarfile.open(fileobj=stream,mode='w:gz') as archive: archive.add(str(source),arcname=label)
    checks=[]
    for file in sorted(directory.iterdir()):
        h=hashlib.sha256()
        with file.open('rb') as stream:
            for chunk in iter(lambda:stream.read(1024*1024),b''):h.update(chunk)
        checks.append(h.hexdigest()+'  '+file.name)
    (directory/'SHA256SUMS').write_text('\n'.join(checks)+'\n',encoding='utf-8')
    info['previous_schema']=int(schema.stdout.strip());info['backup']=str(directory)
    (directory/'context.json').write_text(json.dumps(info,ensure_ascii=False,indent=2)+'\n',encoding='utf-8')
    os.chmod(directory/'context.json',0o600)
    return info

def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('mode',choices=['inspect','backup']);parser.add_argument('--env-file',required=True);parser.add_argument('--install-dir',required=True);parser.add_argument('--output')
    args=parser.parse_args()
    if args.mode=='backup':
        if not args.output: parser.error('--output is required')
        info=backup(args.env_file,args.install_dir,args.output)
    else: _,info=context(args.env_file,args.install_dir)
    print(json.dumps(info,ensure_ascii=False))
if __name__=='__main__':
    try: main()
    except (ValueError,OSError,subprocess.SubprocessError) as error:
        print('Preflight/backup failed: '+str(error),file=sys.stderr);sys.exit(1)
