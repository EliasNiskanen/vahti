# Vahti

Vahti on kotiverkon yksityisyyssuodatin. Se toimii kotiverkon nimipalvelimena ja tarkistaa jokaisen pyynnön ennen kuin mitään lähtee kotoa. Mainonta, analytiikka, laitteiden telemetria ja sijaintia keräävät palvelut estetään oletuksena. Sinä päätät laitekohtaisesti, mitä saa jakaa ja kenelle.

Vahti on täysin paikallinen. Sillä ei ole pilvipalvelua, käyttäjätiliä eikä etähallintaa, eikä se lähetä tietoja kenellekään. Ainoat yhteydet ulos ovat sallittujen nimikyselyjen välittäminen salattuna nimipalvelimelle sekä estolistojen lataus, kun pyydät sitä.

## Mitä Vahti tekee

- **Estää seurannan ennen kuin yhteys avataan.** Estetty pyyntö ei koskaan lähde kotiverkosta.
- **Tunnistaa laitteet** ja näyttää jokaisesta, mihin se on yrittänyt ottaa yhteyttä.
- **Kolme tilaa laitteille:**
  - *Avoin*: tunnetut seurantapalvelut estetään, kaikki muu toimii. Sopii puhelimille ja tietokoneille.
  - *Opettelee*: laite toimii vapaasti, ja Vahti tallentaa sen tarvitsemat palvelut. Opettelun jälkeen laite lukitaan.
  - *Lukittu*: vain sallitut kohteet toimivat. Uudet kohteet estetään ja tuodaan sinulle päätettäviksi. Sopii älylaitteille.
- **Hyväksymisjono:** näet, mihin kohde liittyy (omistaja, saman sivuston muut osoitteet, muut laitteet), ja voit sallia sen kerran, tunniksi tai pysyvästi tai estää sen.
- **Jakaminen jälkikäteen:** voit sallia estetyn kohteen määräajaksi laitteen sivulta.
- **Naamioidun seurannan tunnistus:** jos sivuston oma osoite ohjautuu (CNAME) seurantapalveluun, se estetään.
- **Ohitusyritysten esto:** selainten oma salattu DNS ja iCloud Private Relay kytketään pois, ja tunnetut salatut DNS-palvelut estetään.
- **Ei sijaintivihjeitä:** Vahti ei välitä laitteiden aliverkkotietoa (EDNS Client Subnet) eteenpäin, ja kotiverkon sisäiset nimet eivät koskaan lähde internetiin.

Mitä Vahti ei tee: se ei näe salatun liikenteen sisältöä eikä avaa salausta. Estot tehdään sen perusteella, *mihin* laite ottaa yhteyttä. Lisätietoja: [docs/turvallisuusmalli.md](docs/turvallisuusmalli.md).

## Laitteisto

Vahti on kevyt: ohjelma vie noin 8 Mt ja käyttää tyypillisesti alle 100 Mt muistia. Lähes mikä tahansa jatkuvasti päällä oleva Linux-kone käy.

### Vähimmäisvaatimukset

| | Vaatimus |
|---|---|
| Prosessori | 64-bittinen ARM (arm64), 32-bittinen ARMv7 tai x86-64 (amd64) |
| Muisti | 512 Mt, suositus 1 Gt tai enemmän |
| Tallennustila | 1 Gt vapaata. Muistikortti riittää, SSD on kestävämpi. |
| Verkko | Verkkokaapeli reitittimeen. Wifi toimii, mutta on epävarmempi, ja silloin koko kodin netti riippuu wifin toimivuudesta. |
| Käyttöjärjestelmä | Linux ja systemd: Raspberry Pi OS (Bookworm tai uudempi), Debian 12+ tai Ubuntu 22.04+ |
| Käyttö | Päällä ympäri vuorokauden. Kun Vahti on kodin nimipalvelin, netti ei toimi ilman sitä. |

Windows ja macOS kelpaavat vain esittelytilaan (`vahti demo`). Palveluna Vahti toimii Linuxilla.

### Sopivat laitteet

| Laite | Sopii | Huomioita |
|---|---|---|
| **Raspberry Pi 4 tai 5** | Erinomaisesti | Hiljainen ja vähävirtainen (noin 3–7 W). Suositeltu vaihtoehto. |
| Raspberry Pi 3B+ | Hyvin | Riittää tavalliseen kotiin. Asenna 64-bittinen käyttöjärjestelmä. |
| **Vanha kannettava** | Hyvin | Ilmainen, jos sellainen on jo. Akku toimii varavirtalähteenä sähkökatkossa. Ks. [docs/vanha-kannettava.md](docs/vanha-kannettava.md). |
| Minitietokone (esim. Intel N100) | Erinomaisesti | Hyvä valinta tulevaan reititintilaan, varsinkin malli, jossa on kaksi verkkoporttia. |
| Vanha pöytäkone | Toimii | Kuluttaa enemmän sähköä (usein 30–60 W), mikä näkyy ympärivuotisessa käytössä. |
| Raspberry Pi Zero 2 W | Ei suositella | Vain wifi, ei verkkoporttia. |

### Kytkentä

Tässä versiossa Vahti toimii kotiverkon nimipalvelimena reitittimen *rinnalla*:

```
Internet ── modeemi ── reititin ──┬── puhelimet, TV, tietokoneet
                                  │      (kysyvät nimet Vahdilta)
                                  └── Vahti (kaapelilla)
```

Laitteen tarvitsee siis vain yhden verkkoyhteyden. Reititin kertoo laitteille, että nimipalvelin on Vahti.

Myöhemmin tuleva reititintila sijoittaa Vahdin modeemin ja kotiverkon *väliin*, jolloin se näkee ja hallitsee kaiken liikenteen. Siihen tarvitaan **kaksi verkkoporttia**: koneen oma portti ja esimerkiksi USB-verkkosovitin. Raspberry Pi 5:llä ja vanhalla kannettavalla tämä onnistuu USB-sovittimella, ja kahden portin minitietokone on siihen valmiiksi sopiva.

## Kokeile omalla koneella

Esittelytilassa liikenne on simuloitua, mutta kaikki kulkee oikean suodatusmoottorin läpi. Internetiin ei oteta yhteyttä.

```sh
go build -o vahti ./cmd/vahti
./vahti demo
```

Avaa http://127.0.0.1:8080 ja syötä päätteeseen tulostunut käyttöönottokoodi. Esittelytilan nimipalvelinta voi kokeilla myös suoraan: `dig @127.0.0.1 -p 5353 doubleclick.net`.

## Asennus

Ohje on kirjoitettu Raspberry Pi:lle, mutta se toimii samoin kaikilla Linux-koneilla. Vanhaa kannettavaa käyttäessäsi tee ensin [kannettavan valmistelut](docs/vanha-kannettava.md). Kytke laite reitittimeen verkkokaapelilla.

1. Lataa [julkaisusivulta](https://github.com/EliasNiskanen/vahti/releases) laitteellesi sopiva paketti ja tiedosto `SHA256SUMS`:
   - Raspberry Pi 3/4/5 (64-bittinen käyttöjärjestelmä): `linux_arm64`
   - Raspberry Pi, 32-bittinen käyttöjärjestelmä: `linux_armv7`
   - Kannettava, minitietokone tai pöytäkone: `linux_amd64`
2. Tarkista, että lataus on aito:
   ```sh
   sha256sum --check --ignore-missing SHA256SUMS
   gh attestation verify vahti_*_linux_*.tar.gz --repo EliasNiskanen/vahti
   ```
   Jälkimmäinen todistaa, että paketti on rakennettu GitHubissa tästä lähdekoodista.
3. Pura ja asenna:
   ```sh
   tar xzf vahti_*_linux_*.tar.gz
   cd vahti_*_linux_*/
   sudo ./install.sh
   ```
4. Avaa asennuksen lopussa näkyvä osoite selaimessa ja syötä käyttöönottokoodi. Selain varoittaa itse allekirjoitetusta varmenteesta. Varmista, että sormenjälki on sama kuin asennusohjelman tulostama, ja hyväksy se.
5. Anna reitittimen asetuksista Vahti-laitteelle kiinteä IP-osoite.
6. Aseta reitittimen DHCP-asetuksissa **DNS-palvelimeksi Vahti-laitteen osoite**. Laita vain tämä yksi osoite, sillä varapalvelin ohittaisi Vahdin.
7. Kytke laitteiden wifi hetkeksi pois ja takaisin, jotta ne saavat uuden asetuksen.

### Hyvä tietää

- **IPv6:** Jos reititin jakaa IPv6-osoitteita, se voi mainostaa laitteille omaa nimipalvelintaan IPv6:n kautta. Kytke reitittimestä IPv6-DNS-mainonta pois tai aseta Vahdin IPv6-osoite.
- **Paikalliset laitenimet:** Jos haluat Vahdin näyttävän reitittimen tuntemat laitenimet, aseta tiedostossa `/etc/vahti/config.json` kohtaan `localResolver` reitittimen osoite, esim. `"192.168.1.1:53"`.
- **Jos Vahti pysähtyy, netti pysähtyy.** Tämä on tarkoituksellista: suodatusta ei ohiteta huomaamatta. systemd käynnistää Vahdin automaattisesti uudelleen, jos se kaatuu tai nimipalvelu lakkaa vastaamasta 30 sekunniksi. Hätätilanteessa palauta reitittimen DNS-asetus.
- **Unohtunut salasana:** `sudo systemctl stop vahti && sudo vahti reset-admin && sudo systemctl start vahti`
- **Estolistat:** `sudo systemctl start vahti-update-lists` lataa listat heti. Asennusohjelma voi ottaa viikoittaisen päivityksen käyttöön.

## Asetukset

Asetukset ovat tiedostossa `/etc/vahti/config.json`. Tuntematon tai virheellinen asetus estää käynnistyksen ja kertoo virheen lokissa (`journalctl -u vahti`), jotta kirjoitusvirhe ei jää huomaamatta.

| Asetus | Merkitys |
|---|---|
| `dns.upstreams` | Nimipalvelimet sallituille kyselyille. Oletuksena Quad9 salattuna (DNS over TLS). Muoto: `tls://ip:853#nimi`. |
| `dns.allowedClients` | Osoitteet, joista kyselyihin vastataan. Oletuksena vain kotiverkon yksityiset osoitteet. |
| `dns.localResolver` | Reititin kotiverkon nimille (`.lan` ym.). Tyhjänä paikallisiin nimiin vastataan "ei löydy". |
| `dns.rebindProtection` | Estää julkisia nimiä osoittamasta kotiverkkoon (DNS rebinding). |
| `dns.rebindAllow` | Poikkeukset edelliseen, esim. `plex.direct`. |
| `web.hostnames` | Nimet, joilla hallintasivun voi avata. Muut nimet torjutaan. |
| `lists` | Ladattavat estolistat (vain https). |
| `logEntries` | Montako viimeisintä kyselyä pidetään muistissa. Lokia ei kirjoiteta levylle. |

## Kehittäminen

```sh
make test     # testit rinnakkaisuustarkistimella
make check    # testit, go vet ja muotoilu
make pi       # käännös Raspberry Pi:lle
```

Rakenne:

| Kansio | Sisältö |
|---|---|
| `cmd/vahti` | Ohjelman käynnistys ja komennot |
| `internal/dnsserver` | Suodattava nimipalvelin: välimuisti, salattu välitys, CNAME- ja rebinding-tarkistukset |
| `internal/engine` | Päätöslogiikka: laitteet, säännöt, jono, loki |
| `internal/intel` | Seurantapalveluiden tunnistus ja omistajatiedot |
| `internal/web` | Hallintasivu ja rajapinta: kirjautuminen, istunnot, CSRF-suojaus |
| `deploy` | Asennusohjelma ja eristetyt systemd-palvelut |

Riippuvuuksia on tarkoituksella vähän: [miekg/dns](https://github.com/miekg/dns) sekä Go-projektin omat `golang.org/x/crypto` ja `golang.org/x/net`.

## Tietoturva

Löysitkö haavoittuvuuden? Älä avaa julkista issueta, vaan katso [SECURITY.md](SECURITY.md).

## Lisenssi

Apache License 2.0. Ohjelma toimitetaan sellaisenaan ilman takuuta (ks. [LICENSE](LICENSE)).

Estolistoilla on omat lisenssinsä: StevenBlack/hosts (MIT) ja URLhaus (CC0). Jos lisäät listoja, tarkista niiden käyttöehdot. Osa seurantatietokannoista sallii vain ei-kaupallisen käytön.
