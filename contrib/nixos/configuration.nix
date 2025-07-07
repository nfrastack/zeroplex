{
  imports = [
    inputs.zeroplex.nixosModules.default
  ];

  environment.systemPackages = [
    inputs.zeroplex.packages.${pkgs.system}.zeroplex
  ];

  services.zeroplex = {
    enable = true;

    # Default (top-level) config
    mode = "auto";
    log = {
      level = "info";
      type = "console";
      file = "/var/log/zeroplex.log";
      timestamps = false;
    };
    daemon = {
      enabled = true;
      poll_interval = "1m";
    };
    client = {
      host = "http://localhost";
      port = 9993;
      token_file = "/var/lib/zerotier-one/authtoken.secret";
    };
    features = {
      dns_over_tls = false;
      add_reverse_domains = false;
      multicast_dns = false;
      restore_on_exit = false;
      watchdog_ip = null;
      watchdog_interval = "1m";
      watchdog_backoff = [ "10s" "20s" "30s" ];
    };
    interface_watch = {
      mode = "event";
      retry = {
        count = 3;
        delay = "2s";
      };
    };
    networkd = {
      auto_restart = true;
      reconcile = true;
    };

    # Example profiles using advanced filtering
    profiles = {
      development = {
        log.level = "debug";
        log.timestamps = true;
        daemon.enabled = true;
        daemon.poll_interval = "30s";
        interface_watch.mode = "event";
        interface_watch.retry.count = 3;
        interface_watch.retry.delay = "2s";
        features.restore_on_exit = false;
        filters = [
          {
            type = "interface";
            conditions = [
              { value = "zt12345678"; logic = "or"; }
              { value = "zt87654321"; logic = "or"; }
            ];
          }
        ];
      };
      production = {
        mode = "networkd";
        log.level = "info";
        daemon.enabled = true;
        daemon.poll_interval = "5m";
        networkd.auto_restart = true;
        networkd.reconcile = true;
        interface_watch.mode = "poll";
        interface_watch.retry.count = 2;
        interface_watch.retry.delay = "1s";
        features.restore_on_exit = false;
        filters = [
          {
            type = "network";
            conditions = [
              { value = "prod_network"; logic = "or"; }
              { value = "mgmt_network"; logic = "or"; }
            ];
          }
          {
            type = "network";
            operation = "AND";
            negate = true;
            conditions = [
              { value = "test_network"; }
            ];
          }
        ];
      };
      desktop = {
        mode = "resolved";
        log.level = "info";
        features.add_reverse_domains = true;
        features.restore_on_exit = false;
        features.watchdog_ip = "1.1.1.1";
        features.watchdog_interval = "2m";
        features.watchdog_backoff = [ "5s" "15s" "30s" ];
        features.watchdog_hostname = "test.example.com";
        features.watchdog_expected_ip = "123.123.123.123";
        filters = [
          {
            type = "network_id";
            conditions = [
              { value = "a1b2c3d4e5f6g7h8"; logic = "or"; }
              { value = "h8g7f6e5d4c3b2a1"; logic = "or"; }
            ];
          }
        ];
      };
      daemon_simple = {
        daemon.enabled = true;
        daemon.poll_interval = "2m";
        log.level = "info";
        features.restore_on_exit = false;
      };
      advanced_filtering = {
        mode = "networkd";
        log.level = "debug";
        features.restore_on_exit = false;
        filters = [
          {
            type = "name";
            operation = "AND";
            conditions = [
              { value = "prod*"; logic = "or"; }
              { value = "mgmt*"; logic = "or"; }
            ];
          }
          {
            type = "online";
            operation = "AND";
            conditions = [
              { value = "true"; }
            ];
          }
          {
            type = "name";
            operation = "AND";
            negate = true;
            conditions = [
              { value = "*test*"; }
            ];
          }
        ];
      };
      address_filtering = {
        mode = "resolved";
        log.level = "info";
        features.restore_on_exit = false;
        filters = [
          {
            type = "assigned";
            conditions = [
              { value = "true"; }
            ];
          }
          {
            type = "address";
            operation = "AND";
            conditions = [
              { value = "10.*"; logic = "or"; }
              { value = "192.168.*"; }
            ];
          }
        ];
      };
      interface_advanced = {
        mode = "networkd";
        features.restore_on_exit = false;
        filters = [
          {
            type = "interface";
            conditions = [
              { value = "zt*"; }
            ];
          }
          {
            type = "interface";
            operation = "AND";
            negate = true;
            conditions = [
              { value = "*test*"; }
            ];
          }
        ];
      };
      route_filtering = {
        mode = "networkd";
        features.restore_on_exit = false;
        filters = [
          {
            type = "route";
            conditions = [
              { value = "0.0.0.0/0"; logic = "or"; }
              { value = "10.0.0.0/8"; logic = "or"; }
            ];
          }
        ];
      };
      oneshot = {
        daemon.enabled = false;
        log.level = "info";
        features.restore_on_exit = false;
      };
    };
  };
}
