#!/bin/sh
# Removes Vahti. Settings and data stay unless you pass --purge.
set -eu
[ "$(id -u)" -eq 0 ] || { echo "Run as root: sudo ./uninstall.sh" >&2; exit 1; }
echo "Remember to restore your router's DNS setting first, otherwise the network stops working."
systemctl disable --now vahti-update-lists.timer vahti.service 2>/dev/null || true
rm -f /etc/systemd/system/vahti.service /etc/systemd/system/vahti-update-lists.service /etc/systemd/system/vahti-update-lists.timer
systemctl daemon-reload
rm -f /usr/local/bin/vahti
rm -rf /usr/local/share/doc/vahti
if [ "${1:-}" = "--purge" ]; then
	rm -rf /var/lib/vahti /etc/vahti
	userdel vahti 2>/dev/null || true
	echo "Vahti and all its data were removed."
else
	echo "Vahti was removed. Settings remain in /etc/vahti and /var/lib/vahti (remove them with: sudo ./uninstall.sh --purge)."
fi
