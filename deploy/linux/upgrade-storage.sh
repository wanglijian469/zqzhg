#!/usr/bin/env bash
# Upgrade an existing managed installation. Never import/reset business data.
set -Eeuo pipefail
set +x
umask 077
BUNDLE="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
INSTALL_DIR="${INSTALL_DIR:-/opt/zqzt}"
ENV_FILE="${ENV_FILE:-/etc/zqzt/exhibition.env}"
SERVICE="${SERVICE:-zqzt-exhibition.service}"
MODE=check
case "${1:---check}" in
 --check) ;;
 --apply) MODE=apply;;
 --help|-h) printf 'Usage: sudo bash upgrade.sh [--check|--apply]\nEnvironment: INSTALL_DIR, ENV_FILE, SERVICE, BACKUP_ROOT (absolute private directory).\nCheck is read-only. Apply stops the service, backs up DB/media/control/env, then switches programs.\n'; exit 0;;
 *) echo 'Use --check or --apply.' >&2; exit 1;;
esac
fail(){ printf 'ERROR: %s\n' "$*" >&2; exit 1; }
[[ "$(uname -s)" == Linux ]] || fail 'Run on the Linux ECS host.'
[[ "$INSTALL_DIR" == /* && "$INSTALL_DIR" != / && "$ENV_FILE" == /* ]] || fail 'Use absolute server paths.'
[[ "$SERVICE" =~ ^[A-Za-z0-9_.@-]+$ ]] || fail 'Invalid service name.'
for command in python3 sha256sum systemctl curl tar install; do command -v "$command" >/dev/null || fail "Missing tool: $command"; done
[[ -L "$INSTALL_DIR/current" && -f "$ENV_FILE" ]] || fail 'Existing managed current symlink and environment file required.'
(cd "$BUNDLE" && sha256sum --check --quiet SHA256SUMS) || fail 'Bundle checksum mismatch.'
case "$(uname -m)" in x86_64|amd64) ARCH=amd64;; aarch64|arm64) ARCH=arm64;; *) fail 'Supported: amd64/arm64.';; esac
[[ -f "$BUNDLE/server/exhibition-linux-$ARCH" && -d "$BUNDLE/web/dist" ]] || fail 'Matching binary or static files missing.'
CONTEXT="$(python3 "$BUNDLE/aliyun-upgrade-support.py" inspect --env-file "$ENV_FILE" --install-dir "$INSTALL_DIR")"
printf '%s\n' "$CONTEXT"
[[ "$MODE" == apply ]] || { echo 'Preflight passed. No files, environment, database or service changed.'; exit 0; }
((EUID==0)) || fail 'Run apply with sudo/root.'
OLD="$(readlink -f "$INSTALL_DIR/current")"
[[ "$OLD" == "$INSTALL_DIR/releases/"* ]] || fail 'Current release is outside the managed releases directory.'
VERSION="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["release"])' "$BUNDLE/manifest.json")"
[[ "$VERSION" =~ ^[A-Za-z0-9._-]+$ ]] || fail 'Invalid release identifier.'
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
RELEASE="$INSTALL_DIR/releases/$VERSION-$STAMP"
BACKUP_ROOT="${BACKUP_ROOT:-$INSTALL_DIR/shared/upgrade-backups}"
[[ "$BACKUP_ROOT" == /* && "$BACKUP_ROOT" != / ]] || fail 'Invalid backup root.'
BACKUP="$BACKUP_ROOT/$VERSION-$STAMP"
[[ ! -e "$RELEASE" && ! -e "$BACKUP" ]] || fail 'Release/backup already exists; do not overwrite.'
STOPPED=0
SWITCHED=0
finish(){
 local result=$?
 if ((result!=0 && STOPPED)); then
  if ((SWITCHED)); then
   systemctl stop "$SERVICE" || true
   printf 'Upgrade failed after switch. Service left stopped. Preserve %s and %s.\nReview README before restoring the paired DB/media/env/control backup. Do not start an older backend that lacks hall/address isolation.\n' "$RELEASE" "$BACKUP" >&2
  else
   systemctl start "$SERVICE" || true
   printf 'No program switch occurred; previous service restarted. Backup stage: %s\n' "$BACKUP" >&2
  fi
 fi
 return "$result"
}
trap finish EXIT
install -d -m 0755 "$RELEASE/server" "$RELEASE/web"
install -m 0755 "$BUNDLE/server/exhibition-linux-$ARCH" "$RELEASE/server/exhibition"
cp -a "$BUNDLE/web/dist" "$RELEASE/web/"
cp -a "$BUNDLE/migrations" "$BUNDLE/catalog" "$BUNDLE/licenses" "$RELEASE/server/"
OWNER="$(systemctl show "$SERVICE" --property=User --value)"
[[ -z "$OWNER" ]] || chown -R "$OWNER" "$RELEASE"
STOPPED=1
systemctl stop "$SERVICE"
python3 "$BUNDLE/aliyun-upgrade-support.py" backup --env-file "$ENV_FILE" --install-dir "$INSTALL_DIR" --output "$BACKUP" >"$RELEASE/upgrade-backup-location.json"
PORT="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["port"])' "$BACKUP/context.json")"
# The environment, current database, media roots and encryption key are unchanged.
ln -s "$RELEASE" "$INSTALL_DIR/current.next-$STAMP"
SWITCHED=1
mv -Tf "$INSTALL_DIR/current.next-$STAMP" "$INSTALL_DIR/current"
systemctl start "$SERVICE"
for attempt in {1..45}; do
 if curl --noproxy '*' -fsS --max-time 2 "http://127.0.0.1:$PORT/api/v1/health" >/dev/null; then
  STOPPED=0
  printf 'Healthy. New release: %s\nPrevious program retained: %s\nPrivate backup: %s\n' "$RELEASE" "$OLD" "$BACKUP"
  exit 0
 fi
 sleep 1
done
fail 'New service did not pass health checks. See journalctl and the private backup.'
