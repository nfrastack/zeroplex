{
  description = "Manage ZeroTier DNS with systemd-resolved or networkd";

  inputs = { nixpkgs.url = "nixpkgs/nixos-unstable"; };


  outputs = { self, nixpkgs }:
    let
      version = "2.0.0-beta";
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
          zeroflex = pkgs.buildGoModule {
            pname = "zeroflex";
            inherit version;
            src = ./.;

            meta = {
              description = "Manage ZeroTier DNS with systemd-resolved or networkd";
              homepage = "https://github.com/nfrastack/zeroflex";
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

            vendorHash = "sha256-7WVmKZFonfqcItQ9qTR6f+ty2KaGTTZutMaedOkidDU=";
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

      defaultPackage = forAllSystems (system: self.packages.${system}.zeroflex);

      nixosModules.default = { config, lib, pkgs, ... }:
        let
          cfg = config.services.zeroflex;

          # Utility function to get directory part of path
          getDir = path:
            let
              components = builtins.match "(.*)/.*" path;
            in
              if components == null then "." else builtins.head components;
        in {
          options.services.zeroflex = {
            enable = lib.mkEnableOption {
              default = false;
              description = "Enable the ZeroTier DNS Companion module to configure the tool.";
            };

            service.enable = lib.mkOption {
              type = lib.types.bool;
              default = true;
              description = "Enable the systemd service for ZeroTier DNS Companion.";
            };

            package = lib.mkOption {
              type = lib.types.package;
              default = self.packages.${pkgs.system}.zeroflex;
              description = "ZeroTier DNS Companion package to use.";
            };

            configFile = lib.mkOption {
              type = lib.types.str;
              default = "/etc/zeroflex.yml";
              description = "Path to the YAML configuration file for ZeroFlex.";
            };

            profiles = lib.mkOption {
              type = with lib.types; attrsOf (attrsOf anything);
              default = {};
              description = ''
                Additional profiles for the zeroflex configuration using advanced filtering.
                Each profile is an attribute set where the key is the profile name
                and the value is an attribute set of options for that profile.

                Profiles inherit values from the default profile unless explicitly overridden.

                filtering options:
                - filters: Array of filter objects with syntax
                  - type: Filter type ("name", "interface", "network", "network_id", "online", "assigned", "address", "route")
                  - operation: How filters combine ("AND", "OR") - defaults to "AND"
                  - negate: Whether to invert the filter result (boolean)
                  - conditions: Array of condition objects
                    - value: Pattern to match (supports wildcards like "prod*")
                    - logic: How conditions combine within a filter ("and", "or") - defaults to "and"

                See contrib/nixos/configuration.nix for comprehensive examples.
              '';
            };

            profile = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = ''
                The profile to load for the zeroflex service. This should match one of the keys in the `profiles` option.
                If not specified, the default profile will be used.
              '';
            };

            mode = lib.mkOption {
              type = lib.types.enum [ "auto" "networkd" "resolved" ];
              default = "auto";
              description = "Mode of operation (autodetected, networkd or resolved).";
            };

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

            logLevel = lib.mkOption {
              type = lib.types.enum [ "error" "warn" "info" "verbose" "debug" "trace" ];
              default = "verbose";
              description = "Set the logging level (error, warn, info, verbose, debug, or trace).";
            };

            dnsOverTLS = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Prefer DNS-over-TLS.";
            };

            autoRestart = lib.mkOption {
              type = lib.types.bool;
              default = true;
              description = "Automatically restart systemd-networkd when things change.";
            };

            addReverseDomains = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Add ip6.arpa and in-addr.arpa search domains.";
            };

            logTimestamps = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Log timestamps (YYYY-MM-DD HH:MM:SS).";
            };

            multicastDNS = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Enable mDNS resolution on the ZeroTier interface.";
            };

            reconcile = lib.mkOption {
              type = lib.types.bool;
              default = true;
              description = "Automatically remove left networks from systemd-networkd configuration.";
            };

            tokenFile = lib.mkOption {
              type = lib.types.str;
              default = "/var/lib/zerotier-one/authtoken.secret";
              description = "Path to the ZeroTier authentication token file.";
            };

            daemonMode = lib.mkOption {
              type = lib.types.bool;
              default = true;
              description = "Run in daemon mode with periodic execution.";
            };

            pollInterval = lib.mkOption {
              type = lib.types.str;
              default = "1m";
              description = "Interval for polling execution (e.g., 1m, 5m, 1h, 1d).";
            };

            restoreOnExit = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Restore original DNS settings for all managed interfaces on exit.";
            };

            interfaceWatch = lib.mkOption {
              type = lib.types.attrs;
              default = {
                mode = "event";
                retry = {
                  count = 5;
                  delay = "10s";
                };
              };
              description = "Interface watch configuration (mode, retry).";
            };

          };

          config = lib.mkIf cfg.enable {
            environment.systemPackages = [ cfg.package ];

            # Always write configuration file when service is enabled
            system.activationScripts.zeroflex-config = {
              text = ''
                if [ ! -e "${getDir cfg.configFile}" ]; then
                  mkdir -p "${getDir cfg.configFile}"
                fi
                cat > ${cfg.configFile} << 'EOC'
# ZeroFlex Configuration

default:
  mode: "${cfg.mode}"
  log_level: "${cfg.logLevel}"
  log_timestamps: ${if cfg.logTimestamps then "true" else "false"}
  daemon_mode: ${if cfg.daemonMode then "true" else "false"}
  poll_interval: "${cfg.pollInterval}"
  interface_watch:
    mode: "${cfg.interfaceWatch.mode}"
    retry:
      count: ${toString cfg.interfaceWatch.retry.count}
      delay: "${cfg.interfaceWatch.retry.delay}"
  host: "${cfg.host}"
  port: ${toString cfg.port}
  token_file: "${cfg.tokenFile}"
  dns_over_tls: ${if cfg.dnsOverTLS then "true" else "false"}
  auto_restart: ${if cfg.autoRestart then "true" else "false"}
  add_reverse_domains: ${if cfg.addReverseDomains then "true" else "false"}
  multicast_dns: ${if cfg.multicastDNS then "true" else "false"}
  reconcile: ${if cfg.reconcile then "true" else "false"}
  restore_on_exit: ${if cfg.restoreOnExit then "true" else "false"}
EOC
                chmod 0600 ${cfg.configFile}
              '';
              deps = [];
            };

            systemd.services.zeroflex = lib.mkIf cfg.service.enable {
              description = "ZeroTier DNS Companion";
              wantedBy = [ "multi-user.target" ];
              restartTriggers = [
                cfg.package
                "${cfg.package.outPath}"
              ];
              serviceConfig = {
                ExecStart =
                  let
                    configFileArg = "--config-file ${cfg.configFile}";
                    profileArg = if cfg.profile != "" then "--profile ${cfg.profile}" else "";
                    logTimestampsArg = "--log-timestamps=false";
                    args = lib.strings.concatStringsSep " " (lib.lists.filter (s: s != "") [
                      "${cfg.package}/bin/zeroflex"
                      configFileArg
                      profileArg
                      logTimestampsArg
                    ]);
                  in args;
                User = "root";
                Group = "root";
                RestartSec = "10s";
                Restart = "always";
                StandardOutput = "journal";
                StandardError = "journal";
                SyslogIdentifier = "zeroflex";
              };
            };
          };
        };
    };
}
