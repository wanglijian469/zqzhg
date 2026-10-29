"""Internal allowlisted deployment snapshot, never a public Linux delivery."""
import hashlib,json,tarfile
from pathlib import Path
root=Path(__file__).resolve().parents[1]
entries={'server/exhibition':root/'.local/sms/service104-linux'}
for prefix,folder in [('web/dist',root/'web/dist'),('map-demo',root/'.local/map-preview')]:
 for f in folder.rglob('*'):
  if not f.is_file():continue
  relative=f.relative_to(folder)
  if prefix=='web/dist' and relative.parts[0]=='downloads':continue
  if f.is_symlink() or f.suffix.lower() in ('.apk','.jks','.env','.sql','.keystore'):raise RuntimeError('forbidden_release_input')
  entries[prefix+'/'+relative.as_posix()]=f
manifest=root/'.local/sms/service104-manifest.json'
manifest.write_text(json.dumps({'version':'1.10.4+42','company_verification_mode':'content_review','map_usage':'development-demonstration','files':{name:{'bytes':f.stat().st_size,'sha256':hashlib.sha256(f.read_bytes()).hexdigest()} for name,f in entries.items()}},indent=2),encoding='utf-8')
entries['release-manifest.json']=manifest
target=root/'.local/sms/service-1.10.4.tar.gz'
with tarfile.open(target,'w:gz') as archive:
 for name,f in entries.items():archive.add(f,arcname=name,recursive=False)
receipt={'archive':str(target),'sha256':hashlib.sha256(target.read_bytes()).hexdigest(),'bytes':target.stat().st_size,'files':len(entries)}
(root/'.local/sms/service104-package.json').write_text(json.dumps(receipt),encoding='utf-8')
print(json.dumps(receipt))
