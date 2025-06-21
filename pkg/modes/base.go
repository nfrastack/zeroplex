// filepath: /home/dave/src/gh/zt-dns-companion/pkg/modes/base.go
// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package modes

import (
	"zt-dns-companion/pkg/client"
	"zt-dns-companion/pkg/config"
	"zt-dns-companion/pkg/filters"
	"zt-dns-companion/pkg/logging"
	"zt-dns-companion/pkg/utils"

	"context"
	"fmt"

	"github.com/zerotier/go-zerotier-one/service"
)

// BaseMode provides common functionality for all mode implementations
type BaseMode struct {
	cfg    config.Config
	dryRun bool
	mode   string
}

// NewBaseMode creates a new base mode instance
func NewBaseMode(cfg config.Config, dryRun bool, mode string) *BaseMode {
	return &BaseMode{
		cfg:    cfg,
		dryRun: dryRun,
		mode:   mode,
	}
}

// FetchNetworks retrieves networks from ZeroTier API
func (b *BaseMode) FetchNetworks(ctx context.Context) (*service.GetNetworksResponse, error) {
	logging.APILogger.Trace("Creating ZeroTier API client")

	// Create API client
	sAPI, err := client.NewServiceAPI(b.cfg.Default.TokenFile)
	if err != nil {
		logging.APILogger.Error("Failed to create service API client: %v", err)
		return nil, fmt.Errorf("failed to create service API client: %w", err)
	}
	logging.APILogger.Debug("API client created successfully")

	// Create ZeroTier client
	ztBaseURL := fmt.Sprintf("%s:%d", b.cfg.Default.Host, b.cfg.Default.Port)
	logging.APILogger.Trace("Creating ZeroTier client with URL: %s", ztBaseURL)
	ztClient, err := service.NewClient(ztBaseURL, service.WithHTTPClient(sAPI))
	if err != nil {
		logging.APILogger.Error("Failed to create ZeroTier client: %v", err)
		return nil, fmt.Errorf("failed to create ZeroTier client: %w", err)
	}
	logging.APILogger.Debug("ZeroTier client created successfully")

	// Fetch networks
	logging.APILogger.Trace("Making API request to fetch networks")
	resp, err := ztClient.GetNetworks(ctx)
	if err != nil {
		logging.APILogger.Error("Failed to get networks: %v", err)
		return nil, fmt.Errorf("failed to get networks: %w", err)
	}
	logging.APILogger.Debug("API request completed successfully")

	logging.APILogger.Trace("Parsing API response")
	networks, err := service.ParseGetNetworksResponse(resp)
	if err != nil {
		logging.APILogger.Error("Failed to parse networks response: %v", err)
		return nil, fmt.Errorf("failed to parse networks response: %w", err)
	}
	logging.APILogger.Debug("Response parsed successfully")

	return networks, nil
}

// ApplyFilters applies configured filters to networks
func (b *BaseMode) ApplyFilters(networks *service.GetNetworksResponse) {
	filters.ApplyFilters(networks, b.cfg.Default)
}

// LogNetworkDiscovery logs the network discovery process
func (b *BaseMode) LogNetworkDiscovery(networks *service.GetNetworksResponse, preFilter bool) {
	modeLogger := logging.GetModeLogger(b.mode)
	
	if preFilter {
		modeLogger.Debug("Retrieved %d networks from ZeroTier", len(*networks.JSON200))
		modeLogger.Verbose("Network discovery completed successfully")

		// Log each network found (before filtering)
		for i, network := range *networks.JSON200 {
			modeLogger.Trace("Network %d: ID=%s, Name=%s, Interface=%s, Status=%s",
				i+1,
				utils.GetString(network.Id),
				utils.GetString(network.Name),
				utils.GetString(network.PortDeviceName),
				utils.GetString(network.Status))
		}
	} else {
		modeLogger.Debug("After filtering: %d networks to process", len(*networks.JSON200))
		modeLogger.Verbose("Network filtering completed, proceeding with configuration...")

		// Log each network that will be processed (after filtering)
		for i, network := range *networks.JSON200 {
			modeLogger.Debug("Processing network %d: ID=%s, Name=%s, Interface=%s",
				i+1,
				utils.GetString(network.Id),
				utils.GetString(network.Name),
				utils.GetString(network.PortDeviceName))

			if network.Dns != nil && network.Dns.Servers != nil {
				modeLogger.Verbose("  DNS servers: %v", *network.Dns.Servers)
			}
			if network.Dns != nil && network.Dns.Domain != nil {
				modeLogger.Verbose("  DNS domain: %s", *network.Dns.Domain)
			}
			if network.AssignedAddresses != nil {
				modeLogger.Verbose("  Assigned addresses: %v", *network.AssignedAddresses)
			}
		}
	}
}

// LogConfiguration logs the configuration details
func (b *BaseMode) LogConfiguration() {
	logging.ConfigLogger.Debug("Host: %s, Port: %d, TokenFile: %s",
		b.cfg.Default.Host, b.cfg.Default.Port, b.cfg.Default.TokenFile)
}

// GetConfig returns the configuration
func (b *BaseMode) GetConfig() config.Config {
	return b.cfg
}

// IsDryRun returns whether this is a dry run
func (b *BaseMode) IsDryRun() bool {
	return b.dryRun
}

// GetModeName returns the mode name
func (b *BaseMode) GetModeName() string {
	return b.mode
}

// GetNetworkName returns a display name for the network
func GetNetworkName(network service.Network) string {
	if network.Name != nil && *network.Name != "" {
		return *network.Name
	}
	if network.Id != nil {
		return *network.Id
	}
	return "unknown"
}

// ValidateNetwork performs common network validation
func (b *BaseMode) ValidateNetwork(network service.Network) error {
	if network.Id == nil {
		return fmt.Errorf("network ID is required")
	}

	if network.PortDeviceName == nil || *network.PortDeviceName == "" {
		return fmt.Errorf("network interface name is required for network %s", utils.GetString(network.Id))
	}

	return nil
}

// GetDNSServers extracts DNS servers from a network
func (b *BaseMode) GetDNSServers(network service.Network) []string {
	if network.Dns == nil || network.Dns.Servers == nil {
		return nil
	}
	return *network.Dns.Servers
}

// GetDNSDomain extracts DNS domain from a network
func (b *BaseMode) GetDNSDomain(network service.Network) string {
	if network.Dns == nil || network.Dns.Domain == nil {
		return ""
	}
	return *network.Dns.Domain
}

// ProcessNetworks handles the common network processing workflow
func (b *BaseMode) ProcessNetworks(ctx context.Context) (*service.GetNetworksResponse, error) {
	modeLogger := logging.GetModeLogger(b.mode)
	
	// Log configuration
	b.LogConfiguration()

	// Fetch networks
	modeLogger.Trace("Fetching networks from ZeroTier API")
	modeLogger.Verbose("Connecting to ZeroTier API for network discovery...")
	networks, err := b.FetchNetworks(ctx)
	if err != nil {
		return nil, err
	}

	// Log discovery (before filtering)
	b.LogNetworkDiscovery(networks, true)

	// Apply filters
	modeLogger.Trace("Applying network filters")
	modeLogger.Verbose("Starting network filtering process...")
	logging.FilterLogger.Trace("ApplyFilters() started")
	b.ApplyFilters(networks)

	// Log discovery (after filtering)
	b.LogNetworkDiscovery(networks, false)

	// Validate networks
	for _, network := range *networks.JSON200 {
		if err := b.ValidateNetwork(network); err != nil {
			modeLogger.Warn("Skipping invalid network: %v", err)
			continue
		}
	}

	return networks, nil
}