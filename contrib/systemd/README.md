# Systemd Examples

This directory contains example systemd service files and configurations for deploying the ZeroPlex application.

## Example: zeroplex.service

The `zeroplex.service` file is a systemd service unit that can be used to manage the ZeroPlex application as a background service. To use it:

1. Copy the service file to the systemd directory:

   ```bash
   sudo cp zeroplex.service /etc/systemd/system/
   ```

2. Reload the systemd daemon to recognize the new service:

   ```bash
   sudo systemctl daemon-reload
   ```

3. Enable the service to start on boot:

   ```bash
   sudo systemctl enable zeroplex
   ```

4. Start the service:

   ```bash
   sudo systemctl start zeroplex
   ```

5. Check the service status:

   ```bash
   sudo systemctl status zeroplex
   ```

### Adding Command-Line Arguments

You can customize the behavior of the ZeroPlex application by adding command-line arguments to the `ExecStart` line in the service file. For example:

```ini
[Service]
ExecStart=/usr/local/bin/zeroplex --log-level debug --dry-run
```

In this example:

- `--log-level debug` sets the logging level to debug (nested config key).
- `--dry-run` enables dry-run mode, where no changes are applied.

After modifying the service file, reload the systemd daemon and restart the service:

```bash
sudo systemctl daemon-reload
sudo systemctl restart zeroplex
```

### Using Profiles for Configuration

`zeroplex.service` supports the use of profiles to simplify configuration management. Profiles allow you to define specific settings in configuration files, avoiding the need to pass multiple arguments directly to the service.

#### How to Use Profiles

1. Update the configuration to support profiles with the desired settings (using the nested YAML structure).
2. Modify the `ExecStart` line in the service file to include the `-profile` flag:

   ```ini
   [Service]
   ExecStart=/usr/bin/zeroplex -profile my-profile
   ```

3. Reload the systemd daemon and restart the service:

   ```bash
   sudo systemctl daemon-reload
   sudo systemctl restart zeroplex
   ```