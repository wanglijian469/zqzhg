#!/usr/bin/env bash
set +x
set -euo pipefail
umask 077
PACKAGE_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
VERSION=1.5.13
PUBLIC_URL=http://39.97.250.244:8089
PORT=8089
DB_HOST=127.0.0.1
DB_PORT=3306
DB_ADMIN=root
DB_NAME=zqzt_exhibition_cloud
INSTALL_DIR=/opt/zqzt
MIGRATION="$PACKAGE_DIR/migration"
DRY_RUN=0
PROMPT_APP_PASSWORD=0
MYSQL_CNF=
SWITCHED=0
PREVIOUS=
fail() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
generate_app_password() {
 local random_part
 random_part=$(openssl rand -hex 24) || return 1
 printf 'Aa1@Bb2%%Cc3+%s' "$random_part"
}
usage() {
 cat <<'HELP'
Usage: sudo bash deploy.sh [options]
  --db-host HOST       Existing local MySQL (127.0.0.1 or localhost)
  --db-port PORT       Existing MySQL port (default 3306)
  --db-admin USER      MySQL administrator (default root)
  --db-name NAME       NEW isolated database (default zqzt_exhibition_cloud)
  --migration DIR     Private migration directory (auto-detected next to deploy.sh)
  --public-url URL    Public origin (default http://39.97.250.244:8089)
  --port PORT         HTTP port (default 8089)
  --install-dir DIR   Application root (default /opt/zqzt)
  --dry-run           Validate options and show plan without changing this machine
  --prompt-app-db-password  Privately enter a separate application database password
MySQL password is requested privately through the terminal. No MySQL server is installed/restarted.
HELP
}
while (($#)); do
 case "$1" in
  --db-host|--db-port|--db-admin|--db-name|--migration|--public-url|--port|--install-dir)
   (($#>=2)) || fail "Missing value for $1"
   case "$1" in
    --db-host) DB_HOST=$2;; --db-port) DB_PORT=$2;; --db-admin) DB_ADMIN=$2;; --db-name) DB_NAME=$2;;
    --migration) MIGRATION=$2;; --public-url) PUBLIC_URL=$2;; --port) PORT=$2;; --install-dir) INSTALL_DIR=$2;;
   esac; shift 2;;
  --prompt-app-db-password) PROMPT_APP_PASSWORD=1; shift;;
  --dry-run) DRY_RUN=1; shift;; --help|-h) usage; exit 0;; *) fail "Unknown argument: $1";;
 esac
done
[[ "$DB_NAME" =~ ^zqzt_exhibition_[A-Za-z0-9_]{1,40}$ ]] || fail 'Use a database name starting with zqzt_exhibition_.'
[[ "$DB_ADMIN" =~ ^[A-Za-z0-9_]+$ ]] || fail 'Invalid MySQL administrator name.'
[[ "$DB_HOST" == 127.0.0.1 || "$DB_HOST" == localhost ]] || fail 'This installer is for the existing MySQL on this ECS instance.'
for value in "$PORT" "$DB_PORT"; do [[ "$value" =~ ^[0-9]{1,5}$ ]] && ((10#$value>=1 && 10#$value<=65535)) || fail 'Invalid port.'; done
[[ "$INSTALL_DIR" =~ ^/[A-Za-z0-9/_-]+$ && "$INSTALL_DIR" != / ]] || fail 'Invalid install directory.'
[[ "$PUBLIC_URL" =~ ^https?://[A-Za-z0-9.-]+(:[0-9]{1,5})?$ ]] || fail 'Public URL must be an HTTP(S) origin without a path.'
case "$(uname -m)" in x86_64|amd64) ARCH=amd64;; aarch64|arm64) ARCH=arm64;; *) fail 'Supported CPUs: x86_64 and aarch64.';; esac
if ((DRY_RUN)); then
 printf 'Version: %s\nCPU: %s\nPublic URL: %s\nListen: 0.0.0.0:%s\nExisting MySQL: %s:%s\nNew database: %s\nInstall: %s\nMigration: %s\n' "$VERSION" "$ARCH" "$PUBLIC_URL" "$PORT" "$DB_HOST" "$DB_PORT" "$DB_NAME" "$INSTALL_DIR" "$MIGRATION"
 exit 0
fi
[[ "$(uname -s)" == Linux ]] || fail 'Run this installer on the Linux ECS instance.'
((EUID==0)) || fail 'Run with sudo or as root.'
command -v systemctl >/dev/null || fail 'systemd is required.'
[[ -f "$PACKAGE_DIR/SHA256SUMS" ]] || fail 'Package checksum list is missing.'
(cd "$PACKAGE_DIR" && sha256sum --check --quiet SHA256SUMS) || fail 'Package checksum verification failed.'
[[ -f "$PACKAGE_DIR/bin/linux-$ARCH/exhibition" ]] || fail 'Matching Linux executable is missing.'
if command -v dnf >/dev/null; then PKG=dnf; elif command -v yum >/dev/null; then PKG=yum; elif command -v apt-get >/dev/null; then PKG=apt-get; else PKG=; fi
missing=0
for command_name in curl openssl gzip tar; do command -v "$command_name" >/dev/null || missing=1; done
if ((missing)); then
 [[ -n "$PKG" ]] || fail 'Install curl, openssl, gzip and tar first.'
 if [[ "$PKG" == apt-get ]]; then apt-get update; fi
 "$PKG" install -y curl openssl gzip tar ca-certificates
fi
MYSQL_CLIENT=$(command -v mysql || command -v mariadb || true)
if [[ -z "$MYSQL_CLIENT" ]]; then
 for candidate in /usr/local/mysql/bin/mysql /usr/local/mariadb/bin/mysql /www/server/mysql/bin/mysql; do [[ ! -x "$candidate" ]] || { MYSQL_CLIENT=$candidate; break; }; done
fi
[[ -n "$MYSQL_CLIENT" ]] || fail 'Existing MySQL client was not found. Add its bin directory to PATH and retry; this script will not reinstall your database.'
install -d -m 755 "$INSTALL_DIR" "$INSTALL_DIR/releases" "$INSTALL_DIR/shared"
if ! id zqzt >/dev/null 2>&1; then useradd --system --home-dir "$INSTALL_DIR" --shell /sbin/nologin zqzt; fi
install -d -o zqzt -g zqzt -m 750 "$INSTALL_DIR/shared/media"
install -d -m 750 -g zqzt /etc/zqzt
ENV_FILE=/etc/zqzt/exhibition.env
if [[ ! -f "$ENV_FILE" ]] && command -v ss >/dev/null; then
 if ss -H -ltn "sport = :$PORT" | grep -q .; then fail "TCP $PORT is already in use. Choose another port or stop its owning service first."; fi
fi
cleanup() {
 local status=$?
 [[ -z "$MYSQL_CNF" ]] || rm -f -- "$MYSQL_CNF"
 if ((status!=0 && SWITCHED)); then
  if [[ -n "$PREVIOUS" && -d "$PREVIOUS" ]]; then
   ln -sfn "$PREVIOUS" "$INSTALL_DIR/current"
   systemctl restart zqzt-exhibition.service || true
   printf 'Previous application release restored. Database/media retained.\n' >&2
  else systemctl stop zqzt-exhibition.service || true; fi
 fi
}
trap cleanup EXIT
if [[ ! -f "$ENV_FILE" ]]; then
 [[ -d "$MIGRATION" && -f "$MIGRATION/database.sql.gz" && -d "$MIGRATION/media" ]] || fail 'Migration package is missing. Extract the private data archive into this deployment folder first.'
 MIGRATION="$(cd -- "$MIGRATION" && pwd -P)"
 (cd "$MIGRATION" && sha256sum --check --quiet SHA256SUMS) || fail 'Migration checksum verification failed.'
 printf 'Connecting to existing MySQL %s:%s as %s.\n' "$DB_HOST" "$DB_PORT" "$DB_ADMIN"
 if [[ -z ${DB_ADMIN_PASSWORD+x} ]]; then read -r -s -p 'Existing MySQL administrator password (empty for passwordless local access): ' DB_ADMIN_PASSWORD </dev/tty; printf '\n'; fi
 MYSQL_CNF=$(mktemp /tmp/zqzt-mysql.XXXXXX)
 escaped_password=${DB_ADMIN_PASSWORD//\\/\\\\}; escaped_password=${escaped_password//\"/\\\"}
 [[ "$escaped_password" != *$'\n'* && "$escaped_password" != *$'\r'* ]] || fail 'Multiline database passwords are not supported by the installer.'
 printf '[client]\nhost=%s\nport=%s\nuser=%s\npassword="%s"\ndefault-character-set=utf8mb4\n' "$DB_HOST" "$DB_PORT" "$DB_ADMIN" "$escaped_password" >"$MYSQL_CNF"
 unset DB_ADMIN_PASSWORD escaped_password
 MYSQL=("$MYSQL_CLIENT" "--defaults-extra-file=$MYSQL_CNF" --batch --skip-column-names)
 "${MYSQL[@]}" -e 'SELECT 1' >/dev/null || fail 'Could not authenticate to existing MySQL.'
 exists=$("${MYSQL[@]}" -e "SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='$DB_NAME'")
 [[ "$exists" == 0 ]] || fail "Database $DB_NAME already exists. Refusing to overwrite it; choose another --db-name."
 APP_DB_USER="zqzt_$(openssl rand -hex 6)"
 if ((PROMPT_APP_PASSWORD)); then
  read -r -s -p 'NEW application database password: ' APP_DB_PASSWORD </dev/tty; printf '\n'
  read -r -s -p 'Confirm application database password: ' APP_DB_PASSWORD_CONFIRM </dev/tty; printf '\n'
  [[ "$APP_DB_PASSWORD" == "$APP_DB_PASSWORD_CONFIRM" ]] || fail 'Application passwords do not match.'
  unset APP_DB_PASSWORD_CONFIRM
  [[ "$APP_DB_PASSWORD" =~ ^[A-Za-z0-9@%+_=!?:.,~-]{12,128}$ ]] || fail 'Use 12-128 letters/digits and @%+_=!?:.,~-; spaces and quotes are not supported.'
 else APP_DB_PASSWORD=$(generate_app_password); fi
 # Validate the password through CREATE USER before creating a database.
 # MYSQL stderr may include SQL text: retain only the error code, never the statement/password.
 if ! user_result=$("${MYSQL[@]}" 2>&1 <<SQL
CREATE USER '$APP_DB_USER'@'localhost' IDENTIFIED BY '$APP_DB_PASSWORD';
SQL
 ); then
  error_code=$(printf '%s\n' "$user_result" | sed -n 's/.*ERROR \([0-9][0-9]*\).*/\1/p' | head -n 1)
  unset user_result APP_DB_PASSWORD
  fail "Application database user could not be created (MySQL error ${error_code:-unknown}). No database was created. For password policy error 1819, retry with --prompt-app-db-password and a password matching your server policy."
 fi
 unset user_result
 if ! user_result=$("${MYSQL[@]}" 2>&1 <<SQL
CREATE USER '$APP_DB_USER'@'127.0.0.1' IDENTIFIED BY '$APP_DB_PASSWORD';
SQL
 ); then
  error_code=$(printf '%s\n' "$user_result" | sed -n 's/.*ERROR \([0-9][0-9]*\).*/\1/p' | head -n 1)
  unset user_result APP_DB_PASSWORD
  fail "Second application account host could not be created (MySQL error ${error_code:-unknown}). No database was created."
 fi
 unset user_result
 "${MYSQL[@]}" <<SQL
CREATE DATABASE \`$DB_NAME\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
GRANT ALL PRIVILEGES ON \`$DB_NAME\`.* TO '$APP_DB_USER'@'localhost';
GRANT ALL PRIVILEGES ON \`$DB_NAME\`.* TO '$APP_DB_USER'@'127.0.0.1';
SQL
 printf 'Importing into NEW database %s...\n' "$DB_NAME"
 gzip -dc -- "$MIGRATION/database.sql.gz" | "${MYSQL[@]}" "$DB_NAME"
 "${MYSQL[@]}" "$DB_NAME" <"$MIGRATION/counts.sql" >"$INSTALL_DIR/shared/import-counts.tsv"
 diff -u "$MIGRATION/expected-counts.tsv" "$INSTALL_DIR/shared/import-counts.tsv" || fail 'Restored record counts differ; service has not been started.'
 cp -a "$MIGRATION/media/." "$INSTALL_DIR/shared/media/"
 chown -R zqzt:zqzt "$INSTALL_DIR/shared/media"
 chmod -R u=rwX,g=rX,o= "$INSTALL_DIR/shared/media"
 INIT_ADMIN_PASSWORD=$(openssl rand -hex 24)
 cat >"$ENV_FILE" <<ENV
APP_ENV=production
HTTP_ADDR=0.0.0.0:$PORT
DB_HOST=127.0.0.1
DB_PORT=$DB_PORT
DB_USER=$APP_DB_USER
DB_PASSWORD=$APP_DB_PASSWORD
DB_NAME=$DB_NAME
ADMIN_USERNAME=admin
ADMIN_PASSWORD=$INIT_ADMIN_PASSWORD
MEDIA_DIR=$INSTALL_DIR/shared/media
WEB_DIR=$INSTALL_DIR/current/web/dist
CORS_ORIGIN=$PUBLIC_URL
PUBLIC_BASE_URL=$PUBLIC_URL
APP_DOWNLOAD_BASE_URL=$PUBLIC_URL
SMS_PROVIDER=disabled
ENV
 chmod 640 "$ENV_FILE"; chown root:zqzt "$ENV_FILE"
 unset APP_DB_PASSWORD INIT_ADMIN_PASSWORD
 printf 'Existing administrator/manufacturer accounts were imported; their passwords were retained. Old sessions/SMS codes were excluded.\n'
else
 printf 'Existing application configuration found: upgrade mode. Database import is skipped and existing configuration/media retained.\n'
 configured_port=$(sed -n 's/^HTTP_ADDR=0.0.0.0://p' "$ENV_FILE" | tail -n 1)
 [[ "$configured_port" =~ ^[0-9]+$ ]] || fail 'Existing HTTP_ADDR cannot be read.'
 PORT=$configured_port
 PUBLIC_URL=$(sed -n 's/^PUBLIC_BASE_URL=//p' "$ENV_FILE" | tail -n 1)
fi
if ! command -v ffmpeg >/dev/null || ! command -v ffprobe >/dev/null; then
 if [[ -n "$PKG" ]]; then "$PKG" install -y ffmpeg || true; fi
 if ! command -v ffmpeg >/dev/null || ! command -v ffprobe >/dev/null; then
  printf '\nNOTICE: FFmpeg/FFprobe are absent from configured repositories. Website, images and PDF catalogs can run; video transcoding requires installing FFmpeg and restarting this service.\n' >&2
 fi
fi
RELEASE_DIR="$INSTALL_DIR/releases/$VERSION-$(date -u +%Y%m%d%H%M%S)"
install -d -m 755 "$RELEASE_DIR/server" "$RELEASE_DIR/web"
install -m 755 "$PACKAGE_DIR/bin/linux-$ARCH/exhibition" "$RELEASE_DIR/server/exhibition"
cp -a "$PACKAGE_DIR/web/dist" "$RELEASE_DIR/web/"
chmod -R u=rwX,go=rX "$RELEASE_DIR"
PREVIOUS=$(readlink -f "$INSTALL_DIR/current" 2>/dev/null || true)
ln -s "$RELEASE_DIR" "$INSTALL_DIR/current.new.$$"
mv -Tf "$INSTALL_DIR/current.new.$$" "$INSTALL_DIR/current"
SWITCHED=1
cat >/etc/systemd/system/zqzt-exhibition.service <<UNIT
[Unit]
Description=ZQZT Exhibition
After=network-online.target mysqld.service mariadb.service
Wants=network-online.target
[Service]
Type=simple
User=zqzt
Group=zqzt
WorkingDirectory=$INSTALL_DIR/current
EnvironmentFile=$ENV_FILE
ExecStart=$INSTALL_DIR/current/server/exhibition
Restart=on-failure
RestartSec=5
UMask=0027
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ProtectHome=true
[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable zqzt-exhibition.service
systemctl restart zqzt-exhibition.service
healthy=0
for ((attempt=0;attempt<30;attempt++)); do
 if curl --noproxy '*' -fsS --connect-timeout 2 --max-time 4 "http://127.0.0.1:$PORT/api/v1/health" >/dev/null; then healthy=1; break; fi
 sleep 2
done
((healthy)) || fail 'Health check failed. Inspect: journalctl -u zqzt-exhibition -n 80 --no-pager'
if command -v firewall-cmd >/dev/null && firewall-cmd --state >/dev/null 2>&1; then
 firewall-cmd --permanent --add-port="$PORT/tcp"
 firewall-cmd --add-port="$PORT/tcp"
fi
SWITCHED=0
printf '\nDeployment healthy.\nWebsite: %s\nAdmin: %s/admin\nApp download: %s/app\nHealth: %s/api/v1/health\n' "$PUBLIC_URL" "$PUBLIC_URL" "$PUBLIC_URL" "$PUBLIC_URL"
printf 'ECS security group: allow inbound TCP %s. Do not expose MySQL.\nConfig: %s\nLogs: journalctl -u zqzt-exhibition -f\n' "$PORT" "$ENV_FILE"
