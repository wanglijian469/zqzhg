"""Package a validated acceptance APK and matching server/source without private state."""
import hashlib,json,re,shutil,stat,struct,zipfile
from pathlib import Path
from datetime import datetime,timezone
ROOT=Path(__file__).resolve().parents[1]
def sha(path):
 h=hashlib.sha256()
 with path.open('rb') as stream:
  for chunk in iter(lambda:stream.read(1024*1024),b''):h.update(chunk)
 return h.hexdigest()
def text(path):return (ROOT/path).read_text(encoding='utf-8-sig')
def copy(source,target):
 target.parent.mkdir(parents=True,exist_ok=True)
 if target.exists():target.chmod(target.stat().st_mode|stat.S_IWRITE)
 shutil.copyfile(source,target)
def source_files():
 excluded={'.git','.local','.dart_tool','.gradle','node_modules','build','dist','__pycache__','.idea','.vscode','data','downloads'}
 secret_names={'.env','key.properties','local.properties','signing.json','.flutter-plugins-dependencies','Generated.xcconfig','flutter_export_environment.sh','GeneratedPluginRegistrant.java'}
 for directory in ['mobile','web','server','scripts','deploy']:
  for path in (ROOT/directory).rglob('*'):
   if not path.is_file() or any(part in excluded for part in path.relative_to(ROOT).parts) or path.name in secret_names:continue
   if path.suffix.lower() in {'.exe','.apk','.jks','.keystore','.p12','.pfx','.pem','.key','.log','.dill','.iml','.tsbuildinfo','.pyc','.zqbak','.db','.sqlite'}:continue
   if path.suffix=='.sql' and path.parent!=ROOT/'server/migrations':continue
   yield path
 for name in ['README.md','LICENSE','.gitignore','.gitattributes']:yield ROOT/name
 for path in (ROOT/'docs').glob('*.md'):yield path
 yield ROOT/'docs/openapi.json'
 for path in (ROOT/'docs').glob('*openapi*.yaml'):yield path
 for path in (ROOT/'releases/v1.8.0-rc.1').glob('*.md'):yield path
def zip_tree(source,target):
 with zipfile.ZipFile(target,'w',zipfile.ZIP_DEFLATED,compresslevel=6) as archive:
  for path in sorted(source.rglob('*')):
   if path.is_file():archive.write(path,path.relative_to(source).as_posix())
def main():
 version=re.search(r'^version:\s*(\S+)',text('mobile/pubspec.yaml'),re.M).group(1)
 receipt=json.loads(text('.local/android-preview-verification.json'))
 apk=ROOT/'delivery/android-preview'/('dalunongji-'+version+'.apk')
 if receipt['version']!=version or receipt['sha256']!=sha(apk) or not receipt['signingMatchesPrevious']:raise RuntimeError('APK receipt mismatch')
 metadata=text('docs/test-output/mobile18-apk-metadata.txt');version_name,version_code=version.rsplit('+',1)
 if f"versionCode='{version_code}'" not in metadata or f"versionName='{version_name}'" not in metadata:raise RuntimeError('Acceptance version metadata mismatch')
 if "name='com.zqzt.dalunongji'" not in metadata:raise RuntimeError('Package identity changed')
 signature=text('docs/test-output/mobile18-apk-signature.txt')
 if 'Verifies' not in signature or receipt['certificateSHA256'] not in signature:raise RuntimeError('APK signature output mismatch')
 with zipfile.ZipFile(apk) as archive:
  if any('map-preview' in name or '/maps/' in name for name in archive.namelist()):raise RuntimeError('Unexpected geographic resources in APK')
 for file in ['delivery/android/dalunongji.apk','web/public/downloads/dalunongji.apk','web/dist/downloads/dalunongji.apk','delivery/web/dist/downloads/dalunongji.apk']:
  if sha(ROOT/file)!=receipt['stableSHA256']:raise RuntimeError('Stable APK changed: '+file)
 flutter=text('docs/test-output/mobile18-flutter-tests.txt');analyze=text('docs/test-output/mobile18-flutter-analyze.txt');web=text('docs/test-output/mobile18-web-tests.txt')
 if 'All tests passed!' not in flutter or 'No issues found!' not in analyze or 'failed' in web.lower():raise RuntimeError('Client validation incomplete')
 for file in ['mobile18-backend-tests.txt','mobile18-native-integration.txt','mobile18-web-browser.txt','mobile18-web-editors-browser.txt']:
  result=text('docs/test-output/'+file)
  if not re.search(r'^ok\s',result,re.M) or re.search(r'^--- FAIL|^FAIL',result,re.M):raise RuntimeError('Validation failed: '+file)
 if text('docs/test-output/mobile18-go-vet.txt').strip():raise RuntimeError('go vet must be clean')
 if not text('docs/test-output/mobile18-upgrade-helper.txt').rstrip().endswith('OK'):raise RuntimeError('Upgrade helper tests failed')
 layouts=json.loads(text('docs/test-output/mobile18-native-layout.json'))
 if layouts['checks']!=168 or not all(item['passed'] for item in layouts['layouts']):raise RuntimeError('Native layout matrix incomplete')
 local=json.loads(text('docs/test-output/mobile18-local.json'))
 if not local['readonly'] or len(local['layouts'])!=20 or not all(item['passed'] for item in local['layouts']) or local['browserErrors']:raise RuntimeError('Read-only local browser acceptance incomplete')
 before=json.loads(text('.local/mobile18-business-before.json'))
 if local['businessAfter']!=before:raise RuntimeError('Public business snapshot changed')
 manifest=json.loads(text('web/public/maps/manifest.json'))
 if manifest['resources']:raise RuntimeError('Re-audit resource publication before packaging a non-empty map manifest')
 release='mobile-acceptance-'+version.replace('+','-')+'-20261020';out=ROOT/'delivery'/release;out.mkdir(parents=True,exist_ok=True)
 copy(apk,out/'android'/apk.name)
 copy(ROOT/'docs/MOBILE18_ACCEPTANCE.md',out/'README.md')
 for path in (ROOT/'docs/test-output').glob('mobile18-*'):
  if path.is_file():copy(path,out/'verification'/path.name)
 for path in (ROOT/'docs/screenshots/app-v1.8').glob('*.png'):copy(path,out/'screenshots'/path.name)
 for path in (ROOT/'docs/screenshots/mobile18-local').glob('*.png'):copy(path,out/'screenshots/web-local'/path.name)
 source_zip=out/(release+'-source.zip');sources=sorted(set(source_files()))
 with zipfile.ZipFile(source_zip,'w',zipfile.ZIP_DEFLATED,compresslevel=6) as archive:
  for path in sources:archive.write(path,path.relative_to(ROOT).as_posix())
 stage=ROOT/'.local/mobile18-server-bundle';stage.mkdir(parents=True,exist_ok=True)
 for arch,machine in [('amd64',62),('arm64',183)]:
  binary=ROOT/'.local/mobile18-binaries'/('exhibition-linux-'+arch)
  with binary.open('rb') as stream:header=stream.read(20)
  if header[:4]!=b'\x7fELF' or struct.unpack('<H',header[18:20])[0]!=machine:raise RuntimeError('Wrong ELF architecture '+arch)
  copy(binary,stage/'server'/binary.name)
 for path in (ROOT/'web/dist').rglob('*'):
  if path.is_file() and 'previews' not in path.relative_to(ROOT/'web/dist').parts:copy(path,stage/'web/dist'/path.relative_to(ROOT/'web/dist'))
 for directory in ['migrations','catalog','licenses']:
  for path in (ROOT/'server'/directory).rglob('*'):
   if path.is_file():copy(path,stage/directory/path.relative_to(ROOT/'server'/directory))
 copy(ROOT/'deploy/linux/upgrade-storage.sh',stage/'upgrade.sh');copy(ROOT/'deploy/linux/aliyun-upgrade-support.py',stage/'aliyun-upgrade-support.py')
 copy(ROOT/'docs/MOBILE18_ACCEPTANCE.md',stage/'README.md')
 result={**receipt,'release':release,'schema':16,'builtAt':datetime.now(timezone.utc).isoformat(),'stableDownloadUpdated':False,'formalMapAccepted':False,'mapResourceCount':0,'mapPreviewPackaged':False,'productionDeployed':False,'linuxRuntimeTested':False,'sourceFiles':len(sources),'nativeLayoutChecks':layouts['checks'],'nativeFixtures':'synthetic only; real model integration uses a disposable MySQL database','unverified':['real geographic boundary publication and final drill-down acceptance','Android device cover installation','system camera, permissions, sharing and native media on a device','Linux host installation and MySQL8/real OSS']}
 (stage/'manifest.json').write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n',encoding='utf-8')
 (stage/'SHA256SUMS').write_text(''.join(sha(p)+'  '+p.relative_to(stage).as_posix()+'\n' for p in sorted(stage.rglob('*')) if p.is_file() and p.name!='SHA256SUMS'),encoding='utf-8')
 server_zip=out/(release+'-server-update.zip');zip_tree(stage,server_zip)
 with zipfile.ZipFile(source_zip) as archive:
  forbidden=[name for name in archive.namelist() if re.search(r'(^|/)(\.env|key\.properties|local\.properties|signing\.json)$|\.(jks|keystore|zqbak|db|sqlite)$|(^|/)(\.local|data|node_modules|build)/',name)]
  if forbidden:raise RuntimeError('Private source files: '+str(forbidden))
 with zipfile.ZipFile(server_zip) as archive:
  if any('/previews/' in name or 'map-preview' in name for name in archive.namelist()):raise RuntimeError('Acceptance/learning resources in server update')
 result['artifacts']=[{'file':p.relative_to(out).as_posix(),'bytes':p.stat().st_size,'sha256':sha(p)} for p in [out/'android'/apk.name,source_zip,server_zip]]
 (out/'manifest.json').write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n',encoding='utf-8')
 (out/'SHA256SUMS.txt').write_text(''.join(sha(p)+'  '+p.relative_to(out).as_posix()+'\n' for p in sorted(out.rglob('*')) if p.is_file() and p.name!='SHA256SUMS.txt'),encoding='utf-8')
 preview={'version':version,'url':'/downloads/previews/'+apk.name,'sha256':receipt['sha256'],'formal_map_accepted':False}
 for path in [ROOT/'web/public/downloads/previews/manifest.json',ROOT/'web/dist/downloads/previews/manifest.json']:
  path.parent.mkdir(parents=True,exist_ok=True);path.write_text(json.dumps(preview,ensure_ascii=False,indent=2)+'\n',encoding='utf-8')
 print(json.dumps({'directory':str(out),'sourceFiles':len(sources),'sourcePrivateFiles':[],'stableAPKUnchanged':True,'artifacts':result['artifacts']},ensure_ascii=False,indent=2))
if __name__=='__main__':main()
