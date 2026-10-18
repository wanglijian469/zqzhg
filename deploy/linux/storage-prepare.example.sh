# Prepare persistent directories once. Run as root; substitute the existing service user.
# Do not run fresh-install database scripts over an existing deployment.
install -d -m 0700 -o zqzt -g zqzt /data/zqzhg/media /data/zqzhg/temp /data/zqzhg/backup /data/zqzhg/operations
# In /etc/zqzt/exhibition.env set the four directories; preserve CONFIG_ENCRYPTION_KEY,
# existing DB configuration, TRUSTED_PROXIES, backup catalog and active.json.
# Copy existing objects with storage-copy-verify.py BEFORE changing MEDIA_DIR.
# For NAS register the exact mounted directory in LOCAL_STORAGE_ROOTS_JSON.
# systemctl edit zqzt-exhibition.service:
# [Unit]
# RequiresMountsFor=/data/zqzhg/media /data/zqzhg/operations
# After=network-online.target
# [Service]
# ReadWritePaths=/data/zqzhg
# PrivateTmp=true
# Then: systemctl daemon-reload
