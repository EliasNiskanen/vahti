# Security policy

## Reporting a vulnerability

Please do not report vulnerabilities in a public issue.

Use GitHub's private reporting channel: the repository's **Security** tab → **Report a vulnerability**. Only the maintainers can see the report.

Please include:

- what you found and what it affects
- how to reproduce it (version, settings, requests)
- a suggested fix, if you have one

Vahti is a hobby project maintained in my free time, so I cannot promise exact response times. I aim for:

- a reply within a week
- serious problems fixed before other work
- a published advisory (GitHub Security Advisory) after the fix, crediting the reporter if they wish

Please do not publish details before a fix is available, and do not test against other people's devices.

## Supported versions

Security fixes go into the latest release. Always update to the latest version.

## What counts as a vulnerability

For example:

- access to the web interface without a password, or a way around the CSRF protection
- getting Vahti to answer DNS queries from outside the home network
- a blocked query leaving the home network
- reading or writing files outside the data directory
- a denial of service that a single device on the home network can easily cause
- running malicious code in the web interface (XSS)

Threat model and known limitations: [docs/security-model.md](docs/security-model.md).
