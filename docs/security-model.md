# Vahti security model

This document describes what Vahti protects against, how it is built and what it cannot do. The limitations are written out so that users know what they can rely on.

## What is protected

1. **Home network data:** which services devices connect to, and blocked requests staying at home.
2. **Vahti itself:** the web interface, rules and password. A compromised Vahti could let everything through or send traffic to the wrong place.
3. **The internet:** Vahti must not be usable for attacks on others (an open DNS resolver).

## Threats and protections

| Threat | Protection |
|---|---|
| Attacker on the internet | Vahti answers only private home network addresses, both for DNS and the web interface. Other packets are dropped silently. No port is opened on the router. |
| Abuse as an open resolver | Queries from outside are not answered. ANY and zone transfer queries are refused. Queries are rate limited per device (50/s by default). |
| Malicious website in a home computer's browser | The web interface accepts only its own names (DNS rebinding), changes need a CSRF token, and the Origin header is checked. Cookies are `SameSite=Strict`, `HttpOnly` and `Secure`. The content security policy blocks external and inline scripts, and the page cannot be framed. DNS also stops public names from pointing to home network addresses. |
| Password guessing from the home network | Argon2id hashing, passwords of at least 12 characters, growing delays after failed attempts, and both per-client and global attempt limits. |
| Malicious data from the network (device names, domains) | The web interface builds the page from text nodes, not HTML, so names cannot contain code. Input is validated strictly and unknown fields are rejected. DNS message handling is fuzz tested. |
| Exploiting the Vahti program | Go is a memory-safe language. The service runs as its own user without root and may only bind ports 53, 80 and 443. systemd sandboxes it: only the data directory is writable, and system calls, address families and namespaces are restricted. |
| Forged release | Releases are built on GitHub Actions with signed build provenance (Sigstore) that can be checked with `gh attestation verify`. SHA256 checksums and a software bill of materials (SBOM) are included. Actions are pinned to commit hashes. |
| Vulnerable dependency | There are three dependencies. govulncheck and CodeQL run on every change, and Dependabot proposes updates. |
| Crash or hang | systemd restarts the service. A watchdog checks every 15 seconds that DNS actually answers. State is written to disk atomically and the previous version is kept, so a power cut does not corrupt the settings. |
| Eavesdropping on the way to the DNS resolver | Allowed queries are forwarded encrypted (DNS over TLS) and the server certificate is verified. |

## Privacy by design

- The log is kept only in memory and never written to disk. A restart clears it.
- No queries are written to the system log (journal), only errors and sign-in attempts.
- Client subnet information (EDNS Client Subnet) is not forwarded.
- Home network names (`.lan`, `.home.arpa`, reverse lookups of private addresses) are never sent to the internet.
- Vahti does not check for updates, send statistics or contact the developers.

## Limitations

- **Encrypted content is not visible.** Vahti knows that a device connects to service X, but not what it sends. If the same server handles both functionality and data collection, the only choice is to allow or block both.
- **Hard-coded addresses.** A device that connects straight to an IP address or uses its own encrypted DNS with an unknown server bypasses DNS filtering. A version where Vahti also acts as a firewall will fix this.
- **Self-signed certificate.** The browser warns the first time. Check the fingerprint against the installer's output.
- **Devices on the home network are trusted at the network level.** Anyone on the home network can use the DNS service and try to sign in to the web interface. The password and attempt limits protect the management interface.
- **The DNS resolver sees allowed queries.** By default this is Quad9 (Switzerland, non-profit, does not store IP addresses). You can change the resolver in the settings or run your own recursive resolver, such as Unbound.
- **No external security audit has been done yet.** One should be done before version 1.0.
