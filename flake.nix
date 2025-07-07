{
  description = "Manage ZeroTier DNS with systemd-resolved or networkd";

  inputs = { nixpkgs.url = "nixpkgs/nixos-unstable"; };


  outputs = { self, nixpkgs }:
    let
      version = "2.1.0";
      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
      ];

      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
      nixpkgsFor = forAllSystems (system: import nixpkgs { inherit system; });
    in {
      packages = forAllSystems (system:
        let pkgs = nixpkgsFor.${system};
        in {
          zeroplex = pkgs.buildGoModule {
            pname = "zeroplex";
            inherit version;
            src = ./.;

            meta = {
              description = "Manage ZeroTier DNS with systemd-resolved or networkd";
              homepage = "https://github.com/nfrastack/zeroplex";
              license = "BSD-3";
              maintainers = [
                {
                  name = "nfrastack";
                  email = "code@nfrastack.com";
                  github = "nfrastack";
                }
              ];
            };

            ldflags = [
              "-s"
              "-w"
              "-X main.Version=${version}"
            ];

            vendorHash = "sha256-QYYExOIcFBaaYIq6miZaOdQJnd4p/rv2fTULRACAQWI=";
          };
        });

      devShells = forAllSystems (system:
        let pkgs = nixpkgsFor.${system};
        in pkgs.mkShell {
          buildInputs = with pkgs; [
            make
            go
          ];
        });

      defaultPackage = forAllSystems (system: self.packages.${system}.zeroplex);

      nixosModules.default = { config, lib, pkgs, ... }:
        let
          cfg = config.services.zeroplex;

          # Utility function to get directory part of path
          getDir = path:
            let
              components = builtins.match "(.*)/.*" path;
            in
              if components == null then "." else builtins.head components;
        in {
          options.services.zeroplex = {
            enable = lib.mkEnableOption {
              default = false;
              description = "Enable the ZeroPlex module to configure the tool.";
            };

            service.enable = lib.mkOption {
              type = lib.types.bool;
              default = true;
              description = "Enable the systemd service for ZeroPlex.";
            };

            package = lib.mkOption {
              type = lib.types.package;
              default = self.packages.${pkgs.system}.zeroplex;
              description = "ZeroPlex package to use.";
            };

            configFile = lib.mkOption {
              type = lib.types.str;
              default = "/etc/zeroplex.yml";
              description = "Path to the YAML configuration file for ZeroPlex.";
            };

            mode = lib.mkOption {
              type = lib.types.enum [ "auto" "networkd" "resolved" ];
              default = "auto";
              description = "Mode of operation (autodetected, networkd or resolved).";
            };
            log = lib.mkOption {
              type = lib.types.submodule {
                options = {
                  level = lib.mkOption {
                    type = lib.types.enum [ "error" "warn" "info" "verbose" "debug" "trace" ];
                    default = "verbose";
                    description = "Set the logging level (error, warn, info, verbose, debug, or trace).";
                  };
                  type = lib.mkOption {
                    type = lib.types.str;
                    default = "console";
                    description = "Set the logging type (console, file, or both).";
                  };
                  file = lib.mkOption {
                    type = lib.types.str;
                    default = "/var/log/zeroplex.log";
                    description = "Set the log file path (used if log type is file or both).";
                  };
                  timestamps = lib.mkOption {
                    type = lib.types.bool;
                    default = false;
                    description = "Log timestamps (YYYY-MM-DD HH:MM:SS).";
                  };
                };
              };
              default = {
                level = "verbose";
                type = "console";
                file = "/var/log/zeroplex.log";
                timestamps = false;
              };
              description = "Logging configuration (level, type, file, timestamps).";
            };
            daemon = lib.mkOption {
              type = lib.types.submodule {
                options = {
                  enabled = lib.mkOption {
                    type = lib.types.bool;
                    default = true;
                    description = "Default to daemon mode.";
                  };
                  poll_interval = lib.mkOption {
                    type = lib.types.str;
                    default = "1m";
                    description = "Polling interval.";
                  };
                };
              };
              default = {
                enabled = true;
                poll_interval = "1m";
              };
              description = "Daemon configuration.";
            };
            client = lib.mkOption {
              type = lib.types.submodule {
                options = {
                  host = lib.mkOption {
                    type = lib.types.str;
                    default = "http://localhost";
                    description = "ZeroTier client host address.";
                  };
                  port = lib.mkOption {
                    type = lib.types.int;
                    default = 9993;
                    description = "ZeroTier client port number.";
                  };
                  token_file = lib.mkOption {
                    type = lib.types.str;
                    default = "/var/lib/zerotier-one/authtoken.secret";
                    description = "Path to the ZeroTier authentication token file.";
                  };
                };
              };
              default = {
                host = "http://localhost";
                port = 9993;
                token_file = "/var/lib/zerotier-one/authtoken.secret";
              };
              description = "Client configuration.";
            };
            features = lib.mkOption {
              type = lib.types.submodule {
                options = {
                  dns_over_tls = lib.mkOption {
                    type = lib.types.bool;
                    default = false;
                    description = "Prefer DNS-over-TLS.";
                  };
                  add_reverse_domains = lib.mkOption {
                    type = lib.types.bool;
                    default = false;
                    description = "Add ip6.arpa and in-addr.arpa search domains.";
                  };
                  multicast_dns = lib.mkOption {
                    type = lib.types.bool;
                    default = false;
                    description = "Enable mDNS resolution on the ZeroTier interface.";
                  };
                  restore_on_exit = lib.mkOption {
                    type = lib.types.bool;
                    default = false;
                    description = "Restore original DNS settings for all managed interfaces on exit.";
                  };
                  watchdog_ip = lib.mkOption {
                    type = lib.types.nullOr lib.types.str;
                    default = null;
                    description = "IP address to ping for DNS watchdog (default: first DNS server from ZeroTier config).";
                  };
                  watchdog_hostname = lib.mkOption {
                    type = lib.types.nullOr lib.types.str;
                    default = null;
                    description = "DNS hostname to resolve for DNS watchdog (optional, enables hostname mode).";
                  };
                  watchdog_expected_ip = lib.mkOption {
                    type = lib.types.nullOr lib.types.str;
                    default = null;
                    description = "Expected IP address for resolved hostname (optional, for split-horizon/hijack detection).";
                  };
                  watchdog_interval = lib.mkOption {
                    type = lib.types.str;
                    default = "1m";
                    description = "Interval for DNS watchdog ping (e.g., 1m).";
                  };
                  watchdog_backoff = lib.mkOption {
                    type = lib.types.listOf lib.types.str;
                    default = [ "10s" "30s" "1m" "2m" ];
                    description = "Backoff intervals after failed ping (e.g., [10s, 30s, 1m, 2m]).";
                  };
                };
              };
              default = {
                dns_over_tls = false;
                add_reverse_domains = false;
                multicast_dns = false;
                restore_on_exit = false;
                watchdog_ip = null;
                watchdog_hostname = null;
                watchdog_expected_ip = null;
                watchdog_interval = "1m";
                watchdog_backoff = [ "10s" "30s" "1m" "2m" ];
              };
              description = "Feature toggles.";
            };
            interface_watch = lib.mkOption {
              type = lib.types.submodule {
                options = {
                  mode = lib.mkOption {
                    type = lib.types.str;
                    default = "event";
                    description = "Interface watch mode (event, poll, off).";
                  };
                  retry = lib.mkOption {
                    type = lib.types.submodule {
                      options = {
                        count = lib.mkOption {
                          type = lib.types.int;
                          default = 10;
                          description = "Number of retries after interface event.";
                        };
                        delay = lib.mkOption {
                          type = lib.types.str;
                          default = "10s";
                          description = "Delay between retries (duration string).";
                        };
                        backoff = lib.mkOption {
                          type = lib.types.listOf lib.types.str;
                          default = [ "1s" "2s" "5s" "10s" "15s" "30s" "30s" ];
                          description = "Backoff intervals for interface event retries (overrides delay/count if set).";
                        };
                        max_total = lib.mkOption {
                          type = lib.types.str;
                          default = "2m";
                          description = "Maximum total time to spend retrying interface readiness (duration string).";
                        };
                      };
                    };
                    default = {
                      count = 10;
                      delay = "10s";
                      backoff = [ "1s" "2s" "5s" "10s" "15s" "30s" "30s" ];
                      max_total = "2m";
                    };
                    description = "Retry configuration.";
                  };
                };
              };
              default = {
                mode = "event";
                retry = {
                  count = 10;
                  delay = "10s";
                  backoff = [ "1s" "2s" "5s" "10s" "15s" "30s" "30s" ];
                  max_total = "2m";
                };
              };
              description = "Interface watch configuration.";
            };
            networkd = lib.mkOption {
              type = lib.types.submodule {
                options = {
                  auto_restart = lib.mkOption {
                    type = lib.types.bool;
                    default = true;
                    description = "Automatically restart systemd-networkd when things change.";
                  };
                  reconcile = lib.mkOption {
                    type = lib.types.bool;
                    default = true;
                    description = "Automatically remove left networks from systemd-networkd configuration.";
                  };
                };
              };
              default = {
                auto_restart = true;
                reconcile = true;
              };
              description = "Networkd configuration.";
            };

            # Nested profiles support
            profiles = lib.mkOption {
              type = with lib.types; attrsOf (attrsOf anything);
              default = {};
              description = "Additional profiles for the zeroplex configuration using advanced filtering. Each profile is an attribute set where the key is the profile name and the value is a nested attribute set of options for that profile.";
            };

            profile = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = "The profile to load for the zeroplex service. This should match one of the keys in the `profiles` option. If not specified, the default profile will be used.";
            };
          };

          config = lib.mkIf cfg.enable {
            environment.systemPackages = [ cfg.package ];

            # Always write configuration file when service is enabled
            system.activationScripts.zeroplex-config = {
              text = ''
                if [ ! -e "${getDir cfg.configFile}" ]; then
                  mkdir -p "${getDir cfg.configFile}"
                fi
                cat > ${cfg.configFile} << 'EOC'
# ZeroPlex Configuration

default:
  mode: "${cfg.mode}"
  log:
    level: "${cfg.log.level}"
    type: "${cfg.log.type}"
    file: "${cfg.log.file}"
    timestamps: ${if cfg.log.timestamps then "true" else "false"}
  daemon:
    enabled: ${if cfg.daemon.enabled then "true" else "false"}
    poll_interval: "${cfg.daemon.poll_interval}"
  client:
    host: "${cfg.client.host}"
    port: ${toString cfg.client.port}
    token_file: "${cfg.client.token_file}"
  features:
    dns_over_tls: ${if cfg.features.dns_over_tls then "true" else "false"}
    add_reverse_domains: ${if cfg.features.add_reverse_domains then "true" else "false"}
    multicast_dns: ${if cfg.features.multicast_dns then "true" else "false"}
    restore_on_exit: ${if cfg.features.restore_on_exit then "true" else "false"}
    watchdog_ip: ${if cfg.features.watchdog_ip != null then "\"${cfg.features.watchdog_ip}\"" else "null"}
    watchdog_hostname: ${if cfg.features.watchdog_hostname != null then "\"${cfg.features.watchdog_hostname}\"" else "null"}
    watchdog_expected_ip: ${if cfg.features.watchdog_expected_ip != null then "\"${cfg.features.watchdog_expected_ip}\"" else "null"}
    watchdog_interval: "${cfg.features.watchdog_interval}"
    watchdog_backoff: [${lib.concatStringsSep ", " (map (x: "\"${x}\"") cfg.features.watchdog_backoff)}]
  interface_watch:
    mode: "${cfg.interface_watch.mode}"
    retry:
      count: ${toString cfg.interface_watch.retry.count}
      delay: "${cfg.interface_watch.retry.delay}"
      backoff: [${lib.concatStringsSep ", " (map (x: "\"${x}\"") cfg.interface_watch.retry.backoff)}]
      max_total: "${cfg.interface_watch.retry.max_total}"
  networkd:
    auto_restart: ${if cfg.networkd.auto_restart then "true" else "false"}
    reconcile: ${if cfg.networkd.reconcile then "true" else "false"}

${lib.concatStringsSep "\n" (lib.mapAttrsToList (name: profile: ''
${name}:
${lib.generators.toYAML {} profile}
'') cfg.profiles)}EOC
                chmod 0600 ${cfg.configFile}
              '';
              deps = [];
            };

            systemd.services.zeroplex = lib.mkIf cfg.service.enable {
              description = "ZeroPlex - ZeroTier DNS Manager";
              wantedBy = [ "multi-user.target" ];
              restartTriggers = [
                cfg.package
                cfg.configFile
              ];
              serviceConfig = {
                ExecStart =
                  let
                    configFileArg = "--config-file ${cfg.configFile}";
                    profileArg = if cfg.profile != "" then "--profile ${cfg.profile}" else "";
                    logTimestampsArg = "--log-timestamps=false";
                    bannerArg = "--banner=false";
                    args = lib.strings.concatStringsSep " " (lib.lists.filter (s: s != "") [
                      "${cfg.package}/bin/zeroplex"
                      configFileArg
                      profileArg
                      logTimestampsArg
                      bannerArg
                    ]);
                  in args;
                User = "root";
                Group = "root";
                RestartSec = "10s";
                Restart = "always";
                StandardOutput = "journal";
                StandardError = "journal";
                SyslogIdentifier = "zeroplex";
                # Ensure D-Bus/system bus access for sleep/resume detection
                Environment = "DBUS_SYSTEM_BUS_ADDRESS=unix:path=/run/dbus/system_bus_socket";
                PrivateTmp = false;
                PrivateDevices = false;
                PrivateNetwork = false;
              };
            };
            services.dbus.packages = [
    (pkgs.stdenv.mkDerivation {
      name = "zeroplex-dbus-policy";
      src = null;
      dontUnpack = true;
      installPhase = ''
        mkdir -p $out/etc/dbus-1/system.d
        cat > $out/etc/dbus-1/system.d/zeroplex-login1.conf <<EOF
<!DOCTYPE busconfig PUBLIC
 "-//freedesktop//DTD D-Bus Bus Configuration 1.0//EN"
 "http://www.freedesktop.org/standards/dbus/1.0/busconfig.dtd">
<busconfig>
  <policy user="root">
    <allow send_destination="org.freedesktop.login1"/>
    <allow receive_sender="org.freedesktop.login1"/>
    <allow send_interface="org.freedesktop.login1"/>
    <allow receive_interface="org.freedesktop.login1"/>
    <allow send_interface="org.freedesktop.login1.Manager"/>
    <allow receive_interface="org.freedesktop.login1.Manager"/>
    <allow send_type="signal"/>
    <allow receive_type="signal"/>
    <allow own="org.freedesktop.login1"/>
  </policy>
</busconfig>
EOF
      '';
    })
  ];
          };
        };
    };
}
