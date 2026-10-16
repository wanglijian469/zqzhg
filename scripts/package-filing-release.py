"""Package the filing-icon Android update and public filing documents."""
from pathlib import Path
from datetime import datetime, timezone
import hashlib
import json
import shutil
import tarfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
D = ROOT / 'delivery'
VERSION = '1.5.14'
ANDROID = VERSION + '+22'
FILING = D / ('filing-v' + VERSION)
STAGE = ROOT / '.local' / ('filing-release-' + VERSION) / ('dalunongji-android-v' + VERSION)
if STAGE.exists():
    raise RuntimeError('Release stage already exists')
STAGE.mkdir(parents=True)
apk = D / 'android' / ('dalunongji-' + ANDROID + '.apk')
shutil.copyfile(apk, STAGE / apk.name)
shutil.copytree(FILING, STAGE / 'filing')
shutil.copyfile(ROOT / 'deploy/linux/update-android.sh', STAGE / 'update-android.sh')
for name in ['filing-apk-certificate.txt', 'filing-apk-metadata.txt', 'filing-artifact-validation.json', 'filing-adaptive-icon.txt']:
    (STAGE / 'verification').mkdir(exist_ok=True)
    shutil.copyfile(ROOT / 'docs/test-output' / name, STAGE / 'verification' / name)
data = json.loads((FILING / 'filing-info.json').read_text(encoding='utf-8'))
readme = '''# 大陆农机 Android 1.5.14+22：备案图标更新

使用用户提供的绿色农机备案图标；包名 com.zqzt.dalunongji、原签名和云端地址 http://39.97.250.244:8089 保留。网站及后端仍为 1.5.13。

## 手机安装

安装 dalunongji-1.5.14+22.apk，直接覆盖旧版，不要卸载。原登录、语言、服务地址、本机草稿按原应用逻辑保留。系统启动器有图标缓存时，可先重启手机或等待启动器刷新，勿为刷新图标而清空应用数据。本次没有连接的 Android 实机或模拟器，覆盖安装和桌面图标实机显示尚未验证。

## 备案资料

filing/备案填写信息.md 包含三项备案字段及复制说明；公钥十进制模数见 filing/public-key-modulus-decimal.txt。证书 MD5 见 filing/certificate-md5.txt。该 MD5 属于签名证书。签名证书和公钥可公开，不含私钥、密钥库或密码。filing/filing-icon-original.png 与用户附件逐字节一致。APK 内原始图标和五种缩放图标经过像素核对。

## 更新阿里云手机下载安装链接

将 dalunongji-android-v1.5.14-update.tar.gz 上传到服务器 /opt，在 root 终端执行：

```bash
cd /opt && tar -xzf dalunongji-android-v1.5.14-update.tar.gz && bash /opt/dalunongji-android-v1.5.14/update-android.sh
```

脚本核对全包 SHA-256，将当前下载 APK 备份到 /opt/zqzt/shared/apk-archive，再原子替换当前程序目录的 web/dist/downloads/dalunongji.apk。不会重新导入数据库，也不需重启服务。服务器下载地址保持为 http://39.97.250.244:8089/downloads/dalunongji.apk。本次仅生成更新包，尚未替用户更新 ECS 上的 APK。

旧的 Linux 完整部署归档仍含当时版本的 APK；若未来从旧归档重新部署，需再次执行本更新。历史部署包和 APK 均保留。
'''
(STAGE / 'README.md').write_text(readme, encoding='utf-8')
shutil.copyfile(STAGE / 'README.md', D / 'ANDROID_FILING_RELEASE.md')

def sha(file):
    with file.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()

def checksums(folder):
    files = sorted(file for file in folder.rglob('*') if file.is_file() and file.name != 'SHA256SUMS')
    (folder / 'SHA256SUMS').write_text(''.join(f'{sha(file)}  {file.relative_to(folder).as_posix()}\n' for file in files), encoding='utf-8', newline='\n')

checksums(FILING)
checksums(STAGE)
filing_zip = D / 'dalunongji-filing-v1.5.14.zip'
android_zip = D / 'dalunongji-android-v1.5.14-deploy.zip'
update_tar = D / 'dalunongji-android-v1.5.14-update.tar.gz'
source_zip = D / 'zqzt-exhibition-v1.5.14-source.zip'
for folder, output in [(FILING, filing_zip), (STAGE, android_zip)]:
    with zipfile.ZipFile(output, 'x', compression=zipfile.ZIP_DEFLATED, compresslevel=6) as archive:
        for file in sorted(folder.rglob('*')):
            if file.is_file():
                archive.write(file, folder.name + '/' + file.relative_to(folder).as_posix())

def attributes(info):
    info.uid = info.gid = 0
    info.uname = info.gname = 'root'
    info.mode = 0o755 if info.isdir() or info.name.endswith('/update-android.sh') else 0o644
    return info

if update_tar.exists():
    raise RuntimeError('Update archive already exists')
with tarfile.open(update_tar, 'w:gz', compresslevel=6) as archive:
    archive.add(STAGE, arcname=STAGE.name, filter=attributes)

updates = ['mobile/pubspec.yaml', 'mobile/assets/branding/app-icon.svg', 'mobile/assets/branding/filing-icon-original.png',
           'scripts/generate-android-icons.mjs', 'scripts/extract-filing-info.py', 'scripts/package-filing-release.py',
           'deploy/linux/update-android.sh']
updates += [file.relative_to(ROOT).as_posix() for file in (ROOT / 'mobile/android/app/src/main/res').rglob('*') if file.is_file() and 'ic_launcher' in file.name]
updates.append('mobile/android/app/src/main/res/drawable/ic_zqzt.xml')
with zipfile.ZipFile(D / 'zqzt-exhibition-v1.5.13-fix1-source.zip') as original, zipfile.ZipFile(source_zip, 'x', compression=zipfile.ZIP_DEFLATED, compresslevel=6) as archive:
    for item in original.infolist():
        if item.filename not in updates:
            archive.writestr(item.filename, original.read(item))
    for name in sorted(set(updates)):
        archive.write(ROOT / name, name)

release = {'version': VERSION, 'androidVersion': ANDROID, 'websiteVersion': '1.5.13', 'backendVersion': '1.5.13',
           'applicationId': data['packageName'], 'apiBaseUrl': data['apiBaseUrl'],
           'signingCertificateSHA256': data['certificateSHA256'], 'signingCertificateMD5': data['certificateMD5'],
           'apkSHA256': sha(apk), 'filingIconSHA256': data['iconSHA256'], 'createdAt': datetime.now(timezone.utc).isoformat(),
           'androidDeviceInstallTested': False, 'remoteAPKUpdated': False, 'filingInfoDirectory': FILING.name}
outputs = [apk, filing_zip, android_zip, update_tar, source_zip]
release['files'] = [{'file': file.relative_to(D).as_posix(), 'bytes': file.stat().st_size, 'sha256': sha(file)} for file in outputs]
(D / 'filing-release-manifest.json').write_text(json.dumps(release, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
(D / 'SHA256SUMS-filing-v1.5.14.txt').write_text(''.join(f'{sha(file)}  {file.relative_to(D).as_posix()}\n' for file in outputs), encoding='utf-8', newline='\n')
previous = D / 'manifest.json'
snapshot = D / 'manifest-v1.5.13.json'
if not snapshot.exists():
    shutil.copyfile(previous, snapshot)
manifest = json.loads(previous.read_text(encoding='utf-8'))
old_files = manifest['files']
manifest.update(release)
manifest['files'] = old_files + release['files']
previous.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
print(json.dumps(release, ensure_ascii=False, indent=2))
