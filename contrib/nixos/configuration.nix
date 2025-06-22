{
  imports = [
    inputs.zeroflex.nixosModules.default
  ];

  # Include the package in system packages if you want to use it manually
  # without the systemd service
  environment.systemPackages = [
    inputs.zeroflex.packages.${pkgs.system}.zeroflex
  ];

  services.zeroflex = {
    # Enable the ZeroFlex module
    enable = true;

    # Basic settings
    mode = "auto";                              # Options: "auto", "networkd", "resolved"
    logLevel = "info";                          # Logging level: "info" or "debug"
    logTimestamps = false;                      # Add timestamps to log entries

    # Select a specific profile to use (must match a profile name defined below)
    profile = "nfrastack";                      # Use this profile when running the service

    # DNS-related settings
    addReverseDomains = false;                  # Add ip6.arpa and in-addr.arpa search domains
    dnsOverTLS = false;                         # Use DNS-over-TLS when available
    multicastDNS = false;                       # Enable mDNS resolution on ZeroTier interfaces

    # System behavior settings
    autoRestart = true;                         # Restart systemd-networkd when changes occur
    reconcile = true;                           # Remove left networks from systemd-networkd config
    restoreOnExit = false;                      # Restore DNS for all managed interfaces on exit

    # Interface watch settings
    interfaceWatch = {
      mode = "event";                           # Options: "event", "poll", "off"
      retry = {
        count = 10;                             # Number of retry attempts on failure
        delay = "10s";                          # Delay between retries
      };
    };

    # Example profiles using advanced filtering
    profiles = {
      # nfrastack profile with network name-based filtering
      nfrastack = {
        dnsOverTLS = true;
        logLevel = "debug";
        restoreOnExit = true;                   # Example: enable DNS restore for this profile
        filters = [
          {
            type = "network";
            conditions = [
              { value = "network1"; logic = "or"; }
              { value = "network2"; logic = "or"; }
            ];
          }
        ];
      };

      # Home profile with interface-based filtering
      home = {
        multicastDNS = true;                        # Enable mDNS for home network
        filters = [
          {
            type = "interface";
            conditions = [
              { value = "ztabcd1234"; }             # Include only this specific interface
            ];
          }
        ];
      };

      # Network ID-based filtering example
      networkIDs = {
        autoRestart = false;                        # Don't auto-restart services
        filters = [
          {
            type = "network_id";
            conditions = [
              { value = "a09acf0233e5c609"; }       # Include by ZeroTier network ID
            ];
          }
        ];
      };

      # Example including all networks (no filters)
      allNetworks = {
        # No filters array means process all networks
      };

      # Example excluding specific networks
      excludeSpecific = {
        filters = [
          {
            type = "network";
            negate = true;                          # Exclude instead of include
            conditions = [
              { value = "network3"; logic = "or"; }
              { value = "network4"; logic = "or"; }
            ];
          }
        ];
      } ;
    };
  };
}
