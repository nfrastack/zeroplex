# NixOS

This project provides a Nix flake that allows you to build, run, and configure the ZT DNS Companion. Below are instructions on how to use it within Nix and NixOS.

## Adding as an Input

To use this flake as an input in your own flake, add the following to your `flake.nix`:

```nix
{
  inputs.zt-dns-companion.url = "github:nfrastack/zt-dns-companion";

  outputs = { self, nixpkgs, zt-dns-companion }: {
    packages.default = zt-dns-companion.packages.${system}.default;
  };
}
```

### NixOS Module

This flake provides a NixOS module that allows you to configure and run the ZT DNS Companion as a systemd service. To use it, add the following to your `configuration.nix`:

```nix
{
  imports = [
    inputs.zt-dns-companion.nixosModules.default
  ];

  services.zt-dns-companion = {
    enable = true;
    mode = "networkd";          # Options: "auto", "networkd", "resolved"
    logLevel = "info";          # Logging level: "info" or "debug"
  };
}
```

#### Additional Options

Here are the available options for the NixOS module:

* `enable` (bool): Enable or disable the service.
* `configFile` (str): Configuration file to load. Default `/etc/zt-dns-companion.conf`
* `mode` (enum): Mode of operation. Options: "`"auto"`, `"networkd"`, `"resolved"`. Default: `"auto"`.
* `profile` (str): Specify a profile to load configuration from the configuration file. Default: `null`.
* `port` (int): ZeroTier client port number. Default: `9993`.
* `host` (str): ZeroTier client host address. Default: `"http://localhost"`.
* `tokenFile` (str): Path to the ZeroTier authentication token file. Default: `"/var/lib/zerotier-one/authtoken.secret"`.
* `addReverseDomains` (bool): Add ip6.arpa and in-addr.arpa search domains. Default: `false`.
* `autoRestart` (bool): Automatically restart systemd-resolved when things change. Default: `true`.
* `dnsOverTLS` (bool): Prefer DNS-over-TLS. Requires ZeroNSd v0.4 or better. Default: `false`.
* `dryRun` (bool): Simulate changes without applying them. Default: `false`.
* `multicastDNS` (bool): Enable mDNS resolution on the ZeroTier interface. Default: `false`.
* `reconcile` (bool): Automatically remove left networks from systemd-networkd configuration. Default: `true`.
* `logLevel` (enum): Logging level. Options: `"info"`, `"debug"`. Default: `"info"`.
* `logTimestamps` (bool): Log TimeStamps (YYYY-MM-DD HH:MM:SS) Default: `false`.
* `network` (list of str): List of ZeroTier networks to operate on. Default: `[]`.
* `network_id` (list of str): Alias for `network`. Default: `[]`.
* `interface` (list of str): List of network interfaces to use. Default: `[]`.
* `exclude` (list of str): List of networks or interfaces to exclude. Default: `[]`.
* `restoreOnExit` (bool): Restore original DNS settings for all managed interfaces on exit. Default: `false`.

This setup allows you to fully configure and manage the ZT DNS Companion service declaratively using NixOS.
