"""Build an additive, offline Python3.6 fix; retain all original delivery files."""
from pathlib import Path
import hashlib
import json
import subprocess
import sys
import tarfile

ROOT = Path(__file__).resolve().parents[1]
NAME = 'zqzg-legacy-upgrade-fix2-python36'
STAGE = ROOT / '.local' / NAME
OUT = ROOT / 'delivery' / (NAME + '.tar.gz')
PY36 = ROOT / '.local/python36/python.exe'


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    if OUT.exists() or STAGE.exists():
        raise RuntimeError('Existing deliveries are retained; choose a new fix identifier')
    STAGE.mkdir(parents=True)
    for script, log in [('scripts/test-aliyun-upgrade.py', 'upgrade-tests'),
                        ('scripts/test-legacy-key-init.py', 'key-tests'),
                        ('scripts/test-local-data-import.py', 'import-tests')]:
        result = subprocess.run([str(PY36), str(ROOT / script)], cwd=ROOT, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        if result.returncode:
            raise RuntimeError('Python3.6 test failed: ' + script)
        (STAGE / ('python36-' + log + '.txt')).write_bytes(result.stdout + result.stderr)
    result = subprocess.run([str(PY36), str(ROOT / 'scripts/test-local-data-import.py'), '--restore'], cwd=ROOT,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if result.returncode:
        raise RuntimeError('Python3.6 isolated restore failed')
    (STAGE / 'python36-restore.txt').write_bytes(result.stdout + result.stderr)
    files = {
        'aliyun-upgrade-support-fix2.py': (ROOT / 'deploy/linux/aliyun-upgrade-support.py').read_text(encoding='utf-8'),
        'import-local-data-fix2.py': (ROOT / 'deploy/linux/import-local-data.py').read_text(encoding='utf-8').replace(
            "bundle / 'aliyun-upgrade-support.py'", "bundle / 'aliyun-upgrade-support-fix2.py'"),
        'upgrade-fix2.sh': (ROOT / 'deploy/linux/upgrade-storage.sh').read_text(encoding='utf-8').replace(
            'aliyun-upgrade-support.py', 'aliyun-upgrade-support-fix2.py').replace(
            'sha256sum --check --quiet SHA256SUMS)',
            'sha256sum --check --quiet SHA256SUMS && sha256sum --check --quiet legacy-python36-SHA256SUMS)')
    }
    if "bundle / 'aliyun-upgrade-support-fix2.py'" not in files['import-local-data-fix2.py']:
        raise RuntimeError('Importer did not select the corrected helper')
    if 'legacy-python36-SHA256SUMS)' not in files['upgrade-fix2.sh']:
        raise RuntimeError('Original and supplemental checks must both be retained')
    for name, text in files.items():
        (STAGE / name).write_text(text, encoding='utf-8', newline='\n')
        if 'text=True' in text or 'capture_output=' in text:
            raise RuntimeError('Modern subprocess arguments remain')
    (STAGE / 'legacy-python36-README.md').write_text('''# Python 3.6 离线升级修复 fix2

本补丁仅新增独立脚本及校验清单，不覆盖原包文件、不改密钥、不删除失败批次备份。
适用于已解压的 zqzg-aliyun-20261019-with-local-data-v1，且已完成运维目录及密钥初始化。

上传本补丁到 /opt，在完整包目录解压并校验 legacy-python36-SHA256SUMS 后执行：

```bash
bash upgrade-fix2.sh --check && bash upgrade-fix2.sh --apply && \\
python3 import-local-data-fix2.py --check && \\
python3 import-local-data-fix2.py --apply --confirm-use-local-data
```

使用已生成的密钥，不需再次初始化。只有升级程序成功后才导入数据。
MySQL管理员密码仅在交互提示时输入；原库、媒体及备份保留。
原完整包SHA256检查和本补丁SHA256检查同时执行，不取消完整性校验。
备份阶段失败会重启未切换的原服务。切换后失败按原部署手册诊断，不直接启动不兼容旧后端。

已在Windows官方Python3.6.8运行29项测试，并在隔离MySQL验证当前快照恢复、密码哈希及357个媒体校验、9家企业45款产品和公开页面。
Linux ECS/systemd真实执行仍待服务器验收。本补丁不包含业务数据、密钥或账号凭据。
''', encoding='utf-8', newline='\n')
    report = json.loads((ROOT / '.local/private-release/zqzg-aliyun-20261019-with-local-data-v1/verification/local-data-validation.json').read_text(encoding='utf-8'))
    report.update({'python': '3.6.8 actual Windows runtime', 'unitTests': 29,
                   'programAndDataRebuilt': False, 'regeneratesKey': False,
                   'sourceSHA256': {p: sha(ROOT / p) for p in ['deploy/linux/aliyun-upgrade-support.py', 'deploy/linux/import-local-data.py']}})
    (STAGE / 'legacy-python36-verification.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    (STAGE / 'legacy-python36-SHA256SUMS').write_text(''.join(sha(p) + '  ' + p.name + '\n' for p in sorted(STAGE.iterdir())), encoding='utf-8', newline='\n')
    def attrs(info):
        info.uid = info.gid = 0
        info.uname = info.gname = 'root'
        info.mode = 0o644
        return info
    with tarfile.open(OUT, 'w:gz', compresslevel=6) as archive:
        for file in sorted(STAGE.iterdir()):
            archive.add(file, arcname=file.name, filter=attrs)
    with tarfile.open(OUT, 'r:gz') as check:
        for line in check.extractfile('legacy-python36-SHA256SUMS').read().decode().splitlines():
            want, name = line.split('  ', 1)
            if hashlib.sha256(check.extractfile(name).read()).hexdigest() != want:
                raise RuntimeError('Archive readback mismatch')
    OUT.with_name(OUT.name + '.sha256').write_text(sha(OUT) + '  ' + OUT.name + '\n', encoding='utf-8', newline='\n')
    print(json.dumps({'file': str(OUT), 'size': OUT.stat().st_size, 'sha256': sha(OUT)}, indent=2))


if __name__ == '__main__':
    main()
