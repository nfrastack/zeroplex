// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package app

import (
	"zeroplex/pkg/cli"
	"zeroplex/pkg/config"
	"zeroplex/pkg/log"
	"zeroplex/pkg/runner"
	"zeroplex/pkg/utils"

	"flag"
	"fmt"
	"io"
	"os"
)

var Version = "development"

type App struct {
	cfg    config.Config
	runner *runner.Runner
}

func New() *App {
	return &App{}
}

// ValidateAndLoadConfig validates and loads configuration from file
func ValidateAndLoadConfig(configFile string) config.Config {
	logger := log.NewScopedLogger("[config]", "info")
	logger.Trace("ValidateAndLoadConfig() started with file: %s", configFile)

	// Enhanced config file search logic
	tryFiles := []string{}
	if configFile != "" {
		tryFiles = append(tryFiles, configFile)
	} else {
		tryFiles = append(tryFiles, "./zeroplex.yml", "/etc/zeroplex.yml")
	}

	var cfg config.Config
	var found bool
	for _, f := range tryFiles {
		if fi, err := os.Stat(f); err == nil && !fi.IsDir() {
			logger.Debug("Loading configuration from file: %s", f)
			cfg = config.LoadConfiguration(f)
			found = true
			break
		}
	}

	if !found {
		logger.Warn("No configuration file found (tried: %v). Proceeding with defaults and CLI flags only.", tryFiles)
		cfg = config.Config{} // Use empty config; CLI flags will apply
	} else {
		err := config.ValidateConfig(&cfg)
		if err != nil {
			logger.Debug("Configuration validation failed: %v", err)
			utils.ErrorHandler("Validating configuration", err, true)
		}
	}
	return cfg
}

func showStartupBanner(logLevel string, showTimestamps bool, version string) {
	fmt.Println()
	fmt.Println("             .o88o.                                 .                       oooo")
	fmt.Println("             888 \"\"                                .o8                       888")
	fmt.Println("ooo. .oo.   o888oo  oooo d8b  .oooo.    .oooo.o .o888oo  .oooo.    .ooooo.   888  oooo")
	fmt.Println("`888P\"Y88b   888    `888\"\"8P `P  )88b  d88(  \"8   888   `P  )88b  d88' \"Y8  888 .8P'")
	fmt.Println(" 888   888   888     888      .oP\"888  \"\"Y88b.    888    .oP\"888  888        888888.")
	fmt.Println(" 888   888   888     888     d8(  888  o.  )88b   888 . d8(  888  888   .o8  888 `88b.")
	fmt.Println("o888o o888o o888o   d888b    `Y888\"\"8o 8\"\"888P'   \"888\" `Y888\"\"8o `Y8bod8P' o888o o888o")
	fmt.Println()
}

func printCopyrightAndLicense() {
	fmt.Println("© 2025 Nfrastack https://nfrastack.com - BSD-3-Clause License")
}

func printStartupVersion(version string) {
	fmt.Printf("Starting ZeroPlex version: %s\n", version)
	printCopyrightAndLicense()
}

func printVersion(version string) {
	fmt.Printf("ZeroPlex version: %s | © 2025 Nfrastack https://nfrastack.com - BSD-3-Clause License\n", version)
}

// Run starts the application
func (a *App) Run() error {
	// Use already-parsed flags from cli.FlagsInstance
	flags := cli.FlagsInstance

	// Check for help/version flags before anything else
	if *flags.Help || *flags.HelpShort {
		printHelpWithVersion(false)
		return nil
	}
	if *flags.Version || *flags.VersionShort {
		printVersion(getVersionString())
		return nil
	}

	// Require root for all other operations
	if os.Geteuid() != 0 {
		printVersion(getVersionString())
		fmt.Fprintln(os.Stderr, "This application must be run as root. Exiting.")
		os.Exit(1)
	}

	// Now proceed to config and normal operation
	cfg, dryRun, showBanner, err := a.parseArgsWithBanner()
	if err != nil {
		return err
	}
	if showBanner {
		showStartupBanner(cfg.Default.Log.Level, cfg.Default.Log.Timestamps, "")
	}
	printStartupVersion(getVersionString())
	// Perform mode auto-detection before creating the runner
	if cfg.Default.Mode == "auto" {
		r := runner.New(cfg, dryRun)
		detectedMode, detected := r.DetectMode()
		if detected {
			cfg.Default.Mode = detectedMode
			log.NewLogger("[runner]", cfg.Default.Log.Level).Info("Auto-detected mode: %s", detectedMode)
		} else {
			log.NewLogger("[runner]", cfg.Default.Log.Level).Warn("Failed to auto-detect mode, keeping 'auto'")
		}
	}
	a.cfg = cfg
	r := runner.New(cfg, dryRun)
	if cfg.Default.Daemon.Enabled {
		r.RunDaemon()
	} else {
		r.RunOnce()
	}
	return nil
}

func getVersionString() string {
	return Version
}

func printHelpWithVersion(showTimestamps bool) {
	printVersion(getVersionString())
	fmt.Println()
	flag.Usage() // Use the custom grouped help output
}

func printHelp() {
	// Deprecated: replaced by printHelpWithVersion
}

// parseArgsWithBanner parses command line arguments and loads configuration, returning showBanner
func (a *App) parseArgsWithBanner() (config.Config, bool, bool, error) {
	logger := log.NewScopedLogger("[app/args]", "info")
	logger.Trace("Starting command line argument parsing")

	flags := cli.FlagsInstance
	explicitFlags := cli.ExplicitFlags

	// Help/version logic: allow these even as non-root
	if *flags.Help || *flags.HelpShort {
		logger.Trace("Help flag requested, returning early")
		return config.Config{}, false, false, nil
	}
	if *flags.Version || *flags.VersionShort {
		logger.Trace("Version flag requested, returning early")
		return config.Config{}, false, false, fmt.Errorf("version requested")
	}

	// Determine config file path from any alias
	finalConfigFile := ""
	if *flags.ConfigFile != "" {
		finalConfigFile = *flags.ConfigFile
	}
	if *flags.ConfigFileShort != "" {
		finalConfigFile = *flags.ConfigFileShort
	}
	if *flags.ConfigFileC != "" {
		finalConfigFile = *flags.ConfigFileC
	}

	logger.Verbose("Loading configuration from file: %s", finalConfigFile)
	cfg := ValidateAndLoadConfig(finalConfigFile)
	logger.Debug("Configuration loaded and validated successfully")

	// Handle profile selection
	if *flags.SelectedProfile != "" {
		if profile, exists := cfg.Profiles[*flags.SelectedProfile]; exists {
			logger.Debug("Applying selected profile: %s", *flags.SelectedProfile)
			cfg.Default = mergeProfiles(cfg.Default, profile)
		} else {
			logger.Debug("Selected profile '%s' not found. Using default profile.", *flags.SelectedProfile)
		}
	}

	// Apply explicit flags over config/defaults and merged profile (flags always win)
	cli.ApplyExplicitFlags(&cfg, flags, explicitFlags)

	// Validate daemon configuration
	if cfg.Default.Daemon.Enabled {
		logger.Verbose("Validating daemon mode configuration")
		if cfg.Default.Daemon.PollInterval == "" {
			cfg.Default.Daemon.PollInterval = "1m" // Default interval
			logger.Debug("Set default poll interval to 1m")
		}

		// Validate interval
		if _, err := utils.ParseInterval(cfg.Default.Daemon.PollInterval); err != nil {
			logger.Error("Invalid poll interval '%s': %v", cfg.Default.Daemon.PollInterval, err)
			return config.Config{}, false, false, fmt.Errorf("invalid poll interval '%s': %w", cfg.Default.Daemon.PollInterval, err)
		}
		logger.Verbose("Running in daemon mode with API polling interval: %s", cfg.Default.Daemon.PollInterval)
	} else {
		logger.Verbose("One-shot mode configured")
	}

	// After all config/profile merging and explicit flag application, update logger global state
	log.GetLogger().SetShowTimestamps(cfg.Default.Log.Timestamps)

	// Set up logging output type and file if specified
	if cfg.Default.Log.Type == "file" || cfg.Default.Log.Type == "both" {
		logFile := cfg.Default.Log.File
		if logFile == "" {
			logFile = "/var/log/zeroplex.log"
		}
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			if cfg.Default.Log.Type == "file" {
				log.GetLogger().SetOutput(f)
			} else if cfg.Default.Log.Type == "both" {
				// Log to both file and console: use MultiWriter
				mw := io.MultiWriter(os.Stdout, f)
				log.GetLogger().SetOutput(mw)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Failed to open log file %s: %v\n", logFile, err)
		}
	} else {
		log.GetLogger().SetOutput(os.Stdout)
	}

	logger.Debug("Configuration parsing completed successfully")
	logger.Trace("Final configuration - Mode: %s, LogLevel: %s, DaemonMode: %t, PollInterval: %s",
		cfg.Default.Mode, cfg.Default.Log.Level, cfg.Default.Daemon.Enabled, cfg.Default.Daemon.PollInterval)

	return cfg, *flags.DryRun, *flags.Banner, nil
}

// mergeProfiles merges a selected profile with the default profile
func mergeProfiles(defaultProfile, selectedProfile config.Profile) config.Profile {
	merged := defaultProfile

	if selectedProfile.Mode != "" {
		merged.Mode = selectedProfile.Mode
	}
	// Merge Log
	if selectedProfile.Log.Level != "" {
		merged.Log.Level = selectedProfile.Log.Level
	}
	if selectedProfile.Log.Type != "" {
		merged.Log.Type = selectedProfile.Log.Type
	}
	if selectedProfile.Log.File != "" {
		merged.Log.File = selectedProfile.Log.File
	}
	merged.Log.Timestamps = selectedProfile.Log.Timestamps || merged.Log.Timestamps

	// Merge Daemon
	merged.Daemon.Enabled = selectedProfile.Daemon.Enabled || merged.Daemon.Enabled
	if selectedProfile.Daemon.PollInterval != "" {
		merged.Daemon.PollInterval = selectedProfile.Daemon.PollInterval
	}

	// Merge Client
	if selectedProfile.Client.Host != "" {
		merged.Client.Host = selectedProfile.Client.Host
	}
	if selectedProfile.Client.Port != 0 {
		merged.Client.Port = selectedProfile.Client.Port
	}
	if selectedProfile.Client.TokenFile != "" {
		merged.Client.TokenFile = selectedProfile.Client.TokenFile
	}

	// Merge Networkd
	merged.Networkd.AutoRestart = selectedProfile.Networkd.AutoRestart || merged.Networkd.AutoRestart
	merged.Networkd.Reconcile = selectedProfile.Networkd.Reconcile || merged.Networkd.Reconcile

	// Merge Features
	merged.Features.DNSOverTLS = selectedProfile.Features.DNSOverTLS || merged.Features.DNSOverTLS
	merged.Features.AddReverseDomains = selectedProfile.Features.AddReverseDomains || merged.Features.AddReverseDomains
	merged.Features.MulticastDNS = selectedProfile.Features.MulticastDNS || merged.Features.MulticastDNS
	merged.Features.RestoreOnExit = selectedProfile.Features.RestoreOnExit || merged.Features.RestoreOnExit

	// Merge InterfaceWatch
	if selectedProfile.InterfaceWatch.Mode != "" {
		merged.InterfaceWatch.Mode = selectedProfile.InterfaceWatch.Mode
	}
	if selectedProfile.InterfaceWatch.Retry.Count != 0 {
		merged.InterfaceWatch.Retry.Count = selectedProfile.InterfaceWatch.Retry.Count
	}
	if selectedProfile.InterfaceWatch.Retry.Delay != "" {
		merged.InterfaceWatch.Retry.Delay = selectedProfile.InterfaceWatch.Retry.Delay
	}

	// Merge Filters
	if len(selectedProfile.Filters) > 0 {
		merged.Filters = selectedProfile.Filters
	}

	return merged
}

func init() {
	flags := cli.FlagsInstance
	flag.Usage = func() {
		if flags != nil && *flags.Banner {
			showStartupBanner("info", false, Version)
		}
		printCopyrightAndLicense()
		// Only print version once
		fmt.Fprintf(flag.CommandLine.Output(), "ZeroPlex version: %s\n\n", getVersionString())
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: zeroplex [options]\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "General Options:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  %-29s %s\n", "--help", "Show help message and exit")
		fmt.Fprintf(flag.CommandLine.Output(), "  %-29s %s\n", "--version", "Print the version and exit")
		fmt.Fprintf(flag.CommandLine.Output(), "  %-29s %s\n", "--config-file", "Path to the configuration file")
		fmt.Fprintf(flag.CommandLine.Output(), "  %-29s %s\n", "--profile", "Specify a profile to use from the configuration file")
		fmt.Fprintf(flag.CommandLine.Output(), "  %-29s %s\n", "--dry-run", "Enable dry-run mode. No changes will be made.")
		fmt.Fprintf(flag.CommandLine.Output(), "\nLogging Options:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  %-29s %s\n", "--log-level", "Set the logging level ('info', 'verbose'*, 'error', 'debug', 'trace')")
		fmt.Fprintf(flag.CommandLine.Output(), "  %-29s %s\n", "--log-type", "Log output type: 'console'*, 'file', or 'both'")
		fmt.Fprintf(flag.CommandLine.Output(), "  %-29s %s\n", "--log-file", "Log file path if log-type is 'file' or 'both'")
		fmt.Fprintf(flag.CommandLine.Output(), "  %-29s %s\n", "--log-timestamps", "Enable timestamps in logs")
		fmt.Fprintf(flag.CommandLine.Output(), "\nFeatures:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  %-29s %s\n", "--dns-over-tls", "Automatically prefer DNS-over-TLS")
		fmt.Fprintf(flag.CommandLine.Output(), "  %-29s %s\n", "--multicast-dns", "Enable Multicast DNS (mDNS)")
		fmt.Fprintf(flag.CommandLine.Output(), "  %-29s %s\n", "--add-reverse-domains", "Add ip6.arpa and in-addr.arpa search domains")
		fmt.Fprintf(flag.CommandLine.Output(), "  %-29s %s\n", "--restore-on-exit", "Restore original DNS settings for all managed interfaces on exit")
		fmt.Fprintf(flag.CommandLine.Output(), "\nNetworkd Options:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  %-29s %s\n", "--auto-restart", "Automatically restart systemd-networkd when things change")
		fmt.Fprintf(flag.CommandLine.Output(), "  %-29s %s\n", "--reconcile", "Automatically remove left networks from systemd-networkd configuration")
		fmt.Fprintf(flag.CommandLine.Output(), "\nInterface Watch Options:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  %-29s %s\n", "--interface-watch-mode", "Interface watch mode: event, poll, or off")
		fmt.Fprintf(flag.CommandLine.Output(), "  %-29s %s\n", "--interface-watch-retry-count", "Number of retries after interface event")
		fmt.Fprintf(flag.CommandLine.Output(), "  %-29s %s\n", "--interface-watch-retry-delay", "Delay between interface event retries (e.g., '2s')")
		fmt.Fprintf(flag.CommandLine.Output(), "\nZeroTier Client Options:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  %-29s %s\n", "--host", "ZeroTier client host address")
		fmt.Fprintf(flag.CommandLine.Output(), "  %-29s %s\n", "--port", "ZeroTier client port number")
		fmt.Fprintf(flag.CommandLine.Output(), "  %-29s %s\n", "--token", "API token to use. Overrides token-file if provided")
		fmt.Fprintf(flag.CommandLine.Output(), "  %-29s %s\n", "--token-file", "Path to the ZeroTier authentication token file")
		fmt.Fprintf(flag.CommandLine.Output(), "\n") // Add trailing newline for clean output
	}
}
