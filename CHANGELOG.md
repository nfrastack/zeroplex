## 2.0.0 2025-06-23 <code at nfrastack dot com>

This brings a whole new rewrite to the application, including a project name change. 

### Added
- Broke out application into modular packages instead of a single main.go file.
  - Refactored codebase into modular packages for better maintainability
    - New package structure:
      - `pkg/config` - Configuration management and validation
      - `pkg/logger` - Centralized logging functionality
      - `pkg/client` - ZeroTier API client handling
      - `pkg/utils` - Common utility functions
      - `pkg/filters` - Network filtering logic
      - `pkg/dns` - DNS-related functionality
      - `pkg/modes` - Network mode handlers (networkd, resolved)
- BREAKING - Switched configuration format from TOML to YAML.
- BREAKING - Refactored configuration to use a modern, nested structure (e.g., `log.level`, `daemon.enabled`, `client.host`, `features.dns_over_tls`, etc.).
- Added startup banner for application launch.
- Introduced daemon mode for background operation and configurable Poll Interval to check ZeroTier API
- New config option and CLI flag: `restore_on_exit` / `--restore-on-exit` to restore original DNS settings for all managed interfaces on exit (SIGINT/shutdown).
- Added support for logging to file/console/both via `log.type` and `log.file` in config and CLI flags.
- Improved logging: now supports multiple levels (`info`, `verbose` (default), `error`, `debug`, `trace`).
- Added interface watching modes (`event`, `poll`, `off`) with to trigger settings change outside of daemon polling Interval with batching/debouncing of events.
- Improved systemd-resolved support: mDNS and DNS-over-TLS are now actually set via `resolvectl` for each managed interface, matching the behavior of systemd-networkd mode.
- Rewrote filter configuration for clarity and flexibility.
- Improved DNS cleanup: now uses `resolvectl revert <interface>` to reset all temporary DNS settings for interfaces on removal or exit
- Tonnes of other little improvements and QoL improvements

## 1.0.0 2025-05-08 <code at nfrastack dot com>

Inaugral release of the ZT DNS Companion!
This tool will augment the amazing capabilities of working with the ZeroTier one on a Linux headless or desktop system.
The goal is to provide the opportunity for DNS resolution of netowrk advertised DNS servers and search domains per joined network.
There are a few other projects which perform similar functions, this felt like an opportunity to build a tool to support multiple backends and introduce various quality of life features.

   ### Added
      - Run with its own application defaults to detect the right backend (systemd-networkd or systemd-resolved) to use.
      - Configuration file support with defaults and seperate profile support
      - Command line override functionality for config and app defaults
      - Sparse (info) or very detailed (debug) logging
      - Support for cleaning up old data that is no longer relevant if networks are unjoined
      - Ability to choose custom port, zerotier-one API location, authorization token
      - Ability to execute without performing changes (dry-run)
      - Support utilizing DNS over TLS
      - Support enabling Multicast DNS support
      - Ability to filter operations by Interface, Network Name, or Network ID
      - Single Binary thanks to Go
      - NixOS Module included


