# Changelog

All notable changes are recorded in this file. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow [semantic versioning](https://semver.org/).

## [Unreleased]

### Added
- Logo: a watchdog in a detective's fedora and cape, with a monocle and a calabash pipe. A detailed version (`assets/logo.svg`) for 48 px and up and a simplified one (`assets/logo-small.svg`) for small sizes such as the browser tab. Used in the web interface and README, plus a social preview image (`assets/social-preview.png`).
- Serve-stale: an expired cached answer is given straight away (with a 30-second TTL) while a fresh one is fetched in the background, for up to `dns.serveStaleMinutes` (default 60). This also keeps names working during short upstream outages.
- Prefetch: names asked for in the last 10% of their cached lifetime are refreshed in the background before they expire (`dns.prefetch`, on by default).
- Speed panel on the overview page: typical and slowest answer times, the share answered from the cache and the upstream lookup time over the last 15 minutes. Only timings are kept, no names or addresses.

## [0.1.0] - 2026-10-26

First public release. Vahti is in development and has not had an external security audit; see the README.

### Added
- Filtering DNS server: blocks advertising, analytics, telemetry, location tracking, social tracking, bypass attempts and malicious destinations before the connection opens.
- Per-device modes: open, learning and locked.
- Approval queue that shows what a destination is linked to: owner, other addresses on the same site, other devices and CNAME targets.
- Permissions once, for an hour or permanently, and sharing afterwards for a limited time.
- Disguised tracking (CNAME) detection, DNS rebinding protection and blocking of browsers' own encrypted DNS.
- Encrypted forwarding to the DNS resolver (DNS over TLS, Quad9 by default). Client subnet information is never forwarded.
- Web interface: sign-in (Argon2id), CSRF and Origin checks, Host header validation, strict content security policy and a self-signed HTTPS certificate.
- Sandboxed systemd service, watchdog and atomic state saving. Blocklists reload automatically when their files change.
- Installer and uninstaller for Raspberry Pi OS, Debian and Ubuntu, plus a preparation script for old laptops.
- Demo mode (`vahti demo`) with simulated traffic.
- **Experimental Windows mode** (`vahti local`): filters only the computer Vahti runs on. `start-vahti.ps1` starts Vahti and a guard that points DNS at Vahti and restores the original settings as soon as Vahti is closed, crashes or stops answering; a scheduled task restores them at the next startup if the computer shuts down in the middle. `check-dns.ps1` shows whether the DNS settings are normal, and `restore-dns.ps1` restores them by hand.
- Release packages for Raspberry Pi (arm64, armv7), x86-64 Linux and Windows, with checksums, a software bill of materials (SBOM) and signed build provenance.
- GitHub Actions: tests with the race detector, staticcheck, govulncheck, CodeQL, fuzzing, shellcheck and a test of the Windows restore logic with simulated network adapters.
- Documentation: hardware guide, old-laptop guide, Windows guide, security model, guidance on monitoring other people's traffic and on changing and restoring DNS settings.

[Unreleased]: https://github.com/EliasNiskanen/vahti/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/EliasNiskanen/vahti/releases/tag/v0.1.0
