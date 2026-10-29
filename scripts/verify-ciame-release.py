"""Verify the delivered APK, shared catalog and retained stable download bytes."""
from pathlib import Path
import hashlib
import json
import zipfile

root = Path(__file__).resolve().parents[1]
folder = root / "delivery/android-v1.9.0"
manifest = json.loads((folder / "manifest.json").read_text(encoding="utf-8-sig"))
apk = folder / "大陆农机-1.9.0+37.apk"
digest = lambda p: hashlib.sha256(p.read_bytes()).hexdigest()
assert digest(apk) == manifest["sha256"]
assert manifest["signingMatchesPrevious"] is True
assert manifest["applicationId"] == "com.zqzt.dalunongji"
assert manifest["appName"] == "大陆农机"
assert manifest["version"] == "1.9.0+37"
with zipfile.ZipFile(apk) as package:
    names = package.namelist()
    translation = next(n for n in names if n.endswith("flutter_assets/assets/i18n/zh.json"))
    words = json.loads(package.read(translation))
    assert words["brand"] == "中企展馆"
    assert words["exhibition"] == "中企展馆-农业机械厅"
    assert not any(n.startswith("assets/workspace/") and n.endswith(".gz") for n in names)
    embedded = json.loads(package.read("assets/workspace/bundle-manifest.json"))
    for entry in embedded["files"]:
        data = package.read("assets/workspace/" + entry["path"])
        assert len(data) == entry["bytes"]
        assert hashlib.sha256(data).hexdigest() == entry["sha256"]
        website_file = root / "web/dist" / entry["path"]
        assert website_file.is_file(), "Embedded file missing from website: " + entry["path"]
        assert website_file.read_bytes() == data, "Embedded Web content differs: " + entry["path"]
stable_paths = [root / p for p in ["delivery/android/dalunongji.apk", "web/public/downloads/dalunongji.apk", "web/dist/downloads/dalunongji.apk"]]
stable = [digest(p) for p in stable_paths]
previous = json.loads((root / ".local/android-preview-verification.json").read_text(encoding="utf-8-sig"))
assert len(set(stable)) == 1 and stable[0] == previous["stableSHA256"]
catalog = json.loads((root / "server/catalog/ciame-2026.json").read_text(encoding="utf-8"))
assert len(catalog["groups"]) == 29
assert sum(len(g["children"]) for g in catalog["groups"]) == 159
assert digest(root / catalog["source"]) == catalog["sha256"]
report = {
    "version": manifest["version"], "applicationId": manifest["applicationId"],
    "appName": manifest["appName"], "platformName": words["brand"],
    "industryTitle": words["exhibition"], "apkBytes": apk.stat().st_size,
    "apkSHA256": manifest["sha256"], "certificateSHA256": manifest["certificateSHA256"],
    "signingMatchesPrevious": True, "stableDownloadsUnchanged": True,
    "stableSHA256": stable[0], "embeddedWebFilesVerified": len(embedded["files"]), "embeddedWebMatchesWebsite": True,
    "ciamePrimary": 29, "ciameSecondary": 159, "sourceWorkbookSHA256": catalog["sha256"],
    "productionDeployed": False, "deviceValidation": False, "iosBuilt": False,
}
out = root / "docs/test-output/ciame-1.9.0-release.json"
out.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
print(json.dumps(report, ensure_ascii=True))
