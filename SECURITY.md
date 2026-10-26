# Tietoturvakäytäntö

## Haavoittuvuuksista ilmoittaminen

Älä ilmoita haavoittuvuuksista julkisessa issuessa.

Käytä GitHubin yksityistä ilmoituskanavaa: repositorion **Security**-välilehti → **Report a vulnerability**. Viesti näkyy vain ylläpitäjille.

Kerro ilmoituksessa:

- mitä löysit ja mihin se vaikuttaa
- miten sen voi toistaa (versio, asetukset, pyynnöt)
- jos mahdollista, ehdotus korjaukseksi

Vahti on harrastusprojekti, jota ylläpidetään vapaa-ajalla, joten tarkkoja aikoja en voi luvata. Pyrin seuraavaan:

- vastaus viikon kuluessa
- vakavat ongelmat korjataan ennen muita töitä
- korjauksen jälkeen julkaistaan tiedote (GitHub Security Advisory), ja ilmoittajaa kiitetään, jos hän niin haluaa

Pyydämme, ettet julkaise tietoja ennen korjausta etkä testaa muiden ihmisten laitteita.

## Tuetut versiot

Tietoturvakorjaukset tehdään uusimpaan julkaisuun. Päivitä aina uusimpaan versioon.

## Mitä pidetään haavoittuvuutena

Esimerkiksi:

- hallintasivulle pääsy ilman salasanaa tai CSRF-suojauksen ohitus
- Vahdin saaminen vastaamaan kyselyihin kotiverkon ulkopuolelta
- estetyn kyselyn pääsy kotiverkon ulkopuolelle
- tiedostojen lukeminen tai kirjoittaminen datakansion ulkopuolelle
- palvelunesto, jonka yksittäinen kotiverkon laite voi aiheuttaa helposti
- haitallisen koodin suorittaminen hallintasivulla (XSS)

Uhkamalli ja tunnetut rajoitukset: [docs/turvallisuusmalli.md](docs/turvallisuusmalli.md).
