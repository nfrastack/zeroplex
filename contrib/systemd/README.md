### Systemd Examples

This directory contains example systemd service files and configurations for deploying the ZT DNS Companion application.

#### Example: zt-dns-companion.service

The `zt-dns-companion.service` file is a systemd service unit that can be used to manage the ZT DNS Companion application as a background service. To use it:

1. Copy the service file to the systemd directory:
   ```bash
   sudo cp zt-dns-companion.service /etc/systemd/system/
   ```

2. Reload the systemd daemon to recognize the new service:
   ```bash
   sudo systemctl daemon-reload
   ```

3. Enable the service to start on boot:
   ```bash
   sudo systemctl enable zt-dns-companion
   ```

4. Start the service:
   ```bash
   sudo systemctl start zt-dns-companion
   ```

5. Check the service status:
   ```bash
   sudo systemctl status zt-dns-companion
   ```

#### Adding Command-Line Arguments

You can customize the behavior of the ZT DNS Companion application by adding command-line arguments to the `ExecStart` line in the service file. For example:

```ini
[Service]
ExecStart=/usr/local/bin/zt-dns-companion --log-level debug --dry-run
```

In this example:
- `--log-level debug` sets the logging level to debug.
- `--dry-run` enables dry-run mode, where no changes are applied.

After modifying the service file, reload the systemd daemon and restart the service:
```bash
sudo systemctl daemon-reload
sudo systemctl restart zt-dns-companion
```

