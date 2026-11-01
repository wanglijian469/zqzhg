"""Package the committed 1.10.6 sources, verified APK and development-demo snapshot."""
from datetime import datetime, timezone
from pathlib import Path
import hashlib
import json
import shutil
import struct
import subprocess
import tarfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
WORK = ROOT / '.local/release-1.10.6'
OUT = ROOT / 'delivery'
STAGE = WORK / 'package-r2/zqzg-v1.10.6-r3'
GO = 'D:/Program Files/Go/bin/go.exe'


def sha(p):
    h = hashlib.sha256()
    with p.open('rb') as f:
        for part in iter(lambda: f.read(1024 * 1024), b''): h.update(part)
    return h.hexdigest()


def inventory(folder):
    return {p.relative_to(folder).as_posix(): sha(p) for p in sorted(folder.rglob('*')) if p.is_file()}


def main():
    subprocess.run(['git', 'diff', '--exit-code', 'HEAD'], cwd=ROOT, check=True, stdout=subprocess.DEVNULL)
    commit = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
    binary = WORK / 'linux-amd64/exhibition'
    header = binary.read_bytes()[:64]
    assert header[:6] == b'\x7fELF\x02\x01' and struct.unpack_from('<H', header, 18)[0] == 62
    metadata = subprocess.check_output([GO, 'version', '-m', str(binary)], text=True)
    for match in ['vcs.revision=' + commit, 'vcs.modified=false', 'GOOS=linux', 'CGO_ENABLED=0']:
        assert match in metadata, 'binary provenance mismatch: ' + match
    assert not STAGE.exists()
    STAGE.mkdir(parents=True)
    (STAGE / 'server').mkdir()
    shutil.copy2(binary, STAGE / 'server/exhibition')
    for name in ['migrations', 'catalog', 'licenses']:
        shutil.copytree(ROOT / 'server' / name, STAGE / 'server' / name)
    for name in ['index.html', 'brand.svg', 'assets', 'pdfjs', 'maps', 'guide-images']:
        source = WORK / 'web' / name; target = STAGE / 'web/dist' / name
        target.parent.mkdir(parents=True, exist_ok=True)
        if source.is_dir(): shutil.copytree(source, target)
        else: shutil.copy2(source, target)
    assert len(list((STAGE / 'web/dist/guide-images').glob('*.png'))) == 9
    android = json.loads((ROOT / 'releases/v1.10.6/manifest.json').read_text(encoding='utf-8'))
    apk = ROOT / 'delivery' / android['asset']
    assert sha(apk) == android['sha256'] and apk.stat().st_size == android['bytes']
    downloads = STAGE / 'web/dist/downloads'; downloads.mkdir()
    shutil.copy2(apk, downloads / 'dalunongji.apk')
    (downloads / 'manifest.json').write_text(json.dumps(android, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    shutil.copytree(ROOT / '.local/map-preview', STAGE / 'map-demo')
    maps = inventory(STAGE / 'map-demo')
    assert len(maps) == 77
    reference = json.loads((WORK / 'inventory.json').read_text())['map_files']
    assert maps == reference, 'development and production snapshots differ'
    shutil.copy2(ROOT / 'deploy/linux/upgrade-guide-1.10.6.py', STAGE / 'upgrade.py')
    shutil.copy2(ROOT / 'releases/v1.10.6/README.md', STAGE / 'README.md')
    manifest = {'version': '1.10.6', 'createdAt': datetime.now(timezone.utc).isoformat(), 'gitCommit': commit, 'sourceTag': 'v1.10.6-r3', 'schemaVersion': 28, 'databaseMigrationAdded': False, 'architecture': 'linux-amd64', 'binarySHA256': sha(binary), 'android': android, 'mapFiles': maps, 'mapUsage': 'development-demonstration', 'mapMissingCodes': ['130571'], 'includesBusinessDatabase': False, 'includesPrivateMedia': False, 'includesCredentials': False}
    (STAGE / 'manifest.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    (STAGE / 'SHA256SUMS').write_bytes(''.join(d + '  ' + n + '\n' for n, d in inventory(STAGE).items()).encode())
    linux = OUT / 'zqzg-v1.10.6-r3-linux-amd64.tar.gz'; source = OUT / 'zqzg-v1.10.6-r3-source.zip'
    assert not linux.exists() and not source.exists()
    def attributes(info):
        info.uid = info.gid = 0; info.uname = info.gname = 'root'
        info.mode = 0o755 if info.isdir() or info.name.endswith(('/server/exhibition', '/upgrade.py')) else 0o644
        return info
    with tarfile.open(linux, 'w:gz', compresslevel=6) as out: out.add(STAGE, arcname='zqzg-v1.10.6-r3', filter=attributes)
    # Keep complete build sources and text records. Historical documentation
    # screenshots remain controlled in GitHub; runtime guide images are included.
    proc = subprocess.Popen(['git', 'archive', '--format=tar', commit], cwd=ROOT, stdout=subprocess.PIPE)
    with tarfile.open(fileobj=proc.stdout, mode='r|') as tree, zipfile.ZipFile(source, 'x', compression=zipfile.ZIP_DEFLATED, compresslevel=6) as archive:
        for member in tree:
            if not member.isfile(): continue
            name = member.name
            assert not name.startswith('/') and '..' not in Path(name).parts
            if name.startswith('docs/') and Path(name).suffix.lower() in ['.png', '.jpg', '.jpeg', '.webp', '.gif']: continue
            archive.writestr('zqzg-v1.10.6-r3-source/' + name, tree.extractfile(member).read())
    assert proc.wait() == 0
    with zipfile.ZipFile(source) as archive:
        assert archive.testzip() is None
        forbidden = [n for n in archive.namelist() if '/.local/' in n or n.endswith(('.jks', '.keystore', '/key.properties', '/local.properties', '/.env', '/39.97.250.244.txt'))]
        assert not forbidden, 'private file in source archive'
    public_manifest = OUT / 'manifest-v1.10.6-r3.json'; shutil.copy2(STAGE / 'manifest.json', public_manifest)
    assets = [linux, source, apk, public_manifest]
    sums = OUT / 'SHA256SUMS-v1.10.6-r3.txt'
    sums.write_bytes(''.join(sha(p) + '  ' + p.name + '\n' for p in assets).encode())
    assets.append(sums)
    receipt = {'tag': 'v1.10.6-r3', 'commit': commit, 'name': '中企展馆网站与安卓 1.10.6+44', 'body': '新增厂商入驻指南、紧凑公共页脚与独立展厅页脚、中英文装饰显示。同步原签名 Android 1.10.6+44，包含九张指南配图；Schema 28 不变。Linux 包含与开发一致的77项演示地图快照，保留来源说明及130571缺失轮廓处理。\n\n不含业务数据库、私有媒体、SSH凭据或签名密钥。Android真机及iOS未验收。完整历史测试失败与针对本次改动的通过项单独记录，见源码发布说明。安装前核对SHA256并备份。', 'assets': [{'path': str(p.relative_to(ROOT)).replace('\\', '/'), 'name': p.name, 'sha256': sha(p), 'size': p.stat().st_size} for p in assets]}
    (WORK / 'release-assets.json').write_text(json.dumps(receipt, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    print(json.dumps({'commit': commit, 'files': len(inventory(STAGE)), 'mapFiles': len(maps), 'assets': [{k: v for k, v in a.items() if k != 'path'} for a in receipt['assets']]}, ensure_ascii=False))


if __name__ == '__main__': main()
