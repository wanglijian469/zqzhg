"""Verify APK embedded assets and extract only public WebView resources for QA."""
import argparse, hashlib, json, zipfile
from pathlib import Path

parser = argparse.ArgumentParser()
parser.add_argument('apk', type=Path)
parser.add_argument('--output', type=Path, required=True)
parser.add_argument('--receipt', type=Path, required=True)
args = parser.parse_args()
destination = args.output.resolve()
destination.mkdir(parents=True, exist_ok=True)
with zipfile.ZipFile(args.apk) as archive:
    prefix = 'assets/workspace/'
    manifest = json.loads(archive.read(prefix + 'bundle-manifest.json'))
    for entry in manifest['files']:
        data = archive.read(prefix + entry['path'])
        assert len(data) == entry['bytes'], entry['path']
        assert hashlib.sha256(data).hexdigest() == entry['sha256'], entry['path']
    extracted = 0
    for name in archive.namelist():
        if name.endswith('/'):
            continue
        if not (name.startswith(prefix) or name in ('assets/native-workspace.js', 'assets/native-map.css')):
            continue
        relative = name.removeprefix('assets/')
        target = (destination / relative).resolve()
        assert destination in target.parents
        assert target.suffix.lower() not in ('.env', '.sql', '.jks', '.keystore', '.apk')
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(archive.read(name))
        extracted += 1
receipt = {
    'apk_sha256': hashlib.sha256(args.apk.read_bytes()).hexdigest(),
    'apk_bytes': args.apk.stat().st_size,
    'embedded_manifest_files_verified': len(manifest['files']),
    'public_files_extracted': extracted,
    'bundle_manifest_sha256': hashlib.sha256(json.dumps(manifest,sort_keys=True).encode()).hexdigest(),
    'passed': True,
}
args.receipt.parent.mkdir(parents=True, exist_ok=True)
args.receipt.write_text(json.dumps(receipt, indent=2), encoding='utf-8')
print(json.dumps(receipt))
