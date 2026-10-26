#!/bin/sh
# Prepares an old laptop (Debian/Ubuntu) to run Vahti as an always-on server:
# keeps it awake with the lid closed, turns on automatic security updates and,
# if ufw is installed, allows only DNS, the web interface and SSH from the LAN.
set -eu

# say prints a message. fail prints an error message and stops the script.
say()  { printf '%s\n' "$*"; }
fail() { printf 'Error: %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || fail "run as root: sudo ./prepare-laptop.sh"
command -v systemctl >/dev/null 2>&1 || fail "systemd is missing."

say "1/4 Closing the lid will not put the computer to sleep..."
mkdir -p /etc/systemd/logind.conf.d
cat > /etc/systemd/logind.conf.d/vahti-lid.conf <<'EOF'
[Login]
HandleLidSwitch=ignore
HandleLidSwitchExternalPower=ignore
HandleLidSwitchDocked=ignore
EOF
systemctl restart systemd-logind || say "  Restart the computer for the lid setting to take effect."

say "2/4 Turning off sleep and hibernation..."
systemctl mask sleep.target suspend.target hibernate.target hybrid-sleep.target >/dev/null

say "3/4 Automatic security updates..."
if command -v apt-get >/dev/null 2>&1; then
	DEBIAN_FRONTEND=noninteractive apt-get install -y -q unattended-upgrades >/dev/null
	cat > /etc/apt/apt.conf.d/20auto-upgrades <<'EOF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
EOF
else
	say "  apt is missing, skipping. Keep the system updated yourself."
fi

say "4/4 Firewall..."
if command -v ufw >/dev/null 2>&1; then
	for net in 10.0.0.0/8 172.16.0.0/12 192.168.0.0/16; do
		ufw allow from "$net" to any port 53 >/dev/null
		ufw allow from "$net" to any port 80,443 proto tcp >/dev/null
		ufw allow from "$net" to any port 22 proto tcp >/dev/null
	done
	ufw default deny incoming >/dev/null
	ufw default allow outgoing >/dev/null
	ufw --force enable >/dev/null
	say "  Firewall on: only DNS, the web interface and SSH from the home network are allowed in."
else
	say "  ufw is missing. If you want it: sudo apt install ufw and run this again."
fi

say ""
say "Done. The computer stays on with the lid closed. Next, install Vahti: sudo ./install.sh"
say "Tip: limit battery charging to 60-80 % in the BIOS if possible."
