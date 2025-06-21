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

            vendorHash = "sha256-uqrJspDAvXrSq5E5LM5rbyvd8Jtp7Mr737B59FZgovQ=";
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
              default = "/etc/zt-dns-companion.conf";
              description = "Path to the configuration file for ZT DNS Companion.";
            };

            profiles = lib.mkOption {
              type = with lib.types; attrsOf (attrsOf anything);
              default = {};
              example = {
                example1 = {
                  filterType = "interface";
                  filterInclude = [ "zt12345678" "zt87654321" ];
                  autoRestart = false;
                };
                example2 = {
                  filterType = "network";
                  filterInclude = [ "ztnetwork1" "ztnetwork2" ];
                  mode = "resolved";
                  dnsOverTLS = true;
                };
              };
              description = ''
                Additional profiles for the zt-dns-companion configuration.
                Each profile is an attribute set where the key is the profile name
                and the value is an attribute set of options for that profile.

                Profiles inherit values from the default profile unless explicitly overridden.

                Filtering options:
                - filterType: Type of filter ("interface", "network", "network_id", or "none")
                - filterInclude: List of items to include based on filter type (empty or "any"/"all"/"ignore" means include all)
                - filterExclude: List of items to exclude based on filter type (empty or "none"/"ignore" means exclude nothing)
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
              type = lib.types.enum [ "debug" "info" ];
              default = "info";
              description = "Set the logging level (info or debug).";
            };

            tokenFile = lib.mkOption {
              type = lib.types.str;
              default = "/var/lib/zerotier-one/authtoken.secret";
              description = "Path to the ZeroTier authentication token file.";
            };

            filterType = lib.mkOption {
              type = lib.types.enum [ "interface" "network" "network_id" "none" ];
              default = "none";
              description = "Type of filter to apply (interface, network, network_id, or none).";
            };

            filterInclude = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [];
              description = ''
                List of items to include based on filter-type.
                Empty list or values like "any", "all", or "ignore" mean include all.
              '';
            };

            filterExclude = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [];
              description = ''
                List of items to exclude based on filter-type.
                Empty list or values like "none", or "ignore" mean exclude nothing.
              '';
            };

            addReverseDomains = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Add ip6.arpa and in-addr.arpa search domains.";
            };

            autoRestart = lib.mkOption {
              type = lib.types.bool;
              default = true;
              description = "Automatically restart systemd-networkd when things change.";
            };

            dnsOverTLS = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Automatically prefer DNS-over-TLS. Requires ZeroNSd v0.4 or better.";
            };

            multicastDNS = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Enable mDNS resolution on the zerotier interface.";
            };

            reconcile = lib.mkOption {
              type = lib.types.bool;
              default = true;
              description = "Automatically remove left networks from systemd-networkd configuration.";
            };

          };

          config = lib.mkIf cfg.enable {
            environment.systemPackages = [ cfg.package ];

            # Only write configuration if the user has explicitly set options beyond defaults
            # or if profiles are defined
            system.activationScripts = lib.mkIf (
              # Only create the file if user set custom options or has profiles
              cfg.mode != "auto" ||
              cfg.logLevel != "info" ||
              cfg.host != "http://localhost" ||
              cfg.port != 9993 ||
              cfg.dnsOverTLS != false ||
              cfg.autoRestart != true ||
              cfg.addReverseDomains != false ||
              cfg.multicastDNS != false ||
              cfg.reconcile != true ||
              cfg.filterType != "none" ||
              cfg.filterInclude != [] ||
              cfg.filterExclude != [] ||
              cfg.tokenFile != "/var/lib/zerotier-one/authtoken.secret" ||
              (cfg.profiles != {})
            ) {
              zt-dns-companion-config = {
                text = ''
                  if [ ! -e "${getDir cfg.configFile}" ]; then
                    mkdir -p "${getDir cfg.configFile}"
                  fi
                  cat > ${cfg.configFile} << 'EOC'
                  # Default profile
                  [default]
                  mode = "${cfg.mode}"
                  log_level = "${cfg.logLevel}"
                  host = "${cfg.host}"
                  port = ${toString cfg.port}
                  dns_over_tls = ${lib.boolToString cfg.dnsOverTLS}
                  auto_restart = ${lib.boolToString cfg.autoRestart}
                  add_reverse_domains = ${lib.boolToString cfg.addReverseDomains}
                  multicast_dns = ${lib.boolToString cfg.multicastDNS}
                  reconcile = ${lib.boolToString cfg.reconcile}
                  filter_type = "${cfg.filterType}"
                  filter_include = [${lib.strings.concatMapStringsSep ", " (s: ''"${s}"'') cfg.filterInclude}]
                  filter_exclude = [${lib.strings.concatMapStringsSep ", " (s: ''"${s}"'') cfg.filterExclude}]
                  token_file = "${cfg.tokenFile}"

                  ${lib.concatStringsSep "\n\n" (lib.mapAttrsToList (name: profile: ''
                    # Profile: ${name}
                    [profiles.${name}]
                    ${lib.concatStringsSep "\n" (lib.mapAttrsToList (key: value:
                      if builtins.isBool value then
                        "${lib.replaceStrings ["_"] ["-"] key} = ${lib.boolToString value}"
                      else if builtins.isString value then
                        "${lib.replaceStrings ["_"] ["-"] key} = \"${value}\""
                      else if builtins.isList value then
                        "${lib.replaceStrings ["_"] ["-"] key} = [${lib.strings.concatMapStringsSep ", " (s: ''"${s}"'') value}]"
                      else
                        "${lib.replaceStrings ["_"] ["-"] key} = ${toString value}"
                    ) profile)}
                  '') cfg.profiles)}
                  EOC
                  chmod 0600 ${cfg.configFile}
                '';
                deps = [];
              };
            };

            systemd.services.zt-dns-companion = lib.mkIf cfg.service.enable {
              description = "ZeroTier DNS Companion";
              wantedBy = [ "multi-user.target" ];
              serviceConfig = {
                # Only pass the config-file argument if we've actually written a config file
                ExecStart =
                  let
                    needsConfigFile =
                      cfg.mode != "auto" ||
                      cfg.logLevel != "info" ||
                      cfg.host != "http://localhost" ||
                      cfg.port != 9993 ||
                      cfg.dnsOverTLS != false ||
                      cfg.autoRestart != true ||
                      cfg.addReverseDomains != false ||
                      cfg.multicastDNS != false ||
                      cfg.reconcile != true ||
                      cfg.filterType != "none" ||
                      cfg.filterInclude != [] ||
                      cfg.filterExclude != [] ||
                      cfg.tokenFile != "/var/lib/zerotier-one/authtoken.secret" ||
                      (cfg.profiles != {});

                    configFileArg = if needsConfigFile then "-config-file ${cfg.configFile}" else "";
                    profileArg = if cfg.profile != "" then "-profile ${cfg.profile}" else "";

                    args = lib.strings.concatStringsSep " " (lib.lists.filter (s: s != "") [
                      "${cfg.package}/bin/zt-dns-companion"
                      configFileArg
                      profileArg
                    ]);
                  in args;
                Type = "oneshot";
                User = "root";
                Group = "root";
              };
            };

            systemd.timers.zt-dns-companion = lib.mkIf cfg.service.enable {
              description = "Run ZeroTier DNS Companion periodically";
              wantedBy = [ "timers.target" ];
              timerConfig = {
                OnUnitActiveSec = cfg.service.timerInterval;
                Persistent = true;
              };
            };
          };
        };
    };
}
