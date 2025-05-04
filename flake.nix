{
  description = "Manage ZeroTier DNS with systemd-resolved or networkd";

  inputs = { nixpkgs.url = "nixpkgs/nixos-unstable"; };

  outputs = { self, nixpkgs }:
    let
      lastModifiedDate = self.lastModifiedDate or self.lastModified or "19700101";
      version = "foo";
      supportedSystems = [ "x86_64-linux" "aarch64-linux" ];

      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
      nixpkgsFor = forAllSystems (system: import nixpkgs { inherit system; });
    in {
      packages = forAllSystems (system:
        let pkgs = nixpkgsFor.${system};
        in {
          zerotier-dns-companion = pkgs.buildGoModule {
            pname = "zerotier-dns-companion";
            inherit version;
            src = ./.;
            vendorHash = "sha256-pY9VpCiNOkLu6w7jaiOR8O0NMZYC1RxzmHt/NhlfVZk=";
            ldflags = [ "-w" "-s" "-X main.Version=v${version}" ];
          };
        });

      devShells = forAllSystems (system:
        let pkgs = nixpkgsFor.${system};
        in pkgs.mkShell {
          buildInputs = [
            pkgs.make
            pkgs.go
          ];
        });

      defaultPackage = forAllSystems (system: self.packages.${system}.zerotier-dns-companion);

      nixosModules.default = { config, lib, pkgs, ... }:
        let cfg = config.services.zerotier-dns-companion;
        in {
          options.services.zerotier-dns-companion = {
            enable = lib.mkEnableOption {
              type = lib.types.bool;
              default = false;
              description = "Enable the ZeroTier DNS Companion service to manage DNS for ZeroTier networks.";
            };

            package = lib.mkOption {
              type = lib.types.package;
              default = self.packages.${pkgs.system}.zerotier-dns-companion;
              description = "ZeroTier DNS Companion package to use.";
            };

            configFile = lib.mkOption {
              type = lib.types.str;
              default = "/etc/zt-dns-companion.conf";
              description = "Path to the configuration file for ZT DNS Companion.";
            };

            mode = lib.mkOption {
              type = lib.types.enum [ "networkd" "resolved" ];
              default = "networkd";
              description = "Mode of operation (networkd or resolved).";
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

            addReverseDomains = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Add ip6.arpa and in-addr.arpa search domains.";
            };

            autoRestart = lib.mkOption {
              type = lib.types.bool;
              default = true;
              description =
                "Automatically restart systemd-networkd when things change.";
            };

            dnsOverTLS = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description =
                "Automatically prefer DNS-over-TLS. Requires ZeroNSd v0.4 or better.";
            };

            multicastDNS = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Enable mDNS resolution on the zerotier interface.";
            };

            reconcile = lib.mkOption {
              type = lib.types.bool;
              default = true;
              description =
                "Automatically remove left networks from systemd-networkd configuration.";
            };

            timerInterval = lib.mkOption {
              type = lib.types.str;
              default = "1m";
              description =
                "Interval for the systemd timer (e.g., 1m, 5m, 1h).";
            };
          };

          config = lib.mkIf cfg.enable {
            assertions = [
              {
                assertion = cfg.mode != "resolved" || config.services.resolved.enable;
                message = ''
                  The "resolved" mode requires systemd-resolved to be enabled.
                  Please enable it with:

                  services.resolved.enable = true;
                '';
              };
              {
                assertion = cfg.mode != "networkd" || config.systemd.network.enable;
                message = ''
                  The "networkd" mode requires systemd-networkd to be enabled.
                  Please enable it with:

                  systemd.network.enable = true;

                  If you use NetworkManager, you might also need:

                  systemd.network.wait-online.enable = false;

                  See more info on how to enable systemd-networkd in https://nixos.wiki/wiki/Systemd-networkd
                '';
              };
            ];

            systemd.services.zerotier-dns-companion = {
              description = "ZeroTier DNS Companion";
              wantedBy = [ "multi-user.target" ];
              serviceConfig = {
                ExecStart = ''
                  ${cfg.package}/bin/zerotier-dns-companion \
                    -config ${cfg.configFile}
                '';
                Type = "oneshot";
                User = "root";
                Group = "root";
              };
            };

            systemd.timers.zerotier-dns-companion = {
              description = "Run ZeroTier DNS Companion periodically";
              wantedBy = [ "timers.target" ];
              timerConfig = {
                OnUnitActiveSec = cfg.timerInterval;
                Persistent = true;
              };
            };

            environment.etc."${cfg.configFile}".text = builtins.toJSON {
              add_reverse_domains = cfg.addReverseDomains;
              auto_restart = cfg.autoRestart;
              dns_over_tls = cfg.dnsOverTLS;
              host = cfg.host;
              log_level = cfg.logLevel;
              mode = cfg.mode;
              multicast_dns = cfg.multicastDNS;
              port = cfg.port;
              reconcile = cfg.reconcile;
              token_file = cfg.tokenFile;
            };
          };
        };
    };
}
