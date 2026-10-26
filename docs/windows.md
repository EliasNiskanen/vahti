# Kokeellinen Windows-tila

Windows-tilassa Vahti suodattaa **vain sen tietokoneen** liikenteen, jolla se on käynnissä. Näet esimerkiksi Windowsin oman telemetrian sekä selaimen ja ohjelmien seurantapalvelut, ja voit estää niitä. Muiden kodin laitteiden suojaamiseen tarvitaan Vahti Raspberry Pi:llä tai vanhalla kannettavalla (ks. [README](../README.md#laitteisto)).

Tila on kokeellinen: sitä ei ole vielä testattu laajasti oikeilla Windows-koneilla. Kerro, jos jokin ei toimi.

## Käyttöönotto

Pura `vahti_<versio>_windows_amd64.zip` esimerkiksi kansioon `C:\Vahti`.

**1. Käynnistä Vahti.** Avaa PowerShell kansiossa (Shift + hiiren oikea painike kansion tyhjässä kohdassa → *Avaa PowerShell-ikkuna tässä*) ja aja:

```powershell
.\vahti.exe local
```

Pidä ikkuna auki. Vahti kuuntelee vain tätä konetta (127.0.0.1), joten mikään ei näy verkkoon eikä Windowsin palomuuri kysy lupaa.

**2. Ota Vahti käyttöön ja aseta salasana.** Avaa selaimessa http://127.0.0.1:8080 ja syötä PowerShelliin tulostunut käyttöönottokoodi.

**3. Lataa estolistat (suositeltava).** Toisessa PowerShell-ikkunassa:

```powershell
.\vahti.exe update-lists -local
```

Vahti ottaa listat käyttöön puolen minuutin kuluessa.

**4. Ohjaa koneen nimikyselyt Vahdille.** Avaa PowerShell **järjestelmänvalvojana** (Käynnistä → kirjoita PowerShell → *Suorita järjestelmänvalvojana*), siirry kansioon ja aja:

```powershell
cd C:\Vahti
powershell -ExecutionPolicy Bypass -File .\kayta-vahtia.ps1
```

Skripti tarkistaa ensin, että Vahti vastaa, ja tallentaa nykyiset DNS-asetukset. Sen jälkeen se asettaa Vahdin koneen ainoaksi nimipalvelimeksi.

**5. Kytke selaimen oma salattu DNS pois.** Chromessa ja Edgessä: Asetukset → Tietosuoja ja suojaus → *Käytä suojattua DNS:ää* → pois. Muuten selain voi ohittaa Vahdin. Firefoxissa Vahti kytkee tämän pois automaattisesti.

Nyt Vahdin Yleiskuva-näkymässä näkyy laite *Tämä tietokone* ja sen oikea liikenne.

## Lopetus

**Palauta DNS-asetukset ennen kuin suljet Vahdin.** Muuten koneen internetyhteys lakkaa toimimasta, koska nimikyselyihin ei vastaa kukaan.

```powershell
powershell -ExecutionPolicy Bypass -File .\palauta-dns.ps1
```

Sulje sen jälkeen Vahti painamalla Ctrl+C sen ikkunassa.

**Jos netti lakkaa toimimasta** (esim. suljit Vahdin ensin tai kone käynnistyi uudelleen), aja `palauta-dns.ps1` järjestelmänvalvojana. Se palauttaa alkuperäiset asetukset. Jos varmuuskopiota ei löydy, se asettaa DNS:n automaattiseksi.

## Hyvä tietää

- **Kotiverkon laitenimet:** Jos käytät nimiä kuten `tulostin.lan` tai `fritz.box`, käynnistä Vahti reitittimen osoitteella, esim. `.\vahti.exe local -router 192.168.1.1`.
- **Portti 53 varattu:** Jos Vahti ilmoittaa, että portti 53 on käytössä, jokin muu ohjelma toimii jo nimipalvelimena, esim. Internet-yhteyden jakaminen, Hyper-V tai WSL. Näet ohjelman komennolla `Get-NetUDPEndpoint -LocalPort 53`.
- **VPN-ohjelmat** asettavat usein oman nimipalvelimensa. Kun VPN on päällä, liikenne voi ohittaa Vahdin.
- **SmartScreen** voi varoittaa, koska `vahti.exe` ei ole allekirjoitettu Microsoftin varmenteella. Julkaisun aitouden voi tarkistaa komennolla `gh attestation verify vahti_*_windows_amd64.zip --repo EliasNiskanen/vahti`.
- **Tiedot** tallentuvat kansioon `%LOCALAPPDATA%\Vahti`. Loki on vain muistissa, kuten Linux-versiossa.
- **Windows-tilan rajoitukset:** Vahti ei käynnisty automaattisesti eikä käynnisty uudelleen kaatuessaan, eikä sitä ole eristetty samalla tavalla kuin Linux-palvelua. Siksi se sopii kokeiluun ja oman koneen tarkasteluun. Jatkuvaan suojaukseen käytä Linux-laitetta.
