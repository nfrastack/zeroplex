{
  imports = [
    inputs.zt-dns-companion.nixosModules.default
  ];

  services.zt-dns-companion = {
    enable = true;
    mode = "networkd";          # Options: "networkd", "resolved"
    logLevel = "info";          # Logging level: "info" or "debug"
  };
}
