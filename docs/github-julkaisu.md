# Vahdin julkaiseminen GitHubissa

## 1. Repositorio

Koodi on repositoriossa https://github.com/EliasNiskanen/vahti, ja Go-moduulin nimi on `github.com/EliasNiskanen/vahti`.

## Ennen kuin muutat repositorion julkiseksi

Tee nämä tässä järjestyksessä. Repositorion muuttaminen julkiseksi tapahtuu viimeisenä.

1. **Sähköpostiosoite piiloon.** GitHubissa: profiilikuva → *Settings → Emails*. Laita päälle **Keep my email addresses private** ja **Block command line pushes that expose my email**. Vahdin commit-historiassa on valmiiksi GitHubin piilo-osoite `104171824+EliasNiskanen@users.noreply.github.com`.
2. **Kaksivaiheinen tunnistautuminen.** *Settings → Password and authentication → Enable two-factor authentication.* Valitse todennussovellus tai pääsyavain (passkey) ja tallenna palautuskoodit turvaan.
3. **Muut kohdan 2 asetukset:** Dependabot, salaisuuksien skannaus ja Actions-oikeudet.
4. **Tavaramerkki.** Hae nimeä *Vahti* ennen laajempaa markkinointia:
   - EU ja kansalliset rekisterit yhdellä haulla: [TMview](https://www.tmdn.org/tmview/). Kirjoita hakuun `vahti` ja katso erityisesti luokat 9 (ohjelmistot) ja 42 (ohjelmistopalvelut).
   - Suomi: Patentti- ja rekisterihallituksen tavaramerkkihaku osoitteessa [prh.fi](https://www.prh.fi/).
   - Jos samassa luokassa löytyy voimassa oleva Vahti-merkki, harkitse nimen tarkennusta (esim. *Vahti DNS*) tai kysy neuvoa asiantuntijalta.
5. **CI vihreäksi.** Actions-välilehdellä viimeisimmän ajon pitää olla vihreä.
6. **Julkiseksi:** *Settings → General → Danger Zone → Change repository visibility → Make public.* CodeQL-skannaus käynnistyy tämän jälkeen automaattisesti.
7. **Heti julkaisun jälkeen** (ilmaisella tilillä nämä toimivat vain julkisissa repositorioissa):
   - **Yksityinen haavoittuvuusilmoitusten kanava:** *Settings → Advanced Security* (tai *Code security*) → **Private vulnerability reporting → Enable.**
   - **Pääkehityshaaran suojaus:** *Settings → Rules → Rulesets → New ruleset → New branch ruleset.* Nimeksi `main`, **Enforcement status: Active**, **Target branches → Add target → Include default branch**. Valitse **Restrict deletions** ja **Block force pushes** ja paina **Create**.

## 2. Repositorion asetukset (Settings)

Nämä ovat GitHubin asetuksia, joita ei voi tallentaa koodiin. Tee ne heti.

**Oma tili**
- [ ] Kaksivaiheinen tunnistautuminen päälle (Settings → Password and authentication). Mieluiten pääsyavain tai fyysinen turva-avain.

**Code security** (Settings → Code security)
- [ ] Private vulnerability reporting: Enable
- [ ] Dependabot alerts ja Dependabot security updates: Enable
- [ ] Secret scanning ja Push protection: Enable
- [ ] Code scanning: käytä repositorion `codeql.yml`-tiedostoa (Advanced setup). CodeQL-skannaus on ilmainen julkisille repositorioille, joten se käynnistyy automaattisesti vasta, kun repositorio muutetaan julkiseksi. Yksityisessä repositoriossa se vaatii maksullisen GitHub Code Security -lisäosan. Jos sinulla on se, aseta repositorion muuttuja `ENABLE_CODEQL` arvoon `true` (Settings → Secrets and variables → Actions → Variables).

**Pääkehityshaaran suojaus** (Settings → Rules → Rulesets → New branch ruleset, kohde: oletushaara)
- [ ] Restrict deletions
- [ ] Block force pushes
- [ ] Myöhemmin, jos projektiin tulee muita kehittäjiä: Require a pull request before merging ja Require status checks to pass (`test`, `govulncheck`). Nämä estävät suoran `git push` -komennon, joten yksin kehittäessä ne hankaloittavat työtä.
- [ ] Require signed commits (vapaaehtoinen)

**Tagien suojaus** (uusi tag ruleset, kohde `v*`)
- [ ] Restrict creations: vain ylläpitäjät
- [ ] Restrict updates ja Restrict deletions

**Actions** (Settings → Actions → General)
- [ ] Workflow permissions: Read repository contents
- [ ] Allow GitHub Actions to create and approve pull requests: pois päältä
- [ ] Fork pull request workflows: Require approval for all outside collaborators

## 3. Julkaisu

```sh
git tag -s v0.1.0 -m "Vahti 0.1.0"   # -s allekirjoittaa tagin, jos sinulla on GPG- tai SSH-allekirjoitus
git push origin v0.1.0
```

Release-työnkulku ajaa testit, kääntää ohjelman Raspberry Pi:lle (arm64 ja armv7) ja x86-koneille, laskee tarkistussummat, tekee ohjelmistoluettelon ja allekirjoitetun alkuperätodistuksen sekä luo julkaisusta luonnoksen. Tarkista luonnos Releases-sivulla ja julkaise se.

## 4. Ennen versiota 1.0

- Teetä ulkopuolinen tietoturvatarkastus. Avoin koodi auttaa, koska tarkastaja näkee kaiken.
- Testaa asennus puhtaalla Raspberry Pi OS:llä sekä Pi 4:llä että Pi 5:llä.
- Harkitse OpenSSF Scorecard -työnkulun lisäämistä. Se pisteyttää repositorion tietoturvakäytännöt julkisesti.
- Tarkista, ettei nimi Vahti ole jonkun toisen tavaramerkki ohjelmistoalalla.
