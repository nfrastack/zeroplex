// filepath: /home/dave/src/gh/zt-dns-companion/pkg/modes/base.go
// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package modes

import (
	"zt-dns-companion/pkg/client"
	"zt-dns-companion/pkg/config"
	"zt-dns-companion/pkg/filters"
	"zt-dns-companion/pkg/logger"
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
	logger.Trace("Creating ZeroTier API client")

	// Create API client
	sAPI, err := client.NewServiceAPI(b.cfg.Default.TokenFile)
	if err != nil {
		logger.Error("Failed to create service API client: %v", err)
		return nil, fmt.Errorf("failed to create service API client: %w", err)
	}
	logger.Debug("API client created successfully")

	// Create ZeroTier client
	ztBaseURL := fmt.Sprintf("%s:%d", b.cfg.Default.Host, b.cfg.Default.Port)
	logger.Trace("Creating ZeroTier client with URL: %s", ztBaseURL)
	ztClient, err := service.NewClient(ztBaseURL, service.WithHTTPClient(sAPI))
	if err != nil {
		logger.Error("Failed to create ZeroTier client: %v", err)
		return nil, fmt.Errorf("failed to create ZeroTier client: %w", err)
	}
	logger.Debug("ZeroTier client created successfully")

	// Fetch networks
	logger.Trace("Making API request to fetch networks")
	resp, err := ztClient.GetNetworks(ctx)
	if err != nil {
		logger.Error("Failed to get networks: %v", err)
		return nil, fmt.Errorf("failed to get networks: %w", err)
	}
	logger.Debug("API request completed successfully")

	logger.Trace("Parsing API response")
	networks, err := service.ParseGetNetworksResponse(resp)
	if err != nil {
		logger.Error("Failed to parse networks response: %v", err)
		return nil, fmt.Errorf("failed to parse networks response: %w", err)
	}
	logger.Debug("Response parsed successfully")

	return networks, nil
}

// ApplyFilters applies configured filters to networks
func (b *BaseMode) ApplyFilters(networks *service.GetNetworksResponse) {
	filters.ApplyFilters(networks, b.cfg.Default)
}

// LogNetworkDiscovery logs the network discovery process
func (b *BaseMode) LogNetworkDiscovery(networks *service.GetNetworksResponse, preFilter bool) {
	if preFilter {
		logger.Debugf("Retrieved %d networks from ZeroTier", len(*networks.JSON200))
		logger.Verbose("Network discovery completed successfully")

		// Log each network found (before filtering)
		for i, network := range *networks.JSON200 {
			logger.Trace("Network %d: ID=%s, Name=%s, Interface=%s, Status=%s",
				i+1,
				utils.GetString(network.Id),
				utils.GetString(network.Name),
				utils.GetString(network.PortDeviceName),
				utils.GetString(network.Status))
		}
	} else {
		logger.Debugf("After filtering: %d networks to process", len(*networks.JSON200))
		logger.Verbose("Network filtering completed, proceeding with configuration...")

		// Log each network that will be processed (after filtering)
		for i, network := range *networks.JSON200 {
			logger.Debug("Processing network %d: ID=%s, Name=%s, Interface=%s",
				i+1,
				utils.GetString(network.Id),
				utils.GetString(network.Name),
				utils.GetString(network.PortDeviceName))

			if network.Dns != nil && network.Dns.Servers != nil {
				logger.Verbose("  DNS servers: %v", *network.Dns.Servers)
			}
			if network.Dns != nil && network.Dns.Domain != nil {
				logger.Verbose("  DNS domain: %s", *network.Dns.Domain)
			}
			if network.AssignedAddresses != nil {
				logger.Verbose("  Assigned addresses: %v", *network.AssignedAddresses)
			}
		}
	}
}

// LogConfiguration logs the configuration details
func (b *BaseMode) LogConfiguration() {
	logger.Debug("Configuration - Host: %s, Port: %d, TokenFile: %s",
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