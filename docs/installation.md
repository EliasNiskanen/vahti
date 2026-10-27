# Installing Vahti on Linux

This guide installs Vahti as the DNS server for your whole home network. For the experimental Windows mode, see [windows.md](windows.md). For hardware, see [hardware.md](hardware.md).

> [!NOTE]
> The first release (v0.1.0) is still being prepared. Until it is published on the [releases page](https://github.com/EliasNiskanen/vahti/releases), build the program from source with `make pi` (Raspberry Pi) or `go build ./cmd/vahti`.


These steps are written for a Raspberry Pi, but they work the same on any Linux computer. If you use an old laptop, do the [laptop preparation](old-laptop.md) first. Connect the device to your router with a network cable.

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

## Good to know

- **IPv6:** If your router hands out IPv6 addresses, it may advertise its own DNS server over IPv6. Turn off IPv6 DNS advertisement in the router or set Vahti's IPv6 address.
- **Local device names:** To show the device names your router knows, set `localResolver` in `/etc/vahti/config.json` to the router's address, e.g. `"192.168.1.1:53"`.
- **If Vahti stops, the internet stops.** This is intentional: filtering is never bypassed silently. systemd restarts Vahti automatically if it crashes or DNS stops answering for 30 seconds. In an emergency, restore the router's DNS setting.
- **Forgotten password:** `sudo systemctl stop vahti && sudo vahti reset-admin && sudo systemctl start vahti`
- **Blocklists:** `sudo systemctl start vahti-update-lists` downloads the lists now. The installer can turn on weekly updates.

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

## Monitoring other people's traffic

Vahti sees which services the devices in your home connect to. If other people use those devices, tell them about Vahti: what it sees, what it blocks and who can open the web interface. Vahti's log is kept only in the device's memory and is never sent anywhere.

In your own home Vahti is your own tool. At workplaces, in shared flats, on tenants' networks and on other shared networks, the rules on the confidentiality of communications are stricter. Find out in advance whether you may monitor other people's traffic and how you must tell them about it. This is not legal advice.
