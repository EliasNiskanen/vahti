# Vahti vanhalla kannettavalla

Vanha kannettava sopii Vahdille hyvin. Tehoa on yleensä moninkertaisesti tarpeeseen nähden, ja akku toimii varavirtalähteenä: lyhyessä sähkökatkossa Vahti pysyy päällä, ja kun reititin palaa, nimipalvelu toimii heti.

## Sopiiko oma koneeni?

- 64-bittinen prosessori (lähes kaikki vuoden 2008 jälkeen valmistetut)
- vähintään 1 Gt muistia
- verkkoportti tai USB-verkkosovitin
- Linux asennettavissa (Debian 12 tai Ubuntu Server 24.04)

Windows-kannettava ei käy sellaisenaan, koska Vahti toimii palveluna vain Linuxilla. Asennus tyhjentää koneen, joten varmuuskopioi ensin tiedostot, jotka haluat säilyttää.

## 1. Asenna Linux

Suosittelen kevyttä asennusta ilman työpöytää, koska kone toimii jatkossa palvelimena:

- **Debian 12**: valitse asennuksessa ohjelmistovalinnoista vain *SSH server* ja *standard system utilities*, ei työpöytäympäristöä.
- **Ubuntu Server 24.04 LTS**: valitse asennuksessa *Install OpenSSH server*.

Tee USB-asennustikku esimerkiksi Raspberry Pi Imagerilla tai balenaEtcherillä ja käynnistä kone tikulta.

Asennuksen jälkeen voit hallita konetta pöytäkoneeltasi SSH:lla (`ssh käyttäjä@koneen-ip`). PowerShellissä tämä onnistuu suoraan Windowsista.

## 2. Valmistele kone palvelimeksi

Aja Vahdin julkaisupaketin mukana tuleva valmistelukomento:

```sh
sudo ./prepare-laptop.sh
```

Se tekee seuraavat asiat:

- **Kannen sulkeminen ei laita konetta lepotilaan.** Voit pitää kannen kiinni.
- **Lepo- ja horrostila poistetaan käytöstä.**
- **Tietoturvapäivitykset asentuvat automaattisesti** (unattended-upgrades).
- **Palomuuri päälle** (ufw), jos se on asennettuna: sisään sallitaan vain nimipalvelu, hallintasivu ja SSH, ja vain kotiverkosta.

Voit tehdä samat asiat myös käsin. Komennot ovat skriptissä luettavassa muodossa.

## 3. Asenna Vahti

Lataa julkaisusivulta paketti `linux_amd64` ja jatka [README-tiedoston asennusohjeesta](../README.md#asennus).

## Hyvä tietää

**Akku.** Akku, joka on jatkuvasti 100 %:n varauksessa ja lämpimänä, vanhenee ja voi pahimmillaan turvota. Monen kannettavan BIOS-asetuksissa tai valmistajan työkalussa voi rajoittaa latauksen 60–80 %:iin. Tee se, jos mahdollista. Tarkista akku silmämääräisesti muutaman kuukauden välein: jos kotelo pullistuu tai kosketuslevy nousee, irrota akku ja käytä konetta pelkällä laturilla.

**Lämpö.** Älä sulje konetta kaappiin tai pehmeälle alustalle. Puhdista tuuletusaukot pölystä.

**Wifi.** Käytä verkkokaapelia. Jos koneessa ei ole verkkoporttia, USB-verkkosovitin on parempi vaihtoehto kuin wifi.

**Sähkönkulutus.** Kannettava kuluttaa levossa tyypillisesti noin 5–15 W, mikä on huomattavasti vähemmän kuin pöytäkone. Näytön sammuminen hoituu, kun kansi on kiinni.

**BIOS-asetukset (vapaaehtoinen).** Ota käyttöön automaattinen käynnistys sähkökatkon jälkeen (*AC Power Recovery* tai *Power On After Power Loss*), jos koneessa sellainen on, ja aseta BIOS-salasana.

**Myöhemmin reititintilaan.** Kun Vahti saa reititintilan, kannettava sijoitetaan modeemin ja reitittimen väliin. Tarvitset silloin koneen oman verkkoportin lisäksi USB-verkkosovittimen (mielellään USB 3 ja gigabitti).
