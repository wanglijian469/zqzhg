"""Build an allowlisted deployment archive; exclude downloads and private data."""
import hashlib
import json
from pathlib import Path
import tarfile

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / '.local/sms/sms-demo-1.10.1.tar.gz'
entries = {'server/exhibition': ROOT / '.local/sms/exhibition-final-linux'}
for prefix, directory in [('web/dist', ROOT / 'web/dist'), ('map-demo', ROOT / '.local/map-preview'), ('public-demo', ROOT / '.local/sms/public-demo-20261028')]:
    for file in directory.rglob('*'):
        if not file.is_file():
            continue
        relative = file.relative_to(directory)
        if prefix == 'web/dist' and relative.parts[0] == 'downloads':
            continue
        if file.is_symlink() or file.suffix.lower() in ('.env', '.jks', '.sql', '.keystore', '.apk'):
            raise ValueError('forbidden release input')
        entries[prefix + '/' + relative.as_posix()] = file
inventory = {name: {'bytes': file.stat().st_size, 'sha256': hashlib.sha256(file.read_bytes()).hexdigest()} for name, file in entries.items()}
manifest = ROOT / '.local/sms/release-manifest.json'
manifest.write_text(json.dumps({'version': '1.10.1+39', 'map_usage': 'development-demonstration', 'files': inventory}, ensure_ascii=False, indent=2), encoding='utf-8')
entries['release-manifest.json'] = manifest
with tarfile.open(OUT, 'w:gz') as archive:
    for name, file in entries.items():
        archive.add(file, arcname=name, recursive=False)
print(json.dumps({'archive': str(OUT), 'bytes': OUT.stat().st_size, 'sha256': hashlib.sha256(OUT.read_bytes()).hexdigest(), 'files': len(entries)}))
