#!/bin/sh
# Vahti installer for Raspberry Pi OS, Debian and Ubuntu (systemd).
# Run from an extracted release folder:  sudo ./install.sh
set -eu

PREFIX=/usr/local
CONF=/etc/vahti
DATA=/var/lib/vahti
HERE=$(cd "$(dirname "$0")" && pwd)

say()  { printf '%s\n' "$*"; }
fail() { printf 'Virhe: %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || fail "aja asennus pääkäyttäjänä: sudo ./install.sh"
command -v systemctl >/dev/null 2>&1 || fail "systemd puuttuu. Vahti tarvitsee systemd:n."

BIN="$HERE/vahti"
[ -f "$BIN" ] || fail "vahti-ohjelmaa ei löytynyt kansiosta $HERE"

# The downloaded archive is verified before extraction (see README: sha256sum and gh attestation verify).
"$BIN" version >/dev/null 2>&1 || fail "ohjelma ei käynnisty tällä laitteella. Tarkista, että latasit oikean arkkitehtuurin ($(uname -m))."

# Something else on port 53 (systemd-resolved, dnsmasq, Pi-hole) would block Vahti.
if command -v ss >/dev/null 2>&1; then
	busy=$(ss -H -lunp 'sport = :53' 2>/dev/null | grep -v vahti || true)
	if [ -n "$busy" ]; then
		say ""
		say "Portti 53 on jo toisen ohjelman käytössä:"
		say "$busy"
		if printf '%s' "$busy" | grep -q systemd-resolve; then
			say "Poista systemd-resolvedin paikallinen kuuntelija:"
			say "  sudo mkdir -p /etc/systemd/resolved.conf.d"
			say "  printf '[Resolve]\\nDNSStubListener=no\\n' | sudo tee /etc/systemd/resolved.conf.d/vahti.conf"
			say "  sudo systemctl restart systemd-resolved"
		fi
		fail "vapauta portti 53 ja aja asennus uudelleen."
	fi
fi

say "Luodaan palvelukäyttäjä vahti..."
if ! id vahti >/dev/null 2>&1; then
	useradd --system --no-create-home --home-dir "$DATA" --shell /usr/sbin/nologin vahti
fi

say "Asennetaan ohjelma..."
install -m 0755 -o root -g root "$BIN" "$PREFIX/bin/vahti"
install -d -m 0755 "$PREFIX/share/doc/vahti"
[ -f "$HERE/README.md" ] && install -m 0644 "$HERE/README.md" "$PREFIX/share/doc/vahti/README.md"

install -d -m 0755 "$CONF"
if [ ! -f "$CONF/config.json" ]; then
	install -m 0644 "$HERE/config.json" "$CONF/config.json"
else
	say "Säilytetään nykyinen $CONF/config.json"
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
systemctl is-active --quiet vahti.service || { journalctl -u vahti -n 20 --no-pager; fail "Vahti ei käynnistynyt. Loki yllä."; }

say ""
printf 'Ladataanko estolistat nyt ja päivitetäänkö ne kerran viikossa? Tämä on ainoa asia, jonka Vahti hakee internetistä. [K/e] '
read -r ans || ans=""
case "$ans" in
	[eEnN]*) say "Listoja ei ladattu. Voit ladata ne myöhemmin: sudo systemctl start vahti-update-lists" ;;
	*) systemctl enable --now vahti-update-lists.timer >/dev/null
	   systemctl start vahti-update-lists.service || say "Listojen lataus epäonnistui. Katso: journalctl -u vahti-update-lists" ;;
esac

IP=$(hostname -I 2>/dev/null | awk '{print $1}')
FP=$(journalctl -u vahti --no-pager -o cat 2>/dev/null | grep -o 'sha256=[0-9A-F:]*' | tail -1 | cut -d= -f2)
say ""
say "Vahti on asennettu ja käynnissä."
say ""
say "1. Avaa selaimessa:  https://$IP"
say "   Selain varoittaa itse allekirjoitetusta varmenteesta. Varmista, että sormenjälki on:"
say "   $FP"
if [ -f "$DATA/setup-code" ]; then
	say "2. Syötä käyttöönottokoodi:  $(cat "$DATA/setup-code")"
fi
say "3. Aseta reitittimen DHCP-asetuksiin DNS-palvelimeksi $IP (vain tämä yksi osoite)."
say "   Anna Raspberry Pi:lle myös kiinteä osoite reitittimen asetuksista."
