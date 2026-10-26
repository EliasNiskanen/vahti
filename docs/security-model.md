# Vahti security model

This document describes what Vahti protects against, how it is built and what it cannot do. The limitations are written out so users know what they can rely on.

## What is protected

1. **Your home network's data:** which services your devices contact, and keeping blocked requests at home.
2. **Vahti itself:** the web interface, rules and password. A compromised Vahti could let everything through or send traffic to the wrong place.
3. **The internet:** Vahti must not be usable for attacks on others (an open DNS resolver).

## Threats and protections

| Threat | Protection |
|---|---|
| Attacker on the internet | Vahti only answers private home network addresses, both for DNS and the web interface. Other packets are silently dropped. No port is opened on the router. |
| Abuse as an open resolver | Queries from outside are not answered. ANY and zone transfer queries are refused. Queries are rate-limited per device (50/s by default). |
| Malicious website in a browser at home | The web interface accepts only its own names (DNS rebinding), changes require a CSRF token, and the Origin header is checked. Cookies are `SameSite=Strict`, `HttpOnly` and `Secure`. The Content Security Policy blocks external and inline scripts, and the page cannot be framed. DNS also stops public names from pointing to addresses in the home network. |
| Password guessing from the home network | Argon2id hashing, at least 12-character passwords, growing delays after failed attempts, and attempt limits both per device and for the whole system. |
| Malicious data from the network (device names, domains) | The interface builds the page as text nodes, not HTML, so names cannot contain code. Input is validated strictly and unknown fields are rejected. DNS message handling is fuzz-tested. |
| A bug exploited in Vahti | Go is a memory-safe language. The service runs as its own user without root, with only the right to use ports 53, 80 and 443. systemd sandboxes it: only the data directory is writable, and system calls, network families and namespaces are restricted. |
| Forged release | Releases are built in GitHub Actions with a signed provenance attestation (Sigstore), which can be checked with `gh attestation verify`. SHA256 checksums and a software bill of materials (SBOM) are included. Actions are pinned to commit hashes. |
| Vulnerable dependency | There are three dependencies. govulncheck and CodeQL run on every change, and Dependabot proposes updates. |
| Crash or hang | systemd restarts the service. A watchdog checks every 15 seconds that DNS actually answers. State is written to disk atomically and the previous version is kept, so a power cut does not corrupt settings. |
| Eavesdropping on the way to the resolver | Allowed queries are forwarded encrypted (DNS over TLS) and the server certificate is verified. |

## Privacy by design

- The log is kept only in memory and is never written to disk. A restart clears it.
- No queries are written to the system log (journal), only errors and sign-in attempts.
- Devices' subnet information (EDNS Client Subnet) is not forwarded.
- Home network names (`.lan`, `.home.arpa`, reverse lookups of private addresses) are never sent to the internet.
- Vahti does not check for updates, send statistics or contact its developers.

## Limitations

- **Encrypted content is not visible.** Vahti knows a device contacts service X, but not what it sends. If the same server handles both a feature and data collection, the only choices are to allow both or block both.
- **Hard-coded addresses.** A device that connects straight to an IP address or uses its own encrypted DNS to an unknown server bypasses DNS filtering. This will be addressed in a version where Vahti also acts as a firewall.
- **Self-signed certificate.** The browser warns the first time. Check the fingerprint against the installer's output.
- **Devices on the home network are trusted at the network level.** Anyone on the home network can use DNS and try to sign in to the web interface. The password and attempt limits protect management.
- **The DNS provider sees allowed queries.** By default this is Quad9 (Switzerland, non-profit, does not store IP addresses). You can change the provider in the settings or run your own recursive resolver, such as Unbound.
- **No external security audit has been done yet.** One should be done before version 1.0.
