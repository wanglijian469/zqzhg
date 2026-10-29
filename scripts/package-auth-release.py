"""Allowlisted 1.10.2 deployment build; public downloads and private data excluded."""
import hashlib,json,tarfile
from pathlib import Path
root=Path(__file__).resolve().parents[1]
entries={'server/exhibition':root/'.local/sms/auth102-linux'}
for prefix,folder in [('web/dist',root/'web/dist'),('map-demo',root/'.local/map-preview')]:
 for file in folder.rglob('*'):
  if not file.is_file():continue
  relative=file.relative_to(folder)
  if prefix=='web/dist' and relative.parts[0]=='downloads':continue
  if file.is_symlink() or file.suffix.lower() in ('.apk','.jks','.env','.sql','.keystore'):raise RuntimeError('forbidden_release_input')
  entries[prefix+'/'+relative.as_posix()]=file
manifest=root/'.local/sms/auth102-manifest.json'
manifest.write_text(json.dumps({'version':'1.10.2+40','map_usage':'development-demonstration','files':{name:{'bytes':file.stat().st_size,'sha256':hashlib.sha256(file.read_bytes()).hexdigest()} for name,file in entries.items()}},indent=2),encoding='utf-8')
entries['release-manifest.json']=manifest
output=root/'.local/sms/auth-1.10.2.tar.gz'
with tarfile.open(output,'w:gz') as archive:
 for name,file in entries.items():archive.add(file,arcname=name,recursive=False)
print(json.dumps({'bytes':output.stat().st_size,'sha256':hashlib.sha256(output.read_bytes()).hexdigest(),'files':len(entries)}))
