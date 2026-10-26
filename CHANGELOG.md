# Muutokset

Kaikki merkittävät muutokset kirjataan tähän tiedostoon. Muoto noudattaa [Keep a Changelog](https://keepachangelog.com/fi/1.1.0/) -käytäntöä, ja versiointi [semanttista versiointia](https://semver.org/lang/fi/).

## [Julkaisematon]

### Korjattu
- CI: staticcheckin huomautukset (virheviestien muoto ja käyttämätön funktio).
- CodeQL ajetaan vain julkisessa repositoriossa, koska yksityisessä se vaatii maksullisen lisäosan.

### Lisätty
- README: kehitysvaiheen ja harrastusprojektin tila sekä ohje muiden ihmisten liikenteen seuraamisesta.
- **Kokeellinen Windows-tila** (`vahti local`): suodattaa sen tietokoneen liikenteen, jolla Vahti on käynnissä. Kuuntelee vain tätä konetta (127.0.0.1 ja ::1) ja siirtyy pelkkään IPv4:ään, jos IPv6 ei ole käytössä.
- **DNS-asetusten automaattinen palautus Windowsissa:** `kaynnista-vahti.ps1` käynnistää Vahdin ja vartijan, joka ohjaa nimikyselyt Vahdille ja palauttaa alkuperäiset asetukset heti, kun Vahti suljetaan, kaatuu tai lakkaa vastaamasta. Jos kone sammuu kesken, ajastettu tehtävä palauttaa asetukset seuraavassa käynnistyksessä.
- `tarkista-dns.ps1` näyttää, ovatko koneen DNS-asetukset normaalit. Ei vaadi järjestelmänvalvojan oikeuksia.
- `palauta-dns.ps1` palauttaa asetukset käsin, myös ilman varmuuskopiota. Käsin asetetut DNS-osoitteet säilyvät.
- Varmuuskopio ja palautusskripti tallennetaan kansioon `%ProgramData%\Vahti`, jota vain järjestelmä ja järjestelmänvalvojat voivat muuttaa.
- Windows-skriptien palautuslogiikalle testi simuloiduilla verkkosovittimilla CI:hin.
- `vahti update-lists -local` lataa estolistat Windows-tilaa varten.
- `-router`-valinta kotiverkon laitenimiä (esim. `tulostin.lan`) varten Windows-tilassa.
- Estolistat otetaan käyttöön automaattisesti, kun niiden tiedostot muuttuvat. Uudelleenkäynnistystä ei tarvita.
- Käyttöliittymään ilmoitus, kun Vahti toimii paikallisessa tilassa.
- Windows-paketti (`windows_amd64.zip`) julkaisuihin, ja Windows-käännös ja PowerShell-skriptien tarkistus CI:hin.
- Ohjeet: [docs/windows.md](docs/windows.md) ja Windows-tila README-tiedostoon.
- README-tiedostoon varoitus DNS-asetusten muuttamisesta sekä ohje asetusten muuttamiseen ja palauttamiseen.

## [0.1.0] - 2026-10-26

Ensimmäinen versio.

### Lisätty
- Suodattava nimipalvelin: estää mainonnan, analytiikan, telemetrian, sijaintiseurannan, some-seurannan, ohitusyritykset ja haitalliset kohteet ennen kuin yhteys avataan.
- Laitekohtaiset tilat: avoin, opettelee ja lukittu.
- Hyväksymisjono, joka näyttää, mihin kohde liittyy: omistaja, saman sivuston muut osoitteet, muut laitteet ja CNAME-kohteet.
- Luvat kerran, tunniksi tai pysyvästi, sekä jakaminen jälkikäteen määräajaksi.
- Naamioidun seurannan (CNAME) tunnistus, DNS rebinding -suoja ja selainten oman salatun DNS:n esto.
- Salattu välitys nimipalvelimelle (DNS over TLS, oletuksena Quad9). Laitteiden aliverkkotietoa ei välitetä eteenpäin.
- Hallintasivu: kirjautuminen (Argon2id), CSRF- ja Origin-tarkistukset, Host-otsakkeen tarkistus, tiukka sisältöturvakäytäntö ja itse allekirjoitettu HTTPS-varmenne.
- Eristetty systemd-palvelu, vahtikoira ja atominen tilan tallennus.
- Asennus- ja poisto-ohjelmat Raspberry Pi OS:lle, Debianille ja Ubuntulle sekä valmistelukomento vanhalle kannettavalle.
- Esittelytila (`vahti demo`) simuloidulla liikenteellä.
- GitHub Actions: testit, staticcheck, govulncheck, CodeQL, sumutestaus ja allekirjoitetut julkaisut, joissa on alkuperätodistus ja ohjelmistoluettelo (SBOM).

[Julkaisematon]: https://github.com/EliasNiskanen/vahti/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/EliasNiskanen/vahti/releases/tag/v0.1.0
