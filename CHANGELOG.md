# Changelog

All notable changes are recorded in this file. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow [semantic versioning](https://semver.org/).

## [Unreleased]

### Changed
- The whole project is now in English: web interface, program messages, scripts and documentation.
- Windows scripts were renamed: `start-vahti.ps1`, `use-vahti.ps1`, `restore-dns.ps1`, `check-dns.ps1` and `dns-guard.ps1`. Backups and the startup restore task created by the earlier Finnish scripts are still recognised and restored.

### Fixed
- CI: staticcheck findings (error message style and an unused function).
- CodeQL runs only in a public repository, because code scanning in a private repository needs a paid add-on.

### Added
- README: project status (hobby project, in development) and guidance on monitoring other people's traffic.
- **Experimental Windows mode** (`vahti local`): filters the traffic of the computer Vahti runs on. Listens only on this computer (127.0.0.1 and ::1) and falls back to IPv4 if IPv6 is not available.
- **Automatic DNS restore on Windows:** `start-vahti.ps1` starts Vahti and a guard that points DNS at Vahti and restores the original settings as soon as Vahti is closed, crashes or stops answering. If the computer shuts down in the middle, a scheduled task restores the settings at the next startup.
- `check-dns.ps1` shows whether the computer's DNS settings are normal. Needs no administrator rights.
- `restore-dns.ps1` restores the settings by hand, also without a backup. Manually set DNS servers are preserved.
- The backup and restore script are stored in `%ProgramData%\Vahti`, which only SYSTEM and administrators can change.
- A CI test for the Windows restore logic with simulated network adapters.
- `vahti update-lists -local` downloads blocklists for Windows mode.
- `-router` option for local device names (e.g. `printer.lan`) in Windows mode.
- Blocklists are reloaded automatically when their files change. No restart needed.
- A notice in the web interface when Vahti runs in local mode.
- Windows package (`windows_amd64.zip`) in releases, plus a Windows build and PowerShell script checks in CI.
- Guides: [docs/windows.md](docs/windows.md), Windows mode in the README, and a README warning about changing DNS settings with instructions for restoring them.

## [0.1.0] - 2026-10-26

First version.

### Added
- Filtering DNS server: blocks advertising, analytics, telemetry, location tracking, social tracking, bypass attempts and malicious destinations before the connection opens.
- Per-device modes: open, learning and locked.
- Approval queue that shows what a destination is linked to: owner, other addresses on the same site, other devices and CNAME targets.
- Permissions once, for an hour or permanently, and sharing afterwards for a limited time.
- Disguised tracking (CNAME) detection, DNS rebinding protection and blocking of browsers' own encrypted DNS.
- Encrypted forwarding to the DNS resolver (DNS over TLS, Quad9 by default). Client subnet information is never forwarded.
- Web interface: sign-in (Argon2id), CSRF and Origin checks, Host header validation, strict content security policy and a self-signed HTTPS certificate.
- Sandboxed systemd service, watchdog and atomic state saving.
- Installer and uninstaller for Raspberry Pi OS, Debian and Ubuntu, plus a preparation script for old laptops.
- Demo mode (`vahti demo`) with simulated traffic.
- GitHub Actions: tests, staticcheck, govulncheck, CodeQL, fuzzing and signed releases with build provenance and a software bill of materials (SBOM).

[Unreleased]: https://github.com/EliasNiskanen/vahti/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/EliasNiskanen/vahti/releases/tag/v0.1.0
