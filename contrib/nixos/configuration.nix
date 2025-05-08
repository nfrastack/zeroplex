{
  imports = [
    inputs.zt-dns-companion.nixosModules.default
  ];

  # Include the package in system packages if you want to use it manually
  # without the systemd service
  environment.systemPackages = [
    inputs.zt-dns-companion.packages.${pkgs.system}.zt-dns-companion
  ];

  services.zt-dns-companion = {
    # Enable the ZT DNS Companion module
    enable = true;

    # Basic settings
    mode = "auto";                              # Options: "auto", "networkd", "resolved"
    logLevel = "info";                          # Logging level: "info" or "debug"
    logTimestamps = false;                      # Add timestamps to log entries

    # Filtering - only one filter type can be active at a time
    filterType = "network";                     # Options: "interface", "network", "network_id", "none"
    filterInclude = ["network1", "network2"];   # Items to include (empty means "all")
    filterExclude = [];                         # Items to exclude (empty means "none")

    # Select a specific profile to use (must match a profile name defined below)
    profile = "nfrastack";                      # Use this profile when running the service

    # DNS-related settings
    addReverseDomains = false;                  # Add ip6.arpa and in-addr.arpa search domains
    dnsOverTLS = false;                         # Use DNS-over-TLS when available
    multicastDNS = false;                       # Enable mDNS resolution on ZeroTier interfaces

    # System behavior settings
    autoRestart = true;                         # Restart systemd-networkd when changes occur
    reconcile = true;                           # Remove left networks from systemd-networkd config
    timerInterval = "5m";                       # How often to run the service

    # Example profiles for different use cases
    profiles =nfrastack   # nfrastack profile with network name-based filtering
      nfrastack = {
        filterType = "network";
        filterInclude = ["network1", "network2"];
        dnsOverTLS = true;
        logLevel = "debug";
      };

      # Home profile with interface-based filtering
      home = {
        filterType = "interface";
        filterInclude = ["ztabcd1234"];             # Include only this specific interface
        multicastDNS = true;                        # Enable mDNS for home network
      };

      # Network ID-based filtering example
      networkIDs = {
        filterType = "network_id";
        filterInclude = ["a09acf0233e5c609"];       # Include by ZeroTier network ID
        autoRestart = false;                        # Don't auto-restart services
      };

      # Example using special filter values
      allNetworks = {
        filterType = "network";
        filterInclude = ["any"];                    # Special value to include all networks
        filterExclude = ["none"];                   # Special value to exclude nothing
      };

      # Example excluding specific networks
      excludeSpecific = {
        filterType = "network";
        filterInclude = [];                         # Empty means include all
        filterExclude = ["network3", "network4"];   # Exclude these networks
      } ;
    };
  };
}
