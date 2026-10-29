"""On-host integrity receipts; never export account contents or credentials."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys

def config():
    cfg = {k.strip(): v.strip().strip('"') for line in Path('/etc/zqzt/exhibition.env').read_text(encoding='utf-8-sig').splitlines() if '=' in line and not line.lstrip().startswith('#') for k, v in [line.split('=', 1)]}
    media = Path(cfg['MEDIA_DIR'])
    control = Path(cfg.get('OPERATIONS_DIR', str(media.parent / ('.operations-' + media.name))))
    target = json.loads((control / 'active.json').read_text()).get('current', {})
    return cfg, target.get('database') or cfg['DB_NAME']

cfg, database = config()
def query(sql):
    r = subprocess.run(['mysql', '--host=' + cfg.get('DB_HOST', '127.0.0.1'), '--port=' + cfg.get('DB_PORT', '3306'), '--user=' + cfg['DB_USER'], '--batch', '--skip-column-names', database, '-e', sql], env=dict(os.environ, MYSQL_PWD=cfg.get('DB_PASSWORD', '')), stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if r.returncode:
        raise RuntimeError('audit_query_failed')
    return r.stdout.decode().strip()

if sys.argv[1] == 'locks':
    print(query("SELECT ID,COMMAND,TIME,STATE,REGEXP_SUBSTR(INFO,'^[A-Za-z _`]+') FROM information_schema.processlist WHERE USER=SUBSTRING_INDEX(CURRENT_USER(),'@',1) AND INFO IS NOT NULL"))
elif sys.argv[1] == 'error':
    import re
    text = Path('/opt/zqzt/shared/upgrade-backups/sms-last-operation-error.log').read_text()
    print(json.dumps({'mysql_error_numbers': re.findall(r'Error (\d+) ', text), 'sources': re.findall(r'source=([a-z_.]+:\d+)', text), 'operation_codes': re.findall(r'public demo import failed: ([a-z_]+)', text), 'sql_operations': re.findall(r'(?:INSERT INTO|UPDATE|CREATE TABLE|ALTER TABLE) [`]?([a-z_]+)', text)}))
elif sys.argv[1] == 'registration':
    print(json.dumps({'recipient':'***8982','account_summary':query("SELECT COUNT(DISTINCT a.id),MAX(a.role),MAX(a.phone_verified_at IS NOT NULL),MAX(a.password_hash<>''),COUNT(DISTINCT s.token_hash) FROM accounts a LEFT JOIN sessions s ON s.account_id=a.id AND s.expires_at>UTC_TIMESTAMP() WHERE RIGHT(a.phone,4)='8982'"), 'verification':query("SELECT v.purpose,v.used,a.provider_code,a.request_id,a.biz_id FROM phone_verifications v JOIN sms_send_attempts a ON a.id=v.send_id WHERE RIGHT(v.phone,4)='8982'")}))
elif sys.argv[1] == 'receipt':
    print(json.dumps({'counts_accounts_contents_media_schema': query('SELECT (SELECT COUNT(*) FROM accounts),(SELECT COUNT(*) FROM contents),(SELECT COUNT(*) FROM media),(SELECT MAX(version) FROM schema_migrations)').split('\t'), 'published_counts': query("SELECT kind,COUNT(*) FROM contents WHERE visible=1 AND published_revision_id>0 GROUP BY kind"), 'sms_attempts': query("SELECT state,provider_code,provider_message,request_id,biz_id,COUNT(*) FROM sms_send_attempts GROUP BY state,provider_code,provider_message,request_id,biz_id"), 'sms_verification': query("SELECT purpose,used,attempts,expires_at<UTC_TIMESTAMP() AS expired,COUNT(*) FROM phone_verifications WHERE send_id<>'' GROUP BY purpose,used,attempts,expired"), 'ordinary_verified_users': query("SELECT COUNT(*) FROM accounts WHERE role='visitor' AND phone_verified_at IS NOT NULL AND password_hash<>''")}))
