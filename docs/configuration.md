# Configuration

The settings are in `/etc/vahti/config.json`. An unknown or invalid setting stops Vahti from starting and is reported in the log (`journalctl -u vahti`), so a typo never goes unnoticed.

| Setting | Meaning |
|---|---|
| `dns.upstreams` | DNS servers for allowed queries. Quad9 over DNS over TLS by default. Format: `tls://ip:853#name`. |
| `dns.allowedClients` | Addresses whose queries are answered. By default only private home network addresses. |
| `dns.localResolver` | The router, for local names (`.lan` etc.). If empty, local names get a "not found" answer. |
| `dns.rebindProtection` | Stops public names from pointing into your home network (DNS rebinding). |
| `dns.rebindAllow` | Exceptions to the above, e.g. `plex.direct`. |
| `dns.cacheSize` | How many answers are kept in the cache. Default 10000. |
| `dns.serveStaleMinutes` | For how long an expired cached answer is still given straight away while a fresh one is fetched in the background. Default 60, `0` turns it off. |
| `dns.prefetch` | Refreshes popular names shortly before their cached answer expires. Default `true`. |
| `web.hostnames` | Names the web interface can be opened with. Other names are rejected. |
| `lists` | Blocklists to download (https only). |
| `logEntries` | How many recent queries are kept in memory. The log is never written to disk. |
