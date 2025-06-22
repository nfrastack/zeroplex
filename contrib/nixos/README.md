# NixOS

This project provides a Nix flake that allows you to build, run, and configure ZeroFlex. Below are instructions on how to use it within Nix and NixOS.

## Adding as an Input

To use this flake as an input in your own flake, add the following to your `flake.nix`:

```nix
{
  inputs.zeroflex.url = "github:nfrastack/zeroflex";

  outputs = { self, nixpkgs, zeroflex }: {
    packages.default = zeroflex.packages.${system}.default;
  };
}
```

### NixOS Module

This flake provides a NixOS module that allows you to configure and run ZeroFlex as a systemd service. To use it, add the following to your `configuration.nix`:

```nix
{
  imports = [
    inputs.zeroflex.nixosModules.default
  ];

  services.zeroflex = {
    enable = true;
    mode = "networkd";          # Options: "auto", "networkd", "resolved"
    logLevel = "info";          # Logging level: "info" or "debug"
  };
}
```

#### Additional Options

Here are the available options for the NixOS module:

* `enable` (bool): Enable or disable the service.
* `configFile` (str): Configuration file to load. Default: `/etc/zeroflex.yml`
* `mode` (enum): Mode of operation. Options: `"auto"`, `"networkd"`, `"resolved"`. Default: `"auto"`.
* `profile` (str): Specify a profile to load configuration from the configuration file. Default: `""`.
* `port` (int): ZeroTier client port number. Default: `9993`.
* `host` (str): ZeroTier client host address. Default: `"http://localhost"`.
* `tokenFile` (str): Path to the ZeroTier authentication token file. Default: `"/var/lib/zerotier-one/authtoken.secret"`.
* `addReverseDomains` (bool): Add ip6.arpa and in-addr.arpa search domains. Default: `false`.
* `autoRestart` (bool): Automatically restart systemd-networkd when things change. Default: `true`.
* `dnsOverTLS` (bool): Prefer DNS-over-TLS. Default: `false`.
* `dryRun` (bool): Simulate changes without applying them. Default: `false`.
* `multicastDNS` (bool): Enable mDNS resolution on the ZeroTier interface. Default: `false`.
* `reconcile` (bool): Automatically remove left networks from systemd-networkd configuration. Default: `true`.
* `logLevel` (enum): Logging level. Options: `"error"`, `"warn"`, `"info"`, `"verbose"`, `"debug"`, `"trace"`. Default: `"verbose"`.
* `logTimestamps` (bool): Log timestamps (YYYY-MM-DD HH:MM:SS). Default: `false`.
* `daemonMode` (bool): Run in daemon mode with periodic execution. Default: `true`.
* `pollInterval` (str): Interval for polling execution (e.g., `"1m"`, `"5m"`, `"1h"`). Default: `"1m"`.
* `restoreOnExit` (bool): Restore original DNS settings for all managed interfaces on exit. Default: `false`.
* `profiles` (attrs): Additional profiles for advanced filtering and configuration. Default: `{}`.
* `interfaceWatch` (attrs): Interface watch configuration (mode, retry). Default: `{}`.

This setup allows you to fully configure and manage the ZeroFlex service declaratively using NixOS.
