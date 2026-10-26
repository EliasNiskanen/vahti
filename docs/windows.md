# Kokeellinen Windows-tila

Windows-tilassa Vahti suodattaa **vain sen tietokoneen** liikenteen, jolla se on käynnissä. Näet esimerkiksi Windowsin oman telemetrian sekä selaimen ja ohjelmien seurantapalvelut, ja voit estää niitä. Muiden kodin laitteiden suojaamiseen tarvitaan Vahti Raspberry Pi:llä tai vanhalla kannettavalla (ks. [README](../README.md#laitteisto)).

Tila on kokeellinen: sitä ei ole vielä testattu laajasti oikeilla Windows-koneilla. Kerro, jos jokin ei toimi.

## Käyttöönotto

Pura `vahti_<versio>_windows_amd64.zip` esimerkiksi kansioon `C:\Vahti`. Avaa PowerShell kansiossa (Shift + hiiren oikea painike kansion tyhjässä kohdassa → *Avaa PowerShell-ikkuna tässä*).

**1. Käynnistä Vahti:**

```powershell
powershell -ExecutionPolicy Bypass -File .\kaynnista-vahti.ps1
```

Vahti avautuu omaan ikkunaansa. Kun se vastaa, Windows kysyy lupaa DNS-asetusten muuttamiseen. Hyväksy, niin koneen nimikyselyt ohjataan Vahdille.

**2. Ota Vahti käyttöön ja aseta salasana.** Avaa selaimessa http://127.0.0.1:8080 ja syötä Vahdin ikkunaan tulostunut käyttöönottokoodi. Tämä tehdään vain ensimmäisellä kerralla.

**3. Lataa estolistat (suositeltava).** Tavallisessa PowerShell-ikkunassa:

```powershell
.\vahti.exe update-lists -local
```

Vahti ottaa listat käyttöön puolen minuutin kuluessa.

**4. Kytke selaimen oma salattu DNS pois.** Chromessa ja Edgessä: Asetukset → Tietosuoja ja suojaus → *Käytä suojattua DNS:ää* → pois. Muuten selain voi ohittaa Vahdin. Firefoxissa Vahti kytkee tämän pois automaattisesti.

Nyt Vahdin Yleiskuva-näkymässä näkyy laite *Tämä tietokone* ja sen oikea liikenne.

## Lopetus

Sulje Vahdin ikkuna tai paina siinä Ctrl+C. **DNS-asetukset palautetaan automaattisesti** muutaman sekunnin kuluessa.

## Miten DNS-asetusten palautus toimii

Vahti itse toimii ilman järjestelmänvalvojan oikeuksia, joten se ei voi muuttaa koneen asetuksia. DNS-asetuksista huolehtii erillinen **vartija** (`dns-vartija.ps1`), jolle annat luvan käynnistyksen yhteydessä. Vartija:

1. tallentaa koneen alkuperäiset DNS-asetukset, myös käsin asetetut osoitteet
2. ohjaa nimikyselyt Vahdille
3. palauttaa alkuperäiset asetukset heti, kun Vahti suljetaan, kaatuu tai lakkaa vastaamasta 15 sekunniksi.

Jos kone sammuu tai sähköt katkeavat Vahdin ollessa käynnissä, asetukset palautetaan **seuraavassa käynnistyksessä**. Tätä varten vartija luo Windowsin tehtävien ajoitukseen tehtävän *Vahti DNS-palautus*. Tehtävä poistuu, kun asetukset on palautettu.

Varmuuskopio, palautusskripti ja tapahtumaloki (`dns.log`) ovat kansiossa `%ProgramData%\Vahti`. Kansiota voivat muuttaa vain järjestelmä ja järjestelmänvalvojat, jotta kukaan ei voi muuttaa käynnistyksessä ajettavaa skriptiä.

## Tarkista, että asetukset ovat normaalit

```powershell
powershell -ExecutionPolicy Bypass -File .\tarkista-dns.ps1
```

Skripti näyttää jokaisen verkkosovittimen nimipalvelimet ja kertoo, onko Vahti käynnissä, ohjautuvatko kyselyt sille ja toimiiko nimipalvelu. Se ei muuta mitään eikä vaadi järjestelmänvalvojan oikeuksia.

## Jos netti ei toimi

Aja järjestelmänvalvojana:

```powershell
powershell -ExecutionPolicy Bypass -File .\palauta-dns.ps1
```

Skripti palauttaa asetukset varmuuskopiosta. Jos varmuuskopiota ei ole, se asettaa Vahtiin osoittavat sovittimet automaattisiksi. Jos skriptiä ei ole käsillä: *Asetukset → Verkko ja Internet → Wi-Fi tai Ethernet → yhteyden ominaisuudet → DNS-palvelimen määritys → Muokkaa → Automaattinen (DHCP)*.

## Hyvä tietää

- **Kotiverkon laitenimet:** Jos käytät nimiä kuten `tulostin.lan` tai `fritz.box`, käynnistä Vahti reitittimen osoitteella: `.\kaynnista-vahti.ps1 -Router 192.168.1.1`.
- **Portti 53 varattu:** Jos Vahti ilmoittaa, että portti 53 on käytössä, jokin muu ohjelma toimii jo nimipalvelimena, esim. Internet-yhteyden jakaminen, Hyper-V tai WSL. Näet ohjelman komennolla `Get-NetUDPEndpoint -LocalPort 53`.
- **VPN-ohjelmat** asettavat usein oman nimipalvelimensa. Kun VPN on päällä, liikenne voi ohittaa Vahdin.
- **SmartScreen** voi varoittaa, koska `vahti.exe` ei ole allekirjoitettu Microsoftin varmenteella. Julkaisun aitouden voi tarkistaa komennolla `gh attestation verify vahti_*_windows_amd64.zip --repo EliasNiskanen/vahti`.
- **Vahdin tiedot** (asetukset, salasanan tiiviste) tallentuvat kansioon `%LOCALAPPDATA%\Vahti`. Loki on vain muistissa, kuten Linux-versiossa.
- **Rajoitukset:** Vahti ei käynnisty automaattisesti Windowsin mukana, eikä sitä ole eristetty samalla tavalla kuin Linux-palvelua. Siksi Windows-tila sopii kokeiluun ja oman koneen tarkasteluun. Jatkuvaan koko kodin suojaukseen käytä Linux-laitetta.

## Skriptit

| Tiedosto | Tehtävä | Järjestelmänvalvoja |
|---|---|---|
| `kaynnista-vahti.ps1` | Käynnistää Vahdin ja vartijan. Suositeltu tapa. | Kysyy luvan |
| `tarkista-dns.ps1` | Näyttää DNS-asetusten tilan. Ei muuta mitään. | Ei |
| `palauta-dns.ps1` | Palauttaa asetukset käsin. | Kyllä |
| `kayta-vahtia.ps1` | Käynnistää vartijan jo käynnissä olevalle Vahdille. | Kyllä |
| `dns-vartija.ps1` | Vartija. Muut skriptit käynnistävät sen. | Kyllä |
| `VahtiDns.psm1` | Yhteiset toiminnot. | – |
