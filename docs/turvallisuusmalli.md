# Vahdin turvallisuusmalli

Tämä dokumentti kertoo, mitä vastaan Vahti suojaa, miten se on rakennettu ja mitä se ei pysty tekemään. Kirjoitamme rajoitukset auki, jotta käyttäjä tietää, mihin voi luottaa.

## Mitä suojataan

1. **Kotiverkon data:** se, mihin palveluihin laitteet ottavat yhteyttä, ja estettyjen pyyntöjen pysyminen kotona.
2. **Vahti itse:** hallintasivu, säännöt ja salasana. Murrettu Vahti voisi päästää kaiken läpi tai ohjata liikenteen väärään paikkaan.
3. **Internet:** Vahtia ei saa voida käyttää hyökkäyksiin muita vastaan (avoin nimipalvelin).

## Uhat ja suojaukset

| Uhka | Suojaus |
|---|---|
| Hyökkääjä internetistä | Vahti vastaa vain kotiverkon yksityisiin osoitteisiin, sekä nimipalvelussa että hallintasivulla. Muut paketit hylätään hiljaa. Porttia ei avata reitittimeen. |
| Avoimen nimipalvelimen väärinkäyttö | Ulkopuolisiin kyselyihin ei vastata. ANY- ja vyöhykesiirtokyselyt hylätään. Kyselyt rajoitetaan laitekohtaisesti (oletus 50/s). |
| Haitallinen verkkosivu kotikoneen selaimessa | Hallintasivu hyväksyy vain omat nimensä (DNS rebinding), muutokset vaativat CSRF-tunnisteen, ja Origin-otsake tarkistetaan. Evästeet ovat `SameSite=Strict`, `HttpOnly` ja `Secure`. Sisältöturvakäytäntö estää ulkoiset ja upotetut skriptit, ja sivua ei voi kehystää. Lisäksi nimipalvelu estää julkisia nimiä osoittamasta kotiverkon osoitteisiin. |
| Salasanan arvaus kotiverkosta | Argon2id-tiiviste, vähintään 12 merkin salasana, kasvavat viiveet virheellisten yritysten jälkeen sekä laitekohtainen että koko laitteen yhteinen yritysraja. |
| Haitallinen data verkosta (laitenimet, verkkotunnukset) | Käyttöliittymä rakentaa sivun tekstisolmuina, ei HTML:nä, joten nimet eivät voi sisältää koodia. Syötteet tarkistetaan tiukasti, ja tuntemattomat kentät hylätään. Nimipalvelimen käsittely on sumutestattu (fuzz). |
| Murto Vahdin ohjelmaan | Go on muistiturvallinen kieli. Palvelu toimii omana käyttäjänään ilman pääkäyttäjän oikeuksia, ja sillä on vain oikeus käyttää porttia 53, 80 ja 443. systemd eristää sen: vain datakansioon voi kirjoittaa, ja järjestelmäkutsuja, verkkoperheitä ja nimiavaruuksia on rajoitettu. |
| Väärennetty julkaisu | Julkaisut rakennetaan GitHub Actionsissa, ja niille luodaan allekirjoitettu alkuperätodistus (Sigstore), jonka voi tarkistaa komennolla `gh attestation verify`. Mukana on SHA256-tarkistussummat ja ohjelmistoluettelo (SBOM). Actionsit on kiinnitetty commit-tunnisteisiin. |
| Haavoittuva riippuvuus | Riippuvuuksia on kolme. govulncheck ja CodeQL ajetaan jokaisessa muutoksessa, ja Dependabot ehdottaa päivitykset. |
| Kaatuminen tai jumittuminen | systemd käynnistää palvelun uudelleen. Vahtikoira tarkistaa 15 sekunnin välein, että nimipalvelu oikeasti vastaa. Tila kirjoitetaan levylle atomisesti, ja edellinen versio säilytetään, joten sähkökatko ei riko asetuksia. |
| Salakuuntelu matkalla nimipalvelimelle | Sallitut kyselyt välitetään salattuna (DNS over TLS) ja palvelimen varmenne tarkistetaan. |

## Yksityisyys suunnittelussa

- Loki on vain muistissa, eikä sitä kirjoiteta levylle. Uudelleenkäynnistys tyhjentää sen.
- Järjestelmälokiin (journal) ei kirjoiteta kyselyitä, vain virheet ja kirjautumisyritykset.
- Laitteiden aliverkkotietoa (EDNS Client Subnet) ei välitetä eteenpäin.
- Kotiverkon nimiä (`.lan`, `.home.arpa`, yksityisten osoitteiden käänteishaut) ei lähetetä internetiin.
- Vahti ei tarkista päivityksiä, lähetä tilastoja eikä ota yhteyttä kehittäjiin.

## Rajoitukset

- **Salattua sisältöä ei nähdä.** Vahti tietää, että laite ottaa yhteyttä palveluun X, mutta ei näe, mitä se lähettää. Jos sama palvelin hoitaa sekä toiminnan että datankeruun, valittavana on vain molempien salliminen tai molempien estäminen.
- **Kovakoodatut osoitteet.** Laite, joka ottaa yhteyttä suoraan IP-osoitteeseen tai käyttää omaa salattua nimipalveluaan tuntemattomaan palvelimeen, ohittaa DNS-suodatuksen. Tämä korjataan versiossa, jossa Vahti toimii myös palomuurina.
- **Itse allekirjoitettu varmenne.** Selain varoittaa ensimmäisellä kerralla. Varmista sormenjälki asennusohjelman tulosteesta.
- **Kotiverkon laitteet ovat luotettuja verkkotasolla.** Kuka tahansa kotiverkossa voi käyttää nimipalvelua ja yrittää kirjautua hallintasivulle. Salasana ja yritysrajat suojaavat hallintaa.
- **Nimipalvelun tarjoaja näkee sallitut kyselyt.** Oletuksena se on Quad9 (Sveitsi, voittoa tavoittelematon, ei tallenna IP-osoitteita). Tarjoajan voi vaihtaa asetuksista tai asentaa oman rekursiivisen nimipalvelimen, kuten Unboundin.
- **Ulkoista tietoturvatarkastusta ei ole vielä tehty.** Se kannattaa teettää ennen versiota 1.0.
