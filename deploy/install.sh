#!/bin/sh
# Vahti installer for Raspberry Pi OS, Debian and Ubuntu (systemd).
# Run from an extracted release folder:  sudo ./install.sh
set -eu

PREFIX=/usr/local
CONF=/etc/vahti
DATA=/var/lib/vahti
HERE=$(cd "$(dirname "$0")" && pwd)

# say prints a message. fail prints an error message and stops the script.
say()  { printf '%s\n' "$*"; }
fail() { printf 'Error: %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || fail "run the installer as root: sudo ./install.sh"
command -v systemctl >/dev/null 2>&1 || fail "systemd is missing. Vahti needs systemd."

BIN="$HERE/vahti"
[ -f "$BIN" ] || fail "the vahti program was not found in $HERE"

# The downloaded archive is verified before extraction (see README: sha256sum and gh attestation verify).
"$BIN" version >/dev/null 2>&1 || fail "the program does not run on this device. Check that you downloaded the right architecture ($(uname -m))."

# Something else on port 53 (systemd-resolved, dnsmasq, Pi-hole) would block Vahti.
if command -v ss >/dev/null 2>&1; then
	busy=$(ss -H -lunp 'sport = :53' 2>/dev/null | grep -v vahti || true)
	if [ -n "$busy" ]; then
		say ""
		say "Port 53 is already used by another program:"
		say "$busy"
		if printf '%s' "$busy" | grep -q systemd-resolve; then
			say "Turn off systemd-resolved's local listener:"
			say "  sudo mkdir -p /etc/systemd/resolved.conf.d"
			say "  printf '[Resolve]\\nDNSStubListener=no\\n' | sudo tee /etc/systemd/resolved.conf.d/vahti.conf"
			say "  sudo systemctl restart systemd-resolved"
		fi
		fail "free port 53 and run the installer again."
	fi
fi

say "Creating the service user vahti..."
if ! id vahti >/dev/null 2>&1; then
	useradd --system --no-create-home --home-dir "$DATA" --shell /usr/sbin/nologin vahti
fi

say "Installing the program..."
install -m 0755 -o root -g root "$BIN" "$PREFIX/bin/vahti"
install -d -m 0755 "$PREFIX/share/doc/vahti"
[ -f "$HERE/README.md" ] && install -m 0644 "$HERE/README.md" "$PREFIX/share/doc/vahti/README.md"

install -d -m 0755 "$CONF"
if [ ! -f "$CONF/config.json" ]; then
	install -m 0644 "$HERE/config.json" "$CONF/config.json"
else
	say "Keeping the existing $CONF/config.json"
fi

install -m 0644 "$HERE/vahti.service" /etc/systemd/system/vahti.service
install -m 0644 "$HERE/vahti-update-lists.service" /etc/systemd/system/vahti-update-lists.service
install -m 0644 "$HERE/vahti-update-lists.timer" /etc/systemd/system/vahti-update-lists.timer
systemctl daemon-reload
systemctl enable --now vahti.service

# Wait for the first start to create the setup code.
i=0
while [ ! -f "$DATA/setup-code" ] && [ $i -lt 20 ] && ! grep -q passwordHash "$DATA/state.json" 2>/dev/null; do
	sleep 0.5; i=$((i + 1))
done
systemctl is-active --quiet vahti.service || { journalctl -u vahti -n 20 --no-pager; fail "Vahti did not start. See the log above."; }

say ""
printf 'Download blocklists now and update them weekly? This is the only thing Vahti fetches from the internet. [Y/n] '
read -r ans || ans=""
case "$ans" in
	[nN]*) say "Lists were not downloaded. You can download them later: sudo systemctl start vahti-update-lists" ;;
	*) systemctl enable --now vahti-update-lists.timer >/dev/null
	   systemctl start vahti-update-lists.service || say "Downloading the lists failed. See: journalctl -u vahti-update-lists" ;;
esac

IP=$(hostname -I 2>/dev/null | awk '{print $1}')
FP=$(journalctl -u vahti --no-pager -o cat 2>/dev/null | grep -o 'sha256=[0-9A-F:]*' | tail -1 | cut -d= -f2)
say ""
say "Vahti is installed and running."
say ""
say "1. Open in your browser:  https://$IP"
say "   The browser warns about a self-signed certificate. Check that the fingerprint is:"
say "   $FP"
if [ -f "$DATA/setup-code" ]; then
	say "2. Enter the setup code:  $(cat "$DATA/setup-code")"
fi
say "3. In your router's DHCP settings, set the DNS server to $IP (this one address only)."
say "   Also give this device a fixed IP address in the router settings."
