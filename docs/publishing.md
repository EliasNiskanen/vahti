# Publishing Vahti on GitHub

## 1. Repository

The code lives at https://github.com/EliasNiskanen/vahti, and the Go module is `github.com/EliasNiskanen/vahti`.

## Making the repository public

Do these in order: steps 1–5 before publishing, step 6 is the publishing itself and step 7 right after it.

1. **Hide your email address.** On GitHub: profile picture → *Settings → Emails*. Turn on **Keep my email addresses private** and **Block command line pushes that expose my email**. Vahti's commit history already uses GitHub's private address `104171824+EliasNiskanen@users.noreply.github.com`.
2. **Two-factor authentication.** *Settings → Password and authentication → Enable two-factor authentication.* Choose an authenticator app or a passkey and store the recovery codes safely.
3. **Other settings in section 2:** Dependabot, secret scanning and Actions permissions.
4. **Trademark.** Search for the name *Vahti* before promoting the project widely:
   - EU and national registers in one search: [TMview](https://www.tmdn.org/tmview/). Search for `vahti` and look especially at classes 9 (software) and 42 (software services).
   - Finland: the Finnish Patent and Registration Office's trademark search at [prh.fi](https://www.prh.fi/).
   - If a registered Vahti mark exists in the same class, consider a more specific name (e.g. *Vahti DNS*) or ask an expert.
5. **CI green.** The latest run on the Actions tab must be green.
6. **Make public:** *Settings → General → Danger Zone → Change repository visibility → Make public.* CodeQL scanning starts automatically after this.
7. **Right after publishing** (on a free account these only work for public repositories):
   - **Private vulnerability reporting:** *Settings → Advanced Security* (or *Code security*) → **Private vulnerability reporting → Enable.**
   - **Main branch protection:** *Settings → Rules → Rulesets → New ruleset → New branch ruleset.* Name it `main`, **Enforcement status: Active**, **Target branches → Add target → Include default branch**. Select **Restrict deletions** and **Block force pushes**, then **Create**.

## 2. Repository settings

These are GitHub settings that cannot be stored in code.

**Your account**
- [ ] Two-factor authentication on (Settings → Password and authentication). Preferably a passkey or a hardware security key.

**Code security** (Settings → Code security)
- [ ] Private vulnerability reporting: Enable
- [ ] Dependabot alerts and Dependabot security updates: Enable
- [ ] Secret scanning and Push protection: Enable
- [ ] Code scanning: use the repository's `codeql.yml` (Advanced setup). CodeQL scanning is free for public repositories, so it starts automatically only once the repository is public. In a private repository it needs the paid GitHub Code Security add-on. If you have it, set the repository variable `ENABLE_CODEQL` to `true` (Settings → Secrets and variables → Actions → Variables).

**Main branch protection** (Settings → Rules → Rulesets → New branch ruleset, target: default branch)
- [ ] Restrict deletions
- [ ] Block force pushes
- [ ] Later, if other developers join: Require a pull request before merging and Require status checks to pass (`test`, `govulncheck`). These block a direct `git push`, so they get in the way when you work alone.
- [ ] Require signed commits (optional)

**Tag protection** (new tag ruleset, target `v*`)
- [ ] Restrict creations: maintainers only
- [ ] Restrict updates and Restrict deletions

**Actions** (Settings → Actions → General)
- [ ] Workflow permissions: Read repository contents
- [ ] Allow GitHub Actions to create and approve pull requests: off
- [ ] Fork pull request workflows: Require approval for all outside collaborators

## 3. Releasing

```sh
git tag -s v0.1.0 -m "Vahti 0.1.0"   # -s signs the tag if you have GPG or SSH signing set up
git push origin v0.1.0
```

The release workflow runs the tests, builds Vahti for Raspberry Pi (arm64 and armv7), x86 computers and Windows, computes checksums, creates a software bill of materials and signed build provenance, and drafts a release. Review the draft on the Releases page and publish it.

## 4. Before version 1.0

- Have an external security audit done. Open source helps, because the auditor can see everything.
- Test the installation on a clean Raspberry Pi OS on both a Pi 4 and a Pi 5.
- Consider adding the OpenSSF Scorecard workflow. It publicly rates the repository's security practices.
