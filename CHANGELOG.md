## 1.1.0-beta 2025-06-20 <dave at tiredofit dot ca>

   ### Added
   ### Changed

## 1.0.0 2025-05-08 <dave at tiredofit dot ca>

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


