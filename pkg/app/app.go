// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package app

import (
	"zeroflex/pkg/config"
	"zeroflex/pkg/log"
	"zeroflex/pkg/runner"
	"zeroflex/pkg/utils"
	"flag"
	"fmt"
	"os"
	"time"
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
	logger.Debug("Loading configuration from file: %s", configFile)
	cfg := config.LoadConfiguration(configFile)

	err := config.ValidateConfig(&cfg)
	if err != nil {
		logger.Debug("Configuration validation failed: %v", err)
		utils.ErrorHandler("Validating configuration", err, true)
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

// Run starts the application
func (a *App) Run() error {
	cfg, dryRun, help, err := a.parseArgs()
	if err != nil {
		if err.Error() == "version requested" {
			if cfg.Default.LogTimestamps {
				timestamp := time.Now().Format("2006-01-02 15:04:05")
				fmt.Printf("%s ZeroFlex version: %s\n", timestamp, getVersionString())
			} else {
				fmt.Printf("ZeroFlex version: %s\n", getVersionString())
			}
			return nil
		}
		return err
	}
	if help {
		printHelpWithVersion(cfg.Default.LogTimestamps)
		return nil
	}
	showStartupBanner(cfg.Default.LogLevel, cfg.Default.LogTimestamps, "")
	if os.Geteuid() != 0 {
		if cfg.Default.LogTimestamps {
			timestamp := time.Now().Format("2006-01-02 15:04:05")
			fmt.Printf("%s ZeroFlex version: %s\n", timestamp, getVersionString())
		} else {
			fmt.Printf("ZeroFlex version: %s\n", getVersionString())
		}
		fmt.Fprintln(os.Stderr, "This application must be run as root. Exiting.")
		os.Exit(1)
	}
	if cfg.Default.LogTimestamps {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		fmt.Printf("%s ZeroFlex version: %s\n", timestamp, getVersionString())
	} else {
		fmt.Printf("ZeroFlex version: %s\n", getVersionString())
	}
	// Perform mode auto-detection before creating the runner
	if cfg.Default.Mode == "auto" {
		r := runner.New(cfg, dryRun)
		detectedMode, detected := r.DetectMode()
		if detected {
			cfg.Default.Mode = detectedMode
			log.NewLogger("[runner]", cfg.Default.LogLevel).Info("Auto-detected mode: %s", detectedMode)
		} else {
			log.NewLogger("[runner]", cfg.Default.LogLevel).Warn("Failed to auto-detect mode, keeping 'auto'")
		}
	}
	a.cfg = cfg
	r := runner.New(cfg, dryRun)
	if cfg.Default.DaemonMode {
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
	if showTimestamps {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		fmt.Printf("%s ZeroFlex version: %s\n", timestamp, getVersionString())
	} else {
		fmt.Printf("ZeroFlex version: %s\n", getVersionString())
	}
	fmt.Println()
	flag.PrintDefaults()
}

func printHelp() {
	// Deprecated: replaced by printHelpWithVersion
}

// parseArgs parses command line arguments and loads configuration
func (a *App) parseArgs() (config.Config, bool, bool, error) {
	logger := log.NewScopedLogger("[app/args]", "info")
	logger.Trace("Starting command line argument parsing")

	// Define flags with aliases
	help := flag.Bool("help", false, "Show help message and exit")
	helpShort := flag.Bool("h", false, "Show help message and exit (alias)")
	version := flag.Bool("version", false, "Show version and exit")
	versionShort := flag.Bool("v", false, "Show version and exit (alias)")
	configFile := flag.String("config-file", "/etc/zeroflex.yaml", "Path to the configuration file")
	configFileShort := flag.String("config", "", "Path to the configuration file (alias)")
	configFileC := flag.String("c", "", "Path to the configuration file (alias)")
	dryRun := flag.Bool("dry-run", false, "Enable dry-run mode. No changes will be made.")
	mode := flag.String("mode", "", "Mode of operation (networkd, resolved, or auto).")
	host := flag.String("host", "", "ZeroFlex client host address.")
	port := flag.Int("port", 0, "ZeroFlex client port number.")
	logLevel := flag.String("log-level", "", "Set the logging level (error, warn, info, verbose, debug, or trace).")
	logTimestamps := flag.Bool("log-timestamps", true, "Enable timestamps in logs. Default: true")
	tokenFile := flag.String("token-file", "", "Path to the ZeroFlex authentication token file.")
	token := flag.String("token", "", "API token to use. Overrides token-file if provided.")

	logger.Verbose("Defined command line flags")

	// Daemon-specific flags
	daemonMode := flag.Bool("daemon", false, "Run in daemon mode with periodic execution (default: true)")
	pollInterval := flag.String("poll-interval", "", "Interval for polling execution (e.g., 1m, 5m, 1h)")
	oneshot := flag.Bool("oneshot", false, "Run once and exit (disable daemon mode)")

	// DNS flags
	addReverseDomains := flag.Bool("add-reverse-domains", false, "Add ip6.arpa and in-addr.arpa search domains.")
	autoRestart := flag.Bool("auto-restart", false, "Automatically restart systemd-networkd when things change.")
	dnsOverTLS := flag.Bool("dns-over-tls", false, "Automatically prefer DNS-over-TLS.")
	multicastDNS := flag.Bool("multicast-dns", false, "Enable Multicast DNS (mDNS).")
	reconcile := flag.Bool("reconcile", false, "Automatically remove left networks from systemd-networkd configuration")

	// Profile selection
	selectedProfile := flag.String("profile", "", "Specify a profile to use from the configuration file.")

	logger.Verbose("Defined daemon and DNS specific flags")

	// Parse flags
	flag.Parse()
	logger.Debug("Command line flags parsed successfully")

	// Help/version logic: allow these even as non-root
	if *help || *helpShort {
		logger.Trace("Help flag requested, returning early")
		return config.Config{}, false, true, nil
	}
	if *version || *versionShort {
		logger.Trace("Version flag requested, returning early")
		return config.Config{}, false, false, fmt.Errorf("version requested")
	}

	// Determine config file path from any alias
	finalConfigFile := *configFile
	if *configFileShort != "" {
		finalConfigFile = *configFileShort
	}
	if *configFileC != "" {
		finalConfigFile = *configFileC
	}

	logger.Verbose("Loading configuration from file: %s", finalConfigFile)
	cfg := ValidateAndLoadConfig(finalConfigFile)
	logger.Debug("Configuration loaded and validated successfully")

	// Handle profile selection
	if *selectedProfile != "" {
		if profile, exists := cfg.Profiles[*selectedProfile]; exists {
			logger.Debug("Applying selected profile: %s", *selectedProfile)
			cfg.Default = mergeProfiles(cfg.Default, profile)
		} else {
			logger.Debug("Selected profile '%s' not found. Using default profile.", *selectedProfile)
		}
	}

	// Apply explicit flags over config/defaults and merged profile (flags always win)
	// --- BEGIN: Ensure flags are applied LAST, after all merging ---
	explicitFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		explicitFlags[f.Name] = true
		logger.Trace("Explicit flag detected: %s = %s", f.Name, f.Value.String())
	})
	// Now apply all explicit flags (move this block after profile merging)
	if explicitFlags["daemon"] {
		logger.Trace("Applying explicit daemon flag: %t", *daemonMode)
		cfg.Default.DaemonMode = *daemonMode
	}
	if explicitFlags["poll-interval"] {
		logger.Trace("Applying explicit poll-interval flag: %s", *pollInterval)
		cfg.Default.PollInterval = *pollInterval
	}
	if explicitFlags["oneshot"] && *oneshot {
		cfg.Default.DaemonMode = false
		logger.Debug("Oneshot mode enabled - daemon mode disabled")
	}
	if explicitFlags["mode"] {
		logger.Trace("Applying explicit mode flag: %s", *mode)
		cfg.Default.Mode = *mode
	}
	if explicitFlags["host"] {
		logger.Trace("Applying explicit host flag: %s", *host)
		cfg.Default.Host = *host
	}
	if explicitFlags["port"] {
		logger.Trace("Applying explicit port flag: %d", *port)
		cfg.Default.Port = *port
	}
	if explicitFlags["log-level"] {
		logger.Trace("Applying explicit log-level flag: %s", *logLevel)
		cfg.Default.LogLevel = *logLevel
	}
	if explicitFlags["log-timestamps"] {
		logger.Trace("Applying explicit log-timestamps flag: %t", *logTimestamps)
		cfg.Default.LogTimestamps = *logTimestamps
	}
	if explicitFlags["token-file"] {
		logger.Trace("Applying explicit token-file flag: %s", *tokenFile)
		cfg.Default.TokenFile = *tokenFile
	}
	_ = token // Suppress unused variable warning
	if explicitFlags["add-reverse-domains"] {
		logger.Trace("Applying explicit add-reverse-domains flag: %t", *addReverseDomains)
		cfg.Default.AddReverseDomains = *addReverseDomains
	}
	if explicitFlags["auto-restart"] {
		logger.Trace("Applying explicit auto-restart flag: %t", *autoRestart)
		cfg.Default.AutoRestart = *autoRestart
	}
	if explicitFlags["dns-over-tls"] {
		logger.Trace("Applying explicit dns-over-tls flag: %t", *dnsOverTLS)
		cfg.Default.DNSOverTLS = *dnsOverTLS
	}
	if explicitFlags["multicast-dns"] {
		logger.Trace("Applying explicit multicast-dns flag: %t", *multicastDNS)
		cfg.Default.MulticastDNS = *multicastDNS
	}
	if explicitFlags["reconcile"] {
		logger.Trace("Applying explicit reconcile flag: %t", *reconcile)
		cfg.Default.Reconcile = *reconcile
	}
	// --- END: Ensure flags are applied LAST ---

	// Validate daemon configuration
	if cfg.Default.DaemonMode {
		logger.Verbose("Validating daemon mode configuration")
		if cfg.Default.PollInterval == "" {
			cfg.Default.PollInterval = "1m" // Default interval
			logger.Debug("Set default poll interval to 1m")
		}

		// Validate interval
		if _, err := utils.ParseInterval(cfg.Default.PollInterval); err != nil {
			logger.Error("Invalid poll interval '%s': %v", cfg.Default.PollInterval, err)
			return config.Config{}, false, false, fmt.Errorf("invalid poll interval '%s': %w", cfg.Default.PollInterval, err)
		}
		logger.Verbose("Running in daemon mode with API polling interval: %s", cfg.Default.PollInterval)
	} else {
		logger.Verbose("One-shot mode configured")
	}

	// After all config/profile merging and explicit flag application, update logger global state
	log.GetLogger().SetShowTimestamps(cfg.Default.LogTimestamps)

	logger.Debug("Configuration parsing completed successfully")
	logger.Trace("Final configuration - Mode: %s, LogLevel: %s, DaemonMode: %t, PollInterval: %s",
		cfg.Default.Mode, cfg.Default.LogLevel, cfg.Default.DaemonMode, cfg.Default.PollInterval)

	return cfg, *dryRun, false, nil
}

// mergeProfiles merges a selected profile with the default profile
func mergeProfiles(defaultProfile, selectedProfile config.Profile) config.Profile {
	merged := defaultProfile

	if selectedProfile.Mode != "" {
		merged.Mode = selectedProfile.Mode
	}
	if selectedProfile.LogLevel != "" {
		merged.LogLevel = selectedProfile.LogLevel
	}
	if selectedProfile.Host != "" {
		merged.Host = selectedProfile.Host
	}
	if selectedProfile.Port != 0 {
		merged.Port = selectedProfile.Port
	}
	if selectedProfile.TokenFile != "" {
		merged.TokenFile = selectedProfile.TokenFile
	}
	if selectedProfile.PollInterval != "" {
		merged.PollInterval = selectedProfile.PollInterval
	}

	if len(selectedProfile.Filters) > 0 {
		merged.Filters = selectedProfile.Filters
	}

	if selectedProfile.DNSOverTLS {
		merged.DNSOverTLS = true
	}
	if selectedProfile.AddReverseDomains {
		merged.AddReverseDomains = true
	}
	if selectedProfile.LogTimestamps {
		merged.LogTimestamps = true
	}
	if selectedProfile.MulticastDNS {
		merged.MulticastDNS = true
	}
	if selectedProfile.DaemonMode {
		merged.DaemonMode = true
	}
	if !selectedProfile.AutoRestart {
		merged.AutoRestart = false
	}
	if !selectedProfile.Reconcile {
		merged.Reconcile = false
	}

	return merged
}