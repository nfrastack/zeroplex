# NixOS

This project provides a Nix flake that allows you to build, run, and configure ZeroPlex. Below are instructions on how to use it within Nix and NixOS.

## Adding as an Input

To use this flake as an input in your own flake, add the following to your `flake.nix`:

```nix
{
  inputs.zeroplex.url = "github:nfrastack/zeroplex";

  outputs = { self, nixpkgs, zeroplex }: {
    packages.default = zeroplex.packages.${system}.default;
  };
}
```

### NixOS Module

This flake provides a NixOS module that allows you to configure and run ZeroPlex as a systemd service. To use it, add the following to your `configuration.nix`:

```nix
{
  imports = [
    inputs.zeroplex.nixosModules.default
  ];
}
```

#### Available Options

Here are the available options for the NixOS module (`services.zeroplex`):

* `enable` (bool): Enable or disable the ZeroPlex service.
* `package` (package): The package to use for the service. Default: the flake's Go build.
* `configFile` (str): Path to the YAML configuration file. Default: `/etc/zeroplex.yml`
* `mode` (str): Backend mode. One of `"auto"`, `"networkd"`, or `"resolved"`.
* `log` (attrs): Logging configuration.
  * `level` (str): Logging level (`"error"`, `"warn"`, `"info"`, `"verbose"`, `"debug"`, `"trace"`).
  * `type` (str): Logging output type (`"console"`, `"file"`, or `"both"`).
  * `file` (str): Log file path (used if type is `file` or `both`).
  * `timestamps` (bool): Enable timestamps in logs.
* `daemon` (attrs): Daemon mode configuration.
  * `enabled` (bool): Run in daemon mode (true/false).
  * `poll_interval` (str): Polling interval (e.g., `"1m"`).
* `client` (attrs): ZeroTier client API configuration.
  * `host` (str): ZeroTier client host address.
  * `port` (int): ZeroTier client port.
  * `token_file` (str): Path to ZeroTier API token file.
* `features` (attrs): Feature toggles.
  * `dns_over_tls` (bool): Prefer DNS-over-TLS.
  * `add_reverse_domains` (bool): Add ip6.arpa and in-addr.arpa search domains.
  * `multicast_dns` (bool): Enable Multicast DNS (mDNS).
  * `restore_on_exit` (bool): Restore DNS for all managed interfaces on exit.
* `networkd` (attrs): systemd-networkd integration options.
  * `auto_restart` (bool): Automatically restart systemd-networkd when things change.
  * `reconcile` (bool): Remove left networks from systemd-networkd configuration.
* `interface_watch` (attrs): Interface watcher configuration.
  * `mode` (str): Interface watch mode (`"event"`, `"poll"`, `"off"`).
  * `retry` (attrs): Retry configuration.
    * `count` (int): Number of retries after interface event.
    * `delay` (str): Delay between retries (duration string).
* `profile` (str): The profile to load for the zeroplex service. This should match one of the keys in the `profiles` option. If not specified, the default profile will be used.
* `profiles` (attrs): Additional named profiles for advanced filtering and configuration. Each profile is a nested attribute set with the same structure as the top-level config. Supports advanced filtering via the `filters` key.

See the [configuration.nix](./configuration.nix) for comprehensive examples of advanced filtering and profile usage.

This setup allows you to fully configure and manage the ZeroPlex service declaratively using NixOS.
