# Experimental Windows mode

In Windows mode Vahti filters **only the computer** it runs on. You can see, for example, Windows' own telemetry and the tracking services used by your browser and programs, and block them. Protecting the other devices in your home needs Vahti on a Raspberry Pi or an old laptop (see [hardware.md](hardware.md)).

This mode is experimental: it has not yet been tested widely on real Windows computers. Please report anything that does not work.

## Setup

Extract `vahti_<version>_windows_amd64.zip`, for example to `C:\Vahti`. Open PowerShell in the folder (Shift + right-click on an empty spot in the folder → *Open PowerShell window here*).

**1. Start Vahti:**

```powershell
powershell -ExecutionPolicy Bypass -File .\start-vahti.ps1
```

Vahti opens in its own window. Once it answers, Windows asks for permission to change the DNS settings. Accept, and this computer's DNS queries are sent to Vahti.

**2. Set up Vahti and choose a password.** Open http://127.0.0.1:8080 in your browser and enter the setup code printed in the Vahti window. This is needed only the first time.

**3. Download the blocklists (recommended).** In a normal PowerShell window:

```powershell
.\vahti.exe update-lists -local
```

Vahti picks up the lists within 30 seconds.

**4. Turn off your browser's own encrypted DNS.** In Chrome and Edge: Settings → Privacy and security → *Use secure DNS* → off. Otherwise the browser may bypass Vahti. In Firefox, Vahti turns this off automatically.

The Overview page now shows the device *This computer* and its real traffic.

## Stopping

Close the Vahti window or press Ctrl+C in it. **DNS settings are restored automatically** within a few seconds.

## How the DNS restore works

Vahti itself runs without administrator rights, so it cannot change the computer's settings. A separate **guard** (`dns-guard.ps1`), which you give permission to at start-up, takes care of the DNS settings. The guard:

1. saves the computer's original DNS settings, including manually set addresses
2. points DNS queries at Vahti
3. restores the original settings as soon as Vahti is closed, crashes or stops answering for 15 seconds.

If the computer shuts down or loses power while Vahti is running, the settings are restored **at the next startup**. For this, the guard creates a task called *Vahti DNS restore* in Windows Task Scheduler. The task removes itself once the settings are restored.

The backup, the restore script and the event log (`dns.log`) are kept in `%ProgramData%\Vahti`. Only SYSTEM and administrators can change that folder, so nobody can tamper with the script that runs at startup.

## Check that your settings are normal

```powershell
powershell -ExecutionPolicy Bypass -File .\check-dns.ps1
```

The script shows the DNS servers of each network adapter and tells you whether Vahti is running, whether queries go to it and whether DNS works. It changes nothing and needs no administrator rights.

## If the internet does not work

Run as administrator:

```powershell
powershell -ExecutionPolicy Bypass -File .\restore-dns.ps1
```

The script restores the settings from the backup. Without a backup, it sets the adapters that point at Vahti back to automatic. Without the script: *Settings → Network & Internet → Wi-Fi or Ethernet → connection properties → DNS server assignment → Edit → Automatic (DHCP)*.

## Good to know

- **Local device names:** If you use names such as `printer.lan` or `fritz.box`, start Vahti with your router's address: `.\start-vahti.ps1 -Router 192.168.1.1`.
- **Port 53 in use:** If Vahti reports that port 53 is in use, another program is already acting as a DNS server, e.g. Internet Connection Sharing, Hyper-V or WSL. See which one with `Get-NetUDPEndpoint -LocalPort 53`.
- **VPN programs** often set their own DNS server. While a VPN is connected, traffic may bypass Vahti.
- **SmartScreen** may warn because `vahti.exe` is not signed with a Microsoft-trusted certificate. You can check that a release is genuine with `gh attestation verify vahti_*_windows_amd64.zip --repo EliasNiskanen/vahti`.
- **Vahti's data** (settings, password hash) is stored in `%LOCALAPPDATA%\Vahti`. The log is kept only in memory, as in the Linux version.
- **Limitations:** Vahti does not start with Windows or restart itself after a crash, and it is not sandboxed the way the Linux service is. Windows mode is therefore meant for trying Vahti out and looking at your own computer. For continuous protection of the whole home, use a Linux device.

## Scripts

| File | Purpose | Administrator |
|---|---|---|
| `start-vahti.ps1` | Starts Vahti and the guard. The recommended way. | Asks for permission |
| `check-dns.ps1` | Shows the state of the DNS settings. Changes nothing. | No |
| `restore-dns.ps1` | Restores the settings by hand. | Yes |
| `use-vahti.ps1` | Starts the guard for a Vahti that is already running. | Yes |
| `dns-guard.ps1` | The guard. Started by the other scripts. | Yes |
| `VahtiDns.psm1` | Shared functions. | – |
