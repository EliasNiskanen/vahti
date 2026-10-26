# Vahdin julkaiseminen GitHubissa

## 1. Repositorio

Koodi on repositoriossa https://github.com/EliasNiskanen/vahti, ja Go-moduulin nimi on `github.com/EliasNiskanen/vahti`.

## 2. Repositorion asetukset (Settings)

Nämä ovat GitHubin asetuksia, joita ei voi tallentaa koodiin. Tee ne heti.

**Oma tili**
- [ ] Kaksivaiheinen tunnistautuminen päälle (Settings → Password and authentication). Mieluiten pääsyavain tai fyysinen turva-avain.

**Code security** (Settings → Code security)
- [ ] Private vulnerability reporting: Enable
- [ ] Dependabot alerts ja Dependabot security updates: Enable
- [ ] Secret scanning ja Push protection: Enable
- [ ] Code scanning: käytä repositorion `codeql.yml`-tiedostoa (Advanced setup). CodeQL-skannaus on ilmainen julkisille repositorioille, joten se käynnistyy automaattisesti vasta, kun repositorio muutetaan julkiseksi. Yksityisessä repositoriossa se vaatii maksullisen GitHub Code Security -lisäosan. Jos sinulla on se, aseta repositorion muuttuja `ENABLE_CODEQL` arvoon `true` (Settings → Secrets and variables → Actions → Variables).

**Pääkehityshaaran suojaus** (Settings → Rules → Rulesets → New branch ruleset, kohde `main`)
- [ ] Require a pull request before merging
- [ ] Require status checks to pass: `test`, `govulncheck`, `build`, `analyze`
- [ ] Block force pushes
- [ ] Restrict deletions
- [ ] Require signed commits (vapaaehtoinen, suositeltava)

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
