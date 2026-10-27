# Hardware

Vahti is light: the program is about 8 MB and typically uses less than 100 MB of memory. Almost any always-on Linux computer will do.

## Minimum requirements

| | Requirement |
|---|---|
| Processor | 64-bit ARM (arm64), 32-bit ARMv7 or x86-64 (amd64) |
| Memory | 512 MB, 1 GB or more recommended |
| Storage | 1 GB free. A memory card is enough; an SSD lasts longer. |
| Network | Ethernet cable to the router. Wi-Fi works but is less reliable, and the whole home's internet then depends on it. |
| Operating system | Linux with systemd: Raspberry Pi OS (Bookworm or newer), Debian 12+ or Ubuntu 22.04+ |
| Uptime | On around the clock. When Vahti is your home's DNS server, the internet does not work without it. |

As a service that protects the whole home network, Vahti runs on Linux. On Windows there is an [experimental mode](windows.md) that protects only the computer Vahti runs on. Demo mode (`vahti demo`) works on every operating system.

## Suitable devices

| Device | Fit | Notes |
|---|---|---|
| **Raspberry Pi 4 or 5** | Excellent | Silent and low-power (about 3–7 W). The recommended choice. |
| Raspberry Pi 3B+ | Good | Enough for a normal home. Install a 64-bit operating system. |
| **Old laptop** | Good | Free if you already have one. The battery acts as a backup power supply during outages. See [docs/old-laptop.md](old-laptop.md). |
| Mini PC (e.g. Intel N100) | Excellent | A good choice for the future router mode, especially a model with two network ports. |
| Old desktop | Works | Uses more power (often 30–60 W), which adds up when it runs all year. |
| Raspberry Pi Zero 2 W | Not recommended | Wi-Fi only, no network port. |

## Parts list for a Raspberry Pi 5

A Raspberry Pi 5 with 4 GB of memory is plenty for Vahti, including the planned gateway mode.

| Part | Needed | Why |
|---|---|---|
| Official 27 W USB-C power supply (5 V / 5 A) | Yes | Phone chargers often cannot supply enough current, and the Pi becomes unstable. |
| microSD card, 32 GB or more, class A2 | Yes | A brand-name card lasts longer under constant use. |
| Active Cooler, or the official case with a fan | Yes | The Pi 5 runs hot without cooling. |
| Case | Yes | Not needed separately if you take the official case with a fan. |
| Ethernet cable to the router | Yes | A cable is much more reliable than Wi-Fi for a DNS server. |
| RTC battery (official Raspberry Pi RTC Battery) | Recommended | Keeps the clock running through power cuts. Vahti checks the encrypted DNS server's certificate against the clock, so a correct time at start-up helps. |
| NVMe SSD with an M.2 HAT+ | Optional | Faster and more durable than a memory card. |
| USB 3 gigabit Ethernet adapter (Realtek RTL8153 or ASIX AX88179 chip) | Later | The second network port for the planned gateway mode. These chips work in Linux without extra drivers. |

## Wiring

In this version Vahti runs as your home's DNS server *alongside* the router:

```
Internet ── modem ── router ──┬── phones, TV, computers
                              │      (ask Vahti for names)
                              └── Vahti (wired)
```

The device needs only one network connection. The router tells your devices that Vahti is the DNS server.

A future router mode will place Vahti *between* the modem and your home network, so it can see and control all traffic. That needs **two network ports**: the computer's own port and, for example, a USB network adapter. A Raspberry Pi 5 or an old laptop can do this with a USB adapter, and a two-port mini PC is ready for it out of the box.
