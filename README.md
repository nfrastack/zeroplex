# ZeroPlex

Automate per-interface DNS configuration for [ZeroTier](https://zerotier.com) networks on Linux. ZeroPlex detects DNS assignments from your ZeroTier controller and applies them to your system using either `systemd-networkd` or `systemd-resolved`, supporting both server and desktop environments. It is designed for reliability, automation, and seamless integration with modern Linux workflows.

> **Commercial/Enterprise Users:**
>
> This tool is free to use for all users. However, if you are using ZeroPlex in a commercial or enterprise environment, please consider purchasing a license to support ongoing development and receive priority support. There is no charge to use the tool and no differences in binaries, but a license purchase helps ensure continued improvements and faster response times for your organization. If this is useful to your organization and you wish to support the project [please reach out](mailto:code+zp@nfrastack.com).

## Disclaimer

ZeroPlex is an independent project and is not affiliated with, endorsed by, or sponsored by ZeroTier, Inc. Any references to ZeroTier are solely for the purpose of describing the functionality of this tool, which is designed to enhance the usage of the ZeroTier product. This tool is provided as-is and is not an official ZeroTier product. I'm also not a lawyer, so if you represent commercial interests of companies above and have concerns, let's talk.

## Maintainer

nfrastack <code@nfrastack.com>

## Table of Contents

- [Disclaimer](#disclaimer)
- [Maintainer](#maintainer)
- [Prerequisites and Assumptions](#prerequisites-and-assumptions)
- [Installing](#installing)
  - [From Source](#from-source)
  - [Precompiled Binaries](#precompiled-binaries)
  - [Distributions](#distributions)
- [Configuration](#configuration)
  - [Overview](#overview)
  - [Command Line Flags](#command-line-flags)
  - [Profiles](#profiles)
- [Running as a Service](#running-as-a-service)
- [Support](#support)
- [References](#references)
- [License](#license)

## Prerequisites and Assumptions

- ZeroTier-One client installed and connected to one or more networks.
- An available DNS server to serve records.
- Linux system using either:
  - `systemd-networkd` (for servers/headless)
  - `systemd-resolved` (for desktops, works with NetworkManager, ConnMan, iwd, etc.)

## Installing

### From Source

```bash
go build ./cmd/zeroplex/
```

### Precompiled Binaries

Download from [GitHub Releases](https://github.com/nfrastack/zeroplex/releases).

### Distributions

#### NixOS

See [contrib/nixos](contrib/nixos) for installation instructions and NixOS module usage.

---

## Configuration

### Overview

ZeroPlex now uses a modern, nested YAML configuration structure. All options are grouped under logical keys (e.g., `log.level`, `daemon.enabled`, `client.host`, `features.dns_over_tls`).

**Configuration file search order:**
- If you specify a config file with `-config-file`, that file is used.
- If not specified, ZeroPlex will look for `zeroplex.yml` in the current working directory.
- If not found, it will look for `/etc/zeroplex.yml`.
- If no config file is found, ZeroPlex will print a warning and proceed with only command-line arguments and built-in defaults. All CLI flags will still work and take precedence.
- See the sample config [contrib/config/zeroplex.yml.sample]contrib/config/zeroplex.yml.sample) for a full example.


### Command Line Flags

ZeroPlex can be configured via command line flags or a YAML configuration file. Command line flags always override config file values. Profiles allow you to maintain multiple configuration sets in a single file.

> **Tip:** For boolean flags (such as `-dns-over-tls`, `-daemon`, etc.), you can explicitly set them to true or false using `-flag=true` or `-flag=false`. For example:
>
> ```bash
> zeroplex -dns-over-tls=false -multicast-dns=true
> ```
> This will override any value from the config file or defaults.

| Flag                            | Description                                                              | Default                                  |
| ------------------------------- | ------------------------------------------------------------------------ | ---------------------------------------- |
| **General Options**             |                                                                          |                                          |
| `-config-file` / `-config`/`-c` | Path to YAML configuration file                                          | `/etc/zeroplex.yml`                      |
| `-profile`                      | Profile to use from configuration file (must match a key in `profiles:`) | `default`                                |
| `-mode`                         | Backend mode: `auto`, `networkd`, or `resolved`                          | `auto`                                   |
| `-daemon`                       | Run in daemon mode (true/false)                                          | `true`                                   |
| `-poll-interval`                | Interval for polling execution (e.g., 1m, 5m, 1h)                        | `1m`                                     |
| `-dry-run`                      | Enable dry-run mode. No changes will be made.                            | `false`                                  |
|                                 |                                                                          |                                          |
| **Logging Options**             |                                                                          |                                          |
| `-log-level`                    | Logging level: `info`, `debug`, `verbose`, `trace`                       | `info`                                   |
| `-log-type`                     | Logging output type: `console`, `file`, `both`                           | `console`                                |
| `-log-file`                     | Log file path (if using file or both)                                    | `/var/log/zeroplex.log`                  |
| `-log-timestamps`               | Enable timestamps in logs                                                | `false`                                  |
|                                 |                                                                          |                                          |
| **Features**                    |                                                                          |                                          |
| `-dns-over-tls`                 | Prefer DNS-over-TLS                                                      | `false`                                  |
| `-add-reverse-domains`          | Add ip6.arpa and in-addr.arpa search domains                             | `false`                                  |
| `-multicast-dns`                | Enable Multicast DNS (mDNS)                                              | `false`                                  |
| `-restore-on-exit`              | Restore DNS for all managed interfaces on exit                           | `false`                                  |
| `-watchdog-ip`                  | IP address to ping for DNS watchdog (default: first DNS server from ZeroTier config) | `null`                                   |
| `-watchdog-interval`            | Interval for DNS watchdog ping (e.g., 1m)                                | `1m`                                     |
| `-watchdog-backoff`             | Backoff intervals after failed ping (comma-separated, e.g., 10s,20s,30s) | `10s,20s,30s`                            |
| `-watchdog-hostname`            | Hostname to resolve for DNS watchdog (disables IP ping if set). Supports `%domain%` placeholder, which is replaced with the ZeroTier DNS domain for each network/interface. | `null`                                   |
| `-watchdog-expected-ip`         | Expected IP address for resolved hostname (enables strict DNS check)      | `null`                                   |
|                                 |                                                                          |                                          |
| **Networkd Options**            |                                                                          |                                          |
| `-auto-restart`                 | Automatically restart systemd-networkd when things change                | `true`                                   |
| `-reconcile`                    | Remove left networks from systemd-networkd configuration                 | `true`                                   |
|                                 |                                                                          |                                          |
| **Interface Watch Options**     |                                                                          |                                          |
| `-interface-watch-mode`         | Interface watch mode: `event`, `poll`, `off`                             | `off`                                    |
| `-interface-watch-retry-count`  | Number of retries after interface event                                  | `10`                                     |
| `-interface-watch-retry-delay`  | Delay between retries (duration string)                                  | `10s`                                    |
|                                 |                                                                          |                                          |
| **ZeroTier Client Options**     |                                                                          |                                          |
| `-host`                         | ZeroTier client host address                                             | `http://localhost`                       |
| `-port`                         | ZeroTier client port                                                     | `9993`                                   |
| `-token-file`                   | Path to ZeroTier API token file                                          | `/var/lib/zerotier-one/authtoken.secret` |
| `-token`                        | ZeroTier API token (overrides `-token-file`)                             |                                          |

> **Note:**
> Flags always override config file values.

### Profiles

Profiles allow you to define multiple configuration sets in a single YAML file under the `profiles:` key. Select a profile using the `-profile` flag or the `profile` config option. Each profile uses the same nested structure as the default config.

**Example:**

```yaml
default:
  mode: auto
  log:
    level: info
    timestamps: false
    type: console
    file: /var/log/zeroplex.log
  daemon:
    enabled: false
    poll_interval: 1m
  client:
    host: http://localhost
    port: 9993
    token_file: /var/lib/zerotier-one/authtoken.secret
  features:
    dns_over_tls: false
    auto_restart: true
    add_reverse_domains: false
    multicast_dns: false
    restore_on_exit: false
  networkd:
    reconcile: true
  interface_watch:
    mode: event
    retry:
      count: 3
      delay: 2s

profiles:
  development:
    log:
      level: debug
      timestamps: true
    daemon:
      enabled: true
      poll_interval: 30s
    interface_watch:
      mode: event
      retry:
        count: 3
        delay: 2s
    restore_on_exit: false
    filters:
      - type: interface
        conditions:
          - value: zt12345678
            logic: or
          - value: zt87654321
            logic: or

  production:
    mode: networkd
    daemon:
      enabled: true
      poll_interval: 5m
    interface_watch:
      mode: poll
      retry:
        count: 2
        delay: 1s
    restore_on_exit: false
    filters:
      - type: network
        conditions:
          - value: prod_network
            logic: or
          - value: mgmt_network
            logic: or
      - type: network
        operation: AND
        negate: true
        conditions:
          - value: test_network
```

## Advanced DNS Watchdog & Interface Watch

ZeroPlex includes advanced reliability features to ensure your ZeroTier DNS/network configuration remains correct, even after suspend/resume, network changes, or DNS hijacking by other software.

### DNS Watchdog

The DNS watchdog periodically checks that DNS is working as expected. There are two modes:

- **IP Ping**: By default, ZeroPlex will ping the first DNS server assigned by ZeroTier, or a custom IP set via `-watchdog-ip`. If the ping fails, ZeroPlex will attempt to reapply the DNS configuration, using a configurable backoff and retry schedule.
- **Hostname Resolution**: For more advanced checks, you can set `-watchdog-hostname` to a DNS name to resolve (e.g., `internal.example.com`). Optionally, set `-watchdog-expected-ip` to require that the resolved IP matches an expected value. This is useful for detecting DNS hijacking, split-horizon DNS issues, or upstream resolver problems. If the check fails, ZeroPlex will reapply the config and retry with backoff.

**Backoff and Retry:**
- The `watchdog_backoff` option lets you specify a list of retry intervals (e.g., `["10s", "30s", "1m"]`). If the watchdog check fails, ZeroPlex will retry at each interval in the list before giving up. This helps avoid hammering the network or DNS server after a failure, and provides a graceful recovery from transient issues.

**Example YAML:**

```yaml
default:
  features:
    watchdog_hostname: internal.example.com
    watchdog_expected_ip: 10.10.10.10
    watchdog_interval: 1m
    watchdog_backoff: ["10s", "30s", "1m"]
```

### Interface Watch

ZeroPlex can monitor ZeroTier interfaces for changes (appearance/disappearance, up/down, etc.) using either event-based or polling modes. This is critical for reliability on laptops and desktops, where suspend/resume or network manager actions can disrupt virtual interfaces. If an interface reappears, ZeroPlex will automatically reapply the correct DNS/network configuration.

- `-interface-watch-mode`: Set to `event` (recommended), `poll`, or `off`.
- `-interface-watch-retry-count` and `-interface-watch-retry-delay`: Control how many times and how quickly to retry after an interface event.

---

## Running as a Service

ZeroPlex is designed to run as a background service. See [contrib/systemd](contrib/systemd) for example systemd units.
A NixOS module is also available for declarative configuration ([contrib/nixos](contrib/nixos)).

## Support

### Implementation

[Contact us](mailto:code+zeroplex@nfrastack.com) for rates.

### Usage

- The [Discussions board](discussions) is a great place for working with the community.

### Bugfixes

- Please submit a [Bug Report](issues/new) if something isn't working as expected. I'll do my best to issue a fix in short order.

### Feature Requests

- Feel free to submit a feature request; however, there is no guarantee that it will be added or at what timeline.  [Contact us](mailto:code+zp@nfrastack.com) for custom development.

### Updates

- Best effort to track upstream dependency changes, with more priority if the tool is actively used on our end.

## References

- [zerotier-systemd-manager](https://github.com/zerotier/zerotier-systemd-manager)
- [zerotier-resolved](https://github.com/twisteroidambassador/zerotier-resolved)
- [zeronsd](https://github.com/zerotier/zeronsd)

## License

BSD 3-Clause. See [LICENSE](LICENSE) for details.
