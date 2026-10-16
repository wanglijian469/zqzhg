"""Regression test the installer password against a separate MySQL with MEDIUM policy.
Does not change the existing application's MySQL instance or its global policy.
"""
from pathlib import Path
from datetime import datetime
import json
import os
import re
import socket
import subprocess
import time

ROOT = Path(__file__).resolve().parents[1]
MYSQL = Path('D:/InforGuardMC/InforGuardMC/mysql')
BASH = 'D:/Program Files/Git/bin/bash.exe'
script = (ROOT / 'deploy/linux/deploy.sh').read_text(encoding='utf-8')
function = re.search(r'generate_app_password\(\) \{.*?\n\}', script, re.S).group(0)
result = subprocess.run([BASH, '-c', function + '\nfor ((i=0;i<64;i++)); do generate_app_password; printf "\\n"; done'],
                        capture_output=True, text=True, check=True)
passwords = result.stdout.splitlines()
assert len(set(passwords)) == 64
for value in passwords:
    assert len(value) == 60 and all(re.search(pattern, value) for pattern in ['[A-Z]', '[a-z]', '[0-9]', '[@%+]'])
    assert re.fullmatch(r'[A-Za-z0-9@%+_=!?:.,~-]{12,128}', value)
subprocess.run([BASH, '-n', str(ROOT / 'deploy/linux/deploy.sh')], check=True)
with socket.socket() as probe:
    probe.bind(('127.0.0.1', 0))
    port = probe.getsockname()[1]
work = ROOT / '.local' / ('mysql-policy-test-' + datetime.now().strftime('%Y%m%d%H%M%S'))
data = work / 'data'
data.mkdir(parents=True)
command = [str(MYSQL / 'bin/mysqld.exe'), '--no-defaults', '--basedir=' + str(MYSQL), '--datadir=' + str(data)]
with (work / 'initialize.log').open('wb') as log:
    subprocess.run([*command, '--initialize-insecure', '--console'], stdout=log, stderr=log, check=True)
client = [str(MYSQL / 'bin/mysql.exe'), '--no-defaults', '--protocol=tcp', '--host=127.0.0.1',
          '--port=' + str(port), '--user=root', '--batch', '--skip-column-names']
process = None

def query(sql):
    return subprocess.run(client, input=sql, capture_output=True, text=True)

try:
    with (work / 'server.log').open('wb') as log:
        process = subprocess.Popen([*command, '--console', '--bind-address=127.0.0.1', '--port=' + str(port),
                                    '--mysqlx=OFF', '--plugin-load-add=validate_password.dll',
                                    '--validate-password-policy=MEDIUM', '--validate-password-length=12'],
                                   stdout=log, stderr=log, creationflags=subprocess.CREATE_NO_WINDOW)
        for attempt in range(120):
            if process.poll() is not None:
                raise RuntimeError('Isolated MySQL exited; inspect ' + str(work / 'server.log'))
            if query('SELECT 1').returncode == 0:
                break
            time.sleep(.25)
        else:
            raise RuntimeError('Isolated MySQL startup timeout')
        rejected = query("CREATE USER 'legacy_hex'@'localhost' IDENTIFIED BY '" + os.urandom(24).hex() + "'")
        assert rejected.returncode != 0 and '1819' in rejected.stderr
        for index, value in enumerate(passwords[:8]):
            accepted = query(f"CREATE USER 'new_{index}'@'localhost' IDENTIFIED BY '{value}'")
            assert accepted.returncode == 0, 'Generated password rejected by isolated MEDIUM policy'
        # Do not store or test the user's real password. This synthetic password
        # exercises the same 12+ length / mixed case / number / @ character classes.
        accepted = query("CREATE USER 'manual_example'@'localhost' IDENTIFIED BY 'Sample@9081726'")
        assert accepted.returncode == 0
        # CREATE USER comes before CREATE DATABASE: policy rejection leaves no DB.
        assert script.index("CREATE USER '$APP_DB_USER'@'localhost'") < script.index('CREATE DATABASE \\`$DB_NAME\\`')
        report = {'isolatedMySQL': True, 'policy': 'MEDIUM', 'minimumLength': 12,
                  'legacyHexPassword': 'rejected with 1819', 'generatedPasswordSamples': 64,
                  'generatedPasswordsAcceptedByMySQL': 8, 'manualPasswordExample': 'accepted',
                  'passwordRejectionPrecedesDatabaseCreation': True, 'existingMySQLPolicyChanged': False,
                  'userPasswordIncluded': False, 'bashSyntax': 'passed'}
        (ROOT / 'docs/test-output/deploy-password-policy.json').write_text(json.dumps(report, indent=2), encoding='utf-8')
        print(json.dumps(report, indent=2))
finally:
    if process is not None:
        subprocess.run([str(MYSQL / 'bin/mysqladmin.exe'), '--no-defaults', '--protocol=tcp',
                        '--host=127.0.0.1', '--port=' + str(port), '--user=root', 'shutdown'], capture_output=True)
        try:
            process.wait(timeout=15)
        except subprocess.TimeoutExpired:
            process.terminate()
            process.wait(timeout=15)
