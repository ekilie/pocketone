#!/bin/sh
# After the package is installed or upgraded (.deb, .rpm, .apk). Makes the pocketone user and /var/lib/pocketone, and
# restarts the service if it was running. A new install leaves the service stopped until it's set up.
set -e

if ! id pocketone >/dev/null 2>&1; then
	if command -v useradd >/dev/null 2>&1; then
		useradd --system --user-group --home-dir /var/lib/pocketone --no-create-home \
			--shell "$(command -v nologin || echo /bin/false)" pocketone
	elif command -v adduser >/dev/null 2>&1; then # BusyBox (Alpine)
		addgroup -S pocketone 2>/dev/null || true
		adduser -S -D -H -h /var/lib/pocketone -s /sbin/nologin -G pocketone pocketone
	fi
fi
install -d -o pocketone -g pocketone -m 750 /var/lib/pocketone

if [ -d /run/systemd/system ] && command -v systemctl >/dev/null 2>&1; then
	systemctl daemon-reload || true
	if systemctl is-enabled --quiet pocketone.service 2>/dev/null; then
		systemctl try-restart pocketone.service || true
	else
		cat <<'EOF'

PocketOne is installed. To run it as a service:

  sudo -u pocketone pocketone superuser upsert <email> <password> --config /etc/pocketone/pocketone.yaml
  sudo nano /etc/pocketone/service.env     # your domain, for HTTPS on ports 80 and 443
  sudo systemctl enable --now pocketone

Guide: https://pocketone.dev/docs/guides/linux-server
EOF
	fi
fi
