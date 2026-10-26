# Vahti on an old laptop

An old laptop suits Vahti well. It usually has many times the power needed, and the battery acts as a backup power supply: during a short power cut Vahti stays on, and DNS works as soon as the router comes back.

## Is my computer suitable?

- 64-bit processor (almost everything made after 2008)
- at least 1 GB of memory
- a network port or a USB network adapter
- Linux can be installed (Debian 12 or Ubuntu Server 24.04)

A Windows laptop will not do as such, because Vahti runs as a service only on Linux. Installing Linux wipes the computer, so back up any files you want to keep first.

## 1. Install Linux

A light installation without a desktop is recommended, because the computer will work as a server:

- **Debian 12**: in the software selection, choose only *SSH server* and *standard system utilities*, no desktop environment.
- **Ubuntu Server 24.04 LTS**: choose *Install OpenSSH server* during installation.

Make a USB install stick, for example with Raspberry Pi Imager or balenaEtcher, and boot the computer from it.

After installation you can manage the computer from your desktop over SSH (`ssh user@computer-ip`). This works directly from Windows PowerShell.

## 2. Prepare the computer as a server

Run the preparation script that comes with the Vahti release:

```sh
sudo ./prepare-laptop.sh
```

It does the following:

- **Closing the lid does not put the computer to sleep.** You can keep the lid closed.
- **Sleep and hibernation are turned off.**
- **Security updates install automatically** (unattended-upgrades).
- **Firewall on** (ufw), if installed: only DNS, the web interface and SSH are allowed in, and only from the home network.

You can also do these by hand. The commands are readable in the script.

## 3. Install Vahti

Download the `linux_amd64` package from the releases page and continue with the [installation steps in the README](../README.md#installation).

## Good to know

**Battery.** A battery kept at 100 % and warm ages faster and can, at worst, swell. Many laptops let you limit charging to 60–80 % in the BIOS or in the manufacturer's tool. Do this if you can. Check the battery visually every few months: if the case bulges or the touchpad lifts, remove the battery and run the computer on the charger only.

**Heat.** Do not put the computer in a closed cabinet or on a soft surface. Keep the vents free of dust.

**Wi-Fi.** Use a network cable. If the computer has no network port, a USB network adapter is better than Wi-Fi.

**Power use.** An idle laptop typically uses about 5–15 W, much less than a desktop. The screen turns off when the lid is closed.

**BIOS settings (optional).** Turn on automatic start after a power cut (*AC Power Recovery* or *Power On After Power Loss*) if available, and set a BIOS password.

**Router mode later.** When Vahti gets its router mode, the laptop sits between the modem and the router. You will then need a USB network adapter (ideally USB 3 and gigabit) in addition to the computer's own port.
