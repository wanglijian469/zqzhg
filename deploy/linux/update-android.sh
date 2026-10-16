#!/usr/bin/env bash
set -euo pipefail
umask 022
PACKAGE_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
INSTALL_DIR=/opt/zqzt
APK=dalunongji-1.5.14+22.apk
TARGET_DIR="$INSTALL_DIR/current/web/dist/downloads"
[[ -f "$PACKAGE_DIR/$APK" && -f "$PACKAGE_DIR/SHA256SUMS" ]] || { echo 'Extract the full Android update archive first.' >&2; exit 1; }
(cd "$PACKAGE_DIR" && sha256sum --check --quiet SHA256SUMS)
[[ -d "$TARGET_DIR" && -f "$TARGET_DIR/dalunongji.apk" ]] || { echo 'Existing /opt/zqzt application download directory was not found.' >&2; exit 1; }
expected=$(sha256sum "$PACKAGE_DIR/$APK" | cut -d ' ' -f 1)
old_hash=$(sha256sum "$TARGET_DIR/dalunongji.apk" | cut -d ' ' -f 1)
if [[ "$expected" == "$old_hash" ]]; then echo 'Android download is already 1.5.14+22.'; exit 0; fi
install -d -m 750 "$INSTALL_DIR/shared/apk-archive"
[[ -f "$INSTALL_DIR/shared/apk-archive/$old_hash.apk" ]] || install -m 644 "$TARGET_DIR/dalunongji.apk" "$INSTALL_DIR/shared/apk-archive/$old_hash.apk"
temporary=$(mktemp "$TARGET_DIR/.dalunongji.XXXXXX.apk")
trap 'rm -f -- "$temporary"' EXIT
install -m 644 "$PACKAGE_DIR/$APK" "$temporary"
mv -f -- "$temporary" "$TARGET_DIR/dalunongji.apk"
actual=$(sha256sum "$TARGET_DIR/dalunongji.apk" | cut -d ' ' -f 1)
[[ "$actual" == "$expected" ]]
echo 'Android download updated to 1.5.14+22. Website/database/service configuration retained; no restart required.'
echo 'Download: http://39.97.250.244:8089/downloads/dalunongji.apk'
echo "SHA256: $actual"
