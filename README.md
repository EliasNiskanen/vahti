<p align="center">
  <img src="assets/logo.svg" width="160" alt="Vahti logo: a watchdog in a detective's fedora, with a monocle and a pipe">
</p>

<h1 align="center">Vahti</h1>

Vahti is a privacy filter for your home network. It acts as the network's DNS server and checks every request before anything leaves your home. Advertising, analytics, device telemetry and location-tracking services are blocked by default. You decide, device by device, what may be shared and with whom.

Vahti is fully local. It has no cloud service, no user account and no remote management, and it sends no data to anyone. The only outbound connections are allowed DNS queries, forwarded encrypted to a DNS resolver, and blocklist downloads when you ask for them.

*Vahti* is Finnish for "guard" or "watch".

> [!IMPORTANT]
> **Vahti is a hobby project and in development.** Do not rely on it as critical protection.
> - No external security audit has been done.
> - The installer has not yet been run on a real Raspberry Pi or laptop, and the Windows DNS guard has not been run on a real Windows computer. The installer is checked automatically (shellcheck), and the guard's restore logic is tested with simulated network adapters.
> - Automated tests, staticcheck and a vulnerability check run on every change, but they are no substitute for an audit.
>
> I maintain the project in my free time. I respond to issues and fix problems when I can, but I cannot promise a schedule. To report a security problem, see [SECURITY.md](SECURITY.md).

> [!WARNING]
> **Setting up Vahti changes your DNS settings.** Once your router or computer uses Vahti as its DNS server, the internet works only while Vahti is running. If Vahti stops, the device shuts down or its IP address changes, websites and apps stop working even though the connection itself is fine. Read [Changing and restoring DNS settings](#changing-and-restoring-dns-settings) before you start.

## What Vahti does

- **Blocks tracking before the connection opens.** A blocked request never leaves your home network.
- **Recognises devices** and shows what each one has tried to reach.
- **Three modes per device:**
  - *Open*: known tracking services are blocked, everything else works. Suits phones and computers.
  - *Learning*: the device works freely while Vahti records the services it needs. When learning ends, the device is locked.
  - *Locked*: only allowed destinations work. New destinations are blocked and brought to you for a decision. Suits smart devices.
- **Approval queue:** see what a destination is linked to (owner, other addresses on the same site, other devices) and allow it once, for an hour or permanently, or block it.
- **Share afterwards:** allow a blocked destination for a limited time from the device page.
- **Disguised tracking detection:** if a site's own address redirects (CNAME) to a tracking service, it is blocked.
- **Bypass protection:** browsers' own encrypted DNS and iCloud Private Relay are switched off, and known encrypted DNS services are blocked.
- **No location hints:** Vahti does not forward your network's subnet (EDNS Client Subnet), and names inside your home network never go to the internet.

What Vahti does not do: it does not see the content of encrypted traffic or break encryption. Blocking is based on *where* a device connects. More: [docs/security-model.md](docs/security-model.md).

## Hardware

Vahti is light: the program is about 8 MB and typically uses less than 100 MB of memory. Almost any always-on Linux computer will do.

### Minimum requirements

| | Requirement |
|---|---|
| Processor | 64-bit ARM (arm64), 32-bit ARMv7 or x86-64 (amd64) |
| Memory | 512 MB, 1 GB or more recommended |
| Storage | 1 GB free. A memory card is enough; an SSD lasts longer. |
| Network | Ethernet cable to the router. Wi-Fi works but is less reliable, and the whole home's internet then depends on it. |
| Operating system | Linux with systemd: Raspberry Pi OS (Bookworm or newer), Debian 12+ or Ubuntu 22.04+ |
| Uptime | On around the clock. When Vahti is your home's DNS server, the internet does not work without it. |

As a service that protects the whole home network, Vahti runs on Linux. On Windows there is an [experimental mode](docs/windows.md) that protects only the computer Vahti runs on. Demo mode (`vahti demo`) works on every operating system.

### Suitable devices

| Device | Fit | Notes |
|---|---|---|
| **Raspberry Pi 4 or 5** | Excellent | Silent and low-power (about 3–7 W). The recommended choice. |
| Raspberry Pi 3B+ | Good | Enough for a normal home. Install a 64-bit operating system. |
| **Old laptop** | Good | Free if you already have one. The battery acts as a backup power supply during outages. See [docs/old-laptop.md](docs/old-laptop.md). |
| Mini PC (e.g. Intel N100) | Excellent | A good choice for the future router mode, especially a model with two network ports. |
| Old desktop | Works | Uses more power (often 30–60 W), which adds up when it runs all year. |
| Raspberry Pi Zero 2 W | Not recommended | Wi-Fi only, no network port. |

### Wiring

In this version Vahti runs as your home's DNS server *alongside* the router:

```
Internet ── modem ── router ──┬── phones, TV, computers
                              │      (ask Vahti for names)
                              └── Vahti (wired)
```

The device needs only one network connection. The router tells your devices that Vahti is the DNS server.

A future router mode will place Vahti *between* the modem and your home network, so it can see and control all traffic. That needs **two network ports**: the computer's own port and, for example, a USB network adapter. A Raspberry Pi 5 or an old laptop can do this with a USB adapter, and a two-port mini PC is ready for it out of the box.

## Try it on your computer

In demo mode the traffic is simulated, but everything runs through the real filtering engine. Nothing is sent to the internet.

```sh
go build -o vahti ./cmd/vahti
./vahti demo
```

Open http://127.0.0.1:8080 and enter the setup code printed in the terminal. You can also query the demo DNS server directly: `dig @127.0.0.1 -p 5353 doubleclick.net`.

## See your own traffic on Windows

The experimental Windows mode filters only the computer Vahti runs on. Unlike demo mode, you see your computer's real traffic: for example Windows' own telemetry and the advertising and analytics services on websites. Protecting the other devices in your home needs a Linux device (see [Hardware](#hardware)).

1. Download `vahti_<version>_windows_amd64.zip` from the [releases page](https://github.com/EliasNiskanen/vahti/releases) and extract it, for example to `C:\Vahti`.
2. Open PowerShell in the folder (Shift + right-click on an empty spot in the folder → *Open PowerShell window here*) and start Vahti:
   ```powershell
   powershell -ExecutionPolicy Bypass -File .\start-vahti.ps1
   ```
   Vahti opens in its own window, and Windows asks for permission to change the DNS settings. Accept.
3. Open http://127.0.0.1:8080 in your browser, enter the setup code shown in the Vahti window and set a password.
4. Download the blocklists: `.\vahti.exe update-lists -local`
5. In Chrome or Edge, turn off *Use secure DNS*, otherwise the browser bypasses Vahti.

**To stop:** close the Vahti window. **DNS settings are restored automatically**, also if Vahti crashes. If the computer shuts down in the middle, the settings are restored at the next startup.

**To check** at any time whether your settings are normal:
```powershell
powershell -ExecutionPolicy Bypass -File .\check-dns.ps1
```

More details, troubleshooting and how the restore works: [docs/windows.md](docs/windows.md).

## Installation

These steps are written for a Raspberry Pi, but they work the same on any Linux computer. If you use an old laptop, do the [laptop preparation](docs/old-laptop.md) first. Connect the device to your router with a network cable.

1. Download the package for your device and the `SHA256SUMS` file from the [releases page](https://github.com/EliasNiskanen/vahti/releases):
   - Raspberry Pi 3/4/5 (64-bit operating system): `linux_arm64`
   - Raspberry Pi, 32-bit operating system: `linux_armv7`
   - Laptop, mini PC or desktop: `linux_amd64`
2. Check that the download is genuine:
   ```sh
   sha256sum --check --ignore-missing SHA256SUMS
   gh attestation verify vahti_*_linux_*.tar.gz --repo EliasNiskanen/vahti
   ```
   The second command proves the package was built on GitHub from this source code.
3. Extract and install:
   ```sh
   tar xzf vahti_*_linux_*.tar.gz
   cd vahti_*_linux_*/
   sudo ./install.sh
   ```
4. Open the address shown at the end of the installation and enter the setup code. The browser warns about a self-signed certificate. Check that the fingerprint matches the one the installer printed, then accept it.
5. In your router settings, give the Vahti device a fixed IP address.
6. In the router's DHCP settings, **set the DNS server to the Vahti device's address**. Use only this one address: a secondary server would bypass Vahti.
7. Turn Wi-Fi off and on again on your devices so they pick up the new setting.

### Good to know

- **IPv6:** If your router hands out IPv6 addresses, it may advertise its own DNS server over IPv6. Turn off IPv6 DNS advertisement in the router or set Vahti's IPv6 address.
- **Local device names:** To show the device names your router knows, set `localResolver` in `/etc/vahti/config.json` to the router's address, e.g. `"192.168.1.1:53"`.
- **If Vahti stops, the internet stops.** This is intentional: filtering is never bypassed silently. systemd restarts Vahti automatically if it crashes or DNS stops answering for 30 seconds. In an emergency, restore the router's DNS setting.
- **Forgotten password:** `sudo systemctl stop vahti && sudo vahti reset-admin && sudo systemctl start vahti`
- **Blocklists:** `sudo systemctl start vahti-update-lists` downloads the lists now. The installer can turn on weekly updates.

## Monitoring other people's traffic

Vahti sees which services the devices in your home connect to. If other people use those devices, tell them about Vahti: what it sees, what it blocks and who can open the web interface. Vahti's log is kept only in the device's memory and is never sent anywhere.

In your own home Vahti is your own tool. At workplaces, in shared flats, on tenants' networks and on other shared networks, the rules on the confidentiality of communications are stricter. Find out in advance whether you may monitor other people's traffic and how you must tell them about it. This is not legal advice.

## Changing and restoring DNS settings

Vahti only works if devices ask it for names. That is why setting it up changes the DNS settings of either your router (the whole home) or one computer (Windows mode). This makes Vahti an essential part of your network.

**Before the change**
- **Write down the original settings.** Take a screenshot of the router's DHCP or DNS settings page before you change anything. Usually DNS is automatic, which means the router uses your internet provider's DNS servers.
- **Check that Vahti answers.** Open the web interface and make sure traffic appears before you change the settings for the whole home.
- **Tell the people you live with.** If the internet stops working, they should know what might cause it and whom to call.
- **Make sure you can reach the router's settings without DNS.** The router's address is usually an IP address such as `192.168.1.1`, which works even when DNS does not. Keep the address and the router password at hand.

**Do not add a secondary server.** If you give the router a second DNS server next to Vahti, devices will use it from time to time and tracking gets past Vahti.

**If the internet stops working**
1. **Check Vahti.** On Linux: `systemctl status vahti`. On Windows: is the `vahti.exe local` window open?
2. **Restore the settings if you cannot get Vahti working right away:**
   - **Whole home (router):** open the router's settings by IP address (e.g. http://192.168.1.1), go to the DHCP or LAN settings and set the DNS server back to *automatic* or to the original addresses you wrote down. Turn Wi-Fi off and on again on your devices so they pick up the setting.
   - **Windows mode:** the settings are normally restored automatically when Vahti is closed. Check with `check-dns.ps1`. If they were not restored, run as administrator `powershell -ExecutionPolicy Bypass -File .\restore-dns.ps1`. Without the script: *Settings → Network & Internet → Wi-Fi or Ethernet → connection properties → DNS server assignment → Edit → Automatic (DHCP)*.

**Removing Vahti:** always restore your DNS settings *before* you remove Vahti or shut the device down for good. The uninstaller (`uninstall.sh`) reminds you of this.

## Configuration

The settings are in `/etc/vahti/config.json`. An unknown or invalid setting stops Vahti from starting and is reported in the log (`journalctl -u vahti`), so a typo never goes unnoticed.

| Setting | Meaning |
|---|---|
| `dns.upstreams` | DNS servers for allowed queries. Quad9 over DNS over TLS by default. Format: `tls://ip:853#name`. |
| `dns.allowedClients` | Addresses whose queries are answered. By default only private home network addresses. |
| `dns.localResolver` | The router, for local names (`.lan` etc.). If empty, local names get a "not found" answer. |
| `dns.rebindProtection` | Stops public names from pointing into your home network (DNS rebinding). |
| `dns.rebindAllow` | Exceptions to the above, e.g. `plex.direct`. |
| `web.hostnames` | Names the web interface can be opened with. Other names are rejected. |
| `lists` | Blocklists to download (https only). |
| `logEntries` | How many recent queries are kept in memory. The log is never written to disk. |

## Development

```sh
make test     # tests with the race detector
make check    # tests, go vet and formatting
make pi       # build for Raspberry Pi
```

Layout:

| Folder | Contents |
|---|---|
| `cmd/vahti` | Program start-up and commands |
| `internal/dnsserver` | The filtering DNS server: cache, encrypted forwarding, CNAME and rebinding checks |
| `internal/engine` | Decision logic: devices, rules, queue, log |
| `internal/intel` | Tracker recognition and ownership data |
| `internal/web` | Web interface and API: sign-in, sessions, CSRF protection |
| `deploy` | Installer and sandboxed systemd services |
| `deploy/windows` | PowerShell scripts for Windows mode |
| `docs` | Security model and guides for Windows, old laptops and publishing |

Dependencies are deliberately few: [miekg/dns](https://github.com/miekg/dns) and the Go project's own `golang.org/x/crypto` and `golang.org/x/net`.

## Changes

The version history is in [CHANGELOG.md](CHANGELOG.md).

## Security

Found a vulnerability? Do not open a public issue; see [SECURITY.md](SECURITY.md).

## License

Apache License 2.0. The software is provided as is, without warranty (see [LICENSE](LICENSE)).

The blocklists have their own licenses: StevenBlack/hosts (MIT) and URLhaus (CC0). If you add lists, check their terms. Some tracker databases allow only non-commercial use.
