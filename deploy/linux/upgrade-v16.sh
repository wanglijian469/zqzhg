#!/usr/bin/env bash
# Explicit administrator-run upgrade only. No storage activation or data cutover.
set -Eeuo pipefail
umask 077
INSTALL_DIR="${INSTALL_DIR:-/opt/zqzt}"
SERVICE="${SERVICE:-zqzt-exhibition}"
ENV_FILE="${ENV_FILE:-/etc/zqzt/exhibition.env}"
BUNDLE="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
[[ "$EUID" == 0 ]] || { echo 'Run as root.' >&2; exit 1; }
[[ "$INSTALL_DIR" = /* && "$INSTALL_DIR" != / ]] || exit 1
[[ -L "$INSTALL_DIR/current" && -f "$ENV_FILE" ]] || { echo 'Existing managed installation required; check INSTALL_DIR and ENV_FILE.' >&2; exit 1; }
(cd "$BUNDLE" && sha256sum -c SHA256SUMS)
case "$(uname -m)" in x86_64) ARCH=amd64;; aarch64|arm64) ARCH=arm64;; *) echo 'Unsupported architecture.' >&2; exit 1;; esac
[[ -f "$BUNDLE/server/exhibition-linux-$ARCH" && -d "$BUNDLE/web/dist" ]] || exit 1
RELEASE="$INSTALL_DIR/releases/operations-1.6.0-$(date +%Y%m%d%H%M%S)"
OLD="$(readlink -f "$INSTALL_DIR/current")"
[[ "$OLD" = "$INSTALL_DIR/releases/"* ]] || { echo 'Unexpected current release path.' >&2; exit 1; }
mkdir -p "$RELEASE/server" "$RELEASE/web" "$INSTALL_DIR/shared/operations"
install -m 750 "$BUNDLE/server/exhibition-linux-$ARCH" "$RELEASE/server/exhibition"
cp -a "$BUNDLE/web/dist" "$RELEASE/web/"
chown -R zqzt:zqzt "$RELEASE" "$INSTALL_DIR/shared/operations"
# Keep existing environment, database, media, encryption key and control pointer.
# The deployment administrator configures OPERATIONS_DIR and key before enabling
# sensitive features. Never replace server.env with the example file.
systemctl stop "$SERVICE"
ln -s "$RELEASE" "$INSTALL_DIR/current.v16-next"
mv -Tf "$INSTALL_DIR/current.v16-next" "$INSTALL_DIR/current"
systemctl start "$SERVICE"
PORT="$(sed -n 's/^HTTP_ADDR=.*:\([0-9]*\)$/\1/p' "$ENV_FILE" | tail -n 1)"
[[ "$PORT" =~ ^[0-9]+$ ]] || PORT=8089
for i in $(seq 1 30); do
 if curl -fsS "http://127.0.0.1:$PORT/api/v1/health" >/dev/null; then
  printf 'V1.6.0 is healthy. Previous program release retained: %s\n' "$OLD"
  printf 'Configure private control directory, encryption key and restore maintenance account before page activation.\n'
  exit 0
 fi
 sleep 1
done
systemctl stop "$SERVICE"
ln -s "$OLD" "$INSTALL_DIR/current.v16-rollback"
mv -Tf "$INSTALL_DIR/current.v16-rollback" "$INSTALL_DIR/current"
systemctl start "$SERVICE"
echo 'Health check failed; previous program release restored. Additive migration 010 is retained.' >&2
exit 1