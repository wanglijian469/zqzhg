"""Configure renewal only for the new zqzhg certificate; preserve other sites."""
from pathlib import Path
import subprocess

units = Path('/etc/systemd/system')
(units / 'zqzhg-cert-renew.service').write_text('''[Unit]
Description=Renew zqzhg.com TLS certificate
After=network-online.target
[Service]
Type=oneshot
ExecStart=/opt/zqzt/tools/certbot/bin/certbot renew --cert-name zqzhg.com --quiet --deploy-hook "nginx -t && systemctl reload nginx"
''')
(units / 'zqzhg-cert-renew.timer').write_text('''[Unit]
Description=Check zqzhg.com TLS renewal twice daily
[Timer]
OnCalendar=*-*-* 03,15:20:00
RandomizedDelaySec=1800
Persistent=true
[Install]
WantedBy=timers.target
''')
subprocess.run(['systemctl', 'daemon-reload'], check=True)
subprocess.run(['systemctl', 'enable', '--now', 'zqzhg-cert-renew.timer'], check=True)
print('zqzhg renewal timer enabled; other certificates unchanged')
