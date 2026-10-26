#!/bin/sh
# Prepares an old laptop (Debian/Ubuntu) to run Vahti as an always-on server:
# keeps it awake with the lid closed, turns on automatic security updates and,
# if ufw is installed, allows only DNS, the web interface and SSH from the LAN.
set -eu

say()  { printf '%s\n' "$*"; }
fail() { printf 'Virhe: %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || fail "aja pääkäyttäjänä: sudo ./prepare-laptop.sh"
command -v systemctl >/dev/null 2>&1 || fail "systemd puuttuu."

say "1/4 Kannen sulkeminen ei laita konetta lepotilaan..."
mkdir -p /etc/systemd/logind.conf.d
cat > /etc/systemd/logind.conf.d/vahti-lid.conf <<'EOF'
[Login]
HandleLidSwitch=ignore
HandleLidSwitchExternalPower=ignore
HandleLidSwitchDocked=ignore
EOF
systemctl restart systemd-logind || say "  Uudelleenkäynnistä kone, jotta kansiasetus tulee voimaan."

say "2/4 Lepo- ja horrostila pois käytöstä..."
systemctl mask sleep.target suspend.target hibernate.target hybrid-sleep.target >/dev/null

say "3/4 Automaattiset tietoturvapäivitykset..."
if command -v apt-get >/dev/null 2>&1; then
	DEBIAN_FRONTEND=noninteractive apt-get install -y -q unattended-upgrades >/dev/null
	cat > /etc/apt/apt.conf.d/20auto-upgrades <<'EOF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
EOF
else
	say "  apt puuttuu, ohitetaan. Huolehdi päivityksistä itse."
fi

say "4/4 Palomuuri..."
if command -v ufw >/dev/null 2>&1; then
	for net in 10.0.0.0/8 172.16.0.0/12 192.168.0.0/16; do
		ufw allow from "$net" to any port 53 >/dev/null
		ufw allow from "$net" to any port 80,443 proto tcp >/dev/null
		ufw allow from "$net" to any port 22 proto tcp >/dev/null
	done
	ufw default deny incoming >/dev/null
	ufw default allow outgoing >/dev/null
	ufw --force enable >/dev/null
	say "  Palomuuri päällä: sisään vain DNS, hallintasivu ja SSH kotiverkosta."
else
	say "  ufw puuttuu. Asenna halutessasi: sudo apt install ufw ja aja tämä uudelleen."
fi

say ""
say "Valmis. Kone pysyy päällä kansi kiinni. Asenna seuraavaksi Vahti: sudo ./install.sh"
say "Vinkki: rajoita akun lataus 60-80 %:iin BIOSista, jos mahdollista."
