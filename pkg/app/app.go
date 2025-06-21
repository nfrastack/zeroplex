// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package app

import (
	"zt-dns-companion/pkg/cli"
	"zt-dns-companion/pkg/client"
	"zt-dns-companion/pkg/config"
	"zt-dns-companion/pkg/filters"
	"zt-dns-companion/pkg/logger"
	"zt-dns-companion/pkg/modes"
	appservice "zt-dns-companion/pkg/service"
	"zt-dns-companion/pkg/utils"


	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/zerotier/go-zerotier-one/service"
)

var Version = "dev"

// App represents the main application
type App struct {
	flags         *cli.Flags
	explicitFlags map[string]bool
	config        config.Config
}

// New creates a new App instance
func New() *App {
	flags, explicitFlags := cli.ParseFlags()
	return &App{
		flags:         flags,
		explicitFlags: explicitFlags,
	}
}

// Run executes the main application logic
func (a *App) Run() error {
	// Initialize logging
	logger.SetTimestamps(*a.flags.LogTimestamps)
	logger.SetLogLevel(*a.flags.LogLevel)

	// Load and validate configuration
	a.config = appservice.ValidateAndLoadConfig(*a.flags.ConfigFile)

	// Update logging settings with config values
	logger.SetTimestamps(*a.flags.LogTimestamps || a.config.Default.LogTimestamps)

	if *a.flags.DryRun {
		logger.Debugf("Dry-run mode enabled")
	}

	// Apply explicit flags
	cli.ApplyExplicitFlags(&a.config, a.flags, a.explicitFlags)

	// Handle version and help flags
	if *a.flags.Version {
		fmt.Printf("%s\n", Version)
		os.Exit(0)
	}

	if *a.flags.Help {
		return fmt.Errorf("help requested")
	}

	// Validate runtime requirements
	if err := a.validateRuntime(); err != nil {
		return err
	}

	// Process profiles
	a.processProfiles()

	// Handle mode detection
	modeDetected := a.handleModeDetection()

	// Adjust port if needed
	a.adjustPort()

	// Create ZeroTier client and fetch networks
	networks, err := a.createClientAndFetchNetworks()
	if err != nil {
		return err
	}

	// Apply filters
	filters.ApplyUnifiedFilters(networks, a.config.Default)

	// Run appropriate mode
	return a.runMode(networks, modeDetected)
}

// validateRuntime checks if the app can run in the current environment
func (a *App) validateRuntime() error {
	if os.Geteuid() != 0 {
		utils.ErrorHandler("You need to be root to run this program", nil, true)
	}

	if runtime.GOOS != "linux" {
		utils.ErrorHandler("This tool is only needed on Linux", nil, true)
	}

	return nil
}

// processProfiles handles profile selection and merging
func (a *App) processProfiles() {
	profileNames := []string{}
	for name := range a.config.Profiles {
		profileNames = append(profileNames, name)
	}

	if len(a.config.Profiles) > 0 {
		logger.Debugf("Profiles found in configuration: %v", profileNames)
		if *a.flags.SelectedProfile == "" {
			logger.Debugf("Loading default profile.")
		} else if selectedProfile, ok := a.config.Profiles[*a.flags.SelectedProfile]; ok {
			logger.Debugf("Applying selected profile: %s", *a.flags.SelectedProfile)
			a.config.Default = config.MergeProfiles(a.config.Default, selectedProfile)
		} else {
			logger.Debugf("Selected profile '%s' not found. Using default profile.", *a.flags.SelectedProfile)
		}
	} else {
		logger.Debugf("Using default profile")
	}
}

// handleModeDetection handles automatic mode detection
func (a *App) handleModeDetection() bool {
	var modeDetected bool
	if *a.flags.Mode == "auto" || a.config.Default.Mode == "auto" {
		a.config.Default.Mode, modeDetected = appservice.DetectMode()
	} else if *a.flags.Mode != "" {
		a.config.Default.Mode = *a.flags.Mode
	} else {
		modeDetected = false
	}
	return modeDetected
}

// adjustPort adjusts the port based on configuration
func (a *App) adjustPort() {
	if *a.flags.Port == 9993 {
		if a.config.Default.Port != 0 {
			*a.flags.Port = a.config.Default.Port
		}
	}
}

// createClientAndFetchNetworks creates ZeroTier client and fetches networks
func (a *App) createClientAndFetchNetworks() (*service.GetNetworksResponse, error) {
	ztBaseURL := fmt.Sprintf("%s:%d", a.config.Default.Host, *a.flags.Port)

	apiToken := client.LoadAPIToken(a.config.Default.TokenFile, *a.flags.Token)
	_ = apiToken // Placeholder to ensure the variable is used without exposing it in logs or functionality

	sAPI, err := client.NewServiceAPI(a.config.Default.TokenFile)
	if err != nil {
		utils.ErrorHandler(fmt.Sprintf("Failed to initialize service API client: %v", err), err, true)
	}

	ztClient, err := service.NewClient(ztBaseURL, service.WithHTTPClient(sAPI))
	if err != nil {
		utils.ErrorHandler(fmt.Sprintf("Failed to create ZeroTier client: %v", err), err, true)
	}

	logger.Debugf("Fetching networks from ZeroTier API using base URL: %s", ztBaseURL)
	resp, err := ztClient.GetNetworks(context.Background())
	if err != nil {
		utils.ErrorHandler("Failed to get networks from ZeroTier client", err, true)
	}

	networks, err := service.ParseGetNetworksResponse(resp)
	if err != nil {
		logger.Debugf("Failed to parse networks response: %v", err)
		utils.ErrorHandler("Failed to parse networks response", err, true)
	}

	return networks, nil
}

// runMode executes the appropriate mode
func (a *App) runMode(networks *service.GetNetworksResponse, modeDetected bool) error {
	switch a.config.Default.Mode {
	case "networkd":
		logger.Debugf("Running in networkd mode%s", func() string {
			if modeDetected {
				return " (detected)"
			}
			return ""
		}())
		modes.RunNetworkdMode(networks, *a.flags.AddReverseDomains, *a.flags.AutoRestart, *a.flags.DNSOverTLS, *a.flags.DryRun, *a.flags.MulticastDNS, *a.flags.Reconcile)
	case "resolved":
		output, err := utils.ExecuteCommand("systemctl", "is-active", "systemd-resolved.service")
		if err != nil || strings.TrimSpace(output) != "active" {
			utils.ErrorHandler("systemd-resolved is not running. Resolved mode requires systemd-resolved to be active.", err, true)
		}
		logger.Debugf("Running in resolved mode%s", func() string {
			if modeDetected {
				return " (detected)"
			}
			return ""
		}())
		logger.Debugf("systemd-resolved is running and active")
		modes.RunResolvedMode(networks, *a.flags.AddReverseDomains, *a.flags.DryRun)
	default:
		utils.ErrorHandler("Invalid mode specified in configuration", nil, true)
	}
	return nil
}