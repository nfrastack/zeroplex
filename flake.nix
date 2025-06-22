{
  description = "Manage ZeroTier DNS with systemd-resolved or networkd";

  inputs = { nixpkgs.url = "nixpkgs/nixos-unstable"; };


  outputs = { self, nixpkgs }:
    let
      version = "1.1.0-beta";
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
          zt-dns-companion = pkgs.buildGoModule {
            pname = "zt-dns-companion";
            inherit version;
            src = ./.;

            meta = {
              description = "Manage ZeroTier DNS with systemd-resolved or networkd";
              homepage = "https://github.com/nfrastack/zt-dns-companion";
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

            vendorHash = "sha256-nPpIyHWWigOA1ts6mlN58KDuzXp2pHZglupuFN9+PDQ=";
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

      defaultPackage = forAllSystems (system: self.packages.${system}.zt-dns-companion);

      nixosModules.default = { config, lib, pkgs, ... }:
        let
          cfg = config.services.zt-dns-companion;

          # Utility function to get directory part of path
          getDir = path:
            let
              components = builtins.match "(.*)/.*" path;
            in
              if components == null then "." else builtins.head components;
        in {
          options.services.zt-dns-companion = {
            enable = lib.mkEnableOption {
              default = false;
              description = "Enable the ZeroTier DNS Companion module to configure the tool.";
            };

            service.enable = lib.mkOption {
              type = lib.types.bool;
              default = true;
              description = "Enable the systemd service for ZeroTier DNS Companion.";
            };

            service.timerInterval = lib.mkOption {
              type = lib.types.str;
              default = "1m";
              description = "Interval for the systemd timer (e.g., 1m, 5m, 1h).";
            };

            package = lib.mkOption {
              type = lib.types.package;
              default = self.packages.${pkgs.system}.zt-dns-companion;
              description = "ZeroTier DNS Companion package to use.";
            };

            configFile = lib.mkOption {
              type = lib.types.str;
              default = "/etc/zt-dns-companion.yaml";
              description = "Path to the YAML configuration file for ZT DNS Companion.";
            };

            profiles = lib.mkOption {
              type = with lib.types; attrsOf (attrsOf anything);
              default = {};
              description = ''
                Additional profiles for the zt-dns-companion configuration using advanced filtering.
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
                The profile to load for the zt-dns-companion service. This should match one of the keys in the `profiles` option.
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

          };

          config = lib.mkIf cfg.enable {
            environment.systemPackages = [ cfg.package ];

            # Only write configuration if the user has explicitly set options beyond defaults
            # or if profiles are defined
            system.activationScripts = lib.mkIf (
              # Only create the file if user set custom options or has profiles
              cfg.mode != "auto" ||
              cfg.logLevel != "verbose" ||
              cfg.host != "http://localhost" ||
              cfg.port != 9993 ||
              cfg.tokenFile != "/var/lib/zerotier-one/authtoken.secret" ||
              cfg.daemonMode != true ||
              cfg.pollInterval != "1m" ||
              (cfg.profiles != {})
            ) {
              zt-dns-companion-config = {
                text = ''
                  if [ ! -e "${getDir cfg.configFile}" ]; then
                    mkdir -p "${getDir cfg.configFile}"
                  fi
                  cat > ${cfg.configFile} << 'EOC'
                  # ZT DNS Companion Configuration

                  default:
                    mode: "${cfg.mode}"
                    log_level: "${cfg.logLevel}"
                    host: "${cfg.host}"
                    port: ${toString cfg.port}
                    dns_over_tls: ${lib.boolToString cfg.dnsOverTLS}
                    auto_restart: ${lib.boolToString cfg.autoRestart}
                    add_reverse_domains: ${lib.boolToString cfg.addReverseDomains}
                    log_timestamps: false
                    multicast_dns: ${lib.boolToString cfg.multicastDNS}
                    reconcile: ${lib.boolToString cfg.reconcile}
                    token_file: "${cfg.tokenFile}"
                    daemon_mode: ${lib.boolToString cfg.daemonMode}
                    poll_interval: "${cfg.pollInterval}"
                    restore_on_exit: ${lib.boolToString cfg.restoreOnExit}

                  ${lib.optionalString (cfg.profiles != {}) ''
                  profiles:
                  ${lib.concatStringsSep "\n" (lib.mapAttrsToList (name: profile: ''
                    ${name}:
                  ${lib.concatStringsSep "\n" (lib.mapAttrsToList (key: value:
                    let
                      yamlKey = lib.replaceStrings ["_"] ["_"] key;  # Keep underscores for YAML
                      yamlValue =
                        if builtins.isBool value then lib.boolToString value
                        else if builtins.isString value then ''"${value}"''
                        else if builtins.isList value then
                          if builtins.length value == 0 then "[]"
                          else "\n${lib.concatMapStringsSep "\n" (item:
                            if builtins.isAttrs item then
                              "        - ${lib.concatStringsSep "\n          " (lib.mapAttrsToList (k: v:
                                "${k}: ${if builtins.isString v then ''"${v}"'' else toString v}"
                              ) item)}"
                            else
                              ''        - "${item}"''
                          ) value}"
                        else toString value;
                    in
                      "      ${yamlKey}: ${yamlValue}"
                  ) profile)}
                  '') cfg.profiles)}
                  ''}
                  EOC
                  chmod 0600 ${cfg.configFile}
                '';
                deps = [];
              };
            };

            systemd.services.zt-dns-companion = lib.mkIf cfg.service.enable {
              description = "ZeroTier DNS Companion";
              wantedBy = [ "multi-user.target" ];
              restartTriggers = [
                cfg.package
                config.environment.etc."${lib.removePrefix "/etc/" cfg.configFile}".source
                # Force restart on any source change by using derivation path
                "${cfg.package.outPath}"
              ];
              serviceConfig = {
                # Only pass the config-file argument if we've actually written a config file
                ExecStart =
                  let
                    needsConfigFile =
                      cfg.mode != "auto" ||
                      cfg.logLevel != "info" ||
                      cfg.host != "http://localhost" ||
                      cfg.port != 9993 ||
                      cfg.tokenFile != "/var/lib/zerotier-one/authtoken.secret" ||
                      cfg.daemonMode != true ||
                      cfg.pollInterval != "1m" ||
                      (cfg.profiles != {});

                    configFileArg = if needsConfigFile then "--config-file ${cfg.configFile}" else "";
                    profileArg = if cfg.profile != "" then "--profile ${cfg.profile}" else "";

                    args = lib.strings.concatStringsSep " " (lib.lists.filter (s: s != "") [
                      "${cfg.package}/bin/zt-dns-companion"
                      configFileArg
                      profileArg
                    ]);
                  in args;
                Type = "oneshot";
                User = "root";
                Group = "root";
                RestartSec = "10s";
                Restart = "always";
                StandardOutput = "journal";
                StandardError = "journal";
                SyslogIdentifier = "zt-dns-companion";
              };
            };
          };
        };
    };
}
