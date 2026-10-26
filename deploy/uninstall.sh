#!/bin/sh
# Removes Vahti. Settings and data stay unless you pass --purge.
set -eu
[ "$(id -u)" -eq 0 ] || { echo "Aja pääkäyttäjänä: sudo ./uninstall.sh" >&2; exit 1; }
echo "Muista ensin palauttaa reitittimen DNS-asetus, muuten verkko lakkaa toimimasta."
systemctl disable --now vahti-update-lists.timer vahti.service 2>/dev/null || true
rm -f /etc/systemd/system/vahti.service /etc/systemd/system/vahti-update-lists.service /etc/systemd/system/vahti-update-lists.timer
systemctl daemon-reload
rm -f /usr/local/bin/vahti
rm -rf /usr/local/share/doc/vahti
if [ "${1:-}" = "--purge" ]; then
	rm -rf /var/lib/vahti /etc/vahti
	userdel vahti 2>/dev/null || true
	echo "Vahti ja kaikki sen tiedot poistettu."
else
	echo "Vahti poistettu. Asetukset jäivät kansioihin /etc/vahti ja /var/lib/vahti (poista: sudo ./uninstall.sh --purge)."
fi
