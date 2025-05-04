# ZT DNS Companion

* * *

## About

This tool allows you to query your [ZeroTier](https://zerotier.com) networks and create per-interface DNS settings for name resolution. Whether you use a managed or self hosted controller, you can configure at the controller level [custom DNS server and search domain](https://docs.zerotier.com/dns-management) downstream to the client. Windows, MacOS, Android, and IOS all have the capability of auto configuring their systems to utilize this setting. If you are running a Linux host, you need to perform extra configuration, due to the many types of network tools avaialable. This utility aims to automate that work.

This tool supports using:

## Maintainer

nfrastack <code@nfrastack.com>

## Table of Contents

-[About](#about)

## Prerequisites and Assumptions

* Assumes you are have the Zerotier-One client installed and are connected to one or more networks
* An available DNS server to serve records
* Utilizing either:
  * `systemd-networkd`, a system service that manages network configurations, primarily for servers and headless Linux systems.
  * `systemd-resolved` which is a system service that provides network name resolution for local applications on Linux systems. It works alongside [NetworkManager](https://wiki.archlinux.org/title/NetworkManager#systemd-resolved), [ConnMan](https://wiki.archlinux.org/title/ConnMan#Using_systemd-resolved) and [`iwd`](https://wiki.archlinux.org/title/Iwd#Select_DNS_manager) and others making this suitable for those using desktop Linux. This mode requires `resolvectl` to be available on the system.

## Configuration

### Quick Start

## Usage

This should be run as a systemd service and timer as it only bases its information of present moment. It will not monitor if you change DNS server settings on your controller, and if using `resolved` mode will not survive a reboot. See [systemd](systemd) for examples of units and timers that can be implemented into your distribution.

Run `zerotier-systemd-manager` as `root` with appropriate command line arguments. If you are connected to ZeroTier networks which have DNS assignments depending on which `-mode` you are running it will:

* `networkd` - Populate files in `/etc/systemd/network/99-<network-name>` . It will then restart `systemd-networkd` for you if things have changed.
* `resolved` - Populate search domains and dns server entries with `resolvectl` if entries don't already exist.

### Command Line Arguments

To change the way the tool operates, you can pass various arguments via the command line.

* `-mode`: Set the mode of operation (`networkd` or `resolved`) (default: `networkd`).
* `-log-level`: Set the logging level (`info` or `debug`).
* `-dry-run`: Simulate changes without applying them.
* `-auto-restart`: (networkd) Automatically restart `systemd-networkd` when changes are detected (default: true).
* `-reconcile`: (networkd) Remove unused network files when networks are no longer active (default: true).
* `-dns-over-tls`: Enable DNS-over-TLS for supported configurations (default: false).
* `-multicast-dns`: Enable multicast DNS (mDNS) for ZeroTier interfaces (default: false).
* `-add-reverse-domains`: Add reverse DNS search domains (e.g., `in-addr.arpa`, `ip6.arpa`) based on assigned IPs (default: false).

## Installing

### From Source

Clone this repository and compile with [GoLang 1.21 or later](https://golang.org):

```bash
go build ./cmd/zt-dns-companion/
```

_or_

```bash
go get github.com/nfrastack/zt-dns-companion
```

### Distributions

Placeholder

## Support

### Usage

* The [Discussions board](../../discussions) is a great place for working with the community on tips and tricks
* [Sponsor me](https://tiredofit.ca/sponsor) for personalized support

### Bugfixes

* Please, submit a [Bug Report](issues/new) if something isn't working as expected. I'll do my best to issue a fix in short order.

### Feature Requests

* Feel free to submit a feature request, however there is no guarantee that it will be added, or at what timeline.
* [Sponsor me](https://tiredofit.ca/sponsor) regarding development of features.

### Updates

* Best effort to track upstream dependency changes, More priority if I am actively using the tool.
* [Sponsor me](https://tiredofit.ca/sponsor) for up to date releases.

## References

* [zerotier-systemd-manager](https://github.com/zerotier/zerotier-systemd-manager) - Original forked project by Erik Hollensbe <github@hollensbe.org>
* [zerotier-resolved](https://github.com/twisteroidambassador/zerotier-resolved) - Adds `resolvectl` settings via python script
* [zeronsd](https://github.com/zerotier/zeronsd) DNS server that maps member IDs and names to IP addresses on an ZeroTier network.

## License

BSD 3-Clause. See [LICENSE](LICENSE) for more details.
